import { afterEach, expect, test } from "bun:test";
import {
  Channel,
  call,
  currentWindow,
  event,
  isCallError,
  isMyGo,
  on,
  onFileDrop,
  runtime,
  type FileDrop,
  type Runtime,
} from "./index";

const calls: [string, unknown[]][] = [];
const listeners = new Map<string, (payload: unknown) => void>();

const fake: Runtime = {
  call: async <T>(method: string, ...args: unknown[]) => {
    calls.push([method, args]);
    if (method === "Svc.Fail") {
      const err = new Error("boom") as Error & { method: string };
      err.name = "CallError";
      err.method = method;
      throw err;
    }
    return method as T;
  },
  channel: <T>(onmessage?: (value: T) => void) =>
    ({ onmessage, onclose: null, closed: false, close() {}, [Symbol.asyncIterator]: () => ({}) }) as unknown as Channel<T>,
  on: (name, listener) => {
    listeners.set(name, listener as (payload: unknown) => void);
    return () => listeners.delete(name);
  },
  once: (name, listener) => {
    listeners.set(name, listener as (payload: unknown) => void);
    return () => listeners.delete(name);
  },
  platform: "darwin",
  windowId: 1,
  version: "0.1.0",
  window: {
    minimize: () => fake.call("mygo:window.Minimize"),
    maximize: () => fake.call("mygo:window.Maximize"),
    unmaximize: () => fake.call("mygo:window.Unmaximize"),
    toggleMaximize: () => fake.call("mygo:window.ToggleMaximize"),
    isMaximized: () => fake.call("mygo:window.IsMaximized"),
    toggleFullScreen: () => fake.call("mygo:window.ToggleFullScreen"),
    close: () => fake.call("mygo:window.Close"),
    setTitle: (title) => fake.call("mygo:window.SetTitle", title),
  },
};

afterEach(() => {
  delete (globalThis as { mygo?: Runtime }).mygo;
  calls.length = 0;
  listeners.clear();
});

test("outside MyGo", async () => {
  expect(isMyGo()).toBe(false);
  expect(() => runtime()).toThrow("runtime not found");
  await expect(call("Svc.Method")).rejects.toThrow("runtime not found");
});

test("delegates to window.mygo", async () => {
  (globalThis as { mygo?: Runtime }).mygo = fake;
  expect(isMyGo()).toBe(true);
  expect(await call<string>("Svc.Method", 1, "a")).toBe("Svc.Method");
  expect(calls).toEqual([["Svc.Method", [1, "a"]]]);

  const got: unknown[] = [];
  const off = event<{ n: number }>("tick").on((p) => got.push(p.n));
  listeners.get("tick")?.({ n: 7 });
  off();
  expect(got).toEqual([7]);
  expect(listeners.has("tick")).toBe(false);

  on("other", () => {});
  expect(listeners.has("other")).toBe(true);

  await currentWindow.setTitle("Hi");
  expect(calls.at(-1)).toEqual(["mygo:window.SetTitle", ["Hi"]]);

  const err = await call("Svc.Fail").catch((e: unknown) => e);
  expect(isCallError(err)).toBe(true);
  expect(isCallError(new Error("other"))).toBe(false);
});

test("Channel", () => {
  expect(() => new Channel()).toThrow("runtime not found");
  (globalThis as { mygo?: Runtime }).mygo = fake;
  const fn = (n: number) => n;
  const ch = new Channel<number>(fn);
  expect(ch.onmessage).toBe(fn);
  expect(ch.closed).toBe(false);
});

test("onFileDrop", () => {
  (globalThis as { mygo?: Runtime }).mygo = fake;
  const got: FileDrop[] = [];
  const off = onFileDrop((d) => got.push(d));
  listeners.get("mygo:file-drop")?.({ paths: ["/tmp/a.txt"], x: 10, y: 20 });
  expect(got).toEqual([{ paths: ["/tmp/a.txt"], x: 10, y: 20 }]);
  off();
  expect(listeners.has("mygo:file-drop")).toBe(false);
});
