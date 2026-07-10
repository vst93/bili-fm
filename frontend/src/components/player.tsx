import { AudioPlayer, AudioPlayerRef } from "react-audio-play";
import { useEffect, useRef, useCallback, useState, useImperativeHandle, forwardRef } from "react";
import { ReportPlayProgress } from "../../wailsjs/go/service/BL";

interface PlayerProps {
  src?: string;
  onEnded?: () => void;
  onPlayStateChange?: (isPlaying: boolean) => void;
  onTimeUpdate?: (time: number) => void;
  isPlaying?: boolean;
  aid?: number;
  cid?: number;
}

interface PlayerRef {
  getCurrentTime: () => number;
}

const Player = forwardRef<PlayerRef, PlayerProps>(function Player({
  src,
  onEnded,
  onPlayStateChange,
  onTimeUpdate,
  isPlaying,
  aid,
  cid,
}: PlayerProps, ref) {
  let autoPlay = true;
  const playerRef = useRef<AudioPlayerRef>(null);
  const audioRef = useRef<HTMLAudioElement | null>(null);
  const timeUpdateInterval = useRef<number>();
  const [currentTime, setCurrentTime] = useState(0);

  // 暴露方法给父组件
  useImperativeHandle(ref, () => ({
    getCurrentTime: () => currentTime,
  }), [currentTime]);

  // 尝试获取内部 audio 元素
  useEffect(() => {
    const container = document.getElementById("player");
    if (container) {
      const audioEl = container.querySelector("audio");
      if (audioEl) {
        audioRef.current = audioEl;
      }
    }
  }, [src]);

  const startTimeUpdate = useCallback(() => {
    if (timeUpdateInterval.current) {
      clearInterval(timeUpdateInterval.current);
    }
    timeUpdateInterval.current = window.setInterval(() => {
      let time = 0;
      
      // 方法1: 尝试从 audio 元素获取
      if (audioRef.current) {
        time = Math.floor(audioRef.current.currentTime);
      }
      
      // 方法2: 如果有播放，从 playerRef 尝试
      if (time === 0 && playerRef.current) {
        const player = playerRef.current as unknown as { currentTime?: number };
        if (player.currentTime !== undefined) {
          time = Math.floor(player.currentTime);
        }
      }
      
      // 方法3: 使用内部状态累加
      if (time === 0 && isPlaying) {
        time = currentTime + 0.5;
      }
      
      if (time > 0 && time !== currentTime) {
        setCurrentTime(time);
        onTimeUpdate?.(time);
      }
    }, 500);
  }, [isPlaying, onTimeUpdate, currentTime]);

  const stopTimeUpdate = useCallback(() => {
    if (timeUpdateInterval.current) {
      clearInterval(timeUpdateInterval.current);
      timeUpdateInterval.current = undefined;
    }
  }, []);

  useEffect(() => {
    if (isPlaying) {
      startTimeUpdate();
    } else {
      stopTimeUpdate();
    }
    return () => stopTimeUpdate();
  }, [isPlaying, startTimeUpdate, stopTimeUpdate]);

  useEffect(() => {
    if (src && autoPlay) {
      onPlayStateChange?.(true);
    }
  }, [src]);

  useEffect(() => {
    // Fix: react-audio-play's slider drag doesn't work on Windows WebView2.
    // We replace it entirely using Pointer Events with setPointerCapture.
    //
    // Performance: seeking (setting audio.currentTime) is expensive and causes
    // audio decoder to re-sync, making the UI feel laggy during drag. Instead:
    // - During drag: visually update the progress bar width directly via DOM,
    //   no actual seek. This is instant and smooth.
    // - On pointerup: perform a single seek to the final position.
    // - Volume slider: volume changes are cheap, apply immediately.
    // - Click (no drag): seek immediately (click-to-seek).

    const getAudioEl = (): HTMLAudioElement | null => {
      return document.querySelector("#player audio");
    };

    const computeRatio = (event: PointerEvent, slider: HTMLElement): number => {
      const rect = slider.getBoundingClientRect();
      const isVertical = slider.dataset.direction === "vertical";
      let ratio: number;
      if (isVertical) {
        ratio = 1 - (event.clientY - rect.top) / rect.height;
      } else {
        ratio = (event.clientX - rect.left) / rect.width;
      }
      return Math.max(0, Math.min(1, ratio));
    };

    const applyVolume = (ratio: number, audioEl: HTMLAudioElement) => {
      audioEl.volume = ratio;
    };

    const seekAudio = (ratio: number, audioEl: HTMLAudioElement) => {
      if (audioEl.duration && audioEl.duration !== Infinity) {
        audioEl.currentTime = ratio * audioEl.duration;
      }
    };

    // Visually update progress bar without seeking
    const updateProgressVisual = (slider: HTMLElement, ratio: number) => {
      const isVertical = slider.dataset.direction === "vertical";
      const progress = slider.querySelector<HTMLElement>(".rap-progress");
      if (progress) {
        if (isVertical) {
          progress.style.height = `${ratio * 100}%`;
        } else {
          progress.style.width = `${ratio * 100}%`;
        }
      }
    };

    // Block the library's mousedown handler from starting a competing drag.
    const blockMouseDown = (event: MouseEvent) => {
      const target = event.target as HTMLElement | null;
      if (target?.closest("#player .rap-slider") || target?.closest("#player .rap-pin")) {
        event.preventDefault();
        event.stopImmediatePropagation();
      }
    };

    const handlePointerDown = (event: PointerEvent) => {
      const target = event.target as HTMLElement | null;
      const slider = target?.closest("#player .rap-slider") as HTMLElement | null;
      if (!slider) return;

      const audioEl = getAudioEl();
      if (!audioEl) return;

      slider.setPointerCapture(event.pointerId);
      event.preventDefault();

      const ratio = computeRatio(event, slider);
      const isVolume = slider.closest("#player .rap-volume-controls") !== null;

      if (isVolume) {
        // Volume: apply immediately, it's cheap
        applyVolume(ratio, audioEl);
        updateProgressVisual(slider, ratio);

        const onMove = (e: PointerEvent) => {
          e.preventDefault();
          const r = computeRatio(e, slider);
          applyVolume(r, audioEl);
          updateProgressVisual(slider, r);
        };
        const onUp = (e: PointerEvent) => {
          slider.releasePointerCapture(e.pointerId);
          slider.removeEventListener("pointermove", onMove);
          slider.removeEventListener("pointerup", onUp);
        };
        slider.addEventListener("pointermove", onMove);
        slider.addEventListener("pointerup", onUp);
      } else {
        // Seek: visual-only during drag, actual seek on release
        updateProgressVisual(slider, ratio);

        const onMove = (e: PointerEvent) => {
          e.preventDefault();
          const r = computeRatio(e, slider);
          updateProgressVisual(slider, r);
        };
        const onUp = (e: PointerEvent) => {
          slider.releasePointerCapture(e.pointerId);
          slider.removeEventListener("pointermove", onMove);
          slider.removeEventListener("pointerup", onUp);
          // Single seek on release — uses final position
          const finalRatio = computeRatio(e, slider);
          seekAudio(finalRatio, audioEl);
        };
        slider.addEventListener("pointermove", onMove);
        slider.addEventListener("pointerup", onUp);
      }
    };

    document.addEventListener("pointerdown", handlePointerDown, true);
    document.addEventListener("mousedown", blockMouseDown, true);

    return () => {
      document.removeEventListener("pointerdown", handlePointerDown, true);
      document.removeEventListener("mousedown", blockMouseDown, true);
    };
  }, []);

  useEffect(() => {
    const handlePointerDown = (event: PointerEvent) => {
      const target = event.target as HTMLElement | null;
      const openVolume = document.querySelector<HTMLElement>("#player .rap-volume-open");

      if (!openVolume || target?.closest("#player .rap-volume")) return;
      openVolume.click();
    };

    document.addEventListener("pointerdown", handlePointerDown, true);

    return () => {
      document.removeEventListener("pointerdown", handlePointerDown, true);
    };
  }, []);

  useEffect(() => {
    if (isPlaying && aid && cid) {
      ReportPlayProgress(aid, cid, 0);
    }
  }, [isPlaying, aid, cid]);

  if (isPlaying) {
    playerRef.current?.play();
  } else {
    playerRef.current?.pause();
  }

  if (!src) {
    autoPlay = false;
  }

  return (
    <div id="player" style={{}}>
      <AudioPlayer
        ref={playerRef}
        autoPlay={autoPlay}
        hasKeyBindings={false}
        loop={false}
        sliderColor="#68bca4"
        src={src || ""}
        width="100%"
        onEnd={onEnded}
        onError={() => onPlayStateChange?.(false)}
        onPause={() => onPlayStateChange?.(false)}
        onPlay={() => onPlayStateChange?.(true)}
        className="bg-transparent"
      />
    </div>
  );
});

export default Player;
