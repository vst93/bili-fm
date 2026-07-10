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
    // Fix: react-audio-play's slider drag fights with React's onTimeUpdate.
    // When audio is playing, the library's onTimeUpdate fires every ~250ms,
    // causing a React re-render that sets element.style.width = "XX%". This
    // REPLACES our inline style (including clearing !important), so the
    // progress bar snaps back to the actual playback position during drag.
    //
    // Solution: use a CSS class (.is-dragging) with !important in the CSS
    // rule itself. React can set style.width all it wants — the CSS !important
    // rule wins. We pass the desired width via a CSS custom property.
    //
    // During drag: add .is-dragging, set --drag-w / --drag-h
    // On release: seek, wait for timeupdate to update React state, then
    // remove .is-dragging. This prevents any visual flash.

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

    // Start dragging: add .is-dragging class and set CSS custom property.
    // The CSS rule .rap-progress.is-dragging has !important, so React's
    // inline style.width can never override it.
    const startDrag = (slider: HTMLElement, ratio: number) => {
      const progress = slider.querySelector<HTMLElement>(".rap-progress");
      if (!progress) return;
      const isVertical = slider.dataset.direction === "vertical";
      const pct = `${ratio * 100}%`;
      progress.style.setProperty(isVertical ? "--drag-h" : "--drag-w", pct);
      progress.classList.add("is-dragging");
    };

    // Update drag position (just changes the CSS variable)
    const updateDrag = (slider: HTMLElement, ratio: number) => {
      const progress = slider.querySelector<HTMLElement>(".rap-progress");
      if (!progress) return;
      const isVertical = slider.dataset.direction === "vertical";
      const pct = `${ratio * 100}%`;
      progress.style.setProperty(isVertical ? "--drag-h" : "--drag-w", pct);
    };

    // End dragging: remove .is-dragging class so library's React state
    // resumes control. Called after timeupdate confirms React state is current.
    const endDrag = (slider: HTMLElement) => {
      const progress = slider.querySelector<HTMLElement>(".rap-progress");
      if (!progress) return;
      progress.classList.remove("is-dragging");
      progress.style.removeProperty("--drag-w");
      progress.style.removeProperty("--drag-h");
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
        startDrag(slider, ratio);

        const onMove = (e: PointerEvent) => {
          e.preventDefault();
          const r = computeRatio(e, slider);
          applyVolume(r, audioEl);
          updateDrag(slider, r);
        };
        const onUp = (e: PointerEvent) => {
          slider.releasePointerCapture(e.pointerId);
          slider.removeEventListener("pointermove", onMove);
          slider.removeEventListener("pointerup", onUp);
          endDrag(slider);
        };
        slider.addEventListener("pointermove", onMove);
        slider.addEventListener("pointerup", onUp);
      } else {
        // Seek: visual-only during drag, actual seek on release

        // ---- Cleanup tracking for this drag session ----
        // We must clean up ALL listeners and timers when the drag ends,
        // so a rapid re-drag doesn't have stale callbacks that call
        // endDrag() at the wrong time and cause the "jump" bug.
        let cleanupFns: (() => void)[] = [];
        const cleanup = () => {
          cleanupFns.forEach(fn => fn());
          cleanupFns = [];
        };

        startDrag(slider, ratio);

        const onMove = (e: PointerEvent) => {
          e.preventDefault();
          const r = computeRatio(e, slider);
          updateDrag(slider, r);
        };

        const finishDrag = (e: PointerEvent) => {
          const finalRatio = computeRatio(e, slider);
          // Update visual to final position
          updateDrag(slider, finalRatio);
          // Seek to final position
          seekAudio(finalRatio, audioEl);

          // Wait for timeupdate to confirm the seek completed, then
          // release the CSS lock. The library's React state will now
          // have the correct position, so removing .is-dragging is seamless.
          let done = false;
          const release = () => {
            if (done) return;
            done = true;
            endDrag(slider);
            cleanup();
          };

          const onSeekedUpdate = () => {
            // Verify the audio position actually matches our target
            // before releasing the visual lock. This prevents releasing
            // too early if a stale timeupdate fires.
            const expectedTime = finalRatio * audioEl.duration;
            const actualTime = audioEl.currentTime;
            if (Math.abs(actualTime - expectedTime) < 2) {
              release();
            }
            // If not close enough, keep waiting (fallback timer will handle it)
          };
          audioEl.addEventListener("timeupdate", onSeekedUpdate);
          cleanupFns.push(() => audioEl.removeEventListener("timeupdate", onSeekedUpdate));

          // Fallback: release after 800ms if timeupdate doesn't fire
          const timer = setTimeout(release, 800);
          cleanupFns.push(() => clearTimeout(timer));
        };

        const onUp = (e: PointerEvent) => {
          slider.releasePointerCapture(e.pointerId);
          slider.removeEventListener("pointermove", onMove);
          slider.removeEventListener("pointerup", onUp);
          slider.removeEventListener("pointercancel", onCancel);
          finishDrag(e);
        };

        const onCancel = (e: PointerEvent) => {
          slider.releasePointerCapture(e.pointerId);
          slider.removeEventListener("pointermove", onMove);
          slider.removeEventListener("pointerup", onUp);
          slider.removeEventListener("pointercancel", onCancel);
          // For cancel, seek and release immediately
          const finalRatio = computeRatio(e, slider);
          updateDrag(slider, finalRatio);
          seekAudio(finalRatio, audioEl);
          endDrag(slider);
          cleanup();
        };

        slider.addEventListener("pointermove", onMove);
        slider.addEventListener("pointerup", onUp);
        slider.addEventListener("pointercancel", onCancel);
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
