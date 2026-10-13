package view

import "github.com/egoist/mygo/ui"

// 登录面板的 135° 光泽层（原版 globals.css 的 `.login-glass::before`）。原样：
//
//	.login-glass::before {
//	    content: ""; position: absolute; inset: 0;
//	    pointer-events: none; border-radius: inherit;
//	    background: linear-gradient(135deg,
//	        rgba(255,255,255,.2) 0%,
//	        rgba(255,255,255,.08) 50%,
//	        rgba(255,255,255,0) 100%);
//	}
//	.login-glass > * { position: relative; z-index: 1; }
//
// 它是一层画在面板底之上、**内容之下**的伪元素（原版把真正的子内容抬到
// z-index:1）。原生版此前没画这层——面板走真玻璃，只有框架玻璃边缘的聚光，
// 左上到右下少了原版那道「斜向高光」，看着比原版平。上轮（1892 那轮）把它
// 记成了「下轮候选」，本轮补上。
//
// 与 `dialogSheen`（.liquid-dialog::after）同手法：Absolute inset 0、按父级
// 圆角 Clip、整棵子树 PassThrough（它盖住面板内区，参与命中测试就会把面板里
// 的点击全吃掉），再用 gradBands 把三停靠点渐变拼成四条带。
// 区别只在层级：对话框的光泽是 `::after`，画在内容**之上**；登录的是
// `::before`，画在内容**之下**，所以调用点把它放在 modalLogin 的最前面。
var loginSheenStops = []gradStop{
	{0, ui.Hex("#ffffff").Alpha(0.20)},
	{0.5, ui.Hex("#ffffff").Alpha(0.08)},
	{1, ui.Hex("#ffffff").Alpha(0)},
}

// loginSheenRadius 是登录面板的圆角（原版 rounded-2xl = 16），光泽层
// `border-radius: inherit` 跟着它裁。
const loginSheenRadius = 16

// loginSheen 画 .login-glass::before 那层 135° 白色光泽。
func loginSheen(c *ui.Context) {
	ui.Box(c).Absolute().Left(0).Top(0).Right(0).Bottom(0).
		Radius(loginSheenRadius).Clip().PassThrough().Children(func() {
		for _, b := range gradBands(loginSheenStops, 4) {
			ui.Box(c).FillWidth().Grow(1).PassThrough().
				LinearGradient(ui.LinearGradient{From: b[0], To: b[1], Angle: 135})
		}
	})
}
