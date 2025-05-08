import type { FC } from "react";
import { Refresh } from "@icon-park/react";

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
import { GetBLHistoryList } from "../../wailsjs/go/service/BL";
import { graftingImage } from "@/utils/string";

interface HistoryListProps {
    onSlideClick?: () => void;
    onVideoSelect?: (bvid: string) => void;
    historyList: any[];
    setHistoryList: (list: any[]) => void;
    historyCursor: { max: number, view_at: number, business: string };
    setHistoryCursor: (cursor: { max: number, view_at: number, business: string }) => void;
}

const HistoryList: FC<HistoryListProps> = ({
    onSlideClick,
    onVideoSelect,
    historyList,
    setHistoryList,
    historyCursor,
    setHistoryCursor,
}) => {
    const { isOpen, onOpenChange } = useDisclosure({ isOpen: true });

    const handleOpenChange = (open: boolean) => {
        if (!open) {
            onSlideClick?.();
        }
        onOpenChange();
    };

    const handleScroll = (e: React.UIEvent<HTMLDivElement>) => {
        const bottom = e.currentTarget.scrollHeight - e.currentTarget.scrollTop === e.currentTarget.clientHeight;
        if (bottom) {
            handleLoadMore();
        }
    };

    const handleRefresh = async () => {
        const drawerBody = document.querySelector('.history-drawer-body');
        if (drawerBody) {
            drawerBody.scrollTop = 0;
        }
        setHistoryCursor({max: 0, view_at: 0, business: ''});
        try {
            const data = await GetBLHistoryList(0,0,'', 30);
            setHistoryList(data?.list || []);
            setHistoryCursor(data?.cursor || {});
        } catch (error) {
            console.error("刷新历史记录失败:", error);
        }
    };

    const handleLoadMore = async () => {
        console.log("load more", historyCursor);
        try {
            const data = await GetBLHistoryList(historyCursor?.max, historyCursor?.view_at, historyCursor?.business, 30);
            if (data?.list) {
                setHistoryList([...historyList, ...data.list]);
            }
            if (data?.cursor) { 
                setHistoryCursor(data?.cursor);
            }
        } catch (error) {
            console.error("加载更多历史记录失败:", error);
        }
    };

    const formatTimestamp = (timestamp: number) => {
        const date = new Date(timestamp * 1000);
        return `${date.getFullYear()}-${String(date.getMonth() + 1).padStart(2, '0')}-${String(date.getDate()).padStart(2, '0')} ${String(date.getHours()).padStart(2, '0')}:${String(date.getMinutes()).padStart(2, '0')}`;
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
                            观看历史
                            <Button
                                isIconOnly
                                size="sm"
                                variant="light"
                                onClick={handleRefresh}
                            >
                                <Refresh theme="outline" size="20" fill="#333" />
                            </Button>
                        </DrawerHeader>
                        <DrawerBody className="history-drawer-body" onScroll={handleScroll}>
                            <div
                                className="gap-2 grid grid-cols-2 sm:grid-cols-3"
                                style={{ width: "100%" }}
                            >
                                {historyList?.map((item: any, index: number) => {
                                    return (
                                        <Card
                                            key={index}
                                            isPressable
                                            shadow="sm"
                                            onPress={() => onVideoSelect?.(item?.history?.bvid)}
                                        >
                                            <CardBody className="overflow-visible p-0 img-container">
                                                <Image
                                                    alt={item.title}
                                                    className="c-cover"
                                                    crossOrigin="anonymous"
                                                    fallbackSrc="/cover.png"
                                                    loading="lazy"
                                                    radius="sm"
                                                    shadow="sm"
                                                    src={graftingImage(item.cover)}
                                                    width="100%"
                                                />
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
                    </>
                )}
            </DrawerContent>
        </Drawer>
    );
};

export default HistoryList;