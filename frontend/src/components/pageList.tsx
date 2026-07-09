import type { FC } from "react";
import { useMemo } from "react";
import type { service as blSer } from "../../wailsjs/go/models";

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

import {
  FocusOne,
  AddOne,
  Check,
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
  onAddToPlaylist?: (page: blSer.Page) => void;
  onAddAllToPlaylist?: () => void;
  playlistCids?: Set<number>;
  currentBvid?: string;
  currentPart?: string;
}

const PageList: FC<PageListProps> = ({
  pageNum,
  onSlideClick,
  videoInfo,
  onVideoSelect,
  onAddToPlaylist,
  onAddAllToPlaylist,
  playlistCids,
  currentPart,
}) => {
  const { isOpen, onOpenChange } = useDisclosure({ isOpen: true });

  // 预加载选集封面图
  const coverUrls = useMemo(
    () => videoInfo?.pages?.map((page) => graftingImage(page.first_frame || videoInfo.pic)) ?? [],
    [videoInfo],
  );
  usePreloadImages(coverUrls);

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
        base: "h-[92vh] max-h-[calc(100vh-54px)]",
      }}
      isOpen={isOpen}
      placement="bottom"
      onOpenChange={handleOpenChange}
    >
      <DrawerContent>
        {() => (
          <>
            <DrawerHeader className="flex items-center gap-3 py-2 pr-12">
              <span className="up-drawer-title">选集({pageNum})</span>
              <button
                type="button"
                className="liquid-glass-icon-btn"
                title="定位到当前播放的位置"
                onClick={handlePositionPart}
              >
                <FocusOne theme="outline" size="18" />
              </button>
              <input
                type="text"
                placeholder="搜索"
                className="w-40 part-search-input focus:outline-none text-sm font-light"
                style={{ padding: "0 10px" }}
                onKeyUp={(e) => {
                  if (e.key === "Enter") {
                    // 跳转到搜到的位置
                    handleSearchPart()
                  }
                }}
              />
              <Button
                size="sm"
                title="将全部选集添加到播放列表"
                variant="flat"
                onClick={() => onAddAllToPlaylist?.()}
              >
                <AddOne fill="#666" size="16" theme="outline" />
                <span className="ml-1 text-xs">全部添加</span>
              </Button>

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
                    <CardBody className="overflow-visible p-0 img-container relative">
                      <RetryImg
                        alt={page.part || videoInfo.title}
                        className="c-cover"
                        fallbackSrc="/cover.png"
                        loading="lazy"
                        radius="sm"
                        shadow="sm"
                        src={graftingImage(page.first_frame || videoInfo.pic)}
                        width="100%"
                      />
                      <Button
                        isIconOnly
                        className="absolute top-1 right-1 z-10 min-w-6 w-6 h-6 rounded-full bg-black/30 backdrop-blur-sm border-0"
                        size="sm"
                        title={
                          playlistCids?.has(page.cid)
                            ? "已在播放列表中"
                            : "添加到播放列表"
                        }
                        variant="flat"
                        onPress={() => onAddToPlaylist?.(page)}
                      >
                        {playlistCids?.has(page.cid) ? (
                          <Check fill="#4ade80" size="14" theme="outline" />
                        ) : (
                          <AddOne fill="#fff" size="14" theme="outline" />
                        )}
                      </Button>
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
