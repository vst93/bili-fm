import { AudioPlayer } from "react-audio-play";

interface PlayerProps {
  src?: string;
  onEnded?: () => void;
  onPlayStateChange?: (isPlaying: boolean) => void;
}

export default function Player({
  src,
  onEnded,
  onPlayStateChange,
}: PlayerProps) {
  let autoPlay = true;

  if (!src) {
    autoPlay = false;
  } else {
    onPlayStateChange?.(true);
  }

  return (
    <div id="player">
      <AudioPlayer
        autoPlay={autoPlay}
        hasKeyBindings={false}
        loop={false}
        sliderColor="#68bca4"
        src={src || ""}
        width="100%"
        onEnd={onEnded}
        onPause={() => onPlayStateChange?.(false)}
        onPlay={() => onPlayStateChange?.(true)}
      />
    </div>
  );
}
