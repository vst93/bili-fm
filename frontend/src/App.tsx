import { useEffect, useState } from "react";

import { GetImageProxyPort } from "../wailsjs/go/service/BL";

import ToastContainer from "./components/toast/ToastContainer";
import TitleBar from "./components/titleBar";
import { setProxyImagePort } from "./config";

import IndexPage from "@/pages/index";

function App() {
  const [ready, setReady] = useState(false);

  // 等待代理端口初始化完成后再渲染内容
  useEffect(() => {
    GetImageProxyPort().then((port: number) => {
      setProxyImagePort(port);
      setReady(true);
    });
  }, []);

  return (
    <>
      <TitleBar />
      {ready ? <IndexPage /> : null}
      <ToastContainer />
    </>
  );
}

export default App;
