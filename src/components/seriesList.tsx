import type { FC } from "react";
import { useMemo, useRef } from "react";

import RetryImg from "./retryImg";
import { usePreloadImages } from "../hooks/usePreloadImages";

import { useDisclosure } from "@heroui/react";
import {
    Drawer,
    DrawerContent,
    DrawerBody,
    DrawerHeader,
    Card,
    CardBody,
    CardFooter,
} from "@heroui/react";

import { graftingImage, formatDatetime } from "@/utils/string";

const MAX_RETAINED_ITEMS = 240;
import { invoke } from "@tauri-apps/api/core";
import type { SeriesArchive } from "@/types/bilibili";

interface SeriesVideoItem {
    aid: number;
    bvid: string;
    title: string;
    cover: string;
    pubdate: number;
    duration: number;
    pic: string;
    stat: {
        view: number;
    }
}

interface SeriesListProps {
    onSlideClick?: () => void;
    seriesVideos?: SeriesVideoItem[];
    onVideoSelect?: (bvid: string) => void;
    seriesTitle?: string;
    currentBvid?: string;
    seriesVideosPage: number;
    setSeriesVideosPage: (page: number) => void;
    currentUpMid: number;
    currentSeriesId: number;
    setSeriesVideos: (videos: Array<any>) => void;
}

const SeriesList: FC<SeriesListProps> = ({
    onSlideClick,
    seriesVideos = [],
    onVideoSelect,
    seriesTitle = "",
    currentBvid,
    seriesVideosPage = 1,
    setSeriesVideosPage,
    currentUpMid = 0,
    currentSeriesId = 0,
    setSeriesVideos,
}) => {
    const { isOpen, onOpenChange } = useDisclosure({ isOpen: true });
    const isLoadingMoreRef = useRef(false);

    // 预加载合集视频封面图
    const coverUrls = useMemo(
        () => seriesVideos?.map((v) => graftingImage(v.pic)) ?? [],
        [seriesVideos],
    );
    usePreloadImages(coverUrls);

    const handleOpenChange = (open: boolean) => {
        if (!open) {
            onSlideClick?.();
        }
        onOpenChange();
    };

    const handleScroll = async (e: React.UIEvent<HTMLDivElement>) => {
        if (isLoadingMoreRef.current || seriesVideos.length >= MAX_RETAINED_ITEMS) return;
        const bottom =
            e.currentTarget.scrollHeight - e.currentTarget.scrollTop -
                e.currentTarget.clientHeight <= 80;
        if (bottom) {
            isLoadingMoreRef.current = true;
            const thePage = seriesVideosPage + 1;
            try {
                const seriesVideosData = await invoke<SeriesArchive[]>("get_series_videos", { mid: currentUpMid, seriesId: currentSeriesId, pageNum: thePage });
                if (seriesVideosData.length > 0) {
                    setSeriesVideos([...seriesVideos, ...seriesVideosData].slice(0, MAX_RETAINED_ITEMS));
                    setSeriesVideosPage(thePage);
                }
            } catch (error) {
                console.error("加载更多合集视频失败:", error);
            } finally {
                isLoadingMoreRef.current = false;
            }
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
                        <DrawerHeader className="flex gap-2 py-2">
                            {seriesTitle}
                        </DrawerHeader>
                        <DrawerBody onScroll={handleScroll}>
                            <div
                                className="gap-2 grid grid-cols-2 sm:grid-cols-3"
                                style={{ width: "100%" }}
                            >
                                {seriesVideos.map((video) => (
                                    <Card
                                        key={video.aid}
                                        isPressable
                                        className={currentBvid === video.bvid ? "border-2 border-primary" : ""}
                                        shadow="sm"
                                        onPress={() => onVideoSelect?.(video.bvid)}
                                    >
                                        <CardBody className="overflow-visible p-0 img-container">
                                            <RetryImg
                                                alt={video.title}
                                                className="c-cover"
                                                fallbackSrc="/cover.png"
                                                loading="lazy"
                                                radius="sm"
                                                shadow="sm"
                                                src={graftingImage(video.pic)}
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
                                                {formatDatetime(video?.pubdate)} | {video?.stat?.view}
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

export default SeriesList;
