import { useState, useEffect } from "react";

import { BrowserOpenURL } from "../../wailsjs/runtime";
import { main as MainModels } from "../../wailsjs/go/models";
import { SearchVideo, GetCList, GetUrlByCid } from "../../wailsjs/go/main/BL";

import SearchForm from "@/components/searchForm";
import VideoCover from "@/components/videoCover";
import VideoInfo from "@/components/videoInfo";
import Player from "@/components/player";
import PageList from "@/components/pageList";
import SearchList from "@/components/searchList";
import DefaultLayout from "@/layouts/default";
import { graftingImage, urlToBVID } from "@/utils/string";

export default function IndexPage() {
  const [showPageList, setShowPageList] = useState(false);
  const [showSearchList, setShowSearchList] = useState(false);
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

  useEffect(() => {
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

    return () => {
      window.removeEventListener("keyup", handleKeyPress);
    };
  }, [videoInfo, currentIndex]);

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
   * 处理视频链接跳转
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
      setPageNum(info.pages?.length || 0);
      // 使用新的视频信息
      setVideoInfo(MainModels.VideoInfo.createFrom(info));
    } catch (error) {
      console.error("获取视频信息失败:", error);
    }
  };

  /**
   * 处理抽屉关闭事件
   * @description 关闭选集列表和搜索结果列表
   */
  // const handleSlideClick = () => {
  //   setShowPageList(false);
  //   setShowSearchList(false);
  // };

  /**
   * 处理视频选集选择
   * @param cid 视频分P的cid
   * @param aid 视频aid
   * @param part 分P标题
   * @param index 分P索引
   * @description 选择视频分P后获取播放地址并更新视频信息
   */
  const handleVideoSelect = async (
    cid: number,
    aid: number,
    part: string,
    index?: number,
    first_frame?: string,
  ) => {
    setPageFirstFrame(first_frame || "");

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
   * 处理搜索按钮点击
   * @description 显示搜索结果列表，隐藏选集列表
   */
  const handleSearchClick = () => {
    setShowSearchList(true);
    setShowPageList(false);
  };

  /**
   * 处理选集按钮点击
   * @description 显示选集列表，隐藏搜索结果列表
   */
  const handlePageListClick = () => {
    setShowPageList(true);
    setShowSearchList(false);
  };

  /**
   * 处理搜索结果视频选择
   * @param bvid 视频bvid
   * @description 从搜索结果中选择视频后跳转到该视频
   */
  const handleSearchVideoSelect = async (bvid: string) => {
    try {
      await handleUrlJump(`https://www.bilibili.com/video/${bvid}`);
    } catch (error) {
      console.error("视频选择失败:", error);
    }
  };

  const handleCoverClick = (playing: boolean) => {
    setIsPlaying(playing);
  };

  const handleShareClick = () => {
    if (videoInfo?.bvid) {
      BrowserOpenURL(`https://www.bilibili.com/video/${videoInfo.bvid}`);
    }
  };

  const handleOwnerClick = (name: string) => {
    setSearchInputValue(name);
  };

  return (
    <DefaultLayout>
      <SearchForm
        value={searchInputValue}
        onInputChange={setSearchInputValue}
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
        ownerName={videoInfo?.owner_name}
        part={currentPart}
        title={videoInfo?.title}
        onOwnerClick={handleOwnerClick}
        onPageListClick={handlePageListClick}
        onSearchClick={handleSearchClick}
        onShareClick={handleShareClick}
      />
      <Player
        isPlaying={isPlaying}
        src={playUrl}
        onEnded={handleVideoEnded}
        onPlayStateChange={setIsPlaying}
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
    </DefaultLayout>
  );
}
