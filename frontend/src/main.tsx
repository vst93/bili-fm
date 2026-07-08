import React from "react";
import ReactDOM from "react-dom/client";

import App from "./App.tsx";
import { installDevWailsMock } from "./dev-wails-mock.ts";
import { Provider } from "./provider.tsx";
import "@/styles/globals.css";

installDevWailsMock();

ReactDOM.createRoot(document.getElementById("root")!).render(
  <React.StrictMode>
    <Provider>
      <App />
    </Provider>
  </React.StrictMode>,
);
