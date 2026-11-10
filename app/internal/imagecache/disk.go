package imagecache

import (
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"net/url"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"
)

// 磁盘缓存只保存**下载到的压缩字节**（不保存解码后的位图），属于 RAM LRU
// 之外的第二层。默认 128MiB、7 天 TTL；容量按最近使用淘汰，TTL 从落盘时刻
// 算起（读命中只更新访问时间，不续期）。
const (
	// DefaultDiskMaxBytes 是压缩图磁盘缓存的默认上限。
	DefaultDiskMaxBytes int64 = 128 << 20
	// DefaultDiskTTL 是磁盘条目的默认保鲜期（自下载落盘起算）。
	DefaultDiskTTL = 7 * 24 * time.Hour

	diskDirName             = "images-v1"
	diskMagic               = "BFIMGC1\n"
	diskHeaderSize          = len(diskMagic) + 8
	diskMaintenanceInterval = 10 * time.Minute
	diskTempMaxAge          = time.Hour
	// diskMaxEntries 是磁盘文件数上限。缩略图可能只有几 KB，只靠 128MiB
	// 字节上限会让索引和启动扫描膨胀到几十万个文件；8192 条足以覆盖常见
	// 浏览/播放缓存，同时让启动扫描保持在可接受的范围内。
	diskMaxEntries = 8192
)

// DiskConfig 配置生产环境的压缩图磁盘缓存。零值字段使用默认值；Dir 为空
// 表示不启用。Now 只给测试注入时钟用，生产留 nil（time.Now）。
//
// 磁盘不可用（UserCacheDir 失败、目录不可创建/不可写、文件损坏等）时缓存
// 会自动降级为纯网络：抓图、RAM LRU、Purge 等行为都不变，只是没有磁盘层。
// 错误可用 (*Cache).DiskInfo().Err 查看。
type DiskConfig struct {
	Dir      string
	MaxBytes int64
	TTL      time.Duration
	Now      func() time.Time
}

// DefaultDiskConfig 返回生产默认配置：os.UserCacheDir()/bili-fm/images-v1。
func DefaultDiskConfig() (DiskConfig, error) {
	base, err := os.UserCacheDir()
	if err != nil {
		return DiskConfig{}, fmt.Errorf("定位用户缓存目录: %w", err)
	}
	return DiskConfig{
		Dir:      filepath.Join(base, "bili-fm", diskDirName),
		MaxBytes: DefaultDiskMaxBytes,
		TTL:      DefaultDiskTTL,
	}, nil
}

func (cfg DiskConfig) withDefaults() DiskConfig {
	if cfg.MaxBytes <= 0 {
		cfg.MaxBytes = DefaultDiskMaxBytes
	}
	if cfg.TTL <= 0 {
		cfg.TTL = DefaultDiskTTL
	}
	if cfg.Now == nil {
		cfg.Now = time.Now
	}
	return cfg
}

// NewWithDisk 建一个可选的带磁盘层的缓存。磁盘目录不可用时仍返回可用的
// 纯内存/网络缓存（不会让应用启动失败），失败原因见 DiskInfo。
func NewWithDisk(notify func(), cfg DiskConfig) *Cache {
	c := New(notify)
	if err := c.EnableDisk(cfg); err != nil {
		c.setDiskErr(err)
	}
	return c
}

// NewPersistent 是生产入口：使用 os.UserCacheDir()/bili-fm/images-v1，
// 默认 128MiB / 7 天。它会在构造时**同步**扫描并清理磁盘目录（只读文件头，
// 不 decode 图片），所以不是「永不阻塞」：调用方应把它放在 UI 窗口创建之前
// 的启动阶段。Directory 不可用时自动退回纯网络。
//
// 只有生产 main 的装配显式使用它；view.NewApp 保持 imagecache.New，
// 测试 / 无头环境不碰用户缓存目录。不要把 NewApp 全局换成 NewPersistent。
func NewPersistent(notify func()) *Cache {
	c := New(notify)
	cfg, err := DefaultDiskConfig()
	if err != nil {
		c.setDiskErr(err)
		return c
	}
	if err := c.EnableDisk(cfg); err != nil {
		c.setDiskErr(err)
	}
	return c
}

// EnableDisk 显式开启磁盘层。它同步创建目录、探写并扫描现有文件（只读
// 文件头，不 decode），因此应在首次使用前、UI 窗口创建之前调用；重复调用
// 会替换现有磁盘配置（旧条目仍留在磁盘，新索引会重新扫描）。返回非 nil
// 时磁盘层未启用，但缓存本身仍可正常抓图。
func (c *Cache) EnableDisk(cfg DiskConfig) error {
	if cfg.Dir == "" {
		err := errors.New("imagecache: 磁盘缓存目录为空")
		c.setDiskErr(err)
		return err
	}
	cfg = cfg.withDefaults()
	if err := os.MkdirAll(cfg.Dir, 0o755); err != nil {
		err = fmt.Errorf("创建图片磁盘缓存目录 %s: %w", cfg.Dir, err)
		c.setDiskErr(err)
		return err
	}
	// 启动时探一次可写性：目录存在但只读（或配额/权限问题）时直接降级，
	// 不要等到每张图抓完才在落盘阶段失败。
	probe, err := os.CreateTemp(cfg.Dir, ".tmp-probe-")
	if err != nil {
		err = fmt.Errorf("图片磁盘缓存目录不可写 %s: %w", cfg.Dir, err)
		c.setDiskErr(err)
		return err
	}
	probeName := probe.Name()
	_ = probe.Close()
	_ = os.Remove(probeName)
	d := &diskStore{
		dir:        cfg.Dir,
		maxBytes:   cfg.MaxBytes,
		maxEntries: diskMaxEntries,
		ttl:        cfg.TTL,
		now:        cfg.Now,
		index:      make(map[string]diskMeta),
	}
	// 启动时只扫一次目录建元数据索引（不 decode、不常驻图片字节），顺便
	// 清掉过期条目与陈旧临时文件。之后命中走索引，不再扫目录。
	if err := d.maintain(); err != nil {
		err = fmt.Errorf("扫描图片磁盘缓存 %s: %w", cfg.Dir, err)
		c.setDiskErr(err)
		return err
	}
	c.mu.Lock()
	c.disk = d
	c.diskErr = nil
	c.mu.Unlock()
	return nil
}

// DiskInfo 返回磁盘层的只读诊断信息。Err 为最近一次配置/写入失败的描述；
// 磁盘正常时为 nil。
type DiskInfo struct {
	Enabled  bool
	Entries  int
	Bytes    int64
	MaxBytes int64
	Err      error
}

// DiskInfo 返回磁盘缓存的诊断快照。不扫描目录、不碰图片字节。
func (c *Cache) DiskInfo() DiskInfo {
	c.mu.Lock()
	d, err := c.disk, c.diskErr
	c.mu.Unlock()
	info := DiskInfo{Enabled: d != nil, Err: err}
	if d == nil {
		return info
	}
	d.mu.Lock()
	info.Entries = len(d.index)
	info.Bytes = d.bytes
	info.MaxBytes = d.maxBytes
	d.mu.Unlock()
	return info
}

// RunDiskMaintenance 同步跑一次磁盘清理（过期条目、陈旧临时文件、容量）。
// 生产由后台机会式维护自动触发；测试可用它确定性地推进清理。
func (c *Cache) RunDiskMaintenance() error {
	d := c.diskRef()
	if d == nil {
		return nil
	}
	return d.maintain()
}

func (c *Cache) setDiskErr(err error) {
	if err == nil {
		return
	}
	c.mu.Lock()
	c.diskErr = err
	c.mu.Unlock()
}

func (c *Cache) diskRef() *diskStore {
	c.mu.Lock()
	d := c.disk
	c.mu.Unlock()
	return d
}

func (c *Cache) diskLoad(src string) ([]byte, bool) {
	d := c.diskRef()
	if d == nil {
		return nil, false
	}
	data, ok := d.get(diskKeyURL(src))
	c.mu.Lock()
	if ok {
		c.stats.DiskHits++
	} else {
		c.stats.DiskMisses++
	}
	c.mu.Unlock()
	d.maybeMaintain()
	return data, ok
}

// diskPutURL 返回是否真的写入了一条新文件。写失败/磁盘未启用/单条超过
// 整个容量时返回 false；错误记录到 DiskInfo().Err，但不影响本次抓图。
func (c *Cache) diskPutURL(src string, data []byte) bool {
	d := c.diskRef()
	if d == nil {
		return false
	}
	wrote, err := d.put(diskKeyURL(src), data)
	if err != nil {
		c.setDiskErr(err)
	} else if wrote {
		// 上一次写失败可能只是瞬时问题；成功落盘后清掉旧错误。
		c.mu.Lock()
		c.diskErr = nil
		c.mu.Unlock()
	}
	return wrote
}

func (c *Cache) diskForget(src string) {
	if d := c.diskRef(); d != nil {
		d.remove(diskKeyURL(src))
	}
}

// diskKeyURL 把实际请求 URL（ThumbURL / 转码地址）规范化后取 SHA-256。
// 宽度变体本来就在 URL 里（@240w.webp / @480w.webp），因此自然成为 key 的
// 一部分。文件名只有哈希，绝不出现原始 URL。
func diskKeyURL(raw string) string {
	sum := sha256.Sum256([]byte(normalizeDiskURL(raw)))
	return hex.EncodeToString(sum[:])
}

func normalizeDiskURL(raw string) string {
	u, err := url.Parse(raw)
	if err != nil {
		return raw
	}
	u.Fragment = ""
	u.Scheme = strings.ToLower(u.Scheme)
	u.Host = strings.ToLower(u.Host)
	return u.String()
}

type diskMeta struct {
	size   int64
	stored time.Time
	used   time.Time
}

type diskStore struct {
	dir        string
	maxBytes   int64
	maxEntries int
	ttl        time.Duration
	now        func() time.Time

	mu           sync.Mutex
	index        map[string]diskMeta
	bytes        int64
	lastMaint    time.Time
	maintRunning bool
	// cleanupFailed 表示上一次容量清理删不掉足够多的旧文件。此后暂停写入，
	// 避免「每张新图都写进去、旧图删不掉」把磁盘真正撑爆；维护成功清理
	// 后自动恢复。
	cleanupFailed bool
	// removeFile 默认 os.Remove；测试可替换它确定性地模拟删除失败。
	removeFile func(string) error
}

var errBadDiskEntry = errors.New("bad imagecache disk entry")

func (d *diskStore) path(hash string) string {
	return filepath.Join(d.dir, hash+".img")
}

func (d *diskStore) get(hash string) ([]byte, bool) {
	now := d.now()
	// 读文件、判断 TTL、更新索引和 mtime 都在同一把磁盘锁里完成。磁盘 I/O
	// 不在渲染帧上；串行化后维护/删除不会在「读到文件」和「写回索引」之间
	// 插进来，也就不会把已经被删掉的条目重新写成幽灵元数据。
	d.mu.Lock()
	defer d.mu.Unlock()

	// 不在这里扫目录，也不盲信索引里的过期时间：直接读这个哈希对应的
	// 唯一文件，由文件头里的 stored 判断 TTL。这样另一个进程刚重写过的
	// 同 key 文件不会被本进程的旧索引误删。
	path := d.path(hash)
	data, stored, size, err := readDiskEntry(path, now, d.ttl)
	if err != nil {
		if errors.Is(err, errBadDiskEntry) {
			d.removeLocked(hash)
		} else if os.IsNotExist(err) {
			d.forgetLocked(hash)
		}
		return nil, false
	}

	if old, had := d.index[hash]; had {
		d.bytes -= old.size
	}
	d.index[hash] = diskMeta{size: size, stored: stored, used: now}
	d.bytes += size

	// 磁盘 LRU 用 mtime 持久化（stored 在文件头里，读不会续期 TTL）。
	// 磁盘命中只发生在 RAM 未命中时，不是每帧，所以每次都写 mtime 也不会
	// 给渲染帧增加 I/O；这样跨进程重启后“最近访问”仍然准确。
	_ = os.Chtimes(path, now, now)
	return data, true
}

func (d *diskStore) put(hash string, data []byte) (bool, error) {
	d.mu.Lock()
	blocked := d.cleanupFailed
	d.mu.Unlock()
	if blocked {
		return false, errors.New("磁盘缓存上次清理失败，暂停写入")
	}
	if len(data) > maxDownloadBytes {
		return false, fmt.Errorf("图片超过 4MiB 上限，未写磁盘")
	}
	size := int64(diskHeaderSize + len(data))
	if size > d.maxBytes {
		// 单张图比整个磁盘预算还大：不写，但这不是磁盘故障（也不算失败），
		// 没必要污染 DiskInfo.Err。
		return false, nil
	}
	now := d.now()

	tmp, err := os.CreateTemp(d.dir, ".tmp-")
	if err != nil {
		return false, fmt.Errorf("创建磁盘缓存临时文件: %w", err)
	}
	tmpName := tmp.Name()
	defer func() {
		if tmpName != "" {
			_ = os.Remove(tmpName)
		}
	}()
	header := make([]byte, diskHeaderSize)
	copy(header, diskMagic)
	binary.LittleEndian.PutUint64(header[len(diskMagic):], uint64(now.UnixNano()))
	if _, err = tmp.Write(header); err == nil {
		_, err = tmp.Write(data)
	}
	if err == nil {
		err = tmp.Sync()
	}
	if cerr := tmp.Close(); err == nil {
		err = cerr
	}
	if err != nil {
		return false, fmt.Errorf("写磁盘缓存临时文件: %w", err)
	}
	if err := os.Chmod(tmpName, 0o600); err != nil {
		return false, fmt.Errorf("设置磁盘缓存文件权限: %w", err)
	}

	// rename 与索引更新在同一把锁里：并发同 key / 维护扫描不会看到半个文件。
	d.mu.Lock()
	if d.cleanupFailed {
		d.mu.Unlock()
		return false, errors.New("磁盘缓存上次清理失败，暂停写入")
	}
	if err := os.Rename(tmpName, d.path(hash)); err != nil {
		d.mu.Unlock()
		return false, fmt.Errorf("提交磁盘缓存文件: %w", err)
	}
	tmpName = ""
	if old, ok := d.index[hash]; ok {
		d.bytes -= old.size
	}
	d.index[hash] = diskMeta{size: size, stored: now, used: now}
	d.bytes += size
	if !d.enforceCapacityLocked() {
		// 旧文件删不掉，容量仍超限：不要继续往上堆。回滚刚写的这一条；
		// 如果连它都删不掉，就暂停后续写入，等维护把实际文件清下去。
		removed := d.removeLocked(hash)
		if !removed {
			d.cleanupFailed = true
		}
		d.mu.Unlock()
		return false, errors.New("磁盘缓存超出容量且清理失败，已丢弃本次写入")
	}
	_, kept := d.index[hash]
	d.mu.Unlock()
	return kept, nil
}

func (d *diskStore) remove(hash string) {
	d.mu.Lock()
	d.removeLocked(hash)
	d.mu.Unlock()
}

// removeLocked 删除一个磁盘文件。只有文件确实不存在/删除成功后才扣减索引
// 和字节账目；删除失败时保留账目，否则实际磁盘占用会高于对外声明的上限。
// 返回 false 表示文件仍在磁盘上（调用方应跳过它、不要继续假装已释放）。
func (d *diskStore) removeLocked(hash string) bool {
	path := d.path(hash)
	var err error
	if d.removeFile != nil {
		err = d.removeFile(path)
	} else {
		err = os.Remove(path)
	}
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		return false
	}
	if m, ok := d.index[hash]; ok {
		delete(d.index, hash)
		d.bytes -= m.size
		if d.bytes < 0 {
			d.bytes = 0
		}
	}
	return true
}

func (d *diskStore) forget(hash string) {
	d.mu.Lock()
	d.forgetLocked(hash)
	d.mu.Unlock()
}

// forgetLocked 只丢索引账目（文件已确认不存在，不需要再删）。
func (d *diskStore) forgetLocked(hash string) {
	if m, ok := d.index[hash]; ok {
		delete(d.index, hash)
		d.bytes -= m.size
		if d.bytes < 0 {
			d.bytes = 0
		}
	}
}

func (d *diskStore) effectiveMaxEntries() int {
	if d.maxEntries > 0 {
		return d.maxEntries
	}
	return diskMaxEntries
}

// enforceCapacityLocked 按最近使用时间淘汰，直到条目数和字节数都回到
// 上限内。删除失败的文件保留在索引里并继续尝试下一个；整轮扫描完就返回，
// 绝不在同一个删不掉的条目上循环。返回 false 表示扫描后仍超限（通常因为
// 文件删不掉），调用方应停止继续写入。调用方持有 d.mu。
func (d *diskStore) enforceCapacityLocked() bool {
	within := func() bool {
		return d.bytes <= d.maxBytes && len(d.index) <= d.effectiveMaxEntries()
	}
	if within() {
		return true
	}
	type item struct {
		hash string
		used time.Time
	}
	items := make([]item, 0, len(d.index))
	for hash, m := range d.index {
		items = append(items, item{hash, m.used})
	}
	sort.Slice(items, func(i, j int) bool { return items[i].used.Before(items[j].used) })
	for _, it := range items {
		if within() {
			return true
		}
		d.removeLocked(it.hash)
	}
	return within()
}

// maintain 是启动 / 后台清理：重建索引、删除过期条目和超过 1 小时的临时
// 文件、执行容量淘汰。它只读每个文件的头部（不解码），因此不会把图片
// 字节留在内存里。
func (d *diskStore) maintain() error {
	now := d.now()
	d.mu.Lock()
	defer d.mu.Unlock()

	entries, err := os.ReadDir(d.dir)
	if err != nil {
		return err
	}
	seen := make(map[string]struct{}, len(entries))
	for _, de := range entries {
		name := de.Name()
		if strings.HasPrefix(name, ".tmp-") {
			if info, err := de.Info(); err == nil && now.Sub(info.ModTime()) > diskTempMaxAge {
				_ = os.Remove(filepath.Join(d.dir, name))
			}
			continue
		}
		// 只管理自己的普通文件：不跟随符号链接，也不碰子目录。
		if !de.Type().IsRegular() || !strings.HasSuffix(name, ".img") {
			continue
		}
		hash := strings.TrimSuffix(name, ".img")
		if !validDiskHash(hash) {
			continue
		}
		path := d.path(hash)
		meta, err := readDiskMeta(path)
		if err != nil {
			if errors.Is(err, errBadDiskEntry) {
				if d.removeLocked(hash) {
					continue
				}
				// 坏文件删不掉（权限/配额）：按实际文件大小继续记账，不能
				// 让它变成 unseen 后从索引里消失、成为账外磁盘占用。容量
				// 清理会把它算进去；若仍超限，cleanupFailed 会暂停写入。
				if info, statErr := os.Lstat(path); statErr == nil && info.Mode().IsRegular() {
					meta = diskMeta{size: info.Size(), stored: info.ModTime(), used: info.ModTime()}
					d.index[hash] = meta
					seen[hash] = struct{}{}
				}
			}
			continue
		}
		seen[hash] = struct{}{}
		// 维护扫描读到的 mtime 可能比内存里刚发生的访问旧（Chtimes 失败或
		// 文件系统精度问题）；保留内存索引里更新的 used，不要把最近使用
		// 时间重置回旧值。
		if old, ok := d.index[hash]; ok && old.used.After(meta.used) {
			meta.used = old.used
		}
		if !now.Before(meta.stored.Add(d.ttl)) {
			d.index[hash] = meta
			if d.removeLocked(hash) {
				delete(seen, hash)
			}
			continue
		}
		d.index[hash] = meta
	}
	// 索引里对应文件已经不存在的条目（外部删除 / 上次清理之后崩溃）也丢掉。
	d.bytes = 0
	for hash, m := range d.index {
		if _, ok := seen[hash]; !ok {
			delete(d.index, hash)
			continue
		}
		d.bytes += m.size
	}
	d.cleanupFailed = !d.enforceCapacityLocked()
	d.lastMaint = now
	return nil
}

func (d *diskStore) maybeMaintain() {
	now := d.now()
	d.mu.Lock()
	if d.maintRunning || (!d.lastMaint.IsZero() && now.Sub(d.lastMaint) < diskMaintenanceInterval) {
		d.mu.Unlock()
		return
	}
	d.maintRunning = true
	d.mu.Unlock()
	go func() {
		_ = d.maintain()
		d.mu.Lock()
		d.maintRunning = false
		d.lastMaint = d.now()
		d.mu.Unlock()
	}()
}

func validDiskHash(hash string) bool {
	if len(hash) != sha256.Size*2 {
		return false
	}
	_, err := hex.DecodeString(hash)
	return err == nil
}

func readDiskMeta(path string) (diskMeta, error) {
	// 只用 Lstat 认自己的普通文件：不跟随符号链接，也不接受目录。
	info, err := os.Lstat(path)
	if err != nil {
		return diskMeta{}, err
	}
	if !info.Mode().IsRegular() || info.Size() < int64(diskHeaderSize) || info.Size() > maxDiskEntryBytes() {
		return diskMeta{}, errBadDiskEntry
	}
	f, err := os.Open(path)
	if err != nil {
		return diskMeta{}, err
	}
	defer f.Close()
	header := make([]byte, diskHeaderSize)
	if _, err := io.ReadFull(f, header); err != nil {
		return diskMeta{}, errBadDiskEntry
	}
	stored, err := parseDiskHeader(header)
	if err != nil {
		return diskMeta{}, err
	}
	return diskMeta{size: info.Size(), stored: stored, used: info.ModTime()}, nil
}

func readDiskEntry(path string, now time.Time, ttl time.Duration) ([]byte, time.Time, int64, error) {
	// 同 readDiskMeta：只接受普通文件，符号链接/目录一律当作坏条目。
	info, err := os.Lstat(path)
	if err != nil {
		return nil, time.Time{}, 0, err
	}
	if !info.Mode().IsRegular() || info.Size() < int64(diskHeaderSize) || info.Size() > maxDiskEntryBytes() {
		return nil, time.Time{}, 0, errBadDiskEntry
	}
	f, err := os.Open(path)
	if err != nil {
		return nil, time.Time{}, 0, err
	}
	defer f.Close()
	all, err := io.ReadAll(io.LimitReader(f, maxDiskEntryBytes()+1))
	if err != nil {
		return nil, time.Time{}, 0, err
	}
	if int64(len(all)) != info.Size() || len(all) < diskHeaderSize {
		return nil, time.Time{}, 0, errBadDiskEntry
	}
	stored, err := parseDiskHeader(all[:diskHeaderSize])
	if err != nil {
		return nil, time.Time{}, 0, err
	}
	if !now.Before(stored.Add(ttl)) {
		return nil, time.Time{}, 0, errBadDiskEntry
	}
	return all[diskHeaderSize:], stored, info.Size(), nil
}

func parseDiskHeader(header []byte) (time.Time, error) {
	if len(header) < diskHeaderSize || string(header[:len(diskMagic)]) != diskMagic {
		return time.Time{}, errBadDiskEntry
	}
	nanos := int64(binary.LittleEndian.Uint64(header[len(diskMagic):]))
	stored := time.Unix(0, nanos)
	if stored.Unix() <= 0 {
		return time.Time{}, errBadDiskEntry
	}
	return stored, nil
}

func maxDiskEntryBytes() int64 {
	return int64(diskHeaderSize) + int64(maxDownloadBytes)
}
