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
  const [duration, setDuration] = useState(0);
  const [volume, setVolume] = useState(1);
  const [isVolumeOpen, setIsVolumeOpen] = useState(false);
  const [isSpeedOpen, setIsSpeedOpen] = useState(false);
  const [isLoudnessEq, setIsLoudnessEq] = useState(
    () => localStorage.getItem(EQ_STORAGE_KEY) === "true",
  );
  const [playbackRate, setPlaybackRate] = useState(1);

  onTimeUpdateRef.current = onTimeUpdate;

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
    setDuration(0);
    isSeekingRef.current = false;
    isKeyboardSeekingRef.current = false;
    seekPointerIdRef.current = null;
    seekPreviewRef.current = null;
    currentTimeRef.current = 0;
    updateSeekPreviewUi(timelineRef.current, currentTimeLabelRef.current, 0, 0);
    lastReportedSecondRef.current = -1;
    if (src) onPlayStateChange?.(true);
  }, [src]);

  useEffect(() => {
    const audio = audioRef.current;
    if (!audio) return;

    if (isPlaying && src && !forcePause) {
      const graph = audioGraphRef.current;
      if (graph?.ctx.state === "suspended") void graph.ctx.resume();
      void audio.play().catch(() => onPlayStateChange?.(false));
    } else {
      audio.pause();
    }
  }, [forcePause, isPlaying, src]);

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

  useEffect(() => {
    if (isPlaying && aid && cid) {
      invoke("report_play_progress", { aid, cid, progress: 0 });
    }
  }, [isPlaying, aid, cid]);

  const handleTimeUpdate = () => {
    const audio = audioRef.current;
    if (!audio) return;

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
    audio.volume = volume;
    audio.muted = volume === 0;
    audio.playbackRate = playbackRate;
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
        onDurationChange={(event) => handleDurationChange(event.currentTarget)}
        onEnded={() => {
          suspendAudioGraph();
          onEnded?.();
        }}
        onError={() => {
          suspendAudioGraph();
          onPlayStateChange?.(false);
        }}
        onLoadedMetadata={(event) => handleLoadedMetadata(event.currentTarget)}
        onPause={() => {
          suspendAudioGraph();
          onPlayStateChange?.(false);
        }}
        onPlay={() => onPlayStateChange?.(true)}
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
