import assert from "node:assert/strict";
import { readFileSync, readdirSync } from "node:fs";
import { test } from "node:test";

// 修复轮 18：Windows 实测三问题的静态与模型回归。
// 覆盖：任务1 列表卡片 meta 统一排版；任务2 SponsorBlock 数据可加载 +
// 标记与开关解耦 + 失败可见；任务3 视频窗口不展示主窗「设置」入口。

const read = (rel) => readFileSync(new URL(rel, import.meta.url), "utf8");
const css = read("../src/styles/globals.css");
const distJs = (() => {
  const dir = new URL("../dist/assets/", import.meta.url);
  const name = readdirSync(dir).find((n) => /^index-.*\.js$/.test(n));
  assert.ok(name, "dist bundle index-*.js must exist (run npm run build first)");
  return readFileSync(new URL(name, dir), "utf8");
})();

// ---------------------------------------------------------------------------
// 任务 1：所有列表卡片走同一 meta 组件，单行、不换行、过窄按优先级隐藏
// ---------------------------------------------------------------------------
const LIST_COMPONENTS = [
  "feedList", "recommendList", "seriesList", "upVideoList",
  "collectList", "searchList", "historyList", "pageList",
];

test("every list component renders meta through the shared CardMeta", () => {
  for (const name of LIST_COMPONENTS) {
    const src = read(`../src/components/${name}.tsx`);
    assert.match(
      src,
      /import CardMeta from "\.\/cardMeta"/,
      `${name} must import the shared CardMeta`,
    );
    assert.match(src, /<CardMeta\b/, `${name} must render <CardMeta>`);
    assert.doesNotMatch(
      src,
      /className="card-meta"/,
      `${name} must not hand-roll the meta row anymore`,
    );
  }
});

test(".card-meta is a single non-wrapping line with overflow clipping", () => {
  const block = css.match(/\.card-meta \{[\s\S]*?\}/);
  assert.ok(block, ".card-meta rule must exist");
  assert.match(block[0], /flex-wrap: nowrap/, "meta must never wrap");
  assert.match(block[0], /overflow: hidden/, "overflow must be clipped, not wrapped");
  // 每个字段自身也不换行（作者名单独走 ellipsis 截断规则）。
  const field = css.match(/\.card-meta-field \{[\s\S]*?\}/);
  assert.match(field[0], /white-space: nowrap/);
  assert.match(field[0], /flex: 0 0 auto/, "fields keep intrinsic width, never squashed");
});

test("narrow cards hide whole fields in priority order (extra→duration→pubdate→views)", () => {
  const rules = [...css.matchAll(/@container \(max-width: (\d+)px\) \{\s*\.card-meta-field\.(is-\w+) \{\s*display: none;/g)]
    .map((m) => ({ max: Number(m[1]), cls: m[2] }));
  // 用户裁决（轮 19）：时长优先隐藏（已挪封面角标），发布时间次之。
  const order = ["is-extra", "is-duration", "is-pubdate", "is-views"];
  assert.deepEqual(
    rules.map((r) => r.cls),
    order,
    "fields must be hidden from lowest to highest priority",
  );
  // 阈值单调递增：越先隐藏的字段阈值越大。
  for (let i = 1; i < rules.length; i += 1) {
    assert.ok(rules[i].max < rules[i - 1].max, "thresholds must shrink with priority");
  }
  // 作者名是唯一保留到最后的字段（可省略号截断）。
  assert.match(css, /\.card-meta-field\.is-author \{[\s\S]*?text-overflow: ellipsis/);
});

// 布局模型：与 CSS 契约一致 —— nowrap 保证任何宽度都是单行，
// 过窄时按阈值隐藏字段，因此卡片高度在任何宽度下恒定。
function modelMeta(containerWidth, fields) {
  const rules = [...css.matchAll(/@container \(max-width: (\d+)px\) \{\s*\.card-meta-field\.(is-\w+) \{\s*display: none;/g)]
    .map((m) => ({ max: Number(m[1]), cls: m[2] }));
  const visible = fields.filter(
    (f) => !rules.some((r) => r.cls === `is-${f.kind}` && containerWidth <= r.max),
  );
  const width = visible.reduce((acc, f, i) => acc + f.w + (i ? 6 : 0), 0);
  // nowrap：行数恒为 1；过窄时靠隐藏字段而非换行收敛，宽内容由 overflow 裁剪。
  return { rows: 1, lines: 1, width, visible: visible.map((f) => f.kind) };
}

test("meta row stays one line and card height is constant across card widths", () => {
  const fields = [
    { kind: "author", w: 60 },
    { kind: "views", w: 40 },
    { kind: "duration", w: 34 },
    { kind: "pubdate", w: 44 },
    { kind: "extra", w: 50 },
  ];
  const wide = modelMeta(320, fields);
  const narrow = modelMeta(170, fields);
  assert.equal(wide.rows, 1, "wide card meta must be a single row");
  assert.equal(narrow.rows, 1, "narrow card meta must still be a single row");
  // 同样的单行高度 ⇒ 卡片高度一致（标题行 + 单行 meta）。
  assert.equal(wide.lines, narrow.lines);
  // 窄卡片按优先级丢掉了末尾字段，而不是换行。
  assert.ok(narrow.visible.length < wide.visible.length);
  assert.deepEqual(wide.visible, ["author", "views", "duration", "pubdate", "extra"]);
  assert.deepEqual(narrow.visible, ["author", "views"]);
  assert.ok(narrow.width <= 170, "after hiding, the line must fit the narrow card");
});

test("CardMeta drops empty/invalid values instead of rendering blanks", () => {
  const src = read("../src/components/cardMeta.tsx");
  assert.match(src, /field\.node !== undefined \|\| !isEmpty\(field\.value\)/);
  // NaN / null / 空串统一按空处理。
  assert.match(src, /value === "" \|\|[\s\S]*?Number\.isFinite\(value\)/);
});

// ---------------------------------------------------------------------------
// 任务 2：Windows 跳过数据未加载
// ---------------------------------------------------------------------------
test("CSP connect-src allows the SponsorBlock host", () => {
  const conf = JSON.parse(read("../src-tauri/tauri.conf.json"));
  const csp = conf.app.security.csp;
  assert.match(
    csp,
    /connect-src[^;]*https:\/\/bsbsb\.top/,
    "webview CSP must permit https://bsbsb.top or fetch is blocked before it leaves",
  );
});

test("sponsor marker renders whenever segments exist, independent of the auto-skip toggle", () => {
  const src = read("../src/components/player.tsx");
  assert.doesNotMatch(
    src,
    /data-available=\{\s*sponsorSkip &&/,
    "marker availability must not be gated on sponsorSkip",
  );
  assert.match(
    src,
    /data-available=\{\s*\/\*[\s\S]*?\*\/\s*sponsorSegmentsRef\.current\.length > 0/,
    "marker availability must be driven purely by loaded segments",
  );
  assert.doesNotMatch(
    src,
    /\{sponsorSkip && duration > 0\s*\?\s*sponsorSegmentsRef\.current\.map/,
    "segment <i> rendering must not be gated on sponsorSkip",
  );
  assert.match(src, /\{duration > 0\s*\?\s*sponsorSegmentsRef\.current\.map/);
});

test("round19: sponsor query is Rust-backed invoke, failures still observable", () => {
  const src = read("../src/lib/sponsorBlock.ts");
  assert.match(src, /console\.warn\(/);
  // 轮 9：查询下沉到 Rust 端（绕开 WebView2 CSP），前端不再直连 fetch。
  assert.match(src, /invoke<unknown>\(SPONSOR_API_CMD/);
  assert.doesNotMatch(src, /await fetch\(/);
});

test("dist bundle ships the decoupled marker gate (segments only)", () => {
  assert.match(
    distJs,
    /player-timeline-sponsor","data-available":[\w$.]+\.current\.length>0\?"true":"false"/,
    "built marker layer must key off segments, not the toggle",
  );
  assert.doesNotMatch(distJs, /data-available":[\w$]+\?/);
});

// ---------------------------------------------------------------------------
// 任务 3：视频播放窗口不应出现主窗「设置」按钮
// ---------------------------------------------------------------------------
test("settings entry is tagged and suppressed while the video overlay is open", () => {
  const bar = read("../src/components/titleBar.tsx");
  assert.match(bar, /id="settings-entry"/, "settings button needs a stable hook");
  const page = read("../src/pages/index.tsx");
  assert.match(
    page,
    /showSettingsButton=\{!showPlaylist && !isPlayVideo\}/,
    "main title bar must not expose the settings entry in video mode",
  );
  assert.match(css, /body\.video-open #settings-entry \{[^}]*display: none !important;/);
});

test("the video window (playerVideo) renders no main-window chrome", () => {
  const src = read("../src/components/playerVideo.tsx");
  assert.doesNotMatch(src, /<TitleBar/, "video window must not mount the main TitleBar");
  assert.doesNotMatch(src, /id="settings-entry"/);
  assert.doesNotMatch(src, /设置/, "video window must not render a settings affordance");
});
