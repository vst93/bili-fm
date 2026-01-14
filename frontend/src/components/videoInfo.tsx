import { useState, useEffect } from "react";
import { Button, Image } from "@heroui/react";
import {
  Search,
  DoubleUp,
  Browser,
  ShareSys,
  ChartRing,
  WeixinFavorites,
  ThumbsUp,
  HandleB,
  History,
  Layers,
  VideoTwo,
} from "@icon-park/react";

import {
  LikeVideo,
  HasLiked,
  CoinVideo,
  HasCoin,
} from "../../wailsjs/go/service/BL.js";
import { toast } from "../utils/toast";

import { graftingImage } from "@/utils/string";

interface VideoInfoProps {
  title?: string;
  desc?: string;
  ownerName?: string;
  ownerFace?: string;
  ownerMid?: number;
  part?: string;
  bvid?: string;
  aid?: number;
  onSearchClick?: () => void;
  onPageListClick?: () => void;
  onShareClick?: () => void;
  onOwnerClick?: (mid: number, name: string) => void;
  onFeedClick?: () => void;
  onRecommendClick?: () => void;
  onCollectClick?: () => void;
  onHistoryClick?: () => void;
  onSeriesClick?: () => void;
  onPlayVideoClick?: () => void;
  currentSeriesTitle?: string;
  searchResultsCount?: number;
}

export default function VideoInfo({
  title = "暂无播放内容",
  desc = "",
  ownerName = "",
  ownerFace = "",
  ownerMid = 0,
  part = "",
  bvid = "",
  onSearchClick,
  onPageListClick,
  onShareClick,
  onOwnerClick,
  onFeedClick,
  onRecommendClick,
  onCollectClick,
  onHistoryClick,
  onSeriesClick,
  onPlayVideoClick,
  currentSeriesTitle,
  searchResultsCount = 0,
}: VideoInfoProps) {
  const [isLiked, setIsLiked] = useState(false);
  const [coinCount, setCoinCount] = useState(0);

  const checkLikeStatus = async () => {
    try {
      const hasLiked = await HasLiked(bvid);
      setIsLiked(hasLiked);
    } catch (error) {
      console.error("检查点赞状态失败:", error);
    }
  };

  const checkCoinStatus = async () => {
    try {
      const coins = await HasCoin(bvid);
      setCoinCount(coins);
    } catch (error) {
      console.error("检查投币状态失败:", error);
    }
  };

  useEffect(() => {
    if (bvid) {
      checkLikeStatus();
      checkCoinStatus();
    } else {
      setIsLiked(false);
      setCoinCount(0);
    }
  }, [bvid]);

  const handleLike = async () => {
    try {
      const result = await LikeVideo(bvid, isLiked ? 2 : 1);
      if (result) {
        setIsLiked(!isLiked);
        toast({
          type: "success",
          content: isLiked ? "取消点赞成功" : "点赞成功",
        });
      }
    } catch (error: any) {
      const errorMsg = error?.message || error?.toString() || "未知错误";
      toast({
        type: "error",
        content: "点赞失败: " + errorMsg,
      });
    }
  };

  const handleCoin = async () => {
    if (coinCount >= 2) {
      toast({
        type: "info",
        content: "已经投过币了",
      });
      return;
    }
    try {
      const result = await CoinVideo(bvid, 2);
      if (result) {
        setCoinCount(2);
        toast({
          type: "success",
          content: "投币成功",
        });
      }
    } catch (error: any) {
      const errorMsg = error?.message || error?.toString() || "未知错误";
      toast({
        type: "error",
        content: "投币失败: " + errorMsg,
      });
    }
  };

  return (
    <div id="video-info">
      {/* Owner section */}
      <div className="flex items-center gap-3 mb-4" id="video-owner">
        <Image
          alt={ownerName}
          className="cursor-pointer transition-transform hover:scale-105"
          classNames={{
            wrapper: "min-w-[48px]",
            img: "object-cover",
          }}
          height={48}
          id="video-owner-face"
          loading="lazy"
          radius="full"
          src={graftingImage(
            ownerFace || "https://i0.hdslb.com/bfs/face/member/noface.jpg",
          )}
          width={48}
          onClick={() => onOwnerClick?.(ownerMid, ownerName)}
        />
        <button
          className="cursor-pointer bg-transparent border-none p-0 text-left"
          id="video-owner-name"
          onClick={() => onOwnerClick?.(ownerMid, ownerName)}
        >
          <span className="text-lg font-medium text-gray-800 hover:text-blue-500 transition-colors">
            {ownerName || "神秘的UP主"}
          </span>
        </button>
      </div>

      {/* Title section */}
      <div
        className="can-seelect mb-3 relative group"
        id="video-title"
      >
        <h2 className="text-xl font-semibold text-gray-900 leading-snug line-clamp-2 pr-16">
          {title || "无标题"}
        </h2>
        <div className="absolute left-0 top-full flex gap-2 mt-2 opacity-0 invisible group-hover:opacity-100 group-hover:visible transition-all duration-150">
          <div
            className="flex items-center gap-1.5 px-3 py-1.5 rounded-lg bg-gray-300 hover:bg-blue-200 cursor-pointer transition-colors"
            onClick={onShareClick}
          >
            <Browser fill="#333" size="14" theme="outline" />
            <span className="text-sm text-gray-800 font-medium">浏览器</span>
          </div>
          <div
            className="flex items-center gap-1.5 px-3 py-1.5 rounded-lg bg-gray-300 hover:bg-purple-200 cursor-pointer transition-colors"
            onClick={onPlayVideoClick}
          >
            <VideoTwo fill="#333" size="14" theme="outline" />
            <span className="text-sm text-gray-800 font-medium">播放</span>
          </div>
        </div>
      </div>

      {/* Description */}
      <div className="can-seelect mb-3" id="video-desc">
        <p className="text-sm text-gray-500 leading-relaxed line-clamp-3 min-h-[4.5em]">
          {desc || "无描述"}
        </p>
      </div>

      {/* Current part indicator */}
      <div className="can-seelect mb-4" id="video-part" title={part || "无选集标题"}>
        <span className="inline-flex items-center gap-2 px-3 py-1.5 rounded-full bg-blue-50 text-blue-600 text-sm font-medium max-w-full">
          <span className="w-2 h-2 rounded-full bg-blue-500 animate-pulse flex-shrink-0" />
          <span className="truncate">{part || "无选集标题"}</span>
        </span>
      </div>

      {/* Action buttons */}
      <div className="info-tools">
        <Button
          className="info-tools-button"
          size="sm"
          style={{
            backgroundColor: "transparent",
            minWidth: "32px",
            width: "32px",
            padding: "0",
          }}
          title={isLiked ? "取消点赞" : "点赞"}
          variant="light"
          isDisabled={!bvid}
          onPress={handleLike}
        >
          <ThumbsUp
            fill={isLiked ? "#ec4899" : "#666"}
            size={24}
            theme="outline"
          />
        </Button>
        <Button
          className="info-tools-button"
          size="sm"
          style={{
            backgroundColor: "transparent",
            minWidth: "32px",
            width: "32px",
            padding: "0",
          }}
          title={coinCount >= 2 ? "已投币" : "投币(2个)"}
          variant="light"
          isDisabled={!bvid}
          onPress={handleCoin}
        >
          <HandleB
            fill={coinCount >= 2 ? "#eab308" : "#666"}
            size={24}
            theme="outline"
          />
        </Button>
        <Button
          className="info-tools-button"
          size="sm"
          style={{
            backgroundColor: "#f3f4f6",
          }}
          title="搜索"
          isDisabled={!searchResultsCount}
          onPress={onSearchClick}
        >
          <Search fill="#666" size={24} theme="outline" />
        </Button>
        <Button
          className="info-tools-button"
          size="sm"
          style={{
            backgroundColor: "#f3f4f6",
          }}
          title="选集"
          isDisabled={!bvid}
          onPress={onPageListClick}
        >
          <DoubleUp fill="#666" size={24} theme="outline" />
        </Button>
        <Button
          className="info-tools-button"
          size="sm"
          style={{
            backgroundColor: "#f3f4f6",
          }}
          title="合集"
          isDisabled={!currentSeriesTitle}
          onPress={onSeriesClick}
        >
          <Layers theme="outline" size={24} fill="#666" />
        </Button>
        <div className="info-tools-bl-user-group">
          <Button
            className="info-tools-button bl-feed"
            size="sm"
            title="B站账号关注UP视频动态列表"
            onPress={onFeedClick}
          >
            <ShareSys fill="#666" size={24} theme="outline" />
          </Button>
          <Button
            className="info-tools-button bl-rcmd"
            size="sm"
            title="B站账号推荐视频列表"
            onPress={onRecommendClick}
          >
            <ChartRing fill="#666" size={24} theme="outline" />
          </Button>
          <Button
            className="info-tools-button bl-collect"
            size="sm"
            title="B站账号收藏列表"
            onPress={onCollectClick}
          >
            <WeixinFavorites fill="#666" size={24} theme="outline" />
          </Button>
          <Button
            className="info-tools-button bl-collect"
            size="sm"
            title="B站账号历史记录"
            onPress={onHistoryClick}
          >
            <History theme="outline" size={24} fill="#666" />
          </Button>
        </div>
      </div>
    </div>
  );
}
