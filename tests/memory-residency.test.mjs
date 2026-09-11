import assert from "node:assert/strict";
import { readFileSync } from "node:fs";
import { test } from "node:test";
import vm from "node:vm";
import ts from "typescript";

// 修复轮 20：列表缓存导致 WebView 内存膨胀。
// 覆盖：抽屉会话缓存的条目裁剪 / 全局条目预算 / LRU / TTL；
// 以及「列表运行态上限 > 缓存态上限」「卡片离屏不渲染」等驻留约束。

const read = (rel) => readFileSync(new URL(rel, import.meta.url), "utf8");

// --- 加载 lib/drawerCache.ts（可注入假时钟以测 TTL） -----------------------
const source = read("../src/lib/drawerCache.ts");
const js = ts.transpile(source, {
  module: ts.ModuleKind.CommonJS,
  target: ts.ScriptTarget.ES2022,
});

function loadCache() {
  let clock = 1_000_000;
  let exports = {};
  const context = vm.createContext({
    module: { exports },
    exports,
    Date: { now: () => clock },
  });
  vm.runInContext(js, context);
  const mod = context.module.exports;
  return {
    mod,
    advance: (ms) => {
      clock += ms;
    },
  };
}

const items = (n) => Array.from({ length: n }, (_, i) => ({ bvid: `BV${i}` }));

test("list cache entry is trimmed below the runtime retention cap", () => {
  const { mod } = loadCache();
  mod.writeDrawerCache("feed", { items: items(500), offset: "x" });
  const entry = mod.readDrawerCache("feed");
  assert.equal(entry.items.items.length, mod.LIST_CACHE_MAX);
  // 缓存态必须严格小于运行态上限，避免整份 API 响应长期驻留。
  assert.ok(mod.LIST_CACHE_MAX < 240, "cache cap must be below runtime cap 240");
});

test("array-shaped entries (collect/series/history) are trimmed too", () => {
  const { mod } = loadCache();
  mod.writeDrawerCache("collect", items(400));
  assert.equal(mod.readDrawerCache("collect").items.length, mod.LIST_CACHE_MAX);
});

test("LRU-3: a 4th distinct key evicts the oldest", () => {
  const { mod } = loadCache();
  mod.writeDrawerCache("feed", { items: items(10) });
  mod.writeDrawerCache("hot", { items: items(10) });
  mod.writeDrawerCache("series", items(10));
  mod.writeDrawerCache("collect", items(10));
  const stats = mod.__drawerCacheStats();
  assert.equal(stats.size, mod.DRAWER_CACHE_LIMIT);
  assert.ok(!stats.keys.includes("feed"), "oldest key must be evicted");
});

test("global item budget evicts whole oldest entries", () => {
  const { mod } = loadCache();
  // 3 条各 120 条 = 360 > 预算 300 → 至少淘汰最旧一条。
  mod.writeDrawerCache("feed", { items: items(200) });
  mod.writeDrawerCache("hot", { items: items(200) });
  mod.writeDrawerCache("series", items(200));
  const stats = mod.__drawerCacheStats();
  assert.ok(
    stats.totalItems <= mod.DRAWER_CACHE_TOTAL_ITEMS,
    `total cached items ${stats.totalItems} must respect budget`,
  );
  assert.ok(stats.size >= 1, "must keep at least the newest entry");
});

test("recommend payload trims both recommend and hot lists", () => {
  const { mod } = loadCache();
  mod.writeDrawerCache("recommend", {
    recommendList: { items: items(300) },
    hotList: { items: items(300) },
  });
  const entry = mod.readDrawerCache("recommend");
  assert.equal(entry.items.recommendList.items.length, mod.LIST_CACHE_MAX);
  assert.equal(entry.items.hotList.items.length, mod.LIST_CACHE_MAX);
});

test("danmaku/reply entries have their own tighter caps", () => {
  const { mod } = loadCache();
  mod.writeDrawerCache("danmaku", { items: items(900) }, {
    replyList: { items: items(900) },
    danmakuCid: 1,
  });
  const entry = mod.readDrawerCache("danmaku");
  assert.equal(entry.items.items.length, mod.DANMAKU_CACHE_MAX);
  assert.equal(entry.extra.replyList.items.length, mod.REPLY_CACHE_MAX);
  assert.ok(mod.DANMAKU_CACHE_MAX < mod.LIST_CACHE_MAX);
  assert.ok(mod.REPLY_CACHE_MAX < mod.DANMAKU_CACHE_MAX);
});

test("volatile sources expire after the TTL, stable ones do not", () => {
  const { mod, advance } = loadCache();
  mod.writeDrawerCache("collect", items(10)); // volatile
  mod.writeDrawerCache("feed", { items: items(10) }); // stable
  advance(mod.DRAWER_CACHE_TTL_MS + 1);
  assert.equal(mod.readDrawerCache("collect"), null, "volatile must expire");
  assert.ok(mod.readDrawerCache("feed"), "stable source must survive TTL");
});

test("trim does not mutate the caller's array (no cross-render aliasing)", () => {
  const { mod } = loadCache();
  const original = items(500);
  mod.writeDrawerCache("feed", { items: original });
  assert.equal(original.length, 500, "source state must be untouched");
});

// --- 静态契约：index.tsx 复用 lib，且不再自持另一份 Map ---------------------
const indexSrc = read("../src/pages/index.tsx");

test("index.tsx derives drawer cache from the shared lib module", () => {
  assert.match(
    indexSrc,
    /from "@\/lib\/drawerCache"/,
    "index must import the shared cache module",
  );
  assert.doesNotMatch(
    indexSrc,
    /const drawerCache = new Map/,
    "index must not keep a second cache Map",
  );
  assert.doesNotMatch(indexSrc, /const drawerScrollTops/, "scroll map must move out");
  // 记录滚动走导出 API。
  assert.match(indexSrc, /recordDrawerScroll\(/);
});

test(".c-list-card skips rendering off-screen (content-visibility:auto)", () => {
  const css = read("../src/styles/globals.css");
  const block = css.match(/\.c-list-card \{[\s\S]*?\}/);
  assert.ok(block, ".c-list-card rule must exist");
  assert.match(block[0], /content-visibility: auto/);
  assert.match(block[0], /contain-intrinsic-size: auto 180px/);
});

test("preload hook bounds the number of resident Image objects", () => {
  const hook = read("../src/hooks/usePreloadImages.ts");
  // 预加载只碰首屏前几张，其余交给 loading=lazy，避免一次性解码大量封面。
  assert.match(hook, /PRELOAD_LIMIT/);
  assert.match(hook, /urls\.slice\(0, PRELOAD_LIMIT\)/);
  // 卸载时必须释放引用。
  assert.match(hook, /removeAttribute\("src"\)/);
});
