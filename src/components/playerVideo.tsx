import { useEffect, useRef, useState } from "react";
import _ReactPlayer from "react-player";
const ReactPlayer = _ReactPlayer as any;
import { Close } from "@icon-park/react";

interface PlayerVideoProps {
  src?: string;
  isPlay?: boolean;
  isPlayVideoStop?: boolean;
  setIsplay: (isPlay: boolean) => void;
}

export default function PlayerVideo({
  src,
  isPlay,
  isPlayVideoStop,
  setIsplay,
}: PlayerVideoProps) {
  const containerRef = useRef<HTMLDivElement>(null);
  const [visible, setVisible] = useState(false);

  // 控制 mount/unmount + 过渡动画
  useEffect(() => {
    if (isPlay) {
      setVisible(true);
      // 下一帧触发 transition
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

  if (!src || !visible) return null;

  if (isPlay === undefined) isPlay = false;

  const closePlayerVideo = () => {
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
        <ReactPlayer
          url={src}
          controls={true}
          width="100%"
          height="100%"
          playing={isPlay && !isPlayVideoStop}
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
