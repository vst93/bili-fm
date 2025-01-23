import type { FC } from "react";
import { Refresh } from "@icon-park/react";

import { useDisclosure } from "@nextui-org/react";
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
} from "@nextui-org/react";

import { graftingImage } from "@/utils/string";

interface RecommendListProps {
  onSlideClick?: () => void;
  recommendList?: any;
  onVideoSelect?: (bvid: string) => void;
  onRefresh?: () => void;
  onLoadMore?: () => void;
}

const RecommendList: FC<RecommendListProps> = ({
  onSlideClick,
  recommendList,
  onVideoSelect,
  onRefresh,
  onLoadMore,
}) => {
  const { isOpen, onOpenChange } = useDisclosure({ isOpen: true });

  const handleOpenChange = (open: boolean) => {
    if (!open) {
      onSlideClick?.();
    }
    onOpenChange();
  };

  const handleScroll = (e: React.UIEvent<HTMLDivElement>) => {
    const bottom = e.currentTarget.scrollHeight - e.currentTarget.scrollTop === e.currentTarget.clientHeight;
    if (bottom) {
      onLoadMore?.();
    }
  };

  const handleRefresh = () => {
    const drawerBody = document.querySelector('.recommend-drawer-body');
    if (drawerBody) {
      drawerBody.scrollTop = 0;
    }
    onRefresh?.();
  };

  const formatTimestamp = (timestamp: number) => {
    const date = new Date(timestamp * 1000);
    return `${date.getFullYear()}-${String(date.getMonth() + 1).padStart(2, '0')}-${String(date.getDate()).padStart(2, '0')}`;
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
              推荐视频
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
              <div
                className="gap-2 grid grid-cols-2 sm:grid-cols-3"
                style={{ width: "100%" }}
              >
                {recommendList?.items?.map((item: any, index: number) => {
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
                          {item.owner?.name || item.author} | {formatTimestamp(item.pubdate)}
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

export default RecommendList; 