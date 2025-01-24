import type { FC } from "react";
import type { main } from "../../wailsjs/go/models";

import { useDisclosure } from "@heroui/react";
import {
  Drawer,
  DrawerContent,
  DrawerBody,
  DrawerHeader,
  Card,
  CardBody,
  CardFooter,
  Image,
} from "@heroui/react";

import { convertToDuration, graftingImage } from "@/utils/string";

interface PageListProps {
  pageNum?: number;
  onSlideClick?: () => void;
  videoInfo?: main.VideoInfo;
  onVideoSelect?: (
    cid: number,
    aid: number,
    part: string,
    index: number,
    first_frame: string,
  ) => void;
  currentBvid?: string;
  currentPart?: string;
}

const PageList: FC<PageListProps> = ({
  pageNum,
  onSlideClick,
  videoInfo,
  onVideoSelect,
  currentPart,
}) => {
  const { isOpen, onOpenChange } = useDisclosure({ isOpen: true });

  const handleOpenChange = (open: boolean) => {
    if (!open) {
      onSlideClick?.();
    }
    onOpenChange();
  };

  const handleVideoSelect = (
    cid: number,
    aid: number,
    part: string,
    index: number,
    first_frame: string,
  ) => {
    onVideoSelect?.(cid, aid, part, index, first_frame);
    handleOpenChange(false);
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
                    className={
                      currentPart === page.part ? "border-2 border-primary" : ""
                    }
                    shadow="sm"
                    onPress={() =>
                      handleVideoSelect(
                        page.cid,
                        videoInfo.aid,
                        page.part,
                        index,
                        page.first_frame,
                      )
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
