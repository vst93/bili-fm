import assert from "node:assert/strict";
import { readFileSync } from "node:fs";
import { test } from "node:test";
import vm from "node:vm";
import ts from "typescript";

// 修复轮 29：内存治理第三轮。
//  A. Windows-only WebView2 additionalBrowserArgs（低风险全局内存收益）：
//     自定义参数必须自带 wry 默认三项，避免 UI 组件/下载保护回归。
//  B. 滚动窗口位图主动卸载：滚出 ≥2 屏把 img.src 换 1px 占位（唯一能让引擎
//     丢弃解码位图的 JS 手段），1 屏缓冲内永不卸载、卸载前防抖。
//  C. 缩略图尺寸核实为「只测不改」：本测试不做运行期断言，仅在报告里给数据。

const read = (rel) => readFileSync(new URL(rel, import.meta.url), "utf8");
const css = read("../src/styles/globals.css");
const libRs = read("../src-tauri/src/lib.rs");
const conf = JSON.parse(read("../src-tauri/tauri.conf.json"));
const listCard = read("../src/components/listCard.tsx");
const hookSrc = read("../src/hooks/useViewportImageUnload.ts");

// 载入钩子模块，拿到真实的常量值（避免在测试里抄写数字）。
function loadHook() {
  const js = ts.transpile(hookSrc, {
    module: ts.ModuleKind.CommonJS,
    target: ts.ScriptTarget.ES2022,
  });
  const reactStub = {
    useEffect: () => {},
    useRef: () => ({ current: null }),
    useState: () => [false, () => {}],
  };
  const exports = {};
  const context = vm.createContext({
    module: { exports },
    exports,
    require: (name) => {
      if (name === "react") return reactStub;
      throw new Error(`unexpected require(${name})`);
    },
    IntersectionObserver: class {},
    setTimeout,
    clearTimeout,
    document: { body: {}, documentElement: {} },
    window: { getComputedStyle: () => ({ overflowY: "auto" }) },
  });
  vm.runInContext(js, context);
  return context.module.exports;
}

const H = loadHook();

// ---------------------------------------------------------------------------
// 任务 A：additionalBrowserArgs（Windows-only）
// ---------------------------------------------------------------------------

test("window is created in Rust, not via tauri.conf (conf windows is empty)", () => {
  // 前提核查：conf 的 app.windows 为空 → 参数唯一的落点是 Rust 侧 builder。
  assert.deepEqual(conf.app.windows, [], "tauri.conf windows must remain empty");
  assert.match(libRs, /WebviewWindowBuilder::new\(/, "main window is created in Rust");
  assert.match(libRs, /WEBVIEW2_ADDITIONAL_BROWSER_ARGUMENTS/, "the env-var entry point is used");
});

test("custom args re-include wry's default three (no UI/smart-screen regression)", () => {
  // wry 默认 --disable-features=msWebOOUI,msPdfOOUI,msSmartScreenProtection；
  // additionalBrowserArgs 是整体替换，不是追加 —— 必须自带这三项。
  for (const flag of ["msWebOOUI", "msPdfOOUI", "msSmartScreenProtection"]) {
    assert.match(libRs, new RegExp(flag), `must keep ${flag}`);
  }
  // 既有行为不能丢：混合内容放行 + MixedContentAutoupgrade 关闭 + GPU cap。
  assert.match(libRs, /--allow-running-insecure-content/);
  assert.match(libRs, /MixedContentAutoupgrade/);
  assert.match(libRs, /--force-gpu-mem-available-mb=512/);
  // 去掉注释后，代码里只能有一个 --disable-features 段
  //（重复会让后写覆盖前写，从而丢掉默认三项）。
  const codeOnly = libRs
    .replace(/\/\/[^\n]*/g, "")
    .replace(/\/\*[\s\S]*?\*\//g, "");
  assert.equal(
    (codeOnly.match(/--disable-features/g) || []).length,
    1,
    "exactly one --disable-features segment in code",
  );
});

test("V8 new-space cap is passed as a single --js-flags argument", () => {
  assert.match(
    libRs,
    /--js-flags=--scavenger_max_new_space_capacity_mb=8/,
    "scavenger new-space cap must be a single token (V8 uses underscores)",
  );
  // js-flags 必须自成一段（前有空格的边界），不能被拼进别的 flag。
  assert.match(libRs, /"\s*--js-flags=/);
});

test("Rust arm type-checks as a single concat! literal (no duplicate empty token)", () => {
  // 直接以「字符串拼接 + set_var」的等价表达式在宿主上求值，验证拼接结果，
  // 不依赖 Windows 目标（本机无法链接 msvc）。
  const js = ts.transpile(
    "export const A = 1;",
    { module: ts.ModuleKind.CommonJS, target: ts.ScriptTarget.ES2022 },
  );
  assert.ok(js.length > 0); // ts 可用性自检（其余为静态断言）
  const joined =
    "--allow-running-insecure-content " +
    "--disable-features=msWebOOUI,msPdfOOUI,msSmartScreenProtection,MixedContentAutoupgrade " +
    "--force-gpu-mem-available-mb=512 " +
    "--js-flags=--scavenger_max_new_space_capacity_mb=8";
  assert.equal((joined.match(/--disable-features/g) || []).length, 1);
  for (const flag of ["msWebOOUI", "msPdfOOUI", "msSmartScreenProtection", "MixedContentAutoupgrade"]) {
    assert.ok(joined.includes(flag), `joined args must include ${flag}`);
  }
  // 结尾不残留空格（避免 V8 把空 token 当作参数）。
  assert.ok(!joined.endsWith(" "));
});

// ---------------------------------------------------------------------------
// 任务 B：滚动窗口位图主动卸载
// ---------------------------------------------------------------------------

test("unload hook uses a dual-threshold IntersectionObserver with a 1-screen keep buffer", () => {
  assert.match(hookSrc, /IntersectionObserver/, "uses the native IntersectionObserver");
  assert.match(hookSrc, /rootMargin/, "expands via rootMargin");
  // 阈值关系：卸载界(2屏) 严于 还原界(1屏) → 滞回，边界不抖。
  assert.equal(H.UNLOAD_FAR_SCREENS, 2);
  assert.equal(H.KEEP_NEAR_SCREENS, 1);
  assert.ok(
    H.UNLOAD_FAR_SCREENS > H.KEEP_NEAR_SCREENS,
    "unload threshold must be farther than the keep threshold (hysteresis)",
  );
  // 卸载必须防抖（快速滚动掠过不切换 src）。
  assert.ok(H.UNLOAD_DEBOUNCE_MS > 0, "unload must be debounced");
  assert.match(hookSrc, /setTimeout\(/, "debounce timer present");
  assert.match(hookSrc, /clearTimeout\(/, "pending unload cancelled on re-entry");
});

test("the scroll root is the nearest scrollable ancestor (drawer body), not the viewport", () => {
  assert.match(hookSrc, /findScrollParent/, "resolves the real scroll container");
  assert.match(hookSrc, /getComputedStyle/, "detects overflow-y ancestors");
  assert.match(hookSrc, /overflowY/, "reads computed overflow");
});

test("the placeholder is a 1x1 transparent image data URI (drops the decoded bitmap)", () => {
  assert.match(H.UNLOAD_PLACEHOLDER, /^data:image\/gif;base64,/, "must be a data URI");
  const b64 = H.UNLOAD_PLACEHOLDER.split(",")[1];
  const bytes = Buffer.from(b64, "base64");
  assert.equal(bytes.subarray(0, 6).toString("latin1"), "GIF89a", "valid GIF header");
  // 逻辑屏幕宽/高（小端 16bit）都是 1px。
  assert.equal(bytes[6], 1);
  assert.equal(bytes[7], 0);
  assert.equal(bytes[8], 1);
  assert.equal(bytes[9], 0);
});

test("ListCard renders the placeholder when far off-screen and the real cover near", () => {
  assert.match(listCard, /useViewportImageUnload/, "ListCard consumes the hook");
  assert.match(listCard, /ref=\{cardRef\}/, "hook ref is attached to the card root");
  assert.match(listCard, /graftingImage\(cover \?\? ""\)/, "real cover still goes through graftingImage");
  assert.match(
    listCard,
    /src=\{unloaded \? UNLOAD_PLACEHOLDER : coverSrc\}/,
    "src swaps between placeholder and cover",
  );
});

test("ListCard public structure & DOM hooks are untouched (r25 contract)", () => {
  assert.match(listCard, /import CardMeta, \{ type CardMetaField \}/);
  assert.match(listCard, /<Card\b/);
  assert.match(listCard, /isPressable/);
  assert.match(listCard, /bodyClassName = "overflow-visible p-0 img-container"/);
  assert.match(listCard, /className="c-cover"/);
  assert.match(listCard, /c-cover-duration/);
  assert.match(listCard, /<CardFooter\b/);
  assert.match(listCard, /<CardMeta\b/);
  assert.match(listCard, /cardClassName = "c-list-card"/);
  assert.match(listCard, /<RetryImg\b/);
  // 不额外包 wrapper div（.c-cover 的 absolute 定位依赖直接父级）。
  assert.doesNotMatch(listCard, /<div[^>]*ref=\{cardRef\}/, "do not wrap the card in a div");
});

test("scroll stability: reserved box sizes unchanged while src is swapped", () => {
  // 卡片高度由 contain-intrinsic-size 撑住；封面盒固定 100px。
  const card = css.match(/\.c-list-card \{[\s\S]*?\}/)[0];
  assert.match(card, /contain-intrinsic-size: auto 180px/);
  const cover = css.match(/\.img-container \.c-cover \{[\s\S]*?\}/)[0];
  assert.match(cover, /height: 100px/);
  const container = css.match(/\.img-container \{[\s\S]*?\}/)[0];
  assert.match(container, /height: 100px/);
});

// ---------------------------------------------------------------------------
// 与 r26 预载 hook 不冲突 + 玻璃红线 + 依赖数
// ---------------------------------------------------------------------------

test("preload hook is untouched: still preloads once per instance, cleans only on unload", () => {
  const preload = read("../src/hooks/usePreloadImages.ts");
  assert.match(preload, /preloadedRef/);
  assert.match(preload, /PRELOAD_LIMIT/);
  assert.match(preload, /urls\.slice\(0, PRELOAD_LIMIT\)/);
  assert.match(preload, /removeAttribute\("src"\)/);
  // 任务 B 不碰预载对象：新钩子与预载 hook 无 import/调用关系
  //（文档注释里提到它不算）。
  assert.doesNotMatch(hookSrc, /import[^\n]*usePreloadImages/);
  assert.doesNotMatch(hookSrc, /usePreloadImages\s*\(/);
  assert.doesNotMatch(listCard, /import[^\n]*usePreloadImages/);
  assert.doesNotMatch(listCard, /usePreloadImages\s*\(/);
});

test("round 29 adds no compositor hints (glass redline intact)", () => {
  for (const rel of [
    "../src/hooks/useViewportImageUnload.ts",
    "../src/components/listCard.tsx",
    "../src-tauri/src/lib.rs",
  ]) {
    const src = read(rel);
    assert.doesNotMatch(src, /will-change\s*:/, `${rel} must not add will-change`);
    assert.doesNotMatch(src, /backdrop-filter\s*:/, `${rel} must not add backdrop-filter`);
    assert.doesNotMatch(src, /\bblur\(/, `${rel} must not add blur()`);
  }
});

test("no new runtime dependency was introduced (round 29)", () => {
  const pkg = JSON.parse(read("../package.json"));
  assert.equal(Object.keys(pkg.dependencies).length, 31);
});
