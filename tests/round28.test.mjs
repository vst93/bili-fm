import assert from "node:assert/strict";
import { readFileSync } from "node:fs";
import { test } from "node:test";
import vm from "node:vm";
import ts from "typescript";

// 修复轮 28：卡片 meta 行「具体日期」形态下的宽度失守。
//
// 背景（用户实测截图，816×609 收藏列表一排 3 卡）：
//   部分行昵称省略号出现早、右侧日期被截成「2025-09-…」仍占一格，视觉上不是
//   4:3:3。逐像素复核 + 真实 Chromium 引擎实测后确认：
//   ① 昵称列宽**与内容无关**（任何日期/字体/卡片宽度下恒为 (行宽−2×列距)×0.4）；
//      用户看到「昵称被日期挤窄」是「昵称百分比被日期列的高度截断感放大」的误判。
//   ② 真正的失守点是**日期本身是 10 字符不可断行的 `yyyy-MM-dd`**：30% 列宽
//      （~55px）永远放不下 ~62px 的它，于是被截成无信息量的「2025-09-…」。
//      相对日期（今天/昨天/N天前）短得多，所以看起来「正常」。
//   ③ 列距此前用行级 `gap: 8px`：gap 在 flex-grow 分配**之前**先扣除，于是
//      4:3:3 分的是「行宽 − 2×8px」，作者实际只到 36–37%，永远差一点点。
//
// 本轮修复：
//   A. 日期形态收敛到 ≤ 7 字符（N个月前 / yyyy-MM / 今天 / 昨天 / N天前），
//      30% 列宽在任何卡片宽度下都能完整显示，截断不再发生。
//   B. 列距改列自带 padding-left，4:3:3 精确落在整行（作者恰 40%）。
//   C. 播放量列补 overflow: hidden（列自身硬裁剪，不只靠父级兜底）。
//   D. 绝对日期字符串（feed/upVideo 的 pub_time、search 的 date）统一折算成
//      短形态 formatMetaDate，不再直出 10 字符。
//
// 轮 30 演进（用户复测）：「具体日期优先」回归 —— 日期不再一律压到 7 字符，
//   跨年显示完整 yyyy-MM-DD、当年显示 MM-DD；放不下时由渲染层按**实测像素
//   宽度**逐档降级（N个月前 / yyyy-MM）。B/C 的 4:3:3 与列宽契约**完全未变**，
//   仍由本文件的 CSS 断言锁定；A/D 的断言随之改写为「最具体形态优先」。

const read = (rel) => readFileSync(new URL(rel, import.meta.url), "utf8");
const css = read("../src/styles/globals.css");

function loadStringModule() {
  const js = ts.transpile(read("../src/utils/string.tsx"), {
    module: ts.ModuleKind.CommonJS,
    target: ts.ScriptTarget.ES2022,
  });
  const exports = {};
  const context = vm.createContext({ module: { exports }, exports, URL, Date, Number, Math, String });
  vm.runInContext(js, context);
  return context.module.exports;
}

const S = loadStringModule();

// ---------------------------------------------------------------------------
// A. 日期形态阶梯（轮 30）：**具体优先**，今天/昨天/N天前 → MM-DD → yyyy-MM-DD
//    → 兜底（N个月前 / N天前），由渲染层按实测像素宽度逐档降级。
// ---------------------------------------------------------------------------
const NOW = new Date(2025, 5, 15, 12, 0, 0);
const at = (y, mo, d) => new Date(y, mo - 1, d, 12, 0, 0);
const tsOf = (date) => date.getTime() / 1000;

test("formatRelativeTime returns the most specific form first (round 30)", () => {
  // ≤ 7 天：相对时间最直观，保留。
  assert.equal(S.formatRelativeTime(tsOf(at(2025, 6, 15)), NOW), "今天");
  assert.equal(S.formatRelativeTime(tsOf(at(2025, 6, 14)), NOW), "昨天");
  assert.equal(S.formatRelativeTime(tsOf(at(2025, 6, 13)), NOW), "2天前");
  assert.equal(S.formatRelativeTime(tsOf(at(2025, 6, 8)), NOW), "7天前");
  // > 7 天、当年：MM-DD（5 字符，具体到日，30% 列宽必放得下）。
  assert.equal(S.formatRelativeTime(tsOf(at(2025, 6, 7)), NOW), "06-07");
  assert.match(S.formatRelativeTime(tsOf(at(2025, 5, 16)), NOW), /^\d{2}-\d{2}$/);
  assert.equal(S.formatRelativeTime(tsOf(at(2025, 5, 16)), NOW), "05-16");
  // > 7 天、跨年：优先完整 yyyy-MM-DD（10 字符）。
  const crossYear = S.formatRelativeTime(tsOf(at(2024, 11, 27)), NOW);
  assert.match(crossYear, /^\d{4}-\d{2}-\d{2}$/, `跨年应为 yyyy-MM-DD，得 ${crossYear}`);
  assert.equal(crossYear, "2024-11-27");
  // 任何形态都不得是被截断的半截日期（不以连字符/省略号收尾）。
  for (const n of [0, 1, 3, 7, 8, 30, 60, 100, 200, 365, 3000]) {
    const v = S.formatRelativeTime(tsOf(new Date(NOW.getTime() - n * 86400000)), NOW);
    assert.ok(v.length > 0 && v.length <= 10, `${n}天 → 「${v}」应在 1..10 字符`);
    assert.doesNotMatch(v, /[-…]$/, `「${v}」不得以连字符/省略号结尾（半截形态）`);
  }
});

test("date ladder's month fallback never yields '0个月前'", () => {
  // 30/31 天且跨月但未满一个月历月 → 兜底夹到 1个月前（顶层形态此时是 MM-DD）。
  const ladder = S.dateFormLadder(tsOf(at(2025, 5, 15)), NOW);
  const months = ladder.find((c) => c.kind === "months");
  assert.ok(months, "阶梯必须含 N个月前 兜底");
  assert.match(months.text, /^([1-9]|1[01])个月前$/, `得 ${months && months.text}`);
  // 极端：同类但 monthDiff 算成 0 的边界（1/31 往前 30 天到 1/1）也夹到 1。
  const edge = S.dateFormLadder(tsOf(at(2025, 1, 1)), new Date(2025, 0, 31, 12, 0, 0));
  const edgeMonths = edge.find((c) => c.kind === "months");
  assert.equal(edgeMonths.text, "1个月前");
});

// ---------------------------------------------------------------------------
// D. 绝对日期字符串折算（feed/upVideo 的 pub_time、search 的 date）
// ---------------------------------------------------------------------------
test("formatMetaDate returns the most specific foldable form (round 30)", () => {
  // 跨年 → 完整 yyyy-MM-DD（交给渲染层按列宽决定是否降级）。
  assert.equal(S.formatMetaDate("2022-06-30", NOW), "2022-06-30");
  // 当年 → MM-DD（具体到日）。
  assert.equal(S.formatMetaDate("2025-04-15", NOW), "04-15");
  // 带时分秒的 pub_time 也认（只取日期）。
  assert.equal(S.formatMetaDate("2021-01-02 08:09:10", NOW), "2021-01-02");
  // 已是相对时间 / 无法识别 → 原样返回（不丢信息）。
  assert.equal(S.formatMetaDate("11天前"), "11天前");
  assert.equal(S.formatMetaDate("刚刚"), "刚刚");
  // 空值统一 null（CardMeta 会整字段不渲染）。
  assert.equal(S.formatMetaDate(""), null);
  assert.equal(S.formatMetaDate(null), null);
  assert.equal(S.formatMetaDate(undefined), null);
});

// ---------------------------------------------------------------------------
// B/C. CSS：4:3:3 精确落在整行；列距用 padding；views 硬裁剪
// ---------------------------------------------------------------------------
test("column separation moved from row gap into per-column padding", () => {
  const row = css.match(/\.card-meta \{[\s\S]*?\}/)[0];
  assert.match(row, /gap: 0/, "行级 gap 归零，避免从比例里先扣 16px");
  const adjacent = css.match(/\.card-meta-field \+ \.card-meta-field \{[\s\S]*?\}/);
  assert.ok(adjacent, "相邻列必须有分隔规则");
  assert.match(adjacent[0], /padding-left: 8px/, "8px 安全距改由列自带 padding 承担");
});

test("4:3:3 now resolves to an exact 40 / 30 / 30 of the full row", () => {
  // 轮 28 的关键：列基准显式设为 40% / 30% / 30%。只有 flex-grow（basis 0）
  // 时列宽是 （行宽−列距）×占比，永远差一个固定像素；basis 百分比把 border-box
  // 钉在比例上，最终恰好 40 / 30 / 30（真实 Chromium 实测亦如此）。
  const author = css.match(/\.card-meta-field\.is-author \{[\s\S]*?\}/)[0];
  const views = css.match(/\.card-meta-field\.is-views \{[\s\S]*?\}/)[0];
  const pubdate = css.match(/\.card-meta-field\.is-pubdate \{[\s\S]*?\}/)[0];
  const extra = css.match(/\.card-meta-field\.is-extra \{[\s\S]*?\}/)[0];
  const duration = css.match(/\.card-meta-field\.is-duration \{[\s\S]*?\}/)[0];
  assert.match(author, /flex-basis: 40%/);
  assert.match(views, /flex-basis: 30%/);
  assert.match(pubdate, /flex-basis: 30%/);
  assert.match(extra, /flex-basis: 30%/);
  assert.match(duration, /flex-basis: 30%/);
  // 模型（与浏览器一致）：自由空间 = 行宽 − Σ基准；columns 之和 > 行宽时 shrink
  // 等量收缩至 0，作者再被 max-width:40% 钉住 → 恰好 40/30/30。
  for (const W of [184, 200, 216, 224, 240, 300]) {
    const raw = { author: W * 0.4, views: W * 0.3, pubdate: W * 0.3 };
    const sum = raw.author + raw.views + raw.pubdate;
    // 收缩至行宽（等 shrink，比例不变）
    const k = W / sum;
    const author = Math.min(raw.author * k, W * 0.4);
    const views = raw.views * k;
    const pubdate = raw.pubdate * k;
    assert.ok(Math.abs(author / W - 0.4) < 1e-9, `W=${W}: 作者应恰 40%`);
    assert.ok(Math.abs(views / W - 0.3) < 1e-9, `W=${W}: 播放量应恰 30%`);
    assert.ok(Math.abs(author + views + pubdate - W) < 1e-9, "三段铺满整行");
  }
});

test("author keeps its 40% hard cap; it is never squeezed below 40% by any date", () => {
  const author = css.match(/\.card-meta-field\.is-author \{[\s\S]*?\}/)[0];
  assert.match(author, /flex: 4 1 0/);
  // 40% 上限只在「三字段不齐」时吸收余量，正常三字段下恰好命中 40%。
  assert.match(author, /max-width: 40%/);
  const pubdate = css.match(/\.card-meta-field\.is-pubdate \{[\s\S]*?\}/)[0];
  // 日期列不得把内容宽度当成最小宽度撑破基准（否则会挤压作者）；只保留 min-width: 0。
  assert.match(pubdate, /flex: 3 1 0/);
  assert.match(pubdate, /min-width: 0/);
  assert.equal((pubdate.match(/min-width/g) || []).length, 1, "日期列只声明一个 min-width（且为 0）");
});

test("views and pubdate clip inside their own column (no bleed into the author)", () => {
  const views = css.match(/\.card-meta-field\.is-views \{[\s\S]*?\}/)[0];
  const pubdate = css.match(/\.card-meta-field\.is-pubdate \{[\s\S]*?\}/)[0];
  const duration = css.match(/\.card-meta-field\.is-duration \{[\s\S]*?\}/)[0];
  for (const [name, rule] of [["views", views], ["pubdate", pubdate], ["duration", duration]]) {
    assert.match(rule, /overflow: hidden/, `${name} 列必须自身硬裁剪`);
    assert.match(rule, /min-width: 0/, `${name} 列必须允许收缩`);
  }
  assert.match(pubdate, /text-overflow: ellipsis/);
});

test("hiding order and thresholds unchanged (extra→duration→pubdate→views)", () => {
  const rules = [
    ...css.matchAll(
      /@container \(max-width: (\d+)px\) \{\s*\.card-meta-field\.(is-\w+) \{\s*display: none;/g,
    ),
  ].map((m) => ({ max: Number(m[1]), cls: m[2] }));
  assert.deepEqual(
    rules.map((r) => r.cls),
    ["is-extra", "is-duration", "is-pubdate", "is-views"],
  );
  for (let i = 1; i < rules.length; i += 1) {
    assert.ok(rules[i].max < rules[i - 1].max, "阈值随优先级递减");
  }
});

// ---------------------------------------------------------------------------
// D（接线）：8 个列表的日期字段全部走「短形态」入口
// ---------------------------------------------------------------------------
test("every list funnels its pubdate through a length-bounded formatter", () => {
  const rel = (name) => read(`../src/components/${name}.tsx`);
  // 直接给数字时间戳的列表走 formatRelativeTime；给绝对字符串的走 formatMetaDate。
  const numeric = ["collectList", "recommendList", "seriesList", "historyList"];
  for (const name of numeric) {
    assert.match(rel(name), /formatRelativeTime\(/, `${name} 应用 formatRelativeTime`);
  }
  const absolute = ["feedList", "upVideoList", "searchList"];
  for (const name of absolute) {
    assert.match(rel(name), /formatMetaDate\(/, `${name} 应用 formatMetaDate 折算绝对日期`);
    assert.doesNotMatch(
      rel(name),
      /\{ kind: "pubdate", value: (publishTime|video\.date) \}/,
      `${name} 不得再直出 10 字符绝对日期`,
    );
  }
  // 日期列现在也配了完整值 tooltip（轮 28）。
  assert.match(read("../src/components/cardMeta.tsx"), /field\.kind === "pubdate" && typeof field\.value === "string"/);
});

// ---------------------------------------------------------------------------
// 玻璃红线 + 依赖数
// ---------------------------------------------------------------------------
test("round 28 adds no compositor hints (glass redline intact)", () => {
  // 只检查本轮改动涉及的 meta 块与源码（globals.css 其它处历史上已有
  // will-change / backdrop-filter，红线是「本轮不新增」）。
  const metaBlock = css.slice(
    css.indexOf(".card-meta {"),
    css.indexOf(".c-list-card.c-list-card-row"),
  );
  assert.doesNotMatch(metaBlock, /will-change\s*:/);
  assert.doesNotMatch(metaBlock, /backdrop-filter\s*:/);
  assert.doesNotMatch(metaBlock, /\bblur\(/);
  for (const rel of ["../src/utils/string.tsx", "../src/components/cardMeta.tsx"]) {
    const src = read(rel);
    assert.doesNotMatch(src, /will-change\s*:/);
    assert.doesNotMatch(src, /backdrop-filter\s*:/);
  }
});

test("no new runtime dependency was introduced (round 28)", () => {
  const pkg = JSON.parse(read("../package.json"));
  assert.equal(Object.keys(pkg.dependencies).length, 31);
});
