// SponsorBlock（小电视空降助手）查询：只做「查询 + 跳过」，不做 submit/vote。
//
// 轮 19：查询从 WebView `fetch` 改为 Tauri `invoke("get_sponsor_segments")`，
// 由 Rust 端（reqwest）直连 bsbsb.top。原因见 src-tauri/src/bilibili.rs：
// Windows WebView2 把上游 host 加进 connect-src 后仍可能拦截（Tauri 会改写 CSP），
// 且用户无法开 devtools；App 的所有网络请求本就由 Rust 端发起，下沉后可 100% 绕开。
//
// 失败语义不变：任何失败静默降级为「无片段」，绝不抛错打断播放；
// 会话缓存只存「真实服务端结论」（含空数组），中止/超时/网络失败不缓存。
import { invoke } from "@tauri-apps/api/core";

export type SponsorSegment = {
  segment: [number, number];
  category: string;
  actionType: string;
  UUID: string;
  videoDuration: number;
};

// UI 状态指示（诊断用）：让用户下次截图即可看出卡在哪一环，不再靠猜。
export type SponsorStatus =
  | "disabled" // 灰：开关未开
  | "loading" // 黄：请求中
  | "ok" // 绿：已加载 N 段
  | "empty" // 灰：已加载但该视频无片段
  | "error"; // 红：失败

export type SponsorStatusInfo = {
  state: SponsorStatus;
  /** 已加载片段数（ok 时 > 0）。 */
  count: number;
  /** 失败原因（error 时）。 */
  reason?: string;
  /** 当前查询的 BV 号（便于定位）。 */
  bvid?: string;
};

export const SPONSOR_STATUS_LABEL: Record<SponsorStatus, string> = {
  disabled: "SponsorBlock 未启用",
  loading: "SponsorBlock 查询中…",
  ok: "SponsorBlock 已加载",
  empty: "SponsorBlock 无片段",
  error: "SponsorBlock 查询失败",
};

let statusListener: ((info: SponsorStatusInfo) => void) | null = null;
const lastStatus: { current: SponsorStatusInfo } = {
  current: { state: "disabled", count: 0 },
};

/** 订阅状态变化；新订阅者立即收到最近一次状态。 */
export function onSponsorStatus(listener: (info: SponsorStatusInfo) => void) {
  statusListener = listener;
  listener(lastStatus.current);
  return () => {
    if (statusListener === listener) statusListener = null;
  };
}

function emitStatus(info: SponsorStatusInfo) {
  lastStatus.current = info;
  try {
    statusListener?.(info);
  } catch {
    // 监听者异常绝不能影响查询主流程。
  }
}

// 只处理 skip 动作的片段。
const SPONSOR_API_CMD = "get_sponsor_segments";
const FETCH_TIMEOUT_MS = 5000;

// 会话级缓存：同一 BV+cid 只拉一次；仅缓存「真实服务端结论」（含空数组），
// 中止/超时/网络失败不缓存，以便换曲中止后同曲还能重新拉取。
//
// 内存上界（修复轮 23）：本模块在 React 之外、生命周期等于整个会话，
// 用户一路听下去会不断塞入新的 BV:cid 结论。单条很小，但无上界会随会话时长
// 单调增长（“还是会膨胀”）。只保留最近 SPONSOR_CACHE_MAX 条（LRU），
// 超出淘汰最旧 —— 去重命中率几乎不变，但内存不再无界。
const cache = new Map<string, SponsorSegment[]>();
const SPONSOR_CACHE_MAX = 64;
// 进行中的请求：并发调用复用同一 promise，避免重复打接口。
type InflightEntry = { promise: Promise<FetchOutcome>; signal?: AbortSignal };
const inflight = new Map<string, InflightEntry>();

const cacheKey = (bvid: string, cid: number) => `${bvid}:${cid}`;

// 一次查询的结果：只有真正拿到服务端响应（含空数组）才算 cacheable。
// 中止/超时/网络失败必须视为「未得到结论」，绝不能写进会话缓存——
// 否则一次被 abort 的换曲请求会把该 BV:cid 永久钉死为空。
type FetchOutcome = { segments: SponsorSegment[]; cacheable: boolean };

type RawSegment = Partial<SponsorSegment> & { segment?: unknown };

const parseSegments = (data: unknown): SponsorSegment[] => {
  if (!Array.isArray(data)) return [];
  const result: SponsorSegment[] = [];
  for (const item of data) {
    if (!item || typeof item !== "object") continue;
    const seg = item as RawSegment;
    const range = seg.segment;
    if (
      !Array.isArray(range) ||
      range.length < 2 ||
      typeof range[0] !== "number" ||
      typeof range[1] !== "number" ||
      !Number.isFinite(range[0]) ||
      !Number.isFinite(range[1])
    ) {
      continue;
    }
    const [start, end] = range as [number, number];
    if (end <= start) continue;
    // 只处理 skip，其它 actionType（mute/poi 等）本轮不处理。
    if (seg.actionType != null && seg.actionType !== "skip") continue;
    result.push({
      segment: [start, end],
      category: typeof seg.category === "string" ? seg.category : "",
      actionType: typeof seg.actionType === "string" ? seg.actionType : "skip",
      UUID: typeof seg.UUID === "string" ? seg.UUID : "",
      videoDuration:
        typeof seg.videoDuration === "number" && Number.isFinite(seg.videoDuration)
          ? seg.videoDuration
          : 0,
    });
  }
  return result;
};

export async function fetchSegments(
  bvid: string,
  cid: number,
  signal?: AbortSignal,
): Promise<SponsorSegment[]> {
  if (!bvid || !Number.isFinite(cid) || cid <= 0) return [];
  const key = cacheKey(bvid, cid);
  const cached = cache.get(key);
  if (cached) {
    emitStatus({
      state: cached.length > 0 ? "ok" : "empty",
      count: cached.length,
      bvid,
    });
    return cached;
  }

  // inflight 复用规则：仅当调用方与在途请求“同源”且未中止时才复用
  // （React StrictMode 双挂载：挂载#1 的请求被 abort，挂载#2 若复用它
  //  就会永远拿到空 segments，跳过功能整体失效）。
  const pending = inflight.get(key);
  if (
    pending &&
    (!signal || pending.signal === signal) &&
    !pending.signal?.aborted
  ) {
    return pending.promise.then((outcome) => outcome.segments);
  }

  let externalAborted = false;
  const run = (async (): Promise<FetchOutcome> => {
    // invoke 无法被 AbortController 取消；用外部 signal + 超时来「放弃等待」。
    // 但放弃只影响本次调用方，绝不写缓存（cacheable=false）。
    emitStatus({ state: "loading", count: 0, bvid });
    const timeout = new Promise<never>((_, reject) => {
      const timer = setTimeout(() => reject(new Error("timeout")), FETCH_TIMEOUT_MS);
      signal?.addEventListener("abort", () => {
        clearTimeout(timer);
        externalAborted = true;
        reject(new Error("aborted"));
      });
    });
    try {
      const raw = await Promise.race([
        invoke<unknown>(SPONSOR_API_CMD, { bvid, cid }),
        timeout,
      ]);
      const segments = parseSegments(raw);
      emitStatus({
        state: segments.length > 0 ? "ok" : "empty",
        count: segments.length,
        bvid,
      });
      return { segments, cacheable: true };
    } catch (error) {
      if (!externalAborted) {
        const reason = error instanceof Error ? error.message : String(error);
        // 失败可见性升级：控制台 + UI 状态点变红。
        console.warn(`[sponsor] segment query error for ${bvid}:${cid}`, error);
        emitStatus({ state: "error", count: 0, reason, bvid });
      }
      return { segments: [], cacheable: false };
    }
  })();

  inflight.set(key, { promise: run, signal });
  try {
    const outcome = await run;
    if (outcome.cacheable && !externalAborted) {
      cache.delete(key);
      cache.set(key, outcome.segments);
      // LRU 上界：超出即淘汰最旧（Map 头部）。只影响缓存命中，不影响正确性。
      while (cache.size > SPONSOR_CACHE_MAX) {
        const oldest = cache.keys().next().value;
        if (oldest === undefined) break;
        cache.delete(oldest);
      }
    }
    return outcome.segments;
  } finally {
    if (inflight.get(key)?.promise === run) inflight.delete(key);
  }
}

// 开关关闭或换曲时清理缓存（会话级），并同步状态指示。
export function clearSponsorCache() {
  cache.clear();
  inflight.clear();
  emitStatus({ state: "disabled", count: 0 });
}
