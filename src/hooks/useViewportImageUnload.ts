import { useEffect, useRef, useState } from "react";

/**
 * 轮 29（任务 B）：离屏封面位图 **主动卸载**。
 *
 * 背景（用户痛点：列表滚动越久，图片"一层层叠加"，峰值 400+MB）：
 *   Chromium / WebView2 的 ImageDecodeCache 是 LRU，只按"最近是否绘制"驱逐；
 *   `<img>` 离开视口甚至离开 DOM 都**不会**主动丢弃已解码位图
 *   （`content-visibility: auto`（轮 25）只跳过布局/绘制，同样不丢位图）。
 *   于是滚动过程中，越积越多的解码封面常驻在渲染进程里 —— 这就是"叠加感"的底层原因。
 *
 * 本钩子是 JS 侧**唯一**能主动让引擎丢弃位图的手段：对滚出视口较远的卡片，把
 * `<img src>` 换成 1px 占位（同轮 23 弹幕截断思路）——换 src 会让引擎释放旧位图；
 * 滚近时再还原原 src（命中 HTTP 缓存 / 磁盘缓存，重新解码）。
 *
 * 设计取舍：
 *  - **双阈值滞回**：卸载阈值 = 2 屏（外扩 rootMargin 200%），还原阈值 = 1 屏
 *    （外扩 100%）。两者之间只保持现状、不切换 → 边界附近不会来回抖。
 *    1 屏缓冲内**永不卸载**，回滚时封面已就绪。
 *  - **卸载防抖**：离开 2 屏后静置 `UNLOAD_DEBOUNCE_MS` 才真正卸载。快速滚动
 *   掠过（fling）不会触发 src 切换，避免「切占位 → 又还原」的重新解码抖动。
 *    还原**立即**执行，避免用户回滚时看到白块。
 *  - **滚动根**：Observer 的 root 取卡片最近的可滚动祖先（各抽屉的
 *    `.xxx-drawer-body`），而不是浏览器视口 —— 抽屉是固定定位的覆盖层，
 *    用视口做根会算错"滚出视口"。
 *  - **不新增依赖**（只用原生 IntersectionObserver / React），且不改动
 *    `usePreloadImages`（轮 26：每实例只预热一次，预载对象只在卸载时清理）。
 *
 * 滚动条稳定性：`.c-list-card` 已带 `contain-intrinsic-size: auto 180px`
 * （轮 25），`.c-cover` 高度固定 100px —— 换 1px 占位不改变盒子尺寸，
 * 滚动条不会跳变。
 */

/** 1×1 透明 GIF 占位（换掉 src 即可促使引擎丢弃原图位图）。 */
export const UNLOAD_PLACEHOLDER =
  "data:image/gif;base64,R0lGODlhAQABAIAAAAAAAP///yH5BAEAAAAALAAAAAABAAEAAAIBRAA7";

/** 卸载阈值：超出「N 屏」才卸载（rootMargin 正向外扩 N×100%）。 */
export const UNLOAD_FAR_SCREENS = 2;
/** 还原阈值：进入「N 屏」缓冲内立即还原，缓冲内永不卸载。 */
export const KEEP_NEAR_SCREENS = 1;
/** 卸载前静置时间（毫秒）：快速滚动掠过不触发切换。 */
export const UNLOAD_DEBOUNCE_MS = 200;

/** 找到卡片最近的可滚动祖先；找不到退回浏览器视口（root: null）。 */
function findScrollParent(start: HTMLElement): HTMLElement | null {
  let node: HTMLElement | null = start.parentElement;

  while (node && node !== document.body && node !== document.documentElement) {
    const overflowY = window.getComputedStyle(node).overflowY;

    if (
      overflowY === "auto" ||
      overflowY === "scroll" ||
      overflowY === "overlay"
    ) {
      return node;
    }
    node = node.parentElement;
  }

  return null;
}

/**
 * 把 `ref` 挂到卡片根元素上，返回该元素当前是否应显示 1px 占位。
 *
 * @param enabled 关闭时不注册任何 Observer（用于避免非列表场景空转）。
 */
export function useViewportImageUnload<T extends HTMLElement = HTMLDivElement>(
  enabled = true,
) {
  const ref = useRef<T | null>(null);
  const [unloaded, setUnloaded] = useState(false);

  useEffect(() => {
    if (!enabled) return;
    const el = ref.current;

    if (!el || typeof IntersectionObserver === "undefined") return;

    const root = findScrollParent(el);
    const margin = (screens: number) =>
      `${screens * 100}% 0px ${screens * 100}% 0px`;

    let timer: ReturnType<typeof setTimeout> | null = null;
    const clearTimer = () => {
      if (timer !== null) {
        clearTimeout(timer);
        timer = null;
      }
    };

    // 远界：离开「2 屏」外扩框 → 静置防抖后卸载。
    const far = new IntersectionObserver(
      (entries) => {
        const entry = entries[entries.length - 1];

        if (!entry) return;
        clearTimer();
        if (entry.isIntersecting) return;
        timer = setTimeout(() => {
          timer = null;
          setUnloaded(true);
        }, UNLOAD_DEBOUNCE_MS);
      },
      { root, rootMargin: margin(UNLOAD_FAR_SCREENS) },
    );

    // 近界：进入「1 屏」缓冲内 → 立即还原（避免白块），并取消在途卸载。
    const near = new IntersectionObserver(
      (entries) => {
        const entry = entries[entries.length - 1];

        if (!entry?.isIntersecting) return;
        clearTimer();
        setUnloaded(false);
      },
      { root, rootMargin: margin(KEEP_NEAR_SCREENS) },
    );

    far.observe(el);
    near.observe(el);

    return () => {
      clearTimer();
      far.disconnect();
      near.disconnect();
    };
  }, [enabled]);

  return { ref, unloaded };
}
