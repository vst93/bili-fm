package main

// 「立即重启」只能走 mygo.App.Relaunch()。
//
// mygo 0.9.2（#1909）起，RequestSingleInstanceLock 的锁由框架持有到**进程退出**
// （0.9.1 及以前在 OnQuit 就释放）。更新装好之后点「立即重启」，新进程会等这次
// 退出真正完成再接手；Windows 侧还有 #1897「退出前销毁窗口」，兜住输入法把进程
// 钉住、exe 换不掉的老问题。
//
// 这套语义只对 Relaunch 成立：自己拼 os.StartProcess / exec.Command 起一个新
// 进程，会撞上仍持锁的旧进程 —— 新进程要么等它退出，要么被当成第二个实例、把
// 参数交回旧进程后自己退出。谁把这条换成自拼启动，用户看到的就是「点了重启没
// 反应」或「重启后还是旧版本」。
//
// 用源码断言而不是行为测试：headless 里没有可被替换的进程，Relaunch 本身无法
// 在测试进程里跑（会重启测试二进制）。走 go/parser 读 AST，注释里提到这些名字
// 不算（main.go 的说明文字就写着它们）。

import (
	"go/ast"
	"go/parser"
	"go/token"
	"testing"
)

// spawnSelector 是「自己起一个新进程」的写法：解析成 AST 后按 x.Sel 的
// 完整选择子判，注释与字符串里的同名文字不会误报。
var spawnSelector = map[string]bool{
	"os.StartProcess":     true,
	"exec.Command":        true,
	"exec.CommandContext": true,
	"syscall.Exec":        true,
}

func TestRelaunchGoesThroughFramework(t *testing.T) {
	// 更新 / 启动这几条路径里必须有一条 mygo.App.Relaunch()，且都不许自拼启动。
	files := []string{"prefs.go", "update_auto.go", "update_source.go", "main.go"}
	found := false
	for _, file := range files {
		f, err := parser.ParseFile(token.NewFileSet(), file, nil, 0)
		if err != nil {
			t.Fatalf("解析 %s 失败：%v", file, err)
		}
		ast.Inspect(f, func(n ast.Node) bool {
			sel, ok := n.(*ast.SelectorExpr)
			if !ok {
				return true
			}
			if sel, ok := selectorName(sel); ok {
				if spawnSelector[sel] {
					t.Errorf("%s 出现自拼的启动方式 %q：重启只走 mygo.App.Relaunch"+
						"（单实例锁由框架持有到进程退出）", file, sel)
				}
				if sel == "mygo.App.Relaunch" {
					found = true
				}
			}
			return true
		})
	}
	if !found {
		t.Fatal("找不到 mygo.App.Relaunch()：更新后的「立即重启」丢了")
	}
}

// selectorName 把 x.y 或 x.y.z 这样的选择子还原成点分名字；更深的表达式
// （函数调用的返回值之类）返回 false，不参与判断。
func selectorName(e ast.Expr) (string, bool) {
	switch v := e.(type) {
	case *ast.SelectorExpr:
		if base, ok := v.X.(*ast.Ident); ok {
			return base.Name + "." + v.Sel.Name, true
		}
		if base, ok := selectorName(v.X); ok {
			return base + "." + v.Sel.Name, true
		}
	case *ast.Ident:
		return v.Name, true
	}
	return "", false
}
