package imagecache

import (
	"bytes"
	"context"
	"fmt"
	"image"
	"image/png"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/egoist/mygo/ui"
)

// TestDiskCacheSurvivesRestartWithoutHTTP 钉住磁盘层的核心价值：关掉服务器
// （真正离线）后，新缓存实例仍从磁盘拿到同一张图，且没有任何 HTTP 请求。
func TestDiskCacheSurvivesRestartWithoutHTTP(t *testing.T) {
	dir := t.TempDir()
	srv, hits := countServer(t)
	cfg := DiskConfig{Dir: dir}

	c1 := NewWithDisk(nil, cfg)
	c1.Client = srv.Client()
	u := srv.URL + "/persist.png"
	c1.Bitmap(u, WidthCard)
	waitIdle(t, c1)
	if c1.Get(u, WidthCard) == nil {
		t.Fatalf("首次抓取未就绪: %+v", c1.Stats())
	}
	if st := c1.Stats(); st.Fetches != 1 {
		t.Fatalf("首次抓取 Fetches=%d, want 1", st.Fetches)
	}
	if info := c1.DiskInfo(); !info.Enabled || info.Entries == 0 {
		t.Fatalf("磁盘条目未写入: %+v", info)
	}
	// 文件名只能是哈希：目录里不能出现原始 URL / 主机名 / 路径。
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	for _, de := range entries {
		name := de.Name()
		if strings.Contains(name, "persist") || strings.Contains(name, "127.0.0.1") ||
			strings.Contains(name, "http") || strings.Contains(name, ".png") {
			t.Fatalf("磁盘文件名泄露了原始 URL: %q", name)
		}
		if !validDiskHash(strings.TrimSuffix(name, ".img")) {
			t.Fatalf("磁盘文件名不是哈希: %q", name)
		}
	}

	// 关掉服务器：磁盘命中必须是唯一能拿到图的途径。
	srv.Close()
	c2 := NewWithDisk(nil, cfg)
	c2.Bitmap(u, WidthCard)
	waitIdle(t, c2)
	if c2.Get(u, WidthCard) == nil {
		t.Fatalf("新实例没有从磁盘读到离线缓存: %+v", c2.Stats())
	}
	if st := c2.Stats(); st.Fetches != 0 || st.DiskHits == 0 {
		t.Fatalf("磁盘命中不应发 HTTP：%+v", st)
	}
	if got := hits("/persist.png"); got != 1 {
		t.Fatalf("离线复用仍发了 HTTP：命中 %d 次", got)
	}
}

// animatedFallbackServer 对 .webp 返回动图（解不了），对 .jpg 返回可解码
// 的静态图，模拟 B 站「原地址动图、转码地址 JPEG」的行为。
func animatedFallbackServer(t *testing.T) (*httptest.Server, *atomic.Int32, *atomic.Int32) {
	t.Helper()
	var webpHits, jpgHits atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.HasSuffix(r.URL.Path, ".jpg") {
			jpgHits.Add(1)
			w.Header().Set("Content-Type", "image/png")
			w.Write(coverPNG())
			return
		}
		webpHits.Add(1)
		w.Header().Set("Content-Type", "image/webp")
		w.Write(animatedWebP())
	}))
	t.Cleanup(srv.Close)
	return srv, &webpHits, &jpgHits
}

func localAnimatedTranscode(raw string, width int) string {
	return strings.TrimSuffix(ThumbURL(raw, width), ".webp") + ".jpg"
}

// TestAnimatedFallbackDiskBeforeOriginalHTTP 钉住动图头像的兜底图按磁盘持久化：
// 重启后原地址磁盘未命中时，必须**先**从兜底 JPG 的磁盘条目恢复，而不是先
// 发 HTTP 原地址。服务器关掉（离线）仍能显示一头像。
func TestAnimatedFallbackDiskBeforeOriginalHTTP(t *testing.T) {
	dir := t.TempDir()
	srv, webpHits, jpgHits := animatedFallbackServer(t)
	u := srv.URL + "/face.webp"

	c1 := NewWithDisk(nil, DiskConfig{Dir: dir})
	c1.Client = srv.Client()
	c1.transcodeURL = localAnimatedTranscode
	c1.Bitmap(u, WidthAvatar)
	waitIdle(t, c1)
	if c1.Get(u, WidthAvatar) == nil {
		t.Fatalf("首次动图转码未就绪: %+v", c1.Stats())
	}
	if webpHits.Load() != 1 || jpgHits.Load() != 1 {
		t.Fatalf("首次应各请求一次：webp=%d jpg=%d", webpHits.Load(), jpgHits.Load())
	}
	if info := c1.DiskInfo(); info.Entries != 1 {
		t.Fatalf("应只持久化兜底 JPG 一条: %+v", info)
	}

	srv.Close() // 真正离线
	c2 := NewWithDisk(nil, DiskConfig{Dir: dir})
	c2.Client = srv.Client()
	c2.transcodeURL = localAnimatedTranscode
	c2.Bitmap(u, WidthAvatar)
	waitIdle(t, c2)
	if c2.Get(u, WidthAvatar) == nil {
		t.Fatalf("离线重启没有从兜底 JPG 磁盘恢复: %+v", c2.Stats())
	}
	if st := c2.Stats(); st.Fetches != 0 || st.DiskHits == 0 {
		t.Fatalf("离线兜底应只读磁盘、不发 HTTP：%+v", st)
	}
	if webpHits.Load() != 1 || jpgHits.Load() != 1 {
		t.Fatalf("离线恢复又发了 HTTP：webp=%d jpg=%d", webpHits.Load(), jpgHits.Load())
	}
}

// TestCorruptAnimatedFallbackFallsBackToNetwork 钉住兜底图损坏时不会让头像
// 永久空白：删掉坏文件后照常走原地址 HTTP，再走转码地址重试并重新落盘。
func TestCorruptAnimatedFallbackFallsBackToNetwork(t *testing.T) {
	dir := t.TempDir()
	srv, webpHits, jpgHits := animatedFallbackServer(t)
	u := srv.URL + "/face.webp"

	c1 := NewWithDisk(nil, DiskConfig{Dir: dir})
	c1.Client = srv.Client()
	c1.transcodeURL = localAnimatedTranscode
	c1.Bitmap(u, WidthAvatar)
	waitIdle(t, c1)
	if c1.Get(u, WidthAvatar) == nil {
		t.Fatal("首次动图转码未就绪")
	}

	c2 := NewWithDisk(nil, DiskConfig{Dir: dir})
	c2.Client = srv.Client()
	c2.transcodeURL = localAnimatedTranscode
	fallback := localAnimatedTranscode(u, WidthAvatar)
	if err := os.WriteFile(filepath.Join(dir, diskKeyURL(fallback)+".img"), []byte("garbage"), 0o600); err != nil {
		t.Fatal(err)
	}

	c2.Bitmap(u, WidthAvatar)
	waitIdle(t, c2)
	if c2.Get(u, WidthAvatar) == nil {
		t.Fatalf("兜底文件损坏后没有回退网络: %+v", c2.Stats())
	}
	if webpHits.Load() != 2 || jpgHits.Load() != 2 {
		t.Fatalf("损坏后应重新请求原图 + 转码图：webp=%d jpg=%d", webpHits.Load(), jpgHits.Load())
	}
	if info := c2.DiskInfo(); info.Entries != 1 {
		t.Fatalf("转码重试成功后应重新落盘一条: %+v", info)
	}
}

// TestPurgeIsRAMOnlyDiskStillUsable 钉住 Purge 只清 RAM（含在途请求），
// 不删磁盘；恢复后同一 URL 直接复用磁盘，不发 HTTP。
func TestPurgeIsRAMOnlyDiskStillUsable(t *testing.T) {
	dir := t.TempDir()
	srv, hits := countServer(t)
	c := NewWithDisk(nil, DiskConfig{Dir: dir})
	c.Client = srv.Client()
	u := srv.URL + "/purge.png"

	c.Bitmap(u, WidthCard)
	waitIdle(t, c)
	if c.Get(u, WidthCard) == nil {
		t.Fatal("首次抓取未就绪")
	}
	c.Purge()
	if c.Len() != 0 || c.Bytes() != 0 || c.Pending() != 0 {
		t.Fatalf("Purge 后 RAM 未清空：len=%d bytes=%d pending=%d", c.Len(), c.Bytes(), c.Pending())
	}
	c.Bitmap(u, WidthCard)
	waitIdle(t, c)
	if c.Get(u, WidthCard) == nil {
		t.Fatal("Purge 后没有从磁盘恢复")
	}
	if st := c.Stats(); st.Fetches != 1 || st.DiskHits == 0 {
		t.Fatalf("Purge 后应磁盘复用、不发 HTTP：%+v", st)
	}
	if got := hits("/purge.png"); got != 1 {
		t.Fatalf("Purge 后重新发了 HTTP：%d 次", got)
	}
}

// TestDiskTTLDoesNotExtendOnAccess 钉住：TTL 从落盘时刻算，读命中只更新
// 访问时间（LRU），绝不续期。最近访问过的条目到期也必须失效并回网络。
func TestDiskTTLDoesNotExtendOnAccess(t *testing.T) {
	dir := t.TempDir()
	srv, hits := countServer(t)

	var clock atomic.Int64
	base := time.Date(2026, 1, 2, 3, 4, 5, 0, time.UTC)
	clock.Store(base.UnixNano())
	now := func() time.Time { return time.Unix(0, clock.Load()) }
	cfg := DiskConfig{Dir: dir, TTL: 2 * time.Hour, Now: now}

	c1 := NewWithDisk(nil, cfg)
	c1.Client = srv.Client()
	u := srv.URL + "/ttl.png"
	c1.Bitmap(u, WidthCard)
	waitIdle(t, c1)
	if c1.Get(u, WidthCard) == nil {
		t.Fatal("首次抓取未就绪")
	}

	// 1 小时后访问（磁盘仍未过期）：这只是“最近访问”，不是新下载。
	clock.Store(base.Add(time.Hour).UnixNano())
	c2 := NewWithDisk(nil, cfg)
	c2.Bitmap(u, WidthCard)
	waitIdle(t, c2)
	if c2.Get(u, WidthCard) == nil {
		t.Fatal("TTL 内没有从磁盘恢复")
	}
	if st := c2.Stats(); st.Fetches != 0 || st.DiskHits == 0 {
		t.Fatalf("TTL 内应磁盘命中：%+v", st)
	}

	// 落盘 3 小时后（最近访问过也救不了）：必须过期并重新发 HTTP。
	clock.Store(base.Add(3 * time.Hour).UnixNano())
	c3 := NewWithDisk(nil, cfg)
	if info := c3.DiskInfo(); info.Entries != 0 {
		t.Fatalf("过期条目启动时未被清掉: %+v", info)
	}
	c3.Bitmap(u, WidthCard)
	waitIdle(t, c3)
	if c3.Get(u, WidthCard) == nil {
		t.Fatal("过期后没有回网络重新抓取")
	}
	if got := hits("/ttl.png"); got != 2 {
		t.Fatalf("TTL 过期后应重新请求（命中 %d 次，want 2）", got)
	}
}

// TestDiskCapacityEvictsLeastRecentlyUsed 钉住磁盘容量淘汰按访问时间：
// A 在 B 之后被访问，加入 C 时应淘汰 B 而不是 A。
func TestDiskCapacityEvictsLeastRecentlyUsed(t *testing.T) {
	dir := t.TempDir()
	srv, hits := countServer(t)
	entrySize := int64(diskHeaderSize + len(coverPNG()))
	cfg := DiskConfig{Dir: dir, MaxBytes: 2 * entrySize} // 刚好放两张

	c := NewWithDisk(nil, cfg)
	c.Client = srv.Client()
	a := srv.URL + "/lru-a.png"
	b := srv.URL + "/lru-b.png"
	c.Bitmap(a, WidthCard)
	waitIdle(t, c)
	c.Bitmap(b, WidthCard)
	waitIdle(t, c)
	if info := c.DiskInfo(); info.Entries != 2 {
		t.Fatalf("应已存 A/B 两条: %+v", info)
	}

	// 清 RAM，让 A 真正从磁盘读一次，刷新 A 的磁盘访问时间。
	c.Purge()
	c.Bitmap(a, WidthCard)
	waitIdle(t, c)
	if c.Get(a, WidthCard) == nil {
		t.Fatal("A 未能从磁盘恢复")
	}
	// 重启一个新的缓存实例再加 C：A 的访问时间必须通过磁盘 mtime 持久化，
	// 只靠旧进程内存索引不算数。总容量超限，最久未访问的 B 应被淘汰。
	cc := srv.URL + "/lru-c.png"
	cRestart := NewWithDisk(nil, cfg)
	cRestart.Client = srv.Client()
	cRestart.Bitmap(cc, WidthCard)
	waitIdle(t, cRestart)
	if info := cRestart.DiskInfo(); info.Entries != 2 {
		t.Fatalf("容量淘汰后应仍为 2 条: %+v", info)
	}

	c2 := NewWithDisk(nil, cfg)
	c2.Bitmap(a, WidthCard)
	waitIdle(t, c2)
	if c2.Get(a, WidthCard) == nil {
		t.Fatal("最近访问的 A 被错误淘汰")
	}
	if st := c2.Stats(); st.Fetches != 0 || st.DiskHits == 0 {
		t.Fatalf("A 应命中磁盘: %+v", st)
	}
	beforeB := hits("/lru-b.png")
	c2.Bitmap(b, WidthCard)
	waitIdle(t, c2)
	if got := hits("/lru-b.png"); got != beforeB+1 {
		t.Fatalf("最久未访问的 B 未被淘汰（B 命中 %d 次，want %d）", got, beforeB+1)
	}
}

// TestDiskCleanupFailureKeepsAccountingAndStopsWrites 钉住两件事：
//   - 删除失败的文件仍留在索引/字节账目里，不能假装已释放；
//   - 容量清理删不掉旧文件时，不再继续写入新文件，避免实际磁盘越堆越高。
func TestDiskCleanupFailureKeepsAccountingAndStopsWrites(t *testing.T) {
	dir := t.TempDir()
	d := &diskStore{
		dir:        dir,
		maxBytes:   1 << 30,
		maxEntries: 1,
		ttl:        DefaultDiskTTL,
		now:        time.Now,
		index:      make(map[string]diskMeta),
	}
	if ok, err := d.put(diskKeyURL("old"), coverPNG()); !ok || err != nil {
		t.Fatalf("写入旧条目失败: ok=%v err=%v", ok, err)
	}
	oldBytes := d.bytes

	// 模拟目录不可删（权限/配额等）：保留旧文件与账目，新条目回滚，随后暂停写入。
	d.removeFile = func(string) error { return os.ErrPermission }
	if ok, err := d.put(diskKeyURL("new"), coverPNG()); ok || err == nil {
		t.Fatalf("容量清理失败时不应报告写入成功: ok=%v err=%v", ok, err)
	}
	d.mu.Lock()
	entries, bytes, blocked := len(d.index), d.bytes, d.cleanupFailed
	d.mu.Unlock()
	if entries != 2 || bytes <= oldBytes {
		t.Fatalf("删除失败的文件被错误地从账目里扣掉了：entries=%d bytes=%d oldBytes=%d", entries, bytes, oldBytes)
	}
	if !blocked {
		t.Fatal("清理失败后没有暂停后续写入")
	}
	files, err := filepath.Glob(filepath.Join(dir, "*.img"))
	if err != nil {
		t.Fatal(err)
	}
	if len(files) != 2 {
		t.Fatalf("实际文件数 = %d, want 2（失败删除不能凭空消失）", len(files))
	}

	// 第三次写入必须被直接拒绝，不创建临时文件、也不新增正式文件。
	if ok, err := d.put(diskKeyURL("third"), coverPNG()); ok || err == nil {
		t.Fatalf("清理失败后仍继续写入: ok=%v err=%v", ok, err)
	}
	files, _ = filepath.Glob(filepath.Join(dir, "*.img"))
	if len(files) != 2 {
		t.Fatalf("暂停写入后文件数变成 %d，want 2", len(files))
	}

	// 恢复删除能力并跑维护：旧文件被清掉、写入恢复。
	d.removeFile = nil
	if err := d.maintain(); err != nil {
		t.Fatal(err)
	}
	d.mu.Lock()
	entries, blocked = len(d.index), d.cleanupFailed
	d.mu.Unlock()
	if entries != 1 || blocked {
		t.Fatalf("维护后应回到 1 条且恢复写入：entries=%d blocked=%v", entries, blocked)
	}
}

// TestDiskMaintenanceAccountsUndeletableCorruptFile 钉住坏文件删不掉时仍按
// 实际文件大小留在索引/字节账目里，并因超预算暂停后续写入。改前这里忽略
// os.Remove 错误、随后又通过 unseen 丢掉元数据，坏文件变成账外占用。
func TestDiskMaintenanceAccountsUndeletableCorruptFile(t *testing.T) {
	dir := t.TempDir()
	d := &diskStore{
		dir:        dir,
		maxBytes:   32,
		maxEntries: 10,
		ttl:        DefaultDiskTTL,
		now:        time.Now,
		index:      make(map[string]diskMeta),
	}
	hash := diskKeyURL("corrupt-undeletable")
	path := d.path(hash)
	const corruptSize = 40
	if err := os.WriteFile(path, bytes.Repeat([]byte{0xAA}, corruptSize), 0o600); err != nil {
		t.Fatal(err)
	}
	d.removeFile = func(string) error { return os.ErrPermission }

	if err := d.maintain(); err != nil {
		t.Fatal(err)
	}
	d.mu.Lock()
	meta, tracked := d.index[hash]
	entries, accounted, blocked := len(d.index), d.bytes, d.cleanupFailed
	d.mu.Unlock()
	if !tracked || meta.size != corruptSize {
		t.Fatalf("删不掉的坏文件没有按实际大小记账：tracked=%v meta=%+v", tracked, meta)
	}
	if entries != 1 || accounted != corruptSize {
		t.Fatalf("维护后账目 = %d 条 / %d 字节, want 1 条 / %d 字节", entries, accounted, corruptSize)
	}
	if !blocked {
		t.Fatal("坏文件让磁盘超预算却未暂停写入")
	}

	// 暂停写入：不新增文件（连临时文件也不留）。
	if ok, err := d.put(diskKeyURL("new"), []byte{1}); ok || err == nil {
		t.Fatalf("超预算且清理失败时仍写入：ok=%v err=%v", ok, err)
	}
	files, err := filepath.Glob(filepath.Join(dir, "*.img"))
	if err != nil {
		t.Fatal(err)
	}
	if len(files) != 1 {
		t.Fatalf("实际文件数 = %d, want 1（坏文件仍在，且没有新写入）", len(files))
	}

	// 删除能力恢复后，维护清掉坏文件、账目归零并恢复写入。
	d.removeFile = nil
	if err := d.maintain(); err != nil {
		t.Fatal(err)
	}
	d.mu.Lock()
	entries, accounted, blocked = len(d.index), d.bytes, d.cleanupFailed
	d.mu.Unlock()
	if entries != 0 || accounted != 0 || blocked {
		t.Fatalf("恢复删除后未清理干净：entries=%d bytes=%d blocked=%v", entries, accounted, blocked)
	}
}

// TestDiskMaintenancePreservesNewerAccessTime 钉住维护扫描不会把内存里
// 刚记录的最新访问时间重置回文件系统上的旧 mtime（否则 LRU 会误删刚用过
// 的条目）。
func TestDiskMaintenancePreservesNewerAccessTime(t *testing.T) {
	dir := t.TempDir()
	base := time.Date(2026, 2, 3, 4, 5, 6, 0, time.UTC)
	now := base
	d := &diskStore{
		dir:        dir,
		maxBytes:   1 << 30,
		maxEntries: 10,
		ttl:        DefaultDiskTTL,
		now:        func() time.Time { return now },
		index:      make(map[string]diskMeta),
	}
	hash := diskKeyURL("recent")
	if ok, err := d.put(hash, coverPNG()); !ok || err != nil {
		t.Fatalf("写入失败: ok=%v err=%v", ok, err)
	}

	// 之后访问一次，索引 used = base+2h；再手动把文件 mtime 改回旧值，
	// 模拟 Chtimes 失败/文件系统时间精度导致磁盘上的时间比内存旧。
	now = base.Add(2 * time.Hour)
	if _, ok := d.get(hash); !ok {
		t.Fatal("磁盘读取失败")
	}
	if err := os.Chtimes(d.path(hash), base, base); err != nil {
		t.Fatal(err)
	}
	now = base.Add(3 * time.Hour)
	if err := d.maintain(); err != nil {
		t.Fatal(err)
	}
	d.mu.Lock()
	used := d.index[hash].used
	d.mu.Unlock()
	if !used.Equal(base.Add(2 * time.Hour)) {
		t.Fatalf("维护把最新访问时间重置了：used=%v, want %v", used, base.Add(2*time.Hour))
	}
}

// TestDiskEntryCountCap 钉住除了字节上限，磁盘还有条目数上限：大量极小
// 图片不能把索引/启动扫描撑到几十万条。
func TestDiskEntryCountCap(t *testing.T) {
	dir := t.TempDir()
	d := &diskStore{
		dir:        dir,
		maxBytes:   1 << 30,
		maxEntries: 3,
		ttl:        DefaultDiskTTL,
		now:        time.Now,
		index:      make(map[string]diskMeta),
	}
	for i := 0; i < 6; i++ {
		if ok, err := d.put(diskKeyURL("tiny-"+itoa(i)), []byte{byte(i)}); !ok || err != nil {
			t.Fatalf("写入第 %d 条失败: ok=%v err=%v", i, ok, err)
		}
	}
	d.mu.Lock()
	entries, bytes := len(d.index), d.bytes
	d.mu.Unlock()
	if entries != 3 {
		t.Fatalf("条目数 = %d, want 3（条目上限失效）", entries)
	}
	var sum int64
	for _, m := range d.index {
		sum += m.size
	}
	if bytes != sum {
		t.Fatalf("字节账目 = %d, want %d", bytes, sum)
	}
	files, _ := filepath.Glob(filepath.Join(dir, "*.img"))
	if len(files) != 3 {
		t.Fatalf("实际文件数 = %d, want 3", len(files))
	}
}

// TestDiskCorruptAndOversizeFallBackToNetwork 钉住坏文件不会让封面永久空白：
// 头损坏 / 文件超过单张上限时删掉该条并回网络，且修好后自动重新落盘。
func TestDiskCorruptAndOversizeFallBackToNetwork(t *testing.T) {
	dir := t.TempDir()
	srv, hits := countServer(t)
	c := NewWithDisk(nil, DiskConfig{Dir: dir})
	c.Client = srv.Client()
	u := srv.URL + "/corrupt.png"

	c.Bitmap(u, WidthCard)
	waitIdle(t, c)
	if c.Get(u, WidthCard) == nil {
		t.Fatal("首次抓取未就绪")
	}
	path := filepath.Join(dir, diskKeyURL(ThumbURL(u, WidthCard))+".img")

	// 1) 文件头损坏。
	if err := os.WriteFile(path, []byte("garbage"), 0o600); err != nil {
		t.Fatal(err)
	}
	c.Purge()
	c.Bitmap(u, WidthCard)
	waitIdle(t, c)
	if c.Get(u, WidthCard) == nil {
		t.Fatalf("损坏缓存没有回退网络: %+v", c.Stats())
	}
	if got := hits("/corrupt.png"); got != 2 {
		t.Fatalf("损坏缓存应重发 HTTP（命中 %d 次，want 2）", got)
	}

	// 2) 文件超过 4MiB 上限（不解码、不读进内存）。
	if err := os.Truncate(path, maxDiskEntryBytes()+1); err != nil {
		t.Fatal(err)
	}
	c.Purge()
	c.Bitmap(u, WidthCard)
	waitIdle(t, c)
	if c.Get(u, WidthCard) == nil {
		t.Fatalf("超限缓存没有回退网络: %+v", c.Stats())
	}
	if got := hits("/corrupt.png"); got != 3 {
		t.Fatalf("超限缓存应重发 HTTP（命中 %d 次，want 3）", got)
	}
	if info := c.DiskInfo(); info.Entries != 1 {
		t.Fatalf("回退成功后应重新落盘一条: %+v", info)
	}
}

// TestSustainedTurnoverBoundedAndOfflineReuse 用本地合成服务器跑一段持续
// 换图：120 张互不相同的 480×270 PNG，每批 15 张、顺序换 8 批，磁盘预算
// 只给 8KiB。它验证的是缓存自己的账目边界——RAM 位图 ≤12MiB / 64 条、
// 实际磁盘文件 ≤ 配置字节数和条目上限——以及 Purge 清空 RAM、关掉服务器
// 后仍能从磁盘复用。这里不量 OS RSS，也不代表整进程内存。
func TestSustainedTurnoverBoundedAndOfflineReuse(t *testing.T) {
	const (
		total     = 120
		batchSize = 15
	)
	images := make([][]byte, total)
	for i := range images {
		im := image.NewRGBA(image.Rect(0, 0, 480, 270))
		r, g, b := byte(i*17), byte(i*31), byte(i*47)
		for p := 0; p < len(im.Pix); p += 4 {
			im.Pix[p], im.Pix[p+1], im.Pix[p+2], im.Pix[p+3] = r, g, b, 255
		}
		var buf bytes.Buffer
		if err := png.Encode(&buf, im); err != nil {
			t.Fatal(err)
		}
		images[i] = buf.Bytes()
	}
	var hits atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hits.Add(1)
		var i int
		if _, err := fmt.Sscanf(r.URL.Path, "/turnover-%d.png", &i); err != nil || i < 0 || i >= len(images) {
			http.NotFound(w, r)
			return
		}
		w.Header().Set("Content-Type", "image/png")
		_, _ = w.Write(images[i])
	}))
	t.Cleanup(srv.Close)

	dir := t.TempDir()
	cfg := DiskConfig{Dir: dir, MaxBytes: 8 << 10}
	c := NewWithDisk(nil, cfg)
	c.Client = srv.Client()
	urls := make([]string, total)
	for i := range urls {
		urls[i] = fmt.Sprintf("%s/turnover-%d.png", srv.URL, i)
	}

	peakRAM, peakLen := int64(0), 0
	for start := 0; start < total; start += batchSize {
		for i := start; i < start+batchSize; i++ {
			c.Bitmap(urls[i], WidthPlayer)
		}
		waitIdle(t, c)

		if b := c.Bytes(); b > peakRAM {
			peakRAM = b
		}
		if n := c.Len(); n > peakLen {
			peakLen = n
		}
		if peakRAM > cacheByteLimit {
			t.Fatalf("第 %d 批后 RAM 位图 %d 字节，超过预算 %d", start/batchSize+1, peakRAM, cacheByteLimit)
		}
		if peakLen > c.Limit() {
			t.Fatalf("第 %d 批后 RAM 条目 %d，超过上限 %d", start/batchSize+1, peakLen, c.Limit())
		}
		info := c.DiskInfo()
		if info.Bytes > cfg.MaxBytes {
			t.Fatalf("第 %d 批后磁盘账目 %d 字节，超过配置 %d", start/batchSize+1, info.Bytes, cfg.MaxBytes)
		}
		if info.Entries > diskMaxEntries {
			t.Fatalf("第 %d 批后磁盘条目 %d，超过上限 %d", start/batchSize+1, info.Entries, diskMaxEntries)
		}
	}

	// 直接数实际文件，确认磁盘真的按配置字节/条目封顶，而不只是索引账目。
	var diskBytes int64
	diskCount := 0
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	for _, de := range entries {
		if !strings.HasSuffix(de.Name(), ".img") {
			continue
		}
		fi, err := de.Info()
		if err != nil {
			t.Fatal(err)
		}
		diskBytes += fi.Size()
		diskCount++
	}
	if diskBytes > cfg.MaxBytes {
		t.Fatalf("实际磁盘 %d 字节，超过配置 %d", diskBytes, cfg.MaxBytes)
	}
	if diskCount > diskMaxEntries {
		t.Fatalf("实际磁盘 %d 条，超过上限 %d", diskCount, diskMaxEntries)
	}
	if diskCount == 0 || diskCount >= total {
		t.Fatalf("持续换图后磁盘条目 = %d（应已有淘汰但不为空，总数 %d）", diskCount, total)
	}

	// 从仍留在磁盘里的最新条目取一张，Purge 清 RAM 后关服务器：新实例必须
	// 只靠磁盘恢复，不能再发 HTTP。
	reuse := ""
	d := c.diskRef()
	d.mu.Lock()
	for i := total - 1; i >= 0; i-- {
		if _, ok := d.index[diskKeyURL(ThumbURL(urls[i], WidthPlayer))]; ok {
			reuse = urls[i]
			break
		}
	}
	d.mu.Unlock()
	if reuse == "" {
		t.Fatal("磁盘里没有可复用的最新条目")
	}
	c.Purge()
	if c.Get(reuse, WidthPlayer) != nil {
		t.Fatal("Purge 后 RAM 仍命中")
	}
	srv.Close()
	beforeHits := hits.Load()

	c2 := NewWithDisk(nil, cfg)
	c2.Client = srv.Client()
	c2.Bitmap(reuse, WidthPlayer)
	waitIdle(t, c2)
	bmp := c2.Get(reuse, WidthPlayer)
	if bmp == nil {
		t.Fatalf("离线新实例未从磁盘恢复: %+v", c2.Stats())
	}
	if w, h := bmp.Size(); w != 480 || h != 270 {
		t.Fatalf("离线恢复尺寸 = %dx%d, want 480x270", w, h)
	}
	if st := c2.Stats(); st.Fetches != 0 || st.DiskHits == 0 {
		t.Fatalf("离线复用应只读磁盘、不发 HTTP：%+v", st)
	}
	if got := hits.Load(); got != beforeHits {
		t.Fatalf("离线复用仍发了 HTTP：%d -> %d", beforeHits, got)
	}
}

// TestFailedOrInvalidResponsesNotPersisted 钉住失败 HTTP / 非图片响应不落盘。
func TestFailedOrInvalidResponsesNotPersisted(t *testing.T) {
	dir := t.TempDir()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.HasSuffix(r.URL.Path, "/bad-status.png") {
			w.WriteHeader(http.StatusBadGateway)
			return
		}
		w.Header().Set("Content-Type", "image/png")
		_, _ = w.Write([]byte("this is definitely not an image"))
	}))
	t.Cleanup(srv.Close)

	c := NewWithDisk(nil, DiskConfig{Dir: dir})
	c.Client = srv.Client()
	c.Bitmap(srv.URL+"/bad-status.png", WidthCard)
	waitIdle(t, c)
	c.Bitmap(srv.URL+"/bad-image.png", WidthCard)
	waitIdle(t, c)
	if st := c.Stats(); st.Failed != 2 {
		t.Fatalf("两次失败应记 Failed=2: %+v", st)
	}
	if info := c.DiskInfo(); info.Entries != 0 || info.Bytes != 0 {
		t.Fatalf("失败响应被写进磁盘: %+v", info)
	}
}

// TestDiskUnavailableFallsBackToNetwork 钉住目录不可用（这里是同名普通文件）
// 只降级、不影响抓图。
func TestDiskUnavailableFallsBackToNetwork(t *testing.T) {
	blocked := filepath.Join(t.TempDir(), "not-a-dir")
	if err := os.WriteFile(blocked, []byte("x"), 0o600); err != nil {
		t.Fatal(err)
	}
	c := NewWithDisk(nil, DiskConfig{Dir: blocked})
	if info := c.DiskInfo(); info.Enabled || info.Err == nil {
		t.Fatalf("不可用目录应降级并记录错误: %+v", info)
	}
	srv, hits := countServer(t)
	c.Client = srv.Client()
	u := srv.URL + "/fallback.png"
	c.Bitmap(u, WidthCard)
	waitIdle(t, c)
	if c.Get(u, WidthCard) == nil {
		t.Fatal("磁盘不可用时没有回退网络")
	}
	if got := hits("/fallback.png"); got != 1 {
		t.Fatalf("应只发一次网络请求，实际 %d", got)
	}
}

// TestCancelledFetchNotPersisted 钉住取消的下载不写磁盘（也不算失败）。
func TestCancelledFetchNotPersisted(t *testing.T) {
	started := make(chan struct{}, 1)
	canceled := make(chan struct{}, 1)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		select {
		case started <- struct{}{}:
		default:
		}
		<-r.Context().Done()
		select {
		case canceled <- struct{}{}:
		default:
		}
	}))
	t.Cleanup(srv.Close)

	c := NewWithDisk(nil, DiskConfig{Dir: t.TempDir()})
	c.Client = srv.Client()
	u := srv.URL + "/slow.png"
	c.Bitmap(u, WidthCard)
	select {
	case <-started:
	case <-time.After(3 * time.Second):
		t.Fatal("请求一直没到服务器")
	}
	c.Cancel(u, WidthCard)
	select {
	case <-canceled:
	case <-time.After(3 * time.Second):
		t.Fatal("Cancel 没有 abort 请求")
	}
	waitIdle(t, c)
	if st := c.State(u, WidthCard); st != StateAbsent {
		t.Fatalf("取消后状态=%v, want Absent", st)
	}
	if info := c.DiskInfo(); info.Entries != 0 {
		t.Fatalf("取消的下载被写进磁盘: %+v", info)
	}
	if st := c.Stats(); st.Failed != 0 {
		t.Fatalf("取消被记成失败: %+v", st)
	}
}

// TestAmbientAccountsBytesAndEvictsImmediately 钉住派生氛围光条目也计入
// RAM 字节预算，并在插入时就地执行淘汰（旧实现 e.bytes=0，不计入预算）。
func TestAmbientAccountsBytesAndEvictsImmediately(t *testing.T) {
	c := New(nil)
	c.limit = 1
	u := "ambient-budget"
	srcKey := key{u, WidthAmbient}
	srcBmp := ui.NewBitmap(image.NewRGBA(image.Rect(0, 0, 320, 180)))
	srcSmall := image.NewRGBA(image.Rect(0, 0, 64, 64))
	srcEntry := &entry{bitmap: srcBmp, small: srcSmall}
	srcEntry.bytes = bitmapBytes(srcBmp, srcSmall)
	c.items[srcKey] = srcEntry
	c.order = append(c.order, srcKey)
	c.bytes += srcEntry.bytes

	bmp := c.Ambient(u, WidthAmbient, false)
	if bmp == nil {
		t.Fatal("源图就绪时 Ambient 应返回派生位图")
	}
	k := key{u + "\x00ambient", WidthAmbient}
	c.mu.Lock()
	defer c.mu.Unlock()
	e := c.items[k]
	if e == nil || e.bitmap != bmp {
		t.Fatal("派生条目未插入")
	}
	if e.bytes <= 0 {
		t.Fatal("派生条目的字节数没有记账")
	}
	if c.bytes != e.bytes {
		t.Fatalf("插入后字节账目 = %d, want %d（源图应被条目上限淘汰）", c.bytes, e.bytes)
	}
	if _, ok := c.items[srcKey]; ok {
		t.Fatal("limit=1 时插入派生图应立即淘汰最久未用的源图")
	}
	if len(c.items) != 1 {
		t.Fatalf("淘汰后条目数 = %d, want 1", len(c.items))
	}
}

// TestAmbientDoesNotRepopulateAfterPurge 钉住「派生在锁外算完时若已经 Purge，
// 不得把旧结果塞回缓存」。直接调用插入阶段 storeAmbient，把 Purge 确定性地
// 放在「派生已算完、尚未插入」之间。
func TestAmbientDoesNotRepopulateAfterPurge(t *testing.T) {
	c := New(nil)
	u := "ambient-purge"
	srcKey := key{u, WidthAmbient}
	srcBmp := ui.NewBitmap(image.NewRGBA(image.Rect(0, 0, 64, 64)))
	srcSmall := image.NewRGBA(image.Rect(0, 0, 64, 64))
	srcEntry := &entry{bitmap: srcBmp, small: srcSmall}
	srcEntry.bytes = bitmapBytes(srcBmp, srcSmall)
	c.mu.Lock()
	c.items[srcKey] = srcEntry
	c.order = append(c.order, srcKey)
	c.bytes = srcEntry.bytes
	gen := c.gen
	c.mu.Unlock()

	// 模拟浏览器/派生已算完，此时窗口隐藏触发 Purge。
	c.Purge()
	bmp := ui.NewBitmap(image.NewRGBA(image.Rect(0, 0, 64, 64)))
	k := key{u + "\x00ambient", WidthAmbient}
	if got := c.storeAmbient(srcKey, k, gen, srcEntry, srcSmall, srcBmp, bmp); got != nil {
		t.Fatal("Purge 之后仍返回/插入了旧派生图")
	}
	if c.Len() != 0 || c.Bytes() != 0 || c.Pending() != 0 {
		t.Fatalf("Purge 后派生图又把缓存填回来了：len=%d bytes=%d pending=%d",
			c.Len(), c.Bytes(), c.Pending())
	}
}

// TestDecodePreflightLimits 钉住 image.Decode 之前的两道闸：字节上限与
// DecodeConfig 的像素/单边上限。
func TestDecodePreflightLimits(t *testing.T) {
	if _, err := decodeWebP(make([]byte, maxDownloadBytes+1)); err == nil {
		t.Fatal("超过 4MiB 的数据应在解码前被拒绝")
	}

	mkPNG := func(w, h int) []byte {
		im := image.NewRGBA(image.Rect(0, 0, w, h))
		var b bytes.Buffer
		if err := png.Encode(&b, im); err != nil {
			t.Fatal(err)
		}
		return b.Bytes()
	}
	// 2049×2049 = 4,198,401 > 4Mi 像素。
	tooManyPixels := mkPNG(2049, 2049)
	if cfg, _, err := image.DecodeConfig(bytes.NewReader(tooManyPixels)); err != nil || cfg.Width != 2049 {
		t.Fatalf("测试前提不成立: cfg=%+v err=%v", cfg, err)
	}
	if _, err := decodeWebP(tooManyPixels); err == nil {
		t.Fatal("超过像素上限的图应在 image.Decode 前被拒绝")
	}
	// 单边 8193 > 8192。
	tooWide := mkPNG(8193, 1)
	if _, err := decodeWebP(tooWide); err == nil {
		t.Fatal("超过单边上限的图应被拒绝")
	}
	// 正常小图不受影响。
	if _, err := decodeWebP(coverPNG()); err != nil {
		t.Fatalf("正常缩略图被误拒: %v", err)
	}
}

// TestHTTPGetByteLimitUsesPlusOne 钉住读取用 max+1：刚好超过 4MiB 也必须
// 被识别为超限，而不是静默截断后拿去解码。
func TestHTTPGetByteLimitUsesPlusOne(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "image/png")
		_, _ = w.Write(bytes.Repeat([]byte{0xAA}, maxDownloadBytes+1))
	}))
	t.Cleanup(srv.Close)
	c := New(nil)
	if _, err := c.httpGet(context.Background(), srv.URL); err == nil {
		t.Fatal("超过 4MiB 的响应没有被拒绝")
	}
}

// TestDefaultDiskConfigTarget 钉住生产目录形状（不写文件）。
func TestDefaultDiskConfigTarget(t *testing.T) {
	cfg, err := DefaultDiskConfig()
	if err != nil {
		t.Skipf("环境没有用户缓存目录: %v", err)
	}
	wantSuffix := filepath.Join("bili-fm", "images-v1")
	if !strings.HasSuffix(cfg.Dir, wantSuffix) {
		t.Fatalf("默认磁盘目录 = %q, want 后缀 %q", cfg.Dir, wantSuffix)
	}
	if cfg.MaxBytes != DefaultDiskMaxBytes || cfg.TTL != DefaultDiskTTL {
		t.Fatalf("默认容量/TTL 不对: %+v", cfg)
	}
}

// TestAmbientDoesNotAttachToReplacedSource 钉住派生期间源图被换掉时同样放弃
// 插入，避免把旧图的调色结果挂到新源图旁边。
func TestAmbientDoesNotAttachToReplacedSource(t *testing.T) {
	c := New(nil)
	u := "ambient-replace"
	srcKey := key{u, WidthAmbient}
	srcBmp := ui.NewBitmap(image.NewRGBA(image.Rect(0, 0, 64, 64)))
	srcSmall := image.NewRGBA(image.Rect(0, 0, 64, 64))
	srcEntry := &entry{bitmap: srcBmp, small: srcSmall}
	srcEntry.bytes = bitmapBytes(srcBmp, srcSmall)
	c.mu.Lock()
	c.items[srcKey] = srcEntry
	c.order = append(c.order, srcKey)
	c.bytes = srcEntry.bytes
	gen := c.gen
	c.mu.Unlock()

	// 派生算完后、插入前，源图被重新抓取替换。
	newBmp := ui.NewBitmap(image.NewRGBA(image.Rect(0, 0, 64, 64)))
	newSmall := image.NewRGBA(image.Rect(0, 0, 64, 64))
	c.mu.Lock()
	srcEntry.bitmap, srcEntry.small = newBmp, newSmall
	srcEntry.bytes = bitmapBytes(newBmp, newSmall)
	c.mu.Unlock()

	bmp := ui.NewBitmap(image.NewRGBA(image.Rect(0, 0, 64, 64)))
	k := key{u + "\x00ambient", WidthAmbient}
	if got := c.storeAmbient(srcKey, k, gen, srcEntry, srcSmall, srcBmp, bmp); got != nil {
		t.Fatal("源图已替换，旧派生结果仍被返回")
	}
	c.mu.Lock()
	_, hasAmbient := c.items[k]
	c.mu.Unlock()
	if hasAmbient {
		t.Fatal("源图替换后仍插入了旧派生条目")
	}
}
