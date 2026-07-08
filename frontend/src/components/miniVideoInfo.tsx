
import { ZoomInternal } from "@icon-park/react";

interface MiniVideoInfoProps {
  title?: string;
  desc?: string;
  ownerName?: string;
  ownerFace?: string;
  ownerMid?: number;
  part?: string;
  bvid?: string;
  aid?: number;
  cover?: string;
  onSwitchMode?: () => void;
}

export default function MiniVideoInfo({
  title = "暂无播放内容",
  part = "",
  cover = "",
  onSwitchMode,
}: MiniVideoInfoProps) {
  const coverImage = cover || "/logo.png";

  return (
    <div id="min-video-info">
      <div
        id="min-video-info-cover"
        style={{ backgroundImage: `url(${coverImage})` }}
      >
        <div className="mini-cover-shine" />
      </div>
      <div id="min-video-info-content">
        <div className="mini-title">{title || "暂无播放内容"}</div>
        <div className="mini-part">
          <span className="mini-status-dot" />
          <span>{part || "无选集标题"}</span>
        </div>
      </div>
      {onSwitchMode && (
        <button
          id="switch-window-mode-mini"
          title="切换到窗口模式"
          onClick={onSwitchMode}
        >
          <ZoomInternal theme="outline" size={18} fill="currentColor" />
        </button>
      )}
    </div>
  );
}
