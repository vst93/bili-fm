import React from "react";
import ReactDOM from "react-dom/client";

import App from "./App.tsx";
import { Provider } from "./provider.tsx";
import "@/styles/globals.css";

// 全局禁用右键：不弹 webview 自带的上下文菜单（"重新加载""检查元素"等），
// 那是浏览器语义，出现在桌面应用里很突兀。
//
// Tauri 没有跨平台的开关，只能在前端拦事件；三个平台的 webview
// (WebView2 / WKWebView / webkit2gtk) 都遵守 contextmenu 的 preventDefault。
// 用捕获阶段注册，避免被某个组件先 stopPropagation 掉。
document.addEventListener(
  "contextmenu",
  (event) => event.preventDefault(),
  { capture: true },
);

ReactDOM.createRoot(document.getElementById("root")!).render(
  <React.StrictMode>
    <Provider>
      <App />
    </Provider>
  </React.StrictMode>,
);
