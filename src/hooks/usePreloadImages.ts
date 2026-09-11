import { useEffect } from "react";

/**
 * 预加载图片列表，在数据到达时立即开始下载。
 * 用户打开 Drawer 时首屏图片已缓存，瞬间显示。
 *
 * 仅预加载首屏前几张（Windows 3 / 其它平台 2），其余交给 loading="lazy" 按需加载，
 * 避免一次性解码大量封面位图导致内存尖峰（轮 22：6→3 / 4→2；轮 20 已从「全部」收到 6）。
 *
 * 轮 22 关键修复：cleanup 时除了断开回调，还要**真正释放已解码的位图**。
 * 此前只 `removeAttribute("src")`，但当 `decode()` 已完成的位图仍被浏览器
 * 图像缓存持有，反复开关抽屉会持续累积 —— 这是轮 20/21 只缩条数、
 * 却压不住 WebView2 进程内存的原因之一。现在：
 *   - blob: 源显式 revokeObjectURL；
 *   - 其余源先置空 src 中断在途请求，再指向 1px data URI 让浏览器回收位图。
 */
export function usePreloadImages(urls: (string | undefined)[]) {
  useEffect(() => {
    const imgs: HTMLImageElement[] = [];
    const PRELOAD_LIMIT = navigator.userAgent.includes("Windows") ? 3 : 2;
    for (const url of urls.slice(0, PRELOAD_LIMIT)) {
      if (!url) continue;
      const img = new Image();
      img.loading = "eager";
      img.decoding = "async";
      img.fetchPriority = "low";
      img.src = url;
      imgs.push(img);
    }
    return () => {
      imgs.forEach((img) => {
        img.onload = null;
        img.onerror = null;
        if (img.src.startsWith("blob:")) URL.revokeObjectURL(img.src);
        img.removeAttribute("src");
        // 指向 1px 占位，促使浏览器丢弃已解码位图（仅 removeAttribute 不够）。
        img.src = "data:,";
      });
    };
  }, [urls]);
}
