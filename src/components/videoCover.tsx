import { useEffect, useState } from "react";
import { Halo, Refresh } from "@icon-park/react";
import { invoke } from "@tauri-apps/api/core";

// 内联 flat Lucide SVG (24x24, stroke-width=2), 与现有开关按钮图标风格一致。
function LightfieldIcon() {
  return (
    <svg
      width="14"
      height="14"
      viewBox="0 0 24 24"
      fill="none"
      stroke="currentColor"
      strokeWidth="2"
      strokeLinecap="round"
      strokeLinejoin="round"
      aria-hidden="true"
    >
      <path d="M9.937 15.5A2 2 0 0 0 8.5 14.063l-6.135-1.582a.5.5 0 0 1 0-.962L8.5 9.936A2 2 0 0 0 9.937 8.5l1.582-6.135a.5.5 0 0 1 .963 0L14.063 8.5A2 2 0 0 0 15.5 9.937l6.135 1.581a.5.5 0 0 1 0 .964L15.5 14.063a2 2 0 0 0-1.437 1.437l-1.582 6.135a.5.5 0 0 1-.963 0z" />
      <path d="M20 3v4" />
      <path d="M22 5h-4" />
      <path d="M4 17v2" />
      <path d="M5 18H3" />
    </svg>
  );
}

interface VideoCoverProps {
  cover?: string;
  isPlaying?: boolean;
  onPlayStateChange?: (isPlaying: boolean) => void;
  ambientBackgroundEnabled?: boolean;
  onAmbientBackgroundToggle?: () => void;
  lightfieldSimple?: boolean;
  onLightfieldSimpleToggle?: () => void;
}

// 视频封面：碟片模式（旋转，默认）/ 封面模式（静态方块，省 GPU）
export default function VideoCover({
  cover,
  isPlaying = false,
  onPlayStateChange,
  ambientBackgroundEnabled = true,
  onAmbientBackgroundToggle,
  lightfieldSimple = false,
  onLightfieldSimpleToggle,
}: VideoCoverProps) {
  const coverImage = cover || "/logo.png";
  // 封面「先预载、后换源」：新封面图加载完成前继续显示旧封面，
  // 与背景光场的换源节奏一致，切换歌曲时不会闪空白。
  const [shownCover, setShownCover] = useState(coverImage);
  useEffect(() => {
    if (!cover || cover === shownCover) return;
    let cancelled = false;
    const img = new Image();
    img.onload = () => {
      if (!cancelled) setShownCover(coverImage);
    };
    img.src = coverImage;
    return () => {
      cancelled = true;
    };
  }, [cover, coverImage, shownCover]);
  const [coverMode, setCoverMode] = useState<"disc" | "square">(() => {
    const saved = localStorage.getItem("coverMode");
    return saved === "square" ? "square" : "disc";
  });

  // 切换模式并持久化（localStorage 为主，dkv 同步备份）
  const toggleCoverMode = () => {
    const newMode = coverMode === "disc" ? "square" : "disc";
    setCoverMode(newMode);
    localStorage.setItem("coverMode", newMode);
    invoke("set_kv", { key: "coverMode", value: newMode }).catch(() => {});
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
              backgroundImage: `url(${shownCover})`,
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
        <button
          className={`cover-mode-toggle cover-background-toggle${ambientBackgroundEnabled ? " is-active" : ""}`}
          onClick={(e) => {
            e.stopPropagation();
            onAmbientBackgroundToggle?.();
          }}
          title={ambientBackgroundEnabled ? "关闭封面背景" : "开启封面背景"}
          aria-label={ambientBackgroundEnabled ? "关闭封面背景" : "开启封面背景"}
          aria-pressed={ambientBackgroundEnabled}
        >
          <Halo size="14" theme="outline" />
        </button>
        <button
          className={`cover-mode-toggle cover-lightfield-toggle${lightfieldSimple ? " is-active" : ""}`}
          onClick={(e) => {
            e.stopPropagation();
            onLightfieldSimpleToggle?.();
          }}
          title={lightfieldSimple ? "关闭光场简化" : "开启光场简化"}
          aria-label={lightfieldSimple ? "关闭光场简化" : "开启光场简化"}
          aria-pressed={lightfieldSimple}
        >
          <LightfieldIcon />
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
            backgroundImage: `url(${shownCover})`,
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
        <button
          className={`cover-mode-toggle cover-background-toggle${ambientBackgroundEnabled ? " is-active" : ""}`}
          onClick={(e) => {
            e.stopPropagation();
            onAmbientBackgroundToggle?.();
          }}
          title={ambientBackgroundEnabled ? "关闭封面背景" : "开启封面背景"}
          aria-label={ambientBackgroundEnabled ? "关闭封面背景" : "开启封面背景"}
          aria-pressed={ambientBackgroundEnabled}
        >
          <Halo size="14" theme="outline" />
        </button>
        <button
          className={`cover-mode-toggle cover-lightfield-toggle${lightfieldSimple ? " is-active" : ""}`}
          onClick={(e) => {
            e.stopPropagation();
            onLightfieldSimpleToggle?.();
          }}
          title={lightfieldSimple ? "关闭光场简化" : "开启光场简化"}
          aria-label={lightfieldSimple ? "关闭光场简化" : "开启光场简化"}
          aria-pressed={lightfieldSimple}
        >
          <LightfieldIcon />
        </button>
      </div>
  );
}
