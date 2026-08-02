import { useEffect } from "react";

/**
 * 预加载图片列表，在数据到达时立即开始下载。
 * 用户打开 Drawer 时图片已缓存，瞬间显示。
 */
export function usePreloadImages(urls: (string | undefined)[]) {
  useEffect(() => {
    const imgs: HTMLImageElement[] = [];
    for (const url of urls) {
      if (!url) continue;
      const img = new Image();
      img.src = url;
      imgs.push(img);
    }
    return () => {
      imgs.forEach((img) => {
        img.onload = null;
        img.onerror = null;
      });
    };
  }, [urls.join(",")]);
}
