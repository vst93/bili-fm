import type { FC } from "react";
import type { main } from "../../wailsjs/go/models";

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

interface SearchListProps {
  onSlideClick?: () => void;
  searchResults?: main.SearchResult[];
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

  const handleOpenChange = (open: boolean) => {
    if (!open) {
      onSlideClick?.();
    }
    onOpenChange();
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
              搜索
              <Button
                size="sm"
                variant="flat"
                onClick={() => onSortChange?.("")}
              >
                综合排序
              </Button>
              <Button
                size="sm"
                variant="flat"
                onClick={() => onSortChange?.("click")}
              >
                最多播放
              </Button>
              <Button
                size="sm"
                variant="flat"
                onClick={() => onSortChange?.("update")}
              >
                最新发布
              </Button>
            </DrawerHeader>
            <DrawerBody>
              <div
                className="gap-2 grid grid-cols-2 sm:grid-cols-3"
                style={{ width: "100%" }}
              >
                {searchResults.map((video, index) => (
                  <Card
                    key={index}
                    isPressable
                    shadow="sm"
                    onPress={() => onVideoSelect?.(video.url)}
                  >
                    <CardBody className="overflow-visible p-0 img-container">
                      <Image
                        alt={video.title}
                        className="c-cover"
                        crossOrigin="anonymous"
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
