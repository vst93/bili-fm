interface VideoCoverProps {
  cover?: string;
  isPlaying?: boolean;
  onPlayStateChange?: (isPlaying: boolean) => void;
}

//视频封面
export default function VideoCover({
  cover,
  isPlaying = false,
  onPlayStateChange,
}: VideoCoverProps) {
  const coverImage = cover || "/logo.png";

  const handleClick = () => {
    onPlayStateChange?.(!isPlaying);
  };

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
        <span className="cover-center" />
      </div>
    </div>
  );
}
