import React, { useEffect, useState, useRef } from "react";
import { Close, Minus, ZoomInternal } from "@icon-park/react";
import { invoke } from "@tauri-apps/api/core";
import { listen } from "@tauri-apps/api/event";
import { open } from "@tauri-apps/plugin-shell";
import { check, type Update } from "@tauri-apps/plugin-updater";
import { relaunch } from "@tauri-apps/plugin-process";

import type { UpdateResult } from "@/types/bilibili";
import { useDialog } from "./dialog/DialogProvider";

interface TitleBarProps {
  onSwitchMode?: () => void;
  showSwitchMode?: boolean;
}

const APP_VERSION = "2.0.0";
const APP_VERSION_NO = 200;

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
  const settingsBtnRef = useRef<HTMLButtonElement>(null);
  const { showDialog, closeDialog } = useDialog();
  const [updateProgress, setUpdateProgress] =
    useState<UpdateProgressState | null>(null);

  useEffect(() => {
    invoke<string>("get_platform").then((platform: string) => {
      setIsMac(platform === "darwin");
    });
    const handleResize = () => {
      setWindowWidth(window.innerWidth);
      setShowMenu(false);
    };
    window.addEventListener('resize', handleResize);

    // Mac native menu events (Tauri EventsEmit)
    const onMenuAbout = () => handleShowAbout();
    const onMenuShortcuts = () => handleShowKeyboardShortcuts();
    const unlistenAbout = listen("menu:show-about", onMenuAbout);
    const unlistenShortcuts = listen("menu:show-shortcuts", onMenuShortcuts);
    window.addEventListener('menu:show-about', onMenuAbout as EventListener);
    window.addEventListener('menu:show-shortcuts', onMenuShortcuts as EventListener);

    return () => {
      window.removeEventListener('resize', handleResize);
      window.removeEventListener('menu:show-about', onMenuAbout as EventListener);
      window.removeEventListener('menu:show-shortcuts', onMenuShortcuts as EventListener);
      unlistenAbout.then((fn) => fn());
      unlistenShortcuts.then((fn) => fn());
    };
  }, []);

  const handleClose = () => {
    invoke("hide_window");
  };

  const handleExit = () => {
    invoke("quit_app");
  };

  const handleMinimize = () => {
    invoke("minimize_window");
  };

  const handleShowAbout = () => {
    setShowMenu(false);
    showDialog({
      title: "关于 bili-FM",
      type: "info",
      message: `用音频聆听 B 站内容，既是音乐播放器，也是知识学习工具。\n\n版本 v${APP_VERSION} (Build ${APP_VERSION_NO})\n项目地址：[github.com/vst93/bili-fm](https://github.com/vst93/bili-fm)`,
      buttons: [{ label: "好的", value: "ok", primary: true }],
    });
  };

  /**
   * 通过 tauri-plugin-updater 检查并下载安装更新
   */
  const handleDownloadUpdate = async (update: Update) => {
    setUpdateProgress({ version: update.version, downloaded: 0, total: 0 });
    try {
      let downloaded = 0;
      let total = 0;
      await update.download((event) => {
        if (event.event === "Started") {
          total = event.data.contentLength ?? 0;
        } else if (event.event === "Progress") {
          downloaded += event.data.chunkLength;
        }
        setUpdateProgress({ version: update.version, downloaded, total });
      });
      await update.install();
      setUpdateProgress(null);
      showDialog({
        title: "更新完成",
        type: "success",
        message: "新版本已安装，应用即将重启",
        buttons: [{ label: "好的", value: "ok", primary: true }],
      });
      await relaunch();
    } catch (error: any) {
      console.error("更新失败:", error);
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

  /**
   * 检查更新：立即弹出加载对话框，并行执行两条检查链路
   * 1. tauri-plugin-updater check() — 端点 Gitee 优先、GitHub 回退，支持应用内下载安装
   * 2. check_for_updates 旧命令 — Gitee API 优先、GitHub API 回退，仅返回版本信息
   */
  const handleCheckUpdate = async () => {
    setShowMenu(false);

    // 立即显示加载对话框，不等网络返回
    const loadingId = showDialog({
      title: "检查更新",
      type: "loading",
      message: "正在检查更新...",
      buttons: [], // 无按钮 = 不可手动关闭，等待结果替换
    });

    try {
      const [rustResult, pluginUpdate] = await Promise.allSettled([
        invoke<UpdateResult>("check_for_updates", { isManual: true, gitFrom: "" }),
        check(),
      ]);

      // 用结果对话框替换加载对话框
      closeDialog(loadingId);

      const rust = rustResult.status === "fulfilled" ? rustResult.value : null;
      const plugin = pluginUpdate.status === "fulfilled" ? pluginUpdate.value : null;

      // 两条链路都失败
      if (rust?.error && !plugin?.available) {
        showDialog({
          title: "检查更新失败",
          type: "error",
          message: rust.error,
          buttons: [{ label: "确定", value: "ok", primary: true }],
        });
        return;
      }

      if (!plugin?.available && !rust?.hasUpdate) {
        showDialog({
          title: "检查更新",
          type: "success",
          message: "当前已是最新版本",
          buttons: [{ label: "好的", value: "ok", primary: true }],
        });
        return;
      }

      const version = plugin?.version || rust?.latestVersion || "";

      if (plugin?.available) {
        // updater 插件可用：应用内下载并安装
        showDialog({
          title: "发现新版本",
          type: "question",
          message: `新版本 v${version} 已发布\n是否立即下载并安装更新？`,
          buttons: [
            { label: "立即更新", value: "yes", primary: true },
            { label: "稍后再说", value: "no" },
          ],
          onClose: (value: string) => {
            if (value === "yes") {
              handleDownloadUpdate(plugin);
            }
          },
        });
      } else if (rust?.hasUpdate) {
        // 仅版本检查可用：跳转下载页（Gitee 优先，失败回退 GitHub）
        showDialog({
          title: "发现新版本",
          type: "question",
          message: `新版本 v${version} 已发布\n前往下载页面获取最新版本`,
          buttons: [
            { label: "前往下载", value: "yes", primary: true },
            { label: "稍后再说", value: "no" },
          ],
          onClose: (value: string) => {
            if (value === "yes") {
              open(rust.downloadUrl);
            }
          },
        });
      }
    } catch (error: any) {
      closeDialog(loadingId);
      showDialog({
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
      onClick={handleSwitchMode}
    >
      <ZoomInternal size="14" theme="outline" />
    </button>
  );

  return (
    <>
      <div className="app-title-bar h-12 flex items-center relative" data-tauri-drag-region="deep">
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
                    className="app-title-bar-btn"
                    onClick={handleMinimize}
                  >
                    <Minus size="14" theme="outline" />
                  </button>
                  <button
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
