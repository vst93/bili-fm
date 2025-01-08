import { Form, Input, Button } from "@nextui-org/react";
import { useState } from "react";

interface SearchFormProps {
  onSearch: (keyword: string) => void;
  onUrlJump: (url: string) => void;
}

export default function SearchForm({ onSearch, onUrlJump }: SearchFormProps) {
  const [inputValue, setInputValue] = useState("");

  const handleSearch = () => {
    onSearch(inputValue);
  };

  const handleUrlJump = () => {
    onUrlJump(inputValue);
  };

  return (
    <Form className="items-center" onSubmit={(e) => e.preventDefault()}>
      <div className="flex gap-2 items-center">
        <Input
          isClearable
          className="max-w-full"
          id="search-input-keywrd"
          name="b_url"
          placeholder="B站视频地址 / 搜索关键词"
          type="text"
          value={inputValue}
          onClear={() => setInputValue("")}
          onChange={(e) => setInputValue(e.target.value)}
        />
        <Button onClick={handleSearch}>搜索</Button>
        <Button onClick={handleUrlJump}>链接跳转</Button>
      </div>
    </Form>
  );
}
