package view

import (
	"bytes"
	"fmt"
	"image"
	"image/png"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/egoist/mygo/ui"
)

// testCoverPNG 是一张够小的封面图，供懒加载测试用。
func testCoverPNG() []byte {
	im := image.NewRGBA(image.Rect(0, 0, 24, 15))
	for i := 0; i < len(im.Pix); i += 4 {
		im.Pix[i], im.Pix[i+1], im.Pix[i+2], im.Pix[i+3] = 200, 100, 50, 255
	}
	var b bytes.Buffer
	_ = png.Encode(&b, im)
	return b.Bytes()
}

// coverServer 记录被请求的封面路径，供测试断言「谁被请求了」。
type coverServer struct {
	*httptest.Server
	mu    sync.Mutex
	paths []string
}

func newCoverServer(t *testing.T) *coverServer {
	t.Helper()
	data := testCoverPNG()
	cs := &coverServer{}
	cs.Server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		cs.mu.Lock()
		cs.paths = append(cs.paths, r.URL.Path)
		cs.mu.Unlock()
		w.Header().Set("Content-Type", "image/png")
		w.Write(data)
	}))
	t.Cleanup(cs.Close)
	return cs
}

func (cs *coverServer) requested(path string) bool {
	cs.mu.Lock()
	defer cs.mu.Unlock()
	for _, p := range cs.paths {
		if p == path {
			return true
		}
	}
	return false
}

func (cs *coverServer) count() int {
	cs.mu.Lock()
	defer cs.mu.Unlock()
	return len(cs.paths)
}

// TestCardGridLazyLoadsVisibleCovers 钉住「只请求可见行的封面」：
//
// 从前 renderCard 在构建时直接 Bitmap()，而 mygo 的 List 在行高还没量到之前
// 用主题估计（heights.def = Space(8)）先把行铺开，于是抽屉刚开、一屏只放得下
// ~5 行时，会一次请求 90+ 张封面。现在改成「构建只读、按可见窗口预取」，
// 屏外卡片（如第 250 张）绝不请求，也不 decode。
func TestCardGridLazyLoadsVisibleCovers(t *testing.T) {
	cs := newCoverServer(t)
	a := NewApp(func() {})
	cards := make([]Card, 300)
	for i := range cards {
		cards[i] = Card{
			Bvid: fmt.Sprintf("BV%04d", i), Title: fmt.Sprintf("标题%d", i),
			Up: "UP主", Cover: fmt.Sprintf("%s/c%d.png", cs.URL, i),
		}
	}
	a.ListFor("feed").Cards = cards
	a.Drawer = "feed"

	tst := ui.NewTester(func(c *ui.Context) { a.Shell(c) }, 1400, 900)
	// 等首屏封面抓完。
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		tst.Frame()
		if cs.count() > 0 && a.Images.Pending() == 0 {
			break
		}
		time.Sleep(5 * time.Millisecond)
	}

	n := cs.count()
	if n == 0 {
		t.Fatal("首屏可见封面一张都没请求")
	}
	// 一屏约 5 行 × 3 列 = 15 张；给预取留余量，但不该接近全表。
	if n > 60 {
		t.Fatalf("首屏请求了 %d 张封面（屏外卡片也被请求，懒加载失效）", n)
	}
	if cs.requested("/c250.png") {
		t.Fatal("第 250 张在屏外，不该被请求")
	}

	// 滚到底：远处的卡片这时才请求。
	tst.Scroll(700, 700, 0, 100000)
	for i := 0; i < 5; i++ {
		tst.Frame()
	}
	if !cs.requested("/c299.png") {
		t.Fatal("滚到底后最后一张的封面应被请求")
	}
}

// TestCardGridPrefetchWindow 钉住预取窗口的算法：未布局时只预取初始几行，
// 布局后只覆盖可见行上下各一点，绝不整表铺开。
func TestCardGridPrefetchWindow(t *testing.T) {
	a := NewApp(func() {})
	cards := make([]Card, 300)
	for i := range cards {
		cards[i] = Card{Bvid: fmt.Sprintf("BV%04d", i), Cover: fmt.Sprintf("cover%d", i)}
	}

	// 记录实际被请求的 URL（去重：同 URL 多帧只算一次）。
	asked := newRecordingCache()
	a.Images = asked

	var st ui.ListState
	a.ListFor("feed").Cards = cards

	// 未布局（Visible 返回 0,-1）：只预取初始行（含 overscan 余量）。
	a.prefetchCardCovers(&st, cards)
	got := asked.unique()
	want := (initialCardRows + 1) * cardGridCols
	if got != want {
		t.Fatalf("未布局时预取 %d 张，want %d", got, want)
	}
	if firstURL := asked.order[0]; firstURL != "cover0" {
		t.Fatalf("应从第一张开始预取，得到 %q", firstURL)
	}
}

// recordingCache 是一个只记录请求的假 Images（实现 ImageStore）。
type recordingCache struct {
	mu    sync.Mutex
	set   map[string]bool
	order []string
}

func newRecordingCache() *recordingCache {
	return &recordingCache{set: map[string]bool{}}
}

func (r *recordingCache) Bitmap(url string) *ui.Bitmap {
	if url == "" {
		return nil
	}
	r.mu.Lock()
	if !r.set[url] {
		r.set[url] = true
		r.order = append(r.order, url)
	}
	r.mu.Unlock()
	return nil
}

func (r *recordingCache) Get(url string) *ui.Bitmap { return nil }

func (r *recordingCache) Pending() int { return 0 }

func (r *recordingCache) Ambient(url string, blur bool) *ui.Bitmap { return nil }

func (r *recordingCache) unique() int {
	r.mu.Lock()
	defer r.mu.Unlock()
	return len(r.set)
}

// TestOffScreenCardsUseGetNotFetch 是算法层面的兜底：Lazy 的卡片只 Get，
// 不会在构建时发请求。配合上面的窗口测试，保证「请求」只来自预取。
func TestOffScreenCardsUseGetNotFetch(t *testing.T) {
	a := NewApp(func() {})
	asked := newRecordingCache()
	a.Images = asked
	cards := make([]Card, 50)
	for i := range cards {
		cards[i] = Card{Bvid: fmt.Sprintf("BV%04d", i), Title: "t", Cover: fmt.Sprintf("cover%d", i)}
	}
	a.ListFor("feed").Cards = cards
	a.Drawer = "feed"

	tst := ui.NewTester(func(c *ui.Context) { a.Shell(c) }, 1400, 900)
	tst.Frame()
	// 预取窗口最多覆盖首屏 + 余量：不该是 50 张全请求。
	if got := asked.unique(); got >= len(cards) {
		t.Fatalf("构建期请求了 %d/%d 张（应为可见窗口）", got, len(cards))
	}
	for _, u := range asked.order {
		if strings.HasSuffix(u, "cover49") {
			t.Fatal("屏外第 49 张被请求")
		}
	}
}
