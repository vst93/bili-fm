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

/**
 * 图片组件：优先用 Wails binding 直接获取图片（绕过 HTTP 代理），
 * 失败时回退到 HTTP 代理 URL。
 * 解决 Windows 下 HTTP 代理连接不稳定的问题。
 */
export default function ProxyImg({
  src,
  fallbackSrc = "/cover.png",
  ...imgProps
}: ProxyImgProps) {
  const [imgSrc, setImgSrc] = useState<string>("");
  const [triedFetch, setTriedFetch] = useState(false);

  useEffect(() => {
    if (!src) {
      setImgSrc(fallbackSrc);
      return;
    }

    let cancelled = false;

    // 直接通过 Wails binding 获取图片（不走 HTTP 代理）
    // 从代理 URL 中提取原始图片 URL
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
        if (!cancelled) {
          setImgSrc(dataUrl);
          setTriedFetch(true);
        }
      })
      .catch(() => {
        // Wails binding 失败，回退到 HTTP 代理 URL
        if (!cancelled) {
          setImgSrc(src);
          setTriedFetch(true);
        }
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
