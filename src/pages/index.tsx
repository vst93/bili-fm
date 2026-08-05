import { lazy, Suspense, useState, useEffect, useMemo, useRef } from "react";
import { CloseSmall } from "@icon-park/react";
import { invoke } from "@tauri-apps/api/core";
import { open } from "@tauri-apps/plugin-shell";
import { load } from "@tauri-apps/plugin-store";
import QRCode from "qrcode";

import { toast } from "../utils/toast";
import type * as BL from "@/types/bilibili";

import SearchForm from "@/components/searchForm";
import VideoCover from "@/components/videoCover";
import VideoInfo from "@/components/videoInfo";
import Player from "@/components/player";
import DefaultLayout from "@/layouts/default";
import TitleBar from "@/components/titleBar";
import { graftingImage, urlToBVID } from "@/utils/string";
import PlayerVideo from "@/components/playerVideo";
import MiniVideoInfo from "@/components/miniVideoInfo";
import type {
  PlaylistItem,
  PlaylistPlayMode,
} from "@/components/playlist";

const loadPageList = () => import("@/components/pageList");
const loadSearchList = () => import("@/components/searchList");
const loadFeedList = () => import("@/components/feedList");
const loadRecommendList = () => import("@/components/recommendList");
const loadCollectList = () => import("@/components/collectList");
const loadUpVideoList = () => import("@/components/upVideoList");
const loadHistoryList = () => import("@/components/historyList");
const loadSeriesList = () => import("@/components/seriesList");
const loadDanmakuList = () => import("@/components/danmakuList");
const loadPlaylist = () => import("@/components/playlist");

const drawerLoaders = [
  loadPageList,
  loadSearchList,
  loadFeedList,
  loadRecommendList,
  loadCollectList,
  loadUpVideoList,
  loadHistoryList,
  loadSeriesList,
  loadDanmakuList,
  loadPlaylist,
];

const PageList = lazy(loadPageList);
const SearchList = lazy(loadSearchList);
const FeedList = lazy(loadFeedList);
const RecommendList = lazy(loadRecommendList);
const CollectList = lazy(loadCollectList);
const UpVideoList = lazy(loadUpVideoList);
const HistoryList = lazy(loadHistoryList);
const SeriesList = lazy(loadSeriesList);
const DanmakuList = lazy(loadDanmakuList);
const Playlist = lazy(loadPlaylist);

const MAX_RETAINED_LIST_ITEMS = 240;

export default function IndexPage() {
  const [showPageList, setShowPageList] = useState(false);
  const [showSearchList, setShowSearchList] = useState(false);
  const [showFeedList, setShowFeedList] = useState(false);
  const [pageNum, setPageNum] = useState(0);
  const [searchResults, setSearchResults] = useState<BL.SearchResult[]>(
    [],
  );
  const [currentBvid, setCurrentBvid] = useState("");
  const [currentKeyword, setCurrentKeyword] = useState("");
  const [searchInputValue, setSearchInputValue] = useState("");
  const searchRequestIdRef = useRef(0);
  const [videoInfo, setVideoInfo] = useState<BL.VideoInfo | undefined>();
  // playingInfo: 正在播放的视频信息 (与 videoInfo 分离)
  // videoInfo 随浏览操作(列表点击/handleUrlJump)立即更新, 但此时未必开始播放。
  // playingInfo 仅在真正开始播放时(handleVideoSelect/handlePlaylistVideoSelect)设置,
  // 显示层用它而非 videoInfo, 避免"点了列表但还没播放, 首页信息就变了"。
  const [playingInfo, setPlayingInfo] = useState<BL.VideoInfo | undefined>();
  const [playUrl, setPlayUrl] = useState<string>("");
  const [isLoudnessEq, setIsLoudnessEq] = useState(false);
  const [currentPart, setCurrentPart] = useState<string>("");
  const [currentIndex, setCurrentIndex] = useState<number>(0);
  const [pageFirstFrame, setPageFirstFrame] = useState<string>("");
  const [isPlaying, setIsPlaying] = useState<boolean>(false);
  const [isPlayVideo, setIsPlayVideo] = useState(false);
  const [isPlayVideoStop, setIsPlayVideoStop] = useState(true);
  const [showLoginPanel, setShowLoginPanel] = useState(false);
  const [qrCodeUrl, setQrCodeUrl] = useState("");
  const [userFace, setUserFace] = useState("");
  const [feedList, setFeedList] = useState<BL.FeedList>();
  const [feedOffset, setFeedOffset] = useState("");
  const [showRecommendList, setShowRecommendList] = useState(false);
  const [recommendList, setRecommendList] = useState<any>();
  const [recommendPage, setRecommendPage] = useState(1);
  const [hotList, setHotList] = useState<any>();
  const [hotPage, setHotPage] = useState(1);
  const [showCollectList, setShowCollectList] = useState(false);
  const [collectList, setCollectList] = useState<any>();
  const [collectGroups, setCollectGroups] = useState<any[]>([]);
  const [currentGroupId, setCurrentGroupId] = useState<number>();
  const [collectPage, setCollectPage] = useState(1);
  const [showUpVideoList, setShowUpVideoList] = useState(false);
  const [upVideoList, setUpVideoList] = useState<BL.FeedList>();
  const [upVideoOffset, setUpVideoOffset] = useState("");
  const [currentUpMid, setCurrentUpMid] = useState(0);
  const [currentUpName, setCurrentUpName] = useState("");
  const [showHistoryList, setShowHistoryList] = useState(false);
  const [historyList, setHistoryList] = useState<any>();
  const [historyCursor, setHistoryCursor] = useState<{
    max: number;
    view_at: number;
    business: string;
  }>({ max: 0, view_at: 0, business: "" });
  const [seriesList, setSeriesList] = useState<any[]>([]);
  const [currentSeriesId, setCurrentSeriesId] = useState<number>(0);
  const [seriesVideos, setSeriesVideos] = useState<any[]>([]);
  const [showSeriesList, setShowSeriesList] = useState(false);
  const [currentSeriesTitle, setCurrentSeriesTitle] = useState("");
  const [seriesVideosPage, setSeriesVideosPage] = useState(1);
  const [isMiniMode, setIsMiniMode] = useState(false);
  // Linux 下 webkit2gtk 在 frameless + DisableResize 模式下无法运行时调整窗口大小，
  // 迷你模式会导致内容缩到左上角但窗口不变，因此 Linux 上禁用迷你模式
  const [isLinux, setIsLinux] = useState(false);
  const [showDanmakuList, setShowDanmakuList] = useState(false);
  const [danmakuList, setDanmakuList] = useState<BL.DanmakuList>();
  const [isLoadingDanmaku, setIsLoadingDanmaku] = useState(false);
  const [danmakuCid, setDanmakuCid] = useState<number>(0);
  const [replyList, setReplyList] = useState<BL.ReplyList>();
  const [isLoadingReply, setIsLoadingReply] = useState(false);
  const [replyOid, setReplyOid] = useState<number>(0);
  const [replyPage, setReplyPage] = useState(1);
  const [currentVideoTime, setCurrentVideoTime] = useState(0);
  const [playlist, setPlaylist] = useState<PlaylistItem[]>([]);
  const [currentPlaylistIndex, setCurrentPlaylistIndex] = useState<number>(-1);
  const [seriesPlaylist, setSeriesPlaylist] = useState<PlaylistItem[]>([]);
  const [currentSeriesPlaylistIndex, setCurrentSeriesPlaylistIndex] =
    useState<number>(-1);
  const [activePlaylistType, setActivePlaylistType] =
    useState<"user" | "series">("user");
  const [playlistPlayMode, setPlaylistPlayMode] =
    useState<PlaylistPlayMode>("sequence");
  const [isPlaylistMode, setIsPlaylistMode] = useState<boolean>(false);
  const [showPlaylist, setShowPlaylist] = useState<boolean>(false);

  const playlistCids = useMemo(
    () => new Set(playlist.map((p) => p.cid)),
    [playlist],
  );

  const playlistLoadedRef = useRef(false);
  const playlistStoreRef = useRef<Awaited<ReturnType<typeof load>> | null>(
    null,
  );
  const loginPollTimerRef = useRef<ReturnType<typeof setTimeout> | undefined>(undefined);
  const feedLoadMoreRef = useRef(false);
  const recommendRequestRef = useRef(new Set<string>());
  const collectLoadMoreRef = useRef(false);
  const upVideoLoadMoreRef = useRef(false);
  const mediaNavigationRef = useRef({
    previous: () => {},
    next: () => {},
  });

  useEffect(() => {
    return () => {
      if (loginPollTimerRef.current) clearTimeout(loginPollTimerRef.current);
    };
  }, []);

  useEffect(() => {
    const preloadTimer = setTimeout(() => {
      void Promise.all(drawerLoaders.map((loadDrawer) => loadDrawer())).catch(() => {});
    }, 0);

    return () => clearTimeout(preloadTimer);
  }, []);

  useEffect(() => {
    document.body.classList.toggle("mini-mode", isMiniMode);

    return () => {
      document.body.classList.remove("mini-mode");
    };
  }, [isMiniMode]);

  useEffect(() => {
    if (!showFeedList) {
      setFeedList(undefined);
      setFeedOffset("");
    }
    if (!showRecommendList) {
      setRecommendList(undefined);
      setHotList(undefined);
      setRecommendPage(1);
      setHotPage(1);
    }
    if (!showCollectList) {
      setCollectList(undefined);
      setCollectPage(1);
    }
    if (!showUpVideoList) {
      setUpVideoList(undefined);
      setUpVideoOffset("");
      setSeriesList([]);
    }
    if (!showHistoryList) {
      setHistoryList(undefined);
      setHistoryCursor({ max: 0, view_at: 0, business: "" });
    }
    if (!showSeriesList) {
      setSeriesVideos([]);
      setSeriesVideosPage(1);
    }
    if (!showDanmakuList) {
      setDanmakuList(undefined);
      setReplyList(undefined);
      setCurrentVideoTime(0);
      setReplyPage(1);
    }
  }, [
    showCollectList,
    showDanmakuList,
    showFeedList,
    showHistoryList,
    showRecommendList,
    showSeriesList,
    showUpVideoList,
  ]);

  // 音频/视频互斥：打开视频浮窗时停止音频
  // 直接操作 DOM <audio> 元素作为双保险，确保音频真的停了
  useEffect(() => {
    if (isPlayVideo) {
      setIsPlaying(false);
      // Force pause the audio element immediately
      const audioEl = document.querySelector<HTMLAudioElement>("#player audio");
      if (audioEl && !audioEl.paused) {
        audioEl.pause();
      }
    }
  }, [isPlayVideo]);

  // 切换播放源时关闭视频浮窗，避免新音频和旧视频同时播放
  useEffect(() => {
    if (isPlayVideo && playUrl) {
      setIsPlayVideo(false);
      setIsPlayVideoStop(true);
    }
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [playUrl]);

  // 音频开始播放时确保视频浮窗已关闭
  useEffect(() => {
    if (isPlaying && isPlayVideo) {
      setIsPlayVideo(false);
      setIsPlayVideoStop(true);
    }
  }, [isPlaying, isPlayVideo]);

  useEffect(() => {
    // 检测平台，Linux 下禁用迷你模式
    invoke<string>("get_platform").then((platform: string) => {
      setIsLinux(platform === "linux");
      // Linux: 窗口管理器不支持外层圆角，去掉 #root 及所有使用 --app-window-radius 的圆角
      document.body.classList.toggle("platform-linux", platform === "linux");
      document.body.classList.toggle("platform-windows", platform === "windows");
      document.body.classList.toggle("platform-darwin", platform === "darwin");
    });
    // 初始化时获取用户信息
    refreshUserInfo();
    invoke<string>("get_playlist_play_mode").then((mode) => {
      if (mode === "shuffle" || mode === "sequence") {
        setPlaylistPlayMode(mode);
      }
    });
  }, []);

  // 从本地加载播放列表和播放模式
  useEffect(() => {
    let cancelled = false;

    (async () => {
      try {
        const store = await load("playlist.json", { autoSave: false });
        const [
          savedUserPlaylist,
          savedSeriesPlaylist,
          savedIndex,
          savedSeriesIndex,
        ] = await Promise.all([
          store.get<PlaylistItem[]>("userPlaylist"),
          store.get<PlaylistItem[]>("seriesPlaylist"),
          store.get<number>("currentPlaylistIndex"),
          store.get<number>("currentSeriesPlaylistIndex"),
        ]);

        if (cancelled) return;
        playlistStoreRef.current = store;
        if (savedUserPlaylist) {
          setPlaylist(savedUserPlaylist);
        } else {
          try {
            const json = await invoke<string>("get_playlist");
            if (json) setPlaylist(JSON.parse(json));
          } catch (e) {
            console.error("加载播放列表失败:", e);
          }
        }
        if (savedSeriesPlaylist) setSeriesPlaylist(savedSeriesPlaylist);
        if (savedIndex !== null && savedIndex !== undefined) {
          setCurrentPlaylistIndex(savedIndex);
        }
        if (savedSeriesIndex !== null && savedSeriesIndex !== undefined) {
          setCurrentSeriesPlaylistIndex(savedSeriesIndex);
        }
        playlistLoadedRef.current = true;
      } catch (e) {
        console.error("Failed to load playlist:", e);
      }
    })();

    return () => {
      cancelled = true;
    };
  }, []);

  // 播放列表变更时自动持久化（初始加载完成后才生效）
  useEffect(() => {
    const store = playlistStoreRef.current;
    if (!playlistLoadedRef.current || !store) return;

    (async () => {
      try {
        await store.set("userPlaylist", playlist);
        await store.set("currentPlaylistIndex", currentPlaylistIndex);
        await store.set("seriesPlaylist", seriesPlaylist);
        await store.set(
          "currentSeriesPlaylistIndex",
          currentSeriesPlaylistIndex,
        );
        await store.save();
      } catch (e) {
        console.error("Failed to save playlist:", e);
      }
    })();
  }, [
    playlist,
    currentPlaylistIndex,
    seriesPlaylist,
    currentSeriesPlaylistIndex,
  ]);

  // 播放模式变更时自动持久化
  useEffect(() => {
    invoke("set_playlist_play_mode", { mode: playlistPlayMode });
  }, [playlistPlayMode]);

  /**
   * 蓝牙/系统媒体控制事件处理
   * @description 监听系统媒体控制事件（蓝牙耳机、键盘多媒体键等），同步更新播放状态
   */
  useEffect(() => {
    // 尝试使用 Media Session API（现代浏览器支持）
    if ("mediaSession" in navigator) {
      navigator.mediaSession.setActionHandler("play", () => {
        setIsPlaying(true);
      });

      navigator.mediaSession.setActionHandler("pause", () => {
        setIsPlaying(false);
      });

      navigator.mediaSession.setActionHandler("previoustrack", () => {
        mediaNavigationRef.current.previous();
      });

      navigator.mediaSession.setActionHandler("nexttrack", () => {
        mediaNavigationRef.current.next();
      });
    }

    // 监听键盘多媒体键（作为备用方案）
    const handleMediaKeyPress = (event: KeyboardEvent) => {
      // 跳过输入框中的按键
      if (
        event.target instanceof HTMLInputElement ||
        event.target instanceof HTMLTextAreaElement
      ) {
        return;
      }

      if (event.code === "MediaPlayPause" || event.code === "MediaStop") {
        event.preventDefault();
        setIsPlaying((prev) => !prev);
      } else if (event.code === "MediaTrackPrevious") {
        event.preventDefault();
        mediaNavigationRef.current.previous();
      } else if (event.code === "MediaTrackNext") {
        event.preventDefault();
        mediaNavigationRef.current.next();
      }
    };

    window.addEventListener("keydown", handleMediaKeyPress);

    return () => {
      window.removeEventListener("keydown", handleMediaKeyPress);
      // 清理 Media Session handlers
      if ("mediaSession" in navigator) {
        navigator.mediaSession.setActionHandler("play", null);
        navigator.mediaSession.setActionHandler("pause", null);
        navigator.mediaSession.setActionHandler("previoustrack", null);
        navigator.mediaSession.setActionHandler("nexttrack", null);
      }
    };
  }, []);

  /**
   * 同步 Media Session 播放状态
   * @description 当播放状态变化时，同步更新系统媒体控制中心的显示状态
   */
  useEffect(() => {
    if ("mediaSession" in navigator) {
      navigator.mediaSession.playbackState = isPlaying ? "playing" : "paused";
    }
  }, [isPlaying]);

  /**
   * 键盘事件处理函数
   * @param event 键盘事件对象
   * @description 处理空格键（播放/暂停）和左右方向键（上一个/下一个视频）的按键事件
   */
  useEffect(() => {
    // switchWindowMode()
    //监听快捷键
    const listener = (event: KeyboardEvent) => {
      // 在keydown阶段就阻止空格键对按钮的触发
      if (
        event.code === "Space" &&
        (event.target instanceof HTMLButtonElement ||
          event.target instanceof HTMLInputElement ||
          event.target instanceof HTMLTextAreaElement)
      ) {
        event.preventDefault();

        return;
      }

      if ((event.metaKey || event.ctrlKey) && event.key === "w") {
        invoke("minimize_window");
      } else if ((event.metaKey || event.ctrlKey) && event.key === "q") {
        invoke("quit_app");
      }
    };

    // 专门拦截按钮空格键的函数，在捕获阶段执行
    const handleSpaceKeyIntercept = (event: KeyboardEvent) => {
      if (event.code === "Space") {
        // 只对按钮元素阻止空格键的默认行为，不输入框不拦截
        if (event.target instanceof HTMLButtonElement) {
          event.preventDefault();
          event.stopPropagation();

          return false;
        }
      }
    };

    //监听键盘事件
    const handleKeyPress = (event: KeyboardEvent) => {
      // 排除输入元素，让它们正常处理空格键
      if (
        event.target instanceof HTMLInputElement ||
        event.target instanceof HTMLTextAreaElement
      ) {
        return;
      }

      // 按钮元素也允许空格键触发播放控制（焦点管理已在点击时处理）
      // if (event.target instanceof HTMLButtonElement) {
      //   return;
      // }
      if (isPlayVideo) {
        if (event.code === "Escape" && !event.repeat) {
          event.preventDefault();
          setIsPlayVideo(!isPlayVideo);
        } else if (event.code === "Space" && !event.repeat) {
          event.preventDefault();
          setIsPlayVideoStop(!isPlayVideoStop);
        }

        // 播放视频时屏蔽快捷键
        return;
      }
      if (event.code === "Space" && !event.repeat) {
        event.preventDefault();
        //如果当前对象为 div id = video-cover ，阻止
        if (
          event.target instanceof HTMLDivElement &&
          event.target.id === "video-cover"
        ) {
        } else {
          setIsPlaying((prev) => !prev);
        }
      } else if (event.code === "ArrowLeft" && !event.repeat) {
        event.preventDefault();
        handlePrevTrack();
      } else if (event.code === "ArrowRight" && !event.repeat) {
        event.preventDefault();
        handleNextTrack();
      }
    };

    // 在捕获阶段拦截空格键，优先于按钮的默认行为
    window.addEventListener("keydown", handleSpaceKeyIntercept, true);
    window.addEventListener("keyup", handleKeyPress);
    window.addEventListener("keydown", listener);

    return () => {
      window.removeEventListener("keydown", handleSpaceKeyIntercept, true);
      window.removeEventListener("keyup", handleKeyPress);
      window.removeEventListener("keydown", listener);
    };
  }, [
    videoInfo,
    currentIndex,
    isPlayVideo,
    isPlayVideoStop,
    isPlaylistMode,
    playlist,
    currentPlaylistIndex,
    seriesPlaylist,
    currentSeriesPlaylistIndex,
    activePlaylistType,
    playlistPlayMode,
  ]);

  // 处理按钮焦点问题 - 点击按钮后立即移除焦点
  useEffect(() => {
    const handleButtonClick = (event: MouseEvent) => {
      const target = event.target as HTMLElement;

      if (target instanceof HTMLButtonElement) {
        // 延迟移除焦点，确保按钮的点击事件处理完成
        setTimeout(() => {
          target.blur();
          // 或者将焦点移到body
          document.body.focus();
        }, 0);
      }
    };

    // 使用捕获阶段监听所有点击事件
    document.addEventListener("click", handleButtonClick, true);

    return () => {
      document.removeEventListener("click", handleButtonClick, true);
    };
  }, []);

  /**
   * 处理登录按钮点击事件
   * @description 显示登录面板，获取登录二维码，并开始轮询登录状态
   */
  const handleLogin = async () => {
    try {
      if (loginPollTimerRef.current) clearTimeout(loginPollTimerRef.current);
      await invoke("set_login_status", { status: true });
      setShowLoginPanel(true);
      const qrcodeUrl = await invoke<string>("get_login_qrcode");
      // 本地生成二维码 (data: URL)，避免依赖外部 API 被 CSP 拦截
      const dataUrl = await QRCode.toDataURL(qrcodeUrl, {
        width: 200,
        margin: 1,
      });
      setQrCodeUrl(dataUrl);
      loopLoginStatus();
    } catch (error) {
      console.error("登录失败:", error);
    }
  };

  /**
   * 轮询检查登录状态
   * @description 每2秒检查一次登录状态，直到用户扫码成功或关闭登录面板
   */
  const loopLoginStatus = async () => {
    try {
      const status = await invoke<boolean>("get_login_status");

      if (!status) {
        console.log("已关闭登录页面");

        return;
      }

      const qrCodeStatus = await invoke<boolean>("get_login_qrcode_status");

      if (qrCodeStatus) {
        console.log("扫码成功");
        setShowLoginPanel(false);
        refreshUserInfo();
      } else {
        loginPollTimerRef.current = setTimeout(loopLoginStatus, 2000);
      }
    } catch (error) {
      console.error("获取登录状态失败:", error);
    }
  };

  /**
   * 刷新用户信息
   * @description 获取用户信息并更新头像
   */
  const refreshUserInfo = async () => {
    try {
      const userInfo = await invoke<BL.UserInfo | null>("get_user_info");

      if (userInfo?.face) {
        const processedFace = graftingImage(userInfo.face, 96);

        setUserFace(processedFace);
      }
    } catch (error) {
      console.error("获取用户信息失败:", error);
    }
  };

  /**
   * 处理关闭登录面板事件
   * @description 关闭登录面板并重置登录状态
   */
  const handleCloseLogin = async () => {
    if (loginPollTimerRef.current) {
      clearTimeout(loginPollTimerRef.current);
      loginPollTimerRef.current = undefined;
    }
    await invoke("set_login_status", { status: false });
    setShowLoginPanel(false);
  };

  /**
   * 处理动态按钮点击事件
   * @description 获取并显示用户关注的UP主的动态列表
   */
  const handleFeedClick = async () => {
    try {
      const data = await invoke<BL.FeedList>("get_feed_list", {
        offset: feedOffset,
      });

      setFeedList(data);
      setShowFeedList(true);
      setShowSearchList(false);
      setShowPageList(false);
    } catch (error) {
      console.error("获取动态列表失败:", error);
    }
  };

  /**
   * 处理动态列表刷新事件
   * @description 重置偏移量并重新获取动态列表
   */
  const handleFeedRefresh = async () => {
    try {
      setFeedOffset("");
      const data = await invoke<BL.FeedList>("get_feed_list", { offset: "" });

      setFeedList(data);
    } catch (error) {
      console.error("刷新动态列表失败:", error);
    }
  };

  /**
   * 处理动态列表加载更多事件
   * @param offset 下一页的偏移量
   * @description 根据偏移量加载更多动态内容
   */
  const handleLoadMore = async (offset: string) => {
    if (
      feedLoadMoreRef.current ||
      (feedList?.items?.length || 0) >= MAX_RETAINED_LIST_ITEMS
    ) return;
    feedLoadMoreRef.current = true;
    try {
      const data = await invoke<BL.FeedList>("get_feed_list", { offset });

      if (data?.items) {
        setFeedList((current) => ({
          ...data,
          items: [...(current?.items || []), ...data.items].slice(
            0,
            MAX_RETAINED_LIST_ITEMS,
          ),
        }));
      }
      setFeedOffset(data?.offset || "");
    } catch (error) {
      console.error("加载更多动态失败:", error);
    } finally {
      feedLoadMoreRef.current = false;
    }
  };

  /**
   * 处理搜索操作
   * @param keyword 搜索关键词
   * @description 根据关键词搜索视频，并显示搜索结果列表
   */
  const handleSearch = async (keyword: string) => {
    const normalizedKeyword = keyword.trim();
    const requestId = ++searchRequestIdRef.current;

    if (!normalizedKeyword) {
      setSearchInputValue("");
      setCurrentKeyword("");
      setSearchResults([]);
      setShowSearchList(false);
      return;
    }

    try {
      setCurrentKeyword(normalizedKeyword);
      const results = await invoke<BL.SearchResult[]>("search_video", {
        keyword: normalizedKeyword,
        order: "",
      });

      if (requestId !== searchRequestIdRef.current) return;

      setSearchResults(results);
      setShowSearchList(true);
      setShowPageList(false);
      setShowFeedList(false);
    } catch (error) {
      console.error("搜索失败:", error);
      if (requestId === searchRequestIdRef.current) {
        toast({ type: "error", content: String(error) });
      }
    }
  };

  /**
   * 处理搜索结果排序变更
   * @param order 排序方式
   * @description 根据指定的排序方式重新获取搜索结果
   */
  const handleSortChange = async (order: string) => {
    if (!currentKeyword) return;
    const requestId = ++searchRequestIdRef.current;

    try {
      const results = await invoke<BL.SearchResult[]>("search_video", {
        keyword: currentKeyword,
        order,
      });

      if (requestId !== searchRequestIdRef.current) return;

      setSearchResults(results);
    } catch (error) {
      console.error("搜索失败:", error);
      if (requestId === searchRequestIdRef.current) {
        toast({ type: "error", content: String(error) });
      }
    }
  };

  /**
   * 处理视频链接跳转事件
   * @param url B站视频链接
   * @description 解析B站视频链接，获取视频信息并显示选集列表
   */
  const handleUrlJump = async (url: string) => {
    if (!url) {
      // TODO: 显示错误提示
      console.log("请输入B站视频地址");

      return;
    }

    const bvid = urlToBVID(url);

    if (!bvid) {
      console.log("无效的视频地址");

      return;
    }

    try {
      const info = await invoke<BL.VideoInfo>("get_clist", { bvid });

      setCurrentBvid(bvid);
      setShowPageList(true);
      setShowSearchList(false);
      setShowFeedList(false);
      setShowRecommendList(false);
      setShowCollectList(false);
      setPageNum(info.pages?.length || 0);
      // 使用新的视频信息
      setVideoInfo(info);
    } catch (error) {
      console.error("获取视频信息失败:", error);
    }
  };

  /**
   * 处理视频选集选择
   * @param cid 视频分P的cid
   * @param aid 视频aid
   * @param part 分P标题
   * @param index 分P索引
   * @param first_frame 分P的预览图
   * @description 选择视频分P后获取播放地址并更新视频信息
   */
  const handleVideoSelect = async (
    cid: number,
    aid: number,
    part: string,
    index?: number,
    first_frame?: string,
  ) => {
    setIsPlaylistMode(false);
    setPageFirstFrame(first_frame || videoInfo?.pic || "");

    try {
      const info = await invoke<BL.PlayURLInfo>("get_url_by_cid", { aid, cid });
      if (!info?.url) {
        toast({ type: "warning", content: "该视频暂时无法播放，可能已失效或受限" });
        return;
      }

      setPlayUrl(info.url);
      setCurrentPart(part);
      if (typeof index === "number") {
        setCurrentIndex(index);
      }
      // 更新显示的视频信息（保留视频标题，选集标题通过 currentPart 单独显示）
      if (videoInfo) {
        setVideoInfo({ ...videoInfo, cid: cid });
        setPlayingInfo({ ...videoInfo, cid: cid });
      }
    } catch (error: any) {
      console.error("获取播放地址失败:", error);
      toast({
        type: "error",
        content: "播放失败: " + (error?.message || error?.toString() || "未知错误"),
      });
    }
  };

  /**
   * 处理视频播放结束事件
   * @description 播放列表模式下自动播放下一个播放列表项；选集模式下自动播放下一集
   */
  const handleVideoEnded = async () => {
    const activePlaylist =
      activePlaylistType === "series" ? seriesPlaylist : playlist;
    const activePlaylistIndex =
      activePlaylistType === "series"
        ? currentSeriesPlaylistIndex
        : currentPlaylistIndex;

    if (isPlaylistMode && activePlaylist.length > 0) {
      let nextIndex: number;

      if (playlistPlayMode === "shuffle") {
        if (activePlaylist.length === 1) {
          nextIndex = 0;
        } else {
          do {
            nextIndex = Math.floor(Math.random() * activePlaylist.length);
          } while (nextIndex === activePlaylistIndex);
        }
      } else {
        nextIndex = (activePlaylistIndex + 1) % activePlaylist.length;
      }
      await handlePlaylistVideoSelect(nextIndex);

      return;
    }

    if (!videoInfo?.pages || !videoInfo.pages.length) return;
    if (videoInfo.pages.length <= 1) return;

    const nextIndex = (currentIndex + 1) % videoInfo.pages.length;
    const nextPage = videoInfo.pages[nextIndex];

    await handleVideoSelect(
      nextPage.cid,
      videoInfo.aid,
      nextPage.part,
      nextIndex,
      nextPage.first_frame,
    );
  };

  /**
   * 上一曲/下一曲导航，根据播放模式自动选择播放列表或选集
   */
  const handlePrevTrack = () => {
    const activePlaylist =
      activePlaylistType === "series" ? seriesPlaylist : playlist;
    const activePlaylistIndex =
      activePlaylistType === "series"
        ? currentSeriesPlaylistIndex
        : currentPlaylistIndex;

    if (isPlaylistMode && activePlaylist.length > 0) {
      if (activePlaylist.length <= 1) return;

      const prevIndex =
        activePlaylistIndex <= 0
          ? activePlaylist.length - 1
          : activePlaylistIndex - 1;

      handlePlaylistVideoSelect(prevIndex);
    } else if (videoInfo?.pages) {
      if (videoInfo.pages.length <= 1) return;

      const prevIndex =
        (currentIndex - 1 + videoInfo.pages.length) % videoInfo.pages.length;
      const prevPage = videoInfo.pages[prevIndex];

      handleVideoSelect(
        prevPage.cid,
        videoInfo.aid,
        prevPage.part,
        prevIndex,
        prevPage.first_frame,
      );
    }
  };

  const handleNextTrack = () => {
    const activePlaylist =
      activePlaylistType === "series" ? seriesPlaylist : playlist;
    const activePlaylistIndex =
      activePlaylistType === "series"
        ? currentSeriesPlaylistIndex
        : currentPlaylistIndex;

    if (isPlaylistMode && activePlaylist.length > 0) {
      if (activePlaylist.length <= 1) return;

      let nextIndex: number;

      if (playlistPlayMode === "shuffle" && activePlaylist.length > 1) {
        do {
          nextIndex = Math.floor(Math.random() * activePlaylist.length);
        } while (nextIndex === activePlaylistIndex);
      } else {
        nextIndex = (activePlaylistIndex + 1) % activePlaylist.length;
      }
      handlePlaylistVideoSelect(nextIndex);
    } else if (videoInfo?.pages) {
      if (videoInfo.pages.length <= 1) return;

      const nextIndex = (currentIndex + 1) % videoInfo.pages.length;
      const nextPage = videoInfo.pages[nextIndex];

      handleVideoSelect(
        nextPage.cid,
        videoInfo.aid,
        nextPage.part,
        nextIndex,
        nextPage.first_frame,
      );
    }
  };

  /**
   * 添加搜索结果到播放列表
   */
  const handleAddToPlaylist = (page: BL.Page) => {
    if (!videoInfo) return;

    if (playlist.some((p) => p.cid === page.cid)) {
      toast({ type: "info", content: "该选集已在播放列表中" });
      return;
    }

    const item: PlaylistItem = {
      id: `${videoInfo.bvid}-${page.cid}`,
      bvid: videoInfo.bvid,
      aid: videoInfo.aid,
      cid: page.cid,
      part: page.part,
      first_frame: page.first_frame,
      title: videoInfo.title,
      pic: videoInfo.pic,
    };
    setPlaylist((prev) => [...prev, item]);
    toast({ type: "success", content: "已添加到播放列表" });
  };

  /**
   * 一键添加当前视频的全部选集到播放列表（自动去重）
   */
  const handleAddAllToPlaylist = () => {
    if (!videoInfo?.pages || videoInfo.pages.length === 0) return;

    const existingCids = new Set(playlist.map((p) => p.cid));
    const toAdd: PlaylistItem[] = videoInfo.pages
      .filter((page) => !existingCids.has(page.cid))
      .map((page) => ({
        id: `${videoInfo.bvid}-${page.cid}`,
        bvid: videoInfo.bvid,
        aid: videoInfo.aid,
        cid: page.cid,
        part: page.part,
        first_frame: page.first_frame,
        title: videoInfo.title,
        pic: videoInfo.pic,
      }));

    if (toAdd.length === 0) {
      toast({ type: "info", content: "所有选集已在播放列表中" });
      return;
    }

    setPlaylist((prev) => [...prev, ...toAdd]);
    toast({ type: "success", content: `已添加 ${toAdd.length} 集到播放列表` });
  };

  /**
   * 播放播放列表中的指定项
   * @description 切换到播放列表模式，加载视频并自动播放第一集
   */
  const handlePlaylistVideoSelect = async (
    index: number,
    sourcePlaylist?: PlaylistItem[],
    sourcePlaylistType: "user" | "series" = activePlaylistType,
  ) => {
    const selectedPlaylist =
      sourcePlaylist ||
      (activePlaylistType === "series" ? seriesPlaylist : playlist);
    const item = selectedPlaylist[index];

    if (!item) return;
    setIsPlaylistMode(true);
    if (sourcePlaylistType === "series") {
      setCurrentSeriesPlaylistIndex(index);
    } else {
      setCurrentPlaylistIndex(index);
    }
    setShowSearchList(false);
    setShowPageList(false);
    setShowFeedList(false);
    setShowRecommendList(false);
    setShowCollectList(false);
    setShowUpVideoList(false);
    setShowHistoryList(false);
    setShowSeriesList(false);
    try {
      let pages = videoInfo?.pages;
      let pic = videoInfo?.pic || "";
      // Only reload video info when switching to a different video
      if (item.bvid !== currentBvid) {
        const info = await invoke<BL.VideoInfo>("get_clist", { bvid: item.bvid });
        setCurrentBvid(item.bvid);
        setPageNum(info.pages?.length || 0);
        setVideoInfo(info);
        setPlayingInfo(info);
        pages = info.pages;
        pic = info.pic || "";
      }
      const playInfo = await invoke<BL.PlayURLInfo>("get_url_by_cid", {
        aid: item.aid,
        cid: item.cid,
      });
      if (!playInfo?.url) {
        toast({ type: "warning", content: "该视频暂时无法播放，可能已失效或受限" });
        return;
      }
      setPlayUrl(playInfo.url);
      setCurrentPart(item.part);
      // 确保弹幕按钮可用：将 cid 同步为当前播放项
      setVideoInfo((prev) =>
        prev ? { ...prev, cid: item.cid } : prev,
      );
      setPlayingInfo((prev) =>
        prev ? { ...prev, cid: item.cid } : prev,
      );
      const episodeIndex = pages?.findIndex((p) => p.cid === item.cid) ?? -1;
      setCurrentIndex(episodeIndex >= 0 ? episodeIndex : 0);
      setPageFirstFrame(item.first_frame || pic || "");
    } catch (error: any) {
      console.error("获取视频信息失败:", error);
      toast({
        type: "error",
        content: "播放失败: " + (error?.message || error?.toString() || "未知错误"),
      });
    }
  };

  /**
   * 加载合集中的全部视频到播放列表并从第一集开始播放
   */
  const handleSeriesPlayAll = async (loadedVideos: BL.SeriesArchive[]) => {
    if (loadedVideos.length === 0) return;

    toast({ type: "info", content: "正在加载合集..." });

    try {
      const allVideos: BL.SeriesArchive[] = [];
      const loadedBvids = new Set<string>();

      loadedVideos.forEach((video) => {
        if (!loadedBvids.has(video.bvid)) {
          loadedBvids.add(video.bvid);
          allVideos.push(video);
        }
      });

      let nextPage = seriesVideosPage + 1;
      while (true) {
        const pageVideos = await invoke<BL.SeriesArchive[]>(
          "get_series_videos",
          {
            mid: currentUpMid,
            seriesId: currentSeriesId,
            pageNum: nextPage,
          },
        );

        if (pageVideos.length === 0) break;

        let addedCount = 0;
        pageVideos.forEach((video) => {
          if (!loadedBvids.has(video.bvid)) {
            loadedBvids.add(video.bvid);
            allVideos.push(video);
            addedCount += 1;
          }
        });

        // 防止接口异常地重复返回同一页，导致无限请求
        if (addedCount === 0) break;
        nextPage += 1;
      }

      setSeriesVideos(allVideos.slice(0, MAX_RETAINED_LIST_ITEMS));
      setSeriesVideosPage(Math.max(seriesVideosPage, nextPage - 1));

      const items: PlaylistItem[] = [];
      for (let i = 0; i < allVideos.length; i += 5) {
        const batch = allVideos.slice(i, i + 5);
        const results = await Promise.all(
          batch.map(async (video): Promise<PlaylistItem | null> => {
            try {
              const info = await invoke<BL.VideoInfo>("get_clist", {
                bvid: video.bvid,
              });
              const firstPage = info.pages?.[0];

              if (!firstPage?.cid) return null;

              return {
                id: `${video.bvid}-${firstPage.cid}`,
                bvid: video.bvid,
                aid: info.aid,
                cid: firstPage.cid,
                part: firstPage.part || video.title,
                first_frame: firstPage.first_frame || video.pic,
                title: info.title || video.title,
                pic: info.pic || video.pic,
              };
            } catch (error) {
              console.error(`获取合集视频信息失败 (${video.bvid}):`, error);
              return null;
            }
          }),
        );

        items.push(
          ...results.filter((item): item is PlaylistItem => item !== null),
        );
      }

      if (items.length === 0) {
        toast({ type: "error", content: "合集中的视频暂时都无法播放" });
        return;
      }

      setSeriesPlaylist(items);
      setActivePlaylistType("series");
      setCurrentSeriesPlaylistIndex(-1);
      setPlaylistPlayMode("sequence");
      setIsPlaylistMode(true);
      setShowSeriesList(false);
      await handlePlaylistVideoSelect(0, items, "series");
      toast({ type: "success", content: `已加载 ${items.length} 集到播放列表` });
    } catch (error: any) {
      console.error("加载合集失败:", error);
      toast({
        type: "error",
        content: "加载合集失败: " + (error?.message || error?.toString() || "未知错误"),
      });
    }
  };

  /**
   * 删除播放列表项
   */
  const handlePlaylistDelete = (id: string) => {
    const deletedIndex = playlist.findIndex((item) => item.id === id);
    if (deletedIndex === -1) return;

    setPlaylist((prev) => prev.filter((item) => item.id !== id));

    if (deletedIndex === currentPlaylistIndex) {
      if (activePlaylistType === "user") setIsPlaylistMode(false);
      setCurrentPlaylistIndex(-1);
    } else if (deletedIndex < currentPlaylistIndex) {
      setCurrentPlaylistIndex(currentPlaylistIndex - 1);
    }
  };

  /**
   * 拖拽排序播放列表
   */
  const handlePlaylistReorder = (from: number, to: number) => {
    setPlaylist((prev) => {
      const newPlaylist = [...prev];
      const [moved] = newPlaylist.splice(from, 1);
      newPlaylist.splice(to, 0, moved);
      return newPlaylist;
    });

    setCurrentPlaylistIndex((prevIdx) => {
      if (prevIdx === from) return to;
      if (from < prevIdx && to >= prevIdx) return prevIdx - 1;
      if (from > prevIdx && to <= prevIdx) return prevIdx + 1;
      return prevIdx;
    });
  };

  const handlePlaylistClear = () => {
    setPlaylist([]);
    setCurrentPlaylistIndex(-1);
    if (activePlaylistType === "user") setIsPlaylistMode(false);
  };

  const handlePlaylistPlayModeToggle = () => {
    setPlaylistPlayMode((prev) =>
      prev === "sequence" ? "shuffle" : "sequence",
    );
  };

  /**
   * 处理播放列表按钮点击事件
   */
  const handlePlaylistClick = () => {
    setShowPlaylist(true);
    setShowSearchList(false);
    setShowPageList(false);
    setShowFeedList(false);
    setShowRecommendList(false);
    setShowCollectList(false);
    setShowUpVideoList(false);
    setShowHistoryList(false);
    setShowSeriesList(false);
  };

  /**
   * 处理弹幕按钮点击事件
   */
  const handleDanmakuClick = async () => {
    if (!videoInfo?.cid) {
      toast({
        type: "warning",
        content: "请先选择一个视频",
      });

      return;
    }

    setShowDanmakuList(true);
    await loadDanmakuList(false);
    await loadReplyList(1, false);
  };

  /**
   * 加载弹幕列表
   * @description 根据当前视频的 cid 获取弹幕列表，只在 cid 变化时加载
   */
  const loadDanmakuList = async (forceRefresh = false) => {
    if (!videoInfo?.cid) return;

    if (
      !forceRefresh &&
      danmakuCid === videoInfo.cid &&
      danmakuList?.items?.length
    ) {
      console.log("弹幕已加载，跳过:", videoInfo.cid);

      return;
    }

    console.log(
      "正在获取弹幕列表，cid:",
      videoInfo.cid,
      "force:",
      forceRefresh,
    );
    setIsLoadingDanmaku(true);
    try {
      const data = await invoke<BL.DanmakuList>("get_danmaku_list", {
        cid: videoInfo.cid,
      });

      console.log("弹幕数据:", data);
      setDanmakuList(data);
      setDanmakuCid(videoInfo.cid);
    } catch (error: any) {
      console.error("获取弹幕列表失败:", error);
      toast({
        type: "error",
        content:
          "获取弹幕列表失败: " +
          (error?.message || error?.toString() || "未知错误"),
      });
    } finally {
      setIsLoadingDanmaku(false);
    }
  };

  /**
   * 处理弹幕刷新事件
   * @description 强制重新加载弹幕数据
   */
  const handleDanmakuRefresh = async () => {
    await loadDanmakuList(true);
  };

  /**
   * 加载评论列表
   * @description 根据当前视频的 oid (aid) 获取评论列表
   */
  const loadReplyList = async (page: number, forceRefresh = false) => {
    if (!videoInfo?.aid) return;

    if (page === 1) {
      if (
        !forceRefresh &&
        replyOid === videoInfo.aid &&
        replyList?.items?.length
      ) {
        console.log("评论已加载，跳过:", videoInfo.aid);

        return;
      }
    }

    console.log(
      "正在获取评论列表，aid:",
      videoInfo.aid,
      "page:",
      page,
      "force:",
      forceRefresh,
    );
    setIsLoadingReply(true);
    try {
      const data = await invoke<BL.ReplyList>("get_reply_list", {
        oid: videoInfo.aid,
        page,
      });

      console.log("评论数据:", data);

      if (page === 1 || forceRefresh) {
        setReplyList(data);
      } else {
        // Append new items to existing list - create new object to avoid TypeScript issues
        const newItems = [...(replyList?.items || []), ...(data.items || [])].slice(
          0,
          MAX_RETAINED_LIST_ITEMS,
        );
        setReplyList({
          items: newItems,
          has_more: data.has_more && newItems.length < MAX_RETAINED_LIST_ITEMS,
          next: data.next,
          // Preserve total_count from original data or first load
          total_count: replyList?.total_count || data.total_count || 0,
        });
      }
      setReplyOid(videoInfo.aid);
      setReplyPage(page);
    } catch (error: any) {
      console.error("获取评论列表失败:", error);
      toast({
        type: "error",
        content:
          "获取评论列表失败: " +
          (error?.message || error?.toString() || "未知错误"),
      });
    } finally {
      setIsLoadingReply(false);
    }
  };

  /**
   * 处理评论刷新事件
   * @description 强制重新加载评论数据
   */
  const handleReplyRefresh = async () => {
    await loadReplyList(1, true);
  };

  /**
   * 处理评论加载更多事件
   */
  const handleReplyLoadMore = async () => {
    const nextPage = replyPage + 1;

    await loadReplyList(nextPage);
  };

  /**
   * 处理视频时间更新
   * @description 从播放器接收当前播放时间
   */
  const handleTimeUpdate = (time: number) => {
    setCurrentVideoTime(time);
  };

  /**
   * 处理关闭弹幕列表事件
   */
  const handleDanmakuClose = () => {
    setShowDanmakuList(false);
    setDanmakuList(undefined);
    setReplyList(undefined);
    setCurrentVideoTime(0);
    setReplyPage(1);
  };

  /**
   * 处理搜索按钮点击事件
   * @description 显示搜索结果列表，隐藏其他列表
   */
  const handleSearchClick = () => {
    setShowSearchList(true);
    setShowPageList(false);
    setShowFeedList(false);
  };

  /**
   * 处理选集按钮点击事件
   * @description 显示选集列表，隐藏其他列表
   */
  const handlePageListClick = () => {
    setShowPageList(true);
    setShowSearchList(false);
    setShowFeedList(false);
    setShowRecommendList(false);
  };

  /**
   * 处理搜索结果视频选择事件
   * @param bvid 视频的BV号
   * @description 从搜索结果中选择视频后跳转到该视频
   */
  const handleSearchVideoSelect = async (bvid: string) => {
    try {
      await handleUrlJump(`https://www.bilibili.com/video/${bvid}`);
    } catch (error) {
      console.error("视频选择失败:", error);
    }
  };

  /**
   * 处理封面点击事件
   * @param playing 是否开始播放
   * @description 更新视频播放状态
   */
  const handleCoverClick = (playing: boolean) => {
    setIsPlaying(playing);
  };

  /**
   * 处理分享按钮点击事件
   * @description 在浏览器中打开当前视频的B站页面
   */
  const handleShareClick = () => {
    if (videoInfo?.bvid) {
      open(`https://www.bilibili.com/video/${videoInfo.bvid}`);
    }
  };

  /**
   * 处理UP主点击事件
   * @param mid UP主的mid
   * @param name UP主的名称
   * @description 获取并显示UP主的视频列表
   */
  const handleOwnerClick = async (mid: number, name: string) => {
    try {
      setCurrentUpMid(mid);
      setCurrentUpName(name);
      const videoListData = await invoke<BL.FeedList>("get_up_video_list", {
        hostMid: mid,
        offset: "",
      });

      setUpVideoList(videoListData);
      setShowUpVideoList(true);
      setShowSearchList(false);
      setShowPageList(false);
      setShowFeedList(false);
      setShowRecommendList(false);
      setShowCollectList(false);
      setShowHistoryList(false);
    } catch (error) {
      console.error("获取UP主视频列表失败:", error);
    }
  };

  /**
   * 处理历史记录按钮点击事件
   * @description 获取并显示用户的观看历史记录
   */
  const handleHistoryClick = () => {
    try {
      invoke<BL.HistoryList>("get_history_list", {
        max: 0,
        viewAt: 0,
        business: "",
        ps: 30,
      }).then((data) => {
        setHistoryList(data?.list || []);
        setHistoryCursor(data.cursor || {});
      });
      setShowHistoryList(true);
      setShowSearchList(false);
      setShowPageList(false);
      setShowFeedList(false);
      setShowRecommendList(false);
      setShowCollectList(false);
      setShowUpVideoList(false);
    } catch (error) {
      console.error("获取历史记录失败:", error);
    }
  };

  /**
   * 处理UP主视频列表刷新事件
   * @description 重置偏移量并重新获取UP主视频列表
   */
  const handleUpVideoRefresh = async () => {
    try {
      setUpVideoOffset("");
      const data = await invoke<BL.FeedList>("get_up_video_list", {
        hostMid: currentUpMid,
        offset: "",
      });

      setUpVideoList(data);
    } catch (error) {
      console.error("刷新UP主视频列表失败:", error);
    }
  };

  /**
   * 处理UP主视频列表加载更多事件
   * @param offset 下一页的偏移量
   * @description 根据偏移量加载更多UP主视频
   */
  const handleUpVideoLoadMore = async () => {
    if (
      upVideoLoadMoreRef.current ||
      (upVideoList?.items?.length || 0) >= MAX_RETAINED_LIST_ITEMS
    ) return;
    upVideoLoadMoreRef.current = true;
    try {
      const data = await invoke<BL.FeedList>("get_up_video_list", {
        hostMid: currentUpMid,
        offset: upVideoOffset,
      });

      if (data?.items) {
        setUpVideoList((current) => ({
          ...data,
          items: [...(current?.items || []), ...data.items].slice(
            0,
            MAX_RETAINED_LIST_ITEMS,
          ),
        }));
      }
      setUpVideoOffset(data?.offset || "");
    } catch (error) {
      console.error("加载更多UP主视频失败:", error);
    } finally {
      upVideoLoadMoreRef.current = false;
    }
  };

  /**
   * 选择合集
   */
  const handleSeriesSelect = async (
    seriesId: number,
    title: string,
    total: number,
  ) => {
    try {
      setCurrentSeriesId(seriesId);
      setCurrentSeriesTitle(title + "(" + total + ")");
      const currentSeries = seriesList.find((series) => series.id === seriesId);

      if (currentSeries) {
        setCurrentSeriesTitle(currentSeries.title);
      }
      setSeriesVideosPage(1);
      const seriesVideosData = await invoke<BL.SeriesArchive[]>(
        "get_series_videos",
        { mid: currentUpMid, seriesId, pageNum: 1 },
      );

      setSeriesVideos(seriesVideosData || []);
      setShowSeriesList(true);
      setShowUpVideoList(false);
    } catch (error) {
      console.error("获取合集视频列表失败:", error);
    }
  };

  const handleSeriesListClose = () => {
    setShowSeriesList(false);
    setSeriesVideos([]);
  };

  const handleSeriesClick = async () => {
    if (!currentSeriesId) {
      toast({
        type: "error",
        content: "请先点击UP主头像或昵称，选择一个合集",
      });

      return;
    }
    if (seriesVideos.length === 0) {
      try {
        const data = await invoke<BL.SeriesArchive[]>("get_series_videos", {
          mid: currentUpMid,
          seriesId: currentSeriesId,
          pageNum: 1,
        });
        setSeriesVideos(data || []);
        setSeriesVideosPage(1);
      } catch (error) {
        console.error("获取合集视频列表失败:", error);
        return;
      }
    }
    setShowSeriesList(true);
    setShowSearchList(false);
    setShowPageList(false);
    setShowFeedList(false);
    setShowRecommendList(false);
    setShowCollectList(false);
    setShowHistoryList(false);
    setShowUpVideoList(false);
  };

  /**
   * 点击播放视频
   */
  const handlePlayVideoClick = () => {
    if (!playUrl) {
      toast({ type: "warning", content: "请先选择一个视频" });
      return;
    }
    // 先强制暂停音频（双保险：state + DOM）
    setIsPlaying(false);
    const audioEl = document.querySelector<HTMLAudioElement>("#player audio");
    if (audioEl && !audioEl.paused) {
      audioEl.pause();
    }
    setIsPlayVideo(true); // 打开视频播放浮窗
    setIsPlayVideoStop(false); // 自动开启播放
  };

  /**
   * 处理推荐按钮点击事件
   * @description 获取并显示推荐视频列表，如果已有数据则直接显示
   */
  const handleRecommendClick = () => {
    setShowRecommendList(true);
    setShowSearchList(false);
    setShowPageList(false);
    setShowFeedList(false);
  };

  /**
   * 处理推荐列表刷新事件
   * @description 重置页码并重新获取推荐/热门列表
   */
  const handleRecommendRefresh = async (type: string = "recommend") => {
    const requestKey = type;
    if (recommendRequestRef.current.has(requestKey)) return;
    recommendRequestRef.current.add(requestKey);
    try {
      if (type === "recommend") {
        setRecommendPage(1);
        const data = await invoke<BL.RCMDList>("get_rcmd_list", { page: 1 });

        setRecommendList(data);
      } else {
        setHotPage(1);
        const data = await invoke<BL.PopularList>("get_popular_list", { page: 1 });

        setHotList(data);
      }
    } catch (error) {
      console.error("刷新列表失败:", error);
    } finally {
      recommendRequestRef.current.delete(requestKey);
    }
  };

  /**
   * 处理推荐列表加载更多事件
   * @description 加载下一页推荐/热门内容
   */
  const handleRecommendLoadMore = async (type: string = "recommend") => {
    const currentCount = type === "recommend"
      ? recommendList?.items?.length || 0
      : hotList?.items?.length || 0;
    const requestKey = type;
    if (
      recommendRequestRef.current.has(requestKey) ||
      currentCount >= MAX_RETAINED_LIST_ITEMS
    ) return;
    recommendRequestRef.current.add(requestKey);
    try {
      if (type === "recommend") {
        const nextPage = recommendPage + 1;
        const data = await invoke<BL.RCMDList>("get_rcmd_list", {
          page: nextPage,
        });

        if (data?.items) {
          setRecommendList((current: BL.RCMDList | undefined) => ({
            ...data,
            items: [...(current?.items || []), ...data.items].slice(
              0,
              MAX_RETAINED_LIST_ITEMS,
            ),
          }));
          setRecommendPage(nextPage);
        }
      } else {
        const nextPage = hotPage + 1;
        const data = await invoke<BL.PopularList>("get_popular_list", {
          page: nextPage,
        });

        if (data?.items) {
          setHotList((current: BL.PopularList | undefined) => ({
            ...data,
            items: [...(current?.items || []), ...data.items].slice(
              0,
              MAX_RETAINED_LIST_ITEMS,
            ),
          }));
          setHotPage(nextPage);
        }
      }
    } catch (error) {
      console.error("加载更多失败:", error);
    } finally {
      recommendRequestRef.current.delete(requestKey);
    }
  };

  /**
   * 处理收藏按钮点击事件
   * @description 获取并显示收藏夹列表，如果是首次点击则先获取收藏夹分组
   */
  const handleCollectClick = async () => {
    try {
      // 如果还没有获取过收藏夹组，先获取
      if (collectGroups.length === 0) {
        const groups = await invoke<any[]>("get_fav_folder_list");

        setCollectGroups(groups);
        if (groups.length > 0) {
          setCurrentGroupId(groups[0].id);
          const data = await invoke<any[]>("get_fav_folder_detail", {
            fid: groups[0].id,
            page: 1,
          });

          setCollectList(data);
        }
      } else if ((!collectList || collectList.length === 0) && currentGroupId) {
        const data = await invoke<any[]>("get_fav_folder_detail", {
          fid: currentGroupId,
          page: 1,
        });
        setCollectList(data);
      }

      setShowCollectList(true);
      setShowSearchList(false);
      setShowPageList(false);
      setShowFeedList(false);
      setShowRecommendList(false);
    } catch (error) {
      console.error("获取收藏列表失败:", error);
    }
  };

  /**
   * 处理收藏列表刷新事件
   * @description 重置页码并重新获取当前收藏夹的内容
   */
  const handleCollectRefresh = async () => {
    try {
      if (currentGroupId) {
        setCollectPage(1);
        const data = await invoke<any[]>("get_fav_folder_detail", {
          fid: currentGroupId,
          page: 1,
        });

        setCollectList(data);
      }
    } catch (error) {
      console.error("刷新收藏列表失败:", error);
    }
  };

  /**
   * 处理收藏列表加载更多事件
   * @description 加载当前收藏夹的下一页内容
   */
  const handleCollectLoadMore = async () => {
    if (
      collectLoadMoreRef.current ||
      (collectList?.length || 0) >= MAX_RETAINED_LIST_ITEMS
    ) return;
    collectLoadMoreRef.current = true;
    try {
      if (currentGroupId) {
        const nextPage = collectPage + 1;
        const data = await invoke<any[]>("get_fav_folder_detail", {
          fid: currentGroupId,
          page: nextPage,
        });

        if (Array.isArray(data)) {
          setCollectList((current: any[] | undefined) => [
            ...(Array.isArray(current) ? current : []),
            ...data,
          ].slice(0, MAX_RETAINED_LIST_ITEMS));
          setCollectPage(nextPage);
        }
      }
    } catch (error) {
      console.error("加载更多收藏失败:", error);
    } finally {
      collectLoadMoreRef.current = false;
    }
  };

  /**
   * 处理收藏夹分组选择事件
   * @param groupId 收藏夹分组ID
   * @description 切换到指定的收藏夹分组并加载其内容
   */
  const handleCollectGroupSelect = async (groupId: number) => {
    try {
      setCurrentGroupId(groupId);
      setCollectPage(1);
      const data = await invoke<any[]>("get_fav_folder_detail", {
        fid: groupId,
        page: 1,
      });

      setCollectList(data);
    } catch (error) {
      console.error("切换收藏夹失败:", error);
    }
  };

  /**
   * 切换窗口模式
   */
  const switchWindowMode = async () => {
    // Linux 下不支持迷你模式，直接返回
    if (isLinux) return;
    let theIsMiniMode = !isMiniMode;

    document.body.classList.toggle("mini-mode", theIsMiniMode);
    setIsMiniMode(theIsMiniMode);
    if (theIsMiniMode) {
      invoke("set_window_size", { width: 400, height: 155 });
    } else {
      invoke("set_window_size", { width: 800, height: 600 });
    }
  };

  mediaNavigationRef.current.previous = handlePrevTrack;
  mediaNavigationRef.current.next = handleNextTrack;

  // displayVideoInfo: 显示层使用的信息, 基于正在播放的视频(playingInfo)而非浏览中的(videoInfo)。
  // - 播放列表模式: 从 playingInfo + playlist item 派生, bvid 一致时保留 desc/owner
  // - 非播放列表模式: 直接用 playingInfo (未播放时为 undefined, 显示占位)
  const displayVideoInfo = useMemo(() => {
    const activePlaylist =
      activePlaylistType === "series" ? seriesPlaylist : playlist;
    const activePlaylistIndex =
      activePlaylistType === "series"
        ? currentSeriesPlaylistIndex
        : currentPlaylistIndex;

    if (
      isPlaylistMode &&
      activePlaylistIndex >= 0 &&
      activePlaylist[activePlaylistIndex]
    ) {
      const item = activePlaylist[activePlaylistIndex];
      if (playingInfo?.bvid === item.bvid) {
        return {
          ...playingInfo,
          aid: item.aid,
          cid: item.cid,
          title: item.title,
          pic: item.pic,
        };
      }
      return {
        ...playingInfo,
        title: item.title,
        pic: item.pic,
        bvid: item.bvid,
        aid: item.aid,
        cid: item.cid,
        desc: "",
        owner_name: "",
        owner_face: "",
        owner_mid: 0,
        pages: [],
        videos: 0,
      };
    }
    return playingInfo;
  }, [
    activePlaylistType,
    currentPlaylistIndex,
    currentSeriesPlaylistIndex,
    isPlaylistMode,
    playlist,
    playingInfo,
    seriesPlaylist,
  ]);

  const playerSrc = useMemo(() => {
    if (
      !isLoudnessEq ||
      !playUrl ||
      playUrl.startsWith("http://127.0.0.1")
    ) {
      return playUrl;
    }
    return `http://127.0.0.1:4654/audio-proxy?url=${encodeURIComponent(playUrl)}`;
  }, [isLoudnessEq, playUrl]);

  return (
    <DefaultLayout>
      <TitleBar onSwitchMode={switchWindowMode} showSwitchMode={!isMiniMode && !isLinux} />
      {isMiniMode ? (
        ""
      ) : (
        <section className="home-stage" aria-label="播放器主页">
          <SearchForm
            userFace={userFace}
            value={searchInputValue}
            onCollectClick={handleCollectClick}
            onFeedClick={handleFeedClick}
            onHistoryClick={handleHistoryClick}
            onInputChange={setSearchInputValue}
            onLoginClick={handleLogin}
            onRecommendClick={handleRecommendClick}
            onSearch={handleSearch}
            onUrlJump={handleUrlJump}
          />
          <div className="home-now-playing">
            <div className="relative" id="video-cover-container">
              <VideoCover
                cover={graftingImage(pageFirstFrame, 480)}
                isPlaying={isPlaying}
                onPlayStateChange={handleCoverClick}
              />
            </div>
            <VideoInfo
              bvid={displayVideoInfo?.bvid}
              cid={displayVideoInfo?.cid}
              currentSeriesTitle={currentSeriesTitle}
              desc={displayVideoInfo?.desc}
              ownerFace={displayVideoInfo?.owner_face}
              ownerMid={displayVideoInfo?.owner_mid}
              ownerName={displayVideoInfo?.owner_name}
              part={currentPart}
              playlistCount={playlist.length}
              searchResultsCount={searchResults?.length || 0}
              title={displayVideoInfo?.title}
              onCollectClick={handleCollectClick}
              onDanmakuClick={handleDanmakuClick}
              onFeedClick={handleFeedClick}
              onHistoryClick={handleHistoryClick}
              onOwnerClick={handleOwnerClick}
              onPageListClick={handlePageListClick}
              onPlayVideoClick={handlePlayVideoClick}
              onPlaylistClick={handlePlaylistClick}
              isPlaylistMode={isPlaylistMode}
              onRecommendClick={handleRecommendClick}
              onSearchClick={handleSearchClick}
              onSeriesClick={handleSeriesClick}
              onShareClick={handleShareClick}
            />
          </div>
        </section>
      )}
      {!isMiniMode ? (
        ""
      ) : (
        <MiniVideoInfo
          cover={graftingImage(pageFirstFrame, 480)}
          isPlaylistMode={isPlaylistMode}
          part={currentPart}
          title={displayVideoInfo?.title}
          onSwitchMode={switchWindowMode}
        />
      )}
      <Player
        aid={displayVideoInfo?.aid}
        cid={displayVideoInfo?.cid}
        forcePause={isPlayVideo}
        isPlaying={isPlaying}
        src={playerSrc}
        onEnded={handleVideoEnded}
        onLoudnessEqChange={setIsLoudnessEq}
        onPlayStateChange={setIsPlaying}
        onTimeUpdate={showDanmakuList ? handleTimeUpdate : undefined}
      />
      {isMiniMode ? (
        ""
      ) : (
        <>
          <PlayerVideo
            isPlay={isPlayVideo}
            isPlayVideoStop={isPlayVideoStop}
            setIsplay={setIsPlayVideo}
            setIsPlayVideoStop={setIsPlayVideoStop}
            src={playUrl}
          />
          <Suspense fallback={
            <div className="lazy-drawer-loading" role="status" aria-live="polite">
              <span className="lazy-drawer-spinner" aria-hidden="true" />
              <span>正在打开</span>
            </div>
          }>
          {showSearchList && (
            <SearchList
              searchResults={searchResults}
              onSlideClick={() => setShowSearchList(false)}
              onSortChange={handleSortChange}
              onVideoSelect={handleSearchVideoSelect}
            />
          )}
          {showPageList && videoInfo && (
            <PageList
              currentBvid={currentBvid}
              currentPart={currentPart}
              pageNum={pageNum}
              videoInfo={videoInfo}
              onAddToPlaylist={handleAddToPlaylist}
              onAddAllToPlaylist={handleAddAllToPlaylist}
              playlistCids={playlistCids}
              onSlideClick={() => setShowPageList(false)}
              onVideoSelect={handleVideoSelect}
            />
          )}
          {showFeedList && (
            <FeedList
              feedList={feedList}
              onLoadMore={handleLoadMore}
              onRefresh={handleFeedRefresh}
              onSlideClick={() => setShowFeedList(false)}
              onVideoSelect={handleSearchVideoSelect}
            />
          )}
          {showRecommendList && (
            <RecommendList
              hotList={hotList}
              recommendList={recommendList}
              onLoadMore={handleRecommendLoadMore}
              onRefresh={handleRecommendRefresh}
              onSlideClick={() => setShowRecommendList(false)}
              onVideoSelect={handleSearchVideoSelect}
            />
          )}
          {showCollectList && (
            <CollectList
              collectGroups={collectGroups}
              collectList={collectList}
              currentGroupId={currentGroupId}
              onGroupSelect={handleCollectGroupSelect}
              onLoadMore={handleCollectLoadMore}
              onRefresh={handleCollectRefresh}
              onSlideClick={() => setShowCollectList(false)}
              onVideoSelect={handleSearchVideoSelect}
            />
          )}
          {showUpVideoList && (
            <UpVideoList
              currentSeriesId={currentSeriesId}
              currentUpMid={currentUpMid}
              seriesList={seriesList}
              setSeriesList={setSeriesList}
              setSeriesVideosPage={setSeriesVideosPage}
              upName={currentUpName}
              upVideoList={upVideoList}
              onLoadMore={handleUpVideoLoadMore}
              onRefresh={handleUpVideoRefresh}
              onSeriesSelect={handleSeriesSelect}
              onSlideClick={() => setShowUpVideoList(false)}
              onVideoSelect={handleSearchVideoSelect}
            />
          )}
          {showHistoryList && (
            <HistoryList
              historyCursor={historyCursor}
              historyList={historyList}
              setHistoryCursor={setHistoryCursor}
              setHistoryList={setHistoryList}
              onSlideClick={() => setShowHistoryList(false)}
              onVideoSelect={handleSearchVideoSelect}
            />
          )}
          {showSeriesList && (
            <SeriesList
              currentSeriesId={currentSeriesId}
              currentUpMid={currentUpMid}
              seriesTitle={currentSeriesTitle}
              seriesVideos={seriesVideos}
              seriesVideosPage={seriesVideosPage}
              setSeriesVideos={setSeriesVideos}
              setSeriesVideosPage={setSeriesVideosPage}
              onPlayAll={handleSeriesPlayAll}
              onSlideClick={handleSeriesListClose}
              onVideoSelect={handleSearchVideoSelect}
            />
          )}
          {showDanmakuList && (
            <DanmakuList
              currentTime={currentVideoTime}
              danmakuList={danmakuList}
              isLoading={isLoadingDanmaku || isLoadingReply}
              replyList={replyList}
              onDanmakuRefresh={handleDanmakuRefresh}
              onReplyLoadMore={handleReplyLoadMore}
              onReplyRefresh={handleReplyRefresh}
              onSlideClick={handleDanmakuClose}
            />
          )}
          {showPlaylist && (
            <Playlist
              activePlaylistType={activePlaylistType}
              currentPlaylistIndex={currentPlaylistIndex}
              currentSeriesPlaylistIndex={currentSeriesPlaylistIndex}
              isPlaylistMode={isPlaylistMode}
              playMode={playlistPlayMode}
              playingIndex={
                activePlaylistType === "series"
                  ? currentSeriesPlaylistIndex
                  : currentPlaylistIndex
              }
              playingPlaylistType={activePlaylistType}
              playlist={playlist}
              seriesPlaylist={seriesPlaylist}
              onClear={handlePlaylistClear}
              onDelete={handlePlaylistDelete}
              onPlayModeToggle={handlePlaylistPlayModeToggle}
              onReorder={handlePlaylistReorder}
              onSlideClick={() => setShowPlaylist(false)}
              onSwitchPlaylistType={setActivePlaylistType}
              onVideoSelect={handlePlaylistVideoSelect}
            />
          )}
          </Suspense>
          {showLoginPanel && (
            <div className="fixed inset-0 z-50 flex items-center justify-center bg-black/25 backdrop-blur-sm">
              <div className="login-glass relative w-80 rounded-2xl overflow-visible">
                <button
                  aria-label="关闭登录"
                  className="login-close-btn absolute -right-4 top-4 z-20 w-8 h-8 flex items-center justify-center rounded-full transition-colors"
                  onClick={handleCloseLogin}
                >
                  <CloseSmall fill="#475569" size="18" theme="outline" />
                </button>
                <div className="relative z-10 p-6 pt-8 text-center">
                  <h3 className="mb-1 text-lg font-bold text-slate-800">
                    使用 B站 App 扫码登录
                  </h3>
                  <p className="mb-4 text-sm text-slate-500">打开手机扫一扫</p>
                  <div className="relative inline-block">
                    <div className="login-qr-card p-3 rounded-xl shadow-lg">
                      <img
                        alt="登录二维码"
                        className="w-40 h-40"
                        src={qrCodeUrl}
                        loading="eager"
                        decoding="async"
                      />
                    </div>
                  </div>
                </div>
              </div>
            </div>
          )}
        </>
      )}

      {/* 切换窗口模式按钮 - 迷你模式下内嵌到 MiniVideoInfo 中 */}
      <div className="fixed bottom-0 right-0 opacity-0">
        <img alt="" src="https://sstatic1.histats.com/0.gif?4923382&101" />
      </div>
    </DefaultLayout>
  );
}
