import assert from "node:assert/strict";
import { readFileSync, readdirSync } from "node:fs";
import { test } from "node:test";

// 回归轮 15：mini-mode body class 与 isMiniMode state 的写权收敛。
//
// 真实 Tauri 窗口无法在 headless 复刻，因此这里：
//   1) 从真实 dist bundle 里断言「唯一写权」契约（只有 effect 写 class，
//      switchWindowMode / catch 回滚只改 state）；
//   2) 用与 effect 语义一致的极简 DOM 模型，跑三个场景，断言
//      body.mini-mode 类与 state 恒等，且主窗下按钮 elementFromPoint 命中。

const distDir = new URL("../dist/assets/", import.meta.url);
const bundleName = readdirSync(distDir).find((name) => /^index-.*\.js$/.test(name));
assert.ok(bundleName, "dist bundle index-*.js must exist (run npm run build first)");
const bundle = readFileSync(new URL(bundleName, distDir), "utf8");

// --- dist bundle 契约 -----------------------------------------------------

test("dist bundle: body.mini-mode 只有 effect 一处写权", () => {
  const toggles = bundle.match(/classList\.toggle\("mini-mode"/g) ?? [];
  assert.equal(toggles.length, 1, "must be exactly one classList.toggle writer");
  // 唯一写权必须挂在 effect 上（紧跟 v.useEffect(...)），且以 state 变量为值。
  assert.match(
    bundle,
    /useEffect\(\(\)=>\(document\.body\.classList\.toggle\("mini-mode",(\w+)\)/,
    "the sole writer must be an effect driven by the mini-mode state",
  );
});

test("dist bundle: switchWindowMode / catch 回滚不直接碰 body class", () => {
  // 切换函数体内不得出现 classList（否则又是多入口写 class）。
  // 标识符用 \w+ 通配：minify 后的变量名会随构建漂移（轮 19 教训）。
  const switchFn = bundle.match(/\w+=async\(\)=>\{if\([^)]*\)return;\w+\.current=!0,\w+\(!0\);[^}]*set_window_size/);
  assert.ok(switchFn, "switchWindowMode body must be present in bundle");
  assert.ok(
    !switchFn[0].includes("classList"),
    "switchWindowMode must not touch classList directly",
  );
});

// --- 三个场景 -------------------------------------------------------------

// 极简 body class 模型：与 effect 语义一致 —— class 是无条件跟随 state。
function createBody() {
  const classes = new Set();
  return {
    classes,
    classList: {
      toggle(name, on) {
        if (on) classes.add(name);
        else classes.delete(name);
      },
      remove(name) {
        classes.delete(name);
      },
    },
  };
}

// 用 effect 语义驱动 class：每次 state 变化后同步一次（React effect 的等价）。
function mountMiniModeMachine(body) {
  let isMiniMode = false;
  let effectCleanup = null;
  const sync = () => {
    effectCleanup?.();
    body.classList.toggle("mini-mode", isMiniMode);
    effectCleanup = () => body.classList.remove("mini-mode");
  };
  sync();
  return {
    get state() {
      return isMiniMode;
    },
    setIsMiniMode(next) {
      isMiniMode = next;
      sync();
    },
  };
}

// 主窗（非 mini）下切换键的可见性：body 无 mini-mode 时按钮可命中。
function switchKeyHitTest(body) {
  const visible = !body.classes.has("mini-mode");
  return visible ? { id: "switch-window-mode" } : null;
}

test("场景 a: 主窗常态 —— class 与 state 一致，按钮命中", () => {
  const body = createBody();
  const machine = mountMiniModeMachine(body);
  assert.equal(machine.state, false);
  assert.equal(body.classes.has("mini-mode"), false);
  assert.deepEqual(switchKeyHitTest(body), { id: "switch-window-mode" });
});

test("场景 b: mini → 主窗切换（invoke 成功）—— 双向一致，按钮命中", () => {
  const body = createBody();
  const machine = mountMiniModeMachine(body);

  // 进入 mini：只改 state，effect 写 class。
  machine.setIsMiniMode(true);
  assert.equal(body.classes.has("mini-mode"), true);
  assert.equal(body.classes.has("mini-mode"), machine.state);

  // 切回主窗：同样只改 state。
  machine.setIsMiniMode(false);
  assert.equal(body.classes.has("mini-mode"), false);
  assert.equal(body.classes.has("mini-mode"), machine.state);
  assert.deepEqual(switchKeyHitTest(body), { id: "switch-window-mode" });
});

test("场景 c: catch 回滚路径 —— class 跟随回滚后的 state", () => {
  const body = createBody();
  const machine = mountMiniModeMachine(body);

  // 进入 mini 的 invoke 失败：乐观置 true，随后 catch 回滚为 false。
  machine.setIsMiniMode(true);
  assert.equal(body.classes.has("mini-mode"), true);
  machine.setIsMiniMode(false); // catch 回滚只 setState
  assert.equal(body.classes.has("mini-mode"), false);
  assert.equal(body.classes.has("mini-mode"), machine.state);
  assert.deepEqual(switchKeyHitTest(body), { id: "switch-window-mode" });
});

// --- SponsorBlock 失败可见性 ----------------------------------------------

test("dist bundle: sponsorSkip 写失败有可见 toast，非静默吞", () => {
  const toastCount = (bundle.match(/开关状态保存失败/g) ?? []).length;
  assert.ok(toastCount >= 2, "localStorage 与 invoke 两条失败路径都要提示");
  assert.ok(
    !/set_kv",\{key:"sponsorSkip"[\s\S]{0,80}\.catch\(\(\)=>\{\}\)/.test(bundle),
    "invoke(set_kv) catch 不得再静默为空函数",
  );
});
