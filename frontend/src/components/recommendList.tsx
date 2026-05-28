import type { FC } from "react";
import { Refresh } from "@icon-park/react";

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
  Image,
  Tabs,
  Tab,
} from "@heroui/react";
import React, { useState, useEffect } from "react";

import { convertToDuration, graftingImage, formatNumber, subStr } from "@/utils/string";

const TAB_STORAGE_KEY = "bili-fm-recommend-tab";

interface RecommendListProps {
  onSlideClick?: () => void;
  recommendList?: any;
  hotList?: any;
  onVideoSelect?: (bvid: string) => void;
  onRefresh?: (type: string) => void;
  onLoadMore?: (type: string) => void;
}

const RecommendList: FC<RecommendListProps> = ({
  onSlideClick,
  recommendList,
  hotList,
  onVideoSelect,
  onRefresh,
  onLoadMore,
}) => {
  const { isOpen, onOpenChange } = useDisclosure({ isOpen: true });
  const [activeTab, setActiveTab] = useState<string>(() => {
    if (typeof window !== "undefined") {
      const saved = localStorage.getItem(TAB_STORAGE_KEY);
      if (saved === "recommend" || saved === "hot") {
        return saved;
      }
    }
    return "hot";
  });

  // 保存选中的 tab 到 localStorage
  const handleTabChange = (key: string) => {
    const tabKey = key.toString();
    setActiveTab(tabKey);
    localStorage.setItem(TAB_STORAGE_KEY, tabKey);
    const drawerBody = document.querySelector('.recommend-drawer-body');
    if (drawerBody) {
      drawerBody.scrollTop = 0;
    }
  };

  // 当 Drawer 打开且没有数据时，自动加载当前标签页数据
  useEffect(() => {
    if (isOpen) {
      if (activeTab === "hot" && (!hotList?.items || hotList.items.length === 0)) {
        onRefresh?.("hot");
      } else if (activeTab === "recommend" && (!recommendList?.items || recommendList.items.length === 0)) {
        onRefresh?.("recommend");
      }
    }
  }, [isOpen, activeTab]);

  const handleOpenChange = (open: boolean) => {
    if (!open) {
      onSlideClick?.();
    }
    onOpenChange();
  };

  const handleScroll = (e: React.UIEvent<HTMLDivElement>) => {
    const bottom = e.currentTarget.scrollHeight - e.currentTarget.scrollTop === e.currentTarget.clientHeight;
    if (bottom) {
      onLoadMore?.(activeTab);
    }
  };

  const handleRefresh = () => {
    const drawerBody = document.querySelector('.recommend-drawer-body');
    if (drawerBody) {
      drawerBody.scrollTop = 0;
    }
    onRefresh?.(activeTab);
  };

  const currentList = activeTab === "recommend" ? recommendList : hotList;

  const formatTimestamp = (timestamp: number) => {
    const date = new Date(timestamp * 1000);
    if (date.getFullYear() < (new Date().getFullYear())) {
      return `${date.getFullYear()}-${String(date.getMonth() + 1).padStart(2, '0')}-${String(date.getDate()).padStart(2, '0')}`;
    } else {
      return `${String(date.getMonth() + 1).padStart(2, '0')}-${String(date.getDate()).padStart(2, '0')}`;
    }
  };

  return (
    <Drawer
      classNames={{
        base: "h-[80vh]",
      }}
      isOpen={isOpen}
      placement="bottom"
      onOpenChange={handleOpenChange}
    >
      <DrawerContent>
        {() => (
          <>
            <DrawerHeader className="flex items-center gap-2 py-2">
              <Tabs
                selectedKey={activeTab}
                onSelectionChange={(key) => handleTabChange(key.toString())}
                variant="light"
                classNames={{
                  tabList: "gap-2",
                  cursor: "bg-default-100",
                  tab: "h-8 px-4",
                  tabContent: "group-data-[selected=true]:text-primary",
                }}
              >
                <Tab key="hot" title="热门" />
                <Tab key="recommend" title="推荐" />
              </Tabs>
              <Button
                isIconOnly
                size="sm"
                variant="light"
                onClick={handleRefresh}
              >
                <Refresh theme="outline" size="20" fill="#333" />
              </Button>
            </DrawerHeader>
            <DrawerBody className="recommend-drawer-body" onScroll={handleScroll}>
              {(!currentList?.items || currentList.items.length === 0) ? (
                <div className="flex items-center justify-center h-40 text-gray-400">
                  {activeTab === "recommend" ? "暂无推荐内容" : "暂无热门内容"}
                </div>
              ) : (
                <div
                  className="gap-2 grid grid-cols-2 sm:grid-cols-3"
                  style={{ width: "100%" }}
                >
                  {currentList?.items?.map((item: any, index: number) => {
                    const coverUrl = item.pic || item.cover;
                    return (
                      <Card
                        key={index}
                        isPressable
                        shadow="sm"
                        onPress={() => onVideoSelect?.(item.bvid)}
                      >
                        <CardBody className="overflow-visible p-0 img-container">
                          <Image
                            alt={item.title}
                            className="c-cover"
                            crossOrigin="anonymous"
                            fallbackSrc="/cover.png"
                            loading="lazy"
                            radius="sm"
                            shadow="sm"
                            src={graftingImage(coverUrl)}
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
                            {subStr(item.owner?.name || item.author,7)} | {formatTimestamp(item.pubdate)} | {convertToDuration(item.duration)} | {formatNumber(item?.stat?.view)}
                          </p>
                        </CardFooter>
                      </Card>
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

export default RecommendList; 