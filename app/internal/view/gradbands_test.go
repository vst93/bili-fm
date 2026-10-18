package view

import "github.com/egoist/mygo/ui"

// gradBands 把多停靠点渐变切成 n 条等高的带。**这是被否掉的旧写法，只留给
// 反证用**：那等于把渐变线当成水平的、逐带重新投影，斜向（135°）渐变的误差
// 实测最坏 **38 灰阶**（见 loginsheen_test.go 的 TestLoginSheenMatchesCSS）。
//
// 视图一律用 `gradStops` + `ui.LinearGradient.Stops`（真多停靠点，与 CSS
// `linear-gradient(Adeg, c1 p1%, …)` 逐值等价）。2026-10-25 从 theme.go 挪进
// 测试包：生产代码已无调用点（grep 只剩 loginsheen_test / modal_test /
// titlebar_test 三处反证），留在生产包里有被误用的风险。
func gradBands(stops []gradStop, n int) [][2]ui.Color {
	if n < 1 {
		n = 1
	}
	out := make([][2]ui.Color, n)
	for i := range out {
		out[i] = [2]ui.Color{
			gradAt(stops, float32(i)/float32(n)),
			gradAt(stops, float32(i+1)/float32(n)),
		}
	}
	return out
}
