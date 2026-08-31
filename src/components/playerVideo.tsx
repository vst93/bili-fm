import { useEffect, useRef, useState } from "react";
import { Close, PauseOne, Pic, Play } from "@icon-park/react";
import { invoke } from "@tauri-apps/api/core";

const PLAY_PROGRESS_REPORT_INTERVAL_MS = 30_000;
const PLAY_PROGRESS_CRITICAL_INTERVAL_MS = 3_000;

interface PlayerVideoProps {
  src?: string;
  isPlay?: boolean;
  isPlayVideoStop?: boolean;
  initialTime?: number;
  aid?: number;
  cid?: number;
  cloudHistoryEnabled?: boolean;
  setIsplay: (isPlay: boolean) => void;
  setIsPlayVideoStop: (v: boolean) => void;
}

const formatTime = (seconds: number) => {
  if (!Number.isFinite(seconds) || seconds < 0) return "0:00";
  const total = Math.floor(seconds);
  const h = Math.floor(total / 3600);
  const m = Math.floor((total % 3600) / 60);
  const s = total % 60;
  const mm = h > 0 ? String(m).padStart(2, "0") : String(m);
  const ss = String(s).padStart(2, "0");
  return h > 0 ? `${h}:${mm}:${ss}` : `${mm}:${ss}`;
};

export default function PlayerVideo({
  src,
  isPlay,
  isPlayVideoStop,
  initialTime = 0,
  aid,
  cid,
  cloudHistoryEnabled = true,
  setIsplay,
  setIsPlayVideoStop,
}: PlayerVideoProps) {
  const containerRef = useRef<HTMLDivElement>(null);
  const videoRef = useRef<HTMLVideoElement>(null);
  const initialTimeRef = useRef(initialTime);
  const isInitialSeekPendingRef = useRef(true);
  const cloudHistoryEnabledRef = useRef(cloudHistoryEnabled);
  const lastCloudReportRef = useRef({ mediaKey: "", progress: -1, at: 0 });
  const [visible, setVisible] = useState(false);

  // 自定义控件状态
  const [isPlaying, setIsPlaying] = useState(false);
  const [currentTime, setCurrentTime] = useState(0);
  const [duration, setDuration] = useState(0);
  const [isInPiP, setIsInPiP] = useState(false);
  const [isScrubbing, setIsScrubbing] = useState(false);
  const [scrubTime, setScrubTime] = useState(0);

  cloudHistoryEnabledRef.current = cloudHistoryEnabled;
  const mediaKey = src && aid && cid ? `${src}:${aid}:${cid}` : "";

  const reportCloudProgress = (
    progress: number,
    force = false,
    allowZero = false,
  ) => {
    if (!cloudHistoryEnabledRef.current || !aid || !cid || !mediaKey) return;

    const normalizedProgress = progress < 0 ? -1 : Math.max(0, Math.floor(progress));
    if (normalizedProgress === 0 && !allowZero) return;
    const now = Date.now();
    const lastReport = lastCloudReportRef.current;
    const progressDelta = Math.abs(lastReport.progress - normalizedProgress);
    if (
      lastReport.mediaKey === mediaKey &&
      lastReport.progress === normalizedProgress
    ) return;
    if (
      force &&
      normalizedProgress >= 0 &&
      lastReport.mediaKey === mediaKey &&
      !allowZero &&
      progressDelta < 5 &&
      now - lastReport.at < PLAY_PROGRESS_CRITICAL_INTERVAL_MS
    ) return;
    if (
      !force &&
      lastReport.mediaKey === mediaKey &&
      now - lastReport.at < PLAY_PROGRESS_REPORT_INTERVAL_MS
    ) return;

    lastCloudReportRef.current = {
      mediaKey,
      progress: normalizedProgress,
      at: now,
    };
    void invoke("report_play_progress", {
      aid,
      cid,
      progress: normalizedProgress,
    }).catch(() => {});
  };

  const syncInitialTime = (video: HTMLVideoElement) => {
    if (video.readyState < HTMLMediaElement.HAVE_METADATA) return false;

    const requestedTime = initialTimeRef.current;

    if (!Number.isFinite(requestedTime) || requestedTime <= 0) return true;

    const targetTime = Number.isFinite(video.duration)
      ? Math.min(requestedTime, video.duration)
      : requestedTime;

    // WebKit rejects seeking before metadata is available. Chromium accepts it,
    // but applying it at the same lifecycle point keeps platform behavior aligned.
    try {
      video.currentTime = Math.max(0, targetTime);

      return true;
    } catch {
      return false;
    }
  };

  // 控制 mount/unmount + 过渡动画
  useEffect(() => {
    if (isPlay) {
      initialTimeRef.current = initialTime;
      isInitialSeekPendingRef.current = true;
      lastCloudReportRef.current = {
        mediaKey,
        progress: Math.max(0, Math.floor(initialTime)),
        at: Date.now(),
      };
      setVisible(true);
      requestAnimationFrame(() => {
        requestAnimationFrame(() => {
          containerRef.current?.classList.add("player-video-open");
        });
      });
    } else if (visible) {
      containerRef.current?.classList.remove("player-video-open");
      const timer = setTimeout(() => setVisible(false), 300);
      return () => clearTimeout(timer);
    }
  }, [initialTime, isPlay]); // eslint-disable-line react-hooks/exhaustive-deps

  // 控制 video 播放/暂停
  useEffect(() => {
    const video = videoRef.current;

    if (!video) return;
    if (isPlay && !isPlayVideoStop) {
      const didSync =
        !isInitialSeekPendingRef.current || syncInitialTime(video);

      if (didSync && video.readyState >= HTMLMediaElement.HAVE_FUTURE_DATA) {
        isInitialSeekPendingRef.current = false;
      }
      if (didSync && video.paused) {
        video.play().catch(() => {});
      }
    } else {
      video.pause();
    }
  }, [isPlay, isPlayVideoStop]);

  // 监听画中画进入/离开事件（React 类型未暴露对应 props，手动挂监听）
  useEffect(() => {
    const video = videoRef.current;

    if (!video) return;

    const handleEnterPiP = () => setIsInPiP(true);
    const handleLeavePiP = () => setIsInPiP(false);

    video.addEventListener("enterpictureinpicture", handleEnterPiP);
    video.addEventListener("leavepictureinpicture", handleLeavePiP);

    return () => {
      video.removeEventListener("enterpictureinpicture", handleEnterPiP);
      video.removeEventListener("leavepictureinpicture", handleLeavePiP);
    };
  }, [visible]);

  // 切换视频源时退出画中画，避免浮窗继续展示旧内容
  useEffect(() => {
    if (src && document.pictureInPictureElement) {
      void document.exitPictureInPicture().catch(() => {});
    }
  }, [src]);

  // 组件卸载时确保视频停止
  useEffect(() => {
    return () => {
      const video = videoRef.current;
      if (video) {
        if (document.pictureInPictureElement === video) {
          void document.exitPictureInPicture().catch(() => {});
        }
        video.pause();
        video.src = "";
      }
    };
  }, []);

  if (!src || !visible) return null;

  if (isPlay === undefined) isPlay = false;

  const pipAvailable =
    typeof document !== "undefined" && document.pictureInPictureEnabled;
  const displayTime = isScrubbing ? scrubTime : currentTime;
  const safeDuration = Number.isFinite(duration) && duration > 0 ? duration : 0;
  const progressPercent = safeDuration
    ? Math.min(100, (displayTime / safeDuration) * 100)
    : 0;

  const closePlayerVideo = async () => {
    // 关闭视频浮窗：不要在此恢复音频播放（isPlaying 保持 false），
    // 由用户手动点击播放键继续收听，避免 macOS 上音频与视频抢播。
    const video = videoRef.current;
    if (video) reportCloudProgress(video.currentTime, true);
    // 先退出画中画，再关闭播放器，避免浮窗残留
    if (document.pictureInPictureElement) {
      try {
        await document.exitPictureInPicture();
      } catch {
        // ignore
      }
    }
    setIsplay(false);
  };

  const togglePlay = () => {
    const video = videoRef.current;
    if (!video) return;
    if (video.paused) {
      void video.play().catch(() => {});
    } else {
      video.pause();
    }
  };

  const handleSeekInput = (value: number) => {
    const video = videoRef.current;
    if (!video || !safeDuration) return;
    const newTime = Math.min(Math.max(0, value), safeDuration);
    setIsScrubbing(true);
    setScrubTime(newTime);
    try {
      video.currentTime = newTime;
    } catch {
      // WebKit 偶发拒绝 seek，忽略即可
    }
  };

  const endScrubbing = () => setIsScrubbing(false);

  const togglePictureInPicture = async () => {
    const video = videoRef.current;
    if (!video || !pipAvailable) return;
    try {
      if (document.pictureInPictureElement) {
        await document.exitPictureInPicture();
      } else {
        await video.requestPictureInPicture();
      }
    } catch {
      // 部分平台（如 macOS WKWebView）可能拒绝进入画中画，静默失败
    }
  };

  return (
    <div id="player_video" ref={containerRef}>
      <div className="player-video-panel">
        {/* B 站视频流不提供可挂载到本地媒体元素的字幕轨道。 */}
        {/* eslint-disable-next-line jsx-a11y/media-has-caption */}
        <video
          ref={videoRef}
          src={src}
          className="player-video-element"
          onCanPlay={(event) => {
            const video = event.currentTarget;
            const targetTime = initialTimeRef.current;
            let didSync = true;

            // Some WebKit versions report metadata before the first seek sticks.
            if (
              isInitialSeekPendingRef.current &&
              targetTime > 0 &&
              Math.abs(video.currentTime - targetTime) > 1
            ) {
              didSync = syncInitialTime(video);
            }
            if (didSync) isInitialSeekPendingRef.current = false;
            if (didSync && isPlay && !isPlayVideoStop) {
              void video.play().catch(() => {});
            }
          }}
          onLoadedMetadata={(event) => {
            const video = event.currentTarget;

            setDuration(video.duration);
            syncInitialTime(video);
            if (isPlay && !isPlayVideoStop) void video.play().catch(() => {});
          }}
          onDurationChange={(event) => {
            setDuration(event.currentTarget.duration);
          }}
          onEnded={() => {
            setIsPlaying(false);
            reportCloudProgress(-1, true);
          }}
          onPause={(event) => {
            setIsPlaying(false);
            if (!event.currentTarget.ended) {
              reportCloudProgress(event.currentTarget.currentTime, true);
            }
            setIsPlayVideoStop(true);
          }}
          onPlay={() => {
            setIsPlaying(true);
            setIsPlayVideoStop(false);
          }}
          onSeeked={(event) => {
            if (!isInitialSeekPendingRef.current) {
              reportCloudProgress(
                event.currentTarget.currentTime,
                true,
                event.currentTarget.currentTime <= 0.5,
              );
            }
          }}
          onTimeUpdate={(event) => {
            setCurrentTime(event.currentTarget.currentTime);
            reportCloudProgress(event.currentTarget.currentTime);
          }}
        />
        {/* 画中画模式下浮窗只读，隐藏完整控件条，仅保留退出入口 */}
        {isInPiP ? (
          <button
            className="player-video-pip-exit"
            onClick={togglePictureInPicture}
            title="退出画中画"
          >
            <Pic theme="outline" size="18" fill="#f1f5f9" />
            退出画中画
          </button>
        ) : (
          <div
            className="player-video-controls"
            style={
              { "--pvp-progress": `${progressPercent}%` } as React.CSSProperties
            }
          >
            <button
              className="player-video-ctrl-btn"
              onClick={togglePlay}
              title={isPlaying ? "暂停" : "播放"}
            >
              {isPlaying ? (
                <PauseOne theme="outline" size="18" fill="#f1f5f9" />
              ) : (
                <Play theme="outline" size="18" fill="#f1f5f9" />
              )}
            </button>
            <span className="player-video-time">
              {formatTime(displayTime)}
            </span>
            <div className="player-video-timeline-wrap">
              <span
                aria-hidden="true"
                className="player-video-timeline-track"
              />
              <input
                aria-label="播放进度"
                aria-valuetext={`${formatTime(displayTime)} / ${formatTime(safeDuration)}`}
                className="player-video-timeline"
                disabled={!safeDuration}
                max={safeDuration}
                min="0"
                step="0.1"
                type="range"
                value={displayTime}
                onBlur={endScrubbing}
                onInput={(event) =>
                  handleSeekInput(event.currentTarget.valueAsNumber)
                }
                onPointerUp={endScrubbing}
              />
            </div>
            <span className="player-video-time">
              {formatTime(safeDuration)}
            </span>
            {pipAvailable && (
              <button
                className="player-video-ctrl-btn"
                onClick={togglePictureInPicture}
                title="画中画"
              >
                <Pic theme="outline" size="18" fill="#f1f5f9" />
              </button>
            )}
          </div>
        )}
        <button
          className="player-video-close"
          onClick={closePlayerVideo}
          title="关闭视频"
        >
          <Close theme="outline" size="20" fill="#f1f5f9" />
        </button>
      </div>
    </div>
  );
}
