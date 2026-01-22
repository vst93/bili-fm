import { AudioPlayer, AudioPlayerRef } from "react-audio-play";
import { useEffect, useRef, useCallback, useState, useImperativeHandle, forwardRef } from "react";
import { ReportPlayProgress } from "../../wailsjs/go/service/BL";

interface PlayerProps {
  src?: string;
  onEnded?: () => void;
  onPlayStateChange?: (isPlaying: boolean) => void;
  onTimeUpdate?: (time: number) => void;
  isPlaying?: boolean;
  aid?: number;
  cid?: number;
}

interface PlayerRef {
  getCurrentTime: () => number;
}

const Player = forwardRef<PlayerRef, PlayerProps>(function Player({
  src,
  onEnded,
  onPlayStateChange,
  onTimeUpdate,
  isPlaying,
  aid,
  cid,
}: PlayerProps, ref) {
  let autoPlay = true;
  const playerRef = useRef<AudioPlayerRef>(null);
  const audioRef = useRef<HTMLAudioElement | null>(null);
  const timeUpdateInterval = useRef<number>();
  const [currentTime, setCurrentTime] = useState(0);

  // 暴露方法给父组件
  useImperativeHandle(ref, () => ({
    getCurrentTime: () => currentTime,
  }), [currentTime]);

  // 尝试获取内部 audio 元素
  useEffect(() => {
    const container = document.getElementById("player");
    if (container) {
      const audioEl = container.querySelector("audio");
      if (audioEl) {
        audioRef.current = audioEl;
      }
    }
  }, [src]);

  const startTimeUpdate = useCallback(() => {
    if (timeUpdateInterval.current) {
      clearInterval(timeUpdateInterval.current);
    }
    timeUpdateInterval.current = window.setInterval(() => {
      let time = 0;
      
      // 方法1: 尝试从 audio 元素获取
      if (audioRef.current) {
        time = Math.floor(audioRef.current.currentTime);
      }
      
      // 方法2: 如果有播放，从 playerRef 尝试
      if (time === 0 && playerRef.current) {
        const player = playerRef.current as unknown as { currentTime?: number };
        if (player.currentTime !== undefined) {
          time = Math.floor(player.currentTime);
        }
      }
      
      // 方法3: 使用内部状态累加
      if (time === 0 && isPlaying) {
        time = currentTime + 0.5;
      }
      
      if (time > 0 && time !== currentTime) {
        setCurrentTime(time);
        onTimeUpdate?.(time);
      }
    }, 500);
  }, [isPlaying, onTimeUpdate, currentTime]);

  const stopTimeUpdate = useCallback(() => {
    if (timeUpdateInterval.current) {
      clearInterval(timeUpdateInterval.current);
      timeUpdateInterval.current = undefined;
    }
  }, []);

  useEffect(() => {
    if (isPlaying) {
      startTimeUpdate();
    } else {
      stopTimeUpdate();
    }
    return () => stopTimeUpdate();
  }, [isPlaying, startTimeUpdate, stopTimeUpdate]);

  useEffect(() => {
    if (src && autoPlay) {
      onPlayStateChange?.(true);
    }
  }, [src]);

  useEffect(() => {
    if (isPlaying && aid && cid) {
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
    <div id="player" style={{}}>
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
});

export default Player;
