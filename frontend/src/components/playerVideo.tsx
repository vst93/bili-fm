import ReactPlayer from "react-player";

interface PlayerVideoProps {
  src?: string;
  isPlay?: boolean;
  setIsplay : (isPlay: boolean) => void;
}


export default function PlayerVideo({
  src, 
  isPlay,
  setIsplay,
}: PlayerVideoProps) {

  const closePlayerVideo = () => {
    setIsplay(false);
  }

  if (!src) { 
    return false;
  }
  

  if (isPlay === undefined) { 
    isPlay = false;
  }

  return (
    <div
      id="player_video"
      style={{
        // display: "none",
        position: "fixed",
        top: "0",
        left: "0",
        width: "100%",
        height: "100%",
        backgroundColor: "rgba(0,0,0,0.8)",
        zIndex: 998,
      }}
      hidden={!isPlay}
    >
      <ReactPlayer
        url={src || ""}
        controls={true}
        width="100%"
        height="100%"
        // playing={true}
        // muted={true}
      />
      <>
        <div
          onClick={closePlayerVideo}
          style={{
            position: "absolute",
            top: "10px",
            right: "10px",
            fontSize: "20px",
            cursor: "pointer",
            color: "white",
            backgroundColor: "rgba(0,0,0,0.5)",
            padding: "5px 10px",
            borderRadius: "5px",
            zIndex: 999,
          }}
        >
          关闭
        </div>
      </>
    </div>
  );
}
