import assert from "node:assert/strict";
import { readFileSync } from "node:fs";
import { test } from "node:test";
import vm from "node:vm";
import ts from "typescript";

// 修复轮 21：四个用户实测问题。
// 1) 时长角标未覆盖全部列表（收藏 / 热门 / 推荐 / 搜索缺失）；
// 2) 连续滑动后内存仍高（保留上限与缓存预算）；
// 3) 动态列表滑到上限后无法再翻页（前端保留上限造成的假死）；
// 4) 更优的列表内存缓存方案（有界滑动窗口 + 头部释放 + 更紧的缓存预算）。

const read = (rel) => readFileSync(new URL(rel, import.meta.url), "utf8");

const compile = (rel) =>
  ts.transpile(read(rel), {
    module: ts.ModuleKind.CommonJS,
    target: ts.ScriptTarget.ES2022,
  });

function loadModule(rel, extraContext = {}) {
  const js = compile(rel);
  const exports = {};
  const context = vm.createContext({
    module: { exports },
    exports,
    ...extraContext,
  });
  vm.runInContext(js, context);
  return context.module.exports;
}

const items = (n, tag = "x") =>
  Array.from({ length: n }, (_, i) => ({ id: `${tag}${i}` }));

const indexSrc = read("../src/pages/index.tsx");

const LIST_COMPONENTS = [
  "feedList", "recommendList", "seriesList", "upVideoList",
  "collectList", "searchList", "historyList", "pageList",
];

// ---------------------------------------------------------------------------
// 问题 1：时长角标覆盖全部列表
// ---------------------------------------------------------------------------
test("every list card (incl. collect / recommend / search) shows the cover duration badge", () => {
  for (const name of [...LIST_COMPONENTS]) {
    const src = read(`../src/components/${name}.tsx`);
    assert.match(
      src,
      /c-cover-duration/,
      `${name} must render the cover corner duration badge`,
    );
  }
});

test("collectList & recommendList moved duration off the meta row to the cover", () => {
  for (const name of ["collectList", "recommendList"]) {
    const src = read(`../src/components/${name}.tsx`);
    assert.doesNotMatch(
      src,
      /kind: "duration"/,
      `${name} must not keep duration in the meta row anymore`,
    );
    assert.match(src, /<span className="c-cover-duration">/);
  }
});

test("searchList badge uses the new length field, not views/date", () => {
  const src = read("../src/components/searchList.tsx");
  assert.match(src, /video\.length/);
  assert.match(src, /<span className="c-cover-duration">/);
});

test("Rust search result carries the length field end-to-end", () => {
  const rust = read("../src-tauri/src/bilibili.rs");
  assert.match(rust, /pub length: String/);
  assert.match(rust, /length: str_of\(item, "length"\)/);
  const types = read("../src/types/bilibili.ts");
  assert.match(types, /length: string/);
});

// ---------------------------------------------------------------------------
// 问题 3：动态列表（及所有分页列表）滑到上限后仍可继续翻页
// ---------------------------------------------------------------------------
test("load-more handlers no longer hard-stop at the retention cap", () => {
  // 旧实现：`length >= MAX_RETAINED… ) return;` 使分页到上限后永久假死。
  assert.doesNotMatch(
    indexSrc,
    /MAX_RETAINED_LIST_ITEMS\s*\)\s*return/,
    "index load-more must not gate on the retention cap",
  );
  assert.doesNotMatch(
    read("../src/components/seriesList.tsx"),
    /MAX_RETAINED_ITEMS\s*\)\s*return/,
    "seriesList load-more must not gate on the retention cap",
  );
  assert.doesNotMatch(
    read("../src/components/historyList.tsx"),
    /MAX_RETAINED_ITEMS\s*\)\s*return/,
    "historyList load-more must not gate on the retention cap",
  );
});

test("all paged lists use the bounded sliding window (appendWithRetention)", () => {
  for (const rel of [
    "../src/pages/index.tsx",
    "../src/components/seriesList.tsx",
    "../src/components/historyList.tsx",
  ]) {
    assert.match(read(rel), /appendWithRetention/);
  }
});

test("cache hydration re-bounds the runtime list to the retention cap", () => {
  // 缓存态更小；重开时若直接水合，运行态会一直卡在上限，分页再次假死。
  assert.match(indexSrc, /缓存态被裁到 LIST_CACHE_MAX/);
  assert.match(indexSrc, /MAX_RETAINED_LIST_ITEMS,\n\s*\)/);
});

test("head release compensates scrollTop so the viewport does not jump", () => {
  assert.match(indexSrc, /pendingAnchorAdjustRef/);
  assert.match(indexSrc, /scrollAnchorSampleRef/);
  // 补偿基于内容高度差；列表数据更新后消费。
  assert.match(indexSrc, /prevHeight - body\.scrollHeight/);
});

// --- 运行态滑动窗口行为（vm 执行真实实现） ---------------------------------
test("appendWithRetention keeps a bounded window and drops the oldest head", () => {
  const { appendWithRetention, LIST_RETENTION_CAP } =
    loadModule("../src/lib/listRetention.ts");
  assert.ok(LIST_RETENTION_CAP < 240, "round 21 tightened the runtime cap");

  const first = items(100, "a");
  const grown = appendWithRetention(first, items(100, "b"), 160);
  assert.equal(grown.length, 160);
  // 头部最旧条目被释放，尾部最新条目保留。
  assert.equal(grown[0].id, "a40");
  assert.equal(grown[grown.length - 1].id, "b99");
  // 绝不修改入参（避免与 React state 别名共享）。
  assert.equal(first.length, 100);
});

test("appendWithRetention is exact at/under the cap and handles a page bigger than the cap", () => {
  const { appendWithRetention } = loadModule("../src/lib/listRetention.ts");
  assert.equal(appendWithRetention(items(60), items(40), 100).length, 100);
  assert.equal(appendWithRetention(items(10), items(10), 100).length, 20);
  const big = appendWithRetention(items(10), items(500), 100);
  assert.equal(big.length, 100);
  assert.equal(big[big.length - 1].id, "x499");
  // undefined / null 安全。
  assert.equal(appendWithRetention(undefined, items(5), 100).length, 5);
  assert.equal(appendWithRetention(items(5), null, 100).length, 5);
});

test("runtime cap is tighter than before and above the cache cap", () => {
  const { LIST_CACHE_MAX } = loadModule("../src/lib/drawerCache.ts");
  const runtimeCap = Number(
    indexSrc.match(/const MAX_RETAINED_LIST_ITEMS = (\d+)/)[1],
  );
  assert.ok(runtimeCap < 240, "runtime cap tightened for issue 2/4");
  assert.ok(
    runtimeCap > LIST_CACHE_MAX,
    "cache cap must stay strictly below the runtime cap",
  );
});

// ---------------------------------------------------------------------------
// 问题 2 / 4：更紧的缓存预算 + 正确的条目计量
// ---------------------------------------------------------------------------
test("round 21 tightened the drawer cache budgets", () => {
  const mod = loadModule("../src/lib/drawerCache.ts");
  assert.equal(mod.DRAWER_CACHE_LIMIT, 2, "LRU-2");
  assert.ok(mod.LIST_CACHE_MAX <= 80, "per-entry cap tightened");
  assert.ok(
    mod.DRAWER_CACHE_TOTAL_ITEMS <= mod.LIST_CACHE_MAX * 2,
    "global budget ≈ two full lists",
  );
  assert.ok(mod.DANMAKU_CACHE_MAX <= 60);
  assert.ok(mod.REPLY_CACHE_MAX <= 40);
});

test("global budget counts nested lists too (recommend recommendList/hotList)", () => {
  const mod = loadModule("../src/lib/drawerCache.ts");
  mod.writeDrawerCache("recommend", {
    recommendList: { items: items(300, "r") },
    hotList: { items: items(300, "h") },
  });
  const stats = mod.__drawerCacheStats();
  // 旧实现只数顶层 items（为 0），recommend 整条不计入预算 → 预算形同虚设。
  assert.ok(
    stats.totalItems <= mod.DRAWER_CACHE_TOTAL_ITEMS,
    `nested list items must respect the budget (got ${stats.totalItems})`,
  );
});

test("LRU-2 evicts the oldest when a third key is written", () => {
  const mod = loadModule("../src/lib/drawerCache.ts");
  mod.writeDrawerCache("feed", { items: items(10) });
  mod.writeDrawerCache("hot", { items: items(10) });
  mod.writeDrawerCache("collect", items(10));
  const stats = mod.__drawerCacheStats();
  assert.equal(stats.size, mod.DRAWER_CACHE_LIMIT);
  assert.ok(!stats.keys.includes("feed"), "oldest must be evicted");
});

// ---------------------------------------------------------------------------
// 静态契约：index.tsx 复用共享模块
// ---------------------------------------------------------------------------
test("index.tsx keeps pagination ownership in the shared retention lib", () => {
  assert.match(indexSrc, /from "@\/lib\/listRetention"/);
  assert.doesNotMatch(
    indexSrc,
    /\.slice\(\s*0,\s*MAX_RETAINED_LIST_ITEMS\s*\)\s*\)\s*\);/,
    "paged append must go through appendWithRetention, not a naive head slice",
  );
});
