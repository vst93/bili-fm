import type { FC } from "react";
import { useMemo } from "react";
import type { Page, VideoInfo } from "@/types/bilibili";

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
  Search,
} from "@icon-park/react";
import CardMeta from "./cardMeta";


import { convertToDuration, graftingImage, formatViewCount } from "@/utils/string";

interface PageListProps {
  pageNum?: number;
  onSlideClick?: () => void;
  videoInfo?: VideoInfo;
  onVideoSelect?: (
    cid: number,
    aid: number,
    part: string,
    index: number,
    first_frame: string,
  ) => void;
  onAddToPlaylist?: (page: Page) => void;
  onAddAllToPlaylist?: () => void;
  playlistCids?: Set<number>;
  currentBvid?: string;
  currentPart?: string;
  /** 正在播放的视频 bvid / 分 P cid：与当前浏览的视频一致时才标记播放项 */
  playingBvid?: string;
  playingCid?: number;
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
  playingBvid,
  playingCid,
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

  // 定位到当前播放的位置：按稳定的语义类 part-playing 查找（不要按样式类定位，
  // 样式一改这里就会静默失效）
  const handlePositionPart = () => {
    const currentPartElement = document.querySelector<HTMLElement>(
      ".drawer-body .part-playing",
    );
    if (!currentPartElement) return;

    // .c-list-card 的 content-visibility:auto 会让视口外卡片按 180px 占位
    // 高度参与布局（实际高度更小）。若边滚边实体化，目标位置会不断漂移，
    // WebKit 上表现为第一次点不准、第二次才到。先全部实体化（卡片高度
    // 固定，这是一次性的），布局就完全稳定了。
    document
      .querySelectorAll<HTMLElement>(".drawer-body .c-list-card")
      .forEach((card) => {
        card.style.contentVisibility = "visible";
      });

    // 不用 scrollIntoView：WebKit 在布局刚变化后由它内部增量重算目的地
    // 不可靠。这里读 rect（强制同步布局，拿到实体化后的真实位置），
    // 自己算出目的地一次滚到位。
    const drawerBody = currentPartElement.closest<HTMLElement>(".drawer-body");
    if (!drawerBody) return;
    const top =
      currentPartElement.getBoundingClientRect().top -
      drawerBody.getBoundingClientRect().top +
      drawerBody.scrollTop -
      8;
    drawerBody.scrollTo({ top: Math.max(0, top), behavior: "smooth" });
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
              <span className="text-sm">选集({pageNum})</span>
              <Button
                isIconOnly
                size="sm"
                title="定位到当前播放的位置"
                variant="light"
                onClick={handlePositionPart}
              >
                <FocusOne theme="outline" size="18" />
              </Button>
              <div className="part-search">
                <Search className="part-search-icon" theme="outline" size="15" />
                <input
                  type="text"
                  placeholder="搜索选集，回车跳转"
                  className="part-search-input"
                  onKeyUp={(e) => {
                    if (e.key === "Enter") {
                      handleSearchPart()
                    }
                  }}
                />
              </div>
              <Button
                className="flex-none"
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
                {videoInfo?.pages?.map((page, index) => {
                  // 播放项标识：只在「正在播放的视频 = 当前浏览的视频」时标记，
                  // 优先按 cid 精确匹配（从播放列表/合集播入时 part 文本可能
                  // 与选集名不一致，按名字匹配会漏标）；cid 缺失时回退到 part 名。
                  const isPlayingPage =
                    !!playingBvid &&
                    playingBvid === videoInfo.bvid &&
                    (playingCid != null
                      ? playingCid === page.cid
                      : currentPart === page.part);
                  return (
                  <Card
                    key={page.cid}
                    isPressable
                    className={`c-list-card ${
                      isPlayingPage ? "part-playing border-2 border-[#0284c7] cursor-pointer" : ""
                    }`}
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
                      {page.duration > 0 ? (
                        <span className="c-cover-duration">{convertToDuration(page.duration)}</span>
                      ) : null}
                      {isPlayingPage && (
                        <span className="part-playing-badge">
                          <span className="part-playing-eq" aria-hidden="true">
                            <i />
                            <i />
                            <i />
                          </span>
                          正在播放
                        </span>
                      )}
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
                      <div className="text-default-500 text-left w-full text-xs mt-1 line-clamp-1 max-h-10">
                        <CardMeta fields={[
                          { kind: "views", value: videoInfo?.stat?.view != null ? formatViewCount(videoInfo.stat.view) : null },
                          { kind: "duration", value: convertToDuration(page.duration) },
                        ]} />
                      </div>
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

export default PageList;
