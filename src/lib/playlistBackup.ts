import { invoke } from "@tauri-apps/api/core";

import type { PlaylistItem, PlaylistPlayMode } from "@/components/playlist";
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

// 导出：组装全量歌单 + 播放模式的备份 JSON，写入用户选择的文件。
export async function exportPlaylistToFile(): Promise<void> {
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
  }
}

// 导入：读文件 -> 校验 -> 按 id 去重合并 -> 经 set_playlist 写回。
export async function importPlaylistFromFile(): Promise<void> {
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
  }
}
