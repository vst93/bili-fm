package store

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

// oldItem 是旧版播放列表条目的 JSON（下划线字段）。
func oldItemJSON(id, bvid string, aid, cid int64, part, firstFrame, title, pic string) map[string]any {
	return map[string]any{
		"id": id, "bvid": bvid, "aid": aid, "cid": cid,
		"part": part, "first_frame": firstFrame, "title": title, "pic": pic,
	}
}

// writeLegacyDkv 造一份旧版 dkv 数据。
func writeLegacyDkv(t *testing.T, dir string, values map[string]any) {
	t.Helper()
	db, err := Open(dir, false)
	if err != nil {
		t.Fatal(err)
	}
	for k, v := range values {
		if err := db.Set(k, v); err != nil {
			t.Fatalf("写旧键 %s: %v", k, err)
		}
	}
}

func newKV(t *testing.T) *KV {
	t.Helper()
	kv, err := Open(t.TempDir(), false)
	if err != nil {
		t.Fatal(err)
	}
	return kv
}

// TestMigrateFromLegacyDkv：Wails/Tauri 老版把收藏存在 dkv 的 playlist 键，
// 迁移后应出现在新键 userPlaylist，且条目字段转成新版结构。
func TestMigrateFromLegacyDkv(t *testing.T) {
	legacyDir := t.TempDir()
	items := []any{
		oldItemJSON("BV1-1", "BV1", 11, 1, "P1", "ff1", "视频一", "pic1"),
		oldItemJSON("BV2-2", "BV2", 22, 2, "P2", "ff2", "视频二", "pic2"),
	}
	blob, _ := json.Marshal(items)
	writeLegacyDkv(t, legacyDir, map[string]any{
		legacyKeyPlaylist:      string(blob),
		legacyKeyPlayMode:      "shuffle",
		legacyKeySponsorSkip:   "true",
		legacyKeyIncognitoMode: "true",
	})

	kv := newKV(t)
	rep := Migrate(kv, MigrateOptions{LegacyDir: legacyDir, DisableTauri: true})
	if !rep.Ran || rep.Reason != "migrated" {
		t.Fatalf("expected migration to run, got %+v", rep)
	}

	// 用户歌单。
	var got []map[string]any
	if err := json.Unmarshal([]byte(kv.String(keyUserPlaylist)), &got); err != nil {
		t.Fatalf("userPlaylist 不是 JSON: %v", err)
	}
	if len(got) != 2 {
		t.Fatalf("userPlaylist 条目数 = %d, want 2", len(got))
	}
	first := got[0]
	for _, k := range []string{"ID", "Bvid", "Aid", "Cid", "Part", "FirstFrame", "Title", "Pic"} {
		if _, ok := first[k]; !ok {
			t.Errorf("新版条目缺少字段 %s: %v", k, first)
		}
	}
	if first["Bvid"] != "BV1" || first["FirstFrame"] != "ff1" || first["Aid"] != float64(11) {
		t.Errorf("条目字段值不对: %v", first)
	}
	// 不能残留旧的下划线字段。
	if _, ok := first["first_frame"]; ok {
		t.Errorf("仍残留旧字段 first_frame: %v", first)
	}

	// 播放模式 / SponsorBlock / 隐身。
	if got := kv.String(keyPlayMode); got != "shuffle" {
		t.Errorf("playlistPlayMode = %q, want shuffle", got)
	}
	if got := kv.String(keySponsorSkip); got != "1" {
		t.Errorf("sponsor_skip = %q, want 1", got)
	}
	if got := kv.String(keyIncognito); got != "true" {
		t.Errorf("incognito = %q, want true", got)
	}
	if kv.Get(migratedMarkerKey) == nil {
		t.Error("缺少迁移标记")
	}
	if rep.BackupPath == "" {
		t.Error("缺少备份文件")
	}
	if _, err := os.Stat(rep.BackupPath); err != nil {
		t.Errorf("备份文件不存在: %v", err)
	}
}

// TestMigrateFromTauriStore：Tauri 新版把收藏放在 app_data_dir/playlist.json，
// 迁移后应写进新 dkv。
func TestMigrateFromTauriStore(t *testing.T) {
	tauriDir := t.TempDir()
	store := map[string]any{
		"userPlaylist": []any{
			oldItemJSON("BV1-1", "BV1", 11, 1, "P1", "ff1", "视频一", "pic1"),
		},
		"seriesPlaylist": []any{
			oldItemJSON("BV9-9", "BV9", 99, 9, "P9", "ff9", "合集九", "pic9"),
		},
		"currentPlaylistIndex":       0,
		"currentSeriesPlaylistIndex": 0,
	}
	b, _ := json.Marshal(store)
	if err := os.WriteFile(filepath.Join(tauriDir, "playlist.json"), b, 0o644); err != nil {
		t.Fatal(err)
	}

	kv := newKV(t)
	rep := Migrate(kv, MigrateOptions{LegacyDir: t.TempDir(), TauriDataDir: tauriDir})
	if !rep.Ran {
		t.Fatalf("expected migration to run, got %+v", rep)
	}
	if kv.String(keyUserPlaylist) == "" || kv.String(keySeriesPlaylist) == "" {
		t.Fatal("Tauri 歌单未迁移")
	}
	if got := kv.String(keyPlayingList); got != "user" {
		t.Errorf("playingPlaylistType = %q, want user", got)
	}
	if got := kv.String(keyPlayingIndex); got != "0" {
		t.Errorf("playingPlaylistIndex = %q, want 0", got)
	}
	// playlist.json 应被备份。
	if _, err := os.Stat(filepath.Join(tauriDir, "playlist.json.bak")); err != nil {
		t.Errorf("playlist.json.bak 不存在: %v", err)
	}
}

// TestMigrateIdempotent：迁移两次，第二次必须跳过。
func TestMigrateIdempotent(t *testing.T) {
	legacyDir := t.TempDir()
	blob, _ := json.Marshal([]any{oldItemJSON("BV1-1", "BV1", 1, 1, "P1", "ff", "t", "p")})
	writeLegacyDkv(t, legacyDir, map[string]any{legacyKeyPlaylist: string(blob)})

	kv := newKV(t)
	first := Migrate(kv, MigrateOptions{LegacyDir: legacyDir, DisableTauri: true})
	if !first.Ran {
		t.Fatalf("first migration should run: %+v", first)
	}
	before := kv.String(keyUserPlaylist)

	second := Migrate(kv, MigrateOptions{LegacyDir: legacyDir, DisableTauri: true})
	if second.Ran || second.Reason != "already-migrated" {
		t.Fatalf("second migration should be skipped, got %+v", second)
	}
	if len(second.Applied) != 0 {
		t.Errorf("second migration applied %v", second.Applied)
	}
	if kv.String(keyUserPlaylist) != before {
		t.Error("第二次迁移改动了数据")
	}
}

// TestMigrateDoesNotOverwrite：新版已有数据时绝不覆盖（保守策略）。
func TestMigrateDoesNotOverwrite(t *testing.T) {
	legacyDir := t.TempDir()
	blob, _ := json.Marshal([]any{oldItemJSON("OLD-1", "OLD", 1, 1, "P", "ff", "旧", "p")})
	writeLegacyDkv(t, legacyDir, map[string]any{
		legacyKeyPlaylist:    string(blob),
		legacyKeySponsorSkip: "true",
	})

	kv := newKV(t)
	// 用户已在新版里建了自己的歌单，并关了 SponsorBlock。
	own := `[{"ID":"NEW-1","Bvid":"NEW","Aid":7,"Cid":7,"Part":"","FirstFrame":"","Title":"新","Pic":""}]`
	if err := kv.SetString(keyUserPlaylist, own); err != nil {
		t.Fatal(err)
	}
	if err := kv.SetString(keySponsorSkip, "0"); err != nil {
		t.Fatal(err)
	}

	rep := Migrate(kv, MigrateOptions{LegacyDir: legacyDir, DisableTauri: true})
	if !rep.Ran {
		t.Fatalf("migration should still run for other keys: %+v", rep)
	}
	if kv.String(keyUserPlaylist) != own {
		t.Error("新版 userPlaylist 被旧数据覆盖了")
	}
	if kv.String(keySponsorSkip) != "0" {
		t.Error("新版 sponsor_skip 被旧数据覆盖了")
	}
	for _, k := range rep.Applied {
		if k == keyUserPlaylist || k == keySponsorSkip {
			t.Errorf("不应写入已存在的键 %s", k)
		}
	}
}

// TestMigrateMissingFieldsDefault：旧条目缺字段时用零值补齐，不 panic。
func TestMigrateMissingFieldsDefault(t *testing.T) {
	legacyDir := t.TempDir()
	// 只有 id/bvid/cid，缺 part/first_frame/title/pic/aid。
	items := []any{map[string]any{"id": "BV1-1", "bvid": "BV1", "cid": 1}}
	blob, _ := json.Marshal(items)
	writeLegacyDkv(t, legacyDir, map[string]any{legacyKeyPlaylist: string(blob)})

	kv := newKV(t)
	Migrate(kv, MigrateOptions{LegacyDir: legacyDir, DisableTauri: true})

	var got []map[string]any
	if err := json.Unmarshal([]byte(kv.String(keyUserPlaylist)), &got); err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 {
		t.Fatalf("条目数 = %d, want 1", len(got))
	}
	if got[0]["Part"] != "" || got[0]["Title"] != "" || got[0]["Aid"] != float64(0) {
		t.Errorf("缺字段未补默认值: %v", got[0])
	}
}

// TestMigrateNoOldData：没有任何旧数据时不写 marker、不建备份。
func TestMigrateNoOldData(t *testing.T) {
	kv := newKV(t)
	rep := Migrate(kv, MigrateOptions{LegacyDir: t.TempDir(), DisableTauri: true})
	if rep.Ran || rep.Reason != "no-old-data" {
		t.Fatalf("expected no-old-data, got %+v", rep)
	}
	if kv.Get(migratedMarkerKey) != nil {
		t.Error("无旧数据不应写迁移标记")
	}
}

// TestMigrateBackupContent：备份文件应包含读到的旧键值。
func TestMigrateBackupContent(t *testing.T) {
	legacyDir := t.TempDir()
	writeLegacyDkv(t, legacyDir, map[string]any{
		legacyKeyPlayMode:      "single",
		legacyKeySponsorSkip:   "false",
		legacyKeyIncognitoMode: "false",
	})
	kv := newKV(t)
	rep := Migrate(kv, MigrateOptions{LegacyDir: legacyDir, DisableTauri: true})
	if rep.BackupPath == "" {
		t.Fatal("no backup path")
	}
	data, err := os.ReadFile(rep.BackupPath)
	if err != nil {
		t.Fatal(err)
	}
	var backup map[string]any
	if err := json.Unmarshal(data, &backup); err != nil {
		t.Fatal(err)
	}
	old, ok := backup["legacyDkv"].(map[string]any)
	if !ok || old[legacyKeyPlayMode] != "single" {
		t.Errorf("备份内容缺旧键: %v", backup)
	}
}

// TestDefaultTauriDataDir 只在当前平台验证路径后缀，确保三平台分支都合理。
func TestDefaultTauriDataDir(t *testing.T) {
	got := DefaultTauriDataDir()
	if got == "" {
		t.Skip("当前平台无默认数据目录")
	}
	if filepath.Base(got) != tauriIdentifier {
		t.Errorf("默认目录应以 %s 结尾, got %s", tauriIdentifier, got)
	}
}
