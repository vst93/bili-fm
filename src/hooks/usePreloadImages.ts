import { useEffect, useRef } from "react";

/**
 * 列表封面「首屏预热」钩子（修复轮 26 重写）。
 *
 * 背景（用户反馈：不停翻滚之后图片叠加太多）：本钩子此前把依赖数组设为
 * `[urls]`，而列表每翻一页都会生成新的 `coverUrls` 数组 —— 于是**每一次
 * 加载更多都会重新跑一遍 effect**：cleanup 释放上一批 Image、effect 再
 * `new Image()` 拉一批、再清理…… 滚动越久，创建/销毁的 Image 越多，且每次
 * 都重复请求同一批封面 URL，与卡片 `<img>` 自身形成两套并行的图片加载/解码
 * 生命周期，是「滚动时图片一层层叠加」的一个来源。
 *
 * 轮 26 收敛为「每个列表实例只在首屏数据到达时预热一次」：
 *   - 用 ref 守卫，effect 再触发也直接短路 —— 翻页继续时**不再新建 Image、
 *     不再重复拉取同一批封面**；首屏预热的效果与之前完全一致（打开即秒显）；
 *   - 只预热首屏前几张（Windows 3 / 其它平台 2），其余交给卡片自身的
 *     `loading="lazy"` 按需加载；
 *   - 卸载 / 换页时释放引用即可：中断在途请求 + 清空 src，位图统一交给
 *     浏览器图片缓存管理。`data:` 占位只在真正卸载（cleanup 唯一一次）时
 *     使用一次，不再随滚动反复驱逐位图 —— 否则会让仍在视口内的卡片被
 *     反复重新解码，反而放大滚动时的解码抖动。
 *
 * 轮 31（用户方向②：限制并发加载、优先可见区域）：预热对象虽已压到前几张，
 * 但旧实现是「一次性把这几张全部置 src」—— 它们会在同一瞬间并行解码，短暂叠加
 * 几张解码位图。改为 **最多 `PRELOAD_CONCURRENCY` 张在途**、完成一张再补一张，
 * 进一步压低切歌 / 首次滚动瞬间的解码位图峰值。预热顺序仍从列表头部开始，
 * 而列表头部正是首屏可见区域 → 天然「优先可见区域」。
 *
 * 这样第一屏的用户体验不变，但「不停翻滚」不再产生叠加的预载对象。
 */

/** 预热并发上限（轮 31）：同一时刻在途的解码位图数量以此封顶。 */
export const PRELOAD_CONCURRENCY = 2;

export function usePreloadImages(urls: (string | undefined)[]) {
  // 每个组件实例只预热一次（抽屉关闭即卸载 → 重开时新实例再预热一次）。
  const preloadedRef = useRef(false);

  useEffect(() => {
    if (preloadedRef.current) return;
    if (!urls || urls.length === 0) return;
    preloadedRef.current = true;

    const PRELOAD_LIMIT = navigator.userAgent.includes("Windows") ? 3 : 2;
    // 只取首屏前几张（= 列表头部的可见区域），其余交给卡片自身的 loading="lazy"。
    const limited = urls.slice(0, PRELOAD_LIMIT);
    const queue = limited.filter((url): url is string => !!url);

    const imgs: HTMLImageElement[] = [];
    let cursor = 0;
    let active = 0;
    let cancelled = false;

    // 有界并发：同一时刻最多 PRELOAD_CONCURRENCY 张在途，完成一张再补一张。
    const pump = () => {
      while (
        !cancelled &&
        active < PRELOAD_CONCURRENCY &&
        cursor < queue.length
      ) {
        const img = new Image();

        img.loading = "eager";
        img.decoding = "async";
        img.fetchPriority = "low";
        const release = () => {
          img.onload = null;
          img.onerror = null;
          active -= 1;
          pump();
        };

        img.onload = release;
        img.onerror = release;
        img.src = queue[cursor];
        cursor += 1;
        active += 1;
        imgs.push(img);
      }
    };

    pump();

    return () => {
      // 断掉引用并中断在途请求；置 1px 占位促使浏览器丢弃已解码位图。
      // 有了上面的 preloadedRef 守卫，本 cleanup 只在卸载时跑一次，
      // 不会在滚动翻页过程中反复驱逐仍在视口内的封面。
      cancelled = true;
      imgs.forEach((img) => {
        img.onload = null;
        img.onerror = null;
        if (img.src.startsWith("blob:")) URL.revokeObjectURL(img.src);
        img.removeAttribute("src");
        img.src = "data:,";
      });
      imgs.length = 0;
    };
  }, [urls]);
}
