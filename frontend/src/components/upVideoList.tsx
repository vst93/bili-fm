import type { FC } from "react";
import type { service as blSer } from "../../wailsjs/go/models";

import { Refresh, Add, Close } from "@icon-park/react";
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
  Spinner,
} from "@heroui/react";
import { useState, useEffect } from "react";

import { graftingImage } from "@/utils/string";
import { toast } from "@/utils/toast";

import {
  GetSeriesList,
  IsFollowing,
  Follow,
  Unfollow,
} from "../../wailsjs/go/service/BL";

// interface SeriesItem {
//   id: number;
//   title: string;
//   description: string;
//   cover: string;
//   mid: number;
//   total: number;
// }

// interface SeriesVideoItem {
//   aid: number;
//   bvid: string;
//   title: string;
//   cover: string;
//   author: string;
//   pubdate: number;
// }

interface UpVideoListProps {
  upVideoList?: blSer.FeedList;
  onSlideClick?: () => void;
  onVideoSelect?: (bvid: string) => void;
  onRefresh?: () => void;
  onLoadMore?: (offset: string) => void;
  upName?: string;
  seriesList?: Array<any>;
  onSeriesSelect?: (id: number,title: string,total: number) => void;
  currentSeriesId?: number;
  setSeriesList?: (list: Array<any>) => void;
  currentUpMid?: number;
  setSeriesVideosPage?: (page: number) => void;
}

const UpVideoList: FC<UpVideoListProps> = ({
  upVideoList,
  onSlideClick,
  onVideoSelect,
  onRefresh,
  onLoadMore,
  upName = "",
  seriesList = [],
  onSeriesSelect,
  currentSeriesId,
  setSeriesList,
  currentUpMid = 0,
  setSeriesVideosPage,
}) => {
  const { isOpen, onOpenChange } = useDisclosure({ isOpen: true });
  const [activeTab, setActiveTab] = useState<string>("videos");
  const [isFollowing, setIsFollowing] = useState(false);
  const [isCheckingFollow, setIsCheckingFollow] = useState(false);
  const [isFollowingLoading, setIsFollowingLoading] = useState(false);

  // 检查关注状态
  const checkFollowStatus = async () => {
    if (!currentUpMid || currentUpMid === 0) {
      setIsFollowing(false);
      return;
    }
    setIsCheckingFollow(true);
    try {
      const following = await IsFollowing(currentUpMid);
      setIsFollowing(following);
    } catch (error) {
      console.error("检查关注状态失败:", error);
      setIsFollowing(false);
    } finally {
      setIsCheckingFollow(false);
    }
  };

  // 打开时检查关注状态
  useEffect(() => {
    if (isOpen && currentUpMid > 0) {
      checkFollowStatus();
    }
  }, [isOpen, currentUpMid]);

  const handleAciveTabChange = async (key: string) => {
    setActiveTab(key);
    setSeriesVideosPage?.(1);
    if (key === "series" && currentUpMid) {
      try {
        console.log("获取合集列表中...",currentUpMid);
        const list = await GetSeriesList(currentUpMid);
        if (!list) {
          setSeriesList?.([]);
        } else { 
          setSeriesList?.(list);
        }
      } catch (error) {
        console.error("获取合集列表失败：", error);
      }
    }
  }

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

    if (activeTab === "videos" && bottom && upVideoList?.offset) {
      onLoadMore?.(upVideoList.offset);
    } else if (activeTab === "series" && bottom) {
      try {
        GetSeriesList(currentUpMid).then(list => { 
          if (!list) {
            console.error("获取合集列表失败：用户未登录或登录已过期");
            return;
          }
          console.log("合集列表：", list);
          setSeriesList?.(list);
        });
      } catch (error) {
        console.error("获取合集列表失败：", error);
      }
    }
  };

  const handleRefresh = () => {
    const drawerBody = document.querySelector(".up-video-drawer-body");

    if (drawerBody) {
      drawerBody.scrollTop = 0;
    }
    onRefresh?.();
    // 刷新时重新检查关注状态
    checkFollowStatus();
  };

  // 关注
  const handleFollow = async () => {
    if (!currentUpMid || currentUpMid === 0) {
      toast({
        type: "error",
        content: "无法关注该UP主",
      });
      return;
    }
    setIsFollowingLoading(true);
    try {
      const result = await Follow(currentUpMid);
      if (result) {
        setIsFollowing(true);
        toast({
          type: "success",
          content: `已关注 ${upName}`,
        });
      } else {
        toast({
          type: "error",
          content: "关注失败",
        });
      }
    } catch (error: any) {
      toast({
        type: "error",
        content: "关注失败: " + (error?.message || error?.toString() || "未知错误"),
      });
    } finally {
      setIsFollowingLoading(false);
    }
  };

  // 取消关注
  const handleUnfollow = async () => {
    if (!currentUpMid || currentUpMid === 0) {
      toast({
        type: "error",
        content: "无法取消关注该UP主",
      });
      return;
    }
    setIsFollowingLoading(true);
    try {
      const result = await Unfollow(currentUpMid);
      if (result) {
        setIsFollowing(false);
        toast({
          type: "success",
          content: `取消关注 ${upName}`,
        });
      } else {
        toast({
          type: "error",
          content: "取消关注失败",
        });
      }
    } catch (error: any) {
      toast({
        type: "error",
        content: "取消关注失败: " + (error?.message || error?.toString() || "未知错误"),
      });
    } finally {
      setIsFollowingLoading(false);
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
            <DrawerHeader className="flex items-center justify-between py-2">
              <div className="flex items-center gap-2 flex-grow">
                <span className="text-lg font-medium">「{upName}」的空间</span>
                <div className="flex items-center gap-1">
                  {currentUpMid > 0 && (
                    <Button
                      size="sm"
                      variant="light"
                      isLoading={isFollowingLoading}
                      isDisabled={isCheckingFollow || isFollowingLoading}
                      onPress={isFollowing ? handleUnfollow : handleFollow}
                      className="min-w-[70px] h-8 text-sm"
                    >
                      {isCheckingFollow ? (
                        <Spinner size="sm" />
                      ) : isFollowing ? (
                        <span className="flex items-center gap-1">
                          <Close fill="#666" size={14} />
                          已关注
                        </span>
                      ) : (
                        <span className="flex items-center gap-1 text-primary">
                          <Add fill="#2563eb" size={14} />
                          关注
                        </span>
                      )}
                    </Button>
                  )}
                  <Button
                    isIconOnly
                    size="sm"
                    variant="light"
                    onClick={handleRefresh}
                  >
                    <Refresh fill="#333" size={20} theme="outline" />
                  </Button>
                </div>
              </div>
                <Tabs
                  selectedKey={activeTab}
                  onSelectionChange={(key) => handleAciveTabChange(key.toString())}
                  variant="light"
                  classNames={{
                    tabList: "gap-2 mr-4",
                    cursor: "bg-default-100",
                    tab: "h-8 px-4",
                    tabContent: "group-data-[selected=true]:text-primary",
                  }}
                >
                <Tab key="videos" title="视频" />
                <Tab key="series" title="合集" />
              </Tabs>
            </DrawerHeader>
            <DrawerBody
              className="up-video-drawer-body"
              onScroll={handleScroll}
            >
              {activeTab === "videos" ? (
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
              ) : (
                <div className="flex flex-col gap-2">
                  {seriesList.map((series) => (
                    <Card
                      key={series.id}
                      isPressable
                      shadow="sm"
                      className={currentSeriesId === series.season_id ? "border-2 border-primary" : ""}
                      onPress={() => onSeriesSelect?.(series.season_id,series.name,series.total)}
                    >
                      <CardBody className="p-2">
                        <b className="line-clamp-2 text-sm">{series?.name || ""}</b>
                        <p className="text-xs text-default-500 mt-1">{series.total} 个视频</p>
                      </CardBody>
                    </Card>
                  ))}
                </div>
              )}
            </DrawerBody>
          </>
        )}
      </DrawerContent>
    </Drawer>
  );
};

export default UpVideoList;
