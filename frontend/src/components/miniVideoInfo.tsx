
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
  part = "",
  cover = "",
  onSwitchMode,
}: MiniVideoInfoProps) {
  const coverImage = cover || "/logo.png";
  return (
    <div id="min-video-info" className="relative flex items-center gap-3">
      <div
        id="min-video-info-cover"
        className="relative"
        style={{
          backgroundImage: `url(${coverImage})`,
          backgroundSize: "cover",
          backgroundPosition: "center",
        }}
      >
        <div className="absolute inset-0 bg-gradient-to-br from-black/5 to-transparent" />
      </div>
      <div id="min-video-info-content" className="flex items-center gap-2">
        <span className="w-2 h-2 rounded-full bg-blue-500 animate-pulse" />
        <span className="text-sm text-gray-700 truncate">{part || "无选集标题"}</span>
      </div>
      {onSwitchMode && (
        <button
          id="switch-window-mode-mini"
          // className="absolute top-1 right-1 p-1 rounded-full bg-white/80 hover:bg-white shadow-md transition-colors"
          title="切换到窗口模式"
          onClick={onSwitchMode}
        >
          <ZoomInternal theme="outline" size={18} fill="#333" />
        </button>
      )}
    </div>
  );
}
