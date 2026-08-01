//! bili-FM Tauri 后端 crate。
//!
//! ⚠️ 注意: 这是 Round 1 的占位实现。Round 2 将在此完成完整的应用装配:
//! `tauri::Builder`、插件注册 (shell/store/global-shortcut)、命令注册、
//! 窗口与托盘管理、迷你模式，并在启动时拉起图片代理 (`proxy::start_server`)。
//!
//! 各后端模块已就位，可独立编译:
//! - `dkv`:      旧版 Wails dkv 存储的兼容读写层 (登录态无缝迁移的关键)
//! - `bilibili`: B 站 API 客户端 (service/bl.go 的 Rust 移植)
//! - `proxy`:    内嵌图片代理 HTTP 服务器 (127.0.0.1:4654)
//! - `commands`: `#[tauri::command]` 桥接函数 (Wails -> Tauri 命令映射)

pub mod bilibili;
pub mod commands;
pub mod dkv;
pub mod proxy;

/// 应用入口 (占位)。Round 2 替换为完整的 `tauri::Builder` 装配。
pub fn run() {
    println!("bili-FM: backend placeholder — full Tauri wiring lands in Round 2");
}
