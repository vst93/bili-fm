import { Form, Input, Button, Avatar } from "@nextui-org/react";
import { useState, useEffect } from "react";
import { Search } from "@icon-park/react";

interface SearchFormProps {
  onSearch: (keyword: string) => void;
  onUrlJump: (url: string) => void;
  onInputChange?: (value: string) => void;
  value?: string;
}

export default function SearchForm({
  onSearch,
  value = "",
  onInputChange,
}: SearchFormProps) {
  const [inputValue, setInputValue] = useState(value);

  const handleSearch = () => {
    onSearch(inputValue);
  };

  const handleKeyDown = (e: React.KeyboardEvent) => {
    if (e.key === "Enter") {
      // 只在按下 Enter 键时阻止事件冒泡和默认行为
      e.stopPropagation();
      e.preventDefault();
      handleSearch();
    } else if (e.key === "space" || e.key === " ") {
      e.stopPropagation();
    }
  };

  const handleInputChange = (e: React.ChangeEvent<HTMLInputElement>) => {
    const newValue = e.target.value;

    setInputValue(newValue);
    onInputChange?.(newValue);
  };

  // 当外部 value 改变时更新输入框
  useEffect(() => {
    setInputValue(value);
  }, [value]);

  return (
    <Form className="items-center" onSubmit={(e) => e.preventDefault()}>
      <div className="flex gap-2 items-center">
        <Input
          isClearable
          className="max-w-full"
          id="search-input-keywrd"
          name="b_url"
          placeholder="B站｜搜索关键词"
          type="text"
          value={inputValue}
          onChange={handleInputChange}
          onClear={() => {
            setInputValue("");
            onInputChange?.("");
          }}
          onKeyDown={handleKeyDown}
        />
        <Button onClick={handleSearch}>
          <Search fill="#333" size="24" theme="outline" />
        </Button>
        <Avatar
          isBordered
          classNames={{
            base: "h-10 w-16",
          }}
          size="md"
          style={{
            cursor: "pointer",
          }}
          title="登录或重新登录"
        >
          头像
        </Avatar>
      </div>
    </Form>
  );
}
