//go:build linux

package mediactl

import (
	"testing"

	"github.com/godbus/dbus/v5"
)

func TestTrackSlugIsValidObjectPath(t *testing.T) {
	titles := []string{
		"",
		"【Hi-Res】 2026破亿热门歌曲合集🔥",
		"a/b\\c d?e",
		"纯中文标题",
		"plain",
	}
	for _, title := range titles {
		p := dbus.ObjectPath("/org/bilifm/track/" + trackSlug(title))
		if !p.IsValid() {
			t.Errorf("trackSlug(%q) → 无效对象路径 %q", title, p)
		}
	}
}
