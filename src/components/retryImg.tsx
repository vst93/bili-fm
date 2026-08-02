import { useCallback, useState } from "react";

interface RetryImgProps {
  src?: string;
  fallbackSrc?: string;
  alt?: string;
  className?: string;
  width?: string | number;
  radius?: "none" | "sm" | "md" | "lg" | "full";
  shadow?: "none" | "sm" | "md" | "lg";
  id?: string;
  onClick?: () => void;
  [key: string]: any;
}

/**
 * 图片组件：纯原生 <img>，加载失败自动重试。
 * 无 wrapper div，避免与 .c-cover 的 position:absolute 冲突。
 */
export default function RetryImg({
  src,
  fallbackSrc = "/cover.png",
  alt,
  className,
  id,
  onClick,
  width,
}: RetryImgProps) {
  const [retryKey, setRetryKey] = useState(0);

  const handleError = useCallback(() => {
    if (retryKey < 3) {
      setTimeout(() => setRetryKey((k) => k + 1), 300 * (retryKey + 1));
    }
  }, [retryKey]);

  const retrySuffix = retryKey > 0 ? `${src?.includes("?") ? "&" : "?"}_r=${retryKey}` : "";
  const finalSrc = retryKey >= 3 ? fallbackSrc : (src || fallbackSrc) + retrySuffix;

  return (
    <img
      key={`${src}-${retryKey}`}
      id={id}
      alt={alt}
      src={finalSrc}
      className={className}
      onClick={onClick}
      style={{ width: width || "100%" }}
      onLoad={() => {}}
      onError={handleError}
    />
  );
}
