import { useEffect, useRef, useState } from "react";
import type { CSSProperties } from "react";
import {
  Equalizer,
  Pause,
  PlayOne,
  VolumeMute,
  VolumeNotice,
} from "@icon-park/react";
import { invoke } from "@tauri-apps/api/core";

const PLAYBACK_RATES = [0.5, 0.75, 1, 1.25, 1.5, 2, 3] as const;
const SEEK_KEYS = new Set([
  "ArrowDown",
  "ArrowLeft",
  "ArrowRight",
  "ArrowUp",
  "End",
  "Home",
  "PageDown",
  "PageUp",
]);
const EQ_TRANSITION_SECONDS = 0.06;
const EQ_STORAGE_KEY = "loudnessEqEnabled";
const PLAY_PROGRESS_REPORT_INTERVAL_MS = 30_000;
const PLAY_PROGRESS_CRITICAL_INTERVAL_MS = 3_000;
const CLOUD_PROGRESS_STARTUP_BUDGET_MS = 800;

type AudioGraph = {
  ctx: AudioContext;
  source: MediaElementAudioSourceNode;
  compressor: DynamicsCompressorNode;
};

interface PlayerProps {
  src?: string;
  onEnded?: () => void;
  onPlayStateChange?: (isPlaying: boolean) => void;
  onTimeUpdate?: (time: number) => void;
  isPlaying?: boolean;
  aid?: number;
  cid?: number;
  forcePause?: boolean;
  cloudHistoryEnabled?: boolean;
}

const formatTime = (seconds: number) => {
  if (!Number.isFinite(seconds) || seconds < 0) return "0:00";
  const minutes = Math.floor(seconds / 60);
  const remainder = Math.floor(seconds % 60);

  return `${minutes}:${remainder.toString().padStart(2, "0")}`;
};

const updateSeekPreviewUi = (
  timeline: HTMLInputElement | null,
  timeLabel: HTMLTimeElement | null,
  value: number,
  duration: number,
) => {
  const boundedValue = Math.min(Math.max(value, 0), duration || 0);
  const progress = duration > 0 ? (boundedValue / duration) * 100 : 0;

  if (timeline) {
    timeline.value = String(boundedValue);
    timeline.parentElement?.style.setProperty(
      "--player-progress",
      `${progress}%`,
    );
    timeline.setAttribute(
      "aria-valuetext",
      `${formatTime(boundedValue)} / ${formatTime(duration)}`,
    );
  }
  if (timeLabel) {
    timeLabel.dateTime = `PT${Math.floor(boundedValue)}S`;
    timeLabel.textContent = formatTime(boundedValue);
  }
};

const Player = ({
  src,
  onEnded,
  onPlayStateChange,
  onTimeUpdate,
  isPlaying = false,
  aid,
  cid,
  forcePause = false,
  cloudHistoryEnabled = true,
}: PlayerProps) => {
  const audioRef = useRef<HTMLAudioElement>(null);
  const timelineRef = useRef<HTMLInputElement>(null);
  const currentTimeLabelRef = useRef<HTMLTimeElement>(null);
  const volumePopoverRef = useRef<HTMLDivElement>(null);
  const speedPopoverRef = useRef<HTMLDivElement>(null);
  const onTimeUpdateRef = useRef(onTimeUpdate);
  const audioGraphRef = useRef<AudioGraph | null>(null);
  const lastReportedSecondRef = useRef(-1);
  const lastVolumeRef = useRef(1);
  const isSeekingRef = useRef(false);
  const isKeyboardSeekingRef = useRef(false);
  const seekPointerIdRef = useRef<number | null>(null);
  const seekPreviewRef = useRef<number | null>(null);
  const currentTimeRef = useRef(0);
  const cloudHistoryEnabledRef = useRef(cloudHistoryEnabled);
  const cloudProgressRequestIdRef = useRef(0);
  const cloudProgressPendingRef = useRef(false);
  const cloudProgressReadyRef = useRef(false);
  const pendingCloudProgressRef = useRef<number | null>(null);
  const metadataMediaKeyRef = useRef("");
  const cloudSeekInFlightRef = useRef(false);
  const cloudSeekRetryRef = useRef(0);
  const cloudSeekTimerRef = useRef<ReturnType<typeof setTimeout> | null>(null);
  const cloudProgressStartupTimerRef = useRef<ReturnType<typeof setTimeout> | null>(null);
  const playAttemptIdRef = useRef(0);
  const lastCloudReportRef = useRef({ mediaKey: "", progress: -1, at: 0 });
  const [duration, setDuration] = useState(0);
  const [cloudProgressReadyKey, setCloudProgressReadyKey] = useState("");
  const [volume, setVolume] = useState(1);
  const [isVolumeOpen, setIsVolumeOpen] = useState(false);
  const [isSpeedOpen, setIsSpeedOpen] = useState(false);
  const [isLoudnessEq, setIsLoudnessEq] = useState(
    () => localStorage.getItem(EQ_STORAGE_KEY) === "true",
  );
  const [playbackRate, setPlaybackRate] = useState(1);

  onTimeUpdateRef.current = onTimeUpdate;
  cloudHistoryEnabledRef.current = cloudHistoryEnabled;
  const mediaKey = src && aid && cid ? `${src}:${aid}:${cid}` : "";

  const clearCloudSeekTimer = () => {
    if (cloudSeekTimerRef.current !== null) {
      clearTimeout(cloudSeekTimerRef.current);
      cloudSeekTimerRef.current = null;
    }
  };

  const clearCloudProgressStartupTimer = () => {
    if (cloudProgressStartupTimerRef.current !== null) {
      clearTimeout(cloudProgressStartupTimerRef.current);
      cloudProgressStartupTimerRef.current = null;
    }
  };

  const markCloudProgressReady = () => {
    clearCloudSeekTimer();
    clearCloudProgressStartupTimer();
    cloudSeekInFlightRef.current = false;
    cloudProgressReadyRef.current = true;
    setCloudProgressReadyKey(mediaKey);
  };

  const completeCloudSeek = (audio: HTMLAudioElement, target: number) => {
    pendingCloudProgressRef.current = null;
    currentTimeRef.current = audio.currentTime;
    updateSeekPreviewUi(
      timelineRef.current,
      currentTimeLabelRef.current,
      audio.currentTime,
      audio.duration || 0,
    );
    if (import.meta.env.DEV) {
      console.debug("[player] confirmed cloud progress", {
        aid,
        cid,
        progress: target,
        currentTime: audio.currentTime,
      });
    }
    markCloudProgressReady();
  };

  const applyPendingCloudProgress = (audio: HTMLAudioElement) => {
    const progress = pendingCloudProgressRef.current;
    if (
      progress === null ||
      audio.readyState < HTMLMediaElement.HAVE_METADATA ||
      metadataMediaKeyRef.current !== mediaKey ||
      cloudSeekInFlightRef.current
    ) {
      return false;
    }

    const target = Number.isFinite(audio.duration)
      ? Math.min(progress, Math.max(0, audio.duration - 0.5))
      : progress;

    try {
      const normalizedTarget = Math.max(0, target);
      if (normalizedTarget <= 0.5) {
        audio.currentTime = 0;
        currentTimeRef.current = 0;
        pendingCloudProgressRef.current = null;
        markCloudProgressReady();
        return true;
      }

      if (Math.abs(audio.currentTime - normalizedTarget) <= 1.5) {
        completeCloudSeek(audio, normalizedTarget);
        return true;
      }

      cloudSeekInFlightRef.current = true;
      audio.currentTime = normalizedTarget;
      currentTimeRef.current = normalizedTarget;
      if (import.meta.env.DEV) {
        console.debug("[player] requested cloud seek", {
          aid,
          cid,
          progress,
          target: normalizedTarget,
        });
      }

      clearCloudSeekTimer();
      cloudSeekTimerRef.current = setTimeout(() => {
        if (
          pendingCloudProgressRef.current === null ||
          metadataMediaKeyRef.current !== mediaKey
        ) return;

        if (Math.abs(audio.currentTime - normalizedTarget) <= 1.5) {
          completeCloudSeek(audio, normalizedTarget);
          return;
        }

        cloudSeekInFlightRef.current = false;
        if (cloudSeekRetryRef.current < 1) {
          cloudSeekRetryRef.current += 1;
          applyPendingCloudProgress(audio);
        } else {
          // A broken/missing Range response must not leave playback blocked.
          pendingCloudProgressRef.current = null;
          markCloudProgressReady();
        }
      }, 800);

      return false;
    } catch {
      cloudSeekInFlightRef.current = false;
      return false;
    }
  };

  const reportCloudProgress = (
    progress: number,
    force = false,
    allowZero = false,
  ) => {
    if (
      !cloudHistoryEnabledRef.current ||
      !cloudProgressReadyRef.current ||
      !aid ||
      !cid ||
      !mediaKey
    ) return;

    const normalizedProgress = progress < 0 ? -1 : Math.max(0, Math.floor(progress));
    // Startup/pause events at zero must never erase an existing cloud
    // checkpoint. The first regular report is sent after actual playback.
    if (normalizedProgress === 0 && !allowZero) return;
    const now = Date.now();
    const lastReport = lastCloudReportRef.current;
    const progressDelta = Math.abs(lastReport.progress - normalizedProgress);
    if (
      lastReport.mediaKey === mediaKey &&
      lastReport.progress === normalizedProgress
    ) {
      return;
    }
    if (
      force &&
      normalizedProgress >= 0 &&
      lastReport.mediaKey === mediaKey &&
      !allowZero &&
      progressDelta < 5 &&
      now - lastReport.at < PLAY_PROGRESS_CRITICAL_INTERVAL_MS
    ) {
      return;
    }
    if (
      !force &&
      lastReport.mediaKey === mediaKey &&
      now - lastReport.at < PLAY_PROGRESS_REPORT_INTERVAL_MS
    ) {
      return;
    }

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

  const getOrCreateAudioGraph = (audio: HTMLAudioElement) => {
    const currentGraph = audioGraphRef.current;
    if (currentGraph) return currentGraph;

    const ctx = new AudioContext();
    const source = ctx.createMediaElementSource(audio);
    const compressor = ctx.createDynamicsCompressor();
    // Ratio 1 is a transparent pass-through while keeping the graph stable.
    compressor.threshold.value = 0;
    compressor.knee.value = 0;
    compressor.ratio.value = 1;
    compressor.attack.value = 0;
    compressor.release.value = 0.25;
    source.connect(compressor);
    compressor.connect(ctx.destination);

    const graph = { ctx, source, compressor };
    audioGraphRef.current = graph;
    return graph;
  };

  const applyLoudnessEq = (graph: AudioGraph, enabled: boolean) => {
    const now = graph.ctx.currentTime;
    const end = now + EQ_TRANSITION_SECONDS;
    const targets: Array<[AudioParam, number]> = [
      [graph.compressor.threshold, enabled ? -50 : 0],
      [graph.compressor.knee, enabled ? 40 : 0],
      [graph.compressor.ratio, enabled ? 12 : 1],
    ];

    for (const [param, target] of targets) {
      param.cancelAndHoldAtTime(now);
      param.linearRampToValueAtTime(target, end);
    }
  };

  useEffect(() => {
    return () => {
      clearCloudSeekTimer();
      clearCloudProgressStartupTimer();
      reportCloudProgress(currentTimeRef.current, true);
    };
  }, [mediaKey]); // eslint-disable-line react-hooks/exhaustive-deps

  useEffect(() => {
    setDuration(0);
    isSeekingRef.current = false;
    isKeyboardSeekingRef.current = false;
    seekPointerIdRef.current = null;
    seekPreviewRef.current = null;
    currentTimeRef.current = 0;
    metadataMediaKeyRef.current = "";
    cloudSeekInFlightRef.current = false;
    cloudSeekRetryRef.current = 0;
    clearCloudSeekTimer();
    clearCloudProgressStartupTimer();
    updateSeekPreviewUi(timelineRef.current, currentTimeLabelRef.current, 0, 0);
    lastReportedSecondRef.current = -1;
    if (src) onPlayStateChange?.(true);
  }, [src]);

  useEffect(() => {
    const requestId = ++cloudProgressRequestIdRef.current;
    pendingCloudProgressRef.current = null;
    cloudProgressPendingRef.current = false;
    cloudSeekInFlightRef.current = false;
    cloudSeekRetryRef.current = 0;
    clearCloudSeekTimer();
    clearCloudProgressStartupTimer();

    if (!mediaKey || !cloudHistoryEnabled) {
      markCloudProgressReady();

      return;
    }

    setCloudProgressReadyKey("");
    cloudProgressPendingRef.current = true;
    cloudProgressReadyRef.current = false;
    cloudProgressStartupTimerRef.current = setTimeout(() => {
      if (requestId !== cloudProgressRequestIdRef.current) return;

      // Keep startup responsive on slow/offline networks. Invalidating the
      // request also prevents a late response from jumping an already-playing
      // track.
      cloudProgressRequestIdRef.current += 1;
      cloudProgressPendingRef.current = false;
      pendingCloudProgressRef.current = null;
      markCloudProgressReady();
      if (import.meta.env.DEV) {
        console.debug("[player] cloud progress startup budget exceeded", {
          aid,
          cid,
        });
      }
    }, CLOUD_PROGRESS_STARTUP_BUDGET_MS);
    void invoke<number>("get_play_progress", { aid, cid })
      .then((progress) => {
        if (requestId !== cloudProgressRequestIdRef.current) return;

        clearCloudProgressStartupTimer();
        const normalizedProgress = Math.max(0, progress || 0);
        pendingCloudProgressRef.current = normalizedProgress;
        cloudProgressPendingRef.current = false;
        if (import.meta.env.DEV) {
          console.debug("[player] received cloud progress", {
            aid,
            cid,
            progress: normalizedProgress,
          });
        }
        const audio = audioRef.current;
        if (!audio) {
          markCloudProgressReady();
        } else {
          applyPendingCloudProgress(audio);
        }
      })
      .catch(() => {
        if (requestId !== cloudProgressRequestIdRef.current) return;

        clearCloudProgressStartupTimer();
        cloudProgressPendingRef.current = false;
        markCloudProgressReady();
        if (import.meta.env.DEV) {
          console.debug("[player] cloud progress unavailable", { aid, cid });
        }
      });
  }, [aid, cid, cloudHistoryEnabled, mediaKey]);

  useEffect(() => {
    const audio = audioRef.current;
    if (!audio) return;
    const playAttemptId = ++playAttemptIdRef.current;

    const isCloudProgressReady =
      !cloudHistoryEnabled || !mediaKey || cloudProgressReadyKey === mediaKey;

    if (isPlaying && src && !forcePause && isCloudProgressReady) {
      const graph = audioGraphRef.current;
      void (async () => {
        if (graph?.ctx.state === "suspended") {
          try {
            await graph.ctx.resume();
          } catch {
            return;
          }
        }
        if (playAttemptId !== playAttemptIdRef.current) return;
        try {
          await audio.play();
        } catch {
          onPlayStateChange?.(false);
        }
      })();
    } else {
      if (!audio.paused) {
        audio.pause();
      }
    }
  }, [cloudHistoryEnabled, cloudProgressReadyKey, forcePause, isPlaying, mediaKey, src]);

  useEffect(() => {
    return () => {
      const graph = audioGraphRef.current;
      if (!graph) return;
      graph.source.disconnect();
      graph.compressor.disconnect();
      void graph.ctx.close();
      audioGraphRef.current = null;
    };
  }, []);

  useEffect(() => {
    if (audioRef.current) audioRef.current.playbackRate = playbackRate;
  }, [playbackRate]);

  useEffect(() => {
    if (!isVolumeOpen) return;

    const closeOnOutsidePress = (event: PointerEvent) => {
      if (!volumePopoverRef.current?.contains(event.target as Node)) {
        setIsVolumeOpen(false);
      }
    };

    document.addEventListener("pointerdown", closeOnOutsidePress, true);

    return () => {
      document.removeEventListener("pointerdown", closeOnOutsidePress, true);
    };
  }, [isVolumeOpen]);

  useEffect(() => {
    if (!isSpeedOpen) return;

    const closeOnOutsidePress = (event: PointerEvent) => {
      if (!speedPopoverRef.current?.contains(event.target as Node)) {
        setIsSpeedOpen(false);
      }
    };

    document.addEventListener("pointerdown", closeOnOutsidePress, true);

    return () => {
      document.removeEventListener("pointerdown", closeOnOutsidePress, true);
    };
  }, [isSpeedOpen]);

  const handleTimeUpdate = () => {
    const audio = audioRef.current;
    if (!audio) return;

    if (pendingCloudProgressRef.current !== null) return;

    currentTimeRef.current = audio.currentTime;
    if (!isSeekingRef.current) {
      updateSeekPreviewUi(
        timelineRef.current,
        currentTimeLabelRef.current,
        audio.currentTime,
        duration,
      );
    }
    const second = Math.floor(audio.currentTime);
    if (second !== lastReportedSecondRef.current) {
      lastReportedSecondRef.current = second;
      onTimeUpdateRef.current?.(second);
    }
    reportCloudProgress(second);
  };

  const handleSeek = (value: number) => {
    const audio = audioRef.current;
    if (!audio || !duration) return;

    audio.currentTime = value;
    currentTimeRef.current = value;
    updateSeekPreviewUi(
      timelineRef.current,
      currentTimeLabelRef.current,
      value,
      duration,
    );
  };

  const handleSeekInput = (value: number) => {
    if (isKeyboardSeekingRef.current) {
      handleSeek(value);
      return;
    }

    if (!isSeekingRef.current) {
      updateSeekPreviewUi(
        timelineRef.current,
        currentTimeLabelRef.current,
        currentTimeRef.current,
        duration,
      );
      return;
    }

    seekPreviewRef.current = value;
    updateSeekPreviewUi(
      timelineRef.current,
      currentTimeLabelRef.current,
      value,
      duration,
    );
  };

  useEffect(() => {
    const resetPointerSeek = (restorePlaybackPosition = true) => {
      isSeekingRef.current = false;
      seekPointerIdRef.current = null;
      seekPreviewRef.current = null;
      if (restorePlaybackPosition) {
        updateSeekPreviewUi(
          timelineRef.current,
          currentTimeLabelRef.current,
          currentTimeRef.current,
          duration,
        );
      }
    };

    const commitPointerSeek = (event: PointerEvent) => {
      if (
        !isSeekingRef.current ||
        event.pointerId !== seekPointerIdRef.current
      ) {
        return;
      }

      const value = seekPreviewRef.current;
      const audio = audioRef.current;

      if (value !== null && audio && duration) {
        audio.currentTime = value;
        currentTimeRef.current = value;
        reportCloudProgress(value, true, value <= 0.5);
      }
      resetPointerSeek(false);
    };

    const cancelPointerSeek = () => resetPointerSeek();

    window.addEventListener("pointerup", commitPointerSeek);
    window.addEventListener("pointercancel", cancelPointerSeek);
    window.addEventListener("blur", cancelPointerSeek);

    return () => {
      window.removeEventListener("pointerup", commitPointerSeek);
      window.removeEventListener("pointercancel", cancelPointerSeek);
      window.removeEventListener("blur", cancelPointerSeek);
    };
  }, [duration]);

  const handleVolumeChange = (value: number) => {
    const audio = audioRef.current;
    if (!audio) return;

    audio.volume = value;
    audio.muted = value === 0;
    if (value > 0) lastVolumeRef.current = value;
    setVolume(value);
  };

  const handleLoadedMetadata = (audio: HTMLAudioElement) => {
    metadataMediaKeyRef.current = mediaKey;
    audio.volume = volume;
    audio.muted = volume === 0;
    audio.playbackRate = playbackRate;
    if (!cloudProgressPendingRef.current) {
      const hasPendingProgress = pendingCloudProgressRef.current !== null;
      applyPendingCloudProgress(audio);
      if (!hasPendingProgress) {
        markCloudProgressReady();
      }
    }
    const graph = isLoudnessEq
      ? getOrCreateAudioGraph(audio)
      : audioGraphRef.current;

    if (graph) {
      applyLoudnessEq(graph, isLoudnessEq);
      if (audio.paused && graph.ctx.state === "running") {
        void graph.ctx.suspend();
      } else if (!audio.paused && graph.ctx.state === "suspended") {
        void graph.ctx.resume();
      }
    }
  };

  const handleCanPlay = (audio: HTMLAudioElement) => {
    if (!cloudProgressPendingRef.current && pendingCloudProgressRef.current !== null) {
      applyPendingCloudProgress(audio);
    }
  };

  const handleDurationChange = (audio: HTMLAudioElement) => {
    const nextDuration = audio.duration || 0;

    setDuration(nextDuration);
    updateSeekPreviewUi(
      timelineRef.current,
      currentTimeLabelRef.current,
      currentTimeRef.current,
      nextDuration,
    );
  };

  const suspendAudioGraph = () => {
    const graph = audioGraphRef.current;

    if (graph?.ctx.state === "running") void graph.ctx.suspend();
  };

  const toggleLoudnessEq = () => {
    const newEnabled = !isLoudnessEq;
    const audio = audioRef.current;
    if (audio) {
      const graph = getOrCreateAudioGraph(audio);
      applyLoudnessEq(graph, newEnabled);
      if (audio.paused && graph.ctx.state === "running") {
        void graph.ctx.suspend();
      } else if (!audio.paused && graph.ctx.state === "suspended") {
        void graph.ctx.resume();
      }
    }
    setIsLoudnessEq(newEnabled);
    localStorage.setItem(EQ_STORAGE_KEY, String(newEnabled));
  };

  return (
    <div id="player">
      {/* Audio-only streams do not provide a timed-text track. */}
      {/* eslint-disable-next-line jsx-a11y/media-has-caption */}
      <audio
        ref={audioRef}
        crossOrigin="anonymous"
        preload="metadata"
        src={src || undefined}
        onCanPlay={(event) => handleCanPlay(event.currentTarget)}
        onDurationChange={(event) => handleDurationChange(event.currentTarget)}
        onEnded={() => {
          reportCloudProgress(-1, true);
          suspendAudioGraph();
          onEnded?.();
        }}
        onError={() => {
          suspendAudioGraph();
          onPlayStateChange?.(false);
        }}
        onLoadedMetadata={(event) => handleLoadedMetadata(event.currentTarget)}
        onPause={(event) => {
          // The React state is the source of truth. A native pause event while
          // playback is still desired can only come from source/cloud-sync
          // coordination, even if the event is delivered after playback has
          // already resumed.
          if (isPlaying && Boolean(src) && !forcePause) {
            return;
          }
          if (!event.currentTarget.ended) {
            reportCloudProgress(currentTimeRef.current, true);
          }
          suspendAudioGraph();
          onPlayStateChange?.(false);
        }}
        onPlay={() => {
          reportCloudProgress(currentTimeRef.current, true);
          onPlayStateChange?.(true);
        }}
        onSeeked={(event) => {
          const audio = event.currentTarget;
          const target = pendingCloudProgressRef.current;
          if (target === null) return;

          const boundedTarget = Number.isFinite(audio.duration)
            ? Math.min(target, Math.max(0, audio.duration - 0.5))
            : target;
          if (Math.abs(audio.currentTime - boundedTarget) <= 1.5) {
            completeCloudSeek(audio, boundedTarget);
          }
        }}
        onTimeUpdate={handleTimeUpdate}
      />
      <div className="player-controls">
        <button
          aria-label={isPlaying ? "暂停" : "播放"}
          className="player-button player-play-button"
          data-playing={isPlaying || undefined}
          disabled={!src}
          title={isPlaying ? "暂停" : "播放"}
          type="button"
          onClick={() => onPlayStateChange?.(!isPlaying)}
        >
          {isPlaying ? (
            <Pause fill="currentColor" size={24} theme="filled" />
          ) : (
            <PlayOne fill="currentColor" size={24} theme="filled" />
          )}
        </button>

        <time
          ref={currentTimeLabelRef}
          className="player-time"
          dateTime="PT0S"
        >
          0:00
        </time>

        <div
          className="player-timeline-wrap"
          style={{ "--player-progress": "0%" } as CSSProperties}
        >
          <span aria-hidden="true" className="player-timeline-track" />
          <input
            ref={timelineRef}
            aria-label="播放进度"
            aria-valuetext={`0:00 / ${formatTime(duration)}`}
            className="player-timeline"
            defaultValue={0}
            disabled={!src || !duration}
            max={duration || 0}
            min="0"
            step="0.1"
            type="range"
            onBlur={() => {
              isKeyboardSeekingRef.current = false;
            }}
            onInput={(event) =>
              handleSeekInput(event.currentTarget.valueAsNumber)
            }
            onKeyDown={(event) => {
              if (SEEK_KEYS.has(event.key)) {
                isKeyboardSeekingRef.current = true;
              }
            }}
            onKeyUp={(event) => {
              if (SEEK_KEYS.has(event.key)) {
                isKeyboardSeekingRef.current = false;
                reportCloudProgress(
                  currentTimeRef.current,
                  true,
                  currentTimeRef.current <= 0.5,
                );
              }
            }}
            onPointerDown={(event) => {
              if (
                event.currentTarget.disabled ||
                !event.isPrimary ||
                (event.pointerType === "mouse" && event.button !== 0)
              ) {
                return;
              }

              const value = Number(event.currentTarget.value);

              isSeekingRef.current = true;
              seekPointerIdRef.current = event.pointerId;
              seekPreviewRef.current = value;
            }}
          />
        </div>

        <time className="player-time" dateTime={`PT${Math.floor(duration)}S`}>
          {formatTime(duration)}
        </time>

        <div className="player-volume" ref={volumePopoverRef}>
          <button
            aria-expanded={isVolumeOpen}
            aria-label={volume > 0 ? "音量" : "取消静音"}
            className="player-button player-volume-button"
            data-open={isVolumeOpen || undefined}
            disabled={!src}
            title={volume > 0 ? "音量" : "取消静音"}
            type="button"
            onClick={() => {
              if (volume === 0) handleVolumeChange(lastVolumeRef.current || 1);
              setIsVolumeOpen((open) => !open);
            }}
          >
            {volume > 0 ? (
              <VolumeNotice fill="currentColor" size={20} theme="outline" />
            ) : (
              <VolumeMute fill="currentColor" size={20} theme="outline" />
            )}
          </button>
          {isVolumeOpen && (
            <div className="player-volume-popover">
              <input
                aria-label="音量"
                aria-valuetext={`${Math.round(volume * 100)}%`}
                className="player-volume-slider"
                max="1"
                min="0"
                step="0.01"
                style={{ "--player-volume": `${volume * 100}%` } as CSSProperties}
                type="range"
                value={volume}
                onChange={(event) => handleVolumeChange(Number(event.currentTarget.value))}
              />
            </div>
          )}
        </div>

        <div className="player-speed" ref={speedPopoverRef}>
          <button
            aria-label={`播放速度 ${playbackRate} 倍`}
            aria-expanded={isSpeedOpen}
            className="player-button player-speed-button"
            data-open={isSpeedOpen || undefined}
            disabled={!src}
            title={`播放速度 ${playbackRate} 倍`}
            type="button"
            onClick={() => setIsSpeedOpen((open) => !open)}
          >
            <span className="player-speed-label">
              {playbackRate === 1 || playbackRate === 2 || playbackRate === 3
                ? playbackRate.toFixed(1)
                : playbackRate}x
            </span>
          </button>
          {isSpeedOpen && (
            <div className="player-speed-popover">
              {PLAYBACK_RATES.map((rate) => (
                <button
                  key={rate}
                  className={`player-speed-option ${rate === playbackRate ? "is-active" : ""}`}
                  type="button"
                  onClick={() => {
                    setPlaybackRate(rate);
                    setIsSpeedOpen(false);
                  }}
                >
                  {rate === 1 || rate === 2 || rate === 3 ? rate.toFixed(1) : rate}x
                </button>
              ))}
            </div>
          )}
        </div>

        <button
          aria-label={isLoudnessEq ? "关闭音量均衡" : "开启音量均衡"}
          aria-pressed={isLoudnessEq}
          className="player-button player-eq-button"
          data-active={isLoudnessEq || undefined}
          disabled={!src}
          title={isLoudnessEq ? "音量均衡: 开" : "音量均衡: 关"}
          type="button"
          onClick={toggleLoudnessEq}
        >
          <Equalizer fill="currentColor" size={17} theme="outline" />
        </button>
      </div>
    </div>
  );
};

export default Player;
