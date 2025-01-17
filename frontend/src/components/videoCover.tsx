import { useEffect, useState } from "react";

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
  const [rotation, setRotation] = useState(0);
  const coverImage = cover || "/logo.png";

  useEffect(() => {
    let animationFrame: number;

    const animate = () => {
      if (isPlaying) {
        setRotation((prev) => (prev + 0.2) % 360);
        animationFrame = requestAnimationFrame(animate);
      }
    };

    if (isPlaying) {
      animationFrame = requestAnimationFrame(animate);
    }

    return () => {
      if (animationFrame) {
        cancelAnimationFrame(animationFrame);
      }
    };
  }, [isPlaying]);

  const handleClick = () => {
    onPlayStateChange?.(!isPlaying);
  };

  return (
    <div
      id="video-cover"
      role="button"
      style={{
        backgroundImage: `url(${coverImage})`,
        backgroundSize: "cover",
        backgroundPosition: "center",
        transform: `rotate(${rotation}deg)`,
        transition: isPlaying ? "none" : "transform 0.3s ease-out",
        cursor: "pointer",
      }}
      tabIndex={0}
      onClick={handleClick}
      onKeyDown={(e) => {
        if (e.key === "Enter" || e.key === " ") {
          handleClick();
        }
      }}
    />
  );
}
