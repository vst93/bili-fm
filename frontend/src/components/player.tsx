import { AudioPlayer, AudioPlayerRef } from "react-audio-play";
import { useEffect, useRef } from "react";

interface PlayerProps {
  src?: string;
  onEnded?: () => void;
  onPlayStateChange?: (isPlaying: boolean) => void;
  isPlaying?: boolean;
}

export default function Player({
  src,
  onEnded,
  onPlayStateChange,
  isPlaying,
}: PlayerProps) {
  let autoPlay = true;
  const playerRef = useRef<AudioPlayerRef>(null);

  useEffect(() => {
    if (src && autoPlay) {
      onPlayStateChange?.(true);
    }
  }, [src]);

  if (isPlaying) {
    playerRef.current?.play();
  } else {
    playerRef.current?.pause();
  }

  if (!src) {
    autoPlay = false;
  }

  return (
    <div id="player">
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
      />
    </div>
  );
}
