import type { FC, ReactNode } from "react";
import { Comment, PreviewOpen } from "@icon-park/react";

/**
 * 统一的卡片 meta 行（所有列表卡片共用）。
 *
 * 展示规范（轮 19 依用户裁决修订）：
 * - 字段优先级（高 → 低）：作者 > 播放量 > 发布时间 > 时长 > 附加（进度等）；
 *   时长是最低优先级，已按行业惯例挪到封面右下角角标（见 .c-cover-duration），
 *   不再占用 meta 行空间；如个别卡片仍把时长留在 meta，行内排序也垫底。
 * - 单行、不换行：布局由 .card-meta 的 nowrap + overflow 保证；
 * - 过窄时由 globals.css 的容器查询从末尾按优先级隐藏整字段（先隐藏时长/发布时间），
 *   绝不换行或截断数值；
 * - 空值（undefined / null / 空串 / NaN）统一不渲染，不留空占位。
 */
export interface CardMetaField {
  kind: "author" | "views" | "pubdate" | "duration" | "extra";
  value?: string | number | null;
  /** 需要自定义内容时使用（默认播放量字段自带播放图标）。 */
  node?: ReactNode;
  title?: string;
  /**
   * views 字段的前导图标（轮 22）：默认 play；当播放量缺失、改展示弹幕数
   * 时用 "danmaku"（Comment 图标）避免「弹幕数挂播放图标」的语义错位。
   */
  icon?: "play" | "danmaku" | "none";
}

interface CardMetaProps {
  fields: CardMetaField[];
  /** 行首小胶囊（如「合作视频」）；由调用方决定是否传入。 */
  badge?: ReactNode;
  className?: string;
}

const isEmpty = (value: CardMetaField["value"]) =>
  value === undefined ||
  value === null ||
  value === "" ||
  (typeof value === "number" && !Number.isFinite(value));

const CardMeta: FC<CardMetaProps> = ({ fields, badge, className }) => {
  const visible = fields.filter(
    (field) => field.node !== undefined || !isEmpty(field.value),
  );
  if (visible.length === 0 && badge === undefined) return null;

  return (
    <span className={`card-meta${className ? ` ${className}` : ""}`}>
      {badge !== undefined ? (
        <span className="card-meta-badge">{badge}</span>
      ) : null}
      {visible.map((field) => (
        <span
          key={field.kind}
          className={`card-meta-field is-${field.kind}`}
          title={
            field.title ??
            // 作者名在窄卡片会被 ellipsis 截断，用 title 补全（轮 22）；
            // 其余字段（数值/日期）不再无意义地把值塞进 tooltip。
            (field.kind === "author" && typeof field.value === "string"
              ? field.value
              : undefined)
          }
        >
          {field.kind === "views" && (field.icon ?? "play") === "play" ? (
            <PreviewOpen size={12} theme="outline" />
          ) : null}
          {field.kind === "views" && field.icon === "danmaku" ? (
            <Comment size={12} theme="outline" />
          ) : null}
          {field.node !== undefined ? field.node : field.value}
        </span>
      ))}
    </span>
  );
};

export default CardMeta;
