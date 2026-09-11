import assert from "node:assert/strict";
import { readFileSync } from "node:fs";
import { test } from "node:test";
import vm from "node:vm";
import ts from "typescript";

// Exercise the SponsorBlock query layer with a mocked fetch: silent degradation
// (network / timeout / bad JSON) and session caching must never disturb playback.
const source = readFileSync(
  new URL("../src/lib/sponsorBlock.ts", import.meta.url),
  "utf8",
);
// The module compiles to an ES module; evaluate it as CommonJS so the
// transpiled `export` statements are reachable in this harness.
const js = ts.transpile(source, {
  module: ts.ModuleKind.CommonJS,
  target: ts.ScriptTarget.ES2022,
});

function loadModule(fetchImpl) {
  let exports = {};
  const context = vm.createContext({
    module: { exports },
    exports,
    AbortController,
    setTimeout,
    clearTimeout,
    encodeURIComponent,
    fetch: fetchImpl,
    Promise,
  });
  vm.runInContext(js, context);
  return context.module.exports;
}

test("maps segment ranges and drops non-skip actions", async () => {
  const { fetchSegments } = loadModule(async () => ({
    ok: true,
    json: async () => [
      {
        segment: [84.672, 129.603],
        category: "sponsor",
        actionType: "skip",
        UUID: "u1",
        videoDuration: 169.866,
      },
      {
        segment: [10, 20],
        category: "selfpromo",
        actionType: "mute",
        UUID: "u2",
        videoDuration: 169.866,
      },
      { segment: [30, 30], category: "sponsor", actionType: "skip" },
      { bad: true },
    ],
  }));
  const segs = await fetchSegments("BV1test", 123);
  assert.equal(segs.length, 1);
  assert.equal(segs[0].segment[0], 84.672);
  assert.equal(segs[0].segment[1], 129.603);
  assert.equal(segs[0].category, "sponsor");
});

test("degrades silently to [] on network error, non-200, and bad JSON", async () => {
  const throwing = loadModule(async () => {
    throw new Error("offline");
  });
  assert.equal((await throwing.fetchSegments("BV1test", 1)).length, 0);

  const notOk = loadModule(async () => ({ ok: false, json: async () => [] }));
  assert.equal((await notOk.fetchSegments("BV2test", 2)).length, 0);

  const badJson = loadModule(async () => ({
    ok: true,
    json: async () => {
      throw new SyntaxError("bad json");
    },
  }));
  assert.equal((await badJson.fetchSegments("BV3test", 3)).length, 0);

  const wrongShape = loadModule(async () => ({ ok: true, json: async () => ({}) }));
  assert.equal((await wrongShape.fetchSegments("BV4test", 4)).length, 0);
});

test("caches per BV+cid, including empty results, so repeat lookups do not refetch", async () => {
  let calls = 0;
  const { fetchSegments } = loadModule(async () => {
    calls += 1;
    return { ok: true, json: async () => [] };
  });
  await fetchSegments("BVone", 9);
  await fetchSegments("BVone", 9);
  assert.equal(calls, 1);
});

test("empty arguments never hit the network", async () => {
  let calls = 0;
  const { fetchSegments } = loadModule(async () => {
    calls += 1;
    return { ok: true, json: async () => [] };
  });
  assert.equal((await fetchSegments("", 5)).length, 0);
  assert.equal((await fetchSegments("BVx", 0)).length, 0);
  assert.equal(calls, 0);
});

// 回归：轮 7 的 aborted 请求会把空结果写进会话缓存，导致该 BV:cid 永远为空
// （React StrictMode 双挂载 / 换曲中止后同曲重查都会命中空缓存），跳过功能失效。
test("an aborted request must not poison the session cache (retry can still succeed)", async () => {
  let calls = 0;
  const { fetchSegments } = loadModule((url, opts) =>
    new Promise((resolve, reject) => {
      calls += 1;
      const timer = setTimeout(
        () =>
          resolve({
            ok: true,
            json: async () => [
              {
                segment: [84.672, 129.603],
                category: "sponsor",
                actionType: "skip",
                UUID: "u1",
                videoDuration: 169.866,
              },
            ],
          }),
        40,
      );
      opts?.signal?.addEventListener("abort", () => {
        clearTimeout(timer);
        reject(new Error("aborted"));
      });
    }),
  );

  const controller = new AbortController();
  const aborted = fetchSegments("BVpoison", 42, controller.signal);
  controller.abort();
  assert.equal((await aborted).length, 0);

  // 中止结果不得被缓存：重查应真正打到网络并拿到片段。
  const retried = await fetchSegments("BVpoison", 42);
  assert.equal(retried.length, 1);
  assert.equal(retried[0].segment[0], 84.672);
  assert.equal(calls, 2);
});

// 回归：并发同源请求可去重；但被其他 signal 中止的在途请求不得传染给新调用方。
test("inflight dedupe is scoped to the caller's own signal", async () => {
  let calls = 0;
  const { fetchSegments } = loadModule(
    () =>
      new Promise((resolve) => {
        calls += 1;
        setTimeout(
          () =>
            resolve({
              ok: true,
              json: async () => [
                { segment: [5, 10], category: "sponsor", actionType: "skip" },
              ],
            }),
          30,
        );
      }),
  );
  const shared = new AbortController();
  const [a, b] = await Promise.all([
    fetchSegments("BVdedupe", 7, shared.signal),
    fetchSegments("BVdedupe", 7, shared.signal),
  ]);
  assert.equal(a.length, 1);
  assert.equal(b.length, 1);
  assert.equal(calls, 1);
});

// 触发链回归：直接用 player.tsx 里真实的 safeSeek + maybeSkipSponsor，
// 断言 currentTime 落入 [start, end) 时会 safeSeek 到 end+pad、fired 首次不拦截、
// 二次调用因已 fired 而不再跳（防重生效），且中途跳一次会发一次 toast。
const playerSource = readFileSync(
  new URL("../src/components/player.tsx", import.meta.url),
  "utf8",
);
const playerAst = ts.createSourceFile(
  "player.tsx",
  playerSource,
  ts.ScriptTarget.Latest,
  true,
  ts.ScriptKind.TSX,
);
function playerFn(name) {
  let out = null;
  (function visit(node) {
    if (
      !out &&
      ts.isVariableDeclaration(node) &&
      node.initializer &&
      ts.isArrowFunction(node.initializer) &&
      node.name.getText(playerAst) === name
    ) {
      out = node.initializer.getText(playerAst);
    }
    ts.forEachChild(node, visit);
  })(playerAst);
  if (!out) throw new Error(`player.tsx: ${name} not found`);
  return out;
}
const stripTypes = (src) =>
  ts
    .transpileModule(src, {
      compilerOptions: {
        target: ts.ScriptTarget.ES2022,
        module: ts.ModuleKind.None,
        removeComments: true,
      },
    })
    .outputText.replace(/import\.meta\.env\.DEV/g, "false")
    .replace(/"use strict";/g, "")
    .replace(/Object\.defineProperty\(exports[\s\S]*?\}\);/g, "")
    .replace(/^export.*$/gm, "");

function buildSkipper() {
  const body = `
    ${stripTypes(`const safeSeek = ${playerFn("safeSeek")};`)}
    const sponsorSegmentsRef = { current: segments };
    const sponsorSkipFiredRef = { current: fired };
    const sponsorToastAtRef = { current: 0 };
    const currentTimeRef = { current: 0 };
    const toast = (o) => toasts.push(o);
    const SPONSOR_SEGMENT_LEAD_SECONDS = 0.25;
    const SPONSOR_SEEK_PAD_SECONDS = 0.05;
    const SPONSOR_DURATION_TOLERANCE_SECONDS = 2;
    const SPONSOR_TOAST_MIN_INTERVAL_MS = 10000;
    ${stripTypes(`const maybeSkipSponsor = ${playerFn("maybeSkipSponsor")};`)}
    return { run: () => maybeSkipSponsor(audio), fired, toasts, currentTimeRef };
  `;
  return new Function(
    "audio",
    "segments",
    "fired",
    "toasts",
    "HTMLMediaElement",
    "sponsorSkip",
    body,
  );
}

const SEGMENTS = [
  { segment: [28.133, 35.5], category: "sponsor", actionType: "skip", UUID: "b", videoDuration: 295.033 },
];
const HTML_MEDIA = { HAVE_CURRENT_DATA: 2, HAVE_ENOUGH_DATA: 4, NETWORK_LOADING: 2 };
const makeAudio = (o = {}) => ({
  currentTime: 0,
  duration: 295.033,
  readyState: 4,
  networkState: 1,
  ...o,
});

test("trigger chain: currentTime inside [start,end) calls safeSeek to end+pad and fires once", () => {
  const audio = makeAudio({ currentTime: 29 });
  const fired = new Set();
  const toasts = [];
  const h = buildSkipper()(audio, SEGMENTS, fired, toasts, HTML_MEDIA, true);
  h.run();
  assert.equal(audio.currentTime, 35.5 + 0.05, "seeks to end + pad");
  assert.equal(h.currentTimeRef.current, 35.55);
  assert.deepEqual([...fired], [0], "segment marked fired");
  assert.equal(toasts.length, 1, "one toast on first skip");
  // 防重：二次调用不再重复 seek/toast。
  audio.currentTime = 29; // 模拟手动 seek 回区间
  h.run();
  assert.equal(audio.currentTime, 29, "already-fired segment is not re-skipped");
  assert.equal(toasts.length, 1);
});

test("trigger chain: currentTime before the segment does not skip", () => {
  const audio = makeAudio({ currentTime: 10 });
  const h = buildSkipper()(audio, SEGMENTS, new Set(), [], HTML_MEDIA, true);
  h.run();
  assert.equal(audio.currentTime, 10);
});

// ---------------------------------------------------------------------------
// 回归：AD 按钮「点击状态已翻转但视觉仍灰」的根因是深色/高对比期 CSS 级联丢失。
// 深色期的通用 `html:is([...]) #player .player-button` 规则特异度 (1,2,1) 会盖过
// 基础激活规则 `#player .player-sponsor-button[data-active]` (1,2,0)。EQ 早已有
// 专属的深色激活规则，恰饭开关此前遗漏 —— 这里以静态契约锁死两者必须成对存在。
const cssSource = readFileSync(
  new URL("../src/styles/globals.css", import.meta.url),
  "utf8",
);

// 切出深色时段块：从第一个 time-of-day 选择器首次出现处，到 prefers-reduced-transparency。
const darkRegionStart = cssSource.indexOf('[data-time-of-day="midnight"]');
assert.ok(darkRegionStart > 0, "expected dark time-of-day selectors");
const darkRegionEnd = cssSource.indexOf('@media (prefers-reduced-transparency');
assert.ok(darkRegionEnd > darkRegionStart, "expected prefers-reduced-transparency block");
const darkRegion = cssSource.slice(darkRegionStart, darkRegionEnd);

test("dark time-of-day block declares an active rule for both EQ and sponsor", () => {
  assert.ok(darkRegion, "dark time-of-day region must exist");
  for (const cls of ["player-eq-button", "player-sponsor-button"]) {
    const re = new RegExp(
      `\\)\\s*#player\\s*\\.${cls}\\[data-active\\]\\s*\\{`,
      "u",
    );
    assert.match(
      darkRegion,
      re,
      `dark block must declare ${cls}[data-active] or the evening/night ` +
        `generic .player-button rule (1,2,1) wins over the base active rule (1,2,0)`,
    );
  }
});

test("prefers-contrast: more declares an active rule for both EQ and sponsor", () => {
  const region = cssSource.slice(cssSource.indexOf("@media (prefers-contrast: more)"));
  for (const cls of ["player-eq-button", "player-sponsor-button"]) {
    const re = new RegExp(
      `#player\\s*\\.player-controls\\s*\\.${cls}\\[data-active\\]\\s*\\{`,
      "u",
    );
    assert.match(
      region,
      re,
      `high-contrast block must declare ${cls}[data-active] to survive the ` +
        `generic .player-button high-contrast override`,
    );
  }
});
