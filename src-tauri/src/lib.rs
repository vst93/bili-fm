//! bili-FM Tauri 后端 crate。
//!
//! Round 2: 完整的应用装配。
//!
//! - `dkv`:      旧版 Wails dkv 存储的兼容读写层 (登录态无缝迁移的关键)
//! - `bilibili`: B 站 API 客户端 (service/bl.go 的 Rust 移植)
//! - `proxy`:    内嵌图片代理 HTTP 服务器 (127.0.0.1:4654)
//! - `commands`: `#[tauri::command]` 桥接函数 (Wails -> Tauri 命令映射)
//! - `tray`:     跨平台系统托盘
//!
//! 行为对齐旧版 main.go:
//! - Windows: 设置 WEBVIEW2_ADDITIONAL_BROWSER_ARGUMENTS 允许混合内容 (图片代理)
//! - macOS: 原生菜单 (关于 / 快捷键 / 检查更新 / 退出), 关闭窗口仅隐藏
//! - macOS/Linux: 关闭窗口隐藏到托盘 (托盘常驻); Windows: 关闭即退出
//! - 单实例: 第二实例启动时唤起已运行实例的主窗口

#![cfg_attr(not(debug_assertions), windows_subsystem = "windows")]

pub mod bilibili;
pub mod commands;
pub mod dkv;
pub mod proxy;
pub mod tray;

use tauri::{AppHandle, Emitter, Manager, RunEvent, WindowEvent};

#[cfg(target_os = "macos")]
use tauri::menu::{Menu, MenuItem, Submenu};

/// 应用入口。
pub fn run() {
    // Windows: 允许 WebView2 加载 HTTP 混合内容 (图片代理 127.0.0.1:4654),
    // 与旧版 main.go 一致。
    #[cfg(target_os = "windows")]
    std::env::set_var(
        "WEBVIEW2_ADDITIONAL_BROWSER_ARGUMENTS",
        "--allow-running-insecure-content --disable-features=MixedContentAutoupgrade",
    );

    tauri::Builder::default()
        .plugin(tauri_plugin_shell::init())
        .plugin(tauri_plugin_store::Builder::new().build())
        .plugin(tauri_plugin_global_shortcut::Builder::new().build())
        .plugin(tauri_plugin_single_instance::init(|app, _args, _cwd| {
            // 第二实例启动: 显示并聚焦已运行实例的主窗口
            // (对应旧版 main.go SingleInstanceLock.OnSecondInstanceLaunch)
            if let Some(window) = app.get_webview_window("main") {
                let _ = window.unminimize();
                let _ = window.show();
                let _ = window.set_focus();
            }
        }))
        .invoke_handler(tauri::generate_handler![
            // 搜索 / 视频信息
            commands::search_video,
            commands::get_clist,
            commands::get_url_by_cid,
            // 登录
            commands::get_login_qrcode,
            commands::get_login_status,
            commands::get_login_qrcode_status,
            commands::set_login_status,
            commands::get_sessdata,
            commands::set_sessdata,
            commands::get_user_info,
            // 动态 / 推荐 / 收藏夹 / UP主 / 历史 / 系列 / 热门
            commands::get_feed_list,
            commands::get_rcmd_list,
            commands::get_fav_folder_list,
            commands::get_fav_folder_detail,
            commands::get_up_video_list,
            commands::get_history_list,
            commands::get_series_list,
            commands::get_series_videos,
            commands::get_popular_list,
            // 弹幕 / 评论
            commands::get_danmaku_list,
            commands::get_reply_list,
            // 点赞 / 投币 / 关注 / 播放进度
            commands::like_video,
            commands::has_liked,
            commands::coin_video,
            commands::has_coin,
            commands::follow,
            commands::unfollow,
            commands::is_following,
            commands::report_play_progress,
            // 图片
            commands::proxy_image_url,
            commands::fetch_image,
            commands::get_image_proxy_port,
            // 播放列表持久化
            commands::get_playlist,
            commands::set_playlist,
            commands::get_playlist_play_mode,
            commands::set_playlist_play_mode,
            // 应用信息 / 更新 / 退出
            commands::check_for_updates,
            commands::get_platform,
            commands::quit_app,
            commands::get_app_version,
            // 窗口控制 (迷你模式等)
            commands::set_window_size,
            commands::hide_window,
            commands::minimize_window,
            commands::show_window,
        ])
        .setup(|app| {
            // 启动内嵌图片代理 (127.0.0.1:4654), 对应旧版 main.go 的
            // AssetServer middleware + prewarmDNS。服务器在 tauri async runtime
            // 上运行, 不阻塞主线程。
            tauri::async_runtime::spawn(async move {
                if let Err(e) = proxy::start_server().await {
                    eprintln!("image-proxy exited: {e}");
                }
            });

            // macOS 原生菜单 (与旧版 main.go 一致); 其他平台为 no-op,
            // 由前端自绘标题栏菜单处理。
            setup_native_menu(app.handle())?;

            // 系统托盘 (跨平台; 对应旧版 tray_windows.go / tray_linux.go)
            tray::init(app.handle())?;

            Ok(())
        })
        .on_menu_event(|app, event| match event.id().as_ref() {
            "about" => {
                // 前端 titleBar 监听 'menu:show-about' 弹出关于对话框
                let _ = app.emit("menu:show-about", ());
            }
            "shortcuts" => {
                // 前端 titleBar 监听 'menu:show-shortcuts' 弹出快捷键对话框
                let _ = app.emit("menu:show-shortcuts", ());
            }
            "check-update" => {
                // 手动检查更新 (对应旧版菜单点击 CheckForUpdates(true, ""))
                let app = app.clone();
                tauri::async_runtime::spawn(async move {
                    let version = app.package_info().version.to_string();
                    let result = bilibili::check_for_updates(true, "", &version).await;
                    let _ = app.emit("menu:check-update", result);
                });
            }
            "quit" => app.exit(0),
            _ => {}
        })
        .on_window_event(|window, event| {
            // 窗口关闭行为 (对应旧版 HideWindowOnClose + OnBeforeClose):
            // - macOS/Linux: 关闭按钮 / Cmd+W / Alt+F4 仅隐藏窗口, 托盘常驻
            // - Windows: 关闭窗口即退出应用
            if let WindowEvent::CloseRequested { api, .. } = event {
                #[cfg(not(target_os = "windows"))]
                {
                    api.prevent_close();
                    let _ = window.hide();
                }
            }
        })
        .build(tauri::generate_context!())
        .expect("error while building bili-FM")
        .run(|_app, event| {
            // CMD+Q / 托盘「退出」/ quit_app 触发的退出均不拦截;
            // Windows 关闭窗口后的退出流程也保持默认。
            if let RunEvent::ExitRequested { .. } = event {
                // 无需 prevent_exit: 允许正常退出
            }
        });
}

/// macOS 原生菜单 (对应旧版 main.go 的 isMacOS 分支):
/// 「设置」子菜单 — 关于应用 / 快捷键 / 检查更新 / 退出应用 (Cmd+Q)。
///
/// 菜单点击通过事件总线通知前端 (menu:show-about / menu:show-shortcuts),
/// 由 `on_menu_event` 统一分发。
fn setup_native_menu(app: &AppHandle) -> tauri::Result<()> {
    #[cfg(not(target_os = "macos"))]
    let _ = app; // 非 macOS 无原生菜单, 前端自绘标题栏菜单处理

    #[cfg(target_os = "macos")]
    {
        let about = MenuItem::with_id(app, "about", "关于应用", true, None::<&str>)?;
        let shortcuts = MenuItem::with_id(app, "shortcuts", "快捷键", true, None::<&str>)?;
        let check_update =
            MenuItem::with_id(app, "check-update", "检查更新", true, None::<&str>)?;
        let quit = MenuItem::with_id(app, "quit", "退出应用", true, Some("CmdOrCtrl+Q"))?;
        let settings =
            Submenu::with_items(app, "设置", true, &[&about, &shortcuts, &check_update, &quit])?;
        let menu = Menu::with_items(app, &[&settings])?;
        app.set_menu(menu)?;
    }
    Ok(())
}
