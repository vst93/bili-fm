package main

// 原生视频不可用时的「询问式回退」。
//
// 从前（video.go 的 tryNative / nativeWindow.onFailed）是**自动回退**：判定
// 原生不可用就当场落回 webview，只弹一条 toast。用户看到的是「为什么这次界面
// 不一样 / 为什么内存高了」全是玄学；更要紧的是开关写着「原生视频播放」、实际
// 却在 webview 里播 —— 等于替他改了选择。
//
// 现在的契约（本文件）：
//
//   - 原生不可用 → 弹应用内对话框，说明**为什么**（平台相关：Linux 明确写缺
//     哪些 GStreamer 插件）和**怎么修**（Linux 给可直接复制的安装命令）；
//   - 两个按钮：「切换到 WebView 播放」/「取消」；
//   - 选「切换」→ 开关落盘为 webview（nativeVideoPlayer=false）并用 webview
//     立即重开当前这一集（进度尽量接上），此后不再问；
//   - 选「取消」→ 不播（回主窗），开关**保持用户原来的选择**，下次起播再问；
//   - 没有「不再询问」这类会悄悄吃掉用户选择的选项。
//
// WebView 播放本身、frontend/video.html、设置里的「原生视频播放」开关与
// nativeVideoPlayer 键全部保留。

import (
	"errors"
	"strconv"

	"github.com/vst93/bili-fm/app/internal/view"
	"github.com/vst93/bili-fm/app/internal/video"
)

// videoOpenErrorPrompt 把 video.Manager.Open 的错误分成两类：
//
//	*FallbackPrompt → 原生不可用，要问用户（弹对话框）；
//	其它         → 真的打不开（webview 也建不起来等），走错误 toast。
func videoOpenErrorPrompt(err error) *video.FallbackPrompt {
	var p *video.FallbackPrompt
	if errors.As(err, &p) {
		return p
	}
	return nil
}

// videoFallbackTitle 是这条询问的标题；也用来判「是不是已经挂着一条」——
// 同一时刻只提示一次，不叠层、不循环弹。
const videoFallbackTitle = "原生视频播放不可用"

// showVideoFallbackPrompt 弹出「原生不可用」询问对话框。
//
// 由两条路径调：openVideo 里 Open 返回 *video.FallbackPrompt（起播器根本
// 没建起来），以及 video.Manager.OnFallback（原生窗开出来了才起播失败）。
func (c *controller) showVideoFallbackPrompt(p *video.FallbackPrompt) {
	if p == nil || c.app == nil {
		return
	}
	if m := c.app.Modal; m != nil && m.Title == videoFallbackTitle {
		// 已经有一条同样的询问挂着：不重复叠层。用户答完之后再起播仍会问。
		return
	}
	msg := p.Reason
	if p.Hint != "" {
		msg += "\n\n" + p.Hint
	}
	c.app.Modal = &view.Modal{
		Kind:    "message",
		Type:    "warning",
		Title:   videoFallbackTitle,
		Message: msg,
		Buttons: []view.ModalButton{
			{Label: "切换到 WebView 播放", Primary: true, OnClick: c.switchVideoToWebView},
			{Label: "取消", OnClick: c.cancelVideoFallback},
		},
	}
	c.app.Win.Update(func() {})
}

// cancelVideoFallback 是「取消」：不播（回主窗），什么都不改。
//
// 开关**保持原样**，挂起的那一集也丢掉 —— 用户没说要用 webview，下一次起播
// 仍会尝试原生，不可用时再问一次（选择权始终在他手上）。
func (c *controller) cancelVideoFallback() {
	c.closeModal()
	if c.vid != nil {
		c.vid.CancelFallback()
	}
	c.app.VideoOpen = false
	c.app.Playing = c.audioPlaying()
	c.app.Win.Update(func() {})
}

// switchVideoToWebView 是「切换到 WebView 播放」：把开关落盘成 webview，并用
// webview 立即重开当前这一集（进度尽量接上）。
func (c *controller) switchVideoToWebView() {
	c.closeModal()
	// 用户自己的选择：落盘 + 立即作用到 Manager 的开关上，下一集起播就直接
	// 走 webview，不再问（不循环弹）。
	c.setNativeVideo(false)
	if c.vid == nil {
		return
	}
	trimMemory()
	// 弹窗自带声音：与 openVideo 同一套，先把音频引擎停下来避免双声。
	// wasPlaying 取引擎的实际状态，失败时原样还原。
	wasPlaying := c.audioPlaying()
	c.audio().Pause()
	c.app.Playing = c.audioPlaying()
	c.app.Win.Update(func() {})
	t := c.app.Current()
	go func() {
		if err := c.vid.SwitchToWebView(); err != nil {
			runOnUI(c.app, !c.app.Hidden(), func() {
				c.app.NotifyType("error", actionFailText("打开视频窗口", err))
				c.restoreAudioAfterVideo(wasPlaying)
			})
			return
		}
		if t != nil {
			c.startVideoDanmaku(t.Cid)
		}
		runOnUI(c.app, !c.app.Hidden(), c.videoOpened)
	}()
}

// setNativeVideo 设置「原生视频播放」开关并落盘。
//
// 设置菜单的勾选与「切换到 WebView 播放」都走这里：Manager 的开关要立即
// 生效（不然用户关掉后下一个视频还会先走一遍原生），KV 键仍是既有的
// nativeVideoPlayer（只增不改）。
func (c *controller) setNativeVideo(on bool) {
	if c.app == nil {
		return
	}
	c.app.NativeVideo = on
	if c.vid != nil {
		c.vid.SetNativeEnabled(on)
	}
	if c.kv != nil {
		_ = c.kv.SetString(prefNativeVideo, strconv.FormatBool(on))
	}
	c.app.Win.Update(func() {})
}
