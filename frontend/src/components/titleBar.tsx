import React, { useEffect, useState, useRef } from "react";
import { Close, Minus, ZoomInternal } from "@icon-park/react";

import { GetPlatform, CheckForUpdates, CloseApp } from "../../wailsjs/go/main/Menu";
import { BrowserOpenURL } from "../../wailsjs/runtime/runtime";
import { useDialog } from "./dialog/DialogProvider";

interface TitleBarProps {
  onSwitchMode?: () => void;
  showSwitchMode?: boolean;
}

const APP_VERSION = "1.9.5";
const APP_VERSION_NO = 195;

const TitleBar: React.FC<TitleBarProps> = ({ onSwitchMode, showSwitchMode = true }) => {
  const [isMac, setIsMac] = useState(false);
  const [showMenu, setShowMenu] = useState(false);
  const [windowWidth, setWindowWidth] = useState(window.innerWidth);
  const [menuPos, setMenuPos] = useState({ top: 0, left: 0 });
  const settingsBtnRef = useRef<HTMLButtonElement>(null);
  const showDialog = useDialog();

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

    // Mac native menu events (via Wails EventsEmit)
    const onMenuAbout = () => handleShowAbout();
    const onMenuShortcuts = () => handleShowKeyboardShortcuts();
    // @ts-ignore
    if (window.runtime?.EventsOn) {
      // @ts-ignore
      window.runtime.EventsOn("menu:show-about", onMenuAbout);
      // @ts-ignore
      window.runtime.EventsOn("menu:show-shortcuts", onMenuShortcuts);
    }
    window.addEventListener('menu:show-about', onMenuAbout as EventListener);
    window.addEventListener('menu:show-shortcuts', onMenuShortcuts as EventListener);

    return () => {
      window.removeEventListener('resize', handleResize);
      window.removeEventListener('menu:show-about', onMenuAbout as EventListener);
      window.removeEventListener('menu:show-shortcuts', onMenuShortcuts as EventListener);
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

  const handleShowAbout = () => {
    setShowMenu(false);
    showDialog({
      title: "关于 bili-FM",
      type: "info",
      message: `用音频聆听 B 站内容，既是音乐播放器，也是知识学习工具。\n\n版本 v${APP_VERSION} (Build ${APP_VERSION_NO})\n开源项目：github.com/vst93/bili-fm`,
      buttons: [{ label: "好的", value: "ok", primary: true }],
    });
  };

  const handleCheckUpdate = async () => {
    setShowMenu(false);
    const result = await CheckForUpdates(true, "");
    if (result.error) {
      showDialog({
        title: "检查更新失败",
        type: "error",
        message: result.error,
        buttons: [{ label: "确定", value: "ok", primary: true }],
      });
    } else if (result.hasUpdate) {
      showDialog({
        title: "发现新版本",
        type: "question",
        message: `新版本 v${result.latestVersion} 已发布\n是否前往下载？`,
        buttons: [
          { label: "前往下载", value: "yes", primary: true },
          { label: "稍后再说", value: "no" },
        ],
        onClose: (value: string) => {
          if (value === "yes") {
            BrowserOpenURL(result.downloadUrl);
          }
        },
      });
    } else if (result.isLatest) {
      showDialog({
        title: "检查更新",
        type: "success",
        message: "当前已是最新版本",
        buttons: [{ label: "好的", value: "ok", primary: true }],
      });
    }
  };

  const handleShowKeyboardShortcuts = () => {
    setShowMenu(false);
    showDialog({
      title: "快捷键",
      type: "info",
      message: "播放 / 暂停：空格键\n上一首：←\n下一首：→\n最小化：Ctrl/Cmd + W\n退出：Ctrl/Cmd + Q",
      buttons: [{ label: "知道了", value: "ok", primary: true }],
    });
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
      style={{ ['--wails-draggable' as string]: 'no-drag' }}
      onClick={handleSwitchMode}
    >
      <ZoomInternal size="14" theme="outline" />
    </button>
  );

  return (
    <>
      <div className="app-title-bar h-12 flex items-center relative">
        {isMac ? (
          <>
            <div className="flex-1" />
            <div
              className="title-bar-brand flex justify-center items-center gap-0"
              style={{
                display: windowWidth <= 600 ? 'none' : '',
              }}
            >
              <img alt="logo" className="w-8 h-8" src="/logo-transparent.png" />
              <span className="text-sm">bili-FM</span>
            </div>
            <div className="flex-1 flex justify-end items-center pr-2" style={{ opacity: 1 }}>
              {switchBtn}
            </div>
          </>
        ) : (
          <>
              <div
                className="title-bar-brand flex-1 flex items-center gap-0 px-0"
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
              <div className="flex items-center gap-0" style={{ opacity: 1 }}>
                <div className="flex items-center gap-1 px-1 py-0.5" id="wds-handle-group">
                  {switchBtn}
                  <button
                    style={{ ['--wails-draggable' as string]: 'no-drag' }}
                    className="app-title-bar-btn"
                    onClick={handleMinimize}
                  >
                    <Minus size="14" theme="outline" />
                  </button>
                  <button
                    style={{ ['--wails-draggable' as string]: 'no-drag' }}
                    className="app-title-bar-btn app-title-bar-close"
                    onClick={handleClose}
                  >
                    <Close size="14" theme="outline" />
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
