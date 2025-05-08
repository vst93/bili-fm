import { Button, Image } from "@heroui/react";
import {
  Search,
  DoubleUp,
  Share,
  ShareSys,
  ChartRing,
  WeixinFavorites,
  ThumbsUp,
  HandleB,
  History,
  Layers,
} from "@icon-park/react";
import { useState, useEffect } from "react";

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
}: VideoInfoProps) {
  const [isLiked, setIsLiked] = useState(false);
  const [coinCount, setCoinCount] = useState(0);

  useEffect(() => {
    if (bvid) {
      checkLikeStatus();
      checkCoinStatus();
    } else {
      setIsLiked(false);
      setCoinCount(0);
    }
  }, [bvid]);

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
      <div className="flex items-center gap-2" id="video-owner">
        <Image
          alt={ownerName}
          className="cursor-pointer min-w-[48px]"
          classNames={{
            wrapper: "min-w-[48px]",
            img: "object-cover opacity-100",
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
          className="cursor-pointer bg-transparent border-none p-0"
          id="video-owner-name"
          onClick={() => onOwnerClick?.(ownerMid, ownerName)}
        >
          {ownerName || "神秘的up主"}
        </button>
      </div>
      <div className="can-seelect" id="video-title">
        {title || "无标题"}
        <Button
          style={{
            backgroundColor: "#e4e4e400",
            padding: "0",
          }}
          title="在浏览器打开"
          onClick={onShareClick}>
          <Share fill="#333" size="15" theme="outline" />
        </Button>
      </div>
      <div className="can-seelect" id="video-desc">
        {desc || "无描述"}
      </div>
      <div className="can-seelect" id="video-part">
        {part || "无选集标题"}
      </div>
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
          onClick={handleLike}
        >
          <ThumbsUp
            fill={isLiked ? "#00aeec" : "#333"}
            size="24"
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
          title={coinCount >= 2 ? "已投2币" : "投2币"}
          variant="light"
          onClick={handleCoin}
        >
          <HandleB
            fill={coinCount >= 2 ? "#00aeec" : "#333"}
            size="24"
            theme="outline"
          />
        </Button>
        <Button
          className="info-tools-button"
          size="sm"
          style={{
            backgroundColor: "#e4e4e485",
          }}
          title="搜索"
          onClick={onSearchClick}
        >
          <Search fill="#333" size="24" theme="outline" />
        </Button>
        <Button
          className="info-tools-button"
          size="sm"
          style={{
            backgroundColor: "#e4e4e485",
          }}
          title="选集"
          onClick={onPageListClick}
        >
          <DoubleUp fill="#333" size="24" theme="outline" />
        </Button>
        <Button
          className="info-tools-button"
          size="sm"
          style={{
            backgroundColor: "#e4e4e485",
          }}
          title="合集"
          onClick={onSeriesClick}
        >
          <Layers theme="outline" size="24" fill="#333" />
        </Button>
        <div className="info-tools-bl-user-group">
          <Button
            className="info-tools-button bl-feed"
            size="sm"
            title="B站账号关注UP视频动态列表"
            onClick={onFeedClick}
          >
            <ShareSys fill="#333" size="24" theme="outline" />
          </Button>
          <Button
            className="info-tools-button bl-rcmd"
            size="sm"
            title="B站账号推荐视频列表"
            onClick={onRecommendClick}
          >
            <ChartRing fill="#333" size="24" theme="outline" />
          </Button>
          <Button
            className="info-tools-button bl-collect"
            size="sm"
            title="B站账号收藏列表"
            onClick={onCollectClick}
          >
            <WeixinFavorites fill="#333" size="24" theme="outline" />
          </Button>
          <Button
            className="info-tools-button bl-collect"
            size="sm"
            title="B站账号历史记录"
            onClick={onHistoryClick}
          >
            <History theme="outline" size="24" fill="#333" />
          </Button>
        </div>
      </div>
    </div>
  );
}
