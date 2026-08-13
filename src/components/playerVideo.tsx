import { useEffect, useRef, useState } from "react";
import { Close } from "@icon-park/react";
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

  // 组件卸载时确保视频停止
  useEffect(() => {
    return () => {
      const video = videoRef.current;
      if (video) {
        video.pause();
        video.src = "";
      }
    };
  }, []);

  if (!src || !visible) return null;

  if (isPlay === undefined) isPlay = false;

  const closePlayerVideo = () => {
    // 关闭视频浮窗：不要在此恢复音频播放（isPlaying 保持 false），
    // 由用户手动点击播放键继续收听，避免 macOS 上音频与视频抢播。
    const video = videoRef.current;
    if (video) reportCloudProgress(video.currentTime, true);
    setIsplay(false);
  };

  return (
    <div id="player_video" ref={containerRef}>
      <div className="player-video-panel">
        {/* B 站视频流不提供可挂载到本地媒体元素的字幕轨道。 */}
        {/* eslint-disable-next-line jsx-a11y/media-has-caption */}
        <video
          ref={videoRef}
          src={src}
          controls
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

            syncInitialTime(video);
            if (isPlay && !isPlayVideoStop) void video.play().catch(() => {});
          }}
          onEnded={() => reportCloudProgress(-1, true)}
          onPause={(event) => {
            if (!event.currentTarget.ended) {
              reportCloudProgress(event.currentTarget.currentTime, true);
            }
            setIsPlayVideoStop(true);
          }}
          onPlay={() => setIsPlayVideoStop(false)}
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
            reportCloudProgress(event.currentTarget.currentTime);
          }}
        />
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
