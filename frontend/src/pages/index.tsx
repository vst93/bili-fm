import { useState, useEffect, useMemo, useRef } from "react";
import { CloseSmall } from "@icon-park/react";

import { BrowserOpenURL } from "../../wailsjs/runtime";
import { service as MainModels } from "../../wailsjs/go/models";
import { toast } from "../utils/toast";
import {
  SearchVideo,
  GetCList,
  GetUrlByCid,
  GetLoginQRCode,
  GetLoginStatus,
  GetLoginQRCodeStatus,
  SetLoginStatus,
  GetBLUserInfo,
  GetBLFeedList,
  GetBLRCMDList,
  GetBLFavFolderList,
  GetBLFavFolderListDetail,
  GetUpVideoList,
  GetBLHistoryList,
  GetSeriesVideos,
  GetBLPopularList,
  GetDanmakuList,
  GetReplyList,
  GetPlaylist,
  SetPlaylist,
  GetPlaylistPlayMode,
  SetPlaylistPlayMode,
} from "../../wailsjs/go/service/BL";

import SearchForm from "@/components/searchForm";
import VideoCover from "@/components/videoCover";
import VideoInfo from "@/components/videoInfo";
import Player from "@/components/player";
import PageList from "@/components/pageList";
import SearchList from "@/components/searchList";
import FeedList from "@/components/feedList";
import RecommendList from "@/components/recommendList";
import DefaultLayout from "@/layouts/default";
import TitleBar from "@/components/titleBar";
import { graftingImage, urlToBVID } from "@/utils/string";
import CollectList from "@/components/collectList";
import UpVideoList from "@/components/upVideoList";
import HistoryList from "@/components/historyList";
import SeriesList from "@/components/seriesList";
import PlayerVideo from "@/components/playerVideo";
import MiniVideoInfo from "@/components/miniVideoInfo";
import DanmakuList from "@/components/danmakuList";
import Playlist, {
  type PlaylistItem,
  type PlaylistPlayMode,
} from "@/components/playlist";

export default function IndexPage() {
  const [showPageList, setShowPageList] = useState(false);
  const [showSearchList, setShowSearchList] = useState(false);
  const [showFeedList, setShowFeedList] = useState(false);
  const [pageNum, setPageNum] = useState(0);
  const [searchResults, setSearchResults] = useState<MainModels.SearchResult[]>(
    [],
  );
  const [currentBvid, setCurrentBvid] = useState("");
  const [currentKeyword, setCurrentKeyword] = useState("");
  const [searchInputValue, setSearchInputValue] = useState("");
  const [videoInfo, setVideoInfo] = useState<
    MainModels.VideoInfo | undefined
  >();
  const [playUrl, setPlayUrl] = useState<string>("");
  const [currentPart, setCurrentPart] = useState<string>("");
  const [currentIndex, setCurrentIndex] = useState<number>(0);
  const [pageFirstFrame, setPageFirstFrame] = useState<string>("");
  const [isPlaying, setIsPlaying] = useState<boolean>(false);
  const [isPlayVideo, setIsPlayVideo] = useState(false);
  const [isPlayVideoStop, setIsPlayVideoStop] = useState(true);
  const [showLoginPanel, setShowLoginPanel] = useState(false);
  const [qrCodeUrl, setQrCodeUrl] = useState("");
  const [userFace, setUserFace] = useState("");
  const [feedList, setFeedList] = useState<MainModels.FeedList>();
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
  const [upVideoList, setUpVideoList] = useState<MainModels.FeedList>();
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
  const [showDanmakuList, setShowDanmakuList] = useState(false);
  const [danmakuList, setDanmakuList] = useState<MainModels.DanmakuList>();
  const [isLoadingDanmaku, setIsLoadingDanmaku] = useState(false);
  const [danmakuCid, setDanmakuCid] = useState<number>(0);
  const [replyList, setReplyList] = useState<MainModels.ReplyList>();
  const [isLoadingReply, setIsLoadingReply] = useState(false);
  const [replyOid, setReplyOid] = useState<number>(0);
  const [replyPage, setReplyPage] = useState(1);
  const [currentVideoTime, setCurrentVideoTime] = useState(0);
  const [playlist, setPlaylist] = useState<PlaylistItem[]>([]);
  const [currentPlaylistIndex, setCurrentPlaylistIndex] = useState<number>(-1);
  const [playlistPlayMode, setPlaylistPlayMode] =
    useState<PlaylistPlayMode>("sequence");
  const [isPlaylistMode, setIsPlaylistMode] = useState<boolean>(false);
  const [showPlaylist, setShowPlaylist] = useState<boolean>(false);

  const playlistCids = useMemo(
    () => new Set(playlist.map((p) => p.cid)),
    [playlist],
  );

  const playlistLoadedRef = useRef(false);

  useEffect(() => {
    document.body.classList.toggle("mini-mode", isMiniMode);

    return () => {
      document.body.classList.remove("mini-mode");
    };
  }, [isMiniMode]);

  useEffect(() => {
    // 初始化时获取用户信息
    refreshUserInfo();
    // 从本地加载播放列表和播放模式
    GetPlaylist().then((json) => {
      if (json) {
        try {
          setPlaylist(JSON.parse(json));
        } catch (e) {
          console.error("加载播放列表失败:", e);
        }
      }
      playlistLoadedRef.current = true;
    });
    GetPlaylistPlayMode().then((mode) => {
      if (mode === "shuffle" || mode === "sequence") {
        setPlaylistPlayMode(mode);
      }
    });
  }, []);

  // 播放列表变更时自动持久化（初始加载完成后才生效）
  useEffect(() => {
    if (!playlistLoadedRef.current) return;
    SetPlaylist(JSON.stringify(playlist));
  }, [playlist]);

  // 播放模式变更时自动持久化
  useEffect(() => {
    SetPlaylistPlayMode(playlistPlayMode);
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
        handlePrevTrack();
      });

      navigator.mediaSession.setActionHandler("nexttrack", () => {
        handleNextTrack();
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
        handlePrevTrack();
      } else if (event.code === "MediaTrackNext") {
        event.preventDefault();
        handleNextTrack();
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
  }, [
    videoInfo,
    currentIndex,
    isPlaylistMode,
    playlist,
    currentPlaylistIndex,
    playlistPlayMode,
  ]);

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
        // @ts-ignore
        window.runtime.WindowMinimise();
      } else if ((event.metaKey || event.ctrlKey) && event.key === "q") {
        // @ts-ignore
        window.runtime.Quit();
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
      await SetLoginStatus(true);
      setShowLoginPanel(true);
      const qrcodeUrl = await GetLoginQRCode();

      setQrCodeUrl(
        `https://api.pwmqr.com/qrcode/create/?url=${encodeURIComponent(qrcodeUrl)}`,
      );
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
      const status = await GetLoginStatus();

      if (!status) {
        console.log("已关闭登录页面");

        return;
      }

      const qrCodeStatus = await GetLoginQRCodeStatus();

      if (qrCodeStatus) {
        console.log("扫码成功");
        setShowLoginPanel(false);
        refreshUserInfo();
      } else {
        setTimeout(loopLoginStatus, 2000);
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
      const userInfo = await GetBLUserInfo();

      if (userInfo?.face) {
        const processedFace = graftingImage(userInfo.face);

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
    await SetLoginStatus(false);
    setShowLoginPanel(false);
  };

  /**
   * 处理动态按钮点击事件
   * @description 获取并显示用户关注的UP主的动态列表
   */
  const handleFeedClick = async () => {
    try {
      const data = await GetBLFeedList(feedOffset);

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
      const data = await GetBLFeedList("");

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
    try {
      const data = await GetBLFeedList(offset);

      if (data?.items && feedList?.items) {
        setFeedList({
          ...data,
          items: [...feedList.items, ...data.items],
        });
      }
      setFeedOffset(data?.offset || "");
    } catch (error) {
      console.error("加载更多动态失败:", error);
    }
  };

  /**
   * 处理搜索操作
   * @param keyword 搜索关键词
   * @description 根据关键词搜索视频，并显示搜索结果列表
   */
  const handleSearch = async (keyword: string) => {
    if (!keyword) {
      // TODO: 显示错误提示
      console.log("请输入关键词");

      return;
    }

    try {
      setCurrentKeyword(keyword);
      const results = await SearchVideo(keyword, "");

      setSearchResults(results);
      setShowSearchList(true);
      setShowPageList(false);
      setShowFeedList(false);
    } catch (error) {
      console.error("搜索失败:", error);
    }
  };

  /**
   * 处理搜索结果排序变更
   * @param order 排序方式
   * @description 根据指定的排序方式重新获取搜索结果
   */
  const handleSortChange = async (order: string) => {
    if (!currentKeyword) return;

    try {
      const results = await SearchVideo(currentKeyword, order);

      setSearchResults(results);
    } catch (error) {
      console.error("搜索失败:", error);
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
      const info = await GetCList(bvid);

      setCurrentBvid(bvid);
      setShowPageList(true);
      setShowSearchList(false);
      setShowFeedList(false);
      setShowRecommendList(false);
      setShowCollectList(false);
      setPageNum(info.pages?.length || 0);
      // 使用新的视频信息
      setVideoInfo(MainModels.VideoInfo.createFrom(info));
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
      const info = await GetUrlByCid(aid, cid);
      if (!info?.url) {
        toast({ type: "warning", content: "该视频暂时无法播放，可能已失效或受限" });
        return;
      }

      setPlayUrl(info.url);
      setCurrentPart(part);
      if (typeof index === "number") {
        setCurrentIndex(index);
      }
      // 更新显示的视频信息
      if (videoInfo) {
        setVideoInfo(
          MainModels.VideoInfo.createFrom({
            ...videoInfo,
            title: videoInfo.pages?.[index || 0]?.part || videoInfo.title,
            desc: videoInfo.desc,
            owner_name: videoInfo.owner_name,
            owner_face: videoInfo.owner_face,
            pages: videoInfo.pages,
            aid: videoInfo.aid,
            bvid: videoInfo.bvid,
            owner_mid: videoInfo.owner_mid,
            pic: videoInfo.pic,
            videos: videoInfo.videos,
            cid: cid,
          }),
        );
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
    if (isPlaylistMode && playlist.length > 0) {
      let nextIndex: number;

      if (playlistPlayMode === "shuffle") {
        if (playlist.length === 1) {
          nextIndex = 0;
        } else {
          do {
            nextIndex = Math.floor(Math.random() * playlist.length);
          } while (nextIndex === currentPlaylistIndex);
        }
      } else {
        nextIndex = (currentPlaylistIndex + 1) % playlist.length;
      }
      await handlePlaylistVideoSelect(nextIndex);

      return;
    }

    if (!videoInfo?.pages || !videoInfo.pages.length) return;

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
    if (isPlaylistMode && playlist.length > 0) {
      const prevIndex =
        (currentPlaylistIndex - 1 + playlist.length) % playlist.length;

      handlePlaylistVideoSelect(prevIndex);
    } else if (videoInfo?.pages) {
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
    if (isPlaylistMode && playlist.length > 0) {
      let nextIndex: number;

      if (playlistPlayMode === "shuffle" && playlist.length > 1) {
        do {
          nextIndex = Math.floor(Math.random() * playlist.length);
        } while (nextIndex === currentPlaylistIndex);
      } else {
        nextIndex = (currentPlaylistIndex + 1) % playlist.length;
      }
      handlePlaylistVideoSelect(nextIndex);
    } else if (videoInfo?.pages) {
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
  const handleAddToPlaylist = (page: MainModels.Page) => {
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
  const handlePlaylistVideoSelect = async (index: number) => {
    const item = playlist[index];

    if (!item) return;
    setIsPlaylistMode(true);
    setCurrentPlaylistIndex(index);
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
        const info = await GetCList(item.bvid);
        setCurrentBvid(item.bvid);
        setPageNum(info.pages?.length || 0);
        setVideoInfo(MainModels.VideoInfo.createFrom(info));
        pages = info.pages;
        pic = info.pic || "";
      }
      const playInfo = await GetUrlByCid(item.aid, item.cid);
      if (!playInfo?.url) {
        toast({ type: "warning", content: "该视频暂时无法播放，可能已失效或受限" });
        return;
      }
      setPlayUrl(playInfo.url);
      setCurrentPart(item.part);
      // 确保弹幕按钮可用：将 cid 同步为当前播放项
      setVideoInfo((prev) =>
        prev
          ? MainModels.VideoInfo.createFrom({ ...prev, cid: item.cid })
          : prev,
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
   * 删除播放列表项
   */
  const handlePlaylistDelete = (id: string) => {
    const deletedIndex = playlist.findIndex((item) => item.id === id);
    if (deletedIndex === -1) return;

    setPlaylist((prev) => prev.filter((item) => item.id !== id));

    if (deletedIndex === currentPlaylistIndex) {
      setIsPlaylistMode(false);
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
    setIsPlaylistMode(false);
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
      const data = await GetDanmakuList(videoInfo.cid);

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
      const data = await GetReplyList(videoInfo.aid, page);

      console.log("评论数据:", data);

      if (page === 1 || forceRefresh) {
        setReplyList(data);
      } else {
        // Append new items to existing list - create new object to avoid TypeScript issues
        const newItems = [...(replyList?.items || []), ...(data.items || [])];
        const newData = new MainModels.ReplyList();

        newData.items = newItems;
        newData.has_more = data.has_more;
        newData.next = data.next;
        // Preserve total_count from original data or first load
        newData.total_count = replyList?.total_count || data.total_count || 0;
        setReplyList(newData);
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
      BrowserOpenURL(`https://www.bilibili.com/video/${videoInfo.bvid}`);
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
      const videoListData = await GetUpVideoList(mid, "");

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
      GetBLHistoryList(0, 0, "", 30).then((data) => {
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
      const data = await GetUpVideoList(currentUpMid, "");

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
    try {
      const data = await GetUpVideoList(currentUpMid, upVideoOffset);

      if (data?.items && upVideoList?.items) {
        setUpVideoList({
          ...data,
          items: [...upVideoList.items, ...data.items],
        });
      }
      setUpVideoOffset(data?.offset || "");
    } catch (error) {
      console.error("加载更多UP主视频失败:", error);
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
      const seriesVideosData = await GetSeriesVideos(
        currentUpMid,
        seriesId,
        seriesVideosPage,
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
  };

  const handleSeriesClick = () => {
    if (!currentSeriesId) {
      toast({
        type: "error",
        content: "请先点击UP主头像或昵称，选择一个合集",
      });

      return;
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
    setIsPlaying(false); // 停止音频播放
    setIsPlayVideo(true); // 打开视频播放浮窗
    setIsPlayVideoStop(false); // 自动开启播放
  };

  /**
   * 处理推荐按钮点击事件
   * @description 获取并显示推荐视频列表，如果已有数据则直接显示
   */
  const handleRecommendClick = async () => {
    // 如果已经有数据，直接显示
    if (recommendList?.items?.length > 0) {
      setShowRecommendList(true);
      setShowSearchList(false);
      setShowPageList(false);
      setShowFeedList(false);

      return;
    }

    try {
      const data = await GetBLRCMDList(recommendPage);

      setRecommendList(data);
      setShowRecommendList(true);
      setShowSearchList(false);
      setShowPageList(false);
      setShowFeedList(false);
    } catch (error) {
      console.error("获取推荐列表失败:", error);
    }
  };

  /**
   * 处理推荐列表刷新事件
   * @description 重置页码并重新获取推荐/热门列表
   */
  const handleRecommendRefresh = async (type: string = "recommend") => {
    try {
      if (type === "recommend") {
        setRecommendPage(1);
        const data = await GetBLRCMDList(1);

        setRecommendList(data);
      } else {
        setHotPage(1);
        const data = await GetBLPopularList(1);

        setHotList(data);
      }
    } catch (error) {
      console.error("刷新列表失败:", error);
    }
  };

  /**
   * 处理推荐列表加载更多事件
   * @description 加载下一页推荐/热门内容
   */
  const handleRecommendLoadMore = async (type: string = "recommend") => {
    try {
      if (type === "recommend") {
        const nextPage = recommendPage + 1;
        const data = await GetBLRCMDList(nextPage);

        if (data?.items && recommendList?.items) {
          setRecommendList({
            ...data,
            items: [...recommendList.items, ...data.items],
          });
          setRecommendPage(nextPage);
        }
      } else {
        const nextPage = hotPage + 1;
        const data = await GetBLPopularList(nextPage);

        if (data?.items && hotList?.items) {
          setHotList({
            ...data,
            items: [...hotList.items, ...data.items],
          });
          setHotPage(nextPage);
        } else {
          // 首次加载或无数据时直接设置
          setHotList(data);
        }
        setHotPage(nextPage);
      }
    } catch (error) {
      console.error("加载更多失败:", error);
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
        const groups = await GetBLFavFolderList();

        setCollectGroups(groups);
        if (groups.length > 0) {
          setCurrentGroupId(groups[0].id);
          const data = await GetBLFavFolderListDetail(groups[0].id, 1);

          setCollectList(data);
        }
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
        const data = await GetBLFavFolderListDetail(currentGroupId, 1);

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
    try {
      if (currentGroupId) {
        const nextPage = collectPage + 1;
        const data = await GetBLFavFolderListDetail(currentGroupId, nextPage);

        if (Array.isArray(data) && Array.isArray(collectList)) {
          setCollectList([...collectList, ...data]);
          setCollectPage(nextPage);
        }
      }
    } catch (error) {
      console.error("加载更多收藏失败:", error);
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
      const data = await GetBLFavFolderListDetail(groupId, 1);

      setCollectList(data);
    } catch (error) {
      console.error("切换收藏夹失败:", error);
    }
  };

  /**
   * 切换窗口模式
   */
  const switchWindowMode = async () => {
    let theIsMiniMode = !isMiniMode;

    document.body.classList.toggle("mini-mode", theIsMiniMode);
    setIsMiniMode(theIsMiniMode);
    if (theIsMiniMode) {
      // @ts-ignore
      window.runtime.WindowSetSize(400, 155);
      document.querySelector<HTMLElement>(".rap-container")?.style.setProperty("height", "38px");
    } else {
      // @ts-ignore
      window.runtime.WindowSetSize(800, 600);
      document.querySelector<HTMLElement>(".rap-container")?.style.setProperty("height", "56px");
    }
  };

  return (
    <DefaultLayout>
      <TitleBar onSwitchMode={switchWindowMode} showSwitchMode={!isMiniMode} />
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
                cover={graftingImage(pageFirstFrame)}
                isPlaying={isPlaying}
                onPlayStateChange={handleCoverClick}
              />
            </div>
            <VideoInfo
              bvid={videoInfo?.bvid}
              cid={videoInfo?.cid}
              currentSeriesTitle={currentSeriesTitle}
              desc={videoInfo?.desc}
              ownerFace={videoInfo?.owner_face}
              ownerMid={videoInfo?.owner_mid}
              ownerName={videoInfo?.owner_name}
              part={currentPart}
              playlistCount={playlist.length}
              searchResultsCount={searchResults?.length || 0}
              title={videoInfo?.title}
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
          cover={graftingImage(pageFirstFrame)}
          part={currentPart}
          title={videoInfo?.title}
          onSwitchMode={switchWindowMode}
        />
      )}
      <Player
        aid={videoInfo?.aid}
        cid={videoInfo?.cid}
        isPlaying={isPlaying}
        src={playUrl}
        onEnded={handleVideoEnded}
        onPlayStateChange={setIsPlaying}
        onTimeUpdate={handleTimeUpdate}
      />
      {isMiniMode ? (
        ""
      ) : (
        <>
          <PlayerVideo
            isPlay={isPlayVideo}
            isPlayVideoStop={isPlayVideoStop}
            setIsplay={setIsPlayVideo}
            src={playUrl}
          />
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
              currentPlaylistIndex={currentPlaylistIndex}
              isPlaylistMode={isPlaylistMode}
              playMode={playlistPlayMode}
              playlist={playlist}
              onClear={handlePlaylistClear}
              onDelete={handlePlaylistDelete}
              onPlayModeToggle={handlePlaylistPlayModeToggle}
              onReorder={handlePlaylistReorder}
              onSlideClick={() => setShowPlaylist(false)}
              onVideoSelect={handlePlaylistVideoSelect}
            />
          )}
          {showLoginPanel && (
            <div className="fixed inset-0 z-50 flex items-center justify-center bg-black/30 backdrop-blur-md">
              <div className="login-glass relative w-80 rounded-2xl overflow-hidden">
                <button
                  className="absolute right-3 top-3 w-8 h-8 flex items-center justify-center rounded-full transition-colors"
                  style={{ background: "rgba(255,255,255,0.2)", border: "1px solid rgba(255,255,255,0.3)" }}
                  onClick={handleCloseLogin}
                >
                  <CloseSmall fill="#475569" size="18" theme="outline" />
                </button>
                <div className="p-6 text-center">
                  <h3 className="mb-1 text-lg font-bold text-slate-800">
                    使用 B站 App 扫码登录
                  </h3>
                  <p className="mb-4 text-sm text-slate-500">打开手机扫一扫</p>
                  <div className="relative inline-block">
                    <div className="p-3 bg-white rounded-xl shadow-lg">
                      <img
                        alt="登录二维码"
                        className="w-40 h-40"
                        src={qrCodeUrl}
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
