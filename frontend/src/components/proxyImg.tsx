import { useEffect, useRef, useState } from "react";
import { Image } from "@heroui/react";

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

/**
 * 图片组件：统一使用本地 HTTP 代理加载 B站 CDN 图片。
 * 代理已优化：DNS 缓存 + IPv4 强制 + 连接池，所有平台通用。
 */
export default function ProxyImg({
  src,
  fallbackSrc = "/cover.png",
  ...imgProps
}: ProxyImgProps) {
  const [imgSrc, setImgSrc] = useState<string>(src || fallbackSrc);
  const lastSrcRef = useRef<string | undefined>(undefined);

  useEffect(() => {
    if (!src) {
      setImgSrc(fallbackSrc);
      lastSrcRef.current = undefined;
      return;
    }

    if (src === lastSrcRef.current) return;
    lastSrcRef.current = src;
    setImgSrc(src);
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
