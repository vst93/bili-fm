import type { FC, ReactNode } from "react";

import { Card, CardBody, CardFooter } from "@heroui/react";

import CardMeta, { type CardMetaField } from "./cardMeta";
import RetryImg from "./retryImg";

import { graftingImage } from "@/utils/string";
import {
  UNLOAD_PLACEHOLDER,
  useViewportImageUnload,
} from "@/hooks/useViewportImageUnload";

/**
 * 统一的列表卡片组件（轮 25 组件化）。
 *
 * 背景：8 个列表（feed/recommend/collect/upVideo/series/search/page/history）
 * 各自手写一份结构相同的 HeroUI 卡片（Card + CardBody(.img-container) +
 * RetryImg(.c-cover) + 时长角标(.c-cover-duration) + CardFooter + CardMeta），
 * 出现「各列表卡片宽度不一」「数字与日期粘连」等不一致。抽出本组件后：
 *   - 卡片 DOM 结构、类名与交互钩子完全一致（含 HeroUI 的
 *     `[data-slot="wrapper"] [role="button"][class*="bg-content"]` 选择器命中、
 *     `.c-list-card` 的 content-visibility、`.img-container > .c-cover` 定位）；
 *   - 封面统一走 graftingImage 默认宽度（240w，用户已验收，不再改动）；
 *   - meta 行统一走 CardMeta（单行不换行、比例列、优先级隐藏）。
 *
 * 刻意保持的「非目标」：每个列表仍各自 import CardMeta 之外的字段构造逻辑；
 * 本组件不接管数据解析，只统一渲染。
 */
export interface ListCardProps {
  /** 点击卡片（isPressable 的 onPress）。 */
  onPress?: () => void;
  /** 卡片标题。 */
  title?: string;
  /** 标题前缀（如 feedList 的「充电专属」红胶囊）。 */
  titlePrefix?: ReactNode;
  /** 标题元素类名覆盖（pageList 需要额外的 part-title）。 */
  titleClassName?: string;
  /** 封面原始 URL；组件内部走 graftingImage 默认 240w。 */
  cover?: string;
  /** 封面替代文本。 */
  coverAlt?: string;
  /** 封面右下角时长角标文本（null / undefined / "" 时不渲染）。 */
  duration?: string | number | null;
  /** 封面之上的附加内容（pageList 的「正在播放」胶囊、historyList 的移除按钮等）。 */
  coverChildren?: ReactNode;
  /** meta 行字段（透传 CardMeta）。 */
  fields: CardMetaField[];
  /** 卡片根类名（默认 c-list-card；可附加 border-2 等状态类）。 */
  cardClassName?: string;
  /** CardBody 类名（默认与各列表一致）。 */
  bodyClassName?: string;
}

const ListCard: FC<ListCardProps> = ({
  onPress,
  title,
  titlePrefix,
  titleClassName = "line-clamp-1 text-left w-full max-h-12 overflow-hidden",
  cover,
  coverAlt,
  duration,
  coverChildren,
  fields,
  cardClassName = "c-list-card",
  bodyClassName = "overflow-visible p-0 img-container",
}) => {
  // 轮 29（任务 B）：滚出视口 ≥2 屏后把封面换成 1px 占位（主动让引擎丢弃解码
  // 位图），进入 1 屏缓冲立即还原。`ref` 挂在卡片根（HeroUI Card 的 DOM 节点），
  // 不改动卡片结构、类名与 DOM 钩子（content-visibility / 定位 / 选择器均不依赖 ref）。
  const { ref: cardRef, unloaded } = useViewportImageUnload<HTMLDivElement>();
  const coverSrc = graftingImage(cover ?? "");

  return (
    <Card
      ref={cardRef}
      isPressable
      className={cardClassName}
      shadow="sm"
      onPress={onPress}
    >
      <CardBody className={bodyClassName}>
        <RetryImg
          alt={coverAlt ?? title}
          className="c-cover"
          fallbackSrc="/cover.png"
          loading="lazy"
          radius="sm"
          shadow="sm"
          src={unloaded ? UNLOAD_PLACEHOLDER : coverSrc}
          width="100%"
        />
        {duration !== undefined && duration !== null && duration !== "" ? (
          <span className="c-cover-duration">{duration}</span>
        ) : null}
        {coverChildren}
      </CardBody>
      <CardFooter className="text-small flex-col items-start px-2 py-1">
        <b className={titleClassName} title={title}>
          {titlePrefix}
          {title}
        </b>
        <div className="text-default-500 text-left w-full text-xs mt-1 line-clamp-1 max-h-10">
          <CardMeta fields={fields} />
        </div>
      </CardFooter>
    </Card>
  );
};

export default ListCard;
