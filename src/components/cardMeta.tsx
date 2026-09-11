import type { FC, ReactNode } from "react";
import { PreviewOpen } from "@icon-park/react";

/**
 * 统一的卡片 meta 行（所有列表卡片共用）。
 *
 * 展示规范：
 * - 字段按优先级排序（高 → 低）：作者 > 播放量 > 时长 > 发布时间 > 附加（进度等）；
 * - 单行、不换行：布局由 .card-meta 的 nowrap + overflow 保证；
 * - 过窄时由 globals.css 的容器查询从末尾按优先级隐藏整字段，绝不换行或截断数值；
 * - 空值（undefined / null / 空串 / NaN）统一不渲染，不留空占位。
 */
export interface CardMetaField {
  kind: "author" | "views" | "duration" | "pubdate" | "extra";
  value?: string | number | null;
  /** 需要自定义内容时使用（默认播放量字段自带播放图标）。 */
  node?: ReactNode;
  title?: string;
}

interface CardMetaProps {
  fields: CardMetaField[];
  className?: string;
}

const isEmpty = (value: CardMetaField["value"]) =>
  value === undefined ||
  value === null ||
  value === "" ||
  (typeof value === "number" && !Number.isFinite(value));

const CardMeta: FC<CardMetaProps> = ({ fields, className }) => {
  const visible = fields.filter(
    (field) => field.node !== undefined || !isEmpty(field.value),
  );
  if (visible.length === 0) return null;

  return (
    <span className={`card-meta${className ? ` ${className}` : ""}`}>
      {visible.map((field) => (
        <span
          key={field.kind}
          className={`card-meta-field is-${field.kind}`}
          title={
            field.title ??
            (typeof field.value === "string" ? field.value : undefined)
          }
        >
          {field.kind === "views" ? (
            <PreviewOpen size={12} theme="outline" />
          ) : null}
          {field.node !== undefined ? field.node : field.value}
        </span>
      ))}
    </span>
  );
};

export default CardMeta;
