import { useEffect } from "react";

import { GetImageProxyPort } from "../wailsjs/go/service/BL";

import ToastContainer from "./components/toast/ToastContainer";
import TitleBar from "./components/titleBar";
import { setProxyImagePort } from "./config";

import IndexPage from "@/pages/index";

function App() {
  // 在组件初始化时调用后端方法获取端口号
  useEffect(() => {
    // 假设后端提供了一个 GetImageProxyPort 方法
    GetImageProxyPort().then((port: number) => {
      setProxyImagePort(port);
    });
  }, []);

  return (
    <>
      <TitleBar />
      <IndexPage />
      <ToastContainer />
    </>
  );
}

export default App;
