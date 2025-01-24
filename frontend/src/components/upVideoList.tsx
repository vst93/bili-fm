import type { FC } from "react";
import type { main } from "../../wailsjs/go/models";

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
} from "@heroui/react";

import { graftingImage } from "@/utils/string";

interface UpVideoListProps {
  upVideoList?: main.FeedList;
  onSlideClick?: () => void;
  onVideoSelect?: (bvid: string) => void;
  onRefresh?: () => void;
  onLoadMore?: (offset: string) => void;
  upName?: string;
}

const UpVideoList: FC<UpVideoListProps> = ({
  upVideoList,
  onSlideClick,
  onVideoSelect,
  onRefresh,
  onLoadMore,
  upName = "",
}) => {
  const { isOpen, onOpenChange } = useDisclosure({ isOpen: true });

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

    if (bottom && upVideoList?.offset) {
      onLoadMore?.(upVideoList.offset);
    }
  };

  const handleRefresh = () => {
    const drawerBody = document.querySelector(".up-video-drawer-body");

    if (drawerBody) {
      drawerBody.scrollTop = 0;
    }
    onRefresh?.();
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
            <DrawerHeader className="flex gap-2 py-2">
              「{upName}」的视频
              <Button
                isIconOnly
                size="sm"
                variant="light"
                onClick={handleRefresh}
              >
                <Refresh fill="#333" size="20" theme="outline" />
              </Button>
            </DrawerHeader>
            <DrawerBody
              className="up-video-drawer-body"
              onScroll={handleScroll}
            >
              <div
                className="gap-2 grid grid-cols-2 sm:grid-cols-3"
                style={{ width: "100%" }}
              >
                {upVideoList?.items?.map((item: any, index) => {
                  const info = item.modules.module_dynamic.major.archive;
                  const publishTime = item.modules.module_author.pub_time;

                  return (
                    <Card
                      key={index}
                      isPressable
                      shadow="sm"
                      onPress={() => onVideoSelect?.(info.bvid)}
                    >
                      <CardBody className="overflow-visible p-0 img-container">
                        <Image
                          alt={info.title || "视频封面"}
                          className="c-cover"
                          crossOrigin="anonymous"
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
                          {info.title}
                        </b>
                        <p className="text-default-500 text-left w-full text-xs mt-1 line-clamp-1 max-h-10">
                          {publishTime}
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

export default UpVideoList;
