/**
 * 抽屉会话级缓存（React 外的普通 Map，不进 state、不落盘）。
 *
 * 关抽屉写入、重开命中即水合（含已翻页数据），DOM 照卸。
 *
 * 内存约束（修复轮 21 再收紧）：
 *   1. 每个列表条目裁剪到 LIST_CACHE_MAX 条（本轮 120 → 80）。运行态上限
 *      也降到了 160（LIST_RETENTION_CAP），缓存态必须更小：它会在关抽屉后
 *      长期驻留，且只是「重开时快速水合」用，一两屏足够。
 *   2. LRU-2 之外再加一道全局条目预算 DRAWER_CACHE_TOTAL_ITEMS（本轮 300 → 160）。
 *      预算按「条目内所有列表数组」计（含 recommend 的 recommendList/hotList、
 *      history 的 watchLaterList、danmaku 的 replyList），不再只数顶层 items。
 *   3. 弹幕 / 评论条目单独裁剪（它们随播放不断累积，条数常远超列表上限）。
 * 条目只保留渲染所需数据的引用，不持有 DOM / 定时器 / 事件监听。
 */

export const DRAWER_CACHE_LIMIT = 2;
export const DRAWER_CACHE_TTL_MS = 15 * 60 * 1000;
/** 单个列表缓存条目上限（缓存态，严格小于运行态 160，只需够重开水合）。 */
export const LIST_CACHE_MAX = 80;
/** 全部 LRU 条目的条目数预算。约等于「两个满列表」共存，超出从最旧整条淘汰。 */
export const DRAWER_CACHE_TOTAL_ITEMS = 160;
/** 弹幕 / 评论在缓存态的独立上限。 */
export const DANMAKU_CACHE_MAX = 60;
export const REPLY_CACHE_MAX = 40;
/**
 * 轮 22：缓存窗口收紧为「单条目」。
 * 轮 20/21 只减小了「每条存多少、留几条」，但真正占内存的不是这些轻量 JS
 * 对象，而是 WebView2 已解码的图像位图 / blob 缓存（它们不随条目数下降）。
 * 与其保留 LRU-2（同时驻留两份数据 ── 两个满列表的封面 URL 都还在缓存里），
 * 不如关抽屉后只留最近一份：重开时命中的永远是用户刚看过的那个列表，
 * 其余整条删除。数值上把「常驻数据包」从 2 降到 1。
 */
export const DRAWER_CACHE_SINGLE_ENTRY = 1;

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

/**
 * 统计一个值里「列表形态」的条目总数。
 * 兼容数组本身、{ items: [...] } 响应，以及 { recommendList: {items}, hotList: {items} }
 * 这类多子列表负载 —— 旧实现只数顶层 items，导致 recommend 整条不计入预算。
 */
const countListItems = (value: unknown): number => {
  if (Array.isArray(value)) return value.length;
  if (value && typeof value === "object") {
    let sum = 0;
    for (const nested of Object.values(value as Record<string, unknown>)) {
      if (Array.isArray(nested)) {
        sum += nested.length;
      } else if (nested && typeof nested === "object") {
        const inner = (nested as { items?: unknown }).items;
        if (Array.isArray(inner)) sum += inner.length;
      }
    }
    return sum;
  }
  return 0;
};

/** 条目总条目数 = 主数据 + extra 里的附属列表（稍后再看 / 评论 / 合集等）。 */
const entryItemCount = (entry: DrawerCacheEntry): number =>
  countListItems(entry.items) + countListItems(entry.extra);

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
    for (const entry of drawerCache.values()) sum += entryItemCount(entry);
    return sum;
  };
  for (const key of [...drawerCache.keys()]) {
    if (drawerCache.size <= 1 || total() <= DRAWER_CACHE_TOTAL_ITEMS) break;
    drawerCache.delete(key);
  }
};

/** 只保留指定键（通常为刚关闭列表的键），删除其它所有缓存条目。
 *
 * 轮 22 内存策略：抽屉是「一次看一个」的交互，没必要让两个列表的数据 +
 * 封面一起常驻。关抽屉写入自己的缓存后调用本函数，把缓存窗口收敛为单条目；
 * 若 keepKey 不存在则直接清空。这样即使 LRU 容量是 2，实际常驻也不会超过 1 条。
 */
export const retainOnlyDrawerCache = (keepKey: string) => {
  const keep = drawerCache.get(keepKey);
  drawerCache.clear();
  if (keep) drawerCache.set(keepKey, keep);
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
    (sum, entry) => sum + entryItemCount(entry),
    0,
  ),
});
