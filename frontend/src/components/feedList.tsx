import type { FC } from "react";
import { useMemo } from "react";
import type { service as blSer } from "../../wailsjs/go/models";
import { Refresh } from "@icon-park/react";

import RetryImg from "./retryImg";
import { usePreloadImages } from "../hooks/usePreloadImages";

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
} from "@heroui/react";

import { graftingImage, subStr } from "@/utils/string";

interface FeedListProps {
  feedList?: blSer.FeedList;
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
    const bottom = e.currentTarget.scrollHeight - e.currentTarget.scrollTop === e.currentTarget.clientHeight;
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
              动态列表
              <Button
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
                {feedList?.items?.map((item: any, index) => {
                  const info = item.modules.module_dynamic.major.archive;
                  const publishTime = item.modules.module_author.pub_time;
                  const userName = item.modules.module_author.name;
                  const isCharge = item?.modules?.module_dynamic?.major?.archive?.badge?.text === '充电专属';

                  return (
                    <Card
                      key={index}
                      isPressable
                      shadow="sm"
                      onPress={() => onVideoSelect?.(info.bvid)}
                    >
                      <CardBody className="overflow-visible p-0 img-container">
                        <RetryImg
                          alt={info.title || "视频封面"}
                          className="c-cover"
                          fallbackSrc="/cover.png"
                          loading="lazy"
                          radius="sm"
                          shadow="sm"
                          src={graftingImage(info.cover)}
                          width="100%"
                        />
                      </CardBody>
                      <CardFooter className="text-small flex-col items-start px-2 py-1">
                        <b
                          className="line-clamp-1 text-left w-full max-h-12 overflow-hidden"
                          title={info.title}
                        >
                          {isCharge ? <span className="bg-red-400 px-1 py-0.5 rounded-lg text-white mr-1">充电专属</span> : '' }{info.title}
                        </b>
                        <p className="text-default-500 text-left w-full text-xs mt-1 line-clamp-1 max-h-10">
                          {subStr(userName, 7)} | {publishTime} | {item?.modules?.module_dynamic?.major?.archive?.duration_text || '未知时长'} | {item?.modules?.module_dynamic?.major?.archive?.stat?.play || '未知'}
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

export default FeedList; 