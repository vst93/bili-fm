import React, { useEffect, useState, useRef } from "react";
import { Close, Minus, ZoomInternal } from "@icon-park/react";

import { GetPlatform, ShowAbout, ShowVersion, CheckForUpdates, ShowKeyboardShortcuts, CloseApp } from "../../wailsjs/go/main/Menu";

interface TitleBarProps {
  onSwitchMode?: () => void;
  showSwitchMode?: boolean;
}

const TitleBar: React.FC<TitleBarProps> = ({ onSwitchMode, showSwitchMode = true }) => {
  const [isMac, setIsMac] = useState(false);
  const [showMenu, setShowMenu] = useState(false);
  const [windowWidth, setWindowWidth] = useState(window.innerWidth);
  const [menuPos, setMenuPos] = useState({ top: 0, left: 0 });
  const settingsBtnRef = useRef<HTMLButtonElement>(null);

  useEffect(() => {
    // @ts-ignore
    GetPlatform().then((platform: string) => {
      setIsMac(platform === "darwin");
    });
    const handleResize = () => {
      setWindowWidth(window.innerWidth);
      setShowMenu(false);
    };
    window.addEventListener('resize', handleResize);

    return () => {
      window.removeEventListener('resize', handleResize);
    };
  }, []);

  const handleClose = () => {
    // @ts-ignore
    window.runtime.Hide();
  };

  const handleExit = () => {
    CloseApp();
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

  const handleSwitchMode = () => {
    setShowMenu(false);
    onSwitchMode?.();
  };

  const toggleMenu = () => {
    if (!showMenu && settingsBtnRef.current) {
      const rect = settingsBtnRef.current.getBoundingClientRect();
      setMenuPos({ top: rect.bottom + 4, left: rect.left });
    }
    setShowMenu(!showMenu);
  };

  const switchBtn = showSwitchMode && onSwitchMode && (
    <button
      id="switch-window-mode"
      title="切换到迷你模式"
      className="hover:bg-gray-200 p-1.5 rounded transition-colors"
      onClick={handleSwitchMode}
    >
      <ZoomInternal fill="#333" size="14" theme="outline" />
    </button>
  );

  return (
    <>
      <div className="app-title-bar h-12 flex items-center relative">
        {isMac ? (
          <>
            <div className="flex-1" />
            <div
              className="flex justify-center items-center gap-0"
              style={{
                display: windowWidth <= 600 ? 'none' : '',
              }}
            >
              <img alt="logo" className="w-8 h-8" src="/logo-transparent.png" />
              <span className="text-sm">bili-FM</span>
            </div>
            <div className="flex-1 flex justify-end items-center pr-2">
              {switchBtn}
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
                <img alt="logo" className="w-8 h-8" src="/logo-transparent.png" />
                <span className="text-sm">bili-FM</span>
                <button
                  ref={settingsBtnRef}
                  aria-controls="settings-menu"
                  aria-expanded={showMenu}
                  aria-haspopup="menu"
                  className="hover:bg-gray-200 px-2 py-1 rounded transition-colors text-sm ml-2"
                  onClick={toggleMenu}
                >
                  设置
                </button>
              </div>
              <div className="flex items-center gap-2 px-2">
                {switchBtn}
                <div className="flex gap-1" id='wds-handle-group'>
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
              </div>
          </>
        )}
      </div>
      {/* Dropdown menu — rendered outside .app-title-bar to avoid drag-region and opacity interference */}
      {showMenu && !isMac && (
        <>
          <button
            aria-hidden="true"
            className="fixed inset-0 z-40 w-full h-full bg-transparent cursor-default"
            onClick={() => setShowMenu(false)}
            tabIndex={-1}
          />
          <div
            aria-label="设置菜单"
            className="fixed z-50 rounded-lg shadow-lg py-1 min-w-[120px]"
            id="settings-menu"
            role="menu"
            style={{ top: `${menuPos.top}px`, left: `${menuPos.left}px` }}
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
                  onClick={handleExit}
                  role="menuitem"
                >
                  退出应用
                </button>
              </li>
            </ul>
          </div>
        </>
      )}
    </>
  );
};

export default TitleBar;
