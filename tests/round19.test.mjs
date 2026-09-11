import assert from "node:assert/strict";
import { readFileSync } from "node:fs";
import { test } from "node:test";

// 修复轮 19：
// - 任务 1：SponsorBlock 查询下沉 Rust（绕开 WebView2 CSP）+ UI 状态指示；
// - 任务 2：卡片字段优先级（作者>播放>时间>时长）+ 时长封面角标 + 逐卡信息量补齐；
// - 任务 3：主页合作标签删除、列表卡片承载合作标签（数据边界见报告）。

const read = (rel) => readFileSync(new URL(rel, import.meta.url), "utf8");
const css = read("../src/styles/globals.css");

const LIST_COMPONENTS = [
  "feedList", "recommendList", "seriesList", "upVideoList",
  "collectList", "searchList", "historyList", "pageList",
];

// ---------------------------------------------------------------------------
// 任务 2：字段优先级（时长垫底）与信息量补齐
// ---------------------------------------------------------------------------
test("no list keeps duration ahead of pubdate in the meta row", () => {
  for (const name of LIST_COMPONENTS) {
    const src = read(`../src/components/${name}.tsx`);
    const block = src.slice(src.indexOf("<CardMeta"), src.indexOf("/>", src.indexOf("<CardMeta")));
    const d = block.indexOf('kind: "duration"');
    const p = block.indexOf('kind: "pubdate"');
    if (d !== -1 && p !== -1) {
      assert.ok(d > p, `${name}: duration must come after pubdate (lowest priority)`);
    }
  }
});

test("video-duration moved to a cover corner badge (industry convention)", () => {
  assert.match(css, /\.c-cover-duration \{/);
  assert.match(css, /\.c-cover-duration \{[\s\S]*?position: absolute/);
  // 至少 feedList / upVideoList / seriesList / pageList / historyList 采用角标
  for (const name of ["feedList", "upVideoList", "seriesList", "pageList", "historyList"]) {
    assert.match(read(`../src/components/${name}.tsx`), /className="c-cover-duration"/, `${name} must use cover corner duration`);
  }
});

test("every list card keeps at least two informative fields", () => {
  const informative = (block) =>
    ['kind: "author"', 'kind: "views"', 'kind: "pubdate"', 'kind: "duration"'].filter((k) =>
      block.includes(k),
    ).length;
  for (const name of LIST_COMPONENTS) {
    const src = read(`../src/components/${name}.tsx`);
    const i = src.indexOf("<CardMeta");
    assert.ok(i !== -1, `${name} must render CardMeta`);
    const block = src.slice(i, src.indexOf("/>", i));
    assert.ok(
      informative(block) >= 2,
      `${name} must show at least 2 informative fields (was: ${block})`,
    );
  }
});

test("upVideoList gained the author field (was views/duration/pubdate only)", () => {
  const src = read("../src/components/upVideoList.tsx");
  assert.match(src, /kind: "author"/);
});

test("seriesList gained views (meta) + duration (cover badge), was a lone pubdate", () => {
  const src = read("../src/components/seriesList.tsx");
  assert.match(src, /kind: "views"/);
  assert.match(src, /className="c-cover-duration"/);
});

// ---------------------------------------------------------------------------
// 任务 3：合作标签改位置
// ---------------------------------------------------------------------------
test("playback page no longer renders the redundant collab pill", () => {
  const src = read("../src/components/videoInfo.tsx");
  assert.doesNotMatch(src, /video-collab-badge/);
  assert.doesNotMatch(src, />\s*合作视频\s*</);
  // UP 主名右侧的合作图标仍在（非重复信息，保留）。
  assert.match(src, /选择合作 UP 主/);
});

test("CardMeta can carry a leading badge (for list-card collab tag)", () => {
  const src = read("../src/components/cardMeta.tsx");
  assert.match(src, /badge\?: ReactNode/);
  assert.match(src, /card-meta-badge/);
  assert.match(css, /\.card-meta-badge \{/);
});

// ---------------------------------------------------------------------------
// 任务 1：Rust 端 SponsorBlock 查询
// ---------------------------------------------------------------------------
test("Rust exposes get_sponsor_segments and registers it in the invoke handler", () => {
  const bl = read("../src-tauri/src/bilibili.rs");
  assert.match(bl, /pub async fn get_sponsor_segments\(/);
  assert.match(bl, /https:\/\/bsbsb\.top\/api\/skipSegments/);
  const cmd = read("../src-tauri/src/commands.rs");
  assert.match(cmd, /pub async fn get_sponsor_segments\(/);
  const lib = read("../src-tauri/src/lib.rs");
  assert.match(lib, /commands::get_sponsor_segments/);
});

test("frontend query layer calls the Rust command, not the webview fetch", () => {
  const src = read("../src/lib/sponsorBlock.ts");
  assert.match(src, /invoke<unknown>\(SPONSOR_API_CMD/);
  assert.doesNotMatch(src, /await fetch\(/);
});

test("player renders a SponsorBlock status dot driven by the query layer", () => {
  const src = read("../src/components/player.tsx");
  assert.match(src, /player-sponsor-status/);
  assert.match(src, /data-state=\{sponsorStatus\.state\}/);
  assert.match(css, /\.player-sponsor-status\[data-state="ok"\]/);
  assert.match(css, /\.player-sponsor-status\[data-state="error"\]/);
});

test("status layer distinguishes loading / ok / empty / error / disabled", () => {
  const src = read("../src/lib/sponsorBlock.ts");
  for (const s of ["disabled", "loading", "ok", "empty", "error"]) {
    assert.match(src, new RegExp(`"${s}"`), `status must include ${s}`);
  }
});

// ---------------------------------------------------------------------------
// 2.0.15 构建溯源（静态事实）：CSP 修复确实进了 tag，但仍挡不住 WebView2
// ---------------------------------------------------------------------------
test("CSP still lists the host (trace: 89c7842 is an ancestor of tag 2.0.15)", () => {
  const conf = JSON.parse(read("../src-tauri/tauri.conf.json"));
  assert.match(conf.app.security.csp, /connect-src[^;]*https:\/\/bsbsb\.top/);
});
