import type { FC } from "react";

import { Refresh, Left, Right } from "@icon-park/react";
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
import { useRef } from "react";

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
        base: "h-[80vh]",
      }}
      isOpen={isOpen}
      placement="bottom"
      onOpenChange={handleOpenChange}
    >
      <DrawerContent>
        {() => (
          <>
            <DrawerHeader className="flex flex-col gap-2 py-2">
              <div className="flex items-center gap-2">
                收藏列表
                <Button
                  isIconOnly
                  size="sm"
                  variant="light"
                  onClick={handleRefresh}
                >
                  <Refresh theme="outline" size="20" fill="#333" />
                </Button>
              </div>
              <div className="flex items-center gap-0">
                <Button
                  isIconOnly
                  size="sm"
                  variant="light"
                  onClick={() => scrollTabs("left")}
                >
                  <Left fill="#333" size="20" theme="outline" />
                </Button>
                <div
                  ref={tabsRef}
                  className="flex-1 overflow-x-auto scrollbar-hide"
                >
                  <div className="min-w-max">
                    <Tabs
                      classNames={{
                        tabList: "gap-4 w-full relative rounded-none p-0",
                        cursor: "bg-default-100",
                        tab: "h-10 px-4",
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
                </div>
                <Button
                  isIconOnly
                  size="sm"
                  variant="light"
                  onClick={() => scrollTabs("right")}
                >
                  <Right fill="#333" size="20" theme="outline" />
                </Button>
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
                          <Image
                            alt={item.title}
                            className="c-cover"
                            crossOrigin="anonymous"
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
