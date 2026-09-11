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

// 会话级缓存：同一 BV+cid 只拉一次，失败也缓存空数组（无网时每次换歌只尝试一次，不重试）。
const cache = new Map<string, SponsorSegment[]>();
// 进行中的请求：并发调用复用同一 promise，避免重复打接口。
const inflight = new Map<string, Promise<SponsorSegment[]>>();

const cacheKey = (bvid: string, cid: number) => `${bvid}:${cid}`;

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

  const pending = inflight.get(key);
  if (pending) return pending;

  const run = (async (): Promise<SponsorSegment[]> => {
    const controller = new AbortController();
    const timer = setTimeout(() => controller.abort(), FETCH_TIMEOUT_MS);
    const onAbort = () => controller.abort();
    signal?.addEventListener("abort", onAbort);
    try {
      const url = `${SPONSOR_API}?videoID=${encodeURIComponent(
        bvid,
      )}&cid=${encodeURIComponent(String(cid))}`;
      const res = await fetch(url, { signal: controller.signal });
      if (!res.ok) return [];
      const json = await res.json();
      return parseSegments(json);
    } catch {
      // 网络 / 超时 / JSON 异常：静默降级为空片段。
      return [];
    } finally {
      clearTimeout(timer);
      signal?.removeEventListener("abort", onAbort);
    }
  })();

  inflight.set(key, run);
  try {
    const segments = await run;
    cache.set(key, segments);
    return segments;
  } finally {
    inflight.delete(key);
  }
}

// 开关关闭或换曲时清理缓存（会话级），避免残留状态。
export function clearSponsorCache() {
  cache.clear();
  inflight.clear();
}
