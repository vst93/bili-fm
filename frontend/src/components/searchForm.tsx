import { FC } from "react";
import { Avatar, Input } from "@heroui/react";
import {
  ChartRing,
  History,
  Search,
  ShareSys,
  WeixinFavorites,
} from "@icon-park/react";

interface SearchFormProps {
  value?: string;
  onInputChange?: (value: string) => void;
  onSearch?: (keyword: string) => void;
  onUrlJump?: (url: string) => void;
  onLoginClick?: () => void;
  onFeedClick?: () => void;
  onRecommendClick?: () => void;
  onCollectClick?: () => void;
  onHistoryClick?: () => void;
  userFace?: string;
}

const SearchForm: FC<SearchFormProps> = ({
  value = "",
  onInputChange,
  onSearch,
  onUrlJump,
  onLoginClick,
  onFeedClick,
  onRecommendClick,
  onCollectClick,
  onHistoryClick,
  userFace,
}) => {
  const handleKeyDown = (e: React.KeyboardEvent<HTMLInputElement>) => {
    if (e.key === "Enter") {
      const inputValue = (e.target as HTMLInputElement).value;
      if (inputValue.includes("bilibili.com/video/")) {
        onUrlJump?.(inputValue);
      } else {
        onSearch?.(inputValue);
      }
    }
  };

  return (
    <div className="home-searchbar">
      <div className="home-search-input">
        <Input
          value={value}
          onValueChange={onInputChange}
          onKeyDown={handleKeyDown}
          placeholder="B站 / 关键词 / 视频链接"
          size="sm"
          radius="lg"
          spellCheck={false}
          classNames={{
            base: "max-w-full",
            mainWrapper: "h-9",
            input: "text-sm text-slate-700 placeholder:text-slate-400",
            inputWrapper:
              "pr-1 bg-white/80 border border-slate-200/90 shadow-sm data-[hover=true]:bg-white focus-within:border-sky-400 focus-within:bg-white focus-within:shadow-md transition-all",
          }}
          endContent={
            <button
              className="search-submit-btn"
              title="搜索"
              onClick={() => onSearch?.(value)}
            >
              <Search theme="outline" size={18} />
            </button>
          }
        />
      </div>
      <div className="home-global-actions" aria-label="B站内容入口">
        <button className="top-tool-btn" title="动态" onClick={onFeedClick}>
          <ShareSys size={18} theme="outline" />
        </button>
        <button className="top-tool-btn" title="热门与推荐" onClick={onRecommendClick}>
          <ChartRing size={18} theme="outline" />
        </button>
        <button className="top-tool-btn" title="收藏" onClick={onCollectClick}>
          <WeixinFavorites size={18} theme="outline" />
        </button>
        <button className="top-tool-btn" title="历史记录" onClick={onHistoryClick}>
          <History size={18} theme="outline" />
        </button>
      </div>
      {userFace ? (
        <Avatar
          src={userFace}
          className="home-user-avatar"
          onClick={onLoginClick}
          size="md"
          isBordered
          classNames={{
            img: "opacity-100",
          }}
          title="点击登录其他账号"
        />
      ) : (
        <Avatar
          showFallback
          className="home-user-avatar"
          onClick={onLoginClick}
          size="md"
          isBordered
          title="点击登录账号"
        />
      )}
    </div>
  );
};

export default SearchForm;
