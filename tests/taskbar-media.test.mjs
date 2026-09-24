import assert from "node:assert/strict";
import { readFileSync } from "node:fs";
import { test } from "node:test";

// 用户建议①：Windows 任务栏悬浮小窗（悬停任务栏图标的预览浮层）里的媒体控制按钮。
//
// 应用已经把 Media Session 的 handler 接好了，但 WebView2 只含 Blink 的 mediaSession，
// 不含 Chrome 浏览器层对 Windows SMTC 的集成 —— Windows 上悬停任务栏图标时看不到任何
// 媒体控件。这里补的是原生那一半：ITaskbarList3 的缩略图工具栏（ThumbBarAddButtons）。
//
// 本测试锁定契约（Windows 实机效果无法在 macOS 宿主上运行验证，见报告）：
//   A. 后端加按钮 → 子类化窗口过程收 WM_COMMAND/THBN_CLICKED → 发事件；
//   B. 前端复用已有播放逻辑，不新增第二套播放控制；
//   C. 非 Windows 平台整体是稳定的 no-op（命令返回 Ok，init 空转）；
//   D. 图标由纯几何形状光栅化，不落二进制资源、不引入 SVG 依赖。

const read = (rel) => readFileSync(new URL(rel, import.meta.url), "utf8");

const taskbarRs = read("../src-tauri/src/taskbar.rs");
const libRs = read("../src-tauri/src/lib.rs");
const cargoToml = read("../src-tauri/Cargo.toml");
const indexSrc = read("../src/pages/index.tsx");

// ---------------------------------------------------------------------------
// 后端：缩略图工具栏装配
// ---------------------------------------------------------------------------

test("rust: taskbar 模块已注册并接到 setup / invoke_handler", () => {
  assert.match(libRs, /pub mod taskbar;/);
  assert.match(libRs, /taskbar::set_taskbar_media_state,/);
  assert.match(libRs, /taskbar::init\(&window\);/, "必须在窗口创建后初始化");
});

test("rust: 用 ThumbBarAddButtons 加 3 个按钮，且 ID 唯一", () => {
  assert.match(taskbarRs, /CoCreateInstance\(&TaskbarList, None, CLSCTX_INPROC_SERVER\)/);
  assert.match(taskbarRs, /\.HrInit\(\)/, "ITaskbarList 必须先 HrInit");
  assert.match(taskbarRs, /ThumbBarAddButtons\(hwnd, &buttons\)/);
  assert.match(taskbarRs, /ThumbBarUpdateButtons\(state\.hwnd, &buttons\)/);

  const ids = [...taskbarRs.matchAll(/const BTN_[A-Z]+: u32 = (\d+);/g)].map((m) => m[1]);
  assert.equal(ids.length, 3, "上一首 / 播放暂停 / 下一首 三个按钮");
  assert.equal(new Set(ids).size, 3, "按钮 ID 必须唯一");

  assert.match(taskbarRs, /dwMask = THUMBBUTTONMASK\(THB_ICON\.0 \| THB_TOOLTIP\.0 \| THB_FLAGS\.0\)/);
  assert.match(taskbarRs, /THBF_DISABLED/, "不可用时必须置灰（上一首/下一首边界）");
});

test("rust: 子类化窗口过程只接管 THBN_CLICKED，其余透传", () => {
  assert.match(taskbarRs, /SetWindowSubclass\(hwnd, Some\(subclass_proc\), SUBCLASS_ID, 0\)/);
  assert.match(taskbarRs, /unsafe extern "system" fn subclass_proc\(/);
  assert.match(taskbarRs, /msg == WM_COMMAND && \(\(wparam\.0 >> 16\) & 0xFFFF\) as u32 == THBN_CLICKED/);
  assert.match(
    taskbarRs,
    /DefSubclassProc\(hwnd, msg, wparam, lparam\)/,
    "未命中的消息必须交给 wry 原来的窗口过程，否则窗口行为回归",
  );
});

test("rust: 点击 → emit(taskbar-media)，三种动作齐全", () => {
  const eventConst = taskbarRs.match(/pub const TASKBAR_MEDIA_EVENT: &str = "([^"]+)"/);
  assert.ok(eventConst, "事件名必须是常量");

  const frontendEvent = indexSrc.match(/const TASKBAR_MEDIA_EVENT = "([^"]+)"/);
  assert.ok(frontendEvent, "前端必须有同名常量");
  assert.equal(frontendEvent[1], eventConst[1], "前后端事件名必须一致");

  assert.match(taskbarRs, /guard\.app\.emit\(TASKBAR_MEDIA_EVENT, action\)/);
  for (const action of ["prev", "playpause", "next"]) {
    assert.match(taskbarRs, new RegExp(`Some\\("${action}"\\)`), `缺少 ${action} 动作`);
  }
});

test("rust: 非 Windows 平台是稳定的 no-op", () => {
  // init：非 Windows 分支只吃掉参数
  assert.match(taskbarRs, /#\[cfg\(not\(target_os = "windows"\)\)\]\s*\n\s*let _ = window;/);
  // 命令：非 Windows 分支丢弃参数后返回 Ok
  const cmd = taskbarRs.match(/pub fn set_taskbar_media_state\([\s\S]*?\n\}/);
  assert.ok(cmd, "命令必须存在");
  assert.match(cmd[0], /#\[cfg\(not\(target_os = "windows"\)\)\]/);
  assert.match(cmd[0], /Ok\(\(\)\)/);
  // 主线程投递：ITaskbarList3 是 STA COM 对象
  assert.match(cmd[0], /run_on_main_thread/);
});

// ---------------------------------------------------------------------------
// 后端：图标与主题
// ---------------------------------------------------------------------------

test("rust: 图标由几何形状光栅化（不落二进制资源、不引 SVG 依赖）", () => {
  for (const glyph of ["Prev", "Play", "Pause", "Next"]) {
    assert.match(taskbarRs, new RegExp(`Glyph::${glyph}`), `缺少 ${glyph} 字形`);
  }
  assert.match(taskbarRs, /const SS: i32 = 4;/, "必须超采样，16px 下硬采样锯齿明显");
  assert.match(taskbarRs, /CreateDIBSection\(None, &bmi, DIB_RGB_COLORS/);
  assert.match(taskbarRs, /biBitCount = 32/);
  assert.match(taskbarRs, /biHeight = -size/, "top-down DIB，行序与扫描顺序一致");
  assert.match(taskbarRs, /CreateIconIndirect\(&info\)/);
  // 不新增图标资源文件（文档注释里提到 icons/ 不算，只看实际引用）
  assert.doesNotMatch(taskbarRs, /include_bytes!/);
  assert.doesNotMatch(taskbarRs, /icons\/taskbar/);
});

test("rust: 图标按 DPI 缩放、按系统主题取色", () => {
  assert.match(taskbarRs, /GetDpiForWindow\(hwnd\)/);
  assert.match(taskbarRs, /16\.0 \* dpi as f32 \/ 96\.0/);
  assert.match(taskbarRs, /RegGetValueW\(/, "浅色/深色主题决定图标颜色");
  assert.match(taskbarRs, /AppsUseLightTheme/);
  // 主题切换时重建图标（预览浮层底色跟随系统主题），且重建后必须重新下发：
  // 提前返回的条件必须同时考虑「主题变了」和「播放态变了」两个维度。
  assert.match(taskbarRs, /let theme_changed = dark != state\.dark;/);
  assert.match(taskbarRs, /if !theme_changed && !state_changed \{/);
});

// ---------------------------------------------------------------------------
// 依赖与前端接线
// ---------------------------------------------------------------------------

test("cargo: windows crate 仅在 Windows 目标启用所需 feature", () => {
  const section = cargoToml.match(/\[target\."cfg\(windows\)"\.dependencies\]([\s\S]*)$/);
  assert.ok(section, "必须有 cfg(windows) 依赖段");
  for (const feature of [
    "Win32_Foundation",
    "Win32_Graphics_Gdi",
    "Win32_System_Com",
    "Win32_System_Registry",
    "Win32_UI_HiDpi",
    "Win32_UI_Shell",
    "Win32_UI_WindowsAndMessaging",
  ]) {
    assert.match(section[1], new RegExp(`"${feature}"`), `缺少 feature ${feature}`);
  }
  assert.match(section[1], /windows = \{ version = "0\.61"/);
});

test("index: 按钮点击复用已有播放逻辑，状态变化回写原生按钮", () => {
  assert.match(indexSrc, /listen<string>\(TASKBAR_MEDIA_EVENT,/);
  assert.match(indexSrc, /payload === "playpause"[\s\S]*?setIsPlaying\(\(prev\) => !prev\)/);
  assert.match(indexSrc, /payload === "prev"[\s\S]*?mediaNavigationRef\.current\.previous\(\)/);
  assert.match(indexSrc, /payload === "next"[\s\S]*?mediaNavigationRef\.current\.next\(\)/);

  assert.match(
    indexSrc,
    /invoke\("set_taskbar_media_state", \{\s*playing: isPlaying,\s*canPrev: canNavigateNext,\s*canNext: canNavigateNext,\s*\}\)/,
    "播放状态与上下曲可用性都要同步给原生按钮",
  );
  // StrictMode 双挂载：Promise 落地晚于卸载时必须立刻注销
  assert.match(indexSrc, /listen<string>\(TASKBAR_MEDIA_EVENT[\s\S]*?if \(disposed\) fn\(\);/);
});
