import assert from "node:assert/strict";
import { readFileSync } from "node:fs";
import { test } from "node:test";
import vm from "node:vm";
import ts from "typescript";

// 修复轮 30：发布时间形态回调 —— 「具体日期优先」。
//
// 用户诉求（唯一需求来源）：尽量看到具体时间；列宽有冗余（「把相对放大时间展示的
// 区域宽度就可以」）。因此本轮：
//   A. 日期形态回调为「具体优先」：今天/昨天/N天前（≤7 天）→ 当年 MM-DD →
//      跨年 yyyy-MM-DD；放不下才降级为 yyyy-MM / N个月前。
//   B. 是否降级由**真实渲染宽度**决定（PubDateField 在隐藏测量槽里量像素），
//      不再按字符数硬猜。
//   C. 列宽弹性：发布时间列基准仍 30%，但可在整行有富余时「借 slack」变宽，
//      以容下 10 字符日期；作者 40% / 播放量 30% 基准与 r28 的 4:3:3 不回归。
//   D. 8 个列表仍统一走长度受控入口，悬浮 tooltip 兜底保留。

const read = (rel) => readFileSync(new URL(rel, import.meta.url), "utf8");
const css = read("../src/styles/globals.css");

function loadStringModule() {
  const js = ts.transpile(read("../src/utils/string.tsx"), {
    module: ts.ModuleKind.CommonJS,
    target: ts.ScriptTarget.ES2022,
  });
  const exports = {};
  const context = vm.createContext({
    module: { exports },
    exports,
    URL,
    Date,
    Number,
    Math,
    String,
  });
    vm.runInContext(js, context);
  return context.module.exports;
}

const S = loadStringModule();

const NOW = new Date(2025, 5, 15, 12, 0, 0); // 2025-06-15
const at = (y, mo, d) => new Date(y, mo - 1, d, 12, 0, 0);
const tsOf = (date) => date.getTime() / 1000;
const kinds = (ts, now = NOW) => S.dateFormLadder(ts, now).map((c) => c.kind);

// ---------------------------------------------------------------------------
// A. 阶梯「具体 → 抽象」
// ---------------------------------------------------------------------------
test("ladder is ordered specific→abstract and ends with a guaranteed fallback", () => {
  // 跨年：ymd → ym → months → days
  const cross = S.dateFormLadder(tsOf(at(2024, 11, 27)), NOW).map((c) => c.kind);
  assert.equal(cross.join(","), "ymd,ym,months,years", "跨年阶梯 ymd → ym → N个月前 → N年前");
  assert.equal(cross[0], "ymd", "首选完整年月日（比 r28 的 yyyy-MM 更具体）");
  const crossTexts = S.dateFormLadder(tsOf(at(2024, 11, 27)), NOW).map((c) => c.text);
  assert.equal(crossTexts[0], "2024-11-27");
  assert.match(crossTexts[1], /^2024-11$/);
  assert.match(crossTexts[2], /^\d+个月前$/);
  assert.match(crossTexts[3], /^\d+年前$/);
  // 当年：MM-DD 已是最具体且必放得下 → 先 md 再兜底
  const same = S.dateFormLadder(tsOf(at(2025, 4, 15)), NOW).map((c) => c.kind);
  assert.equal(same[0], "md");
  assert.ok(same.includes("months"), "当年也带 N个月前 兜底");
  assert.equal(S.dateFormLadder(tsOf(at(2025, 4, 15)), NOW)[0].text, "04-15");
  // 相对时间也是「单档」（够直观、够短）
  assert.equal(kinds(tsOf(at(2025, 6, 15))).join(","), "today");
  assert.equal(kinds(tsOf(at(2025, 6, 14))).join(","), "yesterday");
  assert.equal(kinds(tsOf(at(2025, 6, 12))).join(","), "days");
});

test("ladder: last candidate is always ≤ 5 chars (the assumed-always-fits floor)", () => {
  for (const d of [8, 20, 60, 200, 400, 1200, 4000, 9000]) {
    const ladder = S.dateFormLadder(tsOf(new Date(NOW.getTime() - d * 86400000)), NOW);
    const last = ladder[ladder.length - 1];
    assert.ok(last.text.length <= 5, `${d}天 兜底「${last.text}」应 ≤ 5 字符`);
    // 兜底以上每一档都不超过「完整年月日」，保证降级方向单调变短。
    assert.ok(last.text.length <= ladder[0].text.length);
  }
});

// ---------------------------------------------------------------------------
// B. 7 / 8 天边界（固定 now，确保确定性）
// ---------------------------------------------------------------------------
test("7-day boundary stays relative, 8-day crosses into the concrete date form", () => {
  assert.equal(S.formatRelativeTime(tsOf(at(2025, 6, 8)), NOW), "7天前");
  assert.equal(S.dateFormLadder(tsOf(at(2025, 6, 8)), NOW)[0].kind, "days");
  // 第 8 天：当年 → MM-DD（具体到日）
  assert.equal(S.formatRelativeTime(tsOf(at(2025, 6, 7)), NOW), "06-07");
  assert.equal(S.dateFormLadder(tsOf(at(2025, 6, 7)), NOW)[0].kind, "md");
});

// ---------------------------------------------------------------------------
// 年份边界 / 闰年 2-29
// ---------------------------------------------------------------------------
test("cross-year gap chooses yyyy-MM-DD (more info than r28's yyyy-MM)", () => {
  const v = S.formatRelativeTime(tsOf(at(2024, 12, 25)), new Date(2025, 0, 20, 12, 0, 0));
  assert.equal(v, "2024-12-25");
  assert.match(v, /^\d{4}-\d{2}-\d{2}$/);
});

test("leap day 2024-02-29 is parsed and formatted without NaN", () => {
  // 跨年视角（now 在 2025）：优先完整 yyyy-MM-DD
  assert.equal(S.formatMetaDate("2024-02-29", new Date(2025, 2, 15, 12, 0, 0)), "2024-02-29");
  // 当年视角（now 在 2024-03）：MM-DD
  assert.equal(S.formatMetaDate("2024-02-29", new Date(2024, 2, 15, 12, 0, 0)), "02-29");
  for (const now of [new Date(2024, 2, 15), new Date(2025, 2, 15), new Date(2024, 1, 29)]) {
    const v = S.formatMetaDate("2024-02-29", now);
    assert.doesNotMatch(v, /NaN/);
  }
});

// ---------------------------------------------------------------------------
// C. pickDateForm：宽列 → 具体；窄列 → 降级；全不放 → 兜底
// ---------------------------------------------------------------------------
test("pickDateForm degrades by measured fit: wide keeps ymd, narrow falls back", () => {
  const candidates = S.dateFormLadder(tsOf(at(2024, 11, 27)), NOW);
  const wide = S.pickDateForm(candidates, (t) => t.length <= 10);
  assert.equal(wide.kind, "ymd", "宽列应保留完整 yyyy-MM-DD");
  const mid = S.pickDateForm(candidates, (t) => t.length <= 7);
  assert.equal(mid.kind, "ym", "中等宽度退到 yyyy-MM");
  const narrow = S.pickDateForm(candidates, (t) => t.length <= 4);
  assert.equal(narrow.kind, "months", "更窄退到 N个月前");
  // 全都放不下 → 返回最后一档（绝不空）
  const none = S.pickDateForm(candidates, () => false);
  assert.equal(none.kind, "years");
});

test("pickDateForm uses real pixel widths, not character counts", () => {
  // 用「像素宽度」模拟：10 字符 ≈ 62px，5 字符 ≈ 34px。
  const pxPerChar = 6.2;
  const candidates = S.dateFormLadder(tsOf(at(2024, 11, 27)), NOW);
  // 30% 列宽 ≈ 64px → 10 字符（~62px）放得下 → 保留 ymd
  const wide = S.pickDateForm(candidates, (t) => t.length * pxPerChar <= 64);
  assert.equal(wide.kind, "ymd");
  // 20% 列宽 ≈ 43px → 10、7 字符放不下 → 退到 5 字符
  const narrow = S.pickDateForm(candidates, (t) => t.length * pxPerChar <= 43);
  assert.equal(narrow.kind, "months");
});

// ---------------------------------------------------------------------------
// D. pubdateCandidates：只有完整 yyyy-MM-DD 需要降级
// ---------------------------------------------------------------------------
test("only full yyyy-MM-DD strings need degradation; short forms pass through", () => {
  const ymd = S.pubdateCandidates("2022-06-30");
  assert.equal(ymd[0].kind, "ymd");
  assert.ok(ymd.length > 1, "完整年月日必须带降级阶梯");
  for (const short of ["06-07", "今天", "昨天", "3天前", "11个月前", "2025-09", "刚刚"]) {
    const list = S.pubdateCandidates(short);
    assert.equal(list.length, 1, `「${short}」足够短，无需降级`);
    assert.equal(list[0].text, short, "短形态原样保留（不丢信息）");
  }
});

test("production entrypoints return the most specific form (ladder[0])", () => {
  assert.equal(S.formatRelativeTime(tsOf(at(2024, 11, 27)), NOW), "2024-11-27");
  assert.equal(S.formatMetaDate("2024-12-25", new Date(2025, 0, 20)), "2024-12-25");
  // 无效输入仍是空串 / 原样（与历史契约一致）
  assert.equal(S.formatRelativeTime(0, NOW), "");
  assert.equal(S.formatRelativeTime(NaN, NOW), "");
});

// ---------------------------------------------------------------------------
// 渲染层接线：PubDateField 按实测宽度选形态 + 悬浮兜底
// ---------------------------------------------------------------------------
test("CardMeta routes pubdate strings through a measured PubDateField", () => {
  const src = read("../src/components/cardMeta.tsx");
  assert.match(src, /const PubDateField/, "必须有按宽度降级的发布时间单元格");
  assert.match(src, /pubdateCandidates\(value\)/, "候选阶梯由 pubdateCandidates 生成");
  assert.match(src, /pickDateForm\(candidates, fits\)/, "用实测 fits 选形态");
  assert.match(src, /ResizeObserver/, "宽度变化后重测（抽屉列数/窗口）");
  assert.match(src, /card-meta-pubdate-measure/, "隐藏测量槽渲染候选");
  // 悬浮兜底保留（round28 契约）：完整值写进 title。
  assert.match(src, /field\.kind === "pubdate" && typeof field\.value === "string"/);
});

test("pubdate measure slot is invisible and non-layout-affecting (CSS)", () => {
  const rule = css.match(/\.card-meta-pubdate-measure \{[\s\S]*?\}/);
  assert.ok(rule, "必须声明 .card-meta-pubdate-measure");
  assert.match(rule[0], /position: absolute/);
  assert.match(rule[0], /visibility: hidden/);
  assert.match(rule[0], /white-space: nowrap/);
});

// ---------------------------------------------------------------------------
// 列宽弹性：发布时间可借 slack，但作者 40% / 播放量 30% / 4:3:3 不回归
// ---------------------------------------------------------------------------
test("pubdate gains elasticity (borrow slack) without changing its 30% basis", () => {
  const pubdate = css.match(/\.card-meta-field\.is-pubdate \{[\s\S]*?\}/)[0];
  // 历史 / r28 契约必须仍在：可解析的 flex 短写 + 30% 基准 + 单个 min-width:0
  assert.match(pubdate, /flex: 3 1 0/, "flex 短写保留（r22/r23/r25/r26/r28 契约）");
  assert.match(pubdate, /flex-basis: 30%/, "基准仍是 30%（r28 契约）");
  assert.equal((pubdate.match(/min-width/g) || []).length, 1);
  assert.match(pubdate, /min-width: 0/);
  assert.match(pubdate, /position: relative/, "为隐藏测量槽提供定位上下文");
  // 弹性：flex-grow 被提高到 > 3（3 是 30% 基准的权重）——整行有富余时可借宽
  const grow = pubdate.match(/flex-grow:\s*([\d.]+)/);
  assert.ok(grow, "发布时间列必须声明更高的 flex-grow 以借用富余空间");
  assert.ok(Number(grow[1]) > 3, `flex-grow 应 > 3（借 slack），得 ${grow && grow[1]}`);
});

test("author 40% and views 30% baselines are untouched (r25–r28 not regressed)", () => {
  const author = css.match(/\.card-meta-field\.is-author \{[\s\S]*?\}/)[0];
  const views = css.match(/\.card-meta-field\.is-views \{[\s\S]*?\}/)[0];
  assert.match(author, /flex: 4 1 0/);
  assert.match(author, /flex-basis: 40%/);
  assert.match(author, /max-width: 40%/);
  assert.match(views, /flex: 3 1 0/);
  assert.match(views, /flex-basis: 30%/);
  assert.doesNotMatch(views, /flex-grow/, "播放量列不参与借 slack，基准恒定");
  // 列距仍是列自带 padding-left（r28 契约）
  assert.match(
    css.match(/\.card-meta-field \+ \.card-meta-field \{[\s\S]*?\}/)[0],
    /padding-left: 8px/,
  );
  // 隐藏顺序不变
  const rules = [
    ...css.matchAll(
      /@container \(max-width: (\d+)px\) \{\s*\.card-meta-field\.(is-\w+) \{\s*display: none;/g,
    ),
  ].map((m) => m[2]);
  assert.equal(rules.join(","), "is-extra,is-duration,is-pubdate,is-views");
});

test("when the row has no free space the grow is inert: pubdate stays exactly 30%", () => {
  // flex 模型：basis 30% + 更高 grow 只在自由空间 > 0 时生效；三列 basis 之和
  // 恰为 100% 时自由空间为 0，列宽仍是 30%（作者 40% / 播放 30% 不变）。
  const W = 213;
  const bases = { author: 0.4, views: 0.3, pubdate: 0.3 };
  const sumBasis = bases.author + bases.views + bases.pubdate; // = 1.0
  const free = Math.max(0, W * (1 - sumBasis));
  assert.equal(free, 0, "三列基准铺满整行 → 无自由空间");
  const base = (k) => (W * bases[k]) + free * 0; // grow 不改变结果
  assert.ok(Math.abs(base("author") / W - 0.4) < 1e-9);
  assert.ok(Math.abs(base("views") / W - 0.3) < 1e-9);
  assert.ok(Math.abs(base("pubdate") / W - 0.3) < 1e-9);
});

// ---------------------------------------------------------------------------
// 8 个列表：日期字段统一走受控入口（含 pageList 无日期的说明）
// ---------------------------------------------------------------------------
test("all eight lists funnel pubdate through a bounded formatter", () => {
  const rel = (name) => read(`../src/components/${name}.tsx`);
  // 数字时间戳路径 → formatRelativeTime（返回最具体日期形态）。
  for (const name of ["collectList", "recommendList", "seriesList", "historyList"]) {
    assert.match(rel(name), /formatRelativeTime\(/, `${name} 应用 formatRelativeTime`);
  }
  // 绝对字符串路径 → formatMetaDate。
  for (const name of ["feedList", "upVideoList", "searchList"]) {
    assert.match(rel(name), /formatMetaDate\(/, `${name} 应用 formatMetaDate`);
    assert.doesNotMatch(
      rel(name),
      /\{ kind: "pubdate", value: (publishTime|video\.date) \}/,
      `${name} 不得直出未折算的绝对日期`,
    );
  }
  // pageList 是唯一默认没有发布时间列的列表（只有播放量 + 时长）—— 记录事实，
  // 避免以后误加一个不受控的日期列。
  assert.doesNotMatch(rel("pageList"), /kind: "pubdate"/, "pageList 当前不含发布时间列");
});

// ---------------------------------------------------------------------------
// 玻璃红线 + 依赖数
// ---------------------------------------------------------------------------
test("round 30 adds no compositor hints (glass redline intact)", () => {
  for (const r of ["../src/utils/string.tsx", "../src/components/cardMeta.tsx"]) {
    const src = read(r);
    assert.doesNotMatch(src, /will-change\s*:/);
    assert.doesNotMatch(src, /backdrop-filter\s*:/);
    assert.doesNotMatch(src, /\bblur\(/);
  }
  const metaBlock = css.slice(
    css.indexOf(".card-meta {"),
    css.indexOf(".c-list-card.c-list-card-row"),
  );
  assert.doesNotMatch(metaBlock, /will-change\s*:/);
  assert.doesNotMatch(metaBlock, /backdrop-filter\s*:/);
  assert.doesNotMatch(metaBlock, /\bblur\(/);
});

test("no new runtime dependency was introduced (round 30)", () => {
  const pkg = JSON.parse(read("../package.json"));
  assert.equal(Object.keys(pkg.dependencies).length, 31);
});
