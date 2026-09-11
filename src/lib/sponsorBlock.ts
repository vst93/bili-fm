// SponsorBlock（小电视空降助手）查询：只做「查询 + 跳过」，不做 submit/vote。
// 上游 bsbsb.top 免费无 key，CORS 全开，前端直连即可；任何失败静默降级为「无片段」，
// 绝不抛错打断播放。
export type SponsorSegment = {
  segment: [number, number];
  category: string;
  actionType: string;
  UUID: string;
  videoDuration: number;
};

// 只处理 skip 动作的片段。
const SPONSOR_API = "https://bsbsb.top/api/skipSegments";
const FETCH_TIMEOUT_MS = 3000;

// 会话级缓存：同一 BV+cid 只拉一次；仅缓存「真实服务端结论」（含空数组），
// 中止/超时/网络失败不缓存，以便换曲中止后同曲还能重新拉取。
const cache = new Map<string, SponsorSegment[]>();
// 进行中的请求：并发调用复用同一 promise，避免重复打接口。
// 记录发起该请求的 signal，便于判断能否安全复用（见下方 inflight 复用规则）。
type InflightEntry = { promise: Promise<FetchOutcome>; signal?: AbortSignal };
const inflight = new Map<string, InflightEntry>();

const cacheKey = (bvid: string, cid: number) => `${bvid}:${cid}`;

// 一次查询的结果：只有真正拿到服务端响应（含空数组）才算 cacheable。
// 中止/超时/网络失败必须视为「未得到结论」，绝不能写进会话缓存——
// 否则一次被 abort 的换曲请求会把该 BV:cid 永久钉死为空，后续同曲查询
// 直接命中空缓存，segments 永远为空，跳过功能整体失效。
type FetchOutcome = { segments: SponsorSegment[]; cacheable: boolean };

const parseSegments = (data: unknown): SponsorSegment[] => {
  if (!Array.isArray(data)) return [];
  const result: SponsorSegment[] = [];
  for (const item of data) {
    if (!item || typeof item !== "object") continue;
    const seg = item as Record<string, unknown>;
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
    const [start, end] = range;
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
  if (cached) return cached;

  // inflight 复用规则：仅当调用方与在途请求“同源”且未中止时才复用。
  // - 调用方没有 signal：可安全复用任意在途请求
  // - 调用方有 signal：只复用自己的在途请求（同一 signal 引用），
  //   否则一次被另一个 signal 中止的请求会把 doomed 结果传染给新调用方
  //   （React StrictMode 双挂载：挂载#1 的请求被 abort，挂载#2 若复用它
  //   就会永远拿到空 segments，跳过功能整体失效）。
  const pending = inflight.get(key);
  if (
    pending &&
    (!signal || pending.signal === signal) &&
    !(pending.signal?.aborted)
  ) {
    return pending.promise.then((outcome) => outcome.segments);
  }

  // externalAbort 标记调用方信号是否已中止：中止意味着「调用方不再需要本次结果」，
  // 绝不能把它当作服务端结论缓存下来。
  let externalAborted = false;
  const run = (async (): Promise<FetchOutcome> => {
    const controller = new AbortController();
    const timer = setTimeout(() => controller.abort(), FETCH_TIMEOUT_MS);
    const onAbort = () => {
      externalAborted = true;
      controller.abort();
    };
    signal?.addEventListener("abort", onAbort);
    try {
      const url = `${SPONSOR_API}?videoID=${encodeURIComponent(
        bvid,
      )}&cid=${encodeURIComponent(String(cid))}`;
      const res = await fetch(url, { signal: controller.signal });
      // 非 200（含 400/风控）不是有效结论，不缓存，允许重试。
      if (!res.ok) {
        // 失败可见性：此前完全无声，实机无法判断是网络、CSP 还是风控。
        // 仅 warn，不改变 UI 行为（仍降级为「无片段」）。
        console.warn(
          `[sponsor] segment query failed: HTTP ${res.status} for ${bvid}:${cid}`,
        );
        return { segments: [], cacheable: false };
      }
      const json = await res.json();
      return { segments: parseSegments(json), cacheable: true };
    } catch (error) {
      // 网络 / 超时 / 中止 / JSON 异常：静默降级为空片段，但不写缓存。
      // 仍打印一条 warn 便于实机诊断（不影响播放，不抛出）。
      console.warn(`[sponsor] segment query error for ${bvid}:${cid}`, error);
      return { segments: [], cacheable: false };
    } finally {
      clearTimeout(timer);
      signal?.removeEventListener("abort", onAbort);
    }
  })();

  // inflight 只复用「与调用方同源且未中止」的请求；
  // 中止的请求承诺会很快完成，但复用它会把 doomed 结果传染给并发调用者。
  inflight.set(key, { promise: run, signal });
  try {
    const outcome = await run;
    if (outcome.cacheable && !externalAborted) {
      cache.set(key, outcome.segments);
    }
    return outcome.segments;
  } finally {
    // 仅当自己仍是在途登记项时清除，避免误删后加入的请求。
    if (inflight.get(key)?.promise === run) inflight.delete(key);
  }
}

// 开关关闭或换曲时清理缓存（会话级），避免残留状态。
export function clearSponsorCache() {
  cache.clear();
  inflight.clear();
}
