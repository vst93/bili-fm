/**
 * 列表运行态「有界 + 可无限分页」策略（修复轮 21）。
 *
 * 背景（问题 3）：此前各列表把运行态条目硬截断到 MAX_RETAINED_LIST_ITEMS（240）。
 * 一旦到达上限，滚动事件里的「加载更多」守卫便永久短路 —— 用户表现为
 * 「滑动到一定页数之后，再也没有办法加载下一页」；而关掉抽屉再打开时，
 * 抽屉缓存里存的是裁剪过的副本（条数 < 上限），于是又能再加载一点。
 * 这是前端保留上限造成的假死，不是接口频率限制。
 *
 * 新策略：分页照常追加，但**超过上限时从头部（用户已经划过去的最旧条目）
 * 成批释放**，使单个列表在内存里恒定 ≤ 上限。这样既不封顶分页（可一直往下
 * 滑），也不让列表无限膨胀 —— 同时满足问题 2/4 对内存的诉求。
 *
 * 释放头部条目后，调用方需补偿滚动容器的 scrollTop（见 index.tsx 的
 * keepScrollAnchor），否则视口会向上跳动。
 */

/** 单个列表在运行态的内存上限（有界）。轮 25 由 160 再收到 128：
 *  头部释放配合下长期滚动不受影响，但单个列表驻留的条目 / 解码封面位图更少。 */
export const LIST_RETENTION_CAP = 128;

/**
 * 追加一页数据并在超出上限时从头部释放。
 * - 入参为 undefined / null 时按空数组处理；
 * - 返回新数组，绝不修改入参（避免与 React state 发生别名共享）。
 */
export function appendWithRetention<T>(
  current: readonly T[] | undefined | null,
  incoming: readonly T[] | undefined | null,
  cap: number = LIST_RETENTION_CAP,
): T[] {
  const prev = Array.isArray(current) ? current : [];
  const next = Array.isArray(incoming) ? incoming : [];
  const merged = prev.concat(next);
  if (merged.length <= cap) return merged;
  // 只保留最新的 cap 条：丢弃的是最旧的头部条目（无限滚动惯例）。
  return merged.slice(merged.length - cap);
}
