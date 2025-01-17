import { Button } from "@nextui-org/react";
import { Search, DoubleUp, Share } from "@icon-park/react";

interface VideoInfoProps {
  title?: string;
  desc?: string;
  ownerName?: string;
  ownerFace?: string;
  part?: string;
  bvid?: string;
  onSearchClick?: () => void;
  onPageListClick?: () => void;
  onShareClick?: () => void;
  onOwnerClick?: (name: string) => void;
}

export default function VideoInfo({
  title = "暂无播放内容",
  desc = "",
  ownerName = "",
  ownerFace = "",
  part = "",
  bvid = "",
  onSearchClick,
  onPageListClick,
  onShareClick,
  onOwnerClick,
}: VideoInfoProps) {
  console.log(bvid);

  return (
    <div id="video-info">
      <div className="flex items-center gap-2" id="video-owner">
        <img alt="" id="video-owner-face" src={ownerFace} />
        <button 
          className="cursor-pointer bg-transparent border-none p-0" 
          id="video-owner-name"
          onClick={() => onOwnerClick?.(ownerName)}
        >
          {ownerName}
        </button>
      </div>
      <div className="can-seelect" id="video-title">
        {title}
      </div>
      <div className="can-seelect" id="video-desc">
        {desc || "\u00A0"}
      </div>
      <div className="can-seelect" id="video-part">
        {part || "\u00A0"}
      </div>
      <div className="info-tools">
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
          title="浏览器打开"
          onClick={onShareClick}
        >
          <Share fill="#333" size="24" theme="outline" />
        </Button>
        <div className="info-tools-bl-user-group">
          <Button
            className="info-tools-button bl-feed"
            size="sm"
            title="B站账号关注UP视频动态列表"
          >
            动态
          </Button>
          <Button
            className="info-tools-button bl-rcmd"
            size="sm"
            title="B站账号推荐视频列表"
          >
            推荐
          </Button>
          <Button
            className="info-tools-button bl-collect"
            size="sm"
            title="B站账号收藏列表"
          >
            收藏
          </Button>
        </div>
      </div>
    </div>
  );
}
