import type { FC } from "react";

import { Refresh, Left, Right } from "@icon-park/react";
import RetryImg from "./retryImg";

import { useDisclosure } from "@heroui/react";
import {
  Button,
  Drawer,
  DrawerContent,
  DrawerBody,
  DrawerHeader,
  Card,
  CardBody,
  CardFooter,
  Tabs,
  Tab,
} from "@heroui/react";
import { useRef, useMemo } from "react";

import { usePreloadImages } from "../hooks/usePreloadImages";

import { graftingImage } from "@/utils/string";

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
      e.currentTarget.scrollHeight - e.currentTarget.scrollTop ===
      e.currentTarget.clientHeight;

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

  const formatTimestamp = (timestamp: number) => {
    const date = new Date(timestamp * 1000);
    return `${date.getFullYear()}-${String(date.getMonth() + 1).padStart(2, "0")}-${String(date.getDate()).padStart(2, "0")}`;
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
            <DrawerHeader className="collect-drawer-header py-2">
              <div className="collect-drawer-title-row">
                <span>收藏列表</span>
                <Button
                  isIconOnly
                  size="sm"
                  variant="light"
                  onClick={handleRefresh}
                >
                  <Refresh theme="outline" size="20" fill="#333" />
                </Button>
              </div>
              <div className="collect-tabs-row">
                <button
                  className="collect-scroll-btn"
                  title="向左滚动"
                  onClick={() => scrollTabs("left")}
                >
                  <Left size="16" theme="outline" />
                </button>
                <div ref={tabsRef} className="collect-tabs-scroll">
                  <Tabs
                    classNames={{
                      tabList: "gap-2 w-full relative rounded-none p-0 bg-transparent",
                      cursor: "collect-tab-cursor",
                      tab: "collect-tab",
                      tabContent: "group-data-[selected=true]:text-primary",
                    }}
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
                  className="collect-scroll-btn"
                  title="向右滚动"
                  onClick={() => scrollTabs("right")}
                >
                  <Right size="16" theme="outline" />
                </button>
              </div>
            </DrawerHeader>
            <DrawerBody className="collect-drawer-body" onScroll={handleScroll}>
              <div
                className="gap-2 grid grid-cols-2 sm:grid-cols-3"
                style={{ width: "100%" }}
              >
                {Array.isArray(collectList) &&
                  collectList.map((item: any, index: number) => {
                    return (
                      <Card
                        key={index}
                        isPressable
                        shadow="sm"
                        onPress={() => onVideoSelect?.(item.bvid)}
                      >
                        <CardBody className="overflow-visible p-0 img-container">
                          <RetryImg
                            alt={item.title}
                            className="c-cover"
                            fallbackSrc="/cover.png"
                            loading="lazy"
                            radius="sm"
                            shadow="sm"
                            src={graftingImage(item.cover)}
                            width="100%"
                          />
                        </CardBody>
                        <CardFooter className="text-small flex-col items-start px-2 py-1">
                          <b
                            className="line-clamp-1 text-left w-full max-h-12 overflow-hidden"
                            title={item.title}
                          >
                            {item.title}
                          </b>
                          <p className="text-default-500 text-left w-full text-xs mt-1 line-clamp-1 max-h-10">
                            {item.upper?.name || item.author} |{" "}
                            {formatTimestamp(item.ctime)}
                          </p>
                        </CardFooter>
                      </Card>
                    );
                  })}
              </div>
            </DrawerBody>
          </>
        )}
      </DrawerContent>
    </Drawer>
  );
};

export default CollectList;
