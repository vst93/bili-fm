import type { FC } from "react";

import { Refresh, Left, Right } from "@icon-park/react";
import ListCard from "./listCard";
import ListSkeleton from "./listSkeleton";

import { useDisclosure } from "@heroui/react";
import {
  Button,
  Drawer,
  DrawerContent,
  DrawerBody,
  DrawerHeader,
  Tabs,
  Tab,
} from "@heroui/react";
import { useRef, useMemo } from "react";

import { usePreloadImages } from "../hooks/usePreloadImages";

import { graftingImage, formatRelativeTime, convertToDuration, viewsMetaField } from "@/utils/string";

interface CollectListProps {
  onSlideClick?: () => void;
  collectList?: any;
  collectGroups?: any[];
  onVideoSelect?: (bvid: string) => void;
  onRefresh?: () => void;
  onLoadMore?: () => void;
  onGroupSelect?: (id: number) => void;
  currentGroupId?: number;
}

const CollectList: FC<CollectListProps> = ({
  onSlideClick,
  collectList,
  collectGroups,
  onVideoSelect,
  onRefresh,
  onLoadMore,
  onGroupSelect,
  currentGroupId,
}) => {
  const { isOpen, onOpenChange } = useDisclosure({ isOpen: true });

  // 预加载收藏封面图
  const coverUrls = useMemo(
    () => (Array.isArray(collectList) ? collectList.map((item: any) => graftingImage(item.cover)) : []),
    [collectList],
  );
  usePreloadImages(coverUrls);
  const tabsRef = useRef<HTMLDivElement>(null);

  const handleOpenChange = (open: boolean) => {
    if (!open) {
      onSlideClick?.();
    }
    onOpenChange();
  };

  const handleScroll = (e: React.UIEvent<HTMLDivElement>) => {
    const bottom =
      e.currentTarget.scrollHeight - e.currentTarget.scrollTop -
        e.currentTarget.clientHeight <= 80;

    if (bottom) {
      onLoadMore?.();
    }
  };

  const handleRefresh = () => {
    const drawerBody = document.querySelector(".collect-drawer-body");

    if (drawerBody) {
      drawerBody.scrollTop = 0;
    }
    onRefresh?.();
  };

  const scrollTabs = (direction: "left" | "right") => {
    if (tabsRef.current) {
      const scrollAmount = 200;
      const newScrollLeft =
        direction === "left"
          ? tabsRef.current.scrollLeft - scrollAmount
          : tabsRef.current.scrollLeft + scrollAmount;

      tabsRef.current.scrollTo({
        left: newScrollLeft,
        behavior: "smooth",
      });
    }
  };

  return (
    <Drawer
      classNames={{
        base: "h-[92vh] max-h-[calc(100vh-54px)]",
      }}
      isOpen={isOpen}
      placement="bottom"
      onOpenChange={handleOpenChange}
    >
      <DrawerContent>
        {() => (
          <>
            <DrawerHeader className="flex items-center gap-2 py-2">
              <span className="text-sm flex-none">收藏列表</span>
              <button
                aria-label="向左滚动收藏夹"
                className="collect-scroll-btn"
                title="向左滚动"
                onClick={() => scrollTabs("left")}
              >
                <Left size="16" theme="outline" />
              </button>
              <div ref={tabsRef} className="collect-tabs-scroll">
                <Tabs
                  aria-label="收藏夹切换"
                  selectedKey={currentGroupId?.toString()}
                  variant="light"
                  onSelectionChange={(key) => onGroupSelect?.(Number(key))}
                >
                  {collectGroups?.map((group) => (
                    <Tab
                      key={group.id}
                      title={`${group.title} (${group.media_count})`}
                    />
                  ))}
                </Tabs>
              </div>
              <button
                aria-label="向右滚动收藏夹"
                className="collect-scroll-btn"
                title="向右滚动"
                onClick={() => scrollTabs("right")}
              >
                <Right size="16" theme="outline" />
              </button>
              <Button
                aria-label="刷新收藏列表"
                isIconOnly
                size="sm"
                variant="light"
                onClick={handleRefresh}
              >
                <Refresh theme="outline" size="20" fill="#333" />
              </Button>
            </DrawerHeader>
            <DrawerBody className="collect-drawer-body" onScroll={handleScroll}>
              {collectList == null ? (
                <ListSkeleton />
              ) : collectList.length === 0 ? (
                <div className="history-empty-tip" role="status">
                  暂无收藏内容
                </div>
              ) : (
                <div
                  className="gap-2 grid grid-cols-2 sm:grid-cols-3"
                  style={{ width: "100%" }}
                >
                  {collectList.map((item: any) => {
                    return (
                      <ListCard
                        key={item.id || item.bvid}
                        cover={item.cover}
                        coverAlt={item.title}
                        duration={
                          item?.duration != null && item.duration > 0
                            ? convertToDuration(item.duration)
                            : null
                        }
                        fields={[
                          { kind: "author", value: item.upper?.name || item.author },
                          ...viewsMetaField(item?.cnt_info?.play, item?.cnt_info?.danmaku),
                          { kind: "pubdate", value: item.ctime ? formatRelativeTime(item.ctime) : null },
                        ]}
                        onPress={() => onVideoSelect?.(item.bvid)}
                        title={item.title}
                      />
                    );
                  })}
                </div>
              )}
            </DrawerBody>
          </>
        )}
      </DrawerContent>
    </Drawer>
  );
};

export default CollectList;
