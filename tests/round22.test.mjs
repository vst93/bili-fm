import assert from "node:assert/strict";
import { readFileSync } from "node:fs";
import { test } from "node:test";
import vm from "node:vm";
import ts from "typescript";

// 修复轮 22：四个用户实测问题。
// 1) 动态列表播放量为 0（`Number(x) || 0` 把「缺字段」折成假数据 0）；
// 2) 三元素「文本连续展示」→ 固定位置三列布局（列宽不随内容漂移）；
// 3) 播放量字段逐列表选值（缺失时改展示弹幕数，绝不补 0）；
// 4) 内存：轮 20/21 只缩条数压不住 WebView2 进程内存 → 单条目缓存窗口 +
//    真正释放已解码位图 + 更少预加载。

const read = (rel) => readFileSync(new URL(rel, import.meta.url), "utf8");
const css = read("../src/styles/globals.css");
const indexSrc = read("../src/pages/index.tsx");

const LIST_COMPONENTS = [
  "feedList", "recommendList", "seriesList", "upVideoList",
  "collectList", "searchList", "historyList", "pageList",
];

function loadModule(rel) {
  const js = ts.transpile(read(rel), {
    module: ts.ModuleKind.CommonJS,
    target: ts.ScriptTarget.ES2022,
  });
  const exports = {};
  const context = vm.createContext({ module: { exports }, exports });
  vm.runInContext(js, context);
  return context.module.exports;
}

// ---------------------------------------------------------------------------
// 问题 1 + 3：viewsMetaField 选值与「缺字段 ≠ 0」
// ---------------------------------------------------------------------------
test("viewsMetaField shows a real 0 (missing field is not conflated with 0)", () => {
  const { viewsMetaField } = loadModule("../src/utils/string.tsx");
  const zero = viewsMetaField(0, 999)[0];
  assert.equal(zero.value, "0", "a real play count of 0 must render as 0, not be dropped");
  assert.equal(zero.icon, "play");
});

test("viewsMetaField falls back to danmaku only when play is truly missing", () => {
  const { viewsMetaField } = loadModule("../src/utils/string.tsx");
  const fallback = viewsMetaField(undefined, 1234)[0];
  assert.equal(fallback.value, "1234");
  assert.equal(fallback.icon, "danmaku", "danmaku count must not wear the play icon");
  // 播放量为 null / NaN / 空串也算缺失。
  assert.equal(viewsMetaField(null, 20)[0].value, "20");
  assert.equal(viewsMetaField(Number.NaN, 20)[0].icon, "danmaku");
  assert.equal(viewsMetaField("", 20)[0].value, "20");
  // 有真实播放量时优先播放量，不显示弹幕。
  assert.equal(viewsMetaField(5000, 999)[0].value, "5000");
});

test("viewsMetaField drops the whole field when both counts are missing", () => {
  const { viewsMetaField } = loadModule("../src/utils/string.tsx");
  for (const bad of [undefined, null, "", "x", Number.NaN]) {
    const field = viewsMetaField(bad, bad)[0];
    assert.equal(field.value, null, "must not fabricate 0");
    // CardMeta 对 null 值整字段不渲染。
    assert.match(read("../src/components/cardMeta.tsx"), /!isEmpty\(field\.value\)/);
  }
});

test("no dynamic list keeps the `Number(...) || 0` play-count bug", () => {
  for (const name of ["feedList", "upVideoList", "collectList", "recommendList"]) {
    const src = read(`../src/components/${name}.tsx`);
    assert.doesNotMatch(
      src,
      /Number\([^)]*\)\s*\|\|\s*0/,
      `${name} must not flatten a missing play count to 0`,
    );
    assert.match(src, /viewsMetaField\(/, `${name} must route views through viewsMetaField`);
    assert.match(src, /\.\.\.viewsMetaField\(/, "the field must be spread into the CardMeta fields array");
  }
});

// ---------------------------------------------------------------------------
// 问题 2：三元素固定长度 + 发布时间恒定居右（轮 23 依用户反馈修订）
// ---------------------------------------------------------------------------
test("card meta renders three fixed-length columns with pubdate pinned right", () => {
  const author = css.match(/\.card-meta-field\.is-author \{[\s\S]*?\}/);
  const views = css.match(/\.card-meta-field\.is-views \{[\s\S]*?\}/);
  const pubdate = css.match(/\.card-meta-field\.is-pubdate \{[\s\S]*?\}/);
  assert.ok(author && views && pubdate, "all three column rules must exist");
  // 三列各自定宽（flex: 0 0 Npx），长度由列决定，不随内容漂移。
  assert.match(author[0], /flex: 0 0 60px/, "author is a fixed column");
  assert.match(views[0], /flex: 0 0 \d+px/, "views is a fixed column");
  assert.match(pubdate[0], /flex: 0 0 \d+px/, "pubdate is a fixed-length column too");
  // 播放列以 margin-left:auto 把右侧两列推到底；发布时间列右对齐且右边缘贴行尾。
  assert.match(views[0], /margin-left: auto/, "views pushes the right group to the line end");
  assert.match(pubdate[0], /justify-content: flex-end/, "pubdate is right-aligned");
  // 行仍是单行不换行（旧契约保留）。
  assert.match(css.match(/\.card-meta \{[\s\S]*?\}/)[0], /flex-wrap: nowrap/);
});

// ---------------------------------------------------------------------------
// 内存 4：单条目缓存窗口 / 真正释放位图 / 更少预加载
// ---------------------------------------------------------------------------
test("drawer cache exposes a single-entry session window", () => {
  const mod = loadModule("../src/lib/drawerCache.ts");
  assert.equal(mod.DRAWER_CACHE_SINGLE_ENTRY, 1);

  mod.writeDrawerCache("feed", { items: [{ id: 1 }] });
  mod.writeDrawerCache("history", [{ id: 2 }]);
  assert.equal(mod.__drawerCacheStats().size, 2, "write alone may keep LRU-2");

  mod.retainOnlyDrawerCache("history");
  const stats = mod.__drawerCacheStats();
  assert.equal(stats.size, 1, "only the just-closed list may stay resident");
  assert.equal(stats.keys.join(","), "history");
  assert.ok(mod.readDrawerCache("feed") === null, "the other list is dropped whole");
});

test("retainOnlyDrawerCache clears everything when the key is absent", () => {
  const mod = loadModule("../src/lib/drawerCache.ts");
  mod.writeDrawerCache("feed", { items: [{ id: 1 }] });
  mod.retainOnlyDrawerCache("nope");
  assert.equal(mod.__drawerCacheStats().size, 0);
});

test("index.tsx closes every list into a single-entry window", () => {
  assert.match(indexSrc, /retainOnlyDrawerCache\(/);
  const calls = indexSrc.match(/retainOnlyDrawerCache\("(\w+)"\)/g) || [];
  const keys = calls.map((c) => c.match(/"(\w+)"/)[1]);
  for (const key of ["feed", "recommend", "collect", "upVideo", "history", "series", "danmaku"]) {
    assert.ok(keys.includes(key), `closing the ${key} drawer must collapse the cache window`);
  }
});

test("preload hook decodes fewer images and actually releases them", () => {
  const hook = read("../src/hooks/usePreloadImages.ts");
  // 轮 22：Windows 3 / 其它 2（原 6/4）。
  assert.match(hook, /PRELOAD_LIMIT/);
  assert.match(hook, /Windows"\) \? 3 : 2/, "preload budget tightened 6→3 / 4→2");
  assert.match(hook, /urls\.slice\(0, PRELOAD_LIMIT\)/);
  // 释放：blob 撤销 + 置空 + 用 1px 占位促使浏览器回收已解码位图。
  assert.match(hook, /revokeObjectURL\(/);
  assert.match(hook, /img\.src = "data:,"/);
});

test("danmaku drawer scroll restore targets the element that actually scrolls", () => {
  // DrawerBody.danmaku-drawer-body 自身 overflow:hidden 不产生 scroll 事件，
  // 真正滚动的是内层 .danmaku-scroll-area；登记错会让弹幕抽屉重开永远回顶。
  assert.match(indexSrc, /danmaku: "\.danmaku-scroll-area"/);
  assert.doesNotMatch(indexSrc, /danmaku: "\.danmaku-drawer-body"/);
});

test("CardMeta shows a danmaku icon variant and keeps the author tooltip", () => {
  const src = read("../src/components/cardMeta.tsx");
  assert.match(src, /import \{ Comment, PreviewOpen \} from "@icon-park\/react"/);
  assert.match(src, /field\.icon === "danmaku"/);
  assert.match(src, /field\.kind === "author" && typeof field\.value === "string"/);
});

// ---------------------------------------------------------------------------
// 静态契约：所有列表仍走共享 CardMeta（不回归排版）
// ---------------------------------------------------------------------------
test("every list still renders meta through the shared CardMeta", () => {
  for (const name of LIST_COMPONENTS) {
    const src = read(`../src/components/${name}.tsx`);
    assert.match(src, /<CardMeta\b/, `${name} must render <CardMeta>`);
    assert.doesNotMatch(src, /className="card-meta"/, `${name} must not hand-roll the row`);
  }
});
