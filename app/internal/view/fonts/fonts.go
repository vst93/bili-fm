// Package fonts 注册 bili-FM 内置的拉丁字体 Inter（SIL OFL，见 LICENSE.txt 与
// README.md），给 view 包的三平台统一字体栈用。
//
// 为什么单独一个包：注册必须在**包级变量初始化之前**完成。view 里有一批
// 包级变量直接由字体度量算出来（statCountBoxH/Shift、infoTitlePitch、
// miniTitlePitch、infoTitleFont…），而 Go 先初始化被导入的包 —— 放在这里
// 注册，view 的任何 var 初始化都能看到 Inter。写成 view 自己的 init() 就晚了
// （init 在所有 var 之后跑），那些常量和 Font 句柄会悄悄冻住系统字体的度量。
package fonts

import (
	_ "embed"

	"github.com/egoist/mygo/ui"
)

// Family 是注册后 Text / ui.Font 里用的族名。
const Family = "Inter"

//go:embed Inter-Regular.ttf
var regular []byte

//go:embed Inter-Medium.ttf
var medium []byte

//go:embed Inter-SemiBold.ttf
var semiBold []byte

//go:embed Inter-Bold.ttf
var bold []byte

//go:embed Inter-ExtraBold.ttf
var extraBold []byte

// registered 记录注册结果：失败时族名仍留在字体栈里，字体引擎会按栈顺序回退
// 到下一个（中文回退字体 / 系统字体），只是拉丁不再是 Inter。
var registered bool

func init() {
	registered = true
	for _, face := range [][]byte{regular, medium, semiBold, bold, extraBold} {
		if err := ui.RegisterFont(face, Family); err != nil {
			registered = false
			return
		}
	}
}

// Registered 报告内置字体是否注册成功（字体引擎异常时会失败）。
func Registered() bool { return registered }
