import React, { useEffect, useState, useRef } from "react";
import { Close, Minus, ZoomInternal } from "@icon-park/react";

import {
  GetPlatform,
  CheckForUpdates,
  CloseApp,
  IsMSStoreInstall,
  DownloadUpdate,
  ApplyUpdate,
} from "../../wailsjs/go/main/Menu";
import { GetAppVersion } from "../../wailsjs/go/service/BL";
import { EventsOn, EventsOff } from "../../wailsjs/runtime/runtime";
import { useDialog } from "./dialog/DialogProvider";

interface TitleBarProps {
  onSwitchMode?: () => void;
  showSwitchMode?: boolean;
}

/** 下载进度状态 (更新进度浮层) */
interface UpdateProgressState {
  version: string;
  downloaded: number;
  total: number;
}

const formatBytes = (bytes: number) => {
  if (bytes <= 0) return "0 B";
  const units = ["B", "KB", "MB", "GB"];
  const i = Math.min(
    units.length - 1,
    Math.floor(Math.log(bytes) / Math.log(1024)),
  );
  return `${(bytes / 1024 ** i).toFixed(1)} ${units[i]}`;
};
const TitleBar: React.FC<TitleBarProps> = ({ onSwitchMode, showSwitchMode = true }) => {
  const [isMac, setIsMac] = useState(false);
  const [showMenu, setShowMenu] = useState(false);
  const [windowWidth, setWindowWidth] = useState(window.innerWidth);
  const [menuPos, setMenuPos] = useState({ top: 0, left: 0 });
  const [appVersion, setAppVersion] = useState("1.9.5");
  const [appVersionNo, setAppVersionNo] = useState(195);
  const settingsBtnRef = useRef<HTMLButtonElement>(null);
  const { showDialog, updateDialog } = useDialog();
  const [updateProgress, setUpdateProgress] =
    useState<UpdateProgressState | null>(null);
  // 用户点击「取消」后置位，检查完成后不再弹出任何结果对话框
  const updateCancelledRef = useRef(false);

  useEffect(() => {
    // @ts-ignore
    GetPlatform().then((platform: string) => {
      setIsMac(platform === "darwin");
    });
    // 版本号在运行时从后端获取，避免每次发版都要手动改前端
    GetAppVersion()
      .then((v) => {
        if (v?.version) setAppVersion(v.version);
        if (v?.build) setAppVersionNo(v.build);
      })
      .catch((err) => console.error("GetAppVersion 失败:", err));
    const handleResize = () => {
      setWindowWidth(window.innerWidth);
      setShowMenu(false);
    };
    window.addEventListener('resize', handleResize);

    // Mac native menu events (via Wails EventsEmit)
    const onMenuAbout = () => handleShowAbout();
    const onMenuShortcuts = () => handleShowKeyboardShortcuts();
    const onMenuCheckUpdate = () => handleCheckUpdate();
    // 使用 EventsOn（返回取消函数）并在清理时取消，避免 StrictMode 双挂载导致重复注册
    const offAbout = EventsOn("menu:show-about", onMenuAbout);
    const offShortcuts = EventsOn("menu:show-shortcuts", onMenuShortcuts);
    const offCheckUpdate = EventsOn("menu:check-update", onMenuCheckUpdate);

    return () => {
      window.removeEventListener('resize', handleResize);
      offAbout();
      offShortcuts();
      offCheckUpdate();
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
      message: `用音频聆听 B 站内容，既是音乐播放器，也是知识学习工具。\n\n版本 v${appVersion} (Build ${appVersionNo})\n项目地址：[github.com/vst93/bili-fm](https://github.com/vst93/bili-fm)`,
      buttons: [{ label: "好的", value: "ok", primary: true }],
    });
  };

  /**
   * 下载并安装更新
   * @param assetUrl 平台安装包直链
   * @param version 新版本号
   */
  const handleDownloadUpdate = async (assetUrl: string, version: string) => {
    setUpdateProgress({ version, downloaded: 0, total: 0 });

    // 监听后端下载进度事件
    EventsOn("update:progress", (data: any) => {
      const p = data?.[0] || data;
      if (typeof p?.downloaded === "number") {
        setUpdateProgress((prev) => ({
          version: prev?.version || version,
          downloaded: p.downloaded,
          total: p.total || prev?.total || 0,
        }));
      }
    });

    try {
      const filePath = await DownloadUpdate(assetUrl);
      if (!filePath) {
        throw new Error("下载失败：未返回文件路径");
      }
      EventsOff("update:progress");
      setUpdateProgress(null);

      const result = await ApplyUpdate(filePath);
      showDialog({
        title: result.success ? "更新完成" : "更新失败",
        type: result.success ? "success" : "error",
        message:
          result.message +
          "\n可前往 [GitHub Release 页面](https://github.com/vst93/bili-fm/releases/latest) 手动下载",
        buttons: [{ label: "好的", value: "ok", primary: true }],
      });
    } catch (error: any) {
      EventsOff("update:progress");
      setUpdateProgress(null);
      showDialog({
        title: "更新失败",
        type: "error",
        message:
          (error?.message || error?.toString() || "未知错误") +
          "\n可前往 [GitHub Release 页面](https://github.com/vst93/bili-fm/releases/latest) 手动下载",
        buttons: [{ label: "确定", value: "ok", primary: true }],
      });
    }
  };

  const handleCheckUpdate = async () => {
    setShowMenu(false);

    // Microsoft Store 安装的版本由商店接管更新，应用内更新不可用
    try {
      const isStore = await IsMSStoreInstall();
      if (isStore) {
        showDialog({
          title: "检查更新",
          type: "info",
          message:
            "本应用通过 Microsoft Store 安装，请通过商店进行更新。\n\n打开 Microsoft Store -> 搜索 Bili FM -> 点击更新",
          buttons: [{ label: "好的", value: "ok", primary: true }],
        });
        return;
      }
    } catch {
      // 检测失败时继续走正常更新流程
    }

    // 先立即弹出加载对话框（接口可能较慢），结束后原地更新结果
    updateCancelledRef.current = false;
    const loadingId = showDialog({
      title: "检查更新",
      type: "loading",
      message: "正在检查更新...",
      buttons: [{ label: "取消", value: "cancel" }],
      onClose: (value: string) => {
        if (value === "cancel") updateCancelledRef.current = true;
      },
    });

    try {
      const result = await CheckForUpdates(true, "");
      // 用户已取消：不再弹结果
      if (updateCancelledRef.current) return;

      if (result.error) {
        updateDialog(loadingId, {
          title: "检查更新失败",
          type: "error",
          message: result.error,
          buttons: [{ label: "确定", value: "ok", primary: true }],
        });
      } else if (result.hasUpdate) {
        updateDialog(loadingId, {
          title: "发现新版本",
          type: "question",
          message: `新版本 v${result.latestVersion} 已发布\n是否立即下载并安装更新？`,
          buttons: [
            { label: "下载更新", value: "yes", primary: true },
            { label: "稍后再说", value: "no" },
          ],
          onClose: (value: string) => {
            if (value === "yes" && result.downloadUrl) {
              handleDownloadUpdate(result.downloadUrl, result.latestVersion);
            }
          },
        });
      } else if (result.isLatest) {
        updateDialog(loadingId, {
          title: "检查更新",
          type: "success",
          message: "当前已是最新版本",
          buttons: [{ label: "好的", value: "ok", primary: true }],
        });
      }
    } catch (error: any) {
      if (updateCancelledRef.current) return;
      updateDialog(loadingId, {
        title: "检查更新失败",
        type: "error",
        message: error?.message || error?.toString() || "未知错误",
        buttons: [{ label: "确定", value: "ok", primary: true }],
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
      {/* 更新下载进度浮层 */}
      {updateProgress && (
        <div className="fixed inset-0 z-[95] flex items-center justify-center bg-black/25 backdrop-blur-sm">
          <div className="w-80 rounded-xl bg-white/95 shadow-xl p-5">
            <h3 className="text-base font-bold text-slate-800 mb-1">
              正在下载更新
            </h3>
            <p className="text-sm text-slate-500 mb-3">
              v{updateProgress.version}
            </p>
            <div className="h-2 rounded-full bg-slate-200 overflow-hidden">
              <div
                className="h-full rounded-full bg-blue-500 transition-all duration-200"
                style={{
                  width:
                    updateProgress.total > 0
                      ? `${Math.min(
                          100,
                          (updateProgress.downloaded / updateProgress.total) *
                            100,
                        )}%`
                      : "4%",
                }}
              />
            </div>
            <p className="mt-2 text-xs text-slate-400 text-right">
              {formatBytes(updateProgress.downloaded)}
              {updateProgress.total > 0
                ? ` / ${formatBytes(updateProgress.total)}`
                : ""}
            </p>
          </div>
        </div>
      )}
    </>
  );
};

export default TitleBar;
