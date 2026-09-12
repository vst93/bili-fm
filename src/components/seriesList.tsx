import type { FC } from "react";
import { useMemo, useRef, useState } from "react";

import { usePreloadImages } from "../hooks/usePreloadImages";

import { useDisclosure } from "@heroui/react";
import {
    Button,
    Drawer,
    DrawerContent,
    DrawerBody,
    DrawerHeader,
} from "@heroui/react";
import { Play } from "@icon-park/react";
import ListCard from "./listCard";

import { graftingImage, formatViewCount, formatRelativeTime, convertToDuration } from "@/utils/string";

const MAX_RETAINED_ITEMS = 128;
import { appendWithRetention } from "@/lib/listRetention";
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
    onPlayAll?: (videos: SeriesVideoItem[]) => void | Promise<void>;
    seriesTitle?: string;
    currentBvid?: string;
    seriesVideosPage: number;
    setSeriesVideosPage: (page: number) => void;
    currentUpMid: number;
    currentSeriesId: number;
    setSeriesVideos: (videos: Array<any>) => void;
    scrollAnchorSampleRef?: { current: Record<string, { top: number; height: number }> };
    pendingAnchorAdjustRef?: { current: Record<string, { prevTop: number; prevHeight: number }> };
}

const SeriesList: FC<SeriesListProps> = ({
    onSlideClick,
    seriesVideos = [],
    onVideoSelect,
    onPlayAll,
    seriesTitle = "",
    currentBvid,
    seriesVideosPage = 1,
    setSeriesVideosPage,
    currentUpMid = 0,
    currentSeriesId = 0,
    setSeriesVideos,
    scrollAnchorSampleRef,
    pendingAnchorAdjustRef,
}) => {
    const { isOpen, onOpenChange } = useDisclosure({ isOpen: true });
    const isLoadingMoreRef = useRef(false);
    const [isPlayingAll, setIsPlayingAll] = useState(false);

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
        if (isLoadingMoreRef.current) return;
        const bottom = e.currentTarget.scrollHeight - e.currentTarget.scrollTop - e.currentTarget.clientHeight <= 80;
        if (bottom) {
            isLoadingMoreRef.current = true;
            const thePage = seriesVideosPage + 1;
            const anchor = scrollAnchorSampleRef?.current.series;
            try {
                const seriesVideosData = await invoke<SeriesArchive[]>("get_series_videos", { mid: currentUpMid, seriesId: currentSeriesId, pageNum: thePage });
                if (seriesVideosData.length > 0) {
                    const merged = appendWithRetention(seriesVideos, seriesVideosData, MAX_RETAINED_ITEMS);
                    setSeriesVideos(merged);
                    setSeriesVideosPage(thePage);
                    if (anchor) pendingAnchorAdjustRef!.current.series = { prevTop: anchor.top, prevHeight: anchor.height };
                }
            } catch (error) {
                console.error("加载更多合集视频失败:", error);
            } finally {
                isLoadingMoreRef.current = false;
            }
        }
    };

    const handlePlayAll = async () => {
        if (isPlayingAll || seriesVideos.length === 0) return;

        setIsPlayingAll(true);
        try {
            await onPlayAll?.(seriesVideos);
        } finally {
            setIsPlayingAll(false);
        }
    };

    return (
        <Drawer
            /* 与其他抽屉（选集/动态/推荐…）统一为固定高度，抽屉内容多时 DrawerBody 滚动 */
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
                            <span className="text-sm min-w-0 truncate">{seriesTitle}</span>
                            <Button
                                isDisabled={seriesVideos.length === 0}
                                isLoading={isPlayingAll}
                                size="sm"
                                title="播放合集中的全部视频"
                                variant="flat"
                                onClick={handlePlayAll}
                            >
                                {!isPlayingAll && (
                                    <Play fill="#666" size="16" theme="filled" />
                                )}
                                <span className="text-xs">播放全部</span>
                            </Button>
                        </DrawerHeader>
                        <DrawerBody className="series-drawer-body" onScroll={handleScroll}>
                            {seriesVideos.length === 0 ? (
                                <div className="history-empty-tip" role="status">
                                    暂无合集视频
                                </div>
                            ) : (
                            <div
                                className="gap-2 grid grid-cols-2 sm:grid-cols-3"
                                style={{ width: "100%" }}
                            >
                                {seriesVideos.map((video) => (
                                    <ListCard
                                        key={video.aid}
                                        cardClassName={`c-list-card${currentBvid === video.bvid ? " border-2 border-primary" : ""}`}
                                        cover={video.pic}
                                        coverAlt={video.title}
                                        duration={video?.duration ? convertToDuration(video.duration) : null}
                                        fields={[
                                            { kind: "views", value: video?.stat?.view != null ? formatViewCount(video.stat.view) : null },
                                            { kind: "pubdate", value: video?.pubdate ? formatRelativeTime(video.pubdate) : null },
                                        ]}
                                        onPress={() => onVideoSelect?.(video.bvid)}
                                        title={video.title}
                                    />
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

export default SeriesList;
