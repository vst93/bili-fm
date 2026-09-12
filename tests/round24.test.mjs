import assert from "node:assert/strict";
import { readFileSync } from "node:fs";
import { test } from "node:test";
import vm from "node:vm";
import ts from "typescript";

// 修复轮 24：三个方向。
// ① 列表封面默认分辨率再降（300w → 240w；一排 3 卡，用户判断封面不用特别大）；
// ② 卡片信息行空间利用：UP 主名不再在应用层提前截断（CSS ellipsis 按真实像素
//    宽度决定），作者列占满剩余空间，右侧字段为固定宽度档位、末字段（发布时间）
//    靠右贴行尾，只有超限才省略；
// ③ 交互存储/内存：defensive 运行态上界（搜索结果），并回归确认零功能影响。

const read = (rel) => readFileSync(new URL(rel, import.meta.url), "utf8");
const css = read("../src/styles/globals.css");
const indexSrc = read("../src/pages/index.tsx");
const utilSrc = read("../src/utils/string.tsx");

const LIST_COMPONENTS = [
  "feedList",
  "recommendList",
  "seriesList",
  "upVideoList",
  "collectList",
  "searchList",
  "historyList",
  "pageList",
];

function loadModule(rel) {
  const js = ts.transpile(read(rel), {
    module: ts.ModuleKind.CommonJS,
    target: ts.ScriptTarget.ES2022,
  });
  const exports = {};
  const context = vm.createContext({ module: { exports }, exports, URL });
  vm.runInContext(js, context);
  return context.module.exports;
}

// ---------------------------------------------------------------------------
// 方向 ①：列表封面默认宽度再降
// ---------------------------------------------------------------------------
test("graftingImage default width drops 300w → 240w (round 24)", () => {
  const { graftingImage } = loadModule("../src/utils/string.tsx");
  const decoded = decodeURIComponent(
    graftingImage("https://i0.hdslb.com/bfs/archive/abc123.jpg"),
  );
  assert.match(decoded, /@240w\.webp/, "list covers must request the 240w variant");
  assert.doesNotMatch(decoded, /@300w\.webp/, "the round-23 300w default must be gone");
  assert.doesNotMatch(decoded, /@400w\.webp/, "the old 400w default must be gone");
});

test("explicit widths (player/ambient/playlist/avatar) are unaffected by the new default", () => {
  const { graftingImage } = loadModule("../src/utils/string.tsx");
  const url = "https://i0.hdslb.com/bfs/archive/abc123.jpg";
  // 主播放器封面 / 背景光场 / 歌单缩略图 / 头像各自的显式宽度仍生效。
  for (const w of [480, 320, 192, 96]) {
    assert.match(
      decodeURIComponent(graftingImage(url, w)),
      new RegExp(`@${w}w\\.webp`),
      `explicit ${w}w must win over the default`,
    );
  }
  // 调用点仍保留显式宽度（防止后续误改成吃默认值）。
  const callsites = [indexSrc, read("../src/components/playlist.tsx"), read("../src/components/danmakuList.tsx")];
  assert.ok(callsites.some((s) => /graftingImage\([^)]*,\s*480\s*,?\s*\)/.test(s)), "player cover keeps 480w");
  assert.ok(callsites.some((s) => /graftingImage\([^)]*,\s*320\s*,?\s*\)/.test(s)), "ambient cover keeps 320w");
  assert.ok(callsites.some((s) => /graftingImage\([^)]*,\s*192\s*,?\s*\)/.test(s)), "playlist thumbnail keeps 192w");
  assert.ok(callsites.some((s) => /graftingImage\([^)]*,\s*96\s*,?\s*\)/.test(s)), "avatars keep 96w");
});

test("all list-card covers still use the (new) default width", () => {
  for (const name of LIST_COMPONENTS) {
    const src = read(`../src/components/${name}.tsx`);
    assert.match(
      src,
      /src=\{graftingImage\([^)]*\)\}/,
      `${name} cover must flow through the default graftingImage width`,
    );
  }
});

// ---------------------------------------------------------------------------
// 方向 ②：信息行空间利用（作者不提前截断 / 档位定宽 / 末字段靠右）
// ---------------------------------------------------------------------------
test("author names are no longer pre-truncated before reaching CardMeta", () => {
  // 移除应用层 subStr(name, 7)：三个曾截断的列表改传完整名字。
  for (const name of ["feedList", "recommendList", "upVideoList"]) {
    const src = read(`../src/components/${name}.tsx`);
    assert.doesNotMatch(src, /\bsubStr\b/, `${name} must not import/call subStr any more`);
  }
  // 工具函数本体也删掉，避免有人再按字符数提前砍。
  assert.doesNotMatch(utilSrc, /export const subStr/, "subStr export must be removed");
});

test("author is a flexible column that absorbs leftover width and ellipsizes only on overflow", () => {
  const author = css.match(/\.card-meta-field\.is-author \{[\s\S]*?\}/)[0];
  // 不再是被钉死的窄定宽列（那就是「中间留白 + 名字被提前缩写」的根因）。
  assert.doesNotMatch(author, /flex: 0 0 \d+px/, "author must not stay a narrow fixed column");
  assert.match(author, /flex: 1 1 auto/, "author must absorb the leftover width");
  assert.match(author, /overflow: hidden/, "author overflows are clipped");
  assert.match(author, /text-overflow: ellipsis/, "author truncates only when it truly overflows");
  // 作者列吃满剩余空间：有作者的行不再有中间空白（margin-left:auto 仅在无作者行生效）。
});

test("views/pubdate are fixed-size tiers and the last column (pubdate) hugs the line end", () => {
  const views = css.match(/\.card-meta-field\.is-views \{[\s\S]*?\}/)[0];
  const pubdate = css.match(/\.card-meta-field\.is-pubdate \{[\s\S]*?\}/)[0];
  assert.match(views, /flex: 0 0 56px/, "views is a fixed 56px tier");
  assert.match(views, /justify-content: flex-end/, "views content is right-aligned in its tier");
  assert.match(pubdate, /flex: 0 0 62px/, "pubdate is a fixed 62px tier");
  assert.match(pubdate, /justify-content: flex-end/, "pubdate content hugs the right edge (line end)");
});

test("meta row remains a single non-wrapping line with priority-based hiding", () => {
  const row = css.match(/\.card-meta \{[\s\S]*?\}/)[0];
  assert.match(row, /flex-wrap: nowrap/, "meta must never wrap");
  assert.match(row, /overflow: hidden/, "overflow must be clipped, not wrapped");
  const rules = [
    ...css.matchAll(
      /@container \(max-width: (\d+)px\) \{\s*\.card-meta-field\.(is-\w+) \{\s*display: none;/g,
    ),
  ].map((m) => ({ max: Number(m[1]), cls: m[2] }));
  assert.deepEqual(
    rules.map((r) => r.cls),
    ["is-extra", "is-duration", "is-pubdate", "is-views"],
    "fields must still hide from lowest to highest priority",
  );
  for (let i = 1; i < rules.length; i += 1) {
    assert.ok(rules[i].max < rules[i - 1].max, "hiding thresholds must shrink with priority");
  }
});

test("every list still renders meta through the shared CardMeta (no hand-rolled row)", () => {
  for (const name of LIST_COMPONENTS) {
    const src = read(`../src/components/${name}.tsx`);
    assert.match(src, /<CardMeta\b/, `${name} must render <CardMeta>`);
    assert.doesNotMatch(src, /className="card-meta"/, `${name} must not hand-roll the meta row`);
  }
});

// ---------------------------------------------------------------------------
// 方向 ③：交互存储 / 内存
// ---------------------------------------------------------------------------
test("search results get a defensive runtime bound (was unbounded)", () => {
  // 两个写入点（新搜索 / 换排序）都必须上界，且上限复用列表运行态常量。
  assert.match(
    indexSrc,
    /const results = await invoke<BL\.SearchResult\[\]>\("search_video"/,
    "search still hits the same Rust command",
  );
  const bounded = indexSrc.match(/setSearchResults\(results\.slice\(0, MAX_RETAINED_LIST_ITEMS\)\)/g) || [];
  assert.equal(bounded.length, 2, "both search paths must bound the result set");
  // 没有残留的无界写入。
  assert.doesNotMatch(indexSrc, /setSearchResults\(results\)/, "no unbounded search write may remain");
  // 上限本身存在且远大于单页 50 条（正常情况不截断，零显示影响）。
  const cap = Number(indexSrc.match(/MAX_RETAINED_LIST_ITEMS = (\d+)/)[1]);
  assert.ok(cap >= 50, "runtime cap must not truncate a normal 50-item search page");
});

test("storage surfaces that are intentionally kept unbounded are documented as such", () => {
  // 用户歌单（Tauri store）是用户数据，不得裁剪 —— 仍走 store.set 持久化。
  assert.match(indexSrc, /playlistStoreRef/, "playlist is still persisted via the Tauri store");
  assert.match(indexSrc, /load\("playlist\.json"/, "playlist.json store is untouched");
  // 本地断点仍是 LRU-50（不放大也不缩小，行为不变）。
  const player = read("../src/components/player.tsx");
  assert.match(player, /LOCAL_RESUME_MAX_ENTRIES = 50/);
  assert.match(player, /LOCAL_RESUME_STORAGE_KEY = "localResumePoints"/);
});

test("drawer session cache stays a bounded single-entry window (no regression)", () => {
  const cache = loadModule("../src/lib/drawerCache.ts");
  assert.equal(cache.DRAWER_CACHE_SINGLE_ENTRY, 1);
  assert.ok(cache.LIST_CACHE_MAX <= 80, "per-entry cap must stay tight");
  assert.ok(cache.DRAWER_CACHE_TOTAL_ITEMS <= 160, "global item budget must stay tight");
  assert.ok(cache.DANMAKU_CACHE_MAX <= 60 && cache.REPLY_CACHE_MAX <= 40);
});

test("glass redline intact: no will-change / backdrop-filter / blur added to the touched files", () => {
  const touchedText = [utilSrc, read("../src/components/cardMeta.tsx"), indexSrc];
  for (const src of touchedText) {
    assert.doesNotMatch(src, /will-change\s*:/);
    assert.doesNotMatch(src, /backdrop-filter\s*:/);
  }
  // card meta 布局段不得新增合成层提示 / 模糊。
  const metaBlock = css.slice(css.indexOf(".card-meta {"), css.indexOf(".c-list-card.c-list-card-row"));
  assert.doesNotMatch(metaBlock, /will-change\s*:/);
  assert.doesNotMatch(metaBlock, /backdrop-filter\s*:/);
  assert.doesNotMatch(metaBlock, /\bblur\(/);
});

test("no new runtime dependency was introduced", () => {
  const pkg = JSON.parse(read("../package.json"));
  // 轮 24 只改代码/样式，依赖集合不变（数量与关键项固定）。
  assert.equal(Object.keys(pkg.dependencies).length, 31);
  for (const dep of ["react", "@tauri-apps/api", "@tauri-apps/plugin-store", "@icon-park/react"]) {
    assert.ok(dep in pkg.dependencies, `${dep} must remain a declared dependency`);
  }
});
