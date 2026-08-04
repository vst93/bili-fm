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
  return (
    // Shared card images cannot gain a wrapper without breaking .c-cover positioning.
    // eslint-disable-next-line jsx-a11y/no-noninteractive-element-interactions
    <img
      id={id}
      alt={alt}
      src={src || fallbackSrc}
      className={className}
      onClick={onClick}
      onKeyDown={
        onClick
          ? (event) => {
              if (event.key === "Enter" || event.key === " ") onClick();
            }
          : undefined
      }
      role={onClick ? "button" : undefined}
      tabIndex={onClick ? 0 : undefined}
      style={{ width: width || "100%" }}
      loading="lazy"
      decoding="async"
      fetchPriority="low"
      onError={(event) => {
        const image = event.currentTarget;
        if (!image.src.endsWith(fallbackSrc)) image.src = fallbackSrc;
      }}
    />
  );
}
