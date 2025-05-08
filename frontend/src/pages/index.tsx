import { useState, useEffect } from "react";
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
import { graftingImage, urlToBVID } from "@/utils/string";
import CollectList from "@/components/collectList";
import UpVideoList from "@/components/upVideoList";
import HistoryList from "@/components/historyList";
import SeriesList from "@/components/seriesList";

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
  const [showLoginPanel, setShowLoginPanel] = useState(false);
  const [qrCodeUrl, setQrCodeUrl] = useState("");
  const [userFace, setUserFace] = useState("");
  const [feedList, setFeedList] = useState<MainModels.FeedList>();
  const [feedOffset, setFeedOffset] = useState("");
  const [showRecommendList, setShowRecommendList] = useState(false);
  const [recommendList, setRecommendList] = useState<any>();
  const [recommendPage, setRecommendPage] = useState(1);
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
  const [historyCursor, setHistoryCursor] = useState<{max: number, view_at: number, business: string}>({max: 0, view_at: 0, business: ""});
  const [seriesList, setSeriesList] = useState<any[]>([]);
  const [currentSeriesId, setCurrentSeriesId] = useState<number>(0);
  const [seriesVideos, setSeriesVideos] = useState<any[]>([]);
  const [showSeriesList, setShowSeriesList] = useState(false);
  const [currentSeriesTitle, setCurrentSeriesTitle] = useState("");
  const [seriesVideosPage, setSeriesVideosPage] = useState(1);


  useEffect(() => {
    // 初始化时获取用户信息
    refreshUserInfo();
  }, []);

  /**
   * 键盘事件处理函数
   * @param event 键盘事件对象
   * @description 处理空格键（播放/暂停）和左右方向键（上一个/下一个视频）的按键事件
   */
  useEffect(() => {

    //监听快捷键
    const listener = (event: KeyboardEvent) => {
      if ((event.metaKey || event.ctrlKey) && event.key === "w") {
        // @ts-ignore
        window.runtime.WindowMinimise();
      } else if ((event.metaKey || event.ctrlKey) && event.key === "q") { 
        // @ts-ignore
        window.runtime.Quit();
      }
    };

    //监听键盘事件
    const handleKeyPress = (event: KeyboardEvent) => {
      if (
        event.target instanceof HTMLInputElement ||
        event.target instanceof HTMLTextAreaElement
      ) {
        return;
      }

      if (event.code === "Space" && !event.repeat) {
        event.preventDefault();
        setIsPlaying((prev) => !prev);
      } else if (
        event.code === "ArrowLeft" &&
        !event.repeat &&
        videoInfo?.pages
      ) {
        event.preventDefault();
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
      } else if (
        event.code === "ArrowRight" &&
        !event.repeat &&
        videoInfo?.pages
      ) {
        event.preventDefault();
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

    window.addEventListener("keyup", handleKeyPress);
    window.addEventListener("keydown", listener);

    return () => {
      window.removeEventListener("keyup", handleKeyPress);
      window.removeEventListener("keydown", listener);
    };
  }, [videoInfo, currentIndex]);

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
    setPageFirstFrame(first_frame || videoInfo?.pic || "");

    try {
      const info = await GetUrlByCid(aid, cid);

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
    } catch (error) {
      console.error("获取播放地址失败:", error);
    }
  };

  /**
   * 处理视频播放结束事件
   * @description 当前视频播放完成后自动播放下一个视频，最后一个视频播放完成后循环到第一个
   */
  const handleVideoEnded = async () => {
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
      GetBLHistoryList(0,0,'',30).then(data => {
        setHistoryList(data?.list || [])
        setHistoryCursor(data.cursor || {})
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
  const handleSeriesSelect = async (seriesId: number, title: string, total: number) => {
    try {
      setCurrentSeriesId(seriesId);
      setCurrentSeriesTitle(title + "(" + total + ")");
      const currentSeries = seriesList.find(series => series.id === seriesId);
      if (currentSeries) {
        setCurrentSeriesTitle(currentSeries.title);
      }
      setSeriesVideosPage(1);
      const seriesVideosData = await GetSeriesVideos(currentUpMid, seriesId, seriesVideosPage);
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
   * @description 重置页码并重新获取推荐列表
   */
  const handleRecommendRefresh = async () => {
    try {
      setRecommendPage(1);
      const data = await GetBLRCMDList(1);

      setRecommendList(data);
    } catch (error) {
      console.error("刷新推荐列表失败:", error);
    }
  };

  /**
   * 处理推荐列表加载更多事件
   * @description 加载下一页推荐内容
   */
  const handleRecommendLoadMore = async () => {
    try {
      const nextPage = recommendPage + 1;
      const data = await GetBLRCMDList(nextPage);

      if (data?.items && recommendList?.items) {
        setRecommendList({
          ...data,
          items: [...recommendList.items, ...data.items],
        });
        setRecommendPage(nextPage);
      }
    } catch (error) {
      console.error("加载更多推荐失败:", error);
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

  return (
    <DefaultLayout>
      <SearchForm
        userFace={userFace}
        value={searchInputValue}
        onInputChange={setSearchInputValue}
        onLoginClick={handleLogin}
        onSearch={handleSearch}
        onUrlJump={handleUrlJump}
      />
      <VideoCover
        cover={graftingImage(pageFirstFrame)}
        isPlaying={isPlaying}
        onPlayStateChange={handleCoverClick}
      />
      <VideoInfo
        bvid={videoInfo?.bvid}
        desc={videoInfo?.desc}
        ownerFace={videoInfo?.owner_face}
        ownerMid={videoInfo?.owner_mid}
        ownerName={videoInfo?.owner_name}
        part={currentPart}
        title={videoInfo?.title}
        onCollectClick={handleCollectClick}
        onFeedClick={handleFeedClick}
        onOwnerClick={handleOwnerClick}
        onPageListClick={handlePageListClick}
        onRecommendClick={handleRecommendClick}
        onSearchClick={handleSearchClick}
        onShareClick={handleShareClick}
        onHistoryClick={handleHistoryClick}
        onSeriesClick={handleSeriesClick}
      />
      <Player
        isPlaying={isPlaying}
        src={playUrl}
        onEnded={handleVideoEnded}
        onPlayStateChange={setIsPlaying}
        aid={videoInfo?.aid}
        cid={videoInfo?.cid}
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
          upName={currentUpName}
          upVideoList={upVideoList}
          onLoadMore={handleUpVideoLoadMore}
          onRefresh={handleUpVideoRefresh}
          onSlideClick={() => setShowUpVideoList(false)}
          onVideoSelect={handleSearchVideoSelect}
          seriesList={seriesList}
          onSeriesSelect={handleSeriesSelect}
          currentSeriesId={currentSeriesId}
          setSeriesList={setSeriesList}
          currentUpMid={currentUpMid}
          setSeriesVideosPage={setSeriesVideosPage}
        />
      )}
      {showHistoryList && (
        <HistoryList
          onSlideClick={() => setShowHistoryList(false)}
          onVideoSelect={handleSearchVideoSelect}
          historyList={historyList}
          historyCursor={historyCursor}
          setHistoryList={setHistoryList}
          setHistoryCursor={setHistoryCursor}
        />
      )}
      {showSeriesList && (
        <SeriesList
          seriesVideos={seriesVideos}
          onVideoSelect={handleSearchVideoSelect}
          onSlideClick={handleSeriesListClose}
          seriesTitle={currentSeriesTitle}
          seriesVideosPage={seriesVideosPage}
          setSeriesVideosPage={setSeriesVideosPage}
          currentUpMid={currentUpMid}
          currentSeriesId={currentSeriesId}
          setSeriesVideos={setSeriesVideos}
        />
      )}
      {showLoginPanel && (
        <div className="fixed inset-0 z-50 flex items-center justify-center bg-black bg-opacity-50">
          <div className="relative w-80 rounded-lg bg-white p-6">
            <button
              className="absolute right-4 top-4  hover:bg-blue-100 active:bg-blue-300 rounded-full p-1"
              onClick={handleCloseLogin}
            >
              <CloseSmall fill="#333" size="24" theme="outline" />
            </button>
            <h3 className="mb-4 text-center text-lg font-semibold">
              使用BiLiBiLi APP 扫码登录
            </h3>
            <img
              alt="登录二维码"
              className="mx-auto h-48 w-48"
              src={qrCodeUrl}
            />
          </div>
        </div>
      )}
      <div className="fixed bottom-0 right-0 opacity-0">
        <img alt="" src="https://sstatic1.histats.com/0.gif?4923382&101" />
      </div>
    </DefaultLayout>
  );
}
