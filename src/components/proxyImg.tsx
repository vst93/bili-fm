import { useEffect, useRef, useState } from "react";
import { Image } from "@heroui/react";
import { invoke } from "@tauri-apps/api/core";

interface ProxyImgProps {
  src?: string;
  fallbackSrc?: string;
  alt?: string;
  className?: string;
  classNames?: Record<string, string>;
  width?: string | number;
  height?: string | number;
  radius?: "none" | "sm" | "md" | "lg" | "full";
  shadow?: "none" | "sm" | "md" | "lg";
  loading?: "lazy" | "eager";
  crossOrigin?: "anonymous" | "use-credentials" | "";
  id?: string;
  onClick?: () => void;
  [key: string]: any;
}

// 图片缓存：原始URL -> dataURL，避免重复请求
const imgCache = new Map<string, string>();

/**
 * 图片组件：优先用 Tauri command (fetch_image) 直接获取图片转 base64，
 * 绕过 HTTP 代理和 WebView 安全限制。
 * 失败时回退到 HTTP 代理 URL。
 */
export default function ProxyImg({
  src,
  fallbackSrc = "/cover.png",
  ...imgProps
}: ProxyImgProps) {
  const [imgSrc, setImgSrc] = useState<string>(
    () => imgCache.get(src || "") || ""
  );
  const lastSrcRef = useRef<string | undefined>(undefined);

  useEffect(() => {
    if (!src) {
      setImgSrc(fallbackSrc);
      lastSrcRef.current = undefined;
      return;
    }

    if (src === lastSrcRef.current) return;
    lastSrcRef.current = src;

    // 检查缓存
    if (imgCache.has(src)) {
      setImgSrc(imgCache.get(src)!);
      return;
    }

    let cancelled = false;

    // 从代理 URL 中提取原始图片 URL
    let originalUrl = src;
    try {
      const u = new URL(src);
      const urlParam = u.searchParams.get("url");
      if (urlParam) originalUrl = urlParam;
    } catch {
      // 相对 URL（如 /image-proxy?url=xxx）无法用 new URL() 解析，手动提取 url 参数
      const match = src.match(/[?&]url=([^&]+)/);
      if (match) {
        try {
          originalUrl = decodeURIComponent(match[1]);
        } catch {
          originalUrl = match[1];
        }
      }
    }

    // 通过 Tauri command 获取图片（Rust 后端直接 HTTP 请求，绕过 WebView）
    invoke<string>("fetch_image", { url: originalUrl })
      .then((dataUrl: string) => {
        if (!cancelled && dataUrl) {
          imgCache.set(src, dataUrl);
          setImgSrc(dataUrl);
        }
      })
      .catch(() => {
        // Tauri command 失败，回退到原始 URL（可能是代理 URL）
        if (!cancelled) {
          setImgSrc(src);
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
