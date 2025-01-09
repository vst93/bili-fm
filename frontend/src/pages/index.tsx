import type { main } from "../../wailsjs/go/models";

import { useState } from "react";

import { SearchVideo, GetCList, GetUrlByCid } from "../../wailsjs/go/main/BL";


import SearchForm from "@/components/searchForm";
import VideoCover from "@/components/videoCover";
import VideoInfo from "@/components/videoInfo";
import Player from "@/components/player";
import PageList from "@/components/pageList";
import SearchList from "@/components/searchList";
import DefaultLayout from "@/layouts/default";
import { urlToBVID } from "@/utils/string";


export default function IndexPage() {
  const [showPageList, setShowPageList] = useState(false);
  const [showSearchList, setShowSearchList] = useState(false);
  const [pageNum, setPageNum] = useState(0);
  const [searchResults, setSearchResults] = useState<main.SearchResult[]>([]);
  const [currentBvid, setCurrentBvid] = useState("");
  const [currentKeyword, setCurrentKeyword] = useState("");
  const [videoInfo, setVideoInfo] = useState<main.VideoInfo | undefined>();
  const [playUrl, setPlayUrl] = useState<string>("");
  const [currentPart, setCurrentPart] = useState<string>("");

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

  const handleSortChange = async (order: string) => {
    if (!currentKeyword) return;
    
    try {
      const results = await SearchVideo(currentKeyword, order);
      setSearchResults(results);
    } catch (error) {
      console.error("搜索失败:", error);
    }
  };

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
      setVideoInfo(info);
      setCurrentBvid(bvid);
      setShowPageList(true);
      setShowSearchList(false);
      setPageNum(info.pages?.length || 0);
    } catch (error) {
      console.error("获取视频信息失败:", error);
    }
  };

  const handleSlideClick = () => {
    setShowPageList(false);
    setShowSearchList(false);
  };

  const handleVideoSelect = async (cid: number, aid: number, part: string) => {
    try {
      const info = await GetUrlByCid(aid, cid);
      setPlayUrl(info.url);
      setCurrentPart(part);
    } catch (error) {
      console.error("获取播放地址失败:", error);
    }
  };

  return (
    <DefaultLayout>
      <SearchForm onSearch={handleSearch} onUrlJump={handleUrlJump} />
      <VideoCover />
      <VideoInfo 
        title={videoInfo?.title}
        desc={videoInfo?.desc}
        ownerName={videoInfo?.owner_name}
        ownerFace={videoInfo?.owner_face}
        part={currentPart}
      />
      <Player src={playUrl} />
      {showPageList && (
        <PageList
          currentBvid={currentBvid}
          pageNum={pageNum}
          videoInfo={videoInfo}
          onSlideClick={handleSlideClick}
          onVideoSelect={handleVideoSelect}
        />
      )}
      {showSearchList && (
        <SearchList
          searchResults={searchResults}
          onSlideClick={handleSlideClick}
          onSortChange={handleSortChange}
          onVideoSelect={(url) => handleUrlJump(`https://www.bilibili.com/video/${url}`)}
        />
      )}
    </DefaultLayout>
  );
}
