import SearchForm from "./searchForm";
import VideoCover from "./videoCover";
import VideoInfo from "./videoInfo";


import DefaultLayout from "@/layouts/default";

export default function IndexPage() {
  return (
    <DefaultLayout>
      <SearchForm />
      <VideoCover />
      <VideoInfo />
    </DefaultLayout>
  );
}
