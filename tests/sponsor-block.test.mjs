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
