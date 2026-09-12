import assert from "node:assert/strict";
import { readFileSync } from "node:fs";
import { test } from "node:test";
import vm from "node:vm";
import ts from "typescript";

// 修复轮 23：三个方向。
// 1) 内存继续压：会话级 sponsor 缓存无上界 + 全新弹幕拉取未上界；
// 2) 列表页封面分辨率再降（图片代理默认宽度 400w → 300w）；
// 3) 卡片底部信息行：三元素固定长度 + 发布时间恒定居右对齐。

const read = (rel) => readFileSync(new URL(rel, import.meta.url), "utf8");
const css = read("../src/styles/globals.css");
const indexSrc = read("../src/pages/index.tsx");

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
// 问题 2：列表封面分辨率再降低
// ---------------------------------------------------------------------------
test("graftingImage's default width is lowered (list covers are downsampled harder)", () => {
  const { graftingImage } = loadModule("../src/utils/string.tsx");
  const url = "https://i0.hdslb.com/bfs/archive/abc123.jpg";
  const out = graftingImage(url);
  const decoded = decodeURIComponent(out);
  assert.match(decoded, /@300w\.webp/, "default list cover must request the 300w variant");
  assert.doesNotMatch(decoded, /@400w\.webp/, "the old 400w default must be gone");
});

test("explicit widths still win (player/avatar/thumbnail keep their own resolution)", () => {
  const { graftingImage } = loadModule("../src/utils/string.tsx");
  const url = "https://i0.hdslb.com/bfs/archive/abc123.jpg";
  assert.match(decodeURIComponent(graftingImage(url, 480)), /@480w\.webp/);
  assert.match(decodeURIComponent(graftingImage(url, 96)), /@96w\.webp/);
});

test("already-sized hdslb URLs are left untouched (no double @suffix)", () => {
  const { graftingImage } = loadModule("../src/utils/string.tsx");
  const sized = "https://i0.hdslb.com/bfs/archive/abc.jpg@480w.webp";
  assert.match(decodeURIComponent(graftingImage(sized)), /abc\.jpg@480w\.webp/);
  assert.doesNotMatch(decodeURIComponent(graftingImage(sized)), /@300w/);
});

test("every list card cover flows through graftingImage with the default width", () => {
  // 至少这些列表卡片的 <RetryImg src> 不再传显式宽度（吃默认 300w）。
  const LIST = ["feedList", "recommendList", "seriesList", "upVideoList", "collectList", "searchList", "historyList"];
  for (const name of LIST) {
    const src = read(`../src/components/${name}.tsx`);
    assert.match(src, /src=\{graftingImage\([^)]*\)\}/, `${name} cover must use the default graftingImage width`);
  }
});

// ---------------------------------------------------------------------------
// 问题 3：三列固定长度 + 发布时间持续靠右对齐
// ---------------------------------------------------------------------------
test("pubdate column is a fixed length, right-aligned, and pinned to the line end", () => {
  const author = css.match(/\.card-meta-field\.is-author \{[\s\S]*?\}/)[0];
  const views = css.match(/\.card-meta-field\.is-views \{[\s\S]*?\}/)[0];
  const pubdate = css.match(/\.card-meta-field\.is-pubdate \{[\s\S]*?\}/)[0];
  // 三元素各自固定列宽（不再有 flex: 1 1 auto 的「吸收剩余」列）。
  assert.match(author, /flex: 0 0 \d+px/);
  assert.match(views, /flex: 0 0 \d+px/);
  assert.match(pubdate, /flex: 0 0 \d+px/, "pubdate must be a fixed-length column");
  // 播放列 margin-left:auto 把右侧两列推到底部 → 发布时间右边缘恒定贴行尾。
  assert.match(views, /margin-left: auto/);
  assert.match(pubdate, /justify-content: flex-end/, "pubdate content hugs the right edge");
});

test("meta row still never wraps and still hides fields by priority when narrow", () => {
  const row = css.match(/\.card-meta \{[\s\S]*?\}/)[0];
  assert.match(row, /flex-wrap: nowrap/);
  assert.match(row, /overflow: hidden/);
  const rules = [...css.matchAll(/@container \(max-width: (\d+)px\) \{\s*\.card-meta-field\.(is-\w+) \{\s*display: none;/g)]
    .map((m) => ({ max: Number(m[1]), cls: m[2] }));
  assert.deepEqual(
    rules.map((r) => r.cls),
    ["is-extra", "is-duration", "is-pubdate", "is-views"],
    "priority hiding order must be preserved",
  );
});

test("no card meta field can introduce a line break or an unsized gap column", () => {
  // 行内不再有 flex-grow（会随卡片宽度变化，导致右对齐失效）。
  const pubdate = css.match(/\.card-meta-field\.is-pubdate \{[\s\S]*?\}/)[0];
  assert.doesNotMatch(pubdate, /flex:\s*1\s+1/, "pubdate must not absorb slack (kills right alignment)");
  assert.match(css.match(/\.card-meta-field \{[\s\S]*?\}/)[0], /white-space: nowrap/);
});

// ---------------------------------------------------------------------------
// 问题 1：内存继续压
// ---------------------------------------------------------------------------
test("the session-level sponsor cache is bounded (was unbounded, grew for the whole session)", () => {
  const src = read("../src/lib/sponsorBlock.ts");
  assert.match(src, /SPONSOR_CACHE_MAX\s*=\s*\d+/, "a cache cap must exist");
  const cap = Number(src.match(/SPONSOR_CACHE_MAX\s*=\s*(\d+)/)[1]);
  assert.ok(cap > 0 && cap <= 256, "cap must be a sane, small number");
  // 写入时按 LRU 淘汰最旧（Map 头部）。
  assert.match(src, /while \(cache\.size > SPONSOR_CACHE_MAX\)/);
  assert.match(src, /cache\.delete\(oldest\)/);
  // 不再只有一句裸 cache.set。
  assert.match(src, /cache\.delete\(key\);\n\s*cache\.set\(key, outcome\.segments\)/);
});

test("a fresh danmaku fetch is bounded to MAX_RETAINED_DANMAKU (only hydration was sliced before)", () => {
  // 全新拉取路径不能把整份弹幕 data 直接塞进 state。
  assert.match(indexSrc, /items: \(data\?\.items \|\| \[\]\)\.slice\(0, MAX_RETAINED_DANMAKU\)/);
  // 缓存水合路径仍使用同一上限（语义一致）。
  assert.match(indexSrc, /\.slice\(\s*0,\s*MAX_RETAINED_DANMAKU,?\s*\)/);
  // 上限确实存在且为正。
  assert.match(indexSrc, /MAX_RETAINED_DANMAKU = \d+/);
});

test("no changed surface added a compositor-hint or blur (glass redline intact)", () => {
  // 本轮改动的 TS 文件不得引入 will-change / backdrop-filter / blur。
  for (const rel of [
    "../src/components/cardMeta.tsx",
    "../src/utils/string.tsx",
    "../src/lib/sponsorBlock.ts",
  ]) {
    const src = read(rel);
    assert.doesNotMatch(src, /will-change\s*:/, `${rel} must not add will-change`);
    assert.doesNotMatch(src, /backdrop-filter\s*:/, `${rel} must not add backdrop-filter`);
  }
  // card meta 布局段也不得新增合成层提示 / 模糊。
  const metaBlock = css.slice(css.indexOf(".card-meta {"), css.indexOf(".c-list-card.c-list-card-row"));
  assert.doesNotMatch(metaBlock, /will-change\s*:/);
  assert.doesNotMatch(metaBlock, /backdrop-filter\s*:/);
  assert.doesNotMatch(metaBlock, /\bblur\(/);
});
