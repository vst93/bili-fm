import type { FC } from "react";

import { useMemo, useState } from "react";
import { useDisclosure } from "@heroui/react";
import {
  Button,
  Drawer,
  DrawerContent,
  DrawerBody,
  DrawerHeader,
  Tabs,
  Tab,
} from "@heroui/react";
import { Close, Shuffle, Order, Delete, Play, FocusOne, LoopOnce } from "@icon-park/react";

import RetryImg from "./retryImg";

import { graftingImage } from "@/utils/string";
import {
  UNLOAD_PLACEHOLDER,
  useViewportImageUnload,
} from "@/hooks/useViewportImageUnload";

export interface PlaylistItem {
  id: string;
  bvid: string;
  aid: number;
  cid: number;
  part: string;
  first_frame: string;
  title: string;
  pic: string;
}

export type PlaylistPlayMode = "sequence" | "single" | "shuffle";

interface PlaylistRowProps {
  item: PlaylistItem;
  index: number;
  isCurrent: boolean;
  isDragOver: boolean;
  isDragging: boolean;
  isSeriesPlaylist: boolean;
  onSelect: (index: number) => void;
  onDelete?: (id: string) => void;
  onDragStart?: (index: number) => void;
  onDragOver?: (event: React.DragEvent, index: number) => void;
  onDrop?: (event: React.DragEvent, index: number) => void;
  onDragEnd?: () => void;
}

/**
 * 单条播放列表记录（轮 31 抽出）。
 *
 * 播放列表（尤其「我的列表」）可以攒到上百条，每条都有一张 192w 缩略图。
 * 轮 29 的离屏位图卸载只接进了网格卡片（ListCard），播放列表行仍是普通 div ——
 * 它是网页里最后一块「长列表缩略图不会主动丢解码位图」的区域。抽成独立组件后，
 * 就能像 ListCard 一样用 useViewportImageUnload：滚出 ≥2 屏把缩略图换 1px 占位、
 * 回到 1 屏内还原。DOM 结构、类名与拖拽处理完全不变（ref 挂在行根 div 上，
 * 不影响 .c-list-card-row 的 content-visibility / 布局）。
 */
const PlaylistRow: FC<PlaylistRowProps> = ({
  item,
  index,
  isCurrent,
  isDragOver,
  isDragging,
  isSeriesPlaylist,
  onSelect,
  onDelete,
  onDragStart,
  onDragOver,
  onDrop,
  onDragEnd,
}) => {
  const { ref, unloaded } = useViewportImageUnload<HTMLDivElement>();
  const coverSrc = graftingImage(item.first_frame || item.pic, 192);

  return (
    <div
      ref={ref}
      draggable={!isSeriesPlaylist}
      role="button"
      tabIndex={0}
      className={`flex items-center gap-2 p-2 rounded-lg cursor-pointer transition-colors group c-list-card c-list-card-row ${
        isCurrent
          ? "bg-blue-100 playlist-current"
          : isDragOver
            ? "bg-blue-50 border-t-2 border-blue-400"
            : "hover:bg-gray-100"
      } ${isDragging ? "opacity-40" : ""}`}
      onClick={() => onSelect(index)}
      onKeyDown={(e) => {
        if (e.key === "Enter" || e.key === " ") {
          e.preventDefault();
          onSelect(index);
        }
      }}
      onDragEnd={isSeriesPlaylist ? undefined : onDragEnd}
      onDragOver={isSeriesPlaylist ? undefined : (e) => onDragOver?.(e, index)}
      onDragStart={isSeriesPlaylist ? undefined : () => onDragStart?.(index)}
      onDrop={isSeriesPlaylist ? undefined : (e) => onDrop?.(e, index)}
    >
      <div className="flex-shrink-0 w-16 h-9 rounded overflow-hidden bg-gray-200">
        <RetryImg
          alt={item.part}
          className="w-full h-full object-cover"
          src={unloaded ? UNLOAD_PLACEHOLDER : coverSrc}
        />
      </div>

      <div className="flex-1 min-w-0">
        <p
          className={`text-sm truncate ${isCurrent ? "text-blue-600 font-medium" : "text-gray-800"}`}
          title={item.part}
        >
          {isCurrent && (
            <Play
              className="inline-block mr-1 align-middle"
              fill="#3b82f6"
              size="12"
              theme="filled"
            />
          )}
          {item.part}
        </p>
        <p className="text-xs text-gray-400 truncate">
          {item.title}
        </p>
      </div>

      {!isSeriesPlaylist && (
        <Button
          isIconOnly
          className="flex-shrink-0 opacity-0 group-hover:opacity-100 transition-opacity"
          size="sm"
          title="删除"
          variant="light"
          onClick={(e: React.MouseEvent) => {
            e.stopPropagation();
            onDelete?.(item.id);
          }}
        >
          <Close fill="#999" size="16" theme="outline" />
        </Button>
      )}
    </div>
  );
};

interface PlaylistProps {
  onSlideClick?: () => void;
  playlist?: PlaylistItem[];
  currentPlaylistIndex?: number;
  seriesPlaylist?: PlaylistItem[];
  currentSeriesPlaylistIndex?: number;
  activePlaylistType?: "user" | "series";
  playingPlaylistType?: "user" | "series";
  playingIndex?: number;
  playMode?: PlaylistPlayMode;
  isPlaylistMode?: boolean;
  onVideoSelect?: (index: number) => void;
  onDelete?: (id: string) => void;
  onReorder?: (from: number, to: number) => void;
  onClear?: () => void;
  onPlayModeToggle?: () => void;
  onSwitchPlaylistType?: (type: "user" | "series") => void;
}

const Playlist: FC<PlaylistProps> = ({
  onSlideClick,
  playlist = [],
  currentPlaylistIndex = -1,
  seriesPlaylist = [],
  currentSeriesPlaylistIndex = -1,
  activePlaylistType = "user",
  playingPlaylistType,
  playingIndex,
  playMode = "sequence",
  isPlaylistMode = false,
  onVideoSelect,
  onDelete,
  onReorder,
  onClear,
  onPlayModeToggle,
  onSwitchPlaylistType,
}) => {
  const { isOpen, onOpenChange } = useDisclosure({ isOpen: true });
  const [dragIndex, setDragIndex] = useState<number | null>(null);
  const [dragOverIndex, setDragOverIndex] = useState<number | null>(null);
  const isSeriesPlaylist = activePlaylistType === "series";
  const activePlaylist = isSeriesPlaylist ? seriesPlaylist : playlist;
  // 合集列表为空时不显示第二个 tab，只留「我的列表」。
  const playlistTabs = useMemo(
    () =>
      [
        { key: "user", title: `我的列表(${playlist.length})` },
        ...(seriesPlaylist.length > 0
          ? [{ key: "series", title: `合集列表(${seriesPlaylist.length})` }]
          : []),
      ],
    [playlist.length, seriesPlaylist.length],
  );
  const activePlaylistIndex = isSeriesPlaylist
    ? currentSeriesPlaylistIndex
    : currentPlaylistIndex;

  const handleOpenChange = (open: boolean) => {
    if (!open) {
      onSlideClick?.();
    }
    onOpenChange();
  };

  const handleDragStart = (index: number) => {
    setDragIndex(index);
  };

  const handleDragOver = (e: React.DragEvent, index: number) => {
    e.preventDefault();
    if (dragIndex !== null && dragIndex !== index) {
      setDragOverIndex(index);
    }
  };

  const handleDrop = (e: React.DragEvent, index: number) => {
    e.preventDefault();
    if (dragIndex !== null && dragIndex !== index) {
      onReorder?.(dragIndex, index);
    }
    setDragIndex(null);
    setDragOverIndex(null);
  };

  const handleDragEnd = () => {
    setDragIndex(null);
    setDragOverIndex(null);
  };

  // 定位到当前播放的记录
  const handlePositionCurrent = () => {
    const el = document.querySelector(".playlist-current") as HTMLElement;
    if (el) {
      el.scrollIntoView({ behavior: "smooth", block: "center" });
    }
  };

  return (
    <Drawer
      classNames={{ base: "h-[92vh] max-h-[calc(100vh-54px)]" }}
      isOpen={isOpen}
      placement="bottom"
      onOpenChange={handleOpenChange}
    >
      <DrawerContent>
        {() => (
          <>
            <DrawerHeader className="flex items-center gap-2 py-2">
              <Tabs
                aria-label="播放列表来源切换"
                items={playlistTabs}
                selectedKey={isSeriesPlaylist ? "series" : "user"}
                variant="light"
                onSelectionChange={(key) =>
                  onSwitchPlaylistType?.(key as "user" | "series")
                }
              >
                {(item) => <Tab key={item.key} title={item.title} />}
              </Tabs>
              {isPlaylistMode && activePlaylistIndex >= 0 && (
                <Button
                  isIconOnly
                  size="sm"
                  title="定位到当前播放的记录"
                  variant="light"
                  onClick={handlePositionCurrent}
                >
                  <FocusOne fill="#888" size="18" theme="outline" />
                </Button>
              )}
              <Button
                isIconOnly
                size="sm"
                title={
                  playMode === "sequence"
                    ? "当前：顺序播放"
                    : playMode === "single"
                    ? "当前：单曲循环"
                    : "当前：随机播放"
                }
                variant="light"
                onClick={onPlayModeToggle}
              >
                {playMode === "sequence" ? (
                  <Order fill="#888" size="18" theme="outline" />
                ) : playMode === "single" ? (
                  <LoopOnce fill="#3b82f6" size="18" theme="outline" />
                ) : (
                  <Shuffle fill="#3b82f6" size="18" theme="outline" />
                )}
              </Button>
              {!isSeriesPlaylist && playlist.length > 0 && (
                <Button
                  isIconOnly
                  size="sm"
                  title="清空播放列表"
                  variant="light"
                  onClick={onClear}
                >
                  <Delete fill="#888" size="18" theme="outline" />
                </Button>
              )}
            </DrawerHeader>
            <DrawerBody>
              {activePlaylist.length === 0 ? (
                <div className="flex flex-col items-center justify-center h-full text-gray-400">
                  <p className="text-sm">播放列表为空</p>
                  <p className="text-xs mt-1">
                    从选集列表中点击 + 添加选集到播放列表
                  </p>
                </div>
              ) : (
                <div className="flex flex-col gap-1" style={{ width: "100%" }}>
                  {activePlaylist.map((item, index) => {
                    const isCurrent =
                      isPlaylistMode &&
                      activePlaylistType === playingPlaylistType &&
                      index === playingIndex;
                    const isDragOver =
                      !isSeriesPlaylist && dragOverIndex === index;

                    return (
                      <PlaylistRow
                        key={item.id}
                        index={index}
                        isCurrent={isCurrent}
                        isDragOver={isDragOver}
                        isDragging={dragIndex === index}
                        isSeriesPlaylist={isSeriesPlaylist}
                        item={item}
                        onDelete={onDelete}
                        onDragEnd={handleDragEnd}
                        onDragOver={handleDragOver}
                        onDragStart={handleDragStart}
                        onDrop={handleDrop}
                        onSelect={(i) => onVideoSelect?.(i)}
                      />
                    );
                  })}
                </div>
              )}
            </DrawerBody>
          </>
        )}
      </DrawerContent>
    </Drawer>
  );
};

export default Playlist;
