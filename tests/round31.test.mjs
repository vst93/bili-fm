import assert from "node:assert/strict";
import { readFileSync } from "node:fs";
import { test } from "node:test";
import vm from "node:vm";
import ts from "typescript";

// 修复轮 31：内存治理第四轮（用户实测 2.0.24 后仍有压力），逐项评估用户四方向：
//  ① 列表与 DOM：content-visibility 已在轮 25 落地；虚拟滚动评估后不采纳（见报告），
//     本测试锁定「未把 grid 列表改造成虚拟列表」这一决策的可见依据（结构未变）。
//  ② 图片与内存：把「首屏预热」从「一次性全部置 src」改成**有界并发**（在途
//     解码位图数量封顶），并把网格卡片已有的离屏位图卸载扩展到网页里最后一块
//     长列表缩略图区域 —— 播放列表行（PlaylistRow）。
//  ③ Tauri/WebView2：窗口失焦 / 关闭到托盘时把 WebView2 内存目标等级降到 LOW，
//     重新聚焦恢复 NORMAL（ICoreWebView2_19::SetMemoryUsageTargetLevel）。
//  ④ 排查与验收：事件监听 / 定时器 / 全局缓存审计（见报告），无泄漏点。

const read = (rel) => readFileSync(new URL(rel, import.meta.url), "utf8");

const preloadSrc = read("../src/hooks/usePreloadImages.ts");
const playlist = read("../src/components/playlist.tsx");
const listCard = read("../src/components/listCard.tsx");
const libRs = read("../src-tauri/src/lib.rs");
const cargoToml = read("../src-tauri/Cargo.toml");
const css = read("../src/styles/globals.css");

// ---------------------------------------------------------------------------
// 方向②：预热有界并发（在途解码位图数量封顶）
// ---------------------------------------------------------------------------

/**
 * 用假 React + 假 Image 真实执行 usePreloadImages，观察并发行为。
 * useRef/useEffect 的返回值都拿得到：直接调用捕获到的 effect。
 */
function runPreload(urls, { windows = true } = {}) {
  const js = ts.transpile(preloadSrc, {
    module: ts.ModuleKind.CommonJS,
    target: ts.ScriptTarget.ES2022,
  });

  let effect = null;
  const reactStub = {
    useEffect: (fn) => {
      effect = fn;
    },
    useRef: (init) => ({ current: init }),
  };

  const imgs = [];
  class FakeImage {
    constructor() {
      this._src = "";
      this.onload = null;
      this.onerror = null;
      imgs.push(this);
    }
    set src(value) {
      this._src = value;
    }
    get src() {
      return this._src;
    }
    removeAttribute() {
      this._src = "";
    }
  }

  const exports = {};
  const context = vm.createContext({
    module: { exports },
    exports,
    require: (name) => {
      if (name === "react") return reactStub;
      throw new Error(`unexpected require(${name})`);
    },
    Image: FakeImage,
    navigator: { userAgent: windows ? "Windows" : "Linux" },
    URL: { revokeObjectURL: () => {} },
  });
  vm.runInContext(js, context);

  // 必须先调用 hook 才会注册 effect，再执行 effect。
  context.module.exports.usePreloadImages(urls);
  const cleanup = effect ? effect() : undefined;
  const module = context.module.exports;
  const inFlight = () => imgs.filter((img) => img.src && img.src !== "data:," && !img._done).length;
  const settle = (img) => {
    img._done = true;
    img.onload?.();
  };
  return { module, imgs, inFlight, settle, cleanup };
}

test("preload caps in-flight decodes at PRELOAD_CONCURRENCY (bounded concurrency)", () => {
  const h = runPreload(["u1", "u2", "u3", "u4", "u5", "u6"]);
  assert.equal(h.module.PRELOAD_CONCURRENCY, 2, "concurrency constant exported and equals 2");
  // 首波只发起 PRELOAD_CONCURRENCY 张（不是一次性 3 张全发）。
  assert.equal(h.inFlight(), 2, "only PRELOAD_CONCURRENCY images in flight after the first pump");
  // 完成一张 → 立即补一张；仍在途数仍被并发上限约束。
  h.settle(h.imgs[0]);
  assert.ok(h.inFlight() <= 2, "in-flight count never exceeds the cap while refilling");
});

test("preload still bounds the budget to the first screen only", () => {
  // Windows 3 / 其它 2（轮 22 契约不变）。
  assert.match(preloadSrc, /PRELOAD_LIMIT/);
  assert.match(preloadSrc, /Windows"\) \? 3 : 2/);
  assert.match(preloadSrc, /urls\.slice\(0, PRELOAD_LIMIT\)/);
  // 有界并发实现：游标 + 在途计数 + 补位 pump。
  assert.match(preloadSrc, /let active = 0/);
  assert.match(preloadSrc, /active < PRELOAD_CONCURRENCY/);
  assert.match(preloadSrc, /const pump = \(\) =>/);
});

test("preload still preloads once per mount and releases everything on unmount", () => {
  assert.match(preloadSrc, /const preloadedRef = useRef\(false\)/);
  assert.match(preloadSrc, /if \(preloadedRef\.current\) return;/);
  assert.match(preloadSrc, /preloadedRef\.current = true/);
  assert.match(preloadSrc, /revokeObjectURL\(/);
  assert.match(preloadSrc, /removeAttribute\("src"\)/);
  assert.match(preloadSrc, /img\.src = "data:,"/);

  // 真正执行 cleanup：所有预热对象都被置空、置 1px 占位。
  const h = runPreload(["u1", "u2", "u3"]);
  h.settle(h.imgs[0]);
  h.cleanup?.();
  for (const img of h.imgs) {
    assert.equal(img.src, "data:,", "every preloaded Image is released to a data: placeholder");
  }
});

// ---------------------------------------------------------------------------
// 方向②：播放列表行的离屏位图卸载（补齐最后一块长列表缩略图区域）
// ---------------------------------------------------------------------------

test("playlist rows evict decoded thumbnails off-screen via the shared hook", () => {
  assert.match(playlist, /useViewportImageUnload/);
  assert.match(playlist, /UNLOAD_PLACEHOLDER/);
  assert.match(
    playlist,
    /src=\{unloaded \? UNLOAD_PLACEHOLDER : coverSrc\}/,
    "playlist thumbnail swaps between placeholder and cover",
  );
  // 192w 显式宽度契约（轮 24）不变 —— 只是换了个组件承载。
  assert.match(
    playlist,
    /graftingImage\(item\.first_frame \|\| item\.pic, 192\)/,
    "playlist thumbnail keeps the 192w request",
  );
});

test("PlaylistRow keeps the c-list-card-row contract and drag/select handlers", () => {
  // 行根仍是 .c-list-card.c-list-card-row（content-visibility + contain-intrinsic-size 依赖）。
  assert.match(playlist, /c-list-card c-list-card-row/);
  assert.match(playlist, /const PlaylistRow: FC<PlaylistRowProps>/);
  // 拖拽 / 选中 / 删除语义原样保留。
  assert.match(playlist, /onDragStart=\{isSeriesPlaylist \? undefined : \(\) => onDragStart\?\.\(index\)\}/);
  assert.match(playlist, /onDelete\?\.\(item\.id\)/);
  assert.match(playlist, /onSelect\(index\)/);
  // 合集 tab 仍不可拖拽。
  assert.match(playlist, /draggable=\{!isSeriesPlaylist\}/);
});

// ---------------------------------------------------------------------------
// 方向①：网格列表没有被改造成虚拟列表（结构未变，决策可见）
// ---------------------------------------------------------------------------

test("grid lists keep the plain responsive grid (no virtualizer introduced)", () => {
  // 8 个 grid 列表仍是 `grid grid-cols-2 sm:grid-cols-3` + map，全部保持挂载。
  for (const name of [
    "feedList",
    "recommendList",
    "seriesList",
    "upVideoList",
    "collectList",
    "searchList",
    "historyList",
    "pageList",
  ]) {
    const src = read(`../src/components/${name}.tsx`);
    assert.match(src, /grid-cols-2 sm:grid-cols-3/, `${name} keeps the responsive grid`);
    assert.doesNotMatch(src, /useVirtualizer/, `${name} must not be virtualized`);
  }
  // ListCard 仍按内容自适应（content-visibility 让它屏外不渲染）。
  assert.match(listCard, /cardClassName = "c-list-card"/);
  const cardRule = css.match(/\.c-list-card \{[\s\S]*?\}/)[0];
  assert.match(cardRule, /content-visibility: auto/);
});

// ---------------------------------------------------------------------------
// 方向③：WebView2 内存目标等级（失焦/最小化降低，聚焦恢复）
// ---------------------------------------------------------------------------

test("window focus loss / close-to-tray lowers the WebView2 memory target level", () => {
  // 触发点：失焦 + 关闭到托盘 → low；重新聚焦 → normal。
  assert.match(libRs, /WindowEvent::Focused\(false\) \| WindowEvent::CloseRequested/);
  assert.match(libRs, /set_webview_memory_usage_target\(window\.app_handle\(\), true\)/);
  assert.match(libRs, /WindowEvent::Focused\(true\)/);
  assert.match(libRs, /set_webview_memory_usage_target\(window\.app_handle\(\), false\)/);
});

test("memory target mapping uses ICoreWebView2_19 and degrades silently", () => {
  assert.match(libRs, /ICoreWebView2_19/);
  assert.match(libRs, /SetMemoryUsageTargetLevel/);
  assert.match(libRs, /COREWEBVIEW2_MEMORY_USAGE_TARGET_LEVEL_LOW/);
  assert.match(libRs, /COREWEBVIEW2_MEMORY_USAGE_TARGET_LEVEL_NORMAL/);
  // 旧 Runtime 取不到接口时必须静默返回（`let Ok(..) else { return }`），不 panic。
  assert.match(libRs, /let Ok\(webview2\) = core\.cast::<ICoreWebView2_19>\(\) else \{/);
  // 仅在 Windows 目标编译该函数。
  assert.match(libRs, /#\[cfg\(target_os = "windows"\)\]\s*\nfn set_webview_memory_usage_target/);
});

test("Windows-only WebView2 crates are declared for the windows target", () => {
  assert.match(cargoToml, /\[target\."cfg\(windows\)"\.dependencies\]/);
  assert.match(cargoToml, /webview2-com/);
  assert.match(cargoToml, /windows-core/);
});

test("r29 WebView2 launch args are untouched by round 31", () => {
  assert.match(libRs, /WEBVIEW2_ADDITIONAL_BROWSER_ARGUMENTS/);
  assert.match(libRs, /--js-flags=--scavenger_max_new_space_capacity_mb=8/);
  assert.match(libRs, /msWebOOUI,msPdfOOUI,msSmartScreenProtection/);
});

// ---------------------------------------------------------------------------
// 方向④：验收 —— 没有引入新的全局泄漏面（缓存有界 + 无新增全局 Map）
// ---------------------------------------------------------------------------

test("session caches stay bounded (audit: no unbounded global accumulation)", () => {
  const sponsor = read("../src/lib/sponsorBlock.ts");
  assert.match(sponsor, /SPONSOR_CACHE_MAX/);
  assert.match(sponsor, /while \(cache\.size > SPONSOR_CACHE_MAX\)/);
  const drawer = read("../src/lib/drawerCache.ts");
  assert.match(drawer, /DRAWER_CACHE_TOTAL_ITEMS/);
  assert.match(drawer, /LIST_CACHE_MAX/);
});

test("round 31 adds no unbounded module-level cache in the touched files", () => {
  // 本轮唯一新增的模块级导出是并发常量，不是缓存。
  assert.match(preloadSrc, /export const PRELOAD_CONCURRENCY = 2;/);
  assert.doesNotMatch(preloadSrc, /new Map\(\)/);
  assert.doesNotMatch(playlist, /new Map\(\)/);
});

// ---------------------------------------------------------------------------
// 玻璃红线 + 冻结项 + 依赖数
// ---------------------------------------------------------------------------

test("round 31 adds no compositor hints (glass redline intact)", () => {
  for (const rel of [
    "../src/hooks/usePreloadImages.ts",
    "../src/components/playlist.tsx",
    "../src-tauri/src/lib.rs",
  ]) {
    const src = read(rel);
    assert.doesNotMatch(src, /will-change\s*:/, `${rel} must not add will-change`);
    assert.doesNotMatch(src, /backdrop-filter\s*:/, `${rel} must not add backdrop-filter`);
    assert.doesNotMatch(src, /\bblur\(/, `${rel} must not add blur()`);
  }
});

test("cover width default is frozen (240w) — round 31 must not touch it", () => {
  const js = ts.transpile(read("../src/utils/string.tsx"), {
    module: ts.ModuleKind.CommonJS,
    target: ts.ScriptTarget.ES2022,
  });
  const exports = {};
  const context = vm.createContext({ module: { exports }, exports, URL });
  vm.runInContext(js, context);
  const { graftingImage } = context.module.exports;
  const decoded = decodeURIComponent(
    graftingImage("https://i0.hdslb.com/bfs/archive/abc123.jpg"),
  );
  assert.match(decoded, /@240w\.webp/, "list cover default stays 240w");
});

test("no new runtime dependency was introduced (round 31)", () => {
  const pkg = JSON.parse(read("../package.json"));
  assert.equal(Object.keys(pkg.dependencies).length, 31);
});
