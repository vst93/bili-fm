package view

import "runtime"

// viewGOOS 是本进程运行的系统，对应原版给 <body> 打上的
// platform-linux / platform-windows / platform-darwin 类。原版 CSS 里对这些
// 平台各有一组覆写（`body.platform-linux #root { … }`、`body.platform-linux
// .danmaku-message-row { … }` 等），原生版按同一份覆写选值。
//
// runtime.GOOS 是编译期常量，测试没法换系统，所以这里包一层变量：测试改写
// 它来断言平台分支，defer 还原。生产代码只在进程启动时读一次（等价于常量），
// 不会并发改写。
var viewGOOS = runtime.GOOS

// isLinux 报告当前平台是不是 Linux（原版 body.platform-linux）。Linux 没有
// backdrop-filter，原版给它准备了不透明的兜底配色，原生版主平台也正是 Linux。
func isLinux() bool { return viewGOOS == "linux" }
