import type { FC } from "react";
import { useMemo } from "react";
import type { FeedList } from "@/types/bilibili";
import { Refresh } from "@icon-park/react";
import ListCard from "./listCard";
import { usePreloadImages } from "../hooks/usePreloadImages";

import { useDisclosure } from "@heroui/react";
import {
  Button,
  Drawer,
  DrawerContent,
  DrawerBody,
  DrawerHeader,
} from "@heroui/react";

import { graftingImage, viewsMetaField } from "@/utils/string";

interface FeedListProps {
  feedList?: FeedList;
  onSlideClick?: () => void;
  onVideoSelect?: (bvid: string) => void;
  onRefresh?: () => void;
  onLoadMore?: (offset: string) => void;
}

const FeedList: FC<FeedListProps> = ({
  feedList,
  onSlideClick,
  onVideoSelect,
  onRefresh,
  onLoadMore,
}) => {
  const { isOpen, onOpenChange } = useDisclosure({ isOpen: true });

  // 数据到达时立即预加载封面图，用户打开时已缓存
  const coverUrls = useMemo(
    () => feedList?.items?.map((item: any) => graftingImage(item.modules?.module_dynamic?.major?.archive?.cover)) ?? [],
    [feedList],
  );
  usePreloadImages(coverUrls);

  const handleOpenChange = (open: boolean) => {
    if (!open) {
      onSlideClick?.();
    }
    onOpenChange();
  };

  const handleScroll = (e: React.UIEvent<HTMLDivElement>) => {
    const bottom = e.currentTarget.scrollHeight - e.currentTarget.scrollTop - e.currentTarget.clientHeight <= 80;
    if (bottom && feedList?.offset) {
      onLoadMore?.(feedList.offset);
    }
  };

  const handleRefresh = () => {
    const drawerBody = document.querySelector('.feed-drawer-body');
    if (drawerBody) {
      drawerBody.scrollTop = 0;
    }
    onRefresh?.();
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
              <span className="text-sm flex-none">动态列表</span>
              <Button
                aria-label="刷新动态列表"
                isIconOnly
                size="sm"
                variant="light"
                onClick={handleRefresh}
              >
                <Refresh theme="outline" size="20" fill="#333" />
              </Button>
            </DrawerHeader>
            <DrawerBody className="feed-drawer-body" onScroll={handleScroll}>
              <div
                className="gap-2 grid grid-cols-2 sm:grid-cols-3"
                style={{ width: "100%" }}
              >
                {feedList?.items?.map((item: any) => {
                  const info = item.modules.module_dynamic.major.archive;
                  const publishTime = item.modules.module_author.pub_time;
                  const userName = item.modules.module_author.name;
                  const isCharge = item?.modules?.module_dynamic?.major?.archive?.badge?.text === '充电专属';

                  return (
                    <ListCard
                      key={info.bvid}
                      cover={info.cover}
                      coverAlt={info.title || "视频封面"}
                      duration={info?.duration_text}
                      fields={[
                        { kind: "author", value: userName },
                        ...viewsMetaField(info?.stat?.play, info?.stat?.danmaku),
                        { kind: "pubdate", value: publishTime },
                      ]}
                      onPress={() => onVideoSelect?.(info.bvid)}
                      title={info.title}
                      titlePrefix={
                        isCharge ? (
                          <span className="bg-red-400 px-1 py-0.5 rounded-lg text-white mr-1">
                            充电专属
                          </span>
                        ) : undefined
                      }
                    />
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

export default FeedList;
