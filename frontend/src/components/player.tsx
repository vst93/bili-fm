import { AudioPlayer, AudioPlayerRef } from "react-audio-play";
import { useEffect, useRef } from "react";
import { ReportPlayProgress } from "../../wailsjs/go/service/BL";

interface PlayerProps {
  src?: string;
  onEnded?: () => void;
  onPlayStateChange?: (isPlaying: boolean) => void;
  isPlaying?: boolean;
  aid?: number;
  cid?: number;
}

export default function Player({
  src,
  onEnded,
  onPlayStateChange,
  isPlaying,
  aid,
  cid,
}: PlayerProps) {
  let autoPlay = true;
  const playerRef = useRef<AudioPlayerRef>(null);

  useEffect(() => {
    if (src && autoPlay) {
      onPlayStateChange?.(true);
    }
  }, [src]);

  useEffect(() => {
    if (isPlaying && aid && cid) {
      // 开始播放时上报一次观看进度
      ReportPlayProgress(aid, cid, 0);
    }
  }, [isPlaying, aid, cid]);

  if (isPlaying) {
    playerRef.current?.play();
  } else {
    playerRef.current?.pause();
  }

  if (!src) {
    autoPlay = false;
  }

  return (
    <div
      id="player"
      style={{
      }}
    >
      <AudioPlayer
        ref={playerRef}
        autoPlay={autoPlay}
        hasKeyBindings={false}
        loop={false}
        sliderColor="#68bca4"
        src={src || ""}
        width="100%"
        onEnd={onEnded}
        onError={() => onPlayStateChange?.(false)}
        onPause={() => onPlayStateChange?.(false)}
        onPlay={() => onPlayStateChange?.(true)}
        className="bg-transparent"
      />
    </div>
  );
}
