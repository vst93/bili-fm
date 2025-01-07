import { Button } from "@nextui-org/react";

export default function VideoInfo() {
  return (
    <div id="video-info">
      <div id="video-owner">
        <img id="video-owner-face" src="" alt="" />
        <span className="cursor-pointer" id="video-owner-name" />
      </div>
      <div className="can-seelect" id="video-title">
        暂无播放内容
      </div>
      <div className="can-seelect" id="video-desc">
        &nbsp;
      </div>
      <div className="can-seelect" id="video-part">
        &nbsp;
      </div>

      <div className="info-tools">
        <Button className="info-tools-button" title="搜索">
          搜索
        </Button>
        <Button className="info-tools-button" title="选集">
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
