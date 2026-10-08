package view

import (
	"time"

	"github.com/egoist/mygo"
	"github.com/egoist/mygo/ui"

	"github.com/vst93/bili-fm/app/internal/imagecache"
)

// Card 是列表里的一张卡片要显示的内容。
type Card struct {
	// Bvid 用于和正在播放的曲目比对，判断是不是当前这条。
	Bvid     string
	Cover    string
	Title    string
	Duration string // 封面右下角角标，空则不画
	Meta     []string
	// Track 是点击这张卡片要播放的内容。
	Track Track
}

// Track 是正在播放（或可播放）的一条视频。
type Track struct {
	Aid      int64
	Bvid     string
	Cid      int64
	Title    string
	Up       string
	Cover    string
	Duration int64 // 秒
	// Part 是分集标题（多 P 时用）。
	Part string
}

// Part 是一个分集。
type Part struct {
	Cid  int64
	Page int
	Part string
}

// Danmaku 是一条弹幕。
type Danmaku struct {
	Time float64
	Text string
}

// Comment 是一条评论。
type Comment struct {
	User    string
	Avatar  string
	Content string
	Likes   int64
	Time    string
}

// Section 是导航里的一个分区。
type Section struct {
	Key   string // recommend / popular / feed / favorite / history / watchlater
	Label string
	// NeedLogin 为真时未登录就提示登录而不是发请求。
	NeedLogin bool
}

// Sections 是顶部分区，顺序与原版一致。
var Sections = []Section{
	// 推荐接口未登录时返回空，所以也标成需要登录，界面上给提示而不是白屏。
	{Key: "recommend", Label: "推荐", NeedLogin: true},
	{Key: "popular", Label: "热门"},
	{Key: "feed", Label: "动态", NeedLogin: true},
	{Key: "favorite", Label: "收藏", NeedLogin: true},
	{Key: "history", Label: "历史", NeedLogin: true},
	{Key: "watchlater", Label: "稍后再看", NeedLogin: true},
}

// Actions 是界面向上层发出的请求。由 main 装配时注入实现。
type Actions struct {
	LoadSection   func(section string, page int)
	Search        func(query string)
	Play          func(index int)
	TogglePlay    func()
	Next          func()
	Prev          func()
	Seek          func(seconds float64)
	SetSpeed      func(v float64)
	ToggleEQ      func()
	ToggleSponsor func()
	SetVolume     func(v float64)
	OpenVideo     func()
	CloseVideo    func()
	OpenParts     func(t Track)
	ToggleDanmaku func()
	LoadMore      func()
	Login         func()
	Like          func()
	Coin          func()
	Favorite      func()
	Follow        func()
	Quit          func()
	Minimize      func()
	SetMini       func(on bool)
	SaveQueue     func()
}

// App 是主窗口的全部状态。
type App struct {
	Win    *mygo.Window
	Theme  Theme
	Images *imagecache.Cache
	Act    Actions

	// 导航
	Section  int
	Cards    []Card
	Loading  bool
	Status   string
	Page     int
	HasMore  bool
	LoggedIn bool
	UName    string

	// 播放
	Queue   []Track
	Index   int
	Track   *Track
	Playing bool
	Pos     float64
	Dur     float64
	Speed   float64
	EQ      bool
	Sponsor bool
	Volume  float64
	// Buffering 表示正在起播（网络 + 解码初始化）。
	Buffering bool

	// 分集 / 弹幕 / 评论
	Parts       []Part
	ShowParts   bool
	ShowDanmaku bool
	ShowInfo    bool
	Danmaku     []Danmaku
	Comments    []Comment

	// 互动状态
	Liked    bool
	Coined   bool
	Faved    bool
	Followed bool

	// 视频弹窗
	VideoOpen bool

	// 搜索
	Query string

	// 进度条：拖动期间用 SeekValue，松手才真正 seek（seek 会重建解码器，
	// 不能跟着拖动一路触发）。seeking 标记拖动中，SeekValue 是滑块绑定的值。
	seeking   bool
	SeekValue float64
	// ShowSpeed 表示倍速档位菜单展开着。
	ShowSpeed bool

	// Mini 表示处于迷你模式（400×155 置顶小窗）。
	Mini bool
}

// NewApp 建一个用当前时段主题的应用。
func NewApp(repaint func()) *App {
	a := &App{Theme: ThemeAt(time.Now()), Speed: 1, Volume: 1, Images: imagecache.New(repaint)}
	return a
}

// RefreshTheme 在时段变化时重新取令牌。
func (a *App) RefreshTheme() { a.Theme = ThemeAt(time.Now()) }

// CurrentSection 返回当前分区。
func (a *App) CurrentSection() Section {
	if a.Section < 0 || a.Section >= len(Sections) {
		return Sections[0]
	}
	return Sections[a.Section]
}

// Current 返回正在播放的曲目（可能为 nil）。
func (a *App) Current() *Track {
	if a.Index < 0 || a.Index >= len(a.Queue) {
		return nil
	}
	return &a.Queue[a.Index]
}

// progress 返回播放进度 0..1。
func (a *App) progress() float32 {
	if a.Dur <= 0 {
		return 0
	}
	p := a.Pos / a.Dur
	if p < 0 {
		return 0
	}
	if p > 1 {
		return 1
	}
	return float32(p)
}

// icon 解析一段内联 SVG，用于界面图标。
func icon(shapes string) *ui.SVG {
	return ui.MustParseSVG([]byte(`<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 16 16" fill="none" stroke="currentColor" stroke-width="1.4" stroke-linecap="round" stroke-linejoin="round">` + shapes + `</svg>`))
}

var (
	iconPlay    = icon(`<path d="M4.5 2.8v10.4l8.5-5.2z"/>`)
	iconPause   = icon(`<path d="M5.2 3v10M10.8 3v10"/>`)
	iconPrev    = icon(`<path d="M11.5 3v10L4.5 8z"/><path d="M4.5 3v10"/>`)
	iconNext    = icon(`<path d="M4.5 3v10L11.5 8z"/><path d="M11.5 3v10"/>`)
	iconVideo   = icon(`<rect x="1.5" y="3.5" width="13" height="9" rx="1.6"/><path d="M7 6.6l3 1.4-3 1.4z"/>`)
	iconDanmaku = icon(`<rect x="1.5" y="2.8" width="13" height="8.6" rx="1.6"/><path d="M4.5 14.2l2-2.8"/><path d="M4.6 5.9h6.8M4.6 8.4h4.4"/>`)
	iconList    = icon(`<path d="M5.5 4h9M5.5 8h9M5.5 12h9"/><path d="M2.2 4h.6M2.2 8h.6M2.2 12h.6"/>`)
	iconSearch  = icon(`<circle cx="7" cy="7" r="4.4"/><path d="M10.4 10.4L14 14"/>`)
	iconClose   = icon(`<path d="M4 4l8 8M12 4l-8 8"/>`)
	iconMin     = icon(`<path d="M4 8h8"/>`)
	iconLike    = icon(`<path d="M5.2 7.2l2.6-4.6a1.2 1.2 0 0 1 2.2.6V6.4h3.1a1.2 1.2 0 0 1 1.2 1.4l-.9 5A1.2 1.2 0 0 1 12.2 14H5.2"/><path d="M5.2 7.2H2.6V14h2.6z"/>`)
	iconCoin    = icon(`<circle cx="8" cy="8" r="6"/><path d="M8 5v6M6 8h4"/>`)
	iconStar    = icon(`<path d="M8 1.8l1.9 4.2 4.6.5-3.4 3.1.9 4.5L8 11.9l-4 2.2.9-4.5L1.5 6.5l4.6-.5z"/>`)
	iconSpeed   = icon(`<circle cx="8" cy="8" r="6"/><path d="M8 8l3-2.2"/>`)
	iconEQ      = icon(`<path d="M3 11V5M6.5 13V3M10 10V6M13.5 12V4"/>`)
	iconVolume  = icon(`<path d="M3 6.2h2.2L8.2 3.5v9L5.2 9.8H3z"/><path d="M10.6 6.2a2.6 2.6 0 0 1 0 3.6"/>`)
	iconSponsor = icon(`<path d="M2 4.5h12v7H2z"/><path d="M5 7.5h6"/>`)
)
