import { useEffect, useRef, useState } from "react";
import { Close } from "@icon-park/react";
import { invoke } from "@tauri-apps/api/core";

const PLAY_PROGRESS_REPORT_INTERVAL_MS = 30_000;
const PLAY_PROGRESS_CRITICAL_INTERVAL_MS = 3_000;

const SEEK_STEP_SECONDS = 10;

/** 老版本 WebKit 只有私有的 presentation mode API，没有标准 PiP API */
type WebKitVideoElement = HTMLVideoElement & {
  webkitPresentationMode?: "inline" | "picture-in-picture" | "fullscreen";
  webkitSetPresentationMode?: (mode: "inline" | "picture-in-picture") => void;
};

/**
 * 退出画中画。标准 API 优先，回落到 WebKit 私有 API。
 * 切换视频源或关闭播放器时必须调用，否则 macOS 的浮窗会残留并继续播放旧内容。
 */
const exitPictureInPicture = (video?: HTMLVideoElement | null) => {
  try {
    if (document.pictureInPictureElement) {
      if (!video || document.pictureInPictureElement === video) {
        void document.exitPictureInPicture().catch(() => {});
        return;
      }
    }
    const webkitVideo = video as WebKitVideoElement | null | undefined;
    if (webkitVideo?.webkitPresentationMode === "picture-in-picture") {
      webkitVideo.webkitSetPresentationMode?.("inline");
    }
  } catch {
    // 平台不支持或状态已变化，忽略
  }
};

interface PlayerVideoProps {
  src?: string;
  isPlay?: boolean;
  isPlayVideoStop?: boolean;
  initialTime?: number;
  aid?: number;
  cid?: number;
  cloudHistoryEnabled?: boolean;
  /**
   * 当前平台的系统画中画浮窗不提供进度条（macOS WebKit）。
   * 为 true 时隐藏原生控件条上的画中画按钮，避免用户点进去后发现调不了进度。
   */
  nativePipUnusable?: boolean;
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
  nativePipUnusable = false,
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

  /**
   * 把播放位置同步给系统媒体会话。
   * macOS 画中画浮窗和"正在播放"控制中心的进度条是靠这份 positionState 画出来的，
   * 不上报就只有播放/暂停按钮，进度条不出现或拖不动。
   */
  const publishPositionState = (video: HTMLVideoElement) => {
    if (!("mediaSession" in navigator) || !navigator.mediaSession.setPositionState) return;
    const duration = video.duration;

    // duration 为 NaN/Infinity（未加载完或直播流）时调用会抛 TypeError
    if (!Number.isFinite(duration) || duration <= 0) return;
    try {
      navigator.mediaSession.setPositionState({
        duration,
        playbackRate: video.playbackRate > 0 ? video.playbackRate : 1,
        position: Math.min(Math.max(0, video.currentTime), duration),
      });
    } catch {
      // 参数越界时静默忽略，不影响播放
    }
  };

  /**
   * 只注册 seek 相关的 handler。
   * play/pause/previoustrack/nexttrack 归音频播放器所有（见 index.tsx），
   * 这里不能碰 —— 那边卸载时是置 null 而不是恢复，覆盖了就再也回不来。
   * 原生控件条本身已经处理了播放/暂停，画中画浮窗的播放键也直接作用于 video 元素。
   */
  useEffect(() => {
    if (!visible || !src) return;
    if (!("mediaSession" in navigator)) return;

    const seekTo = (time: number) => {
      const video = videoRef.current;

      if (!video || !Number.isFinite(video.duration)) return;
      const target = Math.min(Math.max(0, time), video.duration);

      try {
        // fastSeek 专为拖动进度条设计：跳到最近的关键帧，避免连续 seek 互相取消
        if (typeof video.fastSeek === "function") {
          video.fastSeek(target);
        } else {
          video.currentTime = target;
        }
      } catch {
        // WebKit 偶发拒绝 seek，忽略
      }
      publishPositionState(video);
    };

    try {
      navigator.mediaSession.setActionHandler("seekto", (details) => {
        if (typeof details.seekTime === "number") seekTo(details.seekTime);
      });
      navigator.mediaSession.setActionHandler("seekbackward", (details) => {
        const video = videoRef.current;

        if (!video) return;
        seekTo(video.currentTime - (details.seekOffset || SEEK_STEP_SECONDS));
      });
      navigator.mediaSession.setActionHandler("seekforward", (details) => {
        const video = videoRef.current;

        if (!video) return;
        seekTo(video.currentTime + (details.seekOffset || SEEK_STEP_SECONDS));
      });
    } catch {
      // 平台不支持这些 action，忽略
    }

    return () => {
      try {
        navigator.mediaSession.setActionHandler("seekto", null);
        navigator.mediaSession.setActionHandler("seekbackward", null);
        navigator.mediaSession.setActionHandler("seekforward", null);
        // 无参调用即清空，否则关闭视频后音频的"正在播放"会残留视频的时长
        navigator.mediaSession.setPositionState?.();
      } catch {
        // ignore
      }
    };
  }, [visible, src]);

  /**
   * 收回画中画浮窗。覆盖三种情况：切换视频源、关闭播放器（visible 转 false 时
   * <video> 会从 DOM 移除，但组件本身仍挂载，所以不能只靠卸载清理）、组件卸载。
   * 漏掉任何一种，macOS 都会留下一个继续播放旧内容的孤立浮窗。
   */
  useEffect(() => {
    return () => exitPictureInPicture(videoRef.current);
  }, [visible, src]);

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

  // 组件卸载时确保视频停止，并收回画中画浮窗
  useEffect(() => {
    return () => {
      const video = videoRef.current;
      if (video) {
        exitPictureInPicture(video);
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
    // 先收回浮窗，否则 macOS 会留下一个孤立的画中画窗口继续播放
    exitPictureInPicture(video);
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
          // 用系统原生控件条而不是自绘：各平台自动适配，进度条行为符合系统习惯。
          // 注意这解决不了系统画中画的进度问题 —— macOS WebKit 的 PiP 浮窗根本
          // 没有 scrubber（平台限制，与本页无关），要拖进度请用置顶小窗模式。
          controls
          // 防止 iOS/WebKit 把播放劫持进原生全屏
          playsInline
          // macOS 上系统画中画浮窗没有进度条，隐藏该按钮引导用户用置顶小窗
          disablePictureInPicture={nativePipUnusable}
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
            publishPositionState(video);
            if (isPlay && !isPlayVideoStop) void video.play().catch(() => {});
          }}
          onDurationChange={(event) => publishPositionState(event.currentTarget)}
          onRateChange={(event) => publishPositionState(event.currentTarget)}
          onEnded={() => reportCloudProgress(-1, true)}
          onPause={(event) => {
            if (!event.currentTarget.ended) {
              reportCloudProgress(event.currentTarget.currentTime, true);
            }
            setIsPlayVideoStop(true);
          }}
          onPlay={() => setIsPlayVideoStop(false)}
          onSeeked={(event) => {
            publishPositionState(event.currentTarget);
            if (!isInitialSeekPendingRef.current) {
              reportCloudProgress(
                event.currentTarget.currentTime,
                true,
                event.currentTarget.currentTime <= 0.5,
              );
            }
          }}
          onTimeUpdate={(event) => {
            publishPositionState(event.currentTarget);
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
