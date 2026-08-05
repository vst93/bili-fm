import { useEffect, useRef, useState } from "react";
import { Close } from "@icon-park/react";

interface PlayerVideoProps {
  src?: string;
  isPlay?: boolean;
  isPlayVideoStop?: boolean;
  setIsplay: (isPlay: boolean) => void;
  setIsPlayVideoStop: (v: boolean) => void;
}

export default function PlayerVideo({
  src,
  isPlay,
  isPlayVideoStop,
  setIsplay,
  setIsPlayVideoStop,
}: PlayerVideoProps) {
  const containerRef = useRef<HTMLDivElement>(null);
  const videoRef = useRef<HTMLVideoElement>(null);
  const [visible, setVisible] = useState(false);

  // 控制 mount/unmount + 过渡动画
  useEffect(() => {
    if (isPlay) {
      setVisible(true);
      requestAnimationFrame(() => {
        requestAnimationFrame(() => {
          containerRef.current?.classList.add("player-video-open");
        });
      });
    } else if (visible) {
      containerRef.current?.classList.remove("player-video-open");
      const timer = setTimeout(() => setVisible(false), 300);
      return () => clearTimeout(timer);
    }
  }, [isPlay]); // eslint-disable-line react-hooks/exhaustive-deps

  // 控制 video 播放/暂停
  useEffect(() => {
    const video = videoRef.current;
    if (!video) return;
    if (isPlay && !isPlayVideoStop) {
      video.play().catch(() => {});
    } else {
      video.pause();
    }
  }, [isPlay, isPlayVideoStop]);

  // 组件卸载时确保视频停止
  useEffect(() => {
    return () => {
      const video = videoRef.current;
      if (video) {
        video.pause();
        video.src = "";
      }
    };
  }, []);

  if (!src || !visible) return null;

  if (isPlay === undefined) isPlay = false;

  const closePlayerVideo = () => {
    // 关闭视频浮窗：不要在此恢复音频播放（isPlaying 保持 false），
    // 由用户手动点击播放键继续收听，避免 macOS 上音频与视频抢播。
    setIsplay(false);
  };

  return (
    <div
      id="player_video"
      ref={containerRef}
      onClick={closePlayerVideo}
    >
      <div
        className="player-video-panel"
        onClick={(e) => e.stopPropagation()}
      >
        <video
          ref={videoRef}
          src={src}
          controls
          autoPlay={isPlay && !isPlayVideoStop}
          className="player-video-element"
          onPause={() => setIsPlayVideoStop(true)}
          onPlay={() => setIsPlayVideoStop(false)}
        />
        <button
          className="player-video-close"
          onClick={closePlayerVideo}
          title="关闭视频"
        >
          <Close theme="outline" size="20" fill="#f1f5f9" />
        </button>
      </div>
    </div>
  );
}
