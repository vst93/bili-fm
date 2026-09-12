import { useLayoutEffect, useMemo, useRef, useState, type FC, type ReactNode } from "react";
import { Comment, PreviewOpen } from "@icon-park/react";
import { pickDateForm, pubdateCandidates } from "@/utils/string";

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

/**
 * 发布时间单元格（轮 30）：按**实测像素宽度**在「具体 → 抽象」的日期形态阶梯里
 * 选第一个放得下的形态。
 *
 * 轮 30 把「具体日期优先」交还给用户：跨年显示完整 yyyy-MM-DD、当年显示 MM-DD；
 * 只有当列宽确实放不下时，才逐档退回 yyyy-MM / N个月前（见 pubdateCandidates /  
 * pickDateForm）。判定依据是**真实渲染宽度**（在隐藏测量槽里量），而不是按字符数
 * 硬猜，因此窄卡片/宽松卡片都会自然落到合适的形态。
 *
 * 列宽弹性（见 globals.css .is-pubdate）：列基准仍是 30%，但在同行其它列内容短、
 * 整行有富余空间时可通过更高的 flex-grow「借」到更宽，从而容下 10 字符日期；
 * ResizeObserver 监听列盒尺寸，抽屉列数/窗口变化后自动重测。
 */
const PubDateField: FC<{ value: string; title?: string }> = ({ value, title }) => {
  const colRef = useRef<HTMLSpanElement>(null);
  const measureRef = useRef<HTMLSpanElement>(null);
  const candidates = useMemo(() => pubdateCandidates(value), [value]);
  const [text, setText] = useState(() => candidates[candidates.length - 1].text);

  useLayoutEffect(() => {
    const col = colRef.current;
    const measure = measureRef.current;
    if (!col || !measure) return;
    // 列自带 padding-left: 8px 是安全列距（border-box 计入列宽）—— 可用文本宽 =
    // clientWidth（含 padding）− 8px。
    const PADDING_LEFT = 8;
    const remeasure = () => {
      const available = col.clientWidth - PADDING_LEFT;
      if (available <= 0) {
        setText(candidates[candidates.length - 1].text);
        return;
      }
      const fits = (candidate: string) => {
        measure.textContent = candidate;
        return measure.offsetWidth <= available;
      };
      setText(pickDateForm(candidates, fits).text);
    };
    remeasure();
    if (typeof ResizeObserver === "undefined") return;
    const observer = new ResizeObserver(remeasure);
    observer.observe(col);
    return () => observer.disconnect();
  }, [candidates]);

  return (
    <span ref={colRef} className="card-meta-field is-pubdate" title={title}>
      {text}
      {/* 隐藏测量槽：绝对定位、visibility:hidden（不影响布局/可访问性），
          只用来量候选形态的真实像素宽度。 */}
      <span ref={measureRef} className="card-meta-pubdate-measure" aria-hidden="true" />
    </span>
  );
};

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
      {visible.map((field) => {
        const title =
          field.title ??
          // 作者名在窄卡片会被 ellipsis 截断，用 title 补全（轮 22）；
          // 发布时间同理（轮 28）：日期若真的被截，悬浮仍可看完整值。
          // 其余字段（播放量等）不再无意义地把值塞进 tooltip。
          (field.kind === "author" && typeof field.value === "string"
            ? field.value
            : field.kind === "pubdate" && typeof field.value === "string"
              ? field.value
              : undefined);

        // 发布时间字符串：走「实测宽度降级」单元格（轮 30）。
        if (field.kind === "pubdate" && typeof field.value === "string") {
          return <PubDateField key={field.kind} value={field.value} title={title} />;
        }

        return (
          <span
            key={field.kind}
            className={`card-meta-field is-${field.kind}`}
            title={title}
          >
            {field.kind === "views" && (field.icon ?? "play") === "play" ? (
              <PreviewOpen size={12} theme="outline" />
            ) : null}
            {field.kind === "views" && field.icon === "danmaku" ? (
              <Comment size={12} theme="outline" />
            ) : null}
            {field.node !== undefined ? field.node : field.value}
          </span>
        );
      })}
    </span>
  );
};

export default CardMeta;
