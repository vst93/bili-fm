import assert from "node:assert/strict";
import { readFileSync } from "node:fs";
import { test } from "node:test";

// 修复轮 26：两个方向。
// ① 内存：列表封面预热钩子不再随分页重新跑（消除「不停翻滚 → 反复创建/销毁
//    Image、重复拉取同一批封面」的叠加来源），首屏体验不变。
// ② 卡片 meta 比例：所有字段列统一按 4:3:3 家族比例分配，修正「时长 / 附加」
//    仍是固定小档位时，少数列表里主列独吞剩余空间（昵称显得占 50%+）。

const read = (rel) => readFileSync(new URL(rel, import.meta.url), "utf8");
const css = read("../src/styles/globals.css");

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

/** 从 component 源码里抽出所有 fields 数组内用到的 kind（含多张卡片）。 */
function kindsOf(name) {
  const src = read(`../src/components/${name}.tsx`);
  const kinds = [];
  let from = 0;
  for (;;) {
    const start = src.indexOf("fields={[", from);
    if (start < 0) break;
    const end = src.indexOf("]}", start);
    assert.ok(end > start, `${name} fields block must close`);
    const block = src.slice(start, end);
    for (const m of block.matchAll(/kind:\s*"(\w+)"/g)) kinds.push(m[1]);
    if (/viewsMetaField\(/.test(block)) kinds.push("views");
    from = end + 1;
  }
  assert.ok(kinds.length > 0, `${name} must pass fields to ListCard`);
  return kinds;
}

/** 从 CSS 读取某个 kind 列的 flex-grow（比例分配的权重）。 */
function growOf(kind) {
  const rule = css.match(
    new RegExp(`\\.card-meta-field\\.is-${kind} \\{[^}]*\\}`),
  );
  assert.ok(rule, `.is-${kind} rule must exist`);
  const flex = rule[0].match(/flex:\s*(\d+)\s+(\d+)\s+([\d.]+)/);
  assert.ok(flex, `.is-${kind} must declare a proportional flex`);
  return { grow: Number(flex[1]), shrink: Number(flex[2]), basis: flex[3] };
}

// ---------------------------------------------------------------------------
// 方向 ②：所有列统一 4:3:3 家族比例
// ---------------------------------------------------------------------------
test("every observable tier is proportional (flex: N 1 0), no fixed-px mini tiers", () => {
  for (const kind of ["author", "views", "pubdate", "duration", "extra"]) {
    const { grow, shrink, basis } = growOf(kind);
    assert.ok(grow >= 3, `.is-${kind} must get a proportional grow (≥3, got ${grow})`);
    assert.equal(shrink, 1, `.is-${kind} stays shrinkable for ellipsis`);
    assert.equal(basis, "0", `.is-${kind} must use flex-basis 0 so width is content-independent`);
  }
  // 基态字段仍是固有宽度（未分类字段不被无差别压扁）。
  assert.match(
    css.match(/\.card-meta-field \{[^}]*\}/)[0],
    /flex:\s*0 0 auto/,
    "untyped fields keep intrinsic width",
  );
});

test("the 4:3:3 baseline still holds for author/views/pubdate", () => {
  assert.equal(growOf("author").grow, 4);
  assert.equal(growOf("views").grow, 3);
  assert.equal(growOf("pubdate").grow, 3);
});

test("no list column exceeds half the meta row; author stays ≤ ~45% where present", () => {
  // 对每个列表的可视字段集合构建比例模型（所有列同 shrink，按 grow 均分自由宽）。
  for (const name of LIST_COMPONENTS) {
    const kinds = [...new Set(kindsOf(name))];
    assert.ok(kinds.length >= 2, `${name} should show at least two meta fields`);
    const grows = kinds.map((k) => growOf(k).grow);
    const total = grows.reduce((a, b) => a + b, 0);
    const share = Object.fromEntries(kinds.map((k, i) => [k, grows[i] / total]));
    // 任何列不得过半（旧的固定小档位会让作者独吞到 50%+）。
    const worst = Math.max(...Object.values(share));
    assert.ok(
      worst <= 0.5,
      `${name} share ${JSON.stringify(share)} must keep every column ≤ 50%`,
    );
    // 有作者列时，昵称只拿 4 成，不应超过 ~45%。
    if ("author" in share) {
      assert.ok(share.author <= 0.45, `${name} author share ${share.author} must stay ≤ 45%`);
    }
  }
});

test("combinations that used to starve the row now split evenly", () => {
  // seriesList: views+pubdate → 3:3 = 50/50（此前 views 30% 独吞剩余）。
  const series = [...new Set(kindsOf("seriesList"))].sort();
  assert.deepEqual(series, ["pubdate", "views"]);
  assert.equal(growOf("views").grow, growOf("pubdate").grow);
  // pageList: views+duration → 3:3。
  const page = [...new Set(kindsOf("pageList"))].sort();
  assert.deepEqual(page, ["duration", "views"]);
  assert.equal(growOf("views").grow, growOf("duration").grow);
  // 稍后再看: author+views+extra 进度 → 4:3:3，作者 40%；
  // 历史 tab: author+views+pubdate → 4:3:3。两个 tab 都在此列表内。
  const history = [...new Set(kindsOf("historyList"))].sort();
  assert.deepEqual(history, ["author", "extra", "pubdate", "views"]);
  assert.equal(growOf("author").grow, 4);
  assert.equal(growOf("extra").grow, 3);
});

test("main three-field lists still resolve to 40 / 30 / 30", () => {
  const three = ["feedList", "recommendList", "upVideoList", "collectList", "searchList"];
  for (const name of three) {
    const kinds = [...new Set(kindsOf(name))].sort();
    assert.deepEqual(kinds, ["author", "pubdate", "views"], `${name} is a 3-field list`);
    const total = 4 + 3 + 3;
    assert.equal(4 / total, 0.4);
    assert.equal(3 / total, 0.3);
  }
});

// ---------------------------------------------------------------------------
// 方向 ①：封面预热不再随分页反复重建
// ---------------------------------------------------------------------------
test("preload hook preloads once per mount (no re-run on every page append)", () => {
  const hook = read("../src/hooks/usePreloadImages.ts");
  // 有一次性守卫：翻页导致 urls 变化时不再重复新建 Image / 重复拉封面。
  assert.match(hook, /const preloadedRef = useRef\(false\)/);
  assert.match(hook, /if \(preloadedRef\.current\) return;/);
  assert.match(hook, /preloadedRef\.current = true/);
  // 仍只预热首屏前几张。
  assert.match(hook, /PRELOAD_LIMIT/);
  assert.match(hook, /urls\.slice\(0, PRELOAD_LIMIT\)/);
});

test("preload hook releases references on unmount (blob revoke + data: placeholder)", () => {
  const hook = read("../src/hooks/usePreloadImages.ts");
  // 卸载时中断在途请求 / 清空 src，并置 1px 占位促使浏览器回收位图（沿用既有契约）。
  assert.match(hook, /revokeObjectURL\(/);
  assert.match(hook, /removeAttribute\("src"\)/);
  assert.match(hook, /img\.src = "data:,"/);
});

test("list cards still lazy-load and preload hooks are wired the same way", () => {
  // 卡片封面仍是 lazy + async 解码，预热只是首屏补充。
  const card = read("../src/components/listCard.tsx");
  assert.match(card, /loading="lazy"/);
  for (const name of ["feedList", "recommendList", "collectList", "searchList", "seriesList", "upVideoList", "historyList", "pageList"]) {
    const src = read(`../src/components/${name}.tsx`);
    assert.match(src, /usePreloadImages\(/, `${name} still preheats its first screen`);
  }
});

// ---------------------------------------------------------------------------
// 玻璃红线 + 依赖数
// ---------------------------------------------------------------------------
test("glass redline intact: this round adds no compositor hints", () => {
  for (const rel of [
    "../src/hooks/usePreloadImages.ts",
    "../src/components/listCard.tsx",
    "../src/components/cardMeta.tsx",
  ]) {
    const src = read(rel);
    assert.doesNotMatch(src, /will-change\s*:/);
    assert.doesNotMatch(src, /backdrop-filter\s*:/);
  }
  const metaBlock = css.slice(css.indexOf(".card-meta {"), css.indexOf(".c-list-card.c-list-card-row"));
  assert.doesNotMatch(metaBlock, /will-change\s*:/);
  assert.doesNotMatch(metaBlock, /backdrop-filter\s*:/);
  assert.doesNotMatch(metaBlock, /\bblur\(/);
});

test("no new runtime dependency was introduced (round 26)", () => {
  const pkg = JSON.parse(read("../package.json"));
  assert.equal(Object.keys(pkg.dependencies).length, 31);
});
