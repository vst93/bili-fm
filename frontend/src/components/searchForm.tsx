import { FC } from "react";
import { Avatar, Input } from "@heroui/react";
import { Search } from "@icon-park/react";

interface SearchFormProps {
  value?: string;
  onInputChange?: (value: string) => void;
  onSearch?: (keyword: string) => void;
  onUrlJump?: (url: string) => void;
  onLoginClick?: () => void;
  userFace?: string;
}

const SearchForm: FC<SearchFormProps> = ({
  value = "",
  onInputChange,
  onSearch,
  onUrlJump,
  onLoginClick,
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

    if ((e.ctrlKey || e.metaKey) && e.key === "a") {
      e.preventDefault();
      e.currentTarget.select();
    } else if ((e.ctrlKey || e.metaKey) && e.key === "c") {
      e.preventDefault();
      e.currentTarget.select();
    } else if ((e.ctrlKey || e.metaKey) && e.key === "v") {
      e.preventDefault();
      navigator.clipboard.readText().then((text) => {
        onInputChange?.(text);
      });
    }
  };

  return (
    <div className="flex items-center justify-center gap-3 p-3">
      <div className="w-[360px]">
        <Input
          value={value}
          onValueChange={onInputChange}
          onKeyDown={handleKeyDown}
          placeholder="B站｜关键词搜索"
          size="md"
          radius="lg"
          spellCheck={false}
          classNames={{
            base: "max-w-full",
            mainWrapper: "h-11",
            input: "text-sm text-gray-700",
            inputWrapper: "h-11 pr-0 bg-gray-50 border border-gray-200 focus-within:border-blue-400 focus-within:shadow-sm transition-all",
          }}
          endContent={
            <button
              className="h-9 px-3 cursor-pointer flex items-center justify-center text-gray-500 hover:text-blue-500 transition-colors rounded-md"
              onClick={() => onSearch?.(value)}
            >
              <Search theme="outline" size={18} />
            </button>
          }
        />
      </div>
      {userFace ? (
        <Avatar
          src={userFace}
          className="cursor-pointer ring-2 ring-blue-400/30 hover:ring-blue-500 transition-all"
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
          className="cursor-pointer ring-2 ring-gray-300 hover:ring-blue-400 transition-all"
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
