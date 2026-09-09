import assert from "node:assert/strict";
import { readFileSync } from "node:fs";
import { test } from "node:test";
import vm from "node:vm";
import ts from "typescript";

// Exercise the page's actual handlers with controlled IPC timing, without a
// native Tauri window or a new frontend test dependency.
const source = readFileSync(new URL("../src/pages/index.tsx", import.meta.url), "utf8");
const ast = ts.createSourceFile("index.tsx", source, ts.ScriptTarget.Latest, true, ts.ScriptKind.TSX);
const handlerNames = new Set([
  "handleVideoSelect", "handlePlaylistVideoSelect", "handlePrevTrack",
  "handleNextTrack", "handlePlaylistReorder", "handlePlaylistDelete", "handlePlaylistClear",
]);
const handlers = [];
const refUpdates = [];
let keyboardEffect;
function visit(node) {
  if (ts.isVariableDeclaration(node) && handlerNames.has(node.name.getText(ast))) {
    handlers.push(`const ${node.getText(ast)};`);
  }
  if (ts.isCallExpression(node) && node.expression.getText(ast) === "useEffect"
    && node.arguments[0]?.getText(ast).includes("const handleKeyPress =")) {
    keyboardEffect = node.getText(ast);
  }
  if (ts.isExpressionStatement(node) && ts.isBinaryExpression(node.expression)
    && ["playlistsRef.current", "mediaNavigationRef.current.previous", "mediaNavigationRef.current.next"]
      .includes(node.expression.left.getText(ast))) {
    refUpdates.push(node.getText(ast));
  }
  ts.forEachChild(node, visit);
}
visit(ast);
assert.ok(keyboardEffect);
assert.equal(handlers.length, handlerNames.size);

const makeInfo = (bvid, aid) => ({
  bvid, aid, title: bvid, pic: "", cid: aid * 100 + 1,
  pages: [1, 2, 3].map((part) => ({ cid: aid * 100 + part, part: `${bvid}${part}` })),
});
const infos = { A: makeInfo("A", 1), B: makeInfo("B", 2), C: makeInfo("C", 3) };
const items = Object.values(infos).map((info) => ({
  id: info.bvid, bvid: info.bvid, aid: info.aid, cid: info.cid,
  part: `${info.bvid}1`, title: info.title, pic: "", first_frame: "",
}));
const flush = () => new Promise((resolve) => setImmediate(resolve));

function createHarness(overrides = {}) {
  const state = {
    playlist: [...items], seriesPlaylist: [], activePlaylistType: "user",
    playingPlaylistType: "user", currentPlaylistIndex: 0, currentSeriesPlaylistIndex: -1,
    isPlaylistMode: true, playingInfo: infos.A, videoInfo: infos.A, currentBvid: "A",
    currentIndex: 0, currentPart: "A1", playUrl: "audio-A", playlistPlayMode: "sequence",
    isPlayVideo: false, isPlayVideoStop: false, ...overrides,
  };
  const requests = [];
  const toasts = [];
  const listeners = new Map();
  let previousDeps;
  let cleanup;
  const context = vm.createContext({
    console: { error() {} },
    toast: (message) => toasts.push(message),
    playbackRequestIdRef: { current: 0 },
    playlistsRef: { current: { user: state.playlist, series: state.seriesPlaylist } },
    mediaNavigationRef: { current: {} },
    HTMLInputElement: class {}, HTMLTextAreaElement: class {},
    HTMLButtonElement: class {}, HTMLElement: class {},
    window: {
      addEventListener: (name, fn) => listeners.set(name, fn),
      removeEventListener: (name) => listeners.delete(name),
    },
    useEffect: (fn, deps) => {
      if (previousDeps && deps.every((dep, i) => Object.is(dep, previousDeps[i]))) return;
      cleanup?.();
      previousDeps = deps;
      cleanup = fn();
    },
    invoke: async (command, args) => {
      if (command === "get_clist") return infos[args.bvid];
      assert.equal(command, "get_url_by_cid");
      return new Promise((resolve, reject) => requests.push({ args, resolve, reject }));
    },
  });
  for (const [, setter] of source.matchAll(/\b(set[A-Z]\w*)\(/g)) {
    const key = setter[3].toLowerCase() + setter.slice(4);
    context[setter] = (value) => {
      state[key] = typeof value === "function" ? value(state[key]) : value;
    };
  }
  function render() {
    context.snapshot = { ...state };
    const code = `(() => {
      const {${Object.keys(state).join(",")}} = snapshot;
      ${handlers.join("\n")}
      ${refUpdates.join("\n")}
      ${keyboardEffect};
      globalThis.actions = {${[...handlerNames].join(",")}};
    })()`;
    vm.runInContext(ts.transpile(code, { target: ts.ScriptTarget.ES2022 }), context);
  }
  render();
  return {
    state, requests, toasts, render,
    get actions() { return context.actions; },
    key(code) {
      listeners.get("keyup")({ code, repeat: false, target: {}, preventDefault() {} });
    },
  };
}

for (const mutation of ["reorder", "delete preceding"]) {
  test(`pending playlist selection follows the item after ${mutation}`, async () => {
    const h = createHarness();
    const pending = h.actions.handlePlaylistVideoSelect(1);
    await flush();
    if (mutation === "reorder") h.actions.handlePlaylistReorder(1, 2);
    else h.actions.handlePlaylistDelete("A");
    h.render();
    h.requests[0].resolve({ url: "audio-B" });
    await pending;
    assert.equal(h.state.playUrl, "audio-B");
    assert.equal(h.state.playingInfo.bvid, "B");
    assert.equal(h.state.playlist[h.state.currentPlaylistIndex].bvid, "B");
  });
}

for (const mutation of ["delete selected", "clear"]) {
  test(`pending playlist selection does not start after ${mutation}`, async () => {
    const h = createHarness();
    const pending = h.actions.handlePlaylistVideoSelect(1);
    await flush();
    if (mutation === "clear") h.actions.handlePlaylistClear();
    else h.actions.handlePlaylistDelete("B");
    h.render();
    h.requests[0].resolve({ url: "audio-B" });
    await pending;
    assert.equal(h.state.playUrl, "audio-A");
    assert.equal(h.state.playingInfo.bvid, "A");
  });
}

for (const newerMode of ["playlist", "episode"]) {
  test(`a newer ${newerMode} selection wins over a slow playlist request`, async () => {
    const h = createHarness();
    const old = h.actions.handlePlaylistVideoSelect(1);
    await flush();
    const latest = newerMode === "playlist"
      ? h.actions.handlePlaylistVideoSelect(2)
      : h.actions.handleVideoSelect(301, 3, "C1", 0, "", infos.C);
    await flush();
    h.requests[1].resolve({ url: "audio-C" });
    await latest;
    h.requests[0].resolve({ url: "audio-B" });
    await old;
    assert.equal(h.state.playUrl, "audio-C");
    assert.equal(h.state.playingInfo.bvid, "C");
    assert.equal(h.state.isPlaylistMode, newerMode === "playlist");
  });
}

for (const staleResult of ["success", "error"]) {
  test(`a stale episode ${staleResult} cannot replace or report over a newer playlist selection`, async () => {
    const h = createHarness();
    const old = h.actions.handleVideoSelect(201, 2, "B1", 0, "", infos.B);
    const latest = h.actions.handlePlaylistVideoSelect(2);
    await flush();
    h.requests[1].resolve({ url: "audio-C" });
    await latest;
    if (staleResult === "success") h.requests[0].resolve({ url: "audio-B" });
    else h.requests[0].reject(new Error("stale failure"));
    await old;
    assert.equal(h.state.playUrl, "audio-C");
    assert.equal(h.state.isPlaylistMode, true);
    assert.equal(h.toasts.length, 0);
  });
}

test("a newly populated series playlist can start through the explicit source argument", async () => {
  const h = createHarness();
  const pending = h.actions.handlePlaylistVideoSelect(0, [items[1]], "series");
  h.state.seriesPlaylist = [items[1]];
  h.render();
  await flush();
  h.requests[0].resolve({ url: "audio-B" });
  await pending;
  assert.equal(h.state.playingPlaylistType, "series");
  assert.equal(h.state.currentSeriesPlaylistIndex, 0);
  assert.equal(h.state.playingInfo.bvid, "B");
});

for (const [key, expectedCid] of [["ArrowRight", 202], ["ArrowLeft", 203]]) {
  test(`${key} uses the newly playing video even when browsing state and episode index do not change`, async () => {
    const h = createHarness({ isPlaylistMode: false, videoInfo: infos.B });
    const pending = h.actions.handleVideoSelect(201, 2, "B1", 0, "");
    h.state.videoInfo = infos.C;
    h.render();
    h.requests[0].resolve({ url: "audio-B" });
    await pending;
    h.render();
    h.key(key);
    await flush();
    assert.equal(h.requests[1].args.aid, 2);
    assert.equal(h.requests[1].args.cid, expectedCid);
    h.requests[1].resolve({ url: "audio-next" });
    await flush();
  });
}
