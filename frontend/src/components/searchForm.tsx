import { FC } from "react";
import { Avatar, Input } from "@nextui-org/react";
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
  };

  return (
    <div className="flex items-center justify-center gap-2 p-2">
      <div className="w-[300px]">
        <Input
          value={value}
          onValueChange={onInputChange}
          onKeyDown={handleKeyDown}
          placeholder="B站｜关键词搜索"
          size="md"
          radius="md"
          classNames={{
            base: "max-w-full",
            mainWrapper: "h-12",
            input: "text-small",
            inputWrapper: "h-12 pr-0",
          }}
          endContent={
            <button
              className="bg-transparent border-none cursor-pointer h-12 w-12 flex items-center justify-center hover:bg-default-100/50 active:bg-default-200/70 transition-colors border-l border-default-200"
              onClick={() => onSearch?.(value)}
            >
              <Search theme="outline" size="20" fill="#333" />
            </button>
          }
        />
      </div>
      {userFace ? (
        <Avatar
          src={userFace}
          className="cursor-pointer"
          onClick={onLoginClick}
          size="md"
          isBordered
          classNames={{
            img: "opacity-100",
          }}
        />
      ) : (
        <Avatar
          showFallback
          className="cursor-pointer"
          onClick={onLoginClick}
          size="md"
          isBordered
        />
      )}
    </div>
  );
};

export default SearchForm;
