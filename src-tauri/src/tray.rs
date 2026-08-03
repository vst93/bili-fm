//! 系统托盘 (跨平台, 对应旧版 tray_windows.go / tray_linux.go)。
//!
//! 使用 Tauri 内建 `TrayIconBuilder`, 无需平台相关的 cgo / Win32 代码。
//! - 左键单击 (Windows/macOS): 显示主窗口
//! - 右键菜单: 「显示窗口」/「退出」
//! - Linux (libappindicator): 不触发 Click 事件, 菜单常驻「显示窗口」恢复窗口
//!
//! 窗口关闭行为见 lib.rs 的 `on_window_event`: macOS/Linux 关闭时隐藏到托盘
//! (托盘常驻), Windows 关闭即退出。

use tauri::menu::{Menu, MenuItem};
use tauri::tray::{MouseButton, MouseButtonState, TrayIconBuilder, TrayIconEvent};
use tauri::{AppHandle, Manager};

pub const TRAY_ID: &str = "main-tray";

/// 初始化系统托盘。TrayIcon 由 app 持有, 无需手动保活。
pub fn init(app: &AppHandle) -> tauri::Result<()> {
    let show = MenuItem::with_id(app, "show", "显示窗口", true, None::<&str>)?;
    let quit = MenuItem::with_id(app, "quit", "退出", true, None::<&str>)?;
    let menu = Menu::with_items(app, &[&show, &quit])?;

    let mut builder = TrayIconBuilder::with_id(TRAY_ID)
        .tooltip("bili-FM")
        .menu(&menu)
        // 左键不弹菜单, 交给 Click 事件显示窗口; 右键仍弹出菜单
        .show_menu_on_left_click(false)
        .on_menu_event(|app, event| match event.id().as_ref() {
            "show" => show_main_window(app),
            "quit" => app.exit(0),
            _ => {}
        })
        .on_tray_icon_event(|tray, event| {
            // 平台说明: Linux (libappindicator) 不触发 TrayIconEvent,
            // 通过菜单「显示窗口」恢复窗口。
            if let TrayIconEvent::Click {
                button: MouseButton::Left,
                button_state: MouseButtonState::Up,
                ..
            } = event
            {
                show_main_window(tray.app_handle());
            }
        });

    // 图标: 使用满铺的 icon-square.png (无透明边距), 适配托盘小尺寸显示。
    // icon.png 带有 47px 透明边距 (约 9%) 用于 macOS Dock squircle,
    // 在任务栏/状态栏小尺寸下会显得过小。
    let icon = tauri::image::Image::from_bytes(include_bytes!("../icons/icon-square.png"))?;
    builder = builder.icon(icon);
    builder.build(app)?;
    Ok(())
}

/// 显示并聚焦主窗口 (从托盘恢复, 对应旧版 bringWindowToFront 等逻辑)。
fn show_main_window(app: &AppHandle) {
    if let Some(window) = app.get_webview_window("main") {
        let _ = window.unminimize();
        let _ = window.show();
        let _ = window.set_focus();
    }
}
