import { useState } from "react";
import { Refresh } from "@icon-park/react";

interface VideoCoverProps {
  cover?: string;
  isPlaying?: boolean;
  onPlayStateChange?: (isPlaying: boolean) => void;
}

// 视频封面：碟片模式（旋转，默认）/ 封面模式（静态方块，省 GPU）
export default function VideoCover({
  cover,
  isPlaying = false,
  onPlayStateChange,
}: VideoCoverProps) {
  const coverImage = cover || "/logo.png";
  const [coverMode, setCoverMode] = useState<"disc" | "square">(() => {
    const saved = localStorage.getItem("coverMode");
    return saved === "square" ? "square" : "disc";
  });

  // 切换模式并持久化（localStorage 在 WebView 中跨会话保留）
  const toggleCoverMode = () => {
    const newMode = coverMode === "disc" ? "square" : "disc";
    setCoverMode(newMode);
    localStorage.setItem("coverMode", newMode);
  };

  const handleClick = () => {
    onPlayStateChange?.(!isPlaying);
  };

  if (coverMode === "square") {
    return (
      <div className="cover-shell cover-shell-square">
        <div
          id="video-cover"
          className="cover-square"
          role="button"
          tabIndex={0}
          onClick={handleClick}
          onKeyDown={(e) => {
            if (e.key === "Enter" || e.key === " ") {
              handleClick();
            }
          }}
        >
          <div
            className="cover-art cover-art-square"
            style={{
              backgroundImage: `url(${coverImage})`,
            }}
          />
        </div>
        <button
          className="cover-mode-toggle"
          onClick={(e) => {
            e.stopPropagation();
            toggleCoverMode();
          }}
          title="切换为碟片模式"
        >
          <Refresh size="14" theme="outline" />
        </button>
      </div>
    );
  }

  return (
    <div className="cover-shell">
      <div
        id="video-cover"
        className={isPlaying ? "record-disc is-playing" : "record-disc"}
        role="button"
        tabIndex={0}
        onClick={handleClick}
        onKeyDown={(e) => {
          if (e.key === "Enter" || e.key === " ") {
            handleClick();
          }
        }}
      >
        <div
          className="cover-art"
          style={{
            backgroundImage: `url(${coverImage})`,
          }}
        />
      </div>
      <button
        className="cover-mode-toggle"
        onClick={(e) => {
          e.stopPropagation();
          toggleCoverMode();
        }}
        title="切换为封面模式"
      >
        <Refresh size="14" theme="outline" />
      </button>
    </div>
  );
}