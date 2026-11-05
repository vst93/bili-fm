package view

import (
	"math"
	"sort"

	"github.com/egoist/mygo/plugins/glass"
	"github.com/egoist/mygo/ui"
)

// ---------------------------------------------------------------- 液态玻璃材质（原生版）
//
// 原版 Web 的材质是四条 CSS 声明：
//
//	background: <tint>;
//	backdrop-filter: blur(2px) saturate(180%) brightness(1.06) url(#rim);
//	box-shadow: <edge>;
//	color: <ink>;
//
// 原生版没有 CSS/DOM，也没有 @property / SVG feDisplacementMap / 三通道
// mix-blend-mode。对应关系：
//
//	background + backdrop-filter  →  glass.Effect 的 tint + 色调参数（本文件）
//	url(#rim) 位移贴图           →  glass.Effect 的 bezel/refraction（shader 里
//	                               按圆角法线把边缘的 backdrop 折进来；同一套
//	                               Metal/HLSL/GLSL/CPU 实现，跨平台一致）
//	box-shadow / color           →  各表面原有的 Border/InsetShadow/Shadow 与
//	                               墨色令牌（保持设计系统，不在这里重画）
//
// 颜色仍然从背后取：tint 只叠 6–22%（亮）或 16–22%（烟熏；产品的壁纸是
// 亮色，上限按最暗时段仍过 4.5:1 收到 22%），色调曲线用
// `low=0, high=brightness, curve=1` 精确复刻 CSS 的 brightness()×saturate()：
//
//	t = b·l;  c' = t + (c-l)·b·sat = b·(l + sat·(c-l))
//
// 三通道色散在原生没有可用的加法混合通道，保留产品已有的 `--liquid-chromatic`
// 0.5px 暖/冷边（paintPlayerChromatic）作等价物；不做三遍位移。
//
// 所有常量/颜色都按状态放在 Theme（见 glassState），表面只选状态。

// glassOptions 是一块表面的材质选项。表面只声明「是哪一档」，不写色值。
type glassOptions struct {
	// dark 是烟熏表面（只有播放栏在五个深色时段用）。墨色仍按背景亮度选，
	// 不按这个布尔值选 —— 产品的「深色时段」壁纸其实仍是中亮灰蓝。
	dark bool
	// blur 是 backdrop 的高斯标准差（DIP）。产品既有材质是 14–20；原版
	// Web 的 2px 是起点值，直接搬过来会让抽屉/弹层上的正文失去可读性。
	blur float32
	// lens 打开边缘折射（bezel）。菜单/弹层/toast 这类会做 transform 动画
	// 的合成层按原版建议跳过，避免 Chrome 式「画 map 而不是弯 backdrop」；
	// 原生这里由表面显式选择。
	lens bool
	// sheen 是原有的 `--glass-sheen-static` 内层洗光（左上径向 + 斜向）。
	// 它就是材质里的「inner light」那一层，继续复用，不再叠第二层。
	sheen *sheenBaked
	// inner 在没有 sheen 的大面板（抽屉）上补一层左上径向高光。
	inner bool
	// denseFloor 抬密度下限：贴壁纸的大面板/正文表面需要更高的 tint 才读得清。
	denseFloor float32
	// reading 是「正文阅读面」：抽屉里全是产品级深墨文字，暗主题下如果整块
	// 也换烟熏，正文会失去对比；这类面保持亮玻璃，只让浮动玻璃层（搜索、
	// 播放栏、菜单、toast、弹层）跟着主题变。
	reading bool
	// edge 由材质自己画厚玻璃的边（左上亮、右下淡 + 0.5px 暗轮廓）。
	// 只给原本没有边框/聚光描边的 hero 表面（搜索、播放栏、抽屉），
	// 菜单/toast/弹层沿用各自的 Border/InsetShadow，避免双层描边。
	edge bool
}

// liquidGlass 是一块玻璃表面：材质参数来自 Theme 的 glassState，每个表面
// 只带自己的 blur/lens/密度下限。
type liquidGlassMaterial struct {
	t Theme
	o glassOptions
}

// liquidGlassLayer 记一层 backdrop（backdropcount.go）并给出材质。
func liquidGlassLayer(t Theme, o glassOptions) liquidGlassMaterial {
	if !t.ReducedTransparency {
		backdropLayers.Add(1)
	}
	return liquidGlassMaterial{t: t, o: o}
}

// PaintMaterial 按 CSS 的绘制顺序铺：先模糊/折射/染色（backdrop-filter 的
// 等价物），再铺内层洗光。边框、描光、外投影仍由各表面自己的 Border/
// InsetShadow/Shadow/Draw 声明，材质不重复画。
func (g liquidGlassMaterial) PaintMaterial(p *ui.Painter, box ui.Rect, radii [4]float32) {
	if box.W <= 0 || box.H <= 0 {
		return
	}
	// 减少透明度：换成同色调的不透明面。mygo 的 ui.Preferences 没有
	// forced-colors 信号，桌面强制颜色时由同一分支以外的高对比令牌兜底。
	// 深色烟熏播放栏由 playerBarMaterial 先走自己的实底分支。
	if g.t.ReducedTransparency {
		p.Fill(box, g.t.GlassSolid, radii[0])
		return
	}

	dark := g.o.dark
	if !g.t.classicGlass() {
		// 表面主题是唯一事实源：明主题强制亮状态，暗主题强制烟熏状态。
		dark = g.t.glassIsDark() && !g.o.reading
	}
	st := g.t.glassStateFor(dark)
	scale := g.t.glassTintScale(dark)

	// 密度：背景越暗/越花/越艳，tint 越向 Dense 端走。表面可抬下限。
	density := min(max(g.t.GlassDensity, g.o.denseFloor), 1)
	alpha := (st.Thin + (st.Dense-st.Thin)*density) * scale
	alpha = min(max(alpha, 0), 1)
	tint := st.Tint.Alpha(alpha)

	// brightness()/saturate()：scale=0 时两者都回到 1，只剩模糊、折射与阴影。
	brightness := 1 + (st.Brightness-1)*scale
	saturation := 1 + (st.Saturation-1)*scale

	// 色散边没有三通道混合通道，tint 与色调都是一次采样。
	params := [5][4]float32{
		{0, 0, 0, 0}, // bezel/refraction/rim/rimWidth，下面填
		{float32(tint.R) / 255, float32(tint.G) / 255, float32(tint.B) / 255, float32(tint.A) / 255},
		{0, brightness, 1, saturation},
		{0, 1}, // 光从上方来；rim 为 0 时不影响
		{float32(tint.R) / 255, float32(tint.G) / 255, float32(tint.B) / 255, float32(tint.A) / 255},
	}

	blur := g.o.blur
	if blur <= 0 {
		blur = 2
	}

	// rim 透镜：卡片的 rim 宽约 44px（短边/3.5，最小 4，且不超过短边一半）。
	// 小表面（<32px）不做；玻璃 shader 的 ⁴√(1-(1-x)⁴) 是原生版对
	// 1.1·rim·(1-d/rim)^2.6 的等价轮廓，最大偏移 1.1·rim。
	if g.o.lens {
		short := min(box.W, box.H)
		if short >= 32 {
			rim := min(44, short/3.5)
			rim = max(rim, 4)
			rim = min(rim, short/2)
			s := p.Scale()
			params[0][0] = rim * s
			params[0][1] = 1.1 * rim * s
		}
	}

	p.Effect(glass.Effect, box, radii, blur*p.Scale(), params)

	// 内层光：沿用原有 sheen；暗主题换成更弱的烟熏 sheen（约三分之一强度）。
	sheen := g.o.sheen
	if sheen != nil && g.t.GlassTheme == GlassDark {
		sheen = sheenDarkBaked
	}
	if sheen != nil {
		paintBakedSheen(p, box, radii[0], sheen)
	} else if g.o.inner {
		paintGlassInnerLight(p, box, radii[0])
	}
	if g.o.edge {
		paintGlassEdge(p, box, radii[0])
	}
}

// paintGlassEdge 画材质自己的「厚板边缘」：左上 1.5px 亮、右下淡，外加一圈
// 0.5px 暗轮廓。它替代经典主题的 conic 聚光描边（hero 表面在新主题下不再
// 调 paintPlayerRim），这样即使背后是平滑渐变，边缘也读得出玻璃的厚度。
func paintGlassEdge(p *ui.Painter, box ui.Rect, radius float32) {
	if box.W < 2 || box.H < 2 {
		return
	}
	const w = 1.5
	white := ui.Hex("#ffffff")
	// 暗轮廓在亮边下面，先把形状收住。
	p.Stroke(ui.Rect{X: box.X + 0.25, Y: box.Y + 0.25, W: box.W - 0.5, H: box.H - 0.5},
		ui.Hex("#000000").Alpha(0.06), max(radius-0.25, 0), 0.5)
	p.Clip(box, radius, func() {
		// 上：左亮右淡；左：上亮下淡。
		p.FillGradient(ui.Rect{X: box.X, Y: box.Y, W: box.W, H: w},
			ui.LinearGradient{From: white.Alpha(0.95), To: white.Alpha(0.35), Angle: 90}, 0)
		p.FillGradient(ui.Rect{X: box.X, Y: box.Y, W: w, H: box.H},
			ui.LinearGradient{From: white.Alpha(0.95), To: white.Alpha(0.28), Angle: 180}, 0)
		// 下 / 右：更淡的一圈。
		p.Fill(ui.Rect{X: box.X, Y: box.Y + box.H - w, W: box.W, H: w}, white.Alpha(0.30), 0)
		p.Fill(ui.Rect{X: box.X + box.W - w, Y: box.Y, W: w, H: box.H}, white.Alpha(0.38), 0)
	})
}

// paintGlassInnerLight 是材质内层「从左上来的一团光」：白 24% 到 55% 处
// 归零，只提亮左上，不把整块面板洗白。
func paintGlassInnerLight(p *ui.Painter, box ui.Rect, radius float32) {
	if box.W <= 0 || box.H <= 0 {
		return
	}
	p.FillRadialGradient(box, ui.RadialGradient{
		Center: ui.Point{X: 0.18, Y: -0.05},
		Radius: max(box.W, box.H) * 0.85,
		Stops: []ui.GradientStop{
			{Color: ui.Hex("#ffffff").Alpha(0.24), At: 0},
			{Color: ui.Transparent, At: 0.55},
		},
	}, radius)
}

// glassStats 是时段壁纸的统计量：去极值后的平均亮度、纹理与色度。
type glassStats struct {
	density        float32
	lum            float32
	texture        float32
	chroma         float32
	saturationKept float32
}

// glassStatsFor 在时段渐变上按 5×3 采样，去掉最暗/最亮各两点后求：
//
//	lum      = 平均亮度 (0.299R + 0.587G + 0.114B) / 255
//	texture  = 剩余点的最亮 - 最暗
//	chroma   = 每个点 (max(R,G,B) - min(R,G,B)) / 255 的平均
//	density  = 0.55 × 背景离墨色亮度的距离 + 0.7 × texture + 0.3 × chroma
//	satKept  = 1 - 0.75 × chroma，最低 0.45
//
// 产品的背景是一条已知的 165° 时段渐变，所以这里采样它，而不是每帧回读
// backdrop 像素；同一时段的所有表面共享这一份判断。真实图片/视频背后的表面
// （视频弹窗不是玻璃）在原生里没有可读的跨源像素，按原版规则退回主题值。
func glassStatsFor(p Period, ink ui.Color) glassStats {
	const cols, rows = 5, 3
	lums := make([]float32, 0, cols*rows)
	chromaSum := float32(0)
	for j := 0; j < rows; j++ {
		for i := 0; i < cols; i++ {
			t := float32(i) / float32(cols-1)
			c := p.Sample(t)
			r, g, b := float32(c.R)/255, float32(c.G)/255, float32(c.B)/255
			lums = append(lums, 0.299*r+0.587*g+0.114*b)
			chromaSum += (maxF(r, maxF(g, b)) - minF(r, minF(g, b)))
		}
	}
	sort.Slice(lums, func(i, j int) bool { return lums[i] < lums[j] })
	// 去掉最暗/最亮各两点（15 点剩 11 点）。
	mid := lums[2 : len(lums)-2]
	sum := float32(0)
	lo, hi := mid[0], mid[0]
	for _, l := range mid {
		sum += l
		lo = minF(lo, l)
		hi = maxF(hi, l)
	}
	lum := sum / float32(len(mid))
	texture := hi - lo
	chroma := chromaSum / float32(cols*rows)

	inkLum := (0.299*float32(ink.R) + 0.587*float32(ink.G) + 0.114*float32(ink.B)) / 255
	closeness := 1 - float32(math.Abs(float64(lum-inkLum)))
	if closeness < 0 {
		closeness = 0
	}
	density := 0.55*closeness + 0.7*texture + 0.3*chroma
	density = min(max(density, 0), 1)
	satKept := min(max(1-0.75*chroma, 0.45), 1)
	return glassStats{density: density, lum: lum, texture: texture, chroma: chroma, saturationKept: satKept}
}

// 小工具：minF 已由 theme.go 提供；这里补 maxF/absF，避免为两行引入泛型。
func maxF(a, b float32) float32 {
	if a > b {
		return a
	}
	return b
}

// panelMaterial 给菜单/弹层/toast 这类「经典 = glass.Blur + 自铺底色，
// 新主题 = liquidGlass」的面板选材质。经典分支的底色/洗光仍由调用方的
// Draw 负责（见 settingsMenu / collabMenu）。
func (t Theme) panelMaterial(o glassOptions) ui.Material {
	if t.classicGlass() {
		return blurLayer(glass.Blur{Radius: o.blur})
	}
	o.dark = t.glassIsDark()
	return liquidGlassLayer(t, o)
}
