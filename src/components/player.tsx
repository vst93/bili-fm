import { useEffect, useRef, useState } from "react";
import type { CSSProperties } from "react";
import { Pause, PlayOne, VolumeMute, VolumeNotice } from "@icon-park/react";
import { invoke } from "@tauri-apps/api/core";

const PLAYBACK_RATES = [0.5, 0.75, 1, 1.25, 1.5, 2, 3] as const;

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
  const volumePopoverRef = useRef<HTMLDivElement>(null);
  const speedPopoverRef = useRef<HTMLDivElement>(null);
  const onTimeUpdateRef = useRef(onTimeUpdate);
  const lastReportedSecondRef = useRef(-1);
  const lastVolumeRef = useRef(1);
  const audioGraphRef = useRef<{
    ctx: AudioContext;
    source: MediaElementAudioSourceNode;
    compressor: DynamicsCompressorNode;
  } | null>(null);
  const [currentTime, setCurrentTime] = useState(0);
  const [duration, setDuration] = useState(0);
  const [volume, setVolume] = useState(1);
  const [isVolumeOpen, setIsVolumeOpen] = useState(false);
  const [isSpeedOpen, setIsSpeedOpen] = useState(false);
  const [playbackRate, setPlaybackRate] = useState(1);
  const [isLoudnessEq, setIsLoudnessEq] = useState(false);

  onTimeUpdateRef.current = onTimeUpdate;

  useEffect(() => {
    setCurrentTime(0);
    setDuration(0);
    lastReportedSecondRef.current = -1;
    if (src) onPlayStateChange?.(true);
  }, [src]);

  useEffect(() => {
    const audio = audioRef.current;
    if (!audio) return;

    if (isPlaying && src && !forcePause) {
      void audio.play().catch(() => onPlayStateChange?.(false));
    } else {
      audio.pause();
    }
  }, [forcePause, isPlaying, src]);

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

  useEffect(() => {
    return () => {
      if (audioGraphRef.current) {
        try {
          void audioGraphRef.current.ctx.close();
        } catch {
          // AudioContext may already be closed.
        }
        audioGraphRef.current = null;
      }
    };
  }, []);

  const handleTimeUpdate = () => {
    const audio = audioRef.current;
    if (!audio) return;

    setCurrentTime(audio.currentTime);
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
    setCurrentTime(value);
  };

  const handleVolumeChange = (value: number) => {
    const audio = audioRef.current;
    if (!audio) return;

    audio.volume = value;
    audio.muted = value === 0;
    if (value > 0) lastVolumeRef.current = value;
    setVolume(value);
  };

  const initAudioGraph = () => {
    if (audioGraphRef.current || !audioRef.current) return;

    const ctx = new AudioContext();
    const source = ctx.createMediaElementSource(audioRef.current);
    const compressor = ctx.createDynamicsCompressor();

    compressor.threshold.value = -50;
    compressor.knee.value = 40;
    compressor.ratio.value = 12;
    compressor.attack.value = 0;
    compressor.release.value = 0.25;
    source.connect(ctx.destination);
    audioGraphRef.current = { ctx, source, compressor };
  };

  const toggleLoudnessEq = () => {
    if (!audioGraphRef.current) initAudioGraph();

    const graph = audioGraphRef.current;
    if (!graph) return;

    const newEnabled = !isLoudnessEq;

    setIsLoudnessEq(newEnabled);
    try {
      graph.source.disconnect();
      graph.compressor.disconnect();
    } catch {
      // Nodes may not have an active connection yet.
    }

    if (newEnabled) {
      graph.source.connect(graph.compressor);
      graph.compressor.connect(graph.ctx.destination);
    } else {
      graph.source.connect(graph.ctx.destination);
    }

    if (graph.ctx.state === "suspended") void graph.ctx.resume();
  };

  const progress = duration > 0 ? (currentTime / duration) * 100 : 0;

  return (
    <div id="player">
      {/* Audio-only streams do not provide a timed-text track. */}
      {/* eslint-disable-next-line jsx-a11y/media-has-caption */}
      <audio
        ref={audioRef}
        preload="metadata"
        src={src || undefined}
        onDurationChange={(event) => setDuration(event.currentTarget.duration || 0)}
        onEnded={onEnded}
        onError={() => onPlayStateChange?.(false)}
        onPause={() => onPlayStateChange?.(false)}
        onPlay={() => onPlayStateChange?.(true)}
        onTimeUpdate={handleTimeUpdate}
      />
      <div className="player-controls">
        <button
          aria-label={isPlaying ? "暂停" : "播放"}
          className="player-button player-play-button"
          disabled={!src}
          title={isPlaying ? "暂停" : "播放"}
          type="button"
          onClick={() => onPlayStateChange?.(!isPlaying)}
        >
          {isPlaying ? (
            <Pause fill="currentColor" size={20} theme="filled" />
          ) : (
            <PlayOne fill="currentColor" size={20} theme="filled" />
          )}
        </button>

        <time className="player-time" dateTime={`PT${Math.floor(currentTime)}S`}>
          {formatTime(currentTime)}
        </time>

        <input
          aria-label="播放进度"
          aria-valuetext={`${formatTime(currentTime)} / ${formatTime(duration)}`}
          className="player-timeline"
          disabled={!src || !duration}
          max={duration || 0}
          min="0"
          step="0.1"
          style={{ "--player-progress": `${progress}%` } as CSSProperties}
          type="range"
          value={Math.min(currentTime, duration || 0)}
          onChange={(event) => handleSeek(Number(event.currentTarget.value))}
        />

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
          className="player-button player-eq-button"
          data-active={isLoudnessEq || undefined}
          disabled={!src}
          title={isLoudnessEq ? "音量均衡: 开" : "音量均衡: 关"}
          type="button"
          onClick={toggleLoudnessEq}
        >
          <span className="player-eq-label">EQ</span>
        </button>
      </div>
    </div>
  );
};

export default Player;
