import type { FC } from "react";

import { useState } from "react";
import { useDisclosure } from "@heroui/react";
import {
  Button,
  Drawer,
  DrawerContent,
  DrawerBody,
  DrawerHeader,
} from "@heroui/react";
import { Close, Shuffle, Order, Delete, Play, FocusOne } from "@icon-park/react";

import RetryImg from "./retryImg";

import { graftingImage } from "@/utils/string";

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

export type PlaylistPlayMode = "sequence" | "shuffle";

interface PlaylistProps {
  onSlideClick?: () => void;
  playlist?: PlaylistItem[];
  currentPlaylistIndex?: number;
  playMode?: PlaylistPlayMode;
  isPlaylistMode?: boolean;
  onVideoSelect?: (index: number) => void;
  onDelete?: (id: string) => void;
  onReorder?: (from: number, to: number) => void;
  onClear?: () => void;
  onPlayModeToggle?: () => void;
}

const Playlist: FC<PlaylistProps> = ({
  onSlideClick,
  playlist = [],
  currentPlaylistIndex = -1,
  playMode = "sequence",
  isPlaylistMode = false,
  onVideoSelect,
  onDelete,
  onReorder,
  onClear,
  onPlayModeToggle,
}) => {
  const { isOpen, onOpenChange } = useDisclosure({ isOpen: true });
  const [dragIndex, setDragIndex] = useState<number | null>(null);
  const [dragOverIndex, setDragOverIndex] = useState<number | null>(null);

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
              <span className="text-sm font-medium">播放列表({playlist.length})</span>
              {isPlaylistMode && currentPlaylistIndex >= 0 && (
                <Button
                  isIconOnly
                  className="min-w-7 w-7 h-7"
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
                className="min-w-7 w-7 h-7"
                size="sm"
                title={
                  playMode === "sequence" ? "当前：顺序播放" : "当前：随机播放"
                }
                variant="light"
                onClick={onPlayModeToggle}
              >
                {playMode === "sequence" ? (
                  <Order fill="#888" size="18" theme="outline" />
                ) : (
                  <Shuffle fill="#3b82f6" size="18" theme="outline" />
                )}
              </Button>
              {playlist.length > 0 && (
                <Button
                  isIconOnly
                  className="min-w-7 w-7 h-7"
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
              {playlist.length === 0 ? (
                <div className="flex flex-col items-center justify-center h-full text-gray-400">
                  <p className="text-sm">播放列表为空</p>
                  <p className="text-xs mt-1">
                    从选集列表中点击 + 添加选集到播放列表
                  </p>
                </div>
              ) : (
                <div className="flex flex-col gap-1" style={{ width: "100%" }}>
                  {playlist.map((item, index) => {
                    const isCurrent =
                      isPlaylistMode && index === currentPlaylistIndex;
                    const isDragOver = dragOverIndex === index;

                    return (
                      <div
                        key={item.id}
                        draggable
                        role="button"
                        tabIndex={0}
                        className={`flex items-center gap-2 p-2 rounded-lg cursor-pointer transition-colors group ${
                          isCurrent
                            ? "bg-blue-100 playlist-current"
                            : isDragOver
                              ? "bg-blue-50 border-t-2 border-blue-400"
                              : "hover:bg-gray-100"
                        } ${dragIndex === index ? "opacity-40" : ""}`}
                        onClick={() => onVideoSelect?.(index)}
                        onKeyDown={(e) => {
                          if (e.key === "Enter" || e.key === " ") {
                            e.preventDefault();
                            onVideoSelect?.(index);
                          }
                        }}
                        onDragEnd={handleDragEnd}
                        onDragOver={(e) => handleDragOver(e, index)}
                        onDragStart={() => handleDragStart(index)}
                        onDrop={(e) => handleDrop(e, index)}
                      >
                        <div className="flex-shrink-0 w-16 h-9 rounded overflow-hidden bg-gray-200">
                          <RetryImg
                            alt={item.part}
                            className="w-full h-full object-cover"
                            src={graftingImage(item.first_frame || item.pic)}
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
                      </div>
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
