package main

import (
	"errors"
	"testing"

	"github.com/vst93/bili-fm/app/internal/bilibili"
	"github.com/vst93/bili-fm/app/internal/media"
)

// issue #42 的音轨选择：DASH AAC-LC 优先，缺失或 Play 失败才回退一次到
// 合流 MP4。这些测试只碰 startAudioSource 的三个注入缝（两条查找 + Play），
// 不启动播放器、不发网络、也不改 app 的倍速/均衡等设置。

func urlSource(url string, calls *int) audioURLSource {
	return func(aid, cid int) bilibili.PlayURLInfo {
		*calls++
		if aid != 117624373577490 || cid != 43646192976 {
			panic("查找参数被改掉了")
		}
		return bilibili.PlayURLInfo{URL: url}
	}
}

func TestStartAudioSourcePrefersDASH(t *testing.T) {
	var dashCalls, combinedCalls int
	var played []string
	res := startAudioSource(117624373577490, 43646192976,
		urlSource("dash-aac-lc", &dashCalls),
		urlSource("combined-mp4", &combinedCalls),
		func(url string) error { played = append(played, url); return nil },
		nil)
	if res.Err != nil || res.URL != "dash-aac-lc" {
		t.Fatalf("结果 = %+v，想 DASH 地址成功", res)
	}
	if dashCalls != 1 || combinedCalls != 0 {
		t.Fatalf("查找次数 DASH=%d 合流=%d，想 1/0（DASH 成功不该再取合流）",
			dashCalls, combinedCalls)
	}
	if len(played) != 1 || played[0] != "dash-aac-lc" {
		t.Fatalf("Play 调用 = %v，想只播 DASH", played)
	}
}

func TestStartAudioSourceMissingDASHUsesMP4(t *testing.T) {
	var dashCalls, combinedCalls int
	var played []string
	res := startAudioSource(117624373577490, 43646192976,
		urlSource("", &dashCalls),
		urlSource("combined-mp4", &combinedCalls),
		func(url string) error { played = append(played, url); return nil },
		nil)
	if res.Err != nil || res.URL != "combined-mp4" {
		t.Fatalf("结果 = %+v，想回退合流成功", res)
	}
	if dashCalls != 1 || combinedCalls != 1 {
		t.Fatalf("查找次数 DASH=%d 合流=%d，想各一次", dashCalls, combinedCalls)
	}
	if len(played) != 1 || played[0] != "combined-mp4" {
		t.Fatalf("Play 调用 = %v，想只播合流", played)
	}
}

func TestStartAudioSourceFailingDASHFallsBackOnce(t *testing.T) {
	var dashCalls, combinedCalls int
	var played []string
	dashErr := media.WrapKind(media.KindNetwork, errors.New("dash boom"))
	res := startAudioSource(117624373577490, 43646192976,
		urlSource("dash-aac-lc", &dashCalls),
		urlSource("combined-mp4", &combinedCalls),
		func(url string) error {
			played = append(played, url)
			if url == "dash-aac-lc" {
				return dashErr
			}
			return nil
		},
		nil)
	if res.Err != nil || res.URL != "combined-mp4" {
		t.Fatalf("结果 = %+v，想 DASH 失败后合流成功", res)
	}
	if len(played) != 2 || played[0] != "dash-aac-lc" || played[1] != "combined-mp4" {
		t.Fatalf("Play 调用 = %v，想 DASH 一次 + 合流一次", played)
	}
	if combinedCalls != 1 {
		t.Fatalf("合流查找 %d 次，想只回退一次", combinedCalls)
	}
}

func TestStartAudioSourceBothFailReturnsLastPlayError(t *testing.T) {
	var dashCalls, combinedCalls int
	dashErr := errors.New("dash play failed")
	combinedErr := media.WrapKind(media.KindDecode, errors.New("combined play failed"))
	res := startAudioSource(117624373577490, 43646192976,
		urlSource("dash-aac-lc", &dashCalls),
		urlSource("combined-mp4", &combinedCalls),
		func(url string) error {
			if url == "dash-aac-lc" {
				return dashErr
			}
			return combinedErr
		},
		nil)
	if !errors.Is(res.Err, combinedErr) {
		t.Fatalf("错误 = %v，想最后一条合流 Play 的实际错误", res.Err)
	}
	if media.KindOf(res.Err) != media.KindDecode {
		t.Fatalf("错误类别 = %v，想保留合流的 KindDecode", media.KindOf(res.Err))
	}
}

func TestStartAudioSourceDASHPlayErrorSurvivesEmptyCombined(t *testing.T) {
	var dashCalls, combinedCalls int
	dashErr := media.WrapKind(media.KindNetwork, errors.New("dash play failed"))
	res := startAudioSource(117624373577490, 43646192976,
		urlSource("dash-aac-lc", &dashCalls),
		urlSource("", &combinedCalls),
		func(string) error { return dashErr },
		nil)
	if !errors.Is(res.Err, dashErr) {
		t.Fatalf("错误 = %v，想保留 DASH 的实际错误而不是编一条空地址错误", res.Err)
	}
}

func TestStartAudioSourceNoSourcesIsLocalSentinel(t *testing.T) {
	var dashCalls, combinedCalls int
	res := startAudioSource(117624373577490, 43646192976,
		urlSource("", &dashCalls),
		urlSource("", &combinedCalls),
		func(string) error {
			t.Fatal("两条地址都为空时不该调用 Play")
			return nil
		},
		nil)
	if !errors.Is(res.Err, errNoAudioSource) {
		t.Fatalf("错误 = %v，想本地空地址哨兵", res.Err)
	}
	if media.KindOf(res.Err) != media.KindOther {
		t.Fatalf("空地址被伪装成了 %v，应该是没归类的本地结果", media.KindOf(res.Err))
	}
}

func TestStartAudioSourceStaleBeforeLookupMakesNoRequests(t *testing.T) {
	var dashCalls, combinedCalls, playCalls int
	res := startAudioSource(117624373577490, 43646192976,
		urlSource("dash-aac-lc", &dashCalls),
		urlSource("combined-mp4", &combinedCalls),
		func(string) error { playCalls++; return nil },
		func() bool { return true })
	if !errors.Is(res.Err, media.ErrSuperseded) {
		t.Fatalf("错误 = %v，想 ErrSuperseded", res.Err)
	}
	if dashCalls+combinedCalls+playCalls != 0 {
		t.Fatalf("stale 后仍发了请求：DASH=%d 合流=%d Play=%d",
			dashCalls, combinedCalls, playCalls)
	}
}

func TestStartAudioSourceStaleAfterDASHLookupSkipsPlayAndFallback(t *testing.T) {
	var dashCalls, combinedCalls, playCalls int
	looked := false
	res := startAudioSource(117624373577490, 43646192976,
		func(aid, cid int) bilibili.PlayURLInfo {
			dashCalls++
			looked = true
			return bilibili.PlayURLInfo{URL: "dash-aac-lc"}
		},
		urlSource("combined-mp4", &combinedCalls),
		func(string) error { playCalls++; return nil },
		func() bool { return looked })
	if !errors.Is(res.Err, media.ErrSuperseded) {
		t.Fatalf("错误 = %v，想 ErrSuperseded", res.Err)
	}
	if playCalls != 0 || combinedCalls != 0 {
		t.Fatalf("DASH 查找后 stale 仍继续：Play=%d 合流=%d", playCalls, combinedCalls)
	}
}

func TestStartAudioSourceStaleAfterDASHPlayFailureSkipsFallback(t *testing.T) {
	var combinedCalls, playCalls int
	failed := false
	res := startAudioSource(117624373577490, 43646192976,
		func(int, int) bilibili.PlayURLInfo { return bilibili.PlayURLInfo{URL: "dash-aac-lc"} },
		urlSource("combined-mp4", &combinedCalls),
		func(string) error {
			playCalls++
			failed = true
			return errors.New("dash play failed")
		},
		func() bool { return failed })
	if !errors.Is(res.Err, media.ErrSuperseded) {
		t.Fatalf("错误 = %v，想 ErrSuperseded", res.Err)
	}
	if playCalls != 1 || combinedCalls != 0 {
		t.Fatalf("DASH Play 失败后 stale 仍回退：Play=%d 合流=%d", playCalls, combinedCalls)
	}
}

func TestStartAudioSourceSupersededPlayDoesNotFallback(t *testing.T) {
	var combinedCalls, playCalls int
	res := startAudioSource(117624373577490, 43646192976,
		func(int, int) bilibili.PlayURLInfo { return bilibili.PlayURLInfo{URL: "dash-aac-lc"} },
		urlSource("combined-mp4", &combinedCalls),
		func(string) error {
			playCalls++
			return media.ErrSuperseded
		},
		nil)
	if !errors.Is(res.Err, media.ErrSuperseded) {
		t.Fatalf("错误 = %v，想 ErrSuperseded", res.Err)
	}
	if playCalls != 1 || combinedCalls != 0 {
		t.Fatalf("ErrSuperseded 后仍回退：Play=%d 合流=%d", playCalls, combinedCalls)
	}
}

func TestStartAudioSourceSameURLNotRetried(t *testing.T) {
	var combinedCalls, playCalls int
	sameErr := errors.New("same url failed")
	res := startAudioSource(117624373577490, 43646192976,
		func(int, int) bilibili.PlayURLInfo { return bilibili.PlayURLInfo{URL: "same-url"} },
		func(int, int) bilibili.PlayURLInfo {
			combinedCalls++
			return bilibili.PlayURLInfo{URL: "same-url"}
		},
		func(string) error {
			playCalls++
			return sameErr
		},
		nil)
	if !errors.Is(res.Err, sameErr) {
		t.Fatalf("错误 = %v，想保留第一次 Play 的错误", res.Err)
	}
	if playCalls != 1 || combinedCalls != 1 {
		t.Fatalf("同一 URL 被重播：Play=%d 合流查找=%d，想 1/1", playCalls, combinedCalls)
	}
}

func TestStartAudioSourceStaleAfterSuccessfulPlayIsSuperseded(t *testing.T) {
	var combinedCalls int
	played := false
	res := startAudioSource(117624373577490, 43646192976,
		func(int, int) bilibili.PlayURLInfo { return bilibili.PlayURLInfo{URL: "dash-aac-lc"} },
		urlSource("combined-mp4", &combinedCalls),
		func(string) error { played = true; return nil },
		func() bool { return played })
	if !errors.Is(res.Err, media.ErrSuperseded) {
		t.Fatalf("错误 = %v，想成功 Play 后 stale 也返回 ErrSuperseded", res.Err)
	}
	if combinedCalls != 0 {
		t.Fatalf("成功 Play 后 stale 仍取合流：%d 次", combinedCalls)
	}
}
