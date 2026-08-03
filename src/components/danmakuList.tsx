import type { FC } from "react";
import type { DanmakuItem, DanmakuList, ReplyList } from "@/types/bilibili";
import {
  useDisclosure,
  Button,
  Drawer,
  DrawerContent,
  DrawerBody,
  DrawerHeader,
  Tabs,
  Tab,
} from "@heroui/react";
import { useEffect, useRef, useMemo, useCallback, useState } from "react";
import { Connection, Refresh, Comment, Text } from "@icon-park/react";
import { useVirtualizer } from "@tanstack/react-virtual";
import { graftingImage } from "@/utils/string";

interface DanmakuListProps {
  danmakuList?: DanmakuList;
  replyList?: ReplyList;
  onSlideClick?: () => void;
  onDanmakuRefresh?: () => void;
  onReplyRefresh?: () => void;
  onReplyLoadMore?: () => void;
  isLoading?: boolean;
  currentTime?: number;
}

type ViewMode = "danmaku" | "reply";
interface DanmakuGroupEntry {
  danmaku: DanmakuItem;
  content: string;
  count: number;
}

interface DanmakuTimeGroup {
  second: number;
  entries: DanmakuGroupEntry[];
  totalCount: number;
}

const DANMAKU_OVERSCAN = 8;
const DANMAKU_ESTIMATED_ROW_HEIGHT = 112;

const DanmakuList: FC<DanmakuListProps> = ({
  danmakuList,
  replyList,
  onSlideClick,
  onDanmakuRefresh,
  onReplyRefresh,
  onReplyLoadMore,
  isLoading = false,
  currentTime = 0,
}) => {
  const { isOpen, onOpenChange } = useDisclosure({ isOpen: true });
  const listRef = useRef<HTMLDivElement>(null);
  const [isAutoScroll, setIsAutoScroll] = useState(true);
  const [viewMode, setViewMode] = useState<ViewMode>("danmaku");
  const lastScrollIndexRef = useRef<number>(-1);

  const handleOpenChange = useCallback(
    (open: boolean) => {
      if (!open) {
        lastScrollIndexRef.current = -1;
        setIsAutoScroll(true);
        onSlideClick?.();
      }
      onOpenChange();
    },
    [onSlideClick],
  );

  const handleViewModeChange = useCallback((key: string) => {
    setViewMode(key as ViewMode);
    setIsAutoScroll(true);
    lastScrollIndexRef.current = -1;
  }, []);

  const formatTime = useCallback((seconds: number): string => {
    const mins = Math.floor(seconds / 60);
    const secs = Math.floor(seconds % 60);
    return `${mins.toString().padStart(2, "0")}:${secs.toString().padStart(2, "0")}`;
  }, []);

  const formatDate = useCallback((timestamp: number): string => {
    const date = new Date(timestamp * 1000);
    return date.toLocaleString("zh-CN");
  }, []);

  const getColorStyle = useCallback((colorValue: number): string => {
    // 转换颜色值为RGB
    const r = (colorValue >> 16) & 0xff;
    const g = (colorValue >> 8) & 0xff;
    const b = colorValue & 0xff;

    // 计算亮度 (YIQ公式)
    const brightness = (r * 299 + g * 587 + b * 114) / 1000;

    // 检测系统是否为深色模式
    const isDarkMode = document.documentElement.classList.contains("dark");

    // 如果颜色太亮（接近白色），需要提供对比色
    if (brightness > 200) {
      if (isDarkMode) {
        // 深色模式下，亮色文字在深色背景上对比度很好
        return "#ffffff"; // 白色文字
      } else {
        // 浅色模式下，深灰色文字在浅色背景上提供良好对比度
        return "#1a1a1a"; // 深灰色，比纯黑色更柔和
      }
    }

    // 对于中等亮度颜色，如果接近背景色也提供对比
    if (brightness > 160 && brightness < 200) {
      if (isDarkMode && brightness > 180) {
        return "#e0e0e0"; // 浅灰色文字用于深色背景
      } else if (!isDarkMode && brightness < 170) {
        return "#333333"; // 深灰色文字用于浅色背景
      }
    }

    return `#${colorValue.toString(16).padStart(6, "0")}`;
  }, []);

  const groupedDanmaku = useMemo<DanmakuTimeGroup[]>(() => {
    const groups = new Map<number, Map<string, DanmakuGroupEntry>>();
    for (const danmaku of danmakuList?.items || []) {
      const second = Math.floor(danmaku.time);
      let entries = groups.get(second);
      if (!entries) {
        entries = new Map();
        groups.set(second, entries);
      }
      const normalizedContent = danmaku.content.trim().replace(/\s+/g, " ");
      const existing = entries.get(normalizedContent);
      if (existing) existing.count += 1;
      else
        entries.set(normalizedContent, {
          danmaku,
          content: normalizedContent,
          count: 1,
        });
    }

    return Array.from(groups, ([second, entries]) => {
      const sortedEntries = Array.from(entries.values()).sort(
        (a, b) => b.count - a.count,
      );
      return {
        second,
        entries: sortedEntries,
        totalCount: sortedEntries.reduce(
          (total, entry) => total + entry.count,
          0,
        ),
      };
    }).sort((a, b) => a.second - b.second);
  }, [danmakuList?.items]);

  const danmakuVirtualizer = useVirtualizer({
    count: groupedDanmaku.length,
    getScrollElement: () => listRef.current,
    estimateSize: () => DANMAKU_ESTIMATED_ROW_HEIGHT,
    getItemKey: (index) => groupedDanmaku[index].second,
    overscan: DANMAKU_OVERSCAN,
  });

  const sortedReplies = useMemo(
    () => [...(replyList?.items || [])].sort((a, b) => b.ctime - a.ctime),
    [replyList?.items],
  );

  const currentIndex = useMemo(() => {
    if (groupedDanmaku.length === 0 || currentTime < 0) return -1;
    let low = 0;
    let high = groupedDanmaku.length;
    while (low < high) {
      const middle = (low + high) >> 1;
      if (groupedDanmaku[middle].second < currentTime) low = middle + 1;
      else high = middle;
    }
    if (low === 0) return 0;
    if (low === groupedDanmaku.length) return low - 1;
    return currentTime - groupedDanmaku[low - 1].second <=
      groupedDanmaku[low].second - currentTime
      ? low - 1
      : low;
  }, [groupedDanmaku, currentTime]);

  const scrollToIndex = useCallback(
    (index: number) => {
      if (index < 0) return;
      danmakuVirtualizer.scrollToIndex(index, { align: "center" });
    },
    [danmakuVirtualizer],
  );

  useEffect(() => {
    if (
      viewMode === "danmaku" &&
      isOpen &&
      currentIndex >= 0 &&
      isAutoScroll &&
      currentIndex !== lastScrollIndexRef.current
    ) {
      scrollToIndex(currentIndex);
      lastScrollIndexRef.current = currentIndex;
    }
  }, [isOpen, currentIndex, isAutoScroll, scrollToIndex, viewMode]);

  const handleRefresh = useCallback(() => {
    if (viewMode === "danmaku") {
      onDanmakuRefresh?.();
    } else {
      onReplyRefresh?.();
    }
  }, [viewMode, onDanmakuRefresh, onReplyRefresh]);

  // Scroll handler for auto-loading more comments
  const handleScroll = useCallback(() => {
    if (!listRef.current) return;

    if (viewMode === "danmaku") {
      return;
    }

    if (!replyList?.has_more || isLoading) return;

    const { scrollTop, scrollHeight, clientHeight } = listRef.current;
    // Load more when within 100px of bottom
    if (scrollHeight - scrollTop - clientHeight < 100) {
      onReplyLoadMore?.();
    }
  }, [viewMode, replyList?.has_more, isLoading, onReplyLoadMore]);

  const renderDanmaku = useCallback(
    () => (
      <>
        <div
          className="relative w-full"
          style={{ height: danmakuVirtualizer.getTotalSize() }}
        >
          {danmakuVirtualizer.getVirtualItems().map((virtualRow) => {
            const index = virtualRow.index;
            const group = groupedDanmaku[index];
            return (
              <div
                key={group.second}
                ref={danmakuVirtualizer.measureElement}
                data-index={index}
                className="danmaku-time-row absolute left-0 right-0"
                style={{
                  transform: `translateY(${virtualRow.start}px)`,
                }}
              >
                <div
                  id={`danmaku-item-${index}`}
                  className={`danmaku-time-group ${index === currentIndex ? "is-current" : ""}`}
                >
                  <div className="danmaku-time-head">
                    <span className="danmaku-time-label">
                      {formatTime(group.second)}
                    </span>
                    <span className="danmaku-time-total">
                      {group.totalCount} 条
                    </span>
                  </div>
                  <div className="danmaku-message-list">
                    {group.entries.map(({ danmaku, content, count }) => (
                      <div className="danmaku-message-row" key={content}>
                        <span
                          className="danmaku-message-text"
                          style={{ color: getColorStyle(danmaku.color) }}
                        >
                          {content}
                        </span>
                        {count > 1 && (
                          <span className="danmaku-repeat-count">×{count}</span>
                        )}
                      </div>
                    ))}
                  </div>
                </div>
              </div>
            );
          })}
        </div>
        {groupedDanmaku.length === 0 && (
          <div className="text-center text-default-400 py-8">暂无弹幕</div>
        )}
      </>
    ),
    [
      groupedDanmaku,
      danmakuVirtualizer,
      currentIndex,
      getColorStyle,
      formatTime,
    ],
  );

  const getMemberInfo = useCallback((member: any) => {
    if (!member)
      return { uname: "匿名用户", face: "/default-avatar.png", level: 0 };
    // member can be an object with nested structure
    if (typeof member === "object") {
      // Handle both direct fields and nested level_info
      const levelInfo = member.level_info || member;
      // Bilibili API uses 'avatar' field for face image
      const avatarUrl = member.avatar || member.face;
      // Use graftingImage to proxy the avatar URL
      const faceUrl = avatarUrl
        ? graftingImage(avatarUrl, 96)
        : "/default-avatar.png";
      return {
        uname: member.uname || "匿名用户",
        face: faceUrl,
        level: levelInfo.current_level || levelInfo.level || 0,
      };
    }
    return { uname: "匿名用户", face: "/default-avatar.png", level: 0 };
  }, []);

  const renderReplies = useCallback(
    () => (
      <>
        {sortedReplies.map((reply, index) => {
          const memberInfo = getMemberInfo(reply.member);
          return (
            <div
              key={reply.rpid || index}
              id={`reply-item-${index}`}
              className="py-2 px-8 border-b border-default-100 hover:bg-default-50 dark:hover:bg-default-800/50 transition-colors duration-300"
            >
              <div className="flex flex-col gap-2">
                {/* 用户信息 */}
                <div className="flex items-center gap-2">
                  <img
                    src={memberInfo.face}
                    alt={memberInfo.uname}
                    className="w-8 h-8 rounded-full bg-default-200"
                    loading="lazy"
                    decoding="async"
                  />
                  <div className="flex flex-col">
                    <span className="text-sm font-medium text-default-700 dark:text-default-300">
                      {memberInfo.uname}
                    </span>
                    <span className="text-xs text-default-400">
                      Lv.{memberInfo.level} · {formatDate(reply.ctime)}
                    </span>
                  </div>
                </div>
                {/* 评论内容 */}
                <div className="pl-10">
                  <p className="text-sm leading-relaxed text-default-600 dark:text-default-400">
                    {reply.content?.message || ""}
                  </p>
                  {/* 点赞数 */}
                  {reply.like > 0 && (
                    <div className="mt-1 text-xs text-default-400">
                      👍 {reply.like}
                    </div>
                  )}
                  {/* 楼中楼回复预览 */}
                  {reply.replies && reply.replies.length > 0 && (
                    <div className="mt-2 pl-3 border-l-2 border-default-200 dark:border-default-700 space-y-2">
                      {reply.replies.map((subReply, subIndex) => {
                        const subMemberInfo = getMemberInfo(subReply.member);
                        return (
                          <div
                            key={subReply.rpid || subIndex}
                            className="text-xs"
                          >
                            <span className="font-medium text-primary-500">
                              {subMemberInfo.uname}:
                            </span>
                            <span className="ml-1 text-default-600 dark:text-default-400">
                              {subReply.content?.message || ""}
                            </span>
                          </div>
                        );
                      })}
                      {reply.replies.length >= 3 && (
                        <div className="text-xs text-default-400 cursor-pointer hover:text-primary-500">
                          查看更多回复...
                        </div>
                      )}
                    </div>
                  )}
                </div>
              </div>
            </div>
          );
        })}
        {sortedReplies.length === 0 && (
          <div className="text-center text-default-400 py-8">暂无评论</div>
        )}
        {isLoading && sortedReplies.length > 0 && (
          <div className="text-center text-default-400 py-2 text-xs">
            加载中...
          </div>
        )}
      </>
    ),
    [sortedReplies, formatDate, getMemberInfo, isLoading],
  );

  return (
    <Drawer
      classNames={{ base: "h-[92vh] max-h-[calc(100vh-54px)]" }}
      isOpen={isOpen}
      placement="bottom"
      onOpenChange={handleOpenChange}
    >
      <DrawerContent>
        {() => (
          <>
            <DrawerHeader className="flex gap-2 py-2 items-center border-b border-default-100">
              <Tabs
                variant="light"
                aria-label="弹幕/评论切换"
                selectedKey={viewMode}
                onSelectionChange={(key) => handleViewModeChange(key as string)}
                classNames={{
                  tabList: "gap-2",
                  cursor: "bg-primary-100",
                  tab: "px-3 h-8",
                }}
              >
                <Tab
                  key="danmaku"
                  title={
                    <div className="flex items-center gap-1">
                      <Text size={14} />
                      <span className="text-xs">
                        弹幕 ({danmakuList?.items?.length || 0})
                      </span>
                    </div>
                  }
                />
                <Tab
                  key="reply"
                  title={
                    <div className="flex items-center gap-1">
                      <Comment size={14} />
                      {replyList?.total_count ? (
                        <span className="text-xs">
                          评论 (共{replyList.total_count}条)
                        </span>
                      ) : sortedReplies.length > 0 ? (
                        <span className="text-xs">
                          评论 ({sortedReplies.length})
                        </span>
                      ) : (
                        <span className="text-xs">评论</span>
                      )}
                    </div>
                  }
                />
              </Tabs>
              <div className="flex gap-1">
                {viewMode === "danmaku" && (
                  <Button
                    isIconOnly
                    className={
                      isAutoScroll
                        ? "danmaku-follow-btn is-active"
                        : "danmaku-follow-btn"
                    }
                    size="sm"
                    variant="flat"
                    color="default"
                    onPress={() => setIsAutoScroll(!isAutoScroll)}
                    title={isAutoScroll ? "暂停跟随" : "开始跟随"}
                  >
                    <Connection size={18} fill="currentColor" />
                  </Button>
                )}
                <Button
                  isIconOnly
                  size="sm"
                  variant="light"
                  onPress={handleRefresh}
                  isLoading={isLoading}
                >
                  <Refresh
                    size={18}
                    className={isLoading ? "animate-spin" : ""}
                  />
                </Button>
              </div>
            </DrawerHeader>
            <DrawerBody className="danmaku-drawer-body p-0 m-0 overflow-hidden">
              <div
                ref={listRef}
                className="danmaku-scroll-area gap-0 overflow-y-auto h-full min-h-0 w-full pr-2"
                onScroll={handleScroll}
              >
                {viewMode === "danmaku" ? renderDanmaku() : renderReplies()}
              </div>
            </DrawerBody>
          </>
        )}
      </DrawerContent>
    </Drawer>
  );
};

export default DanmakuList;
