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
  // 轮 24：默认宽度由 300w 再降到 240w（用户：「一排 3 卡，封面不用特别大」）。
  assert.match(decoded, /@240w\.webp/, "default list cover must request the 240w variant");
  assert.doesNotMatch(decoded, /@300w\.webp/, "the old 300w default must be gone");
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
  assert.doesNotMatch(decodeURIComponent(graftingImage(sized)), /@240w/);
});

test("every list card cover flows through graftingImage with the default width", () => {
  // 至少这些列表卡片的 <RetryImg src> 不再传显式宽度（吃默认 240w）。
  const LIST = ["feedList", "recommendList", "seriesList", "upVideoList", "collectList", "searchList", "historyList"];
  for (const name of LIST) {
    const src = read(`../src/components/${name}.tsx`);
    assert.match(src, /src=\{graftingImage\([^)]*\)\}/, `${name} cover must use the default graftingImage width`);
  }
});

// ---------------------------------------------------------------------------
// 问题 3：三列固定长度 + 发布时间持续靠右对齐
// ---------------------------------------------------------------------------
test("pubdate column is a fixed tier, right-aligned, and pinned to the line end", () => {
  const author = css.match(/\.card-meta-field\.is-author \{[\s\S]*?\}/)[0];
  const views = css.match(/\.card-meta-field\.is-views \{[\s\S]*?\}/)[0];
  const pubdate = css.match(/\.card-meta-field\.is-pubdate \{[\s\S]*?\}/)[0];
  // 轮 24：作者列改为可伸缩的「占余」列（不再是定宽 60px，也不再留中间空白）。
  assert.doesNotMatch(author, /flex: 0 0 \d+px/, "author must not stay a narrow fixed column");
  assert.match(author, /flex: 1 1 auto/, "author absorbs the leftover width");
  assert.match(views, /flex: 0 0 \d+px/, "views is a fixed tier");
  assert.match(pubdate, /flex: 0 0 \d+px/, "pubdate must be a fixed-length column");
  // 发布时间右对齐贴行尾。
  assert.match(pubdate, /justify-content: flex-end/, "pubdate content hugs the right edge");
});

test("author names are no longer pre-truncated in the app layer (subStr removed)", () => {
  // 轮 24：feedList/recommendList/upVideoList 不再用 subStr(name, 7) 把中文名提前砍短。
  for (const name of ["feedList", "recommendList", "upVideoList"]) {
    const src = read(`../src/components/${name}.tsx`);
    assert.doesNotMatch(src, /subStr\(/, `${name} must not pre-truncate the author name`);
    assert.doesNotMatch(src, /subStr/, `${name} must not import subStr either`);
  }
  // 工具函数也已移除（截断改由 CSS ellipsis 按真实像素宽度决定）。
  const util = read("../src/utils/string.tsx");
  assert.doesNotMatch(util, /export const subStr/, "subStr export must be removed");
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
  // 右侧定宽组必须仍靠右（末列右边缘贴行尾）；作者列不得引入换行。
  const views = css.match(/\.card-meta-field\.is-views \{[\s\S]*?\}/)[0];
  assert.match(views, /flex: 0 0 \d+px/, "views must stay a fixed-size column for stable right alignment");
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
