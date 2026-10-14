package video

import (
	"os"
	"strings"
	"testing"
)

// TestPageURL 钉住弹窗页面地址里的起播参数：位置 / 倍速 / 静音。这些值决定
// 弹窗打开时的播放状态，改错一个就会「打开是暂停 / 位置不对 / 没声音」。
func TestPageURL(t *testing.T) {
	got := pageURL(Options{
		Src:     "http://127.0.0.1:4654/a?u=x",
		Title:   "标题",
		Poster:  "http://127.0.0.1:4654/p.png",
		StartAt: 12.5,
		Speed:   1.25,
		Muted:   true,
	})
	for _, want := range []string{
		"/video.html?",
		"t=12.50",
		"speed=1.25",
		"muted=1",
	} {
		if !strings.Contains(got, want) {
			t.Errorf("pageURL 缺少 %q：%s", want, got)
		}
	}

	// 倍速下限 0.5（0 / 负值不能原样传进去，否则页面 playbackRate=0 会卡住）。
	low := pageURL(Options{Speed: 0})
	if !strings.Contains(low, "speed=0.50") {
		t.Errorf("倍速未夹到下限：%s", low)
	}
	// 不静音时不带 muted 参数（页面 muted=false）。
	unmuted := pageURL(Options{Speed: 1})
	if strings.Contains(unmuted, "muted=1") {
		t.Errorf("未静音却带了 muted=1：%s", unmuted)
	}
}

// TestVideoPageAutoplayContract 钉住弹窗页面的起播策略（用户反馈「弹窗打开是
// 暂停状态」的回归点）：
//
//  1. 被策略拦下要退回**静音**起播（muted autoplay 各引擎都放行）；
//  2. **不能**在起播后自动取消静音——WebKit/Chromium 会把无手势的取消静音
//     判定为违规并直接暂停媒体，这正是旧实现停住的原因；
//  3. 改为在弹窗里第一次真实手势时再取消静音。
func TestVideoPageAutoplayContract(t *testing.T) {
	data, err := os.ReadFile("../../frontend/video.html")
	if err != nil {
		t.Fatalf("读不到 video.html：%v", err)
	}
	html := string(data)

	if !strings.Contains(html, "v.muted = true;") {
		t.Error("缺少静音起播的兜底路径（v.muted = true 后 play）")
	}
	if strings.Contains(html, "setTimeout(() => { v.muted = false; }, 900)") {
		t.Error("仍有起播后定时取消静音：无手势取消静音会被引擎暂停媒体")
	}
	if !strings.Contains(html, "addEventListener('pointerdown', onGesture") {
		t.Error("缺少手势取消静音（pointerdown）")
	}
	if !strings.Contains(html, "v.muted = false;") {
		t.Error("手势路径里没有取消静音")
	}
}

// TestVideoPageCloseButtonMatchesOriginal 钉住弹窗关闭键的样式契约。原版浮层
// 的 .player-video-close（main:src/styles/globals.css）是 38×38 正圆、墨色
// 45% 底，悬停转 70% 并放大 1.06、按下缩 .92；图标是 icon-park 的 Close
// （playerVideo.tsx 的 `<Close theme="outline" size="20" fill="#f1f5f9"/>`）。
// 原生版此前是一枚 26×22 圆角矩形、还塞在悬停才出现的 #bar 里，鼠标一移开
// 就消失。
func TestVideoPageCloseButtonMatchesOriginal(t *testing.T) {
	data, err := os.ReadFile("../../frontend/video.html")
	if err != nil {
		t.Fatalf("读不到 video.html：%v", err)
	}
	html := string(data)

	for _, want := range []string{
		"width: 38px; height: 38px",             // 原版 38×38
		"border-radius: 50%",                    // 正圆
		"background: rgba(15,23,42,.45)",        // 常态墨色 45%
		"background: rgba(15,23,42,.7)",         // 悬停墨色 70%
		"transform: scale(1.06)",                // 悬停放大
		"#close:active { transform: scale(.92)", // 按下缩 .92
		"top: 44px; right: 10px",                // 原版 body.video-open 的 36+8
		`d="M8 8L40 40"`,                        // icon-park Close 两条 path
		`d="M8 40L40 8"`,
		`stroke="#f1f5f9"`,       // 图标色
		`width="20" height="20"`, // 原版 size=20
		`stroke-width="4"`,       // icon-park 48 画布原始描边
	} {
		if !strings.Contains(html, want) {
			t.Errorf("关闭键缺少 %q（原版 .player-video-close）", want)
		}
	}

	// 关闭键必须常驻：不能还留在 #bar 里（那条默认隐去，只在顶部热区/hover 时显形）。
	bar := strings.Index(html, `<div id="bar">`)
	if bar < 0 {
		t.Fatal("找不到 #bar")
	}
	end := strings.Index(html[bar:], "</div>")
	if end < 0 {
		t.Fatal("#bar 未闭合")
	}
	if strings.Contains(html[bar:bar+end], `id="close"`) {
		t.Error("关闭键仍在 #bar 里，会随标题条一起隐去")
	}
}

// TestVideoPageCloseTitle 钉住弹窗上关闭键的工具提示：原版视频浮层
// （playerVideo.tsx 的 .player-video-close）是 title="关闭视频"，原生版此前
// 写的是「关闭」——悬停时看不出关的是视频还是整个播放。
func TestVideoPageCloseTitle(t *testing.T) {
	data, err := os.ReadFile("../../frontend/video.html")
	if err != nil {
		t.Fatalf("读不到 video.html：%v", err)
	}
	html := string(data)
	if !strings.Contains(html, `id="close" title="关闭视频"`) {
		t.Error(`关闭键应带 title="关闭视频"（原版 playerVideo 的写法）`)
	}
	if strings.Contains(html, `id="close" title="关闭"`) {
		t.Error("关闭键仍是「关闭」（漏改？）")
	}
}

// TestVideoPageVideoElementMatchesOriginal 钉住弹窗 <video> 的呈现契约。
//
// 原版浮层的 .player-video-element（main:src/styles/globals.css:5457）是：
//
//	width:100%; height:100%; object-fit:contain; border-radius:8px; background:#000
//
// `object-fit: contain` 是画面比例的保证：弹窗是固定 960×600（8:5）的独立窗口，
// 而绝大多数 B 站视频是 16:9；没有这一条时 `<video>` 走默认的 `fill`，会把画面
// 拉伸到窗口比例（16:9 → 8:5 ⇒ 横向拉宽约 11%）。此前 video.html 只写了
// width/height/background，正好漏掉这条。
//
// 同时原版按平台传 disablePictureInPicture={nativePipUnusable}，其中
// nativePipUnusable={isMacOS}（index.tsx）：macOS 的系统画中画浮窗没有进度条，
// 点进去调不了进度，所以那边隐藏该按钮。原生版的视频本来就是一个常驻独立窗口，
// 画中画也多余。
func TestVideoPageVideoElementMatchesOriginal(t *testing.T) {
	html := videoPageHTML(t)

	rule := cssRule(t, html, "video")
	for _, want := range []string{
		"width: 100%",
		"height: 100%",
		"background: #000",
		"object-fit: contain",
	} {
		if !strings.Contains(rule, want) {
			t.Errorf("<video> 规则缺少 %q（原版 .player-video-element）", want)
		}
	}
	// 反证：默认的 fill 会拉伸画面。剔注释后不该出现这条覆盖。
	if code := videoPageCode(t); strings.Contains(code, "object-fit: fill") {
		t.Error("<video> 显式写了 object-fit: fill（会把画面拉伸到窗口比例）")
	}

	// macOS 隐藏画中画按钮（原版 nativePipUnusable = isMacOS）。
	code := videoPageCode(t)
	if !strings.Contains(code, "v.disablePictureInPicture = true") {
		t.Error("缺少 macOS 隐藏画中画按钮（原版 playerVideo 的 nativePipUnusable）")
	}
	if !strings.Contains(code, "if (isMac) { v.disablePictureInPicture = true; }") {
		t.Error("画中画禁用应挂在 macOS 判定上（原版 isMacOS）")
	}
}

// cssRule 从页面里取出某条 CSS 规则的声明块（`selector { … }`）。选择器要
// 逐字给出（含 `#bar::before` 这种伪元素）。找不到就 t.Fatal。
func cssRule(t *testing.T, html, selector string) string {
	t.Helper()
	start := strings.Index(html, selector+" {")
	if start < 0 {
		t.Fatalf("找不到 CSS 规则 %q", selector)
	}
	start += len(selector) + 2
	end := strings.Index(html[start:], "}")
	if end < 0 {
		t.Fatalf("CSS 规则 %q 未闭合", selector)
	}
	return html[start : start+end]
}

// videoPageHTML 读弹窗页面源码。
func videoPageHTML(t *testing.T) string {
	t.Helper()
	data, err := os.ReadFile("../../frontend/video.html")
	if err != nil {
		t.Fatalf("读不到 video.html：%v", err)
	}
	return string(data)
}

// videoPageCode 同 videoPageHTML，但剔掉 CSS / HTML 注释——用于「旧写法
// 已不存在」这类反证：注释里引用旧写法的说明文字不该算数。
func videoPageCode(t *testing.T) string {
	t.Helper()
	s := videoPageHTML(t)
	for {
		i := strings.Index(s, "/*")
		if i < 0 {
			break
		}
		j := strings.Index(s[i:], "*/")
		if j < 0 {
			break
		}
		s = s[:i] + s[i+j+2:]
	}
	for {
		i := strings.Index(s, "<!--")
		if i < 0 {
			break
		}
		j := strings.Index(s[i:], "-->")
		if j < 0 {
			break
		}
		s = s[:i] + s[i+j+3:]
	}
	return s
}

// TestVideoPageTitleBarMatchesOriginal 钉住弹窗顶部拖拽条的显隐契约。
//
// 原版视频浮层的标题栏（main:src/styles/globals.css 的
// `body.video-open .app-title-bar`，唤出热区在 playerVideo.tsx 的
// `syncTitlebarHotzone` / TITLEBAR_HOTZONE_PX = 48）是：
//
//	静息   opacity:0; pointer-events:none; transition: opacity .18s ease .2s
//	唤出   opacity:1; pointer-events:auto;  transition: opacity .16s ease
//
// 原生版此前写 `#wrap:hover #bar, #bar.pinned`：`#wrap` 铺满整窗，鼠标只要在
// 窗口里拖拽条就常驻（原版只在顶部 48px 热区里出现），而且 opacity:0 时它照吃
// 顶部那条的指针事件（原版隐去时 pointer-events:none 不挡画面/原生控件）。
func TestVideoPageTitleBarMatchesOriginal(t *testing.T) {
	html := videoPageHTML(t)

	// 静息态：#bar 自己不接管指针，退场带延迟。
	base := cssRule(t, html, "#bar")
	for _, want := range []string{
		"opacity: 0",
		"pointer-events: none",
		"transition: opacity .18s ease .2s",
		"height: 36px", // 原版 --titlebar-height
	} {
		if !strings.Contains(base, want) {
			t.Errorf("#bar 静息规则缺少 %q（原版 body.video-open .app-title-bar）", want)
		}
	}

	// 唤出态：热区悬浮（JS 挂 body.titlebar-hover）时恢复绘制与命中。
	reveal := cssRule(t, html, "body.titlebar-hover #bar, #bar:hover")
	for _, want := range []string{
		"opacity: 1",
		"pointer-events: auto",
		"transition: opacity .16s ease",
	} {
		if !strings.Contains(reveal, want) {
			t.Errorf("#bar 唤出规则缺少 %q", want)
		}
	}

	// 压暗渐变层：原版 ::before 的 inset 0 0 -40% 0 + 三个停靠点。
	shade := cssRule(t, html, "#bar::before")
	for _, want := range []string{
		"inset: 0 0 -40% 0",
		"rgba(8,12,20,.72) 0%",
		"rgba(8,12,20,.42) 55%",
		"transparent 100%",
	} {
		if !strings.Contains(shade, want) {
			t.Errorf("#bar::before 压暗层缺少 %q", want)
		}
	}

	// 唤出由顶部 48px 热区驱动（原版 playerVideo.tsx），不是整窗 hover。
	code := videoPageCode(t)
	for _, want := range []string{
		"const TITLEBAR_HOTZONE_PX = 48;",
		"const MACOS_HOTZONE_VOLUME_SAFE_PX = 260;",
		"const MACOS_HOTZONE_PIP_SAFE_PX = 120;",
		"clientY <= TITLEBAR_HOTZONE_PX",
		"classList.toggle('titlebar-hover'",
		"addEventListener('mousemove'",
		"addEventListener('mouseleave'",
	} {
		if !strings.Contains(code, want) {
			t.Errorf("热区逻辑缺少 %q", want)
		}
	}

	// 反证：整窗 hover 与那个没人置位的 .pinned 都该消失（注释里的旧写法不算数）。
	if strings.Contains(code, "#wrap:hover #bar") {
		t.Error("拖拽条仍靠 #wrap:hover 唤出（#wrap 铺满整窗 ⇒ 鼠标在窗口里就常驻）")
	}
	if strings.Contains(code, "#bar.pinned") {
		t.Error("仍在用没人置位的 #bar.pinned")
	}
	if strings.Contains(code, "opacity: 0; transition: opacity .2s ease") {
		t.Error("静息规则仍是旧的 opacity .2s 过渡")
	}
}

// TestVideoPageStatePillHiddenByDefault 钉住弹窗左下角状态胶囊的显隐契约。
//
// 原版视频浮层（playerVideo.tsx）没有这一层：出错时由浮层自身/主窗处理。
// 原生版的 #state 是「心跳里回传 lastErr」的排障输出，但它此前默认可见、初始
// 文案「准备中…」，于是弹窗打开后第一次心跳（1s）之前会在画面左下角闪一下
// 「准备中…」。改为默认 display:none，只在出错时显形：error 监听里立刻打开
// （否则要等最多 1s 心跳才看得见），每秒心跳兜底。
func TestVideoPageStatePillHiddenByDefault(t *testing.T) {
	html := videoPageHTML(t)

	if rule := cssRule(t, html, "#state"); !strings.Contains(rule, "display: none") {
		t.Error("#state 默认未隐藏：弹窗首帧会闪「准备中…」")
	}
	// 占位文案检查用剔注释源码：注释里对旧写法的说明不该算数。
	code := videoPageCode(t)
	if strings.Contains(code, "准备中…") {
		t.Error("仍有初始文案「准备中…」（默认隐藏也不该留占位文案）")
	}

	// error 监听里必须自己打开显示：否则出错要等到下一次心跳才显形。
	const errHandler = "addEventListener('error'"
	i := strings.Index(html, errHandler)
	if i < 0 {
		t.Fatal("找不到 error 监听")
	}
	blk := html[i:]
	if j := strings.Index(blk, "});"); j >= 0 {
		blk = blk[:j]
	}
	if !strings.Contains(blk, "state.style.display = 'block'") {
		t.Error("出错时未立刻显示状态胶囊（只在心跳里设，最多迟 1s）")
	}

	// 心跳兜底：有错显示、无错隐藏。
	if !strings.Contains(code, "state.style.display = 'none'") {
		t.Error("心跳里没有把无错的状态胶囊收起来")
	}
}

// TestVideoPagePanelEnterAnimationMatchesOriginal 钉住弹窗的入场动画。
//
// 原版浮层的 .player-video-panel（globals.css:5440）入场是：
//
//	静息   opacity: 0; transform: scale(1.02);
//	       transition: transform .28s cubic-bezier(.22,1,.36,1), opacity .28s ease
//	展开   transform: scale(1); opacity: 1
//
// 由 playerVideo.tsx 的双重 requestAnimationFrame 挂 .player-video-open 触发。
// 原生版此前 #wrap 没有任何过渡，弹窗是硬切出现。
func TestVideoPagePanelEnterAnimationMatchesOriginal(t *testing.T) {
	html := videoPageHTML(t)

	base := cssRule(t, html, "#wrap")
	for _, want := range []string{
		"opacity: 0",
		"transform: scale(1.02)",
		"transition: transform 0.28s cubic-bezier(0.22, 1, 0.36, 1), opacity 0.28s ease",
	} {
		if !strings.Contains(base, want) {
			t.Errorf("#wrap 静息规则缺少 %q（原版 .player-video-panel）", want)
		}
	}

	open := cssRule(t, html, "#wrap.open")
	for _, want := range []string{"opacity: 1", "transform: scale(1)"} {
		if !strings.Contains(open, want) {
			t.Errorf("#wrap.open 缺少 %q（原版 .player-video-open）", want)
		}
	}

	// 双重 rAF 后挂类：只挂一次 rAF 会与首帧样式合并成「无动画」的一次计算。
	code := videoPageCode(t)
	if !strings.Contains(code, "requestAnimationFrame(() => requestAnimationFrame(") {
		t.Error("挂 .open 未用双重 rAF（原版做法）：transition 可能不生效")
	}
	if !strings.Contains(code, "classList.add('open')") {
		t.Error("加载后没有给 #wrap 挂 .open，入场动画不会触发")
	}
}

// TestVideoPageKeyboardShortcutsMatchOriginal 钉住弹窗的「视频模式快捷键」契约。
//
// 原版是单 webview，视频浮层打开时（index.tsx 的 handleKeyPress，isPlayVideo
// 分支）：
//
//	Esc   关闭视频浮层（setIsPlayVideo(false)）
//	空格  播放 / 暂停视频（setIsPlayVideoStop）
//	其余  直接 return（「播放视频时屏蔽快捷键」）
//
// 两颗键都要求 !event.repeat（按住不放不连发）。
//
// 原生版的视频是**独立窗口**：主窗那套快捷键在弹窗聚焦时根本到不了，而弹窗页
// 此前没有任何键盘处理 —— 于是聚焦弹窗按 Esc 关不掉、按空格也不一定切得了播放
// （只有 <video> 恰好有焦点时由原生控件接管）。
func TestVideoPageKeyboardShortcutsMatchOriginal(t *testing.T) {
	code := videoPageCode(t)

	// 必须有 keydown 监听（弹窗是独立窗口，靠页面自己收键）。
	const handlerStart = "document.addEventListener('keydown', (e) => {"
	i := strings.Index(code, handlerStart)
	if i < 0 {
		t.Fatal("弹窗没有注册 keydown 监听，Esc / 空格收不到")
	}
	// 整个处理器块：从监听注册到它的收尾 `\n});`。
	body := code[i+len(handlerStart):]
	if j := strings.Index(body, "\n});"); j >= 0 {
		body = body[:j]
	}

	// 去抖：原版两颗键都带 !event.repeat。
	if !strings.Contains(body, "if (e.repeat) return;") {
		t.Error("键盘处理缺少 repeat 去抖（原版 !event.repeat）")
	}

	// 拆出 Esc / 空格两条分支（`} else if (…)` 分隔）。
	esc := strings.Index(body, "e.code === 'Escape'")
	space := strings.Index(body, "e.code === 'Space'")
	if esc < 0 {
		t.Fatal("缺少 Esc 分支（原版 isPlayVideo 下 Esc 关浮层）")
	}
	if space < 0 {
		t.Fatal("缺少空格分支（原版 isPlayVideo 下空格切视频播放 / 暂停）")
	}
	if space < esc {
		t.Fatal("Esc / 空格分支顺序异常")
	}
	escBlk, spaceBlk := body[esc:space], body[space:]

	// Esc 关弹窗：走页面的 Video.Close（与关闭键同一条路径）。
	if !strings.Contains(escBlk, "call('Video.Close')") {
		t.Error("Esc 分支没有调用 Video.Close（关不掉弹窗）")
	}
	// 空格切播放 / 暂停，并 preventDefault（别把事件再交给原生控件二次切换）。
	for _, want := range []string{"e.preventDefault()", "v.paused", "v.play()", "v.pause()"} {
		if !strings.Contains(spaceBlk, want) {
			t.Errorf("空格分支缺少 %q（要真的切播放 / 暂停，不是把事件吞掉）", want)
		}
	}
}
