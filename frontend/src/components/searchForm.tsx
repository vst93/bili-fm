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

  const handleKeyDown = (e: React.KeyboardEvent) => {
    if (e.key === "Enter") {
      e.preventDefault();
      handleSearch();
    }
  };

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
          onChange={(e) => setInputValue(e.target.value)}
          onClear={() => setInputValue("")}
          onKeyDown={handleKeyDown}
        />
        <Button onClick={handleSearch}>搜索</Button>
      </div>
    </Form>
  );
}
