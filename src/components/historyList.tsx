import type { FC } from "react";
import { useCallback, useMemo, useRef, useState } from "react";
import { Close, MaskOne, Refresh, Time } from "@icon-park/react";

import RetryImg from "./retryImg";
import { usePreloadImages } from "../hooks/usePreloadImages";

import { useDisclosure } from "@heroui/react";
import {
    Button,
    Tooltip,
    Drawer,
    DrawerContent,
    DrawerBody,
    DrawerHeader,
    Card,
    CardBody,
    CardFooter,
    Tabs,
    Tab,
} from "@heroui/react";
import { invoke } from "@tauri-apps/api/core";
import type { HistoryList as BLHistoryList } from "@/types/bilibili";
import type { WatchLaterItem as BLWatchLaterItem } from "@/types/bilibili";
import { convertToDuration, graftingImage } from "@/utils/string";

const MAX_RETAINED_ITEMS = 240;

type HistoryTab = "history" | "watchlater";

const HISTORY_TAB_STORAGE_KEY = "historyActiveTab";

const isHistoryTab = (value: string | null): value is HistoryTab =>
    value === "history" || value === "watchlater";

/**
 * 阻止卡片的 press 事件。
 * HeroUI 的 Card isPressable 走 React Aria usePress，它在 pointerdown/mousedown 阶段
 * 就开始识别按压，所以只在 onClick 里 stopPropagation 已经太晚 —— 点封面上的
 * "稍后再看 / 移除" 按钮会连带触发卡片的播放跳转。这里在按下阶段就截断冒泡。
 */
const stopCardPress = {
    onPointerDown: (e: React.PointerEvent) => e.stopPropagation(),
    onMouseDown: (e: React.MouseEvent) => e.stopPropagation(),
};

const loadInitialHistoryTab = (): HistoryTab => {
    try {
        const stored = window.localStorage.getItem(HISTORY_TAB_STORAGE_KEY);
        return isHistoryTab(stored) ? stored : "history";
    } catch {
        // localStorage 不可用（如隐私模式）时回退到默认 tab
        return "history";
    }
};

interface HistoryListProps {
    onSlideClick?: () => void;
    onVideoSelect?: (bvid: string) => void;
    historyList: any[];
    setHistoryList: (list: any[]) => void;
    historyCursor: { max: number, view_at: number, business: string };
    setHistoryCursor: (cursor: { max: number, view_at: number, business: string }) => void;
    isIncognitoMode?: boolean;
    onIncognitoModeChange?: (enabled: boolean) => void;
    watchLaterList?: BLWatchLaterItem[];
    onWatchLaterRefresh?: () => void | Promise<void>;
    onWatchLaterRemove?: (aid: number) => void | Promise<void>;
    onAddToWatchLater?: (aid: number) => void | Promise<void>;
}

const HistoryList: FC<HistoryListProps> = ({
    onSlideClick,
    onVideoSelect,
    historyList,
    setHistoryList,
    historyCursor,
    setHistoryCursor,
    isIncognitoMode = false,
    onIncognitoModeChange,
    watchLaterList = [],
    onWatchLaterRefresh,
    onWatchLaterRemove,
    onAddToWatchLater,
}) => {
    const { isOpen, onOpenChange } = useDisclosure({ isOpen: true });
    const isLoadingMoreRef = useRef(false);
    const [activeTab, setActiveTabState] = useState<HistoryTab>(loadInitialHistoryTab);
    const [isRefreshing, setIsRefreshing] = useState(false);
    // 正在请求中的 aid，避免连点重复提交
    const [pendingAids, setPendingAids] = useState<Set<number>>(new Set());

    // 切换 tab 时同步持久化，下次打开抽屉时恢复上次选择
    const setActiveTab = (tab: HistoryTab) => {
        setActiveTabState(tab);
        try {
            window.localStorage.setItem(HISTORY_TAB_STORAGE_KEY, tab);
        } catch {
            // 写入失败（如隐私模式）时忽略，仅影响下次恢复
        }
    };

    // 预加载历史记录封面图
    const coverUrls = useMemo(
        () => historyList?.map((item: any) => graftingImage(item.cover)) ?? [],
        [historyList],
    );
    usePreloadImages(coverUrls);

    // 已加入稍后再看的视频 aid 集合，用于历史记录卡片的"稍后再看"按钮状态
    const watchLaterAids = useMemo(
        () => new Set((watchLaterList ?? []).map((item) => item.aid)),
        [watchLaterList],
    );

    const handleOpenChange = (open: boolean) => {
        if (!open) {
            onSlideClick?.();
        }
        onOpenChange();
    };

    const handleScroll = (e: React.UIEvent<HTMLDivElement>) => {
        // 稍后再看接口无分页，只有观看历史支持加载更多
        if (activeTab !== "history") return;
        const bottom = e.currentTarget.scrollHeight - e.currentTarget.scrollTop - e.currentTarget.clientHeight <= 80;
        if (bottom) {
            handleLoadMore();
        }
    };

    const handleRefresh = async () => {
        if (isRefreshing) return;
        setIsRefreshing(true);
        // 两个 tab 都回到顶部，否则刷新后停在旧的滚动位置上
        const drawerBody = document.querySelector('.history-drawer-body');
        if (drawerBody) {
            drawerBody.scrollTop = 0;
        }
        try {
            if (activeTab === "watchlater") {
                await onWatchLaterRefresh?.();
                return;
            }
            setHistoryCursor({ max: 0, view_at: 0, business: '' });
            const data = await invoke<BLHistoryList>("get_history_list", { max: 0, viewAt: 0, business: '', ps: 30 });
            setHistoryList(data?.list || []);
            setHistoryCursor(data?.cursor || {});
        } catch (error) {
            console.error("刷新失败:", error);
        } finally {
            setIsRefreshing(false);
        }
    };

    const handleLoadMore = async () => {
        if (isLoadingMoreRef.current || (historyList?.length || 0) >= MAX_RETAINED_ITEMS) return;
        isLoadingMoreRef.current = true;
        try {
            const data = await invoke<BLHistoryList>("get_history_list", { max: historyCursor?.max, viewAt: historyCursor?.view_at, business: historyCursor?.business, ps: 30 });
            if (data?.list) {
                setHistoryList([...historyList, ...data.list].slice(0, MAX_RETAINED_ITEMS));
            }
            if (data?.cursor) { 
                setHistoryCursor(data?.cursor);
            }
        } catch (error) {
            console.error("加载更多历史记录失败:", error);
        } finally {
            isLoadingMoreRef.current = false;
        }
    };

    // 请求期间把 aid 记进 pendingAids，按钮置灰，避免连点重复请求
    const runWatchLaterAction = useCallback(
        async (aid: number, action?: (aid: number) => void | Promise<void>) => {
            if (!aid || !action) return;
            let shouldRun = false;
            setPendingAids((prev) => {
                if (prev.has(aid)) return prev;
                shouldRun = true;
                const next = new Set(prev);
                next.add(aid);
                return next;
            });
            if (!shouldRun) return;
            try {
                await action(aid);
            } finally {
                setPendingAids((prev) => {
                    if (!prev.has(aid)) return prev;
                    const next = new Set(prev);
                    next.delete(aid);
                    return next;
                });
            }
        },
        [],
    );

    const handleRemoveWatchLater = (aid: number) => {
        void runWatchLaterAction(aid, onWatchLaterRemove);
    };

    const handleAddWatchLater = (aid: number) => {
        void runWatchLaterAction(aid, onAddToWatchLater);
    };

    const formatTimestamp = (timestamp: number) => {
        const date = new Date(timestamp * 1000);
        return `${date.getFullYear()}-${String(date.getMonth() + 1).padStart(2, '0')}-${String(date.getDate()).padStart(2, '0')} ${String(date.getHours()).padStart(2, '0')}:${String(date.getMinutes()).padStart(2, '0')}`;
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
                        <DrawerHeader className="flex gap-2 py-2 items-center border-b border-default-100">
                            <Tabs
                                aria-label="历史/稍后再看切换"
                                selectedKey={activeTab}
                                variant="light"
                                onSelectionChange={(key) => setActiveTab(key as HistoryTab)}
                            >
                                <Tab key="history" title="观看历史" />
                                <Tab key="watchlater" title="稍后再看" />
                            </Tabs>
                            <Button
                                aria-label={activeTab === "history" ? "刷新历史记录" : "刷新稍后再看"}
                                isIconOnly
                                isDisabled={isRefreshing}
                                size="sm"
                                variant="light"
                                onClick={handleRefresh}
                            >
                                <Refresh
                                    className={isRefreshing ? "history-refresh-spinning" : undefined}
                                    theme="outline"
                                    size="20"
                                    fill="#333"
                                />
                            </Button>
                            {activeTab === "history" && (
                                <Tooltip
                                    closeDelay={100}
                                    content={isIncognitoMode
                                        ? "关闭后恢复云端播放记录与进度同步"
                                        : "开启后不读取或上报云端播放记录与进度"}
                                    delay={350}
                                    placement="bottom-end"
                                >
                                    <button
                                        aria-checked={isIncognitoMode}
                                        aria-label={isIncognitoMode ? "关闭隐身模式" : "开启隐身模式"}
                                        className="history-incognito-switch"
                                        data-active={isIncognitoMode || undefined}
                                        role="switch"
                                        type="button"
                                        onClick={() => onIncognitoModeChange?.(!isIncognitoMode)}
                                    >
                                        <MaskOne
                                            className="history-incognito-icon"
                                            size="15"
                                            theme={isIncognitoMode ? "filled" : "outline"}
                                        />
                                        <span className="history-incognito-label">隐身</span>
                                        <span aria-hidden="true" className="history-incognito-track">
                                            <span className="history-incognito-thumb" />
                                        </span>
                                    </button>
                                </Tooltip>
                            )}
                        </DrawerHeader>
                        {activeTab === "history" ? (
                            <DrawerBody
                                className="history-drawer-body"
                                onScroll={handleScroll}
                            >
                                <div
                                    className="gap-2 grid grid-cols-2 sm:grid-cols-3"
                                    style={{ width: "100%" }}
                                >
                                    {historyList?.map((item: any) => {
                                        const aid = Number(item?.history?.oid) || 0;
                                        const isWatchLater = aid > 0 && watchLaterAids.has(aid);
                                        return (
                                            <Card
                                                key={`${item?.history?.bvid}-${item?.view_at || item?.progress || 0}`}
                                                isPressable
                                                shadow="sm"
                                                className="c-list-card"
                                                onPress={() => onVideoSelect?.(item?.history?.bvid)}
                                            >
                                                <CardBody className="overflow-visible p-0 img-container">
                                                    <RetryImg
                                                        alt={item.title}
                                                        className="c-cover"
                                                        fallbackSrc="/cover.png"
                                                        loading="lazy"
                                                        radius="sm"
                                                        shadow="sm"
                                                        src={graftingImage(item.cover)}
                                                        width="100%"
                                                    />
                                                    {aid > 0 && (
                                                        <button
                                                            aria-label={isWatchLater ? "已在稍后再看" : "添加到稍后再看"}
                                                            className="history-watchlater-btn"
                                                            data-added={isWatchLater || undefined}
                                                            disabled={isWatchLater || pendingAids.has(aid)}
                                                            title={isWatchLater ? "已在稍后再看" : "添加到稍后再看"}
                                                            type="button"
                                                            {...stopCardPress}
                                                            onClick={(e) => {
                                                                e.stopPropagation();
                                                                if (!isWatchLater) {
                                                                    handleAddWatchLater(aid);
                                                                }
                                                            }}
                                                        >
                                                            <Time
                                                                size="14"
                                                                theme={isWatchLater ? "filled" : "outline"}
                                                            />
                                                        </button>
                                                    )}
                                                </CardBody>
                                                <CardFooter className="text-small flex-col items-start px-2 py-1">
                                                    <b
                                                        className="line-clamp-1 text-left w-full max-h-12 overflow-hidden"
                                                        title={item.title}
                                                    >
                                                        {item.title}
                                                    </b>
                                                    <p className="text-default-500 text-left w-full text-xs mt-1 line-clamp-1 max-h-10">
                                                        {item.author_name} | {formatTimestamp(item.view_at)}
                                                    </p>
                                                </CardFooter>
                                            </Card>
                                        );
                                    })}
                                </div>
                            </DrawerBody>
                        ) : (
                            <DrawerBody
                                className="history-drawer-body"
                            >
                                {(watchLaterList?.length || 0) === 0 ? (
                                    <div className="history-empty-tip" role="status">
                                        暂无稍后再看视频
                                    </div>
                                ) : (
                                    <div
                                        className="gap-2 grid grid-cols-2 sm:grid-cols-3"
                                        style={{ width: "100%" }}
                                    >
                                        {watchLaterList?.map((item) => {
                                            // B 站用 progress = -1 表示已看完，直接按比例算会得到负数而丢掉这个状态
                                            const progress = item.progress ?? 0;
                                            const ratio = item.duration > 0
                                                ? Math.min(100, Math.round((progress / item.duration) * 100))
                                                : 0;
                                            const progressLabel = progress < 0
                                                ? " | 已看完"
                                                : ratio > 0 ? ` | 已看 ${ratio}%` : "";
                                            const isPending = pendingAids.has(item.aid);
                                            return (
                                                <Card
                                                    key={`${item?.bvid}-${item?.aid}`}
                                                    isPressable
                                                    shadow="sm"
                                                    className="c-list-card"
                                                    onPress={() => onVideoSelect?.(item?.bvid)}
                                                >
                                                    <CardBody className="overflow-visible p-0 img-container">
                                                        <RetryImg
                                                            alt={item.title}
                                                            className="c-cover"
                                                            fallbackSrc="/cover.png"
                                                            loading="lazy"
                                                            radius="sm"
                                                            shadow="sm"
                                                            src={graftingImage(item.pic)}
                                                            width="100%"
                                                        />
                                                        <span className="watchlater-duration-badge">
                                                            {convertToDuration(item.duration)}
                                                        </span>
                                                        <button
                                                            aria-label="从稍后再看移除"
                                                            className="watchlater-remove-btn"
                                                            disabled={isPending}
                                                            title="从稍后再看移除"
                                                            type="button"
                                                            {...stopCardPress}
                                                            onClick={(e) => {
                                                                e.stopPropagation();
                                                                handleRemoveWatchLater(item.aid);
                                                            }}
                                                        >
                                                            <Close size="12" theme="outline" />
                                                        </button>
                                                    </CardBody>
                                                    <CardFooter className="text-small flex-col items-start px-2 py-1">
                                                        <b
                                                            className="line-clamp-1 text-left w-full max-h-12 overflow-hidden"
                                                            title={item.title}
                                                        >
                                                            {item.title}
                                                        </b>
                                                        <p className="text-default-500 text-left w-full text-xs mt-1 line-clamp-1 max-h-10">
                                                            {item.owner?.name}
                                                            {progressLabel}
                                                        </p>
                                                    </CardFooter>
                                                </Card>
                                            );
                                        })}
                                    </div>
                                )}
                            </DrawerBody>
                        )}
                    </>
                )}
            </DrawerContent>
        </Drawer>
    );
};

export default HistoryList;
