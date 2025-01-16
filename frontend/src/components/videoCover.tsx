import { useEffect, useState } from "react";

interface VideoCoverProps {
  cover?: string;
  isPlaying?: boolean;
}

//视频封面
export default function VideoCover({
  cover,
  isPlaying = false,
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

  return (
    <div
      id="video-cover"
      style={{
        backgroundImage: `url(${coverImage})`,
        backgroundSize: "cover",
        backgroundPosition: "center",
        transform: `rotate(${rotation}deg)`,
        transition: isPlaying ? "none" : "transform 0.3s ease-out",
      }}
    />
  );
}
