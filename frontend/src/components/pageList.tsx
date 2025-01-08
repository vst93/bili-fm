import type { FC } from "react";
import type { main } from "../../wailsjs/go/models";

import { useDisclosure } from "@nextui-org/react";
import {
  Drawer,
  DrawerContent,
  DrawerBody,
  DrawerHeader,
  Card,
  CardBody,
  CardFooter,
  Image,
} from "@nextui-org/react";

import { convertToDuration, graftingImage } from "@/utils/string";

interface PageListProps {
  pageNum?: number;
  onSlideClick?: () => void;
  videoInfo?: main.VideoInfo;
  onVideoSelect?: (cid: number, aid: number, part: string) => void;
  currentBvid?: string;
}

const PageList: FC<PageListProps> = ({
  pageNum,
  onSlideClick,
  videoInfo,
  onVideoSelect,
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
            <DrawerHeader className="flex gap-2">
              选集 <span>{pageNum}</span>
            </DrawerHeader>
            <DrawerBody>
              <div
                className="gap-2 grid grid-cols-2 sm:grid-cols-3"
                style={{ width: "100%" }}
              >
                {videoInfo?.pages?.map((page, index) => (
                  <Card
                    key={index}
                    isPressable
                    shadow="sm"
                    onPress={() =>
                      onVideoSelect?.(page.cid, videoInfo.aid, page.part)
                    }
                  >
                    <CardBody className="overflow-visible p-0 img-container">
                      <Image
                        alt={page.part || videoInfo.title}
                        className="c-cover"
                        crossOrigin="anonymous"
                        fallbackSrc="/cover.png"
                        loading="lazy"
                        radius="sm"
                        shadow="sm"
                        src={graftingImage(page.first_frame || videoInfo.pic)}
                        width="100%"
                      />
                    </CardBody>
                    <CardFooter className="text-small flex-col items-start px-2 py-1">
                      <b
                        className="line-clamp-1 text-left w-full max-h-12 overflow-hidden"
                        title={page.part || videoInfo.title}
                      >
                        {page.part || videoInfo.title}
                      </b>
                      <p className="text-default-500 text-left w-full text-xs mt-1 line-clamp-1 max-h-10">
                        {convertToDuration(page.duration)}
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

export default PageList;
