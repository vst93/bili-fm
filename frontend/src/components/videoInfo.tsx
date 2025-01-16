import { Button } from "@nextui-org/react";

interface VideoInfoProps {
  title?: string;
  desc?: string;
  ownerName?: string;
  ownerFace?: string;
  part?: string;
  onSearchClick?: () => void;
  onPageListClick?: () => void;
}

export default function VideoInfo({
  title = "暂无播放内容",
  desc = "",
  ownerName = "",
  ownerFace = "",
  part = "",
  onSearchClick,
  onPageListClick,
}: VideoInfoProps) {
  return (
    <div id="video-info">
      <div className="flex items-center gap-2" id="video-owner">
        <img alt="" id="video-owner-face" src={ownerFace} />
        <span className="cursor-pointer" id="video-owner-name">
          {ownerName}
        </span>
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
          title="搜索"
          onClick={onSearchClick}
        >
          搜索
        </Button>
        <Button
          className="info-tools-button"
          title="选集"
          onClick={onPageListClick}
        >
          选集
        </Button>
        <Button className="info-tools-button" title="浏览器打开">
          浏览器打开
        </Button>

        <div className="info-tools-bl-user-group">
          <Button
            className="info-tools-button bl-feed"
            title="B站账号关注UP视频动态列表"
          >
            动态
          </Button>
          <Button
            className="info-tools-button bl-rcmd"
            title="B站账号推荐视频列表"
          >
            推荐
          </Button>
          <Button
            className="info-tools-button bl-collect"
            title="B站账号收藏列表"
          >
            收藏
          </Button>
          <Button
            className="info-tools-button user-avatar"
            title="登录或重新登录"
          >
            头像
          </Button>
        </div>
      </div>
    </div>
  );
}
