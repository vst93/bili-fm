import assert from "node:assert/strict";
import { readFileSync } from "node:fs";
import { test } from "node:test";

// 用户建议③（收敛版）：标题 / 简介文字可以复制 —— 只放开文本选中，
// **不加任何复制按钮 / 下拉**（用户明确要求）。
//
// 根因：html 上有全局 `user-select: none`（main.tsx 又全局 preventDefault 掉
// contextmenu，右键里也没有「复制」），且 `#video-title, #video-desc` 这条 id 规则
// 把它再写死了一次。id 选择器特异度（1,0,0）高于 `.selectable-text`（0,1,0），
// 所以必须在 id 规则上显式打开，加 class 没用。
//
// 本测试锁定：标题与简介可选、其余 UI 仍保持原生 App 的不可选行为、
// 且没有为了「复制」新增任何按钮（#video-info 的高度预算 366px 不变）。

const read = (rel) => readFileSync(new URL(rel, import.meta.url), "utf8");

const css = read("../src/styles/globals.css");
const videoInfo = read("../src/components/videoInfo.tsx");
const mainTsx = read("../src/main.tsx");

test("css: #video-title / #video-desc 都显式打开 user-select: text", () => {
  for (const selector of ["#video-title", "#video-desc"]) {
    const rules = css.match(new RegExp(`${selector}\\s*\\{[^}]*\\}`, "g")) ?? [];
    assert.ok(rules.length > 0, `${selector} 规则必须存在`);
    assert.doesNotMatch(rules.join("\n"), /user-select:\s*none/, `${selector} 不得再写死 none`);
  }

  assert.match(
    css,
    /#video-title,\s*#video-desc\s*\{[^}]*user-select:\s*text/,
    "标题/简介必须显式 user-select: text（id 规则才压得住 .selectable-text）",
  );
});

test("css: 其余 UI 仍是原生 App 观感（全局 user-select: none 保留）", () => {
  assert.match(
    css,
    /html\s*\{[^}]*user-select:\s*none/,
    "html 上的全局禁用必须保留，否则整个界面都会变成可选中的网页",
  );
  // 可编辑控件仍然单独放开（macOS WKWebView 严格遵循 user-select: none）
  assert.match(css, /input,\s*textarea,\s*\[contenteditable\],\s*\.selectable-text\s*\{/);
});

test("videoInfo: 没有为「复制」新增按钮（高度预算不变）", () => {
  // 入口是「选中 + Ctrl/Cmd+C」；用户明确不要独立复制按钮
  assert.doesNotMatch(videoInfo, /copyText|handleCopyText|utils\/clipboard/);
  assert.doesNotMatch(videoInfo, /title="复制/);

  // #video-info 的高度预算仍按原操作行（6 个按钮）计算
  assert.match(css, /#video-info\s*\{[^}]*height:\s*366px/);
});

test("main: 右键菜单仍全局禁用（复制入口只有键盘）", () => {
  assert.match(mainTsx, /"contextmenu"/);
  assert.match(mainTsx, /event\.preventDefault\(\)/);
});
