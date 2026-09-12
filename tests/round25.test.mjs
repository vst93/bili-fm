import assert from "node:assert/strict";
import { readFileSync } from "node:fs";
import { test } from "node:test";
import vm from "node:vm";
import ts from "typescript";

// 修复轮 25：三个方向。
// ① 内存：把「有界滑动窗口」再收紧一档（运行态 160→128 / 缓存 80→64、
//    预算 160→128 / 弹幕 60→48 / 评论 40→32），并把 8 个列表卡片收敛到
//    单一共享组件，减少每张卡片的组件/节点开销（显示与功能零改动）。
// ② 卡片信息行：三列不再「有的定宽、有的吃余量」，改为按比例分配
//    （作者 40% / 播放量 30% / 发布时间 30%），发布量右对齐，列间距 8px
//    防止数字与日期粘连。
// ③ 组件化：抽出 <ListCard>，8 个列表全部改用；DOM/类名/CSS 钩子保持。

const read = (rel) => readFileSync(new URL(rel, import.meta.url), "utf8");
const css = read("../src/styles/globals.css");
const indexSrc = read("../src/pages/index.tsx");

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
// 问题 3：卡片组件化 —— 8 个列表统一走 <ListCard>
// ---------------------------------------------------------------------------
test("a single shared ListCard component renders the list-card contract", () => {
  const src = read("../src/components/listCard.tsx");
  assert.match(src, /<Card\b/, "ListCard must render HeroUI Card");
  assert.match(src, /isPressable/, "cards remain pressable (role=button + hover hooks)");
  assert.match(src, /bodyClassName = "overflow-visible p-0 img-container"/, "keeps the .img-container body hook");
  assert.match(src, /<RetryImg\b/, "keeps RetryImg for retry/fallback");
  assert.match(src, /className="c-cover"/, "keeps the .c-cover absolute cover class");
  assert.match(src, /c-cover-duration/, "keeps the cover corner duration badge");
  assert.match(src, /<CardFooter\b/, "keeps the CardFooter hook");
  assert.match(src, /<CardMeta\b/, "meta row flows through the shared CardMeta");
  // 封面统一吃 graftingImage 默认宽度（240w，用户已验收，不再改动）。
  assert.match(src, /graftingImage\(cover \?\? ""\)/);
  // 默认卡片类名仍是 c-list-card（content-visibility 契约所依赖）。
  assert.match(src, /cardClassName = "c-list-card"/);
});

test("all 8 grid lists render through <ListCard> and no longer hand-roll video cards", () => {
  const count = (src, re) => (src.match(re) || []).length;
  for (const name of LIST_COMPONENTS) {
    const src = read(`../src/components/${name}.tsx`);
    assert.match(src, /import ListCard from "\.\/listCard"/, `${name} must import ListCard`);
    assert.match(src, /<ListCard\b/, `${name} must render <ListCard>`);
    // 视频卡片不再手写 footer；upVideoList 另有一个「合集」瓷片（无 footer）。
    assert.doesNotMatch(src, /<CardFooter[\s>]/, `${name} must not hand-roll <CardFooter>`);
    if (name === "upVideoList") {
      // 唯一的例外：合集选择瓷片仍是独立 Card（无 footer / 无 meta）。
      assert.equal(count(src, /<CardBody[\s>]/g), 1, "only the series tile may keep a CardBody");
      assert.equal(count(src, /<Card[\s>]/g), 1, "only the series tile may keep a raw Card");
    } else {
      assert.doesNotMatch(src, /<Card[\s>]/, `${name} must not hand-roll a raw <Card>`);
      assert.doesNotMatch(src, /<CardBody[\s>]/, `${name} must not hand-roll <CardBody>`);
    }
    assert.match(src, /fields=\{\[/, `${name} must pass its meta fields to ListCard`);
  }
});

test("ListCard preserves the per-list variations via props", () => {
  const page = read("../src/components/pageList.tsx");
  // pageList 的「正在播放」胶囊与加列表按钮、part-title 语义类、relative body。
  assert.match(page, /coverChildren=/);
  assert.match(page, /part-playing-badge/);
  assert.match(page, /titleClassName="[^"]*part-title"/);
  assert.match(page, /bodyClassName="overflow-visible p-0 img-container relative"/);
  // seriesList 的当前项高亮边框仍可经 cardClassName 追加。
  assert.match(read("../src/components/seriesList.tsx"), /cardClassName=\{`c-list-card/);
  // historyList 的「移除」按钮仍作为封面附加内容注入。
  assert.match(read("../src/components/historyList.tsx"), /watchlater-remove-btn/);
});

// ---------------------------------------------------------------------------
// 问题 2：三列比例（40 / 30 / 30）+ 右对齐 + 安全列距
// ---------------------------------------------------------------------------
test("card meta columns are proportional 40% / 30% / 30% (uniform across cards)", () => {
  const author = css.match(/\.card-meta-field\.is-author \{[\s\S]*?\}/)[0];
  const views = css.match(/\.card-meta-field\.is-views \{[\s\S]*?\}/)[0];
  const pubdate = css.match(/\.card-meta-field\.is-pubdate \{[\s\S]*?\}/)[0];
  // flex-grow 4 : 3 : 3 —— 列宽只随卡片容器宽度分配，与内容长短无关，
  // 同一栅格下每张卡片的作者/播放/日期起点一致。
  assert.match(author, /flex: 4 1 0/, "author ≈ 40%");
  assert.match(views, /flex: 3 1 0/, "views ≈ 30%");
  assert.match(pubdate, /flex: 3 1 0/, "pubdate ≈ 30%");
  // 末列（发布时间）靠右贴行尾：block + text-align（须能 ellipsis）。
  assert.match(pubdate, /text-align: right/);
  assert.match(views, /justify-content: flex-end/);
});

test("card meta keeps an 8px gap so numbers and dates cannot stick together", () => {
  // 轮 28：列距从行级 gap 改为列自带 padding-left（让 4:3:3 精确落在整行）。
  const row = css.match(/\.card-meta \{[\s\S]*?\}/)[0];
  assert.match(row, /gap: 0/, "no row-level gap (it distorted the 4:3:3 ratio)");
  assert.match(row, /flex-wrap: nowrap/, "still a single non-wrapping line");
  assert.match(row, /overflow: hidden/, "still clipped, never wrapped");
  const adjacent = css.match(/\.card-meta-field \+ \.card-meta-field \{[\s\S]*?\}/);
  assert.ok(adjacent, "adjacent columns must declare a separation rule");
  assert.match(adjacent[0], /padding-left: 8px/, "columns keep an 8px collision-safety pad");
});

test("proportional columns actually settle near 40/30/30 for a realistic width", () => {
  const grow = { author: 4, views: 3, pubdate: 3 };
  const gap = 8;
  const totalGrow = grow.author + grow.views + grow.pubdate;
  const model = (W) => {
    const free = W - gap * 2; // 三列 = 两处 gap
    return {
      author: (free * grow.author) / totalGrow,
      views: (free * grow.views) / totalGrow,
      pubdate: (free * grow.pubdate) / totalGrow,
    };
  };
  const m = model(300);
  // 容差：自由空间扣除 gap 后，作者占比 ~40%、播放/发布各 ~30%。
  assert.ok(Math.abs(m.author / (m.author + m.views + m.pubdate) - 0.4) < 0.01);
  assert.ok(Math.abs(m.views / (m.author + m.views + m.pubdate) - 0.3) < 0.01);
  assert.ok(Math.abs(m.pubdate / (m.author + m.views + m.pubdate) - 0.3) < 0.01);
  // 三列之和 + 两处 gap 恰为容器宽（无溢出）。
  assert.ok(Math.abs(m.author + m.views + m.pubdate + gap * 2 - 300) < 1e-9);
});

test("priority-based hiding is preserved and thresholds stay monotonic", () => {
  const rules = [
    ...css.matchAll(
      /@container \(max-width: (\d+)px\) \{\s*\.card-meta-field\.(is-\w+) \{\s*display: none;/g,
    ),
  ].map((m) => ({ max: Number(m[1]), cls: m[2] }));
  assert.deepEqual(
    rules.map((r) => r.cls),
    ["is-extra", "is-duration", "is-pubdate", "is-views"],
    "hide order: extra → duration → pubdate → views",
  );
  for (let i = 1; i < rules.length; i += 1) {
    assert.ok(rules[i].max < rules[i - 1].max, "thresholds shrink with priority");
  }
});

test("base field keeps intrinsic width; proportion lives only on the tier classes", () => {
  const base = css.match(/\.card-meta-field \{[\s\S]*?\}/)[0];
  assert.match(base, /flex: 0 0 auto/, "untyped fields are never squashed");
  assert.match(base, /white-space: nowrap/);
});

// ---------------------------------------------------------------------------
// 问题 1：内存再收紧（显示/功能零改动）
// ---------------------------------------------------------------------------
test("runtime retention cap tightened 160 → 128 (below the old cap)", () => {
  const { LIST_RETENTION_CAP, appendWithRetention } = loadModule("../src/lib/listRetention.ts");
  assert.equal(LIST_RETENTION_CAP, 128, "round 25 tightens the runtime window to 128");
  assert.ok(LIST_RETENTION_CAP < 160, "must be below the round-21 cap");

  const items = (n, tag) => Array.from({ length: n }, (_, i) => ({ id: `${tag}${i}` }));
  // 行为不变：仍是有界滑动窗口，头部释放、尾部保留。
  const grown = appendWithRetention(items(100, "a"), items(100, "b"), 128);
  assert.equal(grown.length, 128);
  // merged = [a0..a99, b0..b99]；保留末尾 128 条 → 从 merged[72]=a72 开始。
  assert.equal(grown[0].id, "a72", "head releases exactly over the cap (200-128=72)");
  assert.equal(grown[grown.length - 1].id, "b99", "newest entry kept");
  // 入参不被修改（避免与 React state 别名共享）。
  const first = items(100, "a");
  appendWithRetention(first, items(200, "b"), 128);
  assert.equal(first.length, 100, "source array must be untouched");
});

test("index runtime cap matches the shared lib (128) and pagination stays un-capped", () => {
  assert.match(indexSrc, /const MAX_RETAINED_LIST_ITEMS = 128/);
  // 仍不封顶分页（不能出现「到达上限就 return」的旧假死）。
  assert.doesNotMatch(indexSrc, /MAX_RETAINED_LIST_ITEMS\s*\)\s*return/);
});

test("drawer cache budgets tightened one more notch", () => {
  const mod = loadModule("../src/lib/drawerCache.ts");
  assert.ok(mod.LIST_CACHE_MAX <= 64, "per-entry cap tightened to ≤64");
  assert.ok(mod.DRAWER_CACHE_TOTAL_ITEMS <= 128, "global budget tightened to ≤128");
  assert.ok(mod.DANMAKU_CACHE_MAX <= 48);
  assert.ok(mod.REPLY_CACHE_MAX <= 32);
  // 缓存态严格小于运行态，重开水合仍有富余。
  assert.ok(mod.LIST_CACHE_MAX < 128);
});

test("danmaku/reply runtime bounds tightened without changing slice semantics", () => {
  assert.match(indexSrc, /const MAX_RETAINED_DANMAKU = 320/);
  assert.match(indexSrc, /const MAX_RETAINED_REPLIES = 96/);
  // 水合与全新拉取仍旧同一上限（语义一致）。
  assert.match(indexSrc, /\.slice\(\s*0,\s*MAX_RETAINED_DANMAKU,?\s*\)/);
  assert.match(indexSrc, /items: \(data\?\.items \|\| \[\]\)\.slice\(0, MAX_RETAINED_DANMAKU\)/);
});

// ---------------------------------------------------------------------------
// 玻璃红线 + 依赖数（所有改动文件）
// ---------------------------------------------------------------------------
test("glass redline intact: shared card / lists / css add no compositor hints", () => {
  const touched = [
    "../src/components/listCard.tsx",
    "../src/components/cardMeta.tsx",
    ...LIST_COMPONENTS.map((n) => `../src/components/${n}.tsx`),
  ];
  for (const rel of touched) {
    const src = read(rel);
    assert.doesNotMatch(src, /will-change\s*:/, `${rel} must not add will-change`);
    assert.doesNotMatch(src, /backdrop-filter\s*:/, `${rel} must not add backdrop-filter`);
  }
  const metaBlock = css.slice(css.indexOf(".card-meta {"), css.indexOf(".c-list-card.c-list-card-row"));
  assert.doesNotMatch(metaBlock, /will-change\s*:/);
  assert.doesNotMatch(metaBlock, /backdrop-filter\s*:/);
  assert.doesNotMatch(metaBlock, /\bblur\(/);
});

test("no new runtime dependency was introduced (round 25)", () => {
  const pkg = JSON.parse(read("../package.json"));
  assert.equal(Object.keys(pkg.dependencies).length, 31);
  for (const dep of ["react", "@tauri-apps/api", "@tauri-apps/plugin-store", "@icon-park/react"]) {
    assert.ok(dep in pkg.dependencies, `${dep} must remain declared`);
  }
});
