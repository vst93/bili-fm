import { useState, useEffect } from "react";
import {
  Search,
  DoubleUp,
  Browser,
  ThumbsUp,
  HandleB,
  Layers,
  VideoTwo,
  MusicList,
  Comment,
} from "@icon-park/react";

import {
  LikeVideo,
  HasLiked,
  CoinVideo,
  HasCoin,
} from "../../wailsjs/go/service/BL.js";
import { toast } from "../utils/toast";

import RetryImg from "./retryImg";

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
  onPlaylistClick?: () => void;
  onDanmakuClick?: () => void;
  isPlaylistMode?: boolean;
  currentSeriesTitle?: string;
  searchResultsCount?: number;
  playlistCount?: number;
  cid?: number;
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
  onSeriesClick,
  onPlayVideoClick,
  onPlaylistClick,
  onDanmakuClick,
  isPlaylistMode = false,
  currentSeriesTitle,
  searchResultsCount = 0,
  playlistCount = 0,
  cid,
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
        toast({ type: "success", content: isLiked ? "取消点赞成功" : "点赞成功" });
      }
    } catch (error: any) {
      toast({
        type: "error",
        content: "点赞失败: " + (error?.message || error?.toString() || "未知错误"),
      });
    }
  };

  const handleCoin = async () => {
    if (coinCount >= 2) {
      toast({ type: "info", content: "已经投过币了" });
      return;
    }
    try {
      const result = await CoinVideo(bvid, 2);
      if (result) {
        setCoinCount(2);
        toast({ type: "success", content: "投币成功" });
      }
    } catch (error: any) {
      toast({
        type: "error",
        content: "投币失败: " + (error?.message || error?.toString() || "未知错误"),
      });
    }
  };

  const gray = "#64748b";
  const blue = "#0ea5e9";

  return (
    <div id="video-info">
      <div className="video-meta-area">
        <div className="video-info-head">
          <div className="video-owner-wrap">
            <RetryImg
              alt={ownerName}
              className="cursor-pointer transition-transform hover:scale-105 flex-shrink-0"
              height={40}
              id="video-owner-face"
              loading="lazy"
              radius="full"
              src={graftingImage(
                ownerFace || "https://i0.hdslb.com/bfs/face/member/noface.jpg",
              )}
              width={40}
              onClick={() => onOwnerClick?.(ownerMid, ownerName)}
            />
            <div className="min-w-0">
              <span className="video-kicker">UP 主</span>
              <button
                id="video-owner-name"
                className="truncate bg-transparent border-none cursor-pointer p-0"
                onClick={() => onOwnerClick?.(ownerMid, ownerName)}
              >
                {ownerName || "神秘的UP主"}
              </button>
            </div>
          </div>

        </div>

        <div className="can-seelect" id="video-title">
          <h2>{title || "无标题"}</h2>
        </div>

        <div className="can-seelect" id="video-desc">
          <p>{desc || "无描述"}</p>
        </div>

        <div className="video-part-pill" title={part || "无选集标题"}>
          <span
            className={`video-part-dot ${
              isPlaylistMode ? "video-part-dot-playlist" : ""
            }`}
          />
          <span className="video-part-source">
            {isPlaylistMode ? "播放列表" : "当前选集"}
          </span>
          <span className="video-part-title">{part || "无选集标题"}</span>
        </div>

        <div className="video-content-actions" aria-label="内容操作">
          <button
            className="nav-icon-btn"
            disabled={!bvid}
            title={isLiked ? "取消点赞" : "点赞"}
            onClick={handleLike}
          >
            <ThumbsUp fill={isLiked ? "#e11d48" : gray} size={20} theme="outline" />
          </button>
          <button
            className="nav-icon-btn"
            disabled={!bvid}
            title={coinCount >= 2 ? "已投币" : "投币(2个)"}
            onClick={handleCoin}
          >
            <HandleB fill={coinCount >= 2 ? "#ca8a04" : gray} size={20} theme="outline" />
          </button>
          <button className="nav-icon-btn" title="浏览器打开" onClick={onShareClick}>
            <Browser fill={gray} size={18} theme="outline" />
          </button>
          <button className="nav-icon-btn" title="视频播放" onClick={onPlayVideoClick}>
            <VideoTwo fill={gray} size={18} theme="outline" />
          </button>
          <button
            className="nav-icon-btn"
            disabled={!cid}
            title="弹幕/评论"
            onClick={onDanmakuClick}
          >
            <Comment fill={gray} size={18} theme="outline" />
          </button>
        </div>

        <div className="video-context-dock" aria-label="播放上下文">
          <button
            className="nav-icon-btn"
            disabled={!searchResultsCount}
            title="搜索结果"
            onClick={onSearchClick}
          >
            <Search fill={gray} size={20} theme="outline" />
          </button>
          <button
            className="nav-icon-btn"
            disabled={!bvid}
            title="选集"
            onClick={onPageListClick}
          >
            <DoubleUp
              fill={!isPlaylistMode && bvid ? blue : gray}
              size={20}
              theme="outline"
            />
          </button>
          <button
            className="nav-icon-btn"
            disabled={!currentSeriesTitle}
            title="合集"
            onClick={onSeriesClick}
          >
            <Layers fill={gray} size={20} theme="outline" />
          </button>
          <button
            className="nav-icon-btn"
            title="播放列表"
            onClick={onPlaylistClick}
          >
            <MusicList
              fill={isPlaylistMode ? blue : gray}
              size={20}
              theme="outline"
            />
            {playlistCount > 0 && (
              <span className="nav-badge">
                {playlistCount > 99 ? "99+" : playlistCount}
              </span>
            )}
          </button>
        </div>
      </div>
    </div>
  );
}
