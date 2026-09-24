//! Windows 任务栏缩略图工具栏 —— 悬停任务栏图标时预览小窗里的媒体控制按钮。
//!
//! 背景：应用已经把 Media Session 的 handler 接好了，但 WebView2 只包含 Blink 的
//! `mediaSession`，不包含 Chrome 浏览器层对 Windows SMTC 的集成，所以 Windows 上
//! 悬停任务栏图标时看不到任何媒体控件。这里补上原生那一半：
//! `ITaskbarList3::ThumbBarAddButtons`（Windows 7 ~ 11 的缩略图工具栏）。
//!
//! 结构：
//! - [`init`]（主线程，`lib.rs` 的 setup 里调用）：建 `ITaskbarList3`、按当前 DPI 与
//!   系统主题光栅化 4 个 HICON（上一首 / 播放 / 暂停 / 下一首）、加 3 个按钮，并用
//!   `SetWindowSubclass` 挂子类过程接管 `WM_COMMAND`。
//! - 按钮点击 → `emit("taskbar-media", "prev" | "playpause" | "next")`，前端复用已有的
//!   `mediaNavigationRef` / `setIsPlaying`，不新增播放逻辑。
//! - [`set_taskbar_media_state`] 命令（前端在播放状态 / 可上一首 / 可下一首变化时调用）
//!   → `ThumbBarUpdateButtons` 换图标与禁用态；系统主题切换时顺带重建图标。
//!
//! 图标不落盘：直接按几何形状 4×4 超采样光栅化成 32bpp DIB 再 `CreateIconIndirect`，
//! 因此不需要往 `icons/` 里加二进制资源，也不需要 SVG 渲染依赖。
//!
//! 已知限制（v1，Windows 实机未验证）：explorer.exe 重启后任务栏会重建，缩略图工具栏
//! 需要重新 `ThumbBarAddButtons`（监听 `TaskbarCreated` 注册消息）。当前版本只在启动时
//! 添加一次；若实机发现「资源管理器重启后按钮消失」，再补这一段。

use tauri::AppHandle;

/// 任务栏媒体按钮点击事件名。前端 `listen` 后按 `prev` / `playpause` / `next` 分发。
pub const TASKBAR_MEDIA_EVENT: &str = "taskbar-media";

/// 初始化任务栏缩略图工具栏（仅 Windows 生效；其余平台 no-op）。
pub fn init(window: &tauri::WebviewWindow) {
    #[cfg(target_os = "windows")]
    imp::init(window);

    #[cfg(not(target_os = "windows"))]
    let _ = window;
}

/// 同步任务栏媒体按钮的状态（播放/暂停图标 + 上一首/下一首可用性）。
///
/// 非 Windows 平台直接返回 `Ok(())`：前端无需按平台分支调用，命令是稳定的 no-op。
#[tauri::command]
pub fn set_taskbar_media_state(
    app: AppHandle,
    playing: bool,
    can_prev: bool,
    can_next: bool,
) -> Result<(), String> {
    #[cfg(target_os = "windows")]
    {
        // ITaskbarList3 是 STA 的 in-proc COM 对象，且缩略图工具栏属于窗口自身状态；
        // 命令本身跑在 async runtime 上，统一投递回主线程执行。
        app.run_on_main_thread(move || imp::update(playing, can_prev, can_next))
            .map_err(|e| format!("set_taskbar_media_state 失败: {e}"))?;
    }

    #[cfg(not(target_os = "windows"))]
    {
        let _ = (app, playing, can_prev, can_next);
    }

    Ok(())
}

#[cfg(target_os = "windows")]
mod imp {
    use std::sync::{Mutex, OnceLock};

    use tauri::{AppHandle, Emitter};
    use windows::core::{w, BOOL};
    use windows::Win32::Foundation::{ERROR_SUCCESS, HWND, LPARAM, LRESULT, WPARAM};
    use windows::Win32::Graphics::Gdi::{
        CreateBitmap, CreateDIBSection, DeleteObject, BITMAPINFO, BITMAPINFOHEADER, BI_RGB,
        DIB_RGB_COLORS, HGDIOBJ,
    };
    use windows::Win32::System::Com::{CoCreateInstance, CLSCTX_INPROC_SERVER};
    use windows::Win32::System::Registry::{RegGetValueW, HKEY_CURRENT_USER, RRF_RT_REG_DWORD};
    use windows::Win32::UI::HiDpi::GetDpiForWindow;
    use windows::Win32::UI::Shell::{
        DefSubclassProc, SetWindowSubclass, ITaskbarList3, TaskbarList, THBN_CLICKED,
        THBF_DISABLED, THBF_ENABLED, THB_FLAGS, THB_ICON, THB_TOOLTIP, THUMBBUTTON,
        THUMBBUTTONMASK,
    };
    use windows::Win32::UI::WindowsAndMessaging::{
        CreateIconIndirect, HICON, ICONINFO, WM_COMMAND,
    };

    use super::TASKBAR_MEDIA_EVENT;

    /// 子类化标识（任意非零值即可，用于后续 `RemoveWindowSubclass`）。
    const SUBCLASS_ID: usize = 0x4249_4C46; // "BILF"

    const BTN_PREV: u32 = 1;
    const BTN_PLAY: u32 = 2;
    const BTN_NEXT: u32 = 3;

    /// 图标形状（归一化坐标，v 轴向下）。
    #[derive(Clone, Copy, PartialEq, Eq)]
    enum Glyph {
        Prev,
        Play,
        Pause,
        Next,
    }

    struct Icons {
        prev: HICON,
        play: HICON,
        pause: HICON,
        next: HICON,
    }

    struct MediaState {
        app: AppHandle,
        taskbar: ITaskbarList3,
        hwnd: HWND,
        icons: Icons,
        icon_size: i32,
        dark: bool,
        playing: bool,
        can_prev: bool,
        can_next: bool,
    }

    // HWND / HICON / COM 接口指针都不实现 Send/Sync（windows crate 里是裸指针包装）。
    // 这里所有访问都发生在窗口所属线程：`init` 在 setup（主线程）调用，`update` 经
    // `run_on_main_thread` 投递到主线程，子类化回调由主线程消息循环触发。显式声明
    // 是为了能把状态放进 `static`。
    unsafe impl Send for MediaState {}

    static STATE: OnceLock<Mutex<MediaState>> = OnceLock::new();

    pub fn init(window: &tauri::WebviewWindow) {
        let Ok(hwnd) = window.hwnd() else {
            return;
        };

        let taskbar: ITaskbarList3 =
            match unsafe { CoCreateInstance(&TaskbarList, None, CLSCTX_INPROC_SERVER) } {
                Ok(taskbar) => taskbar,
                Err(error) => {
                    eprintln!("[taskbar] 创建 ITaskbarList3 失败: {error}");
                    return;
                }
            };
        if let Err(error) = unsafe { taskbar.HrInit() } {
            eprintln!("[taskbar] ITaskbarList::HrInit 失败: {error}");
            return;
        }

        // 缩略图工具栏图标按 DPI 缩放：16px @100%，24px @150%，32px @200%。
        let dpi = unsafe { GetDpiForWindow(hwnd) }.max(96);
        let icon_size = ((16.0 * dpi as f32 / 96.0).round() as i32).clamp(16, 48);
        let dark = system_uses_dark_theme();
        let icons = build_icons(icon_size, dark);

        let buttons = [
            make_button(BTN_PREV, icons.prev, "上一首", true),
            make_button(BTN_PLAY, icons.play, "播放", true),
            make_button(BTN_NEXT, icons.next, "下一首", true),
        ];
        if let Err(error) = unsafe { taskbar.ThumbBarAddButtons(hwnd, &buttons) } {
            eprintln!("[taskbar] ThumbBarAddButtons 失败: {error}");
            return;
        }

        // 子类化窗口过程以接管 WM_COMMAND。失败（极老系统没有 comctl32 v6）时只丢按钮
        // 交互，不影响其它功能。
        if !unsafe { SetWindowSubclass(hwnd, Some(subclass_proc), SUBCLASS_ID, 0) }.as_bool() {
            eprintln!("[taskbar] SetWindowSubclass 失败，任务栏媒体按钮点击不会生效");
        }

        let state = MediaState {
            app: window.app_handle().clone(),
            taskbar,
            hwnd,
            icons,
            icon_size,
            dark,
            playing: false,
            can_prev: true,
            can_next: true,
        };
        let _ = STATE.set(Mutex::new(state));
    }

    pub fn update(playing: bool, can_prev: bool, can_next: bool) {
        let Some(state) = STATE.get() else {
            return;
        };
        let Ok(mut state) = state.lock() else {
            return;
        };

        // 系统主题切换（浅色/深色）时重建图标：任务栏预览浮层的底色跟随系统主题，
        // 图标颜色不跟着换的话，浅色主题下白图标会看不见。
        let dark = system_uses_dark_theme();
        let theme_changed = dark != state.dark;
        if theme_changed {
            state.dark = dark;
            let size = state.icon_size;
            state.icons = build_icons(size, dark);
        }

        let state_changed =
            state.playing != playing || state.can_prev != can_prev || state.can_next != can_next;
        // 主题变了必须重新下发图标；只有两者都没变才跳过这次 IPC。
        if !theme_changed && !state_changed {
            return;
        }
        state.playing = playing;
        state.can_prev = can_prev;
        state.can_next = can_next;

        let buttons = [
            make_button(BTN_PREV, state.icons.prev, "上一首", can_prev),
            make_button(
                BTN_PLAY,
                if playing {
                    state.icons.pause
                } else {
                    state.icons.play
                },
                if playing { "暂停" } else { "播放" },
                true,
            ),
            make_button(BTN_NEXT, state.icons.next, "下一首", can_next),
        ];
        let _ = unsafe { state.taskbar.ThumbBarUpdateButtons(state.hwnd, &buttons) };
    }

    /// 窗口子类过程：只关心缩略图工具栏按钮的 `THBN_CLICKED`，其余透传给 wry。
    ///
    /// edition 2021 下 `unsafe fn` 体内不强制 `unsafe {}`（加了反而触发 unused_unsafe），
    /// 这里保持无块写法。
    unsafe extern "system" fn subclass_proc(
        hwnd: HWND,
        msg: u32,
        wparam: WPARAM,
        lparam: LPARAM,
        _subclass_id: usize,
        _ref_data: usize,
    ) -> LRESULT {
        if msg == WM_COMMAND && ((wparam.0 >> 16) & 0xFFFF) as u32 == THBN_CLICKED {
            let action = match (wparam.0 & 0xFFFF) as u32 {
                BTN_PREV => Some("prev"),
                BTN_PLAY => Some("playpause"),
                BTN_NEXT => Some("next"),
                _ => None,
            };
            if let Some(action) = action {
                if let Some(state) = STATE.get() {
                    if let Ok(guard) = state.lock() {
                        let _ = guard.app.emit(TASKBAR_MEDIA_EVENT, action);
                    }
                }
                return LRESULT(0);
            }
        }
        DefSubclassProc(hwnd, msg, wparam, lparam)
    }

    fn build_icons(size: i32, dark: bool) -> Icons {
        Icons {
            prev: build_icon(size, Glyph::Prev, dark),
            play: build_icon(size, Glyph::Play, dark),
            pause: build_icon(size, Glyph::Pause, dark),
            next: build_icon(size, Glyph::Next, dark),
        }
    }

    /// 拼装一个 `THUMBBUTTON`。`szTip` 需要以 0 结尾，`THUMBBUTTON::default()` 已把
    /// 整个数组清零，这里只覆盖实际文本。
    fn make_button(id: u32, icon: HICON, tip: &str, enabled: bool) -> THUMBBUTTON {
        let mut button = THUMBBUTTON::default();
        button.dwMask = THUMBBUTTONMASK(THB_ICON.0 | THB_TOOLTIP.0 | THB_FLAGS.0);
        button.iId = id;
        button.iBitmap = 0;
        button.hIcon = icon;
        button.dwFlags = if enabled { THBF_ENABLED } else { THBF_DISABLED };
        for (slot, unit) in button.szTip.iter_mut().zip(tip.encode_utf16()) {
            *slot = unit;
        }
        button
    }

    /// 系统（任务栏/预览浮层）是否使用深色主题。
    ///
    /// `AppsUseLightTheme`：1 = 浅色，0 = 深色。读不到（键缺失/无权限）时按浅色处理，
    /// 保证至少不会出现白底白图标。
    fn system_uses_dark_theme() -> bool {
        let mut value: u32 = 1;
        let mut size = std::mem::size_of::<u32>() as u32;
        let status = unsafe {
            RegGetValueW(
                HKEY_CURRENT_USER,
                w!("Software\\Microsoft\\Windows\\CurrentVersion\\Themes\\Personalize"),
                w!("AppsUseLightTheme"),
                RRF_RT_REG_DWORD,
                None,
                Some(&mut value as *mut u32 as *mut core::ffi::c_void),
                Some(&mut size),
            )
        };
        status == ERROR_SUCCESS && value == 0
    }

    /// 把一个字形光栅化成 32bpp（直通 alpha）的 HICON。
    ///
    /// 4×4 超采样：16px 下按像素中心硬采样锯齿明显，超采样后边缘像素的 alpha 是
    /// 覆盖率，缩到 16px 仍然干净。
    fn build_icon(size: i32, glyph: Glyph, dark: bool) -> HICON {
        const SS: i32 = 4;

        let mut bmi = BITMAPINFO::default();
        bmi.bmiHeader.biSize = std::mem::size_of::<BITMAPINFOHEADER>() as u32;
        bmi.bmiHeader.biWidth = size;
        // 负高度 = top-down DIB，行序与下面的扫描顺序一致。
        bmi.bmiHeader.biHeight = -size;
        bmi.bmiHeader.biPlanes = 1;
        bmi.bmiHeader.biBitCount = 32;
        bmi.bmiHeader.biCompression = BI_RGB.0;

        let mut bits: *mut core::ffi::c_void = std::ptr::null_mut();
        let color = match unsafe {
            CreateDIBSection(None, &bmi, DIB_RGB_COLORS, &mut bits, None, 0)
        } {
            Ok(bitmap) => bitmap,
            Err(_) => return HICON::default(),
        };
        if bits.is_null() {
            unsafe { let _ = DeleteObject(HGDIOBJ(color.0)); };
            return HICON::default();
        }

        // 深色主题 → 白图标；浅色主题 → 近黑图标。
        let channel: u8 = if dark { 255 } else { 32 };
        let pixels = unsafe {
            std::slice::from_raw_parts_mut(bits as *mut u8, (size * size * 4) as usize)
        };
        for y in 0..size {
            for x in 0..size {
                let mut hits = 0u32;
                for sy in 0..SS {
                    for sx in 0..SS {
                        let u = (x as f32 + (sx as f32 + 0.5) / SS as f32) / size as f32;
                        let v = (y as f32 + (sy as f32 + 0.5) / SS as f32) / size as f32;
                        if inside(glyph, u, v) {
                            hits += 1;
                        }
                    }
                }
                let alpha = (hits * 255 / (SS * SS) as u32) as u8;
                let offset = ((y * size + x) * 4) as usize;
                // BGRA
                pixels[offset] = channel;
                pixels[offset + 1] = channel;
                pixels[offset + 2] = channel;
                pixels[offset + 3] = alpha;
            }
        }

        // AND mask：32bpp 图标的透明度由 alpha 通道决定，mask 全 0（不遮挡）。
        // 1bpp 位图的行必须按 WORD 对齐。
        let mask_stride = (((size + 15) / 16) * 2) as usize;
        let mask_bits = vec![0u8; mask_stride * size as usize];
        let mask = unsafe {
            CreateBitmap(
                size,
                size,
                1,
                1,
                Some(mask_bits.as_ptr() as *const core::ffi::c_void),
            )
        };
        if mask.0.is_null() {
            unsafe { let _ = DeleteObject(HGDIOBJ(color.0)); };
            return HICON::default();
        }

        let info = ICONINFO {
            fIcon: BOOL::from(true),
            xHotspot: 0,
            yHotspot: 0,
            hbmMask: mask,
            hbmColor: color,
        };
        let icon = unsafe { CreateIconIndirect(&info) }.unwrap_or_default();

        // HICON 已经复制了位图内容，原始位图立即释放；HICON 本身进程期内一直持有。
        unsafe {
            let _ = DeleteObject(HGDIOBJ(mask.0));
            let _ = DeleteObject(HGDIOBJ(color.0));
        }
        icon
    }

    fn inside(glyph: Glyph, u: f32, v: f32) -> bool {
        match glyph {
            Glyph::Play => in_triangle(u, v, (0.30, 0.20), (0.30, 0.80), (0.78, 0.50)),
            Glyph::Pause => {
                in_rect(u, v, 0.32, 0.20, 0.44, 0.80) || in_rect(u, v, 0.56, 0.20, 0.68, 0.80)
            }
            Glyph::Prev => {
                in_rect(u, v, 0.22, 0.20, 0.30, 0.80)
                    || in_triangle(u, v, (0.80, 0.20), (0.80, 0.80), (0.34, 0.50))
            }
            Glyph::Next => {
                in_rect(u, v, 0.70, 0.20, 0.78, 0.80)
                    || in_triangle(u, v, (0.20, 0.20), (0.20, 0.80), (0.66, 0.50))
            }
        }
    }

    fn in_rect(u: f32, v: f32, x0: f32, y0: f32, x1: f32, y1: f32) -> bool {
        u >= x0 && u <= x1 && v >= y0 && v <= y1
    }

    /// 三角形命中测试：三条边的叉积同号即在内（含边界）。
    fn in_triangle(u: f32, v: f32, a: (f32, f32), b: (f32, f32), c: (f32, f32)) -> bool {
        let cross = |p: (f32, f32), q: (f32, f32)| {
            (u - p.0) * (q.1 - p.1) - (v - p.1) * (q.0 - p.0)
        };
        let d1 = cross(a, b);
        let d2 = cross(b, c);
        let d3 = cross(c, a);
        let has_negative = d1 < 0.0 || d2 < 0.0 || d3 < 0.0;
        let has_positive = d1 > 0.0 || d2 > 0.0 || d3 > 0.0;
        !(has_negative && has_positive)
    }
}
