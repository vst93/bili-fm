import { useEffect } from "react";

/**
 * 预加载图片列表，在数据到达时立即开始下载。
 * 用户打开 Drawer 时图片已缓存，瞬间显示。
 * 仅预加载前 10 张（首屏可见区域），其余由 loading="lazy" 按需加载，
 * 避免一次性创建大量 Image 对象导致内存尖峰。
 */
export function usePreloadImages(urls: (string | undefined)[]) {
  useEffect(() => {
    const imgs: HTMLImageElement[] = [];
    const PRELOAD_LIMIT = navigator.userAgent.includes("Windows") ? 6 : 4;
    for (const url of urls.slice(0, PRELOAD_LIMIT)) {
      if (!url) continue;
      const img = new Image();
      img.loading = "eager";
      img.decoding = "async";
      img.src = url;
      imgs.push(img);
    }
    return () => {
      imgs.forEach((img) => {
        img.onload = null;
        img.onerror = null;
        img.removeAttribute("src");
      });
    };
  }, [urls]);
}
