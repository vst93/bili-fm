import React, { useEffect, useState, useRef } from "react";
import { Close, Minus, ZoomInternal } from "@icon-park/react";
import { invoke } from "@tauri-apps/api/core";
import { listen } from "@tauri-apps/api/event";
import { open } from "@tauri-apps/plugin-shell";
import { check, type Update } from "@tauri-apps/plugin-updater";
import { relaunch } from "@tauri-apps/plugin-process";

import type { AppVersion, UpdateResult } from "@/types/bilibili";
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
  // 版本号在运行时从后端获取 (Cargo.toml / tauri.conf.json)
  const [isMac, setIsMac] = useState(false);
  const [isLinux, setIsLinux] = useState(navigator.userAgent.includes("Linux"));
  const [showMenu, setShowMenu] = useState(false);
  const [windowWidth, setWindowWidth] = useState(window.innerWidth);
  const [menuPos, setMenuPos] = useState({ top: 0, left: 0 });
  const settingsBtnRef = useRef<HTMLButtonElement>(null);
  const versionRef = useRef({ version: "", build: 0 });
  const { showDialog, closeDialog, updateDialog } = useDialog();
  // 用户点击「取消」后置位，检查完成后不再弹出任何结果对话框
  const updateCancelledRef = useRef(false);
  const [updateProgress, setUpdateProgress] =
    useState<UpdateProgressState | null>(null);

  useEffect(() => {
    invoke<AppVersion>("get_app_version").then((v) => {
      versionRef.current = { version: v.version, build: v.build };
    });
    invoke<string>("get_platform").then((platform: string) => {
      setIsMac(platform === "darwin");
      setIsLinux(platform === "linux");
    });
    const handleResize = () => {
      setWindowWidth(window.innerWidth);
      setShowMenu(false);
    };
    window.addEventListener('resize', handleResize);

    // Mac native menu events (Tauri EventsEmit)
    const onMenuAbout = () => handleShowAbout();
    const onMenuShortcuts = () => handleShowKeyboardShortcuts();
    const onMenuCheckUpdate = () => handleCheckUpdate();
    const unlistenAbout = listen("menu:show-about", onMenuAbout);
    const unlistenShortcuts = listen("menu:show-shortcuts", onMenuShortcuts);
    const unlistenCheckUpdate = listen("menu:check-update", onMenuCheckUpdate);
    window.addEventListener('menu:show-about', onMenuAbout as EventListener);
    window.addEventListener('menu:show-shortcuts', onMenuShortcuts as EventListener);

    return () => {
      window.removeEventListener('resize', handleResize);
      window.removeEventListener('menu:show-about', onMenuAbout as EventListener);
      window.removeEventListener('menu:show-shortcuts', onMenuShortcuts as EventListener);
      unlistenAbout.then((fn) => fn());
      unlistenShortcuts.then((fn) => fn());
      unlistenCheckUpdate.then((fn) => fn());
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
    const version = versionRef.current;
    showDialog({
      title: "关于 bili-FM",
      type: "info",
      message: `用音频聆听 B 站内容，既是音乐播放器，也是知识学习工具。\n\n版本 v${version.version}\n项目地址：[github.com/vst93/bili-fm](https://github.com/vst93/bili-fm)`,
      buttons: [{ label: "好的", value: "ok", primary: true }],
    });
  };

  /**
   * 通过 tauri-plugin-updater 检查并下载安装更新
   * @param dialogId 若提供，则在同一个对话框内展示下载进度与完成/失败结果（不关闭重开）
   */
  const handleDownloadUpdate = async (update: Update, dialogId?: number) => {
    if (dialogId === undefined) {
      setUpdateProgress({ version: update.version, downloaded: 0, total: 0 });
    }
    try {
      let downloaded = 0;
      let total = 0;
      await update.download((event) => {
        if (event.event === "Started") {
          total = event.data.contentLength ?? 0;
        } else if (event.event === "Progress") {
          downloaded += event.data.chunkLength;
        }
        if (dialogId !== undefined) {
          // 同一对话框内原地更新下载进度
          const pct =
            total > 0 ? Math.min(100, Math.round((downloaded / total) * 100)) : 0;
          updateDialog(dialogId, {
            title: "正在下载更新",
            type: "loading",
            message:
              total > 0
                ? `正在下载 v${update.version}... ${pct}%\n${formatBytes(downloaded)} / ${formatBytes(total)}`
                : `正在下载 v${update.version}... ${formatBytes(downloaded)}`,
            buttons: [],
          });
        } else {
          setUpdateProgress({ version: update.version, downloaded, total });
        }
      });
      await update.install();
      if (dialogId !== undefined) {
        updateDialog(dialogId, {
          title: "更新完成",
          type: "success",
          message: "新版本已安装，应用即将重启",
          buttons: [{ label: "好的", value: "ok", primary: true }],
        });
      } else {
        setUpdateProgress(null);
        showDialog({
          title: "更新完成",
          type: "success",
          message: "新版本已安装，应用即将重启",
          buttons: [{ label: "好的", value: "ok", primary: true }],
        });
      }
      await relaunch();
    } catch (error: any) {
      console.error("更新失败:", error);
      const message =
        (error?.message || error?.toString() || "未知错误") +
        "\n可前往 [GitHub Release 页面](https://github.com/vst93/bili-fm/releases/latest) 手动下载";
      if (dialogId !== undefined) {
        updateDialog(dialogId, {
          title: "更新失败",
          type: "error",
          message,
          buttons: [{ label: "确定", value: "ok", primary: true }],
        });
      } else {
        setUpdateProgress(null);
        showDialog({
          title: "更新失败",
          type: "error",
          message,
          buttons: [{ label: "确定", value: "ok", primary: true }],
        });
      }
    }
  };

  /**
   * 检查更新：立即弹出加载对话框，并行执行两条检查链路
   * 1. tauri-plugin-updater check() — macOS/Windows/deb/rpm 支持应用内下载安装
   * 2. check_for_updates 旧命令 — Gitee API 优先、GitHub API 回退，仅返回版本信息
   */
  const handleCheckUpdate = async () => {
    setShowMenu(false);

    // Microsoft Store 安装的版本由商店接管更新，应用内更新不可用
    try {
      const isStore = await invoke<boolean>("is_ms_store_install");
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

    updateCancelledRef.current = false;

    // 立即显示加载对话框，不等网络返回；提供「取消」按钮避免慢网络下无法退出
    const loadingId = showDialog({
      title: "检查更新",
      type: "loading",
      message: "正在检查更新...",
      buttons: [
        {
          label: "取消",
          value: "cancel",
          // 立即置位取消标记（不等 200ms 关闭动画），避免与检查完成的竞态
          onClick: (id) => {
            updateCancelledRef.current = true;
            closeDialog(id, "cancel");
          },
        },
      ],
      onClose: (value: string) => {
        if (value === "cancel") {
          updateCancelledRef.current = true;
        }
      },
    });

    try {
      // Tauri 不支持 pacman 包；识别失败时在 Linux 上保守地回退到手动升级。
      const isPacmanSystem = await invoke<boolean>("is_pacman_system").catch(
        () => isLinux,
      );
      const linuxUpdaterTarget = isLinux
        ? await invoke<string | null>("get_linux_updater_target").catch(() => null)
        : null;
      const pluginCheck = isPacmanSystem
        ? Promise.resolve(null)
        : check(linuxUpdaterTarget ? { target: linuxUpdaterTarget } : undefined);
      const [rustResult, pluginUpdate] = await Promise.allSettled([
        invoke<UpdateResult>("check_for_updates", { isManual: true, gitFrom: "" }),
        pluginCheck,
      ]);

      // 用户已取消：不弹出任何结果对话框
      if (updateCancelledRef.current) {
        return;
      }

      const rust = rustResult.status === "fulfilled" ? rustResult.value : null;
      const plugin = pluginUpdate.status === "fulfilled" ? pluginUpdate.value : null;

      // 两条链路都失败
      if (rust?.error && !plugin?.available) {
        updateDialog(loadingId, {
          title: "检查更新失败",
          type: "error",
          message: rust.error,
          buttons: [{ label: "确定", value: "ok", primary: true }],
        });
        return;
      }

      if (!plugin?.available && !rust?.hasUpdate) {
        updateDialog(loadingId, {
          title: "检查更新",
          type: "success",
          message: "当前已是最新版本",
          buttons: [{ label: "好的", value: "ok", primary: true }],
        });
        return;
      }

      const version = plugin?.version || rust?.latestVersion || "";

      if (plugin?.available) {
        // updater 插件可用：应用内下载并安装。
        // 「立即更新」保持对话框打开，下载进度与完成状态在同一对话框内原地更新。
        updateDialog(loadingId, {
          title: "发现新版本",
          type: "question",
          message: `新版本 v${version} 已发布\n是否立即下载并安装更新？`,
          buttons: [
            {
              label: "立即更新",
              value: "yes",
              primary: true,
              onClick: (id) => handleDownloadUpdate(plugin, id),
            },
            { label: "稍后再说", value: "no" },
          ],
        });
      } else if (rust?.hasUpdate) {
        // pacman 包和 updater 不可用时跳转下载页。
        updateDialog(loadingId, {
          title: "发现新版本",
          type: "question",
          message: isPacmanSystem
            ? `新版本 v${version} 已发布\nArch 系统请重新运行安装脚本，或前往下载页面升级`
            : `新版本 v${version} 已发布\n前往下载页面获取最新版本`,
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
      aria-label="切换到迷你模式"
      title="切换到迷你模式"
      onClick={handleSwitchMode}
    >
      <ZoomInternal size="15" theme="outline" />
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
              <img alt="logo" className="w-8 h-8" src="/logo-transparent.png" loading="eager" decoding="async" />
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
                <img alt="logo" className="w-8 h-8" src="/logo-transparent.png" loading="eager" decoding="async" />
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
                <div className="flex items-center pr-1">{switchBtn}</div>
                <div className="flex items-center gap-1 px-1 py-0.5" id="wds-handle-group">
                  <button
                    aria-label="最小化"
                    className="app-title-bar-btn"
                    title="最小化"
                    onClick={handleMinimize}
                  >
                    <Minus size="14" theme="outline" />
                  </button>
                  <button
                    aria-label="隐藏窗口"
                    className="app-title-bar-btn app-title-bar-close"
                    title="隐藏窗口"
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
