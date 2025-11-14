import React, { useEffect, useState } from "react";
import { Close, Minus } from "@icon-park/react";

import { GetPlatform, ShowAbout, ShowVersion, CheckForUpdates, ShowKeyboardShortcuts } from "../../wailsjs/go/main/Menu";

const TitleBar: React.FC = () => {
  const [isMac, setIsMac] = useState(false);
  const [showMenu, setShowMenu] = useState(false);
  const [windowWidth, setWindowWidth] = useState(window.innerWidth);

  useEffect(() => {
    // @ts-ignore
    GetPlatform().then((platform: string) => {
      setIsMac(platform === "darwin");
    });
    const handleResize = () => {
      setWindowWidth(window.innerWidth);
    };
    window.addEventListener('resize', handleResize);

    // 清理函数：在组件卸载时移除事件监听器
    return () => {
      window.removeEventListener('resize', handleResize);
    };
  }, []);

  const handleClose = () => {
    // @ts-ignore
    window.runtime.Quit();
  };

  const handleMinimize = () => {
    // @ts-ignore
    window.runtime.WindowMinimise();
  };

  const handleShowAbout = async () => {
    await ShowAbout();
    setShowMenu(false);
  };

  const handleShowVersion = async () => {
    await ShowVersion();
    setShowMenu(false);
  };

  const handleCheckUpdate = async () => {
    await CheckForUpdates(true, "");
    setShowMenu(false);
  };

  const handleShowKeyboardShortcuts = async () => {
    await ShowKeyboardShortcuts();
    setShowMenu(false);
  };

  return (
    <>
      <div className="app-title-bar h-12 flex items-center relative">
        {isMac ? (
          <>
            <div
              className="flex-1 flex justify-center items-center gap-0"
              style={{
                display: windowWidth <= 600 ? 'none' : '',
              }}
            >
              <img alt="logo" className="w-8 h-8" src="/logo.png" />
              <span className="text-sm">bili-FM</span>
            </div>
          </>
        ) : (
          <>
              <div
                className="flex-1 flex items-center gap-0 px-0"
                style={{
                  display: windowWidth <= 600 ? 'none' : '',
                }}
              >
                <img alt="logo" className="w-8 h-8" src="/logo.png" />
                <span className="text-sm">bili-FM</span>
                <div
                  className="relative ml-2"
                  style={{
                    display: windowWidth <= 600 ? 'none' : 'block',
                  }}
                >
                  <button
                    aria-controls="settings-menu"
                    aria-expanded={showMenu}
                    aria-haspopup="menu"
                    className="hover:bg-gray-200 px-2 py-1 rounded transition-colors text-sm"
                    onClick={() => setShowMenu(!showMenu)}
                  >
                    设置
                  </button>
                {showMenu && (
                  <div
                    aria-label="设置菜单"
                    className="absolute top-full left-0 mt-1 bg-white rounded-lg shadow-lg py-1 min-w-[120px] z-50"
                    id="settings-menu"
                    role="menu"
                  >
                    <ul>
                      <li role="none">
                        <button
                          className="w-full text-left px-4 py-2 hover:bg-gray-100 text-sm"
                          onClick={handleShowAbout}
                          role="menuitem"
                        >
                          关于应用
                        </button>
                      </li>
                      <li role="none">
                        <button
                          className="w-full text-left px-4 py-2 hover:bg-gray-100 text-sm"
                          onClick={handleShowVersion}
                          role="menuitem"
                        >
                          当前版本
                        </button>
                      </li>
                      <li role="none">
                        <button
                          className="w-full text-left px-4 py-2 hover:bg-gray-100 text-sm"
                          onClick={handleShowKeyboardShortcuts}
                          role="menuitem"
                        >
                          快捷键
                        </button>
                      </li>
                      <li role="none">
                        <button
                          className="w-full text-left px-4 py-2 hover:bg-gray-100 text-sm"
                          onClick={handleCheckUpdate}
                          role="menuitem"
                        >
                          检查更新
                        </button>
                      </li>
                      <li role="none">
                        <button
                          className="w-full text-left px-4 py-2 hover:bg-gray-100 text-sm"
                            onClick={handleClose}
                          role="menuitem"
                        >
                          退出应用
                        </button>
                      </li>
                    </ul>
                  </div>
                )}
                </div>
              </div>
              <div className="flex gap-1 px-2" id='wds-handle-group'>
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
      {/* 点击空白处关闭菜单 */}
      {showMenu && (
        <button
          aria-hidden="true"
          className="fixed inset-0 z-40 w-full h-full bg-transparent cursor-default"
          onClick={() => setShowMenu(false)}
          tabIndex={-1}
        />
      )}
    </>
  );
};

export default TitleBar;
