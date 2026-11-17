package main

import (
	"errors"

	"github.com/vst93/bili-fm/app/internal/bilibili"
	"github.com/vst93/bili-fm/app/internal/media"
)

// errNoAudioSource 表示两条取地址路径都没有给出可播放的 URL。
//
// 它不是网络错误：上游可能只是明确回了空 durl / 空 dash。调用方据此沿用
// 旧的「该视频暂时无法播放」提示，而不是把一次本地判断伪装成 HTTP 失败。
var errNoAudioSource = errors.New("起播：没有可用的音频地址")

// audioURLSource 是一次取播放地址的查找。生产实现是
// bilibili.BL.GetDashAudioByCid / GetUrlByCid；测试用闭包替身。
type audioURLSource func(aid, cid int) bilibili.PlayURLInfo

// audioURLPlayer 只暴露起播需要的 Play。生产实现带上 audioHeaders 与
// 回填曲目信息的回调；测试用假的播放器记录调用顺序。
type audioURLPlayer func(url string) error

// audioStartResult 是音轨选择的结果：URL 非空表示已经起播成功；Err 非空
// 表示没有可播的地址或最后一次 Play 的错误。
type audioStartResult struct {
	URL string
	Err error
}

// startAudioSource 选择音频地址并起播。
//
// 先试 DASH 的 AAC-LC（GetDashAudioByCid 的既有挑轨规则）：它是独立音轨，
// 起播只读 ftyp/moov/sidx + 第一个分片，不会像 360P 合流那样先扫一整块
// 巨大 moov。DASH 没有可用地址、或这次 Play 失败时，才回退到既有的合流
// MP4（GetUrlByCid），且整次起播最多回退一次。
//
// stale 在每次可能阻塞的查找 / Play 前后复查：用户切歌后旧请求不得继续
// 发请求、不得回退、更不得把新一集的起播顶掉。ErrSuperseded 原样返回，
// 调用方会整条忽略；同一个 URL 不会因为回退被重播第二次。
func startAudioSource(aid, cid int, dash, combined audioURLSource, play audioURLPlayer, stale func() bool) audioStartResult {
	isStale := func() bool { return stale != nil && stale() }
	superseded := audioStartResult{Err: media.ErrSuperseded}

	if isStale() {
		return superseded
	}

	dashURL := dash(aid, cid).URL
	if isStale() {
		return superseded
	}

	var dashErr error
	if dashURL != "" {
		err := play(dashURL)
		if err == nil {
			if isStale() {
				return superseded
			}
			return audioStartResult{URL: dashURL}
		}
		if errors.Is(err, media.ErrSuperseded) {
			return audioStartResult{Err: err}
		}
		if isStale() {
			return superseded
		}
		dashErr = err
	}

	combinedURL := combined(aid, cid).URL
	if isStale() {
		return superseded
	}
	if combinedURL == "" {
		if dashErr != nil {
			return audioStartResult{Err: dashErr}
		}
		return audioStartResult{Err: errNoAudioSource}
	}
	if dashErr != nil && combinedURL == dashURL {
		// 回退地址就是刚刚失败的那一条：不重播同一个 URL。
		return audioStartResult{Err: dashErr}
	}

	err := play(combinedURL)
	if err != nil {
		return audioStartResult{Err: err}
	}
	if isStale() {
		return superseded
	}
	return audioStartResult{URL: combinedURL}
}
