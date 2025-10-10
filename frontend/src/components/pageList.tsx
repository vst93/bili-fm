import type { FC } from "react";
import type { service as blSer } from "../../wailsjs/go/models";

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

import {
  FocusOne,
} from "@icon-park/react";


import { convertToDuration, graftingImage } from "@/utils/string";

interface PageListProps {
  pageNum?: number;
  onSlideClick?: () => void;
  videoInfo?: blSer.VideoInfo;
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

  const handleSearchPart = () => { 
    var keyword = (document.querySelector(".part-search-input") as HTMLInputElement).value;
    if (keyword) {
      // 循环DrawerBody，比对，找到 part-title 包含 keyword 的元素，跳转到对应的位置，连续回车支持自动下一个
      const partElements = document.querySelectorAll(".drawer-body .part-title");
      // 获取 drawer-body 已经滚动的高度
      const drawerBody = document.querySelector(".drawer-body") as HTMLElement;
      const drawerBodyScrollTop = drawerBody.scrollTop;
      const drawerBodyScrollBottom = drawerBody.scrollHeight - drawerBody.scrollTop - drawerBody.offsetHeight;
      // console.log(drawerBodyScrollTop, drawerBodyScrollBottom, drawerBody.scrollHeight);
      let firstIndex = -1;
      for (let i = 0; i < partElements.length; i++) {
        const partElement = partElements[i] as HTMLElement;
        if (partElement.textContent?.includes(keyword)) {
          // console.log("找到", partElement.textContent);
          if (firstIndex === -1) { 
            firstIndex = i;
          }
          // console.log(partElement.parentElement?.parentElement?.offsetTop, drawerBodyScrollTop);
          if (((partElement.parentElement?.parentElement?.offsetTop || 0) - drawerBodyScrollTop > 50) || drawerBodyScrollBottom < 50) {
            // 找到第一个符合条件的元素，滚动到对应的位置
            partElement.parentElement?.parentElement?.scrollIntoView({ behavior: "smooth" });
            return;
          }
        }
      }
      // 兜底逻辑，如果没有找到，滚动到第一个
      if (firstIndex >= 0) { 
        const partElement = partElements[firstIndex].parentElement?.parentElement as HTMLElement;
        partElement.scrollIntoView({ behavior: "smooth" });
      }
    }
  }

  // 定位到当前播放的位置
  const handlePositionPart = () => {
    // 循环DrawerBody，比对part，找到 class 为 currentPart 的元素，页面滚动到对应的位置
    const currentPartElement = document.querySelector(
      `.drawer-body .border-2.border-primary.cursor-pointer`,
    ) as HTMLElement;
    if (currentPartElement) {
      currentPartElement.scrollIntoView({ behavior: "smooth" });
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
            <DrawerHeader className="flex gap-3 py-2">
              选集({pageNum}) 
              <FocusOne
                theme="outline"
                size="20"
                fill="#333"
                className="hover:bg-blue-300 w-5 h-5 rounded-full mt-1 cursor-pointer"
                onClick={handlePositionPart}
                title="定位到当前播放的位置"
              />
              <input
                type="text"
                placeholder="搜索"
                className="w-40 part-search-input focus:outline-none text-base font-light"
                style={{ border: "solid 1px #ccc", borderRadius: "15px", padding: "0 10px", margin: "0 0 0 6px" }}
                onKeyUp={(e) => {
                  if (e.key === "Enter") {
                    // 跳转到搜到的位置
                    handleSearchPart()
                  }
                }}
              />

            </DrawerHeader>
            <DrawerBody className="drawer-body">
              <div
                className="gap-2 grid grid-cols-2 sm:grid-cols-3 "
                style={{ width: "100%" }}
              >
                {videoInfo?.pages?.map((page, index) => (
                  <Card
                    key={index}
                    isPressable
                    className={
                      currentPart === page.part ? "border-2 border-primary cursor-pointer" : ""
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
                        className="line-clamp-1 text-left w-full max-h-12 overflow-hidden part-title"
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
