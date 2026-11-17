// Package video 管理视频弹窗：一个临时的 webview 窗口。
//
// 为什么视频要单独开 webview：H.264 没有可用的纯 Go 解码器（唯一像样的
// eyevinn/hi264 还很新），自己实现等于重写一个解码器。而浏览器引擎本来就
// 带着 H.264 硬解、系统原生控件、画中画、全屏 —— 这些自己写反而做不好。
//
// 为什么是「临时」窗口：一个 webview 窗口在 Windows 上带着约 330MB 的
// WebView2 基础开销（浏览器/GPU/网络/存储等进程）。实测：原生 UI 主窗
// 只有 57MB，打开媒体窗口升到 506MB，**关掉后回落到 69MB**。所以看完就关，
// 内存能收回来。
//
// 音频不在这里：音频走 internal/media 的纯 Go 引擎，不需要 webview。
package video

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/url"
	"sync"
	"time"

	"github.com/egoist/mygo"

	"github.com/vst93/bili-fm/app/internal/danmaku"
	"github.com/vst93/bili-fm/app/internal/view"
)

// Options 描述要打开的视频。
type Options struct {
	// Src 是播放地址，应当是本地代理 URL（webview 带不了 Referer）。
	Src string
	// SrcFallback 是 Src 起播失败时**自动**再试一次的那条地址（当前是
	// platform=html5 的 360P 合流）。它只用于原生路径：视频窗优先播 DASH
	// 分片（见 internal/dashhls），而分片地址/索引在极少数稿件上可能不被
	// 系统播放器接受 —— 那种情况下退回合流比弹一个「改用 WebView」的询问
	// 好得多（用户没做错任何事，画质也一样）。
	//
	// 空串表示没有可退的地址。**不**在 ErrUnsupported 时重试：那是平台
	// 缺解码器，换地址救不了，仍然走既有的「问用户要不要用 WebView」。
	SrcFallback string
	// Title 显示在弹窗标题栏。
	Title string
	// Poster 是封面（可选）。
	Poster string
	// StartAt 是续播位置（秒）。
	StartAt float64
	// Speed 是当前倍速，跟随主窗。
	Speed float64
	// Muted 表示主窗是静音状态。
	Muted bool
	// Theme 是主窗这一帧的主题令牌：原生窗的标题栏与控件条按它画（深浅两态、
	// 高对比、减少透明度都跟着主窗走），见 internal/view/videochrome.go。
	Theme view.Theme
	// Parent 是主窗，用于让弹窗跟随。
	Parent *mygo.Window

	// DanmakuCid 是这一集的弹幕 cid（0 表示没有，覆盖层保持空）。
	DanmakuCid int64
	// Danmaku 是这一集的弹幕条目（可以为空：数据还在路上，随后由
	// Manager.SetDanmaku 补上）。只给原生窗用；webview 回退路径本轮不注入
	// 覆盖层（见 PLAN.md 的取舍）。
	Danmaku []danmaku.Item

	// Shared 是主窗与视频窗共享的音频设置（音量 / 静音 / 倍速 / 均衡 /
	// 广告跳过）。原生窗拿到的是 &App.PlayerSettings，两边同一份内存：
	// 任一侧改了，另一侧立刻看见（见 view.PlayerSettings）。
	// webview 回退路径仍用 Speed / Muted 起播。
	Shared *view.PlayerSettings
	// SponsorSegments 是当前这一集的恰饭分段（进度条标记 + 视频窗跳过）。
	SponsorSegments []view.Segment
	// DanmakuHeat / HeatOn 是弹幕热力：与主窗**同一份数据**（主窗 App 的
	// DanmakuHeat / HeatOn，本身来自那条 cid 缓存）。视频窗只是把它画到
	// 自己的控件条上（同一个 painter），不另拉一次请求。
	DanmakuHeat []view.HeatPoint
	HeatOn      bool
	// VideoPinned 是视频窗开窗时的置顶状态（主窗落盘的 videoPinned）。
	// 默认 false：与主窗迷你模式的固定一致；窗口创建后那颗图钉可以改它。
	// 只对原生窗生效 —— webview 回退路径没有自绘顶栏，也就没有改动它的
	// 入口（不把一个退不回来的置顶强塞给用户）。
	VideoPinned bool
}

// State 是弹窗回传的播放状态。
type State struct {
	Time     float64
	Duration float64
	Paused   bool
	Muted    bool
	Speed    float64
	// Ready 是「有没有第一帧画面」：原生路径只有 0 / 1，webview 回退路径
	// 回传的是 HTMLMediaElement.readyState（0..4）。两者语义不同，新增字段
	// 不要拿它当通用的「就绪」判据（见 Buffering）。
	Ready int
	// Buffering 是原生播放器报告的「正在等数据」。webview 路径没有同名字段
	// （页面只回传 readyState），保持 false；主窗对回退路径仍按 Ready 判加载。
	Buffering bool
	Err       string
}

// FallbackPrompt 是「原生播放不可用，等你决定要不要改用 WebView」的询问。
//
// 它是 Manager.Open 的返回值（实现了 error）：主窗拿到后弹一个应用内对话框，
// 由用户自己选「切换到 WebView 播放」还是「取消」。**不自动回退** —— 开关是
// 用户的，替他从原生改到 webview 等于悄悄剥夺选择权（用户定调）。
//
// 起播失败（原生窗已经开出来了才发现解码器缺失）没法从 Open 返回，走的是
// Manager.OnFallback 回调，但带的是同一个结构：两条路径在主窗那里汇成同一个
// 对话框。
type FallbackPrompt struct {
	// Reason 是「为什么不能用」的一句话，平台相关（Linux 写明缺哪些
	// GStreamer 插件）。
	Reason string
	// Hint 是「怎么修」：Linux 给可直接复制的安装命令，macOS / Windows
	// 给通用兜底说明。
	Hint string
}

// Error 让 FallbackPrompt 能当成 Open 的返回值用（errors.As 取回）。
func (p *FallbackPrompt) Error() string {
	if p == nil {
		return ""
	}
	return p.Reason
}

// ErrNoPendingFallback 表示没有「等用户决定」的那一集可重开：主窗的
// 「切换到 WebView 播放」只在真的挂起过一集时才该调 SwitchToWebView。
var ErrNoPendingFallback = errors.New("video: 没有等待重开的视频")

// RelayOnClose 报告关掉视频窗之后要不要把音频引擎接回来接着听（音视接力）。
//
// 语义以原版为准（tauri 分支 src/components/playerVideo.tsx 的
// closePlayerVideo）：
//
//   - **暂停时关窗不续播**。原版注释写明「不要在此恢复音频播放（isPlaying
//     保持 false），由用户手动点击播放键继续收听，避免 macOS 上音频与视频
//     抢播」—— 暂停是用户的明确意图，关窗不是「开始播放」的手势。
//   - 播放中关窗才接回（本仓库 webview / 原生两条路径既有的音视接力）。
//   - 没起来（Ready==0）、出错、或者自然播完（位置停在片尾）都不接：这时
//     接回来只会得到一段静音、一段错误提示音，或者贴着片尾的一两声。
//
// webview 路径的 State 来自页面回传（play/pause 事件已经即时上报），原生
// 路径来自关窗那一刻的播放器，两者判据一致。
func RelayOnClose(st State) bool {
	if st.Paused || st.Ready == 0 || st.Err != "" {
		return false
	}
	// 自然播完：ended 之后页面不再每秒回传，缓存的 Paused 可能还是旧的
	// 「在播」，但位置已经在末尾。用与主窗「播完再点播放」同一条判据。
	if st.Duration > 0 && st.Time >= st.Duration-0.5 {
		return false
	}
	return true
}

// Manager 管理唯一一个视频弹窗。
type Manager struct {
	mu  sync.Mutex
	win *mygo.Window
	st  State
	// opening 表示一次 Open 正在进行（异步的窗口创建落定之前，
	// 再来的 Open 直接拒绝，避免快速连点开出多个弹窗）。
	opening bool

	onState func(State)
	onClose func(State)
	onEnded func()
	onQuit  func()

	// ended 记录页面是否已经自然播完。页面在 ended 后不再每秒回传状态，
	// 单看 Paused/Time 会误判；主窗再点播放时用它决定先 seek(0)。
	ended bool

	// minimize 是最小化弹窗自身的函数：Open 时绑定到窗口的 Minimize，
	// Close / 窗口关闭时清空。抽成字段便于单测验证 Service.Minimize 的接线
	// （测试里没有真实窗口，注入一个计数函数即可）。
	minimize func()

	// native 是当前的原生视频窗（ui/video）；nil 表示走 webview 那条路径。
	// 两条路径互斥：同一时刻只有一个弹窗，State/Seek/SetSpeed 等都要按它分派。
	native *nativeWindow

	// nativeOK 是用户的选择（设置项「原生视频播放」，默认开）。
	//
	// 与从前不同，它**不再**在「判定不可用」时被置假：原生不可用要问用户
	// （见 FallbackPrompt），不能替用户把开关改掉。只有两条路会置假 ——
	// 用户在设置里关掉，或用户在上一条询问里点了「切换到 WebView 播放」。
	nativeOK bool

	// pending 是「原生不可用、等用户决定」的那一次请求。用户选「切换」时用
	// 它重开同一集（进度尽量接上）；选「取消」时清掉。
	pending *Options

	// theme / hasTheme 是主窗最近一次下发的主题令牌（SetTheme）。它是视频窗
	// 外壳的唯一事实源：建窗时优先用它，避免「快照—建窗」之间主窗换了深浅、
	// 窗口却带着旧令牌开出来，要关掉重开才跟上。
	theme    view.Theme
	hasTheme bool

	// onFallback 在原生路径不可用、等用户决定时调用（主窗据此弹询问对话框）。
	onFallback func(p *FallbackPrompt)

	// 下面三个是「起播 / 建窗」的接线口：默认是真实的 openPlayer /
	// newNativeWindow / openWebView。无头测试换成桩，就能断言「不可用时
	// 没有自动切 webview」「切换后这一集用 webview 重开」而不碰窗口与
	// 系统媒体框架。
	openNative    func(src string) (playerAPI, error)
	newNativeWin  func(o Options, pl playerAPI) *nativeWindow
	openWebViewFn func(o Options) error

	// 弹幕：当前这一集的 cid / 条目 / 开关 / 基本设置。设置是主窗的
	// 事实源（落盘后经 SetDanmakuSettings 下发），窗口只读。
	dmCid      int64
	dmItems    []danmaku.Item
	dmOn       bool
	dmSettings danmaku.Settings
	// onDanmakuToggle 在视频窗内点弹幕键时调用（主窗落盘并同步回来）。
	onDanmakuToggle func(bool)
	// onDanmakuStyle 在视频窗设置面板里改不透明度 / 字号 / 显示区域 /
	// 同屏上限时调用（主窗落盘 + 同步回所有入口）。
	onDanmakuStyle func(danmaku.Settings)
	// onNativeToggle 在视频窗设置面板里点「原生视频播放」时调用（主窗落盘
	// + 作用到本 Manager 的开关上）。这是这个开关的**唯一**入口。
	onNativeToggle func(bool)
	// onPinToggle 在视频窗顶栏点「置顶」键时调用（主窗落盘 videoPinned）。
	onPinToggle func(bool)

	// heat / heatOn 是弹幕热力（主窗同一份数据的镜像，见 SetDanmakuHeat）。
	// 建窗时取自 Options，之后由主窗推送。
	heat   []view.HeatPoint
	heatOn bool
}

// New 建一个管理器。
func New() *Manager {
	return &Manager{nativeOK: true, dmOn: true, dmSettings: danmaku.DefaultSettings()}
}

// SetDanmaku 更新当前这一集的弹幕条目（按 cid 匹配才落到窗口上：快速换集
// 时旧响应不能画到新一集）。
func (m *Manager) SetDanmaku(cid int64, items []danmaku.Item) {
	m.mu.Lock()
	m.dmCid, m.dmItems = cid, items
	n := m.native
	m.mu.Unlock()
	if n != nil {
		n.apply(func() { n.setDanmaku(cid, items) })
	}
}

// SetSponsorSegments 把当前这一集的恰饭分段同步给视频窗（进度条标记 +
// 开关打开时的跳过）。主窗查询完成 / 换集清空时调用。
func (m *Manager) SetSponsorSegments(segs []view.Segment) {
	m.mu.Lock()
	n := m.native
	m.mu.Unlock()
	if n == nil || n.win == nil {
		return
	}
	n.win.Update(func() {
		n.segments = segs
		if n.chrome != nil {
			n.chrome.SponsorSegments = segs
		}
	})
}

// SetDanmakuHeat 把弹幕热力同步给视频窗（控件条上那条热力带）。
//
// 数据是主窗**同一份**（同一个 cid 缓存聚出来的桶），这里只转发引用：视频窗
// 不自己请求、不自己聚合，两窗的热力因此不可能对不上。热力是可选增强：没有
// 原生窗（webview / 未开窗）时什么都不做。
func (m *Manager) SetDanmakuHeat(pts []view.HeatPoint, on bool) {
	m.mu.Lock()
	m.heat, m.heatOn = pts, on
	n := m.native
	m.mu.Unlock()
	if n == nil || n.win == nil {
		return
	}
	n.win.Update(func() {
		n.heat, n.heatOn = pts, on
		if n.chrome != nil {
			n.chrome.DanmakuHeat = pts
			n.chrome.HeatOn = on
		}
	})
}

// SetDanmakuOn 开关弹幕层（视频窗内那颗键与主窗设置共用这个状态）。
func (m *Manager) SetDanmakuOn(on bool) {
	m.mu.Lock()
	m.dmOn = on
	n := m.native
	m.mu.Unlock()
	if n != nil {
		n.apply(func() { n.dmOn = on })
	}
}

// SetDanmakuSettings 下发基本设置（不透明度 / 字号 / 区域 / 上限）。
// 视频窗下一帧就按新值重排，不需要重开。
func (m *Manager) SetDanmakuSettings(s danmaku.Settings) {
	s = s.Normalize()
	m.mu.Lock()
	m.dmSettings = s
	n := m.native
	m.mu.Unlock()
	if n != nil {
		n.apply(func() { n.dmSettings = s })
	}
}

// DanmakuSettings 返回当前下发中的设置（测试 / 状态展示）。
func (m *Manager) DanmakuSettings() danmaku.Settings {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.dmSettings
}

// DanmakuState 返回当前 cid、条目数与开关（测试用；条目本身不复制）。
func (m *Manager) DanmakuState() (cid int64, items int, on bool) {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.dmCid, len(m.dmItems), m.dmOn
}

// OnDanmakuToggle 设置视频窗内弹幕键的回调。
func (m *Manager) OnDanmakuToggle(fn func(bool)) {
	m.mu.Lock()
	m.onDanmakuToggle = fn
	m.mu.Unlock()
}

// OnPinToggle 设置视频窗内「置顶」键的回调（参数是点击后的状态）。
func (m *Manager) OnPinToggle(fn func(bool)) {
	m.mu.Lock()
	m.onPinToggle = fn
	m.mu.Unlock()
}

// OnDanmakuStyle 设置视频窗设置面板里「改弹幕基本设置」的回调。
func (m *Manager) OnDanmakuStyle(fn func(danmaku.Settings)) {
	m.mu.Lock()
	m.onDanmakuStyle = fn
	m.mu.Unlock()
}

// OnNativeToggle 设置视频窗设置面板里「原生视频播放」开关的回调。
func (m *Manager) OnNativeToggle(fn func(bool)) {
	m.mu.Lock()
	m.onNativeToggle = fn
	m.mu.Unlock()
}

// SetNativeEnabled 开关原生视频路径（设置项「原生视频播放」）。
// 关掉后所有 Open 直接走 webview —— 给「GStreamer 装是装了但播得不好」或
// 需要弹幕覆盖层 / 画中画的用户一条明确的退路，也正好是「切换到 WebView
// 播放」这一个动作的落点。
//
// 注意它**不**清 pending：用户的「切换」是先落盘再重开挂起的那一集，清掉就
// 没得重开了。
func (m *Manager) SetNativeEnabled(on bool) {
	m.mu.Lock()
	m.nativeOK = on
	n := m.native
	m.mu.Unlock()
	// 已开着的原生窗也要跟着变：设置面板里那一行的按压态读它（面板里自己
	// 点的那一下已经翻过本地状态，这里管的是主窗 / 另一条入口改的情况）。
	if n != nil {
		n.apply(func() { n.nativeVideo = on })
	}
}

// NativeEnabled 报告设置项当前是否允许走原生路径。
func (m *Manager) NativeEnabled() bool {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.nativeOK
}

// OnFallback 设置「原生视频不可用，等用户决定」的回调。
// 参数是给用户看的「为什么 + 怎么修」，主窗据此弹询问对话框（不自动回退）。
func (m *Manager) OnFallback(fn func(p *FallbackPrompt)) { m.onFallback = fn }

// PendingFallback 报告有没有「等用户决定」的那一集（测试 / 状态展示用）。
func (m *Manager) PendingFallback() bool {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.pending != nil
}

// CancelFallback 清掉挂起的那一集（用户选了「取消」）。
//
// 开关**保持原样**：用户没说要用 webview，下一次起播仍会尝试原生，不可用时
// 再问一次 —— 选择权始终在他手上。
func (m *Manager) CancelFallback() {
	m.mu.Lock()
	m.pending = nil
	m.mu.Unlock()
}

// SwitchToWebView 用 WebView 重开挂起的那一集（用户点了「切换到 WebView
// 播放」）。
//
// 进度尽量接上：原生窗已经播过一段时用回传的位置，否则用请求自带的续播点
// （StartAt）。没有挂起的请求时返回 ErrNoPendingFallback —— 这时候**不建窗**，
// 用户看到一条错误提示，而不是莫名其妙弹一个视频窗。
func (m *Manager) SwitchToWebView() error {
	m.mu.Lock()
	var o Options
	ok := m.pending != nil
	if ok {
		o = *m.pending
	}
	m.pending = nil
	m.mu.Unlock()
	if !ok {
		return ErrNoPendingFallback
	}
	if t := m.State().Time; t > o.StartAt {
		o.StartAt = t
	}
	return m.openWeb(o)
}

// holdFallback 记下「原生不可用、等用户决定」的那一次请求。
func (m *Manager) holdFallback(o Options) {
	m.mu.Lock()
	m.pending = &o
	m.mu.Unlock()
}

// openNativePlayer 起一个原生播放器（默认 openPlayer，测试可换桩）。
func (m *Manager) openNativePlayer(src string) (playerAPI, error) {
	if fn := m.openNative; fn != nil {
		return fn(src)
	}
	return openPlayer(src)
}

// makeNativeWindow 建原生视频窗（默认 newNativeWindow，测试可换桩）。
func (m *Manager) makeNativeWindow(o Options, pl playerAPI) *nativeWindow {
	// 热力以 Manager 收到的最后一次为准：建窗期间可能刚好有新数据到达
	// （Options 里那份只是开窗那一刻的快照）。同一份数据，不复制第二份。
	m.mu.Lock()
	if m.heat != nil || m.heatOn {
		o.DanmakuHeat, o.HeatOn = m.heat, m.heatOn
	}
	m.mu.Unlock()
	if fn := m.newNativeWin; fn != nil {
		return fn(o, pl)
	}
	return newNativeWindow(o, pl)
}

// openWeb 用 webview 打开 o（默认 openWebView，测试可换桩）。
//
// 走 webview 之前一定把源换成 SrcFallback（合流 MP4）：
//   - 本地 HLS 是给原生播放器用的（分片 + 音轨分离），页面里的 <video>
//     要一条完整的渐进式 MP4，喂它 HLS 播放列表播不出来；
//   - SrcFallback 正是同一条合流地址，也是 webview 一直以来的源。
//
// 没有 SrcFallback（本来就是合流）时不动 o.Src。
func (m *Manager) openWeb(o Options) error {
	if o.SrcFallback != "" {
		o.Src = o.SrcFallback
		o.SrcFallback = ""
	}
	if fn := m.openWebViewFn; fn != nil {
		return fn(o)
	}
	return m.openWebView(o)
}

// NativeActive 报告当前弹窗是否走原生路径（供设置/状态展示与测试）。
func (m *Manager) NativeActive() bool {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.native != nil
}

// OnState 设置状态回调（约每秒一次）。
func (m *Manager) OnState(fn func(State)) { m.onState = fn }

// OnClose 设置关闭回调。
func (m *Manager) OnClose(fn func(State)) { m.onClose = fn }

// OnEnded 设置播放结束回调。
func (m *Manager) OnEnded(fn func()) { m.onEnded = fn }

// OnQuit 设置退出回调（弹窗里按 Ctrl/Cmd+Q）。
func (m *Manager) OnQuit(fn func()) { m.onQuit = fn }

// IsOpen 报告弹窗是否开着。
func (m *Manager) IsOpen() bool {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.win != nil || m.native != nil
}

// State 返回最近一次回传的状态。
func (m *Manager) State() State {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.st
}

// Ended 报告弹窗是否已经自然播完。
func (m *Manager) Ended() bool {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.ended
}

// Replay 让弹窗从开头重新播放（播完后主窗再点播放）。
func (m *Manager) Replay() {
	m.mu.Lock()
	m.ended = false
	w, n := m.win, m.native
	m.mu.Unlock()
	if n != nil {
		n.apply(func() {
			n.endedFired = false
			n.pl.Seek(0)
			n.pl.Play()
		})
		return
	}
	if w != nil {
		w.Page().Eval("(()=>{const v=document.getElementById('v');if(v){try{v.currentTime=0}catch(_){ }const p=v.play();if(p&&p.catch)p.catch(()=>{});}})()")
	}
}

// pageURL 把打开选项编成弹窗页面的地址（/video.html?src=…&muted=…）。
// 抽成纯函数便于测试：弹窗的起播状态（位置 / 倍速 / 静音）全靠这组参数。
func pageURL(o Options) string {
	q := url.Values{}
	q.Set("src", o.Src)
	q.Set("title", o.Title)
	q.Set("poster", o.Poster)
	q.Set("t", fmt.Sprintf("%.2f", o.StartAt))
	q.Set("speed", fmt.Sprintf("%.2f", max(o.Speed, 0.5)))
	if o.Muted {
		q.Set("muted", "1")
	}
	return "/video.html?" + q.Encode()
}

// switchScript 生成「原地换集」的 JS：复用同一个弹窗窗口，只换 <video> 的
// 源与起播参数（页面里的 window.biliSwitch，见 frontend/video.html）。
// 抽成纯函数便于测试：换集漏掉任何一个参数都会「换集后位置/倍速/静音不对」。
func switchScript(o Options) string {
	payload, err := json.Marshal(map[string]any{
		"src":    o.Src,
		"title":  o.Title,
		"poster": o.Poster,
		"t":      o.StartAt,
		"speed":  max(o.Speed, 0.5),
		"muted":  o.Muted,
	})
	if err != nil { // map[string]any 里的值全是可序列化的，这里不会失败
		return "window.biliSwitch({})"
	}
	return "window.biliSwitch(" + string(payload) + ")"
}

// Open 打开（或复用）弹窗。已开着的弹窗**原地换集**，不再关掉重建。
// 有一次 Open 还在创建中时，后续的 Open 拒绝：快速连点不会开出多个。
func (m *Manager) Open(o Options) error {
	m.mu.Lock()
	if m.opening {
		m.mu.Unlock()
		return nil
	}
	m.opening = true
	w, n := m.win, m.native
	m.mu.Unlock()
	defer func() {
		m.mu.Lock()
		m.opening = false
		m.mu.Unlock()
	}()

	// 新的一次起播：上一次「等用户决定」的询问到此作废（要被新一集替掉；
	// 成功的话更不该再留着旧请求）。
	m.CancelFallback()

	// 原生窗还开着：原地换集（换播放器、不换窗口）。原生窗没有浏览器进程
	// 那套开销，但也没有理由每次换集都重建窗口（窗口位置、尺寸会跳）。
	if n != nil {
		if err := m.switchNative(n, o); err == nil {
			return nil
		}
		m.CloseNative()
	}

	// 复用路径：弹窗还在，只换源。这是内存上最关键的一条 —— 见 switchTo。
	//
	// 换源失败（窗口正好在这一帧被用户关掉：OnClosed 还没跑完、IsDestroyed()
	// 仍是 false 的极短窗口）时**不吞掉**这一集，而是回落到下面的新建路径。
	// 此前是「复用就直接 return nil」——那一下 Eval 落空、下一集不播，用户看到
	// 的是「点下一集没反应」。
	if w != nil && !w.IsDestroyed() {
		if err := m.switchTo(w, o); err == nil {
			return nil
		}
	}

	m.Close()

	// 原生优先：ui/video 用系统媒体框架解码 + 出声，音画同步由框架保证，
	// 没有 WebView2 那套进程开销。判定不可用（缺媒体框架）时**不自动回退**：
	// 把这一集挂起，返回一条询问，由主窗弹对话框问用户要不要改用 WebView。
	if prompt, ok := m.tryNative(o); ok {
		return nil
	} else if prompt != nil {
		m.holdFallback(o)
		return prompt
	}

	return m.openWeb(o)
}

// openWebView 是原来的 webview 弹窗路径（回退用，也是唯一能播 webm / 弹幕
// 覆盖层 / 画中画的路径）。
func (m *Manager) openWebView(o Options) error {
	page := pageURL(o)

	w := mygo.NewWindow(mygo.WindowOptions{
		Title:     o.Title,
		URL:       page,
		Width:     960,
		Height:    600,
		MinWidth:  320,
		MinHeight: 200,
		Frameless: true,
		// mygo 0.6 的 UserAgentSuffix：让视频页在站点侧带上应用名/版本，
		// 同时保留 WebView2 自带的浏览器标识。
		Page: mygo.PageOptions{UserAgentSuffix: "bili-FM/" + mygo.App.Version()},
	})
	if w == nil {
		return fmt.Errorf("video: 创建弹窗失败")
	}

	m.mu.Lock()
	m.win, m.native, m.minimize = w, nil, w.Minimize
	m.st = State{Speed: o.Speed}
	m.mu.Unlock()

	w.OnClosed(func() {
		m.mu.Lock()
		if m.win == w {
			m.win, m.minimize = nil, nil
		}
		st := m.st
		cb := m.onClose
		m.mu.Unlock()
		if cb != nil {
			cb(st)
		}
	})
	return nil
}

// tryNative 尝试用原生路径打开 o。ok 表示「原生窗已经开起来了」；
// 否则 prompt 非空时表示「试过但不可用」，调用方据此问用户要不要改用 WebView。
//
// 判定顺序（每一步都能省掉一次黑窗或一次多余询问）：
//  1. 用户把开关关掉了（nativeVideoPlayer=false，或刚点了「切换到 WebView
//     播放」）→ 直接走 webview，不问（用户的决定要一直被尊重）。
//  2. 地址不是 http(s)/file → 跳过（不是故障，不问）。
//  3. ui/video.OpenURL 返回 ErrUnsupported / 播放器建不起来 → 给出「为什么
//     + 怎么修」的询问。此时**连窗口都没开**，所以不会有「先黑一下再问」。
//  4. 播放器起来了，但起播失败（Err 非空）→ 由 onFailed 关窗并询问。
//     这里不阻塞等待；也正因为判据是 pl.Err()，还在加载 / 缓冲时不会弹
//     （见 nativeWindow.report）。
func (m *Manager) tryNative(o Options) (prompt *FallbackPrompt, ok bool) {
	m.mu.Lock()
	allowed := m.nativeOK
	m.mu.Unlock()
	if !allowed {
		return nil, false
	}
	if !nativeEligible(o) {
		return nil, false
	}

	pl, err := m.openNativePlayer(o.Src)
	if err != nil {
		return nativeFallbackPrompt(err), false
	}

	n := m.makeNativeWindow(o, pl)
	if n == nil {
		return nativeFallbackPrompt(errors.New("创建视频窗口失败")), false
	}
	set := n.ensureSettings()
	// 弹幕状态以管理器为准：设置可能在窗口创建期间被改过；条目只在 cid
	// 对得上时才套用（上一集的缓存不能画到这一集）。
	m.mu.Lock()
	// 主题也以管理器最近一次下发的那份为准（建窗过程中主窗可能刚切了
	// 深浅色）：它比 Options 里的建窗快照新。
	if m.hasTheme {
		n.setTheme(m.theme)
	}
	n.dmOn, n.dmSettings = m.dmOn, m.dmSettings
	n.onDanmakuToggle = m.onDanmakuToggle
	n.onDanmakuStyle = m.onDanmakuStyle
	n.onNativeToggle = m.onNativeToggle
	n.onPinToggle = m.onPinToggle
	// 能在原生路径里，说明这个开关是开的（关掉的话上面就 return 了）。
	n.nativeVideo = true
	if m.dmCid == o.DanmakuCid {
		n.dm.SetItems(m.dmItems)
	}
	m.mu.Unlock()
	n.onState = func(st State) {
		m.mu.Lock()
		m.st = st
		cb := m.onState
		m.mu.Unlock()
		if cb != nil {
			cb(st)
		}
	}
	n.onClose = func(st State) {
		m.mu.Lock()
		if m.native == n {
			m.native = nil
		}
		cb := m.onClose
		m.mu.Unlock()
		if cb != nil {
			cb(st)
		}
	}
	n.onEnded = func() {
		m.mu.Lock()
		m.ended = true
		cb := m.onEnded
		m.mu.Unlock()
		if cb != nil {
			cb()
		}
	}
	n.onMin = func() {
		m.mu.Lock()
		fn := m.minimize
		m.mu.Unlock()
		if fn != nil {
			fn()
		}
	}
	n.onQuit = func() {
		m.mu.Lock()
		cb := m.onQuit
		m.mu.Unlock()
		if cb != nil {
			cb()
		}
	}
	// 起播失败（解码器缺失 / 流断了）：关掉原生窗，**问用户**要不要改用
	// WebView 播放（不自己切、也不改开关）。
	//
	// 这段回调是在视图构建（主线程）里被 n.report 触发的，而关窗 + 弹对话框
	// 都会再进主线程消息循环 —— 直接做会嵌套。丢到 goroutine 上，顺序仍然
	// 确定（先挂起、再关窗、再问）。
	//
	// 不设置 suppressClose：关窗走「退出视频播放」的既有收尾（主窗
	// VideoOpen 复位、位置接回），用户就停在那里看到这条询问。
	m.wireNativeFail(n, o)

	m.mu.Lock()
	m.native, m.win, m.minimize = n, nil, n.minimize
	m.st = State{Speed: set.Speed}
	m.ended = false
	m.mu.Unlock()

	// 起播参数：位置 / 倍速 / 静音全部在回调接好之后下发一次（播放器默认从
	// 0 起播、1 倍速、不静音）。放在最后，保证这一帧的状态回传有人接。
	// 用 n.apply（不是 n.win.Update）：测试里的桩窗没有 win，但还要验到
	// 「起播参数下发」这条路径。
	n.apply(func() {
		pl.SetRate(set.Speed)
		pl.SetMuted(set.Muted)
		pl.SetVolume(set.Volume)
		if o.StartAt > 0 {
			pl.Seek(time.Duration(o.StartAt * float64(time.Second)))
		}
		n.reported = time.Time{} // 下一帧立刻回传一次
	})
	return nil, true
}

// wireNativeFail 给一个原生窗挂上「起播失败」的处理：先自动降级一次，再问人。
//
// 为什么单独成一个函数：换集（switchNative）会把播放器换成新的，而
// nativeWindow.report 在回调触发后会把 onFailed 置 nil（防重复）。所以每次
// 换播放器都必须**重新挂一遍**，否则新播放器失败时没有任何人接手 —— 表现
// 就是「退回合流之后又失败了，界面卡在那里什么也不说」。
func (m *Manager) wireNativeFail(n *nativeWindow, o Options) {
	n.onFailed = func(err error) {
		// 第一层自动降级：Src 是 DASH 分片（见 internal/dashhls），个别稿件
		// 的索引/分片可能不被系统播放器接受；这时退回合流 MP4 重开一次，
		// 用户什么都看不见 —— 比弹「要不要改用 WebView」合适得多（画质相同，
		// 用户也没做错任何事）。只退一次，退完再失败才问人。
		//
		// 判据用 n.fallbackTried 而不是 onFailed=nil：重试要重新挂一个
		// onFailed（新的播放器），靠「回调只挂一次」记不住这件事。
		if o.SrcFallback != "" && !n.fallbackTried {
			n.fallbackTried = true
			next := o
			next.Src, next.SrcFallback = o.SrcFallback, ""
			go m.retryNative(o, next)
			return
		}
		p := nativeFallbackPrompt(err)
		go func() {
			m.holdFallback(o)
			m.CloseNative()
			m.reportFallback(p)
		}()
	}
}

// retryNative 用 next 就地重开一次（Src 起播失败的自动降级）。
//
// 为什么不用 Manager.Open：那会重新走一遍「复用 / 新建 / 原生 / webview」
// 的全套判定，还会被 opening 闸门与 CancelFallback 影响；这里只做一件事
// —— 把同一集的播放器换到备用地址上，窗口、弹幕、共享设置、进度全部保留。
//
// stale 的两道闸：调用方传进来的 orig 是**失败那一刻**的请求快照；如果这期间
// 用户已经换了集（Manager.native 不再是原来那个窗）或关了窗，就什么都不做，
// 免得把新一集顶掉。
func (m *Manager) retryNative(orig, next Options) {
	m.mu.Lock()
	n := m.native
	if n == nil {
		m.mu.Unlock()
		return
	}
	m.mu.Unlock()
	// 窗口已经不在（用户关掉了）：备用地址也不必再试。win 为 nil 是测试里的
	// 桩窗（没有真窗口），那条路径要照常走，否则测不到重试。
	if n.win != nil && n.win.IsDestroyed() {
		return
	}
	if err := m.switchNative(n, next); err != nil {
		// 换不了（窗口在换的途中被关掉）：仍然按「这一集播不了」问用户。
		p := nativeFallbackPrompt(err)
		go func() {
			m.holdFallback(orig)
			m.CloseNative()
			m.reportFallback(p)
		}()
	}
}

// switchNative 在已开着的原生窗里原地换集。
func (m *Manager) switchNative(n *nativeWindow, o Options) error {
	if n.win != nil && n.win.IsDestroyed() {
		return fmt.Errorf("video: 原生视频窗已消失")
	}
	pl, err := m.openNativePlayer(o.Src)
	if err != nil {
		return err
	}
	// 用 n.apply（不是 n.win.Update）：桩窗没有真窗口，直接执行同一段逻辑，
	// 换集的真实路径才测得到（生产有窗口时行为不变）。
	n.apply(func() {
		old := n.pl
		n.pl = pl
		n.title, n.startAt = o.Title, o.StartAt
		n.segments = o.SponsorSegments
		n.endedFired = false
		n.reported = time.Time{}
		// 新一集有它自己的自动降级机会（上一集用掉的那次不能顺延过来）。
		n.fallbackTried = false
		// 换集先清掉上一集的弹幕；这一集的条目由主窗随后 SetDanmaku 补上。
		n.dmCid = o.DanmakuCid
		n.dm.SetItems(o.Danmaku)
		// 倍速 / 静音 / 音量跟着共享设置走（换集不该把用户刚调的值重置）。
		pl.SetRate(n.settings.Speed)
		pl.SetMuted(n.settings.Muted)
		pl.SetVolume(n.settings.Volume)
		if o.StartAt > 0 {
			pl.Seek(time.Duration(o.StartAt * float64(time.Second)))
		}
		if old != nil {
			old.Close()
		}
		// 换了播放器就要重新挂失败处理（见 wireNativeFail 的说明）。
		m.wireNativeFail(n, o)
	})
	// 没有窗口（无头测试里的桩窗）时跳过窗口操作：mygo 的 SetTitle 会走
	// onMain 的 chan，nil 窗口上调用就是永久阻塞（AGENT.md 记过同一个坑）。
	if n.win != nil {
		n.win.SetTitle(o.Title)
		n.focus()
	}
	m.mu.Lock()
	m.st = State{Speed: o.Speed}
	m.ended = false
	m.mu.Unlock()
	return nil
}

// CloseNative 关掉原生窗（若有）。
func (m *Manager) CloseNative() {
	m.mu.Lock()
	n := m.native
	m.native, m.minimize = nil, nil
	m.ended = false
	m.mu.Unlock()
	if n != nil {
		n.close()
	}
}

// reportFallback 把「原生不可用，等用户决定」的询问交给主窗（弹对话框）。
func (m *Manager) reportFallback(p *FallbackPrompt) {
	if p == nil {
		return
	}
	m.mu.Lock()
	cb := m.onFallback
	m.mu.Unlock()
	if cb != nil {
		cb(p)
	}
}

// switchTo 在已开着的弹窗里原地换集（不新建 webview 窗口）。
//
// 为什么必须复用：每新建一个弹窗就要起一套 WebView2 渲染进程，换集时
// 「旧窗口正在拆、新窗口正在建」会同时驻留两份，用户实测「切几个视频 +
// 翻几个抽屉后内存 315MB」里有一份就是这种叠加。原地换集只改 <video> 的
// src，进程、页面、解码器初始化都留着，切集的内存曲线是平的。
//
// 返回 error 表示「这一下没换成」（窗口已死 / 页面还没就绪）：调用方据此回落
// 到新建路径，而不是静默丢掉这一集。
func (m *Manager) switchTo(w *mygo.Window, o Options) error {
	p := w.Page()
	if p == nil {
		return fmt.Errorf("video: 弹窗页面已消失")
	}
	if _, err := p.Eval(switchScript(o)); err != nil {
		return err
	}
	w.SetTitle(o.Title)
	if w.IsMinimized() {
		w.Restore()
	}
	w.Show()
	w.Focus()
	m.mu.Lock()
	m.st = State{Speed: o.Speed}
	m.ended = false
	m.mu.Unlock()
	return nil
}

// Close 关闭弹窗（若开着）。两条路径都收：原生窗与 webview 互斥，但
// 回退过程中可能短暂同时存在（原生刚关、webview 正在建），所以都清。
func (m *Manager) Close() {
	m.mu.Lock()
	w, n := m.win, m.native
	m.win, m.native, m.minimize = nil, nil, nil
	m.ended = false
	m.mu.Unlock()
	if n != nil {
		n.close()
	}
	if w != nil {
		w.Close()
	}
}

// SetTheme 把主窗的新主题令牌推给视频窗（App.OnThemeChange 的唯一接收方）。
//
// 视频窗是独立窗口，不跑主窗的 Shell，读不到 App.Theme；从前的实现只在
// 建窗时拷一份快照，主窗切浅色 / 深色 / 按时间 / 跟随系统（含跟随系统下
// 桌面外观变化）、高对比、减少透明度之后，视频窗仍停在开窗那一刻的深浅上，
// 必须关掉重开才生效（用户 2026-11 报）。
//
// 这里只换外壳的主题令牌，**不重建窗口**：播放器实例、播放位置、弹幕、
// 全屏与置顶都不动，下一帧标题栏 / 控件条 / 设置面板 / 弹幕区底色就按新
// 令牌重画。主题也不进画面区：ui/video 的画面不读任何令牌。
func (m *Manager) SetTheme(t view.Theme) {
	m.mu.Lock()
	m.theme, m.hasTheme = t, true
	n := m.native
	m.mu.Unlock()
	if n == nil {
		// 窗口还没建：值已记在 m.theme 上，tryNative 建窗时会套给外壳。
		return
	}
	if n.win == nil {
		// 无头测试 / 窗口还没挂上（与 Invalidate 同一判据）：直接改，
		// 下一次 n.view 就按新令牌画。
		n.setTheme(t)
		return
	}
	n.win.Update(func() { n.setTheme(t) })
}

// Invalidate 让原生视频窗重画一帧。主窗改了共享设置（音量 / 视频窗里
// 只有 EQ / 广告跳过这类没有引擎可落的项）时，视频窗可能正暂停、没有自己的
// 动画帧，需要显式叫它重画才能立刻看到新状态。
func (m *Manager) Invalidate() {
	m.mu.Lock()
	n := m.native
	m.mu.Unlock()
	if n != nil && n.win != nil {
		n.win.Update(func() {})
	}
}

// SetSpeed 同步倍速到弹窗（原生 / webview 两条路径都支持：原生走
// VideoPlayer.SetRate，webview 走 playbackRate）。
func (m *Manager) SetSpeed(v float64) {
	m.mu.Lock()
	w, n := m.win, m.native
	m.mu.Unlock()
	if n != nil {
		n.setRate(v)
		return
	}
	if w != nil {
		w.Page().Eval(fmt.Sprintf("(()=>{const v=document.getElementById('v');if(v)v.playbackRate=%f;})()", v))
	}
}

// Pause / Resume 供主窗控制弹窗（例如主窗按了空格）。
func (m *Manager) Pause() {
	m.mu.Lock()
	n := m.native
	m.mu.Unlock()
	if n != nil {
		n.pause()
		return
	}
	m.eval("document.getElementById('v')?.pause()")
}

func (m *Manager) Resume() {
	m.mu.Lock()
	n := m.native
	m.mu.Unlock()
	if n != nil {
		n.play()
		return
	}
	m.eval("document.getElementById('v')?.play()")
}

// Seek 让弹窗跳到指定秒数。主窗底部进度条在视频弹窗打开时也要控制视频，
// 而不是去 seek 已经暂停的音频引擎。
func (m *Manager) Seek(seconds float64) {
	if seconds < 0 {
		seconds = 0
	}
	m.mu.Lock()
	n := m.native
	m.mu.Unlock()
	if n != nil {
		n.seek(seconds)
		return
	}
	m.eval(fmt.Sprintf("(()=>{const v=document.getElementById('v');if(v){v.currentTime=%f;}})()", seconds))
}

// SetMuted 同步静音（主窗静音时原生视频也要静音；webview 页面自己管）。
func (m *Manager) SetMuted(muted bool) {
	m.mu.Lock()
	n := m.native
	m.mu.Unlock()
	if n != nil {
		n.setMuted(muted)
	}
}

func (m *Manager) eval(js string) {
	m.mu.Lock()
	w := m.win
	m.mu.Unlock()
	if w != nil {
		w.Page().Eval(js)
	}
}

// ---------------------------------------------------------------- 页面 → Go

// Service 是暴露给弹窗页面的服务。
type Service struct{ m *Manager }

// NewService 建一个页面服务。
func NewService(m *Manager) *Service { return &Service{m: m} }

// Log 页面写日志。
func (s *Service) Log(msg string) { videoLog("弹窗 | " + msg) }

// Report 页面每秒回传状态。
func (s *Service) Report(t, dur float64, paused, muted bool, speed float64, ready int, err string) {
	s.m.mu.Lock()
	s.m.st = State{Time: t, Duration: dur, Paused: paused, Muted: muted, Speed: speed, Ready: ready, Err: err}
	cb := s.m.onState
	s.m.mu.Unlock()
	if cb != nil {
		cb(s.m.st)
	}
}

// Minimize 最小化视频弹窗（弹窗里按 Ctrl/Cmd+W）。最小化的是弹窗自身 ——
// 它是当前获得键盘焦点的窗口，与主窗 Ctrl/Cmd+W（最小化主窗）的语义一致，
// 都是「最小化当前窗口」。弹窗最小化后视频仍在播放（音频不停），可从
// 任务栏 / Dock 还原；不开窗时静默忽略。
func (s *Service) Minimize() {
	s.m.mu.Lock()
	fn := s.m.minimize
	s.m.mu.Unlock()
	if fn != nil {
		fn()
	}
}

// Quit 退出整个应用（弹窗里按 Ctrl/Cmd+Q）。原版 index.tsx 的快捷键
// listener 在 `isPlayVideo` 分支之前执行，所以视频模式下 Ctrl/Cmd+Q 同样
// 生效；原生版的弹窗是独立窗口，主窗快捷键到不了这里，必须由弹窗页面接。
func (s *Service) Quit() {
	s.m.mu.Lock()
	cb := s.m.onQuit
	s.m.mu.Unlock()
	if cb != nil {
		go cb()
	}
}

// Close 页面请求关闭弹窗。
func (s *Service) Close() {
	go func() {
		time.Sleep(20 * time.Millisecond) // 让这次调用先返回
		s.m.Close()
	}()
}

// Ended 页面报告播放结束。
func (s *Service) Ended() {
	s.m.mu.Lock()
	s.m.ended = true
	cb := s.m.onEnded
	s.m.mu.Unlock()
	if cb != nil {
		go cb()
	}
}

var videoLog = func(string) {}

// SetLog 注入日志函数（避免这个包依赖调用方的日志实现）。
func SetLog(fn func(string)) { videoLog = fn }

func max(a, b float64) float64 {
	if a > b {
		return a
	}
	return b
}
