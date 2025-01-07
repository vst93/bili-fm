import { Form, Input, Button } from "@nextui-org/react";

export default function SearchForm() {
  return (
    <Form className="items-center">
      <div className="flex gap-2 items-center">
        <Input
          isClearable
          className="max-w-full"
          id="search-input-keywrd"
          name="b_url"
          placeholder="B站视频地址 / 搜索关键词"
          type="text"
        />
        <Button>搜索</Button>
        <Button>链接跳转</Button>
      </div>
    </Form>
  );
}
