import type { FC } from "react";
import type { service as blSer } from "../../wailsjs/go/models";
import { useDisclosure, Button, Drawer, DrawerContent, DrawerBody, DrawerHeader } from "@heroui/react";
import { useEffect, useRef, useMemo, useCallback, useState } from "react";
import { Connection, Refresh } from "@icon-park/react";

interface DanmakuListProps {
  danmakuList?: blSer.DanmakuList;
  onSlideClick?: () => void;
  onRefresh?: () => void;
  isLoading?: boolean;
  currentTime?: number;
}

const DanmakuList: FC<DanmakuListProps> = ({
  danmakuList,
  onSlideClick,
  onRefresh,
  isLoading = false,
  currentTime = 0,
}) => {
  const { isOpen, onOpenChange } = useDisclosure({ isOpen: true });
  const listRef = useRef<HTMLDivElement>(null);
  const [isAutoScroll, setIsAutoScroll] = useState(true);
  const lastScrollIndexRef = useRef<number>(-1);

  const handleOpenChange = useCallback((open: boolean) => {
    if (!open) {
      lastScrollIndexRef.current = -1;
      setIsAutoScroll(true);
      onSlideClick?.();
    }
    onOpenChange();
  }, [onSlideClick]);

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
    if (colorValue >= 16777210) return "#000000";
    return `#${colorValue.toString(16).padStart(6, "0")}`;
  }, []);

  const getFontSize = useCallback((fontSize: number): string => {
    switch (fontSize) {
      case 18: return "text-xs";
      case 36: return "text-lg";
      default: return "text-sm";
    }
  }, []);

  const getTypeName = useCallback((type: number): string => {
    switch (type) {
      case 1: case 2: case 3: return "滚动";
      case 4: return "底部";
      case 5: return "顶部";
      case 6: return "逆向";
      default: return "普通";
    }
  }, []);

  const sortedDanmaku = useMemo(() =>
    [...(danmakuList?.items || [])].sort((a, b) => a.time - b.time),
    [danmakuList?.items]
  );

  const currentIndex = useMemo(() => {
    if (sortedDanmaku.length === 0 || currentTime <= 0) return -1;
    let closestIndex = 0;
    let minDiff = Math.abs(sortedDanmaku[0].time - currentTime);
    for (let i = 1; i < sortedDanmaku.length; i++) {
      const diff = Math.abs(sortedDanmaku[i].time - currentTime);
      if (diff < minDiff) {
        minDiff = diff;
        closestIndex = i;
      }
    }
    return closestIndex;
  }, [sortedDanmaku, currentTime]);

  const smoothScrollTo = useCallback((index: number) => {
    if (index < 0 || !listRef.current) return;
    const element = document.getElementById(`danmaku-item-${index}`);
    if (!element) return;

    const listHeight = listRef.current.clientHeight;
    const elementTop = element.offsetTop;
    const elementHeight = element.offsetHeight;
    const targetScroll = elementTop - listHeight / 2 + elementHeight / 2;

    const currentScroll = listRef.current.scrollTop;
    const scrollDiff = Math.abs(targetScroll - currentScroll);

    if (scrollDiff < 10) return;

    listRef.current.scrollTo({
      top: targetScroll,
      behavior: "smooth"
    });
  }, []);

  useEffect(() => {
    if (isOpen && currentIndex >= 0 && isAutoScroll) {
      const timer = setTimeout(() => {
        smoothScrollTo(currentIndex);
        lastScrollIndexRef.current = currentIndex;
      }, 500);
      return () => clearTimeout(timer);
    }
  }, [isOpen, currentIndex, isAutoScroll, smoothScrollTo]);

  useEffect(() => {
    if (isOpen && currentIndex >= 0 && isAutoScroll && currentIndex !== lastScrollIndexRef.current) {
      smoothScrollTo(currentIndex);
      lastScrollIndexRef.current = currentIndex;
    }
  }, [isOpen, currentIndex, isAutoScroll, smoothScrollTo]);

  return (
    <Drawer classNames={{ base: "h-[80vh]" }} isOpen={isOpen} placement="bottom" onOpenChange={handleOpenChange}>
      <DrawerContent>
        {() => (
          <>
            <DrawerHeader className="flex gap-2 py-2 items-center">
              <span>弹幕列表 ({sortedDanmaku.length})</span>
              <div className="flex gap-2">
                <Button
                  isIconOnly
                  size="sm"
                  variant={isAutoScroll ? "solid" : "light"}
                  color={isAutoScroll ? "primary" : "default"}
                  onPress={() => setIsAutoScroll(!isAutoScroll)}
                  title={isAutoScroll ? "暂停跟随" : "开始跟随"}
                >
                  <Connection size={18} fill={isAutoScroll ? "#fff" : "#68bca4"} />
                </Button>
                <Button
                  isIconOnly
                  size="sm"
                  variant="light"
                  onPress={onRefresh}
                  isLoading={isLoading}
                >
                  <Refresh size={18} className={isLoading ? "animate-spin" : ""} />
                </Button>
              </div>
            </DrawerHeader>
            <DrawerBody>
              <div
                ref={listRef}
                className="gap-0"
                style={{ width: "100%", maxHeight: "calc(80vh - 100px)", overflowY: "auto" }}
              >
                {sortedDanmaku.map((danmaku, index) => (
                  <div
                    key={danmaku.dmid || index}
                    id={`danmaku-item-${index}`}
                    className={`py-2 px-3 border-b border-default-100 transition-colors duration-300 cursor-pointer ${
                      index === currentIndex
                        ? "bg-blue-50/80 dark:bg-blue-900/30"
                        : "hover:bg-default-50 dark:hover:bg-default-800/50"
                    }`}
                    onClick={() => {
                      lastScrollIndexRef.current = index;
                      smoothScrollTo(index);
                    }}
                  >
                    <div className="flex flex-col gap-0.5">
                      <span className={`font-medium leading-snug ${getFontSize(danmaku.fontSize)}`} style={{ color: getColorStyle(danmaku.color) }}>
                        {danmaku.content}
                      </span>
                      <div className="flex items-center gap-3 text-xs text-default-400">
                        <span>{formatTime(danmaku.time)}</span>
                        <span>{getTypeName(danmaku.type)}</span>
                        <span>{formatDate(danmaku.sendTime)}</span>
                      </div>
                    </div>
                  </div>
                ))}
                {sortedDanmaku.length === 0 && (
                  <div className="text-center text-default-400 py-8">暂无弹幕</div>
                )}
              </div>
            </DrawerBody>
          </>
        )}
      </DrawerContent>
    </Drawer>
  );
};

export default DanmakuList;
