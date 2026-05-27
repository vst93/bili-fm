import { useEffect, useState } from "react";
import { Image } from "@heroui/react";
import { FetchImage } from "../../wailsjs/go/service/BL";

interface ProxyImgProps {
  src: string;
  fallbackSrc?: string;
  alt?: string;
  className?: string;
  width?: string;
  radius?: "none" | "sm" | "md" | "lg" | "full";
  shadow?: "none" | "sm" | "md" | "lg";
  loading?: "lazy" | "eager";
  crossOrigin?: string;
}

const isWindows = navigator.userAgent.includes("Windows");

/**
 * 图片组件：
 * - Windows: 通过 Wails binding 直接获取图片（绕过不稳定的 HTTP 代理）
 * - Mac/Linux: 直接使用 HTTP 代理 URL（原逻辑，性能更优）
 */
export default function ProxyImg({
  src,
  fallbackSrc = "/cover.png",
  ...imgProps
}: ProxyImgProps) {
  const [imgSrc, setImgSrc] = useState<string>("");

  useEffect(() => {
    if (!src) {
      setImgSrc(fallbackSrc);
      return;
    }

    // Mac/Linux: 直接用 HTTP 代理 URL，不走 Wails binding
    if (!isWindows) {
      setImgSrc(src);
      return;
    }

    // Windows: 通过 Wails binding 获取图片，绕过 HTTP 代理
    let cancelled = false;
    let originalUrl = src;
    try {
      const u = new URL(src);
      const urlParam = u.searchParams.get("url");
      if (urlParam) originalUrl = urlParam;
    } catch {
      // src 不是 URL，直接用
    }

    FetchImage(originalUrl)
      .then((dataUrl: string) => {
        if (!cancelled) setImgSrc(dataUrl);
      })
      .catch(() => {
        if (!cancelled) setImgSrc(src); // 回退到 HTTP 代理
      });

    return () => {
      cancelled = true;
    };
  }, [src]);

  return (
    <Image
      {...imgProps}
      src={imgSrc || fallbackSrc}
      fallbackSrc={fallbackSrc}
      loading="lazy"
    />
  );
}
