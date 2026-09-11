import { useState, useEffect } from "react";
import { Button, Card, CardBody, CardHeader, Chip, Link, Divider } from "@heroui/react";
import { Download, Github, Info, Upload } from "@icon-park/react";
import { invoke } from "@tauri-apps/api/core";

import type { AppVersion } from "@/types/bilibili";
import type { PlaylistItem, PlaylistPlayMode } from "@/components/playlist";
import DefaultLayout from "@/layouts/default";
import { toast } from "@/utils/toast";

// 备份文件标识与版本，导入时据此校验。
const BACKUP_APP = "bili-fm";
const BACKUP_VERSION = 1;
// 导入文件大小上限 5MB，与 Rust 侧保持一致（双端防呆）。
const IMPORT_MAX_BYTES = 5 * 1024 * 1024;

interface PlaylistBackup {
  app: string;
  version: number;
  exportedAt: string;
  playlist: PlaylistItem[];
  playMode: PlaylistPlayMode;
}

/** 粗校验单条播放项结构：必须有可作唯一键的 id 及必要字段。 */
function isValidPlaylistItem(item: unknown): item is PlaylistItem {
  if (!item || typeof item !== "object") return false;
  const it = item as Record<string, unknown>;
  return (
    typeof it.id === "string" &&
    it.id.length > 0 &&
    typeof it.bvid === "string" &&
    typeof it.aid === "number" &&
    typeof it.cid === "number"
  );
}

function isValidPlayMode(mode: unknown): mode is PlaylistPlayMode {
  return mode === "sequence" || mode === "single" || mode === "shuffle";
}

export default function AboutPage() {
  const [appVersion, setAppVersion] = useState<AppVersion | null>(null);
  const [importing, setImporting] = useState(false);
  const [exporting, setExporting] = useState(false);

  useEffect(() => {
    invoke<AppVersion>("get_app_version").then(setAppVersion).catch(console.error);
  }, []);

  // 导出：组装全量歌单 + 播放模式的备份 JSON，写入用户选择的文件。
  const handleExport = async () => {
    setExporting(true);
    try {
      const [rawPlaylist, playMode] = await Promise.all([
        invoke<string>("get_playlist"),
        invoke<string>("get_playlist_play_mode"),
      ]);
      let playlist: PlaylistItem[] = [];
      try {
        const parsed = rawPlaylist ? JSON.parse(rawPlaylist) : [];
        if (Array.isArray(parsed)) playlist = parsed;
      } catch {
        playlist = [];
      }
      const backup: PlaylistBackup = {
        app: BACKUP_APP,
        version: BACKUP_VERSION,
        exportedAt: new Date().toISOString(),
        playlist,
        playMode: isValidPlayMode(playMode) ? playMode : "sequence",
      };
      const fileName = await invoke<string | null>("export_playlist_to_file", {
        content: JSON.stringify(backup, null, 2),
      });
      if (fileName) {
        toast({ type: "success", content: `已导出到 ${fileName}` });
      }
      // fileName 为 null 表示用户取消，静默。
    } catch (e) {
      toast({ type: "error", content: `导出失败: ${String(e)}` });
    } finally {
      setExporting(false);
    }
  };

  // 导入：读文件 -> 校验 -> 按 id 去重合并 -> 经 set_playlist 写回。
  const handleImport = async () => {
    setImporting(true);
    try {
      const text = await invoke<string | null>("import_playlist_from_file");
      if (text === null) return; // 用户取消，静默
      if (text.length > IMPORT_MAX_BYTES) {
        toast({ type: "error", content: "文件过大，已超过 5MB 上限" });
        return;
      }
      let data: unknown;
      try {
        data = JSON.parse(text);
      } catch {
        toast({ type: "error", content: "文件格式不正确" });
        return;
      }
      if (!data || typeof data !== "object") {
        toast({ type: "error", content: "文件格式不正确" });
        return;
      }
      const obj = data as Record<string, unknown>;
      const isBiliFmBackup = obj.app === BACKUP_APP;
      const rawList = obj.playlist;
      if (!isBiliFmBackup && !Array.isArray(rawList)) {
        toast({ type: "error", content: "文件格式不正确" });
        return;
      }
      const incoming: PlaylistItem[] = Array.isArray(rawList)
        ? (rawList.filter(isValidPlaylistItem) as PlaylistItem[])
        : [];
      if (incoming.length === 0) {
        toast({ type: "error", content: "文件中没有可导入的歌曲" });
        return;
      }

      // 读取现有列表，按 id 去重合并（保留用户现有内容，不覆盖）。
      const rawExisting = await invoke<string>("get_playlist");
      let existing: PlaylistItem[] = [];
      try {
        const parsed = rawExisting ? JSON.parse(rawExisting) : [];
        if (Array.isArray(parsed)) existing = parsed;
      } catch {
        existing = [];
      }
      const seen = new Set(existing.map((item) => item.id));
      const merged = [...existing];
      let added = 0;
      for (const item of incoming) {
        if (seen.has(item.id)) continue;
        seen.add(item.id);
        merged.push(item);
        added += 1;
      }
      const skipped = incoming.length - added;

      await invoke("set_playlist", { playlistJson: JSON.stringify(merged) });

      // 播放模式仅在文件合法、且用户当前列表为空时才采用，否则保留现状。
      if (
        isBiliFmBackup &&
        isValidPlayMode(obj.playMode) &&
        existing.length === 0 &&
        merged.length > 0
      ) {
        await invoke("set_playlist_play_mode", { mode: obj.playMode });
      }

      // 通知已挂载的播放列表同步内存态（与存储保持一致）。
      window.dispatchEvent(
        new CustomEvent("bili-fm:playlist-updated", { detail: merged }),
      );

      toast({
        type: "success",
        content: `成功导入 ${added} 首（跳过 ${skipped} 首重复）`,
      });
    } catch (e) {
      toast({ type: "error", content: `导入失败: ${String(e)}` });
    } finally {
      setImporting(false);
    }
  };

  const features = [
    "输入关键词搜索视频",
    "登录后查看订阅、收藏、推荐",
    "点击 UP 主查看作品列表",
    "支持点赞和投币",
    "系统媒体控制支持",
    "迷你模式",
  ];

  return (
    <DefaultLayout>
      <div className="max-w-4xl mx-auto py-8 space-y-8">
        <div className="text-center space-y-4">
          <h1 className="text-3xl font-bold text-gray-900 dark:text-white">
            关于 bili-FM
          </h1>
          <p className="text-lg text-gray-600 dark:text-gray-400 max-w-2xl mx-auto">
            通过音频来听 B 站节目，你可以把它作为一个音乐播放器，也可以用来作为知识学习的工具。
          </p>
          <div className="flex justify-center gap-4">
            <Chip color="primary" variant="flat" size="lg">
              版本: {appVersion?.version ?? "..."}
            </Chip>
          </div>
        </div>

        <div className="grid md:grid-cols-2 gap-6">
          <Card className="h-fit">
            <CardHeader className="pb-2">
              <div className="flex items-center gap-2">
                <Info fill="#666" size={20} theme="outline" />
                <h2 className="text-xl font-semibold">功能特性</h2>
              </div>
            </CardHeader>
            <CardBody>
              <ul className="space-y-2">
                {features.map((feature, index) => (
                  <li key={index} className="flex items-center gap-2 text-gray-700 dark:text-gray-300">
                    <span className="w-1.5 h-1.5 rounded-full bg-blue-500 flex-shrink-0" />
                    {feature}
                  </li>
                ))}
              </ul>
            </CardBody>
          </Card>

          <Card className="h-fit">
            <CardHeader className="pb-2">
              <h2 className="text-xl font-semibold">项目链接</h2>
            </CardHeader>
            <CardBody>
              <div className="flex flex-col gap-3">
                <Button
                  color="primary"
                  variant="flat"
                  isLoading={exporting}
                  startContent={<Download size={18} />}
                  onPress={handleExport}
                  className="justify-start"
                >
                  导出歌单
                </Button>
                <Button
                  color="secondary"
                  variant="flat"
                  isLoading={importing}
                  startContent={<Upload size={18} />}
                  onPress={handleImport}
                  className="justify-start"
                >
                  导入歌单
                </Button>
                <p className="text-xs text-gray-500 dark:text-gray-400">
                  导出为 JSON 备份文件，导入时按视频去重合并。
                </p>
                <Link
                  isExternal
                  href="https://github.com/vst93/bili-fm"
                  color="primary"
                  className="flex items-center gap-2"
                >
                  <Github size={18} />
                  GitHub
                </Link>
                <Link
                  isExternal
                  href="https://gitee.com/vst93/bili-fm"
                  color="secondary"
                  className="flex items-center gap-2"
                >
                  Gitee
                </Link>
              </div>
            </CardBody>
          </Card>
        </div>

        <div className="text-center text-sm text-gray-500 dark:text-gray-400 pt-4">
          <Divider className="mb-4" />
          <p>
            本项目仅用于学习和研究。如果存在侵权，请联系我们删除。
          </p>
          <p className="mt-1">
            感谢{" "}
            <Link isExternal href="https://tauri.app" size="sm" color="primary">
              Tauri
            </Link>
            {" / "}
            <Link isExternal href="https://heroui.com" size="sm" color="primary">
              HeroUI
            </Link>
            {" / "}
            <Link
              isExternal
              href="https://github.com/SocialSisterYi/bilibili-API-collect"
              size="sm"
              color="primary"
            >
              bilibili-API-collect
            </Link>
            {" "}等开源项目
          </p>
        </div>
      </div>
    </DefaultLayout>
  );
}
