import { AudioPlayer } from "react-audio-play";

interface PlayerProps {
  src?: string;
}

export default function Player({ src }: PlayerProps) {
  let autoPlay = true;

  if (!src) {
    autoPlay = false;
  }

  return (
    <div id="player">
      <AudioPlayer
        autoPlay={autoPlay}
        hasKeyBindings={false}
        loop={true}
        sliderColor="#68bca4"
        src={src || ""}
        width="100%"
      />
    </div>
  );
}
