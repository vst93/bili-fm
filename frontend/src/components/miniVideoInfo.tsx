
interface MiniVideoInfoProps {
  title?: string;
  desc?: string;
  ownerName?: string;
  ownerFace?: string;
  ownerMid?: number;
  part?: string;
  bvid?: string;
  aid?: number;
  cover?: string;
}

export default function MiniVideoInfo({
  part = "",
  cover = "",
}: MiniVideoInfoProps) {
  const coverImage = cover || "/logo.png";
  return (
    <div id="min-video-info">
      <div
        id="min-video-info-cover"
        style={{
          backgroundImage: `url(${coverImage})`,
          backgroundSize: "cover",
          backgroundPosition: "center",
        }}
      />
      <div id="min-video-info-content">
        {part || "无选集标题"}
      </div>
    </div>
  );
}
