import assert from "node:assert/strict";
import { readFileSync } from "node:fs";
import { test } from "node:test";
import vm from "node:vm";
import ts from "typescript";

// 用户建议②：记忆迷你模式窗口位置。
//
// 迷你窗尺寸是唯一的 400×155，进迷你模式时只做 set_window_size(center:false)，
// 窗口停在原主窗左上角；用户拖到哪儿，下次进迷你模式应该回到哪儿。
//
// 契约：
//   A. Rust `set_window_position` 按显示器 **work_area**（不是 size，要避开任务栏）
//      夹取后再落位，多屏拔插/分辨率变化不会把窗口丢到屏幕外；
//   B. 前端只在「窗口确实是迷你尺寸」时落盘（尺寸守卫），并且模式切换期间跳过，
//      避免出迷你模式时的「先 resize 再居中」把主窗位置写脏；
//   C. 恢复只发生在进迷你模式那一条分支里。

const read = (rel) => readFileSync(new URL(rel, import.meta.url), "utf8");

const indexSrc = read("../src/pages/index.tsx");
const commandsRs = read("../src-tauri/src/commands.rs");
const libRs = read("../src-tauri/src/lib.rs");

// ---------------------------------------------------------------------------
// 前端：读取持久化位置的真实实现（直接从源码抽出执行）
// ---------------------------------------------------------------------------

function loadPositionReader(localStorageStub) {
  const keyMatch = indexSrc.match(/const MINI_WINDOW_POSITION_STORAGE_KEY = "([^"]+)"/);
  assert.ok(keyMatch, "MINI_WINDOW_POSITION_STORAGE_KEY 必须存在");

  const fnMatch = indexSrc.match(
    /\/\*\* 读取上次迷你窗位置[\s\S]*?const readMiniWindowPosition[\s\S]*?\n\};/,
  );
  assert.ok(fnMatch, "readMiniWindowPosition 必须存在");

  const source = `const MINI_WINDOW_POSITION_STORAGE_KEY = "${keyMatch[1]}";\n${fnMatch[0]}\nmodule.exports = { readMiniWindowPosition };`;
  // 抽出的片段带 TS 类型注解，先转译再执行
  const js = ts.transpile(source, {
    module: ts.ModuleKind.CommonJS,
    target: ts.ScriptTarget.ES2022,
  });
  const exports = {};
  const context = vm.createContext({
    module: { exports },
    exports,
    localStorage: localStorageStub,
    JSON,
    Number,
    Math,
  });
  vm.runInContext(js, context);
  return context.module.exports.readMiniWindowPosition;
}

const storageWith = (value) => ({
  getItem: (key) => (key === "miniWindowPosition" ? value : null),
});

test("readMiniWindowPosition: 无记录 / 损坏 / 非法值都返回 null", () => {
  assert.equal(loadPositionReader(storageWith(null))(), null);
  assert.equal(loadPositionReader(storageWith("{not json"))(), null);
  assert.equal(loadPositionReader(storageWith('{"x":"a","y":1}'))(), null);
  assert.equal(loadPositionReader(storageWith('{"x":1}'))(), null);
  assert.equal(loadPositionReader(storageWith('{"x":null,"y":null}'))(), null);
});

test("readMiniWindowPosition: 合法值取整返回", () => {
  // 跨 realm 对象原型不同，逐字段断言
  const rounded = loadPositionReader(storageWith('{"x":120.6,"y":-30.2}'))();
  assert.equal(rounded.x, 121);
  assert.equal(rounded.y, -30);

  const origin = loadPositionReader(storageWith('{"x":0,"y":0}'))();
  assert.equal(origin.x, 0);
  assert.equal(origin.y, 0);
});

// ---------------------------------------------------------------------------
// 前端：恢复 / 落盘路径与守卫
// ---------------------------------------------------------------------------

test("index: 进迷你模式时恢复位置，出迷你模式不做任何恢复", () => {
  const miniBranch = indexSrc.match(
    /if \(theIsMiniMode\) \{[\s\S]*?\} else \{/,
  );
  assert.ok(miniBranch, "switchWindowMode 的迷你分支必须存在");
  assert.match(miniBranch[0], /invoke\("set_window_size",\s*\{[\s\S]*?MINI_WINDOW_WIDTH/);
  assert.match(miniBranch[0], /readMiniWindowPosition\(\)/);
  assert.match(miniBranch[0], /invoke\("set_window_position",\s*savedPosition\)/);

  // 迷你尺寸只能来自常量，避免 set_window_size 与尺寸守卫脱钩
  assert.doesNotMatch(indexSrc, /set_window_size",\s*\{\s*width:\s*400/, "迷你尺寸必须走常量");
});

test("index: 落盘有尺寸守卫 + 模式切换守卫 + 防抖", () => {
  const effect = indexSrc.match(/getCurrentWindow\(\)\s*\n?\s*\.onMoved\([\s\S]*?\n  \}, \[\]\);/);
  assert.ok(effect, "onMoved effect 必须存在");
  const body = effect[0];

  assert.match(body, /windowModeChangingRef\.current\)\s*return/, "模式切换期间必须跳过");
  assert.match(body, /window\.innerWidth > MINI_POSITION_MAX_WIDTH/, "必须有尺寸守卫（宽）");
  assert.match(body, /window\.innerHeight > MINI_POSITION_MAX_HEIGHT/, "必须有尺寸守卫（高）");
  assert.match(body, /MINI_POSITION_SAVE_DEBOUNCE_MS/, "必须防抖，拖拽会连续触发 move");
  assert.match(body, /localStorage\.setItem\(\s*MINI_WINDOW_POSITION_STORAGE_KEY/);
  // StrictMode 双挂载：Promise 落地晚于卸载时必须立刻注销
  assert.match(body, /if \(disposed\) fn\(\);/);
  assert.match(body, /unlisten\?\.\(\);/);
});

test("index: 尺寸守卫与迷你窗尺寸同源", () => {
  assert.match(indexSrc, /const MINI_WINDOW_WIDTH = 400;/);
  assert.match(indexSrc, /const MINI_WINDOW_HEIGHT = 155;/);
  assert.match(
    indexSrc,
    /const MINI_POSITION_MAX_WIDTH = MINI_WINDOW_WIDTH \+ 20;/,
    "守卫阈值必须由迷你尺寸推导",
  );
  assert.match(indexSrc, /const MINI_POSITION_MAX_HEIGHT = MINI_WINDOW_HEIGHT \+ 20;/);
});

// ---------------------------------------------------------------------------
// Rust：set_window_position 的多屏语义
// ---------------------------------------------------------------------------

test("rust: 目标点还在某块屏的可用区内 → 原样恢复，不挪动", () => {
  const fn = commandsRs.match(/pub fn set_window_position\([\s\S]*?\n\}/);
  assert.ok(fn, "set_window_position 必须存在");
  const body = fn[0];

  // 用 work_area（不是屏幕 bounds）判定包含关系：任务栏那一条要归入「需要救援」
  assert.match(body, /let containing = monitors\.iter\(\)\.find\(/);
  assert.match(body, /x >= a\.position\.x[\s\S]*?y < a\.position\.y \+ a\.size\.height as i32/);

  // 命中就直接原样落位并返回（保留用户刻意悬挂在屏幕边缘 / 跨两块屏的姿势）
  const exact = body.match(/if containing\.is_some\(\) \{[\s\S]*?\n    \}/);
  assert.ok(exact, "命中包含关系时必须提前返回");
  assert.match(exact[0], /set_position\(tauri::PhysicalPosition::new\(x, y\)\)/);
});

test("rust: 够不着时夹到最近的屏，并保证整个窗口在屏内", () => {
  const fn = commandsRs.match(/pub fn set_window_position\([\s\S]*?\n\}/);
  const body = fn[0];

  assert.match(body, /available_monitors\(\)/);
  assert.match(body, /\.work_area\(\)/, "必须用 work_area（避开任务栏），不能用 size");
  // 最近屏 = 点到 work_area 的钳位距离平方最小
  assert.match(body, /let nearest = monitors\.iter\(\)\.min_by_key\(/);
  assert.match(body, /dx \* dx \+ dy \* dy/);
  assert.match(body, /\.clamp\(area\.position\.x, max_x\)/);
  assert.match(body, /\.clamp\(area\.position\.y, max_y\)/);
  assert.match(body, /set_position\(tauri::PhysicalPosition::new\(/);
});

test("rust: 夹取用的物理尺寸按目标屏缩放换算（混合 DPI 双屏）", () => {
  const fn = commandsRs.match(/pub fn set_window_position\([\s\S]*?\n\}/);
  const body = fn[0];

  // 当前屏缩放：把 outer_size（当前屏物理尺寸）还原成逻辑尺寸
  assert.match(body, /window\s*\n?\s*\.scale_factor\(\)/);
  assert.match(body, /\.to_logical::<f64>\(current_scale\)/);
  // 目标屏缩放：换算落地后的物理尺寸，否则会溢出 (target-current) × 逻辑宽
  assert.match(body, /\.to_physical::<i32>\(monitor\.scale_factor\(\)\)/);
  // 屏比窗口还小时不能 panic（clamp 的 lo 必须 ≤ hi）
  assert.match(body, /\.max\(area\.position\.x\)/);
  assert.match(body, /\.max\(area\.position\.y\)/);
});

test("rust: set_window_position 已注册到 invoke_handler", () => {
  assert.match(libRs, /commands::set_window_position,/);
});
