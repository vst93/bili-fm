// Package store 的旧版迁移：把 Wails / Tauri 老版本持久化的配置与本地收藏
// 转换成新版（mygo）的键名与结构。
//
// 背景：新版与旧版共用同一份 dkv（<UserConfigDir>/bili-fm/data.db），所以
// 键名一致的项（volume / playbackRate / coverMode / ambientBackgroundEnabled /
// premiumTexture / 登录态）本来就能直接读到。真正需要迁移的是**改了名的键**
// 和**换了存储位置的收藏**：
//
//	旧版位置 / 键名                     新版键名
//	---------------------------------  --------------------------
//	dkv  playlist                       userPlaylist
//	dkv  playlist_play_mode             playlistPlayMode
//	dkv  sponsorSkip                    sponsor_skip（"true" -> "1"）
//	dkv  incognitoMode                  incognito
//	Tauri playlist.json userPlaylist    userPlaylist
//	Tauri playlist.json seriesPlaylist  seriesPlaylist
//	Tauri currentPlaylistIndex          playingPlaylistType/playingPlaylistIndex
//	Tauri currentSeriesPlaylistIndex    （同上，type=series）
//
// 旧版把主题（light/dark）、推荐/历史 tab、迷你窗位置、本地断点只写进了
// webview 的 localStorage（Chromium LevelDB / WebKit 站点数据），跨平台格式
// 差异大、解析成本高且容易误伤，这里不做读取；新版这些项要么不持久化
// （主题按时间自动切换），要么会在用户下次操作时重新落盘。
//
// 迁移是**保守且幂等**的：
//   - 已迁移过（存在 marker 键）直接跳过；
//   - 逐个目标键写入前先检查，目标键已存在（用户已在新版产生数据）就绝不覆盖；
//   - 迁移前把读到的旧数据落到新版目录下的 migrated-from-old.json 作为备份，
//     并把 Tauri 的 playlist.json 复制一份为 playlist.json.bak。
package store

import (
	"encoding/json"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"time"
)

// 迁移相关常量。
const (
	// migratedMarkerKey 标记迁移已执行（幂等）。
	migratedMarkerKey = "migrated_from_old"
	// migratedBackupFile 是旧数据备份文件名（放在新版数据目录下）。
	migratedBackupFile = "migrated-from-old.json"
	// tauriIdentifier 与 src-tauri/tauri.conf.json 的 identifier 一致。
	tauriIdentifier = "com.bili-fm.app"
)

// 旧版 dkv 键名。
const (
	legacyKeyPlaylist      = "playlist"
	legacyKeyPlayMode      = "playlist_play_mode"
	legacyKeySponsorSkip   = "sponsorSkip"
	legacyKeyIncognitoMode = "incognitoMode"
)

// 新版 dkv 键名（与 app/playlist.go、app/main.go 保持一致）。
const (
	keyUserPlaylist   = "userPlaylist"
	keySeriesPlaylist = "seriesPlaylist"
	keyPlayMode       = "playlistPlayMode"
	keyPlayingList    = "playingPlaylistType"
	keyPlayingIndex   = "playingPlaylistIndex"
	keySponsorSkip    = "sponsor_skip"
	keyIncognito      = "incognito"
)

// MigrateOptions 控制迁移的数据来源，便于测试注入临时目录。
type MigrateOptions struct {
	// LegacyDir 是旧版 dkv 目录；为空表示与目标 KV 同目录（新旧共用一份）。
	LegacyDir string
	// TauriDataDir 是 Tauri 的 app_data_dir（内含 playlist.json）；为空表示按
	// 当前平台推断。
	TauriDataDir string
	// DisableTauri 为真时不读取 Tauri 的 playlist.json。
	DisableTauri bool
	// Log 输出迁移过程；nil 时静默。
	Log func(format string, args ...any)
}

// MigrateReport 描述一次迁移的结果，供启动日志与无头测试断言。
type MigrateReport struct {
	Ran        bool     // 是否真的执行了迁移
	Reason     string   // 执行/跳过的原因
	Applied    []string // 实际写入的新键
	Sources    []string // 读取过的旧数据位置
	BackupPath string   // 旧数据备份文件
}

// legacyPlayItem 是旧版（Wails/Tauri）播放列表条目的 JSON 结构。
type legacyPlayItem struct {
	ID         string `json:"id"`
	Bvid       string `json:"bvid"`
	Aid        int64  `json:"aid"`
	Cid        int64  `json:"cid"`
	Part       string `json:"part"`
	FirstFrame string `json:"first_frame"`
	Title      string `json:"title"`
	Pic        string `json:"pic"`
}

// newPlayItem 对应新版 view.PlayItem 的 JSON（字段无 tag，序列化用导出名）。
type newPlayItem struct {
	ID         string `json:"ID"`
	Bvid       string `json:"Bvid"`
	Aid        int64  `json:"Aid"`
	Cid        int64  `json:"Cid"`
	Part       string `json:"Part"`
	FirstFrame string `json:"FirstFrame"`
	Title      string `json:"Title"`
	Pic        string `json:"Pic"`
}

// DefaultTauriDataDir 返回 Tauri v2 的 app_data_dir（BaseDirectory::AppData）。
//
//	Linux:   $XDG_DATA_HOME/com.bili-fm.app 或 ~/.local/share/com.bili-fm.app
//	macOS:   ~/Library/Application Support/com.bili-fm.app
//	Windows: %APPDATA%\com.bili-fm.app
func DefaultTauriDataDir() string {
	switch runtime.GOOS {
	case "darwin":
		if home, err := os.UserHomeDir(); err == nil {
			return filepath.Join(home, "Library", "Application Support", tauriIdentifier)
		}
	case "windows":
		if appData := os.Getenv("APPDATA"); appData != "" {
			return filepath.Join(appData, tauriIdentifier)
		}
	default:
		if xdg := os.Getenv("XDG_DATA_HOME"); xdg != "" {
			return filepath.Join(xdg, tauriIdentifier)
		}
		if home, err := os.UserHomeDir(); err == nil {
			return filepath.Join(home, ".local", "share", tauriIdentifier)
		}
	}
	return ""
}

// Migrate 执行一次性迁移。返回的 Report 可用于日志/测试。
func Migrate(kv *KV, opts MigrateOptions) MigrateReport {
	logf := opts.Log
	if logf == nil {
		logf = func(string, ...any) {}
	}
	rep := MigrateReport{}

	if kv.Get(migratedMarkerKey) != nil {
		rep.Reason = "already-migrated"
		logf("store: 旧版迁移已执行过，跳过")
		return rep
	}

	legacyDir := opts.LegacyDir
	if legacyDir == "" {
		legacyDir = kv.Path()
	}

	// 1. 读旧版 dkv（与新 dkv 同格式，可能就在同一目录）。
	old := map[string]any{}
	if legacy, err := Open(legacyDir, true); err == nil {
		for _, k := range []string{legacyKeyPlaylist, legacyKeyPlayMode, legacyKeySponsorSkip, legacyKeyIncognitoMode} {
			if v := legacy.Get(k); v != nil {
				old[k] = v
			}
		}
		rep.Sources = append(rep.Sources, legacyDir)
	}

	// 2. 读 Tauri plugin-store 的 playlist.json。
	var tauri map[string]any
	tauriPath := ""
	if !opts.DisableTauri {
		dir := opts.TauriDataDir
		if dir == "" {
			dir = DefaultTauriDataDir()
		}
		if dir != "" {
			tauriPath = filepath.Join(dir, "playlist.json")
			if data, err := os.ReadFile(tauriPath); err == nil {
				var m map[string]any
				if json.Unmarshal(data, &m) == nil && len(m) > 0 {
					tauri = m
					rep.Sources = append(rep.Sources, tauriPath)
				}
			}
		}
	}

	if len(old) == 0 && len(tauri) == 0 {
		rep.Reason = "no-old-data"
		logf("store: 未发现旧版数据，跳过迁移")
		return rep
	}

	// 3. 备份旧数据。
	rep.BackupPath = writeBackup(kv, legacyDir, old, tauriPath, tauri, logf)

	// 4. 转换并写入（逐字段、目标已存在则跳过）。
	apply := func(key, value string) {
		if kv.Get(key) != nil {
			logf("store: 目标键 %s 已有数据，保持不动", key)
			return
		}
		if err := kv.SetString(key, value); err != nil {
			logf("store: 写入 %s 失败: %v", key, err)
			return
		}
		rep.Applied = append(rep.Applied, key)
	}

	userItems := extractItems(tauri, "userPlaylist")
	if len(userItems) == 0 {
		userItems = parseLegacyItems(old[legacyKeyPlaylist])
	}
	seriesItems := extractItems(tauri, "seriesPlaylist")

	if len(userItems) > 0 {
		if b, err := json.Marshal(userItems); err == nil {
			apply(keyUserPlaylist, string(b))
		}
	}
	if len(seriesItems) > 0 {
		if b, err := json.Marshal(seriesItems); err == nil {
			apply(keySeriesPlaylist, string(b))
		}
	}

	if mode, ok := old[legacyKeyPlayMode].(string); ok && validPlayMode(mode) {
		apply(keyPlayMode, mode)
	}
	if v, ok := old[legacyKeySponsorSkip].(string); ok {
		switch v {
		case "true":
			apply(keySponsorSkip, "1")
		case "false":
			apply(keySponsorSkip, "0")
		}
	}
	if v, ok := old[legacyKeyIncognitoMode].(string); ok && (v == "true" || v == "false") {
		apply(keyIncognito, v)
	}

	// 旧 Tauri 分别存用户/合集列表的当前下标，新版只存一个「来源 + 下标」。
	if idx, ok := number(tauri["currentPlaylistIndex"]); ok && len(userItems) > 0 {
		apply(keyPlayingList, "user")
		apply(keyPlayingIndex, strconv.Itoa(idx))
	} else if idx, ok := number(tauri["currentSeriesPlaylistIndex"]); ok && len(seriesItems) > 0 {
		apply(keyPlayingList, "series")
		apply(keyPlayingIndex, strconv.Itoa(idx))
	}

	// 5. 打标记，保证幂等。
	_ = kv.SetString(migratedMarkerKey, time.Now().Format(time.RFC3339))
	rep.Ran = true
	rep.Reason = "migrated"
	logf("store: 旧版迁移完成，写入 %d 个键: %v", len(rep.Applied), rep.Applied)
	return rep
}

// writeBackup 把读到的旧数据落到新版目录下的 migrated-from-old.json，并把
// Tauri 的 playlist.json 复制成 .bak。失败只记日志，不影响迁移本身。
func writeBackup(kv *KV, legacyDir string, old map[string]any, tauriPath string, tauri map[string]any, logf func(string, ...any)) string {
	if tauriPath != "" {
		if data, err := os.ReadFile(tauriPath); err == nil {
			if err := os.WriteFile(tauriPath+".bak", data, 0o644); err != nil {
				logf("store: 备份 %s 失败: %v", tauriPath, err)
			}
		}
	}
	backup := map[string]any{
		"migratedAt": time.Now().Format(time.RFC3339),
		"legacyDir":  legacyDir,
		"legacyDkv":  old,
		"tauriStore": tauri,
	}
	b, err := json.MarshalIndent(backup, "", "  ")
	if err != nil {
		return ""
	}
	if err := os.MkdirAll(kv.Path(), 0o755); err != nil {
		return ""
	}
	path := filepath.Join(kv.Path(), migratedBackupFile)
	if err := os.WriteFile(path, b, 0o644); err != nil {
		logf("store: 写备份 %s 失败: %v", path, err)
		return ""
	}
	return path
}

// parseLegacyItems 解析旧 dkv 里的 playlist 值。旧版存的是 JSON 字符串，
// 但为稳妥也接受已经是数组的情况。
func parseLegacyItems(v any) []newPlayItem {
	switch t := v.(type) {
	case string:
		if t == "" {
			return nil
		}
		var arr []any
		if json.Unmarshal([]byte(t), &arr) != nil {
			return nil
		}
		return convertItems(arr)
	case []any:
		return convertItems(t)
	}
	return nil
}

// extractItems 从 Tauri playlist.json 的对象里取一个数组并转换。
func extractItems(m map[string]any, key string) []newPlayItem {
	if m == nil {
		return nil
	}
	arr, ok := m[key].([]any)
	if !ok {
		return nil
	}
	return convertItems(arr)
}

// convertItems 把旧版条目（下划线字段）转成新版条目（导出字段名）。
// 已经是新版格式（含 ID/Bvid）的条目原样保留，保证重复调用安全。
func convertItems(arr []any) []newPlayItem {
	out := make([]newPlayItem, 0, len(arr))
	for _, raw := range arr {
		obj, ok := raw.(map[string]any)
		if !ok {
			continue
		}
		if _, isNew := obj["Bvid"]; isNew {
			b, _ := json.Marshal(obj)
			var it newPlayItem
			if json.Unmarshal(b, &it) == nil {
				out = append(out, it)
			}
			continue
		}
		var old legacyPlayItem
		b, err := json.Marshal(obj)
		if err != nil || json.Unmarshal(b, &old) != nil {
			continue
		}
		if old.Cid == 0 && old.Bvid == "" {
			continue
		}
		out = append(out, newPlayItem{
			ID:         old.ID,
			Bvid:       old.Bvid,
			Aid:        old.Aid,
			Cid:        old.Cid,
			Part:       old.Part,
			FirstFrame: old.FirstFrame,
			Title:      old.Title,
			Pic:        old.Pic,
		})
	}
	return out
}

// number 把 JSON 解出来的数字（float64）转成 int；非整数/非法返回 false。
func number(v any) (int, bool) {
	switch n := v.(type) {
	case float64:
		if n < 0 {
			return 0, false
		}
		return int(n), true
	case int:
		if n < 0 {
			return 0, false
		}
		return n, true
	}
	return 0, false
}

// validPlayMode 只接受新版认识的三种模式。
func validPlayMode(mode string) bool {
	switch mode {
	case "sequence", "single", "shuffle":
		return true
	}
	return false
}
