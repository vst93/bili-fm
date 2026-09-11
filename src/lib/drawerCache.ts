/**
 * 抽屉会话级缓存（React 外的普通 Map，不进 state、不落盘）。
 *
 * 关抽屉写入、重开命中即水合（含已翻页数据），DOM 照卸。
 *
 * 内存约束（本轮收紧，针对「滑动几页 + 开卡片」后 WebView 驻留膨胀）：
 *   1. 每个列表条目裁剪到 LIST_CACHE_MAX 条。列表运行态上限是 240
 *      （MAX_RETAINED_LIST_ITEMS），但缓存会在关抽屉后长期驻留，
 *      按「够用即可」取更小的值，避免整份 API 响应（含冗余字段）被钉住。
 *   2. LRU-3 之外再加一道全局条目预算 DRAWER_CACHE_TOTAL_ITEMS：
 *      3 个满条目（3×240 完整 API 行）叠加是峰值主因，预算保证
 *      「约 2 个整列表 + 1 个不满的列表」共存，超出则从最旧整条淘汰。
 *   3. 弹幕 / 评论条目单独裁剪（它们随播放不断累积，条数常远超 240）。
 * 条目只保留渲染所需数据的引用，不持有 DOM / 定时器 / 事件监听。
 */

export const DRAWER_CACHE_LIMIT = 3;
export const DRAWER_CACHE_TTL_MS = 15 * 60 * 1000;
/** 单个列表缓存条目上限（缓存态，小于运行态 240）。 */
export const LIST_CACHE_MAX = 120;
/** 全部 LRU 条目的条目数预算。3 条各 120 = 360 时只保留约 2 条满列表。 */
export const DRAWER_CACHE_TOTAL_ITEMS = 300;
/** 弹幕 / 评论在缓存态的独立上限。 */
export const DANMAKU_CACHE_MAX = 80;
export const REPLY_CACHE_MAX = 60;

export type DrawerCacheEntry = {
  items: unknown;
  extra: Record<string, unknown>;
  scrollTop: number;
  ts: number;
};

const drawerCache = new Map<string, DrawerCacheEntry>();

// 会话内可能实质变化的数据源（如收藏夹用户在别处增删）缓存 15 分钟过期。
const DRAWER_CACHE_VOLATILE: Record<string, boolean> = {
  collect: true,
  history: true,
  upVideo: true,
};

// 记录各抽屉滚动容器的实时 scrollTop（抽屉卸载后 DOM 查不到，只能靠滚动事件采集）。
const drawerScrollTops: Record<string, number> = {};

export const recordDrawerScroll = (key: string, top: number) => {
  drawerScrollTops[key] = top;
};

const entryItemCount = (items: unknown): number => {
  if (Array.isArray(items)) return items.length;
  if (items && typeof items === "object") {
    const inner = (items as { items?: unknown }).items;
    if (Array.isArray(inner)) return inner.length;
  }
  return 0;
};

// 兼容两种形状：数组本身，或 { items: [...] } 的接口响应。
const trimListContainer = (items: unknown, max: number): unknown => {
  if (Array.isArray(items)) return items.slice(0, max);
  if (
    items &&
    typeof items === "object" &&
    Array.isArray((items as { items?: unknown }).items)
  ) {
    const obj = items as { items: unknown[] };
    return { ...obj, items: obj.items.slice(0, max) };
  }
  return items;
};

/** 按缓存键把条目裁剪到内存预算内（导出以便 headless 断言）。 */
export const trimDrawerCacheEntry = (
  key: string,
  entry: DrawerCacheEntry,
): DrawerCacheEntry => {
  if (key === "danmaku") {
    return {
      ...entry,
      items: trimListContainer(entry.items, DANMAKU_CACHE_MAX),
      extra: {
        ...entry.extra,
        replyList: trimListContainer(entry.extra.replyList, REPLY_CACHE_MAX),
      },
    };
  }
  if (key === "recommend") {
    const payload = entry.items as {
      recommendList?: { items?: unknown[] };
      hotList?: { items?: unknown[] };
    } | null;
    const trimOne = (list: { items?: unknown[] } | undefined) =>
      list && Array.isArray(list.items)
        ? { ...list, items: list.items.slice(0, LIST_CACHE_MAX) }
        : list;
    return {
      ...entry,
      items: payload
        ? {
            ...payload,
            recommendList: trimOne(payload.recommendList),
            hotList: trimOne(payload.hotList),
          }
        : payload,
    };
  }
  const extra = { ...entry.extra };
  if (key === "history" && Array.isArray(extra.watchLaterList)) {
    extra.watchLaterList = (extra.watchLaterList as unknown[]).slice(
      0,
      LIST_CACHE_MAX,
    );
  }
  return {
    ...entry,
    items: trimListContainer(entry.items, LIST_CACHE_MAX),
    extra,
  };
};

// 全缓存条目数超预算时，从最旧（Map 头部）整条淘汰，至少保留最近一条。
const enforceTotalBudget = () => {
  const total = () => {
    let sum = 0;
    for (const entry of drawerCache.values()) sum += entryItemCount(entry.items);
    return sum;
  };
  for (const key of [...drawerCache.keys()]) {
    if (drawerCache.size <= 1 || total() <= DRAWER_CACHE_TOTAL_ITEMS) break;
    drawerCache.delete(key);
  }
};

export const writeDrawerCache = (
  key: string,
  items: unknown,
  extra: Record<string, unknown> = {},
) => {
  if (items === undefined || items === null) return;
  const entry = trimDrawerCacheEntry(key, {
    items,
    extra,
    scrollTop: drawerScrollTops[key] ?? 0,
    ts: Date.now(),
  });
  // LRU：删除后重新插入，保证 Map 迭代顺序里最新使用的在末尾。
  drawerCache.delete(key);
  drawerCache.set(key, entry);
  // 超 LRU 上限时淘汰最旧（Map 头部）的列表缓存。
  while (drawerCache.size > DRAWER_CACHE_LIMIT) {
    const oldestKey = drawerCache.keys().next().value;
    if (oldestKey === undefined) break;
    drawerCache.delete(oldestKey);
  }
  enforceTotalBudget();
};

const touchDrawerCache = (key: string) => {
  const entry = drawerCache.get(key);
  if (!entry) return;
  drawerCache.delete(key);
  drawerCache.set(key, entry);
};

export const readDrawerCache = (key: string): DrawerCacheEntry | null => {
  const entry = drawerCache.get(key);
  if (!entry) return null;
  if (DRAWER_CACHE_VOLATILE[key] && Date.now() - entry.ts > DRAWER_CACHE_TTL_MS) {
    drawerCache.delete(key);
    return null;
  }
  touchDrawerCache(key);
  return entry;
};

/** headless 测试用只读快照（生产代码不引用）。 */
export const __drawerCacheStats = () => ({
  size: drawerCache.size,
  keys: [...drawerCache.keys()],
  totalItems: [...drawerCache.values()].reduce(
    (sum, entry) => sum + entryItemCount(entry.items),
    0,
  ),
});
