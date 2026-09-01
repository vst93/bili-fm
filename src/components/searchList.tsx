import type { FC, Key } from "react";
import type { SearchResult } from "@/types/bilibili";

import { useMemo, useState } from "react";
import { useDisclosure } from "@heroui/react";
import {
  Drawer,
  DrawerContent,
  DrawerBody,
  DrawerHeader,
  Card,
  CardBody,
  CardFooter,
  Tab,
  Tabs,
} from "@heroui/react";

import { usePreloadImages } from "../hooks/usePreloadImages";

import RetryImg from "./retryImg";

import { graftingImage } from "@/utils/string";

interface SearchListProps {
  onSlideClick?: () => void;
  searchResults?: SearchResult[];
  onVideoSelect?: (bvid: string) => void;
  onSortChange?: (order: string) => void;
}

const SearchList: FC<SearchListProps> = ({
  onSlideClick,
  searchResults = [],
  onVideoSelect,
  onSortChange,
}) => {
  const { isOpen, onOpenChange } = useDisclosure({ isOpen: true });
  const [sortOrder, setSortOrder] = useState("totalrank");

  // 预加载搜索结果封面图
  const coverUrls = useMemo(
    () => searchResults?.map((v) => graftingImage(v.picture_url)) ?? [],
    [searchResults],
  );

  usePreloadImages(coverUrls);

  const handleOpenChange = (open: boolean) => {
    if (!open) {
      onSlideClick?.();
    }
    onOpenChange();
  };

  const handleSortChange = (key: Key) => {
    const nextOrder = key.toString();
    setSortOrder(nextOrder);
    onSortChange?.(nextOrder === "totalrank" ? "" : nextOrder);
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
              <span className="text-sm">搜索</span>
              <Tabs
                aria-label="搜索结果排序"
                selectedKey={sortOrder}
                variant="light"
                onSelectionChange={handleSortChange}
              >
                <Tab key="totalrank" title="综合" />
                <Tab key="click" title="最多播放" />
                <Tab key="update" title="最新发布" />
              </Tabs>
            </DrawerHeader>
            <DrawerBody>
              <div
                className="gap-2 grid grid-cols-2 sm:grid-cols-3"
                style={{ width: "100%" }}
              >
                {searchResults.map((video) => (
                  <Card
                    key={video.url}
                    isPressable
                    shadow="sm"
                    className="c-list-card"
                    onPress={() => onVideoSelect?.(video.url)}
                  >
                    <CardBody className="overflow-visible p-0 img-container">
                      <RetryImg
                        alt={video.title}
                        className="c-cover"
                        fallbackSrc="/cover.png"
                        loading="lazy"
                        radius="sm"
                        shadow="sm"
                        src={graftingImage(video.picture_url)}
                        width="100%"
                      />
                    </CardBody>
                    <CardFooter className="text-small flex-col items-start px-2 py-1">
                      <b
                        className="line-clamp-1 text-left w-full max-h-12 overflow-hidden"
                        title={video.title}
                      >
                        {video.title}
                      </b>
                      <p className="text-default-500 text-left w-full text-xs mt-1 line-clamp-1 max-h-10">
                        {video.author} | {video.date} | {video.views}
                      </p>
                    </CardFooter>
                  </Card>
                ))}
              </div>
            </DrawerBody>
          </>
        )}
      </DrawerContent>
    </Drawer>
  );
};

export default SearchList;
