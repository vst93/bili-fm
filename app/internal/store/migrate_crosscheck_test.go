package store_test

import (
	"encoding/json"
	"testing"

	"github.com/vst93/bili-fm/app/internal/store"
	"github.com/vst93/bili-fm/app/internal/view"
)

// TestMigrationMatchesViewPlayItem 交叉校验：迁移产出的 JSON 必须能被
// view.PlayItem 正确反序列化（防止迁移结构与界面类型字段名漂移）。
func TestMigrationMatchesViewPlayItem(t *testing.T) {
	legacyDir := t.TempDir()
	legacy, err := store.Open(legacyDir, false)
	if err != nil {
		t.Fatal(err)
	}
	blob := `[{"id":"BV1-1","bvid":"BV1","aid":11,"cid":1,"part":"P1","first_frame":"ff","title":"标题","pic":"p"}]`
	if err := legacy.Set("playlist", blob); err != nil {
		t.Fatal(err)
	}

	kv, err := store.Open(t.TempDir(), false)
	if err != nil {
		t.Fatal(err)
	}
	rep := store.Migrate(kv, store.MigrateOptions{LegacyDir: legacyDir, DisableTauri: true})
	if !rep.Ran {
		t.Fatalf("migration did not run: %+v", rep)
	}

	var items []view.PlayItem
	if err := json.Unmarshal([]byte(kv.String("userPlaylist")), &items); err != nil {
		t.Fatalf("userPlaylist 无法被 view.PlayItem 解析: %v", err)
	}
	if len(items) != 1 {
		t.Fatalf("条目数 = %d, want 1", len(items))
	}
	got := items[0]
	if got.ID != "BV1-1" || got.Bvid != "BV1" || got.Aid != 11 || got.Cid != 1 ||
		got.Part != "P1" || got.FirstFrame != "ff" || got.Title != "标题" || got.Pic != "p" {
		t.Errorf("迁移条目与 view.PlayItem 不一致: %+v", got)
	}
}
