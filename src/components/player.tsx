import { useEffect, useRef, useState } from "react";
import type { CSSProperties, PointerEvent as ReactPointerEvent } from "react";
import {
  Ad,
  Equalizer,
  GoEnd,
  Pause,
  PlayOne,
  VolumeMute,
  VolumeNotice,
} from "@icon-park/react";
import { invoke } from "@tauri-apps/api/core";
import { fetchSegments } from "../lib/sponsorBlock";
import type { SponsorSegment } from "../lib/sponsorBlock";
import { toast } from "../utils/toast";

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
const SPONSOR_SKIP_STORAGE_KEY = "sponsorSkip";
const PLAYBACK_RATE_STORAGE_KEY = "playbackRate";
const VOLUME_STORAGE_KEY = "volume";
// 进度条容差：currentTime 落入 [start - LEAD, end) 即视为跨入该片段。
const SPONSOR_SEGMENT_LEAD_SECONDS = 0.25;
// 跳过后落到 end + PAD，确保不再落回同一区间（避免死循环）。
const SPONSOR_SEEK_PAD_SECONDS = 0.05;
// 过期校验容差：与提交时 videoDuration 相差超过该值则丢弃片段。
const SPONSOR_DURATION_TOLERANCE_SECONDS = 2;
// Toast 频控：10 秒内多条合并为一次。
const SPONSOR_TOAST_MIN_INTERVAL_MS = 10_000;

// 读取倍速偏好：localStorage 为主，非法/越界值回落 1（必须命中 PLAYBACK_RATES 档位）
const readStoredPlaybackRate = (): number => {
  const saved = Number(localStorage.getItem(PLAYBACK_RATE_STORAGE_KEY));
  return (PLAYBACK_RATES as readonly number[]).includes(saved) ? saved : 1;
};

// 读取音量偏好：clamp 到 0~1，非法值回落 1
const readStoredVolume = (): number => {
  const raw = localStorage.getItem(VOLUME_STORAGE_KEY);
  if (raw === null) return 1;
  const saved = Number(raw);
  if (!Number.isFinite(saved) || saved < 0 || saved > 1) return 1;
  return saved;
};
const PLAY_PROGRESS_REPORT_INTERVAL_MS = 30_000;
const PLAY_PROGRESS_CRITICAL_INTERVAL_MS = 3_000;
const CLOUD_PROGRESS_STARTUP_BUDGET_MS = 800;

// 本地播放断点（不依赖账号）：登录/离线/风控导致云端进度不可用时兜底续播。
const LOCAL_RESUME_STORAGE_KEY = "localResumePoints";
const LOCAL_RESUME_MAX_ENTRIES = 50;
const LOCAL_RESUME_WRITE_INTERVAL_MS = 5_000;
const LOCAL_RESUME_MAX_AGE_MS = 7 * 24 * 60 * 60 * 1000;

type LocalResumePoint = { t: number; ts: number };
type LocalResumeMap = Record<string, LocalResumePoint>;

const readLocalResumeMap = (): LocalResumeMap => {
  try {
    const raw = localStorage.getItem(LOCAL_RESUME_STORAGE_KEY);
    if (!raw) return {};
    const parsed = JSON.parse(raw);
    if (!parsed || typeof parsed !== "object" || Array.isArray(parsed)) return {};
    return parsed as LocalResumeMap;
  } catch {
    return {};
  }
};

const readLocalResumePoint = (key: string): number => {
  const map = readLocalResumeMap();
  const entry = map[key];
  if (!entry || typeof entry.t !== "number" || typeof entry.ts !== "number") return -1;
  if (Date.now() - entry.ts > LOCAL_RESUME_MAX_AGE_MS) return -1;
  return entry.t;
};

const writeLocalResumePoint = (key: string, t: number) => {
  if (!key || !Number.isFinite(t) || t < 0) return;
  try {
    const map = readLocalResumeMap();
    const now = Date.now();
    // 顺手清理 7 天以上未更新的旧条目，防止无限膨胀。
    for (const k of Object.keys(map)) {
      const entry = map[k];
      if (!entry || typeof entry.ts !== "number" || now - entry.ts > LOCAL_RESUME_MAX_AGE_MS) {
        delete map[k];
      }
    }
    map[key] = { t: Math.floor(t), ts: now };
    // 超过上限时淘汰最旧的 50-key 之外条目（LRU by ts）。
    const keys = Object.keys(map);
    if (keys.length > LOCAL_RESUME_MAX_ENTRIES) {
      keys
        .sort((a, b) => map[a].ts - map[b].ts)
        .slice(0, keys.length - LOCAL_RESUME_MAX_ENTRIES)
        .forEach((k) => delete map[k]);
    }
    localStorage.setItem(LOCAL_RESUME_STORAGE_KEY, JSON.stringify(map));
  } catch {
    // 隐私模式等场景 localStorage 会抛异常，静默降级为不做本地断点。
  }
};

type AudioGraph = {
  ctx: AudioContext;
  source: MediaElementAudioSourceNode;
  compressor: DynamicsCompressorNode;
};

interface PlayerProps {
  src?: string;
  onEnded?: () => void;
  onNext?: () => void;
  canNext?: boolean;
  onPlayStateChange?: (isPlaying: boolean) => void;
  onTimeUpdate?: (time: number) => void;
  onError?: (error: MediaError | null) => void;
  isPlaying?: boolean;
  aid?: number;
  bvid?: string;
  cid?: number;
  forcePause?: boolean;
  cloudHistoryEnabled?: boolean;
  // 单曲循环重播信号：自增时把音频 seek 回 0 并继续播放（走 safeSeek 保护路径）
  replaySignal?: number;
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
  bubble?: HTMLDivElement | null,
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
  // 拖动中若 hover 气泡已存在，复用它跟随拖动预览位置。
  if (bubble && duration > 0) {
    bubble.textContent = formatTime(boundedValue);
    bubble.style.left = `${progress}%`;
  }
};

const updateBufferedUi = (
  bufferedEl: HTMLSpanElement | null,
  audio: HTMLAudioElement,
  duration: number,
) => {
  if (!bufferedEl) return;
  const ranges = audio.buffered;
  const usableDuration = Number.isFinite(duration) && duration > 0;
  const end = ranges.length > 0 ? ranges.end(ranges.length - 1) : 0;
  if (!usableDuration || !Number.isFinite(end) || end <= 0) {
    bufferedEl.style.setProperty("--player-buffered", "0%");
    bufferedEl.dataset.available = "false";
    return;
  }
  const buffered = Math.min(Math.max(end, 0), duration) / duration;
  bufferedEl.style.setProperty("--player-buffered", `${buffered * 100}%`);
  bufferedEl.dataset.available = "true";
};

const Player = ({
  src,
  onEnded,
  onNext,
  canNext = true,
  onPlayStateChange,
  onTimeUpdate,
  onError,
  isPlaying = false,
  aid,
  bvid,
  cid,
  forcePause = false,
  cloudHistoryEnabled = true,
  replaySignal = 0,
}: PlayerProps) => {
  const audioRef = useRef<HTMLAudioElement>(null);
  const timelineRef = useRef<HTMLInputElement>(null);
  const bufferedRef = useRef<HTMLSpanElement>(null);
  const seekBubbleRef = useRef<HTMLDivElement>(null);
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
  const lastLocalResumeWriteRef = useRef(0);
  // SponsorBlock 跳过：当前曲目的片段、已跳过索引集合、Toast 频控时间戳。
  const sponsorSegmentsRef = useRef<SponsorSegment[]>([]);
  const sponsorSkipFiredRef = useRef<Set<number>>(new Set());
  const sponsorToastAtRef = useRef(0);
  // 广告段可视化：segments 变化时递增版本号以驱动进度条标记层重渲染。
  const sponsorMarkerRef = useRef<HTMLSpanElement>(null);
  const [sponsorSegmentsVersion, setSponsorSegmentsVersion] = useState(0);
  const [duration, setDuration] = useState(0);
  const [cloudProgressReadyKey, setCloudProgressReadyKey] = useState("");
  const [volume, setVolume] = useState(readStoredVolume);
  const [isVolumeOpen, setIsVolumeOpen] = useState(false);
  const [isSpeedOpen, setIsSpeedOpen] = useState(false);
  const [isLoudnessEq, setIsLoudnessEq] = useState(
    () => localStorage.getItem(EQ_STORAGE_KEY) === "true",
  );
  const [playbackRate, setPlaybackRate] = useState(readStoredPlaybackRate);
  const [sponsorSkip, setSponsorSkip] = useState(
    () => localStorage.getItem(SPONSOR_SKIP_STORAGE_KEY) === "true",
  );

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

  const safeSeek = (audio: HTMLAudioElement, target: number): boolean => {
    // Only write currentTime when the media has at least a current data frame.
    // Writing while still loading or without a playable frame can block
    // the WebKit media pipeline on some Linux configurations.
    // However, if readyState is HAVE_ENOUGH_DATA, we can safely seek even if
    // the network is still loading (buffering ahead).
    if (audio.readyState < HTMLMediaElement.HAVE_CURRENT_DATA) {
      if (import.meta.env.DEV) {
        console.debug("[player] safeSeek blocked: insufficient data", {
          target,
          readyState: audio.readyState,
          HAVE_CURRENT_DATA: HTMLMediaElement.HAVE_CURRENT_DATA,
        });
      }
      return false;
    }

    // If we don't have enough data yet, also check networkState
    if (
      audio.readyState < HTMLMediaElement.HAVE_ENOUGH_DATA &&
      audio.networkState === HTMLMediaElement.NETWORK_LOADING
    ) {
      if (import.meta.env.DEV) {
        console.debug("[player] safeSeek blocked: network loading", {
          target,
          readyState: audio.readyState,
          networkState: audio.networkState,
          HAVE_ENOUGH_DATA: HTMLMediaElement.HAVE_ENOUGH_DATA,
          NETWORK_LOADING: HTMLMediaElement.NETWORK_LOADING,
        });
      }
      return false;
    }

    audio.currentTime = target;
    currentTimeRef.current = target;
    return true;
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
      // 如果进度小于等于 0.5 秒，从头开始
      if (normalizedTarget <= 0.5) {
        safeSeek(audio, 0);
        currentTimeRef.current = 0;
        pendingCloudProgressRef.current = null;
        markCloudProgressReady();
        return true;
      }

      // 如果进度距离结尾小于 5 秒，认为已播放完毕，重置到开头
      if (Number.isFinite(audio.duration) && audio.duration - normalizedTarget < 5) {
        safeSeek(audio, 0);
        currentTimeRef.current = 0;
        pendingCloudProgressRef.current = null;
        markCloudProgressReady();
        if (import.meta.env.DEV) {
          console.debug("[player] cloud progress near end, reset to start", {
            aid,
            cid,
            progress,
            duration: audio.duration,
          });
        }
        return true;
      }

      if (Math.abs(audio.currentTime - normalizedTarget) <= 1.5) {
        completeCloudSeek(audio, normalizedTarget);
        return true;
      }

      cloudSeekInFlightRef.current = true;
      const seekSuccess = safeSeek(audio, normalizedTarget);
      if (!seekSuccess) {
        // Media not ready yet — canplay will retry below.
        cloudSeekInFlightRef.current = false;
        return false;
      }
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

  // 应用续播进度：最终断点 = max(云端值, 本地值)。
  // 云端不可用（未登录/风控/离线）时 cloudValue 传 -1，仅依赖本地值。
  const applyResumeProgress = (cloudValue: number, requestId: number) => {
    const localValue = mediaKey ? readLocalResumePoint(mediaKey) : -1;
    const normalizedCloud = Number.isFinite(cloudValue) && cloudValue > 0 ? cloudValue : -1;
    const effective = Math.max(normalizedCloud, localValue);

    pendingCloudProgressRef.current = effective > 0 ? effective : null;
    cloudProgressPendingRef.current = false;
    const audio = audioRef.current;
    if (!audio) {
      markCloudProgressReady();
      return;
    }
    const applied = applyPendingCloudProgress(audio);
    // 如果无法立即应用进度（通常因为 readyState 不够），
    // 设置一个超时保护，防止永久阻塞播放
    if (!applied) {
      setTimeout(() => {
        if (requestId !== cloudProgressRequestIdRef.current) return;
        if (pendingCloudProgressRef.current !== null) {
          console.warn("[player] cloud progress apply timeout, giving up");
          pendingCloudProgressRef.current = null;
          markCloudProgressReady();
        }
      }, 3000); // 3 秒超时
    }
  };

  const reportCloudProgress = (
    progress: number,
    force = false,
    allowZero = false,
  ) => {
    const normalizedProgress = progress < 0 ? -1 : Math.max(0, Math.floor(progress));

    // 本地断点写入：复用同一上报节流点，不依赖云端是否可用。
    // 每 5 秒最多一次；暂停/切歌/卸载（force=true）时补写。
    if (normalizedProgress > 0 && mediaKey) {
      const now = Date.now();
      if (force || now - lastLocalResumeWriteRef.current >= LOCAL_RESUME_WRITE_INTERVAL_MS) {
        lastLocalResumeWriteRef.current = now;
        writeLocalResumePoint(mediaKey, normalizedProgress);
      }
    }

    if (
      !cloudHistoryEnabledRef.current ||
      !cloudProgressReadyRef.current ||
      !aid ||
      !cid ||
      !mediaKey
    ) return;

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
    })
      .then(() => {
        if (import.meta.env.DEV) {
          console.debug("[player] progress reported", {
            aid,
            cid,
            progress: normalizedProgress,
          });
        }
      })
      .catch((error) => {
        console.error("[player] report progress failed:", error);
      });
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

  // SponsorBlock 拉取：仅在开关开启且存在 bvid/cid 时发起；关闭时零请求。
  // 换曲（mediaKey 变化）时中止上一个请求并重置已跳过状态。
  useEffect(() => {
    sponsorSegmentsRef.current = [];
    sponsorSkipFiredRef.current = new Set();
    setSponsorSegmentsVersion((version) => version + 1);
    if (!sponsorSkip || !bvid || !cid) return;

    const controller = new AbortController();
    let cancelled = false;
    void fetchSegments(bvid, cid, controller.signal).then((segments) => {
      if (cancelled) return;
      sponsorSegmentsRef.current = segments;
      setSponsorSegmentsVersion((version) => version + 1);
    });
    return () => {
      cancelled = true;
      controller.abort();
    };
  }, [bvid, cid, mediaKey, sponsorSkip]);

  useEffect(() => {
    return () => {
      clearCloudSeekTimer();
      clearCloudProgressStartupTimer();
      reportCloudProgress(currentTimeRef.current, true);
    };
  }, [mediaKey]); // eslint-disable-line react-hooks/exhaustive-deps

  // 页面卸载 / 切到后台时补写一次本地断点（节流窗口之外的兜底）。
  useEffect(() => {
    const flushLocalResume = () => {
      reportCloudProgress(currentTimeRef.current, true);
    };
    const handleVisibility = () => {
      if (document.visibilityState === "hidden") flushLocalResume();
    };

    window.addEventListener("beforeunload", flushLocalResume);
    document.addEventListener("visibilitychange", handleVisibility);

    return () => {
      window.removeEventListener("beforeunload", flushLocalResume);
      document.removeEventListener("visibilitychange", handleVisibility);
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

    if (!mediaKey) {
      markCloudProgressReady();

      return;
    }

    // 未登录/云端不可用时，直接回退到本地断点。
    if (!cloudHistoryEnabled) {
      applyResumeProgress(-1, requestId);

      return;
    }

    setCloudProgressReadyKey("");
    cloudProgressPendingRef.current = true;
    cloudProgressReadyRef.current = false;
    cloudProgressStartupTimerRef.current = setTimeout(() => {
      if (requestId !== cloudProgressRequestIdRef.current) return;

      // Keep startup responsive on slow/offline networks. Invalidating the
      // request also prevents a late response from jumping an already-playing
      // track. Cloud budget exceeded → 仍回退到本地断点。
      cloudProgressRequestIdRef.current += 1;
      applyResumeProgress(-1, cloudProgressRequestIdRef.current);
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
        applyResumeProgress(Math.max(0, progress || 0), requestId);
      })
      .catch((error) => {
        if (requestId !== cloudProgressRequestIdRef.current) return;

        console.error("[player] get cloud progress failed:", error);
        clearCloudProgressStartupTimer();
        // 云端失败（未登录/风控/离线）也要 fallthrough 到本地断点。
        applyResumeProgress(-1, requestId);
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
      void (async () => {
        if (playAttemptId !== playAttemptIdRef.current) return;

        // 确保 AudioContext 在播放前恢复
        const graph = audioGraphRef.current;
        if (graph) {
          try {
            if (graph.ctx.state === "suspended") {
              await graph.ctx.resume();
            }
            if (playAttemptId !== playAttemptIdRef.current) return;
          } catch (error) {
            console.error("[player] AudioContext resume failed:", error);
          }
        }

        // 播放音频
        try {
          await audio.play();
          if (playAttemptId !== playAttemptIdRef.current) return;
        } catch (error) {
          if (playAttemptId !== playAttemptIdRef.current) return;
          console.error("[player] audio.play() failed:", error);
          onPlayStateChange?.(false);
        }
      })();
    } else {
      if (!audio.paused) {
        audio.pause();
      }
    }
    return () => {
      // A source switch cannot cancel an in-flight play() promise. Invalidate
      // the attempt and pause the element so only the newest effect may start
      // audible playback.
      playAttemptIdRef.current += 1;
      if (!audio.paused) audio.pause();
    };
  }, [cloudHistoryEnabled, cloudProgressReadyKey, forcePause, isPlaying, mediaKey, src]);

  // 单曲循环：音频结束后由 replaySignal 触发，seek 回 0 继续播。
  // 走 safeSeek 保护路径（元数据未就绪时直接 play 当前尾部会被挡）。
  useEffect(() => {
    if (replaySignal <= 0) return;
    const audio = audioRef.current;
    if (!audio || !src) return;

    const replay = () => {
      if (safeSeek(audio, 0)) {
        void audio.play().catch((error) => {
          console.error("[player] single-loop replay failed:", error);
        });
      } else {
        // 数据暂未就绪：等 canplay 后再重试一次
        const onReady = () => {
          audio.removeEventListener("canplay", onReady);
          if (safeSeek(audio, 0)) {
            void audio.play().catch(() => {});
          }
        };
        audio.addEventListener("canplay", onReady, { once: true });
      }
    };

    replay();
    // 仅在信号变化时重播，不依赖其它 props
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [replaySignal]);

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

  // SponsorBlock 跳过判定：仅在开关开启、有片段、且当前自然播放跨入时跳一次。
  // 每个 segment 只跳一次（记录索引）；用户手动 seek 回该区间不重复自动跳，
  // 避免「跳不出」死循环。换曲时由拉取 effect 重置已跳过集合。
  const maybeSkipSponsor = (audio: HTMLAudioElement) => {
    if (!sponsorSkip) return;
    const segments = sponsorSegmentsRef.current;
    if (segments.length === 0) return;

    const current = audio.currentTime;
    const mediaDuration = audio.duration;
    const hasDuration = Number.isFinite(mediaDuration) && mediaDuration > 0;

    for (let i = 0; i < segments.length; i += 1) {
      if (sponsorSkipFiredRef.current.has(i)) continue;
      const seg = segments[i];
      const [start, end] = seg.segment;
      // 过期校验：时长就绪且与提交时长偏差过大则丢弃，防止时间线错位误跳。
      if (
        hasDuration &&
        seg.videoDuration > 0 &&
        Math.abs(mediaDuration - seg.videoDuration) >
          SPONSOR_DURATION_TOLERANCE_SECONDS
      ) {
        sponsorSkipFiredRef.current.add(i);
        continue;
      }
      if (start >= mediaDuration && hasDuration) continue;
      if (current >= start - SPONSOR_SEGMENT_LEAD_SECONDS && current < end) {
        const target = end + SPONSOR_SEEK_PAD_SECONDS;
        if (safeSeek(audio, target)) {
          sponsorSkipFiredRef.current.add(i);
          currentTimeRef.current = target;
          const now = Date.now();
          if (now - sponsorToastAtRef.current >= SPONSOR_TOAST_MIN_INTERVAL_MS) {
            sponsorToastAtRef.current = now;
            toast({ type: "info", content: "已跳过恰饭片段" });
          }
        }
        return;
      }
    }
  };

  const handleTimeUpdate = () => {
    const audio = audioRef.current;
    if (!audio) return;

    if (pendingCloudProgressRef.current !== null) return;

    currentTimeRef.current = audio.currentTime;
    maybeSkipSponsor(audio);
    if (!isSeekingRef.current) {
      updateSeekPreviewUi(
        timelineRef.current,
        currentTimeLabelRef.current,
        audio.currentTime,
        duration,
      );
    }
    updateBufferedUi(bufferedRef.current, audio, duration);
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
      seekBubbleRef.current,
    );
  };

  // hover 时间气泡：鼠标悬停轨道时显示对应时间，coarse pointer 下不启用。
  const handleTimelineHover = (event: ReactPointerEvent<HTMLInputElement>) => {
    if (event.pointerType === "touch") return;
    const bubble = seekBubbleRef.current;
    if (!bubble || !duration) return;
    const rect = event.currentTarget.getBoundingClientRect();
    if (rect.width <= 0) return;
    const ratio = Math.min(Math.max((event.clientX - rect.left) / rect.width, 0), 1);
    const time = ratio * duration;
    bubble.textContent = formatTime(time);
    bubble.style.left = `${ratio * 100}%`;
    bubble.dataset.visible = "true";
  };

  const hideTimelineHover = () => {
    const bubble = seekBubbleRef.current;
    if (!bubble || isSeekingRef.current) return;
    bubble.dataset.visible = "false";
  };

  useEffect(() => {
    const resetPointerSeek = (restorePlaybackPosition = true) => {
      isSeekingRef.current = false;
      seekPointerIdRef.current = null;
      seekPreviewRef.current = null;
      if (seekBubbleRef.current) {
        seekBubbleRef.current.dataset.visible = "false";
      }
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

  // 音量偏好持久化：localStorage 为主，dkv 同步备份（跟随 coverMode 双写范式）。
  // 仅在拖动提交/静音切换时写，不跟随每次 input 事件。
  const persistVolume = (value: number) => {
    localStorage.setItem(VOLUME_STORAGE_KEY, String(value));
    invoke("set_kv", { key: VOLUME_STORAGE_KEY, value: String(value) }).catch(() => {});
  };

  const persistPlaybackRate = (value: number) => {
    localStorage.setItem(PLAYBACK_RATE_STORAGE_KEY, String(value));
    invoke("set_kv", { key: PLAYBACK_RATE_STORAGE_KEY, value: String(value) }).catch(() => {});
  };

  const handleLoadedMetadata = (audio: HTMLAudioElement) => {
    metadataMediaKeyRef.current = mediaKey;
    audio.volume = volume;
    audio.muted = volume === 0;
    audio.playbackRate = playbackRate;
    updateBufferedUi(bufferedRef.current, audio, audio.duration || 0);
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
      // During a source switch React still wants playback, but the element has
      // not entered the playing state yet. Suspending the graph at that point
      // races with the play effect and leaves currentTime advancing silently.
      const isPlaybackDesired = Boolean(src) && isPlaying && !forcePause;
      if (audio.paused && !isPlaybackDesired && graph.ctx.state === "running") {
        void graph.ctx.suspend();
      } else if (!audio.paused && graph.ctx.state === "suspended") {
        void graph.ctx.resume();
      }
    }
  };

  const handleCanPlay = (audio: HTMLAudioElement) => {
    if (!cloudProgressPendingRef.current && pendingCloudProgressRef.current !== null) {
      const applied = applyPendingCloudProgress(audio);
      // 如果 canplay 时仍然无法应用进度（通常不应该发生），
      // 放弃云端进度，让播放继续
      if (!applied && audio.readyState >= HTMLMediaElement.HAVE_FUTURE_DATA) {
        if (import.meta.env.DEV) {
          console.warn("[player] canplay: giving up on cloud progress", {
            readyState: audio.readyState,
            pendingProgress: pendingCloudProgressRef.current,
          });
        }
        pendingCloudProgressRef.current = null;
        markCloudProgressReady();
      }
    }
    // Re-attempt any cloud seek that was deferred because the media was not ready.
    if (pendingCloudProgressRef.current !== null) {
      const audio_ = audioRef.current;
      if (audio_ && applyPendingCloudProgress(audio_)) {
        return;
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
    updateBufferedUi(bufferedRef.current, audio, nextDuration);
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

  // 恰饭跳过开关：localStorage 为主，dkv 同步备份（跟随音量/倍速双写范式）。
  const toggleSponsorSkip = () => {
    const newEnabled = !sponsorSkip;
    setSponsorSkip(newEnabled);
    // 隐私模式 Safari 等环境下 localStorage.setItem 会抛（QuotaExceededError /
    // SecurityError），不能让它冒泡打断点击处理。
    try {
      localStorage.setItem(SPONSOR_SKIP_STORAGE_KEY, String(newEnabled));
    } catch (error) {
      console.error("保存恰饭跳过开关到本地存储失败:", error);
      toast({ type: "warning", content: "开关状态保存失败" });
    }
    invoke("set_kv", {
      key: SPONSOR_SKIP_STORAGE_KEY,
      value: String(newEnabled),
    }).catch((error) => {
      // 状态写失败必须可见：不再静默吞掉，否则用户会以为按钮没反应。
      console.error("同步恰饭跳过开关失败:", error);
      toast({ type: "warning", content: "开关状态保存失败" });
    });
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
          // 注意：这里不能把播放状态置回 false —— 自动连播依赖 isPlaying
          // 跨切歌保持 true（playUrl 变化后由下方 play effect 拉起下一曲）。
          // 「没有下一曲时归位」由 handleVideoEnded 的无下一曲分支负责。
          onEnded?.();
        }}
        onError={(event) => {
          const error = event.currentTarget.error;
          console.error("音频加载失败:", error);
          suspendAudioGraph();
          onPlayStateChange?.(false);
          onError?.(error);
        }}
        onLoadedMetadata={(event) => handleLoadedMetadata(event.currentTarget)}
        onProgress={(event) =>
          updateBufferedUi(bufferedRef.current, event.currentTarget, duration)
        }
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
        <div className="player-transport-controls" role="group" aria-label="播放控制">
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

          <button
            aria-label="下一个视频"
            className="player-button player-next-button"
            disabled={!src || !onNext || !canNext}
            title="下一个视频"
            type="button"
            onClick={onNext}
          >
            <GoEnd fill="currentColor" size={18} theme="outline" />
          </button>
        </div>

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
          <span
            aria-hidden="true"
            className="player-timeline-buffered"
            data-available="false"
            ref={bufferedRef}
          />
          <span
            aria-hidden="true"
            className="player-timeline-sponsor"
            data-available={
              sponsorSkip && sponsorSegmentsRef.current.length > 0 ? "true" : "false"
            }
            data-version={sponsorSegmentsVersion}
            ref={sponsorMarkerRef}
          >
            {sponsorSkip && duration > 0
              ? sponsorSegmentsRef.current.map((seg, index) => {
                  const mediaDuration = duration;
                  if (!Number.isFinite(mediaDuration) || mediaDuration <= 0) return null;
                  if (
                    seg.videoDuration > 0 &&
                    Math.abs(mediaDuration - seg.videoDuration) >
                      SPONSOR_DURATION_TOLERANCE_SECONDS
                  ) {
                    return null;
                  }
                  const [start, end] = seg.segment;
                  const left = Math.max(0, Math.min(100, (start / mediaDuration) * 100));
                  const right = Math.max(0, Math.min(100, (end / mediaDuration) * 100));
                  if (right <= left) return null;
                  return (
                    <i
                      // 广告段为纯视觉标记，索引即稳定 key（同曲内不变）。
                      key={index}
                      className="player-timeline-sponsor-segment"
                      style={{ left: `${left}%`, width: `${right - left}%` }}
                    />
                  );
                })
              : null}
          </span>
          <div
            aria-hidden="true"
            className="player-seek-bubble"
            data-visible="false"
            ref={seekBubbleRef}
          >
            0:00
          </div>
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
            onPointerEnter={handleTimelineHover}
            onPointerMove={handleTimelineHover}
            onPointerLeave={hideTimelineHover}
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
              handleTimelineHover(event);
            }}
          />
        </div>

        <time className="player-time" dateTime={`PT${Math.floor(duration)}S`}>
          {formatTime(duration)}
        </time>

        <div className="player-volume" ref={volumePopoverRef}>
          <button
            aria-expanded={isVolumeOpen}
            aria-label="音量"
            className="player-button player-volume-button"
            data-open={isVolumeOpen || undefined}
            disabled={!src}
            title="音量"
            type="button"
            onClick={() => setIsVolumeOpen((open) => !open)}
          >
            {volume > 0 ? (
              <VolumeNotice fill="currentColor" size={20} theme="outline" />
            ) : (
              <VolumeMute fill="currentColor" size={20} theme="outline" />
            )}
          </button>
          {isVolumeOpen && (
            <div className="player-volume-popover">
              <button
                aria-label={volume > 0 ? "静音" : "取消静音"}
                aria-pressed={volume === 0}
                className="player-button player-volume-mute-button"
                data-active={volume === 0 || undefined}
                title={volume > 0 ? "静音" : "取消静音"}
                type="button"
                onClick={() => {
                  if (volume === 0) {
                    const restored = lastVolumeRef.current || 1;
                    handleVolumeChange(restored);
                    persistVolume(restored);
                  } else {
                    handleVolumeChange(0);
                    persistVolume(0);
                  }
                }}
              >
                {volume > 0 ? (
                  <VolumeNotice fill="currentColor" size={16} theme="outline" />
                ) : (
                  <VolumeMute fill="currentColor" size={16} theme="outline" />
                )}
              </button>
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
                onPointerUp={(event) =>
                  persistVolume(Number(event.currentTarget.value))
                }
                onKeyUp={(event) =>
                  persistVolume(Number(event.currentTarget.value))
                }
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
                    persistPlaybackRate(rate);
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

        <button
          aria-label="自动跳过恰饭片段（SponsorBlock）"
          aria-pressed={sponsorSkip}
          className="player-button player-sponsor-button"
          data-active={sponsorSkip || undefined}
          title="自动跳过恰饭片段（SponsorBlock）"
          type="button"
          onClick={toggleSponsorSkip}
        >
          <Ad fill="currentColor" size={17} theme="outline" />
        </button>
      </div>
    </div>
  );
};

export default Player;
