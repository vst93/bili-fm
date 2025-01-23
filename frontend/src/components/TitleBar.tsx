import React, { useEffect, useState } from "react";
import { Close, Minus } from "@icon-park/react";

import { GetPlatform } from "../../wailsjs/go/main/Menu";

const TitleBar: React.FC = () => {
  const [isMac, setIsMac] = useState(false);

  useEffect(() => {
    // @ts-ignore
    GetPlatform().then((platform: string) => {
      setIsMac(platform === "darwin");
    });
  }, []);

  const handleClose = () => {
    // @ts-ignore
    window.runtime.Quit();
  };

  const handleMinimize = () => {
    // @ts-ignore
    window.runtime.WindowMinimise();
  };

  return (
    <div className="app-title-bar h-12 flex items-center relative">
      {isMac ? (
        <>
          <div className="flex-1 flex justify-center items-center gap-0">
            <img alt="logo" className="w-8 h-8" src="/logo.png" />
            <span className="text-sm">bili-FM</span>
          </div>
        </>
      ) : (
        <>
          <div className="flex-1 flex items-center gap-2 px-0">
            <img alt="logo" className="w-8 h-8" src="/logo.png" />
            <span className="text-sm">bili-FM</span>
          </div>
          <div className="flex gap-2 px-2">
            <button
              className="hover:bg-gray-200 p-1.5 rounded transition-colors"
              onClick={handleMinimize}
            >
              <Minus fill="#333" size="14" theme="outline" />
            </button>
            <button
              className="hover:bg-red-200 hover:text-white p-1.5 rounded transition-colors"
              onClick={handleClose}
            >
              <Close fill="#333" size="14" theme="outline" />
            </button>
          </div>
        </>
      )}
    </div>
  );
};

export default TitleBar;
