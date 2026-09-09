
import { Pushpin, ZoomInternal } from "@icon-park/react";

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
  isPlaylistMode?: boolean;
  onSwitchMode?: () => void;
  isPinned?: boolean;
  isWindowControlPending?: boolean;
  onTogglePin?: () => void;
}

export default function MiniVideoInfo({
  title = "暂无播放内容",
  part = "",
  cover = "",
  isPlaylistMode = false,
  onSwitchMode,
  isPinned = false,
  isWindowControlPending = false,
  onTogglePin,
}: MiniVideoInfoProps) {
  const coverImage = cover || "/logo.png";

  return (
    <div id="min-video-info" data-tauri-drag-region="deep">
      <div
        id="min-video-info-cover"
        style={{ backgroundImage: `url(${coverImage})` }}
      >
        <div className="mini-cover-shine" />
      </div>
      <div id="min-video-info-content">
        <div className="mini-title">{title || "暂无播放内容"}</div>
        <div className="mini-part">
          <span className={`mini-status-dot ${isPlaylistMode ? "mini-status-dot-playlist" : ""}`} />
          <span>{part || "无选集标题"}</span>
        </div>
      </div>
      <div className="mini-window-controls" role="group" aria-label="窗口控制">
        {onSwitchMode && (
          <button
            id="switch-window-mode-mini"
            aria-label="切换到窗口模式"
            className="app-title-bar-btn"
            disabled={isWindowControlPending}
            title="切换到窗口模式"
            type="button"
            onClick={onSwitchMode}
          >
            <ZoomInternal fill="currentColor" theme="outline" size={16} />
          </button>
        )}
        {onTogglePin && (
          <button
            id="toggle-mini-always-on-top"
            aria-label={isPinned ? "取消窗口置顶" : "窗口置顶"}
            aria-pressed={isPinned}
            className="app-title-bar-btn"
            disabled={isWindowControlPending}
            title={isPinned ? "取消窗口置顶" : "窗口置顶"}
            type="button"
            onClick={onTogglePin}
          >
            <Pushpin fill="currentColor" size={16} theme={isPinned ? "filled" : "outline"} />
          </button>
        )}
      </div>
    </div>
  );
}
