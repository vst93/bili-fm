package view

import (
	"fmt"
	"sync"
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
	// meta 行按旧版 CardMeta 的字段类型分开存，视图按卡片宽度决定显示哪几列
	// （作者 40% / 播放量 30% / 发布时间 30%，窄了从末尾隐藏）。
	Up      string // 作者
	Views   string // 播放量
	Pubdate string // 发布时间（相对或短的绝对形式）
	Extra   string // 附加（稍后再看的「已看 30%」等）
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
	// Duration 是这一集的时长（秒），FirstFrame 是这一集的封面首帧。
	Duration   int64
	FirstFrame string
}

// Danmaku 是一条弹幕。
type Danmaku struct {
	Time float64
	Text string
}

// Segment 是一个跳过分段（SponsorBlock），用于进度条上的广告段标记。
type Segment struct {
	Start, End float64
}

// Comment 是一条评论（Replies 是楼中楼预览，原版只取前几条）。
type Comment struct {
	User    string
	Avatar  string
	Content string
	Likes   int64
	Time    string
	Replies []Comment
}

// PlayItem 是播放列表里的一条记录（原版 PlaylistItem）。
type PlayItem struct {
	ID         string // bvid-cid，去重用
	Bvid       string
	Aid        int64
	Cid        int64
	Part       string
	FirstFrame string
	Title      string
	Pic        string
}

// List 是一个抽屉的列表状态。
//
// 旧版每个抽屉有自己的 state（各自的 cards / loading / page / hasMore），
// 所以切抽屉不会互相覆盖，也不会闪一下上一个抽屉的内容。
type List struct {
	Cards   []Card
	Loading bool
	Page    int
	HasMore bool
}

// Section 是搜索栏右侧一个入口对应的抽屉。
//
// 注意：旧版的「热门与推荐」「历史」是两个抽屉内各自带 tab，
// 而不是四个独立分区（见 recommendList.tsx / historyList.tsx）。
type Section struct {
	Key   string // feed / popular / favorite / history
	Label string
	// NeedLogin 为真时未登录就提示登录而不是发请求。
	NeedLogin bool
}

// Folder 是一个收藏夹（收藏抽屉的表头 tab）。
type Folder struct {
	ID    int64
	Title string
	Count int64
}

// Series 是 UP 主的一个合集（UP 空间抽屉的「合集」tab）。
type Series struct {
	ID    int64
	Title string
	Count int64
}

// Info 是当前视频的详情（标题 / 简介 / UP 主 / 分集 / 互动数），
// 来自 GetCList，主区右栅和「详情」抽屉都用它。
type Info struct {
	Aid       int64
	Bvid      string
	Cid       int64
	Title     string
	Desc      string
	Pic       string
	OwnerMid  int64
	OwnerName string
	OwnerFace string
	Parts     []Part
	Like      int64
	Coin      int64
	Favorite  int64
	View      int64
}

// 抽屉的 key。Drawer 为空表示没有抽屉打开；分区抽屉直接用分区的 key。
const (
	DrawerSearch  = "search"
	DrawerParts   = "parts"
	DrawerDanmaku = "danmaku"
	DrawerInfo    = "info"
	// DrawerUp 是「UP 主的空间」（视频 / 合集两个 tab），点主区的 UP 头像进。
	DrawerUp = "up"
	// DrawerSeries 是一个合集的视频列表，从 UP 空间里选一个合集进。
	DrawerSeries = "series"
	// DrawerPlaylist 是播放列表（我的列表 / 合集列表两个 tab）。
	DrawerPlaylist = "playlist"
)

// 播放模式（原版 PlaylistPlayMode）。
const (
	PlayModeSequence = "sequence"
	PlayModeSingle   = "single"
	PlayModeShuffle  = "shuffle"
)

// 播放列表的来源（原版 activePlaylistType）。
const (
	ListUser   = "user"
	ListSeries = "series"
)

// 弹幕/评论抽屉的两个 tab。
const (
	TabDanmaku = "danmaku"
	TabReply   = "reply"
)

// UP 空间抽屉的两个 tab（原版 upVideoList.tsx 的「视频 / 合集」）。
const (
	UpTabVideos = "videos"
	UpTabSeries = "series"
)

// Sections 是搜索栏右侧四个入口对应的抽屉，顺序与原版一致。
var Sections = []Section{
	{Key: "feed", Label: "动态", NeedLogin: true},
	// 热门不需要登录，推荐需要，所以这个抽屉的登录提示看 RecTab。
	{Key: "popular", Label: "热门与推荐"},
	{Key: "favorite", Label: "收藏", NeedLogin: true},
	// 观看历史需要登录；稍后再看也一样（接口会直接返回空）。
	{Key: "history", Label: "历史", NeedLogin: true},
}

// 抽屉表头 tab 的取值（与原版 searchList / recommendList / historyList 一致）。
const (
	SortTotal  = "totalrank" // 搜索：综合
	SortClick  = "click"     // 搜索：最多播放
	SortUpdate = "update"    // 搜索：最新发布

	RecHot       = "hot"       // 热门与推荐：热门（GetBLPopularList）
	RecRecommend = "recommend" // 热门与推荐：推荐（GetBLRCMDList）

	HistHistory    = "history"    // 历史：观看历史
	HistWatchLater = "watchlater" // 历史：稍后再看
)

// Modal 是一个模态对话框（原版的 DialogProvider / 登录面板）。
//
// 同一时刻只显示一个；Kind 决定内容：
//
//	login      扫码登录（QR 是二维码位图）
//	about      关于应用
//	shortcuts  快捷键
//	update     检查更新 / 下载进度
//	message    通用提示（标题 + 正文）
type Modal struct {
	Kind    string
	Title   string
	Message string
	// QR 是登录二维码的位图（Kind == "login"）。
	QR *ui.Bitmap
	// 更新相关（Kind == "update"）：目标版本号与下载进度（Total 为 0 表示不确定）。
	Version    string
	Downloaded int64
	Total      int64
	Busy       bool
}

// Actions 是界面向上层发出的请求。由 main 装配时注入实现。
type Actions struct {
	LoadSection func(section string, page int)
	// Reload 按当前抽屉的 tab / 排序重拉第一页（刷新键和切 tab 都走它）。
	Reload func()
	// SelectFolder 切收藏夹（收藏抽屉的表头 tab）。
	SelectFolder func(id int64)
	// OpenUp 打开某个 UP 主的空间（主区的 UP 头像 / 名字）。
	OpenUp func(mid int64, name string)
	// SelectSeries 在 UP 空间里选一个合集。
	SelectSeries func(id int64)
	// ToggleFollow 关注 / 取关当前视频的 UP（或 UP 空间里那个）。
	ToggleFollow func()
	// ToggleIncognito 切换隐身模式（历史抽屉表头）。
	ToggleIncognito func()
	Search          func(query string)
	// UrlJump 直接打开一个 B 站视频链接（搜索框里粘链接时走它）。
	UrlJump func(url string)
	// OpenCard 点击列表卡片：拉详情并打开选集面板（与原版 handleSearchVideoSelect
	// / handleUrlJump 一致，不直接起播）。
	OpenCard func(index int)
	// OpenBrowser 用系统浏览器打开当前视频。
	OpenBrowser   func()
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
	CloseLogin    func()
	ShowAbout     func()
	ShowShortcuts func()
	CheckUpdate   func()
	CloseModal    func()
	Like          func()
	Coin          func()
	Favorite      func()
	Follow        func()
	Quit          func()
	Minimize      func()
	SetMini       func(on bool)
	TogglePin     func()
	SetCoverMode  func(mode string)
	ToggleAmbient func()
	TogglePremium func()
	// 播放列表
	PlayPlaylist       func(list string, index int)
	DeletePlaylistItem func(list, id string)
	ReorderPlaylist    func(list string, from, to int)
	ClearPlaylist      func(list string)
	CyclePlayMode      func()
	SwitchPlaylistTab  func(list string)
	AddToPlaylist      func(part Part)
	AddAllToPlaylist   func()
	SeriesPlayAll      func()
	// 弹幕/评论
	SwitchDanmakuTab func(tab string)
	LoadComments     func(page int)
	// RemoveWatchLater 把一条从「稍后再看」移除（原版 historyList 的移除键）。
	RemoveWatchLater func(aid int64)
	SaveQueue        func()
}

// App 是主窗口的全部状态。
type App struct {
	Win    *mygo.Window
	Theme  Theme
	Images *imagecache.Cache
	Act    Actions

	// 导航
	Section int
	// Lists 按抽屉 key（分区 key 或 DrawerSearch）存各自的列表。
	Lists map[string]*List
	// listsMu 保护 Lists 这个 map：网络 goroutine 与界面线程都会调 ListFor，
	// 而 Go 的 map 并发读写会直接 panic。
	listsMu  sync.Mutex
	Status   string
	LoggedIn bool
	UName    string
	Face     string // 登录用户头像 URL，未登录为空

	// 抽屉表头（tab / 排序 / 收藏夹）
	SortOrder string // 搜索排序：SortTotal / SortClick / SortUpdate
	RecTab    string // 热门与推荐：RecHot / RecRecommend
	HistTab   string // 历史：HistHistory / HistWatchLater
	Folders   []Folder
	FolderID  int64
	// Incognito 是隐身模式：不读也不写云端的观看记录与进度（只用本地断点）。
	Incognito bool

	// UP 空间 / 合集
	UpMid      int64
	UpName     string
	UpFollowed bool
	UpFans     int64  // 粉丝数，UP 空间表头显示
	UpTab      string // UpTabVideos / UpTabSeries
	SeriesList []Series
	SeriesID   int64
	SeriesName string

	// UpOffset 是 UP 视频列表的翻页游标（旧版是 offset 而不是页码）。
	UpOffset string

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
	// SponsorStatus 是 SponsorBlock 查询状态：off/loading/ok/empty/error，
	// 用于播放栏按钮上的状态点（原版解 SponsorStatusInfo）。
	SponsorStatus string
	// SponsorSegments 是当前曲目的跳过分段（进度条上的广告段标记）。
	SponsorSegments []Segment
	// Buffering 表示正在起播（网络 + 解码初始化）。
	Buffering bool

	// 分集 / 弹幕 / 评论 / 详情
	Info     *Info
	Danmaku  []Danmaku
	Comments []Comment

	// 弹幕/评论抽屉：DanmakuTab 是当前 tab；DanmakuAutoScroll 是自动跟随
	// 当前播放时间；Reply* 是评论分页状态。
	DanmakuTab        string
	DanmakuAutoScroll bool
	RepliesLoading    bool
	RepliesHasMore    bool
	ReplyPage         int
	ReplyTotal        int
	danmakuScrollIdx  int

	// 播放列表（原版 playlist / seriesPlaylist）。
	//
	// Playlist 是「我的列表」（用户从选集面板手动添加的）；SeriesPlaylist 是
	// 「合集列表」（从合集「播放全部」加载的）。两者都落盘。
	Playlist       []PlayItem
	SeriesPlaylist []PlayItem
	// PlayingPlaylist 是当前正在播放的来源：ListUser / ListSeries / ""（从
	// 普通列表或选集进入时为空）。
	PlayingPlaylist string
	// PlayMode 是播放模式（sequence / single / shuffle）。
	PlayMode string
	// PlaylistTab 是播放列表抽屉当前显示的 tab。
	PlaylistTab string
	// locateNow 在点了「定位到当前」后置位，下一帧把当前行滚进视野。
	locateNow bool

	// Drawer 是当前打开的抽屉："" 表示没有，否则是分区 key 或
	// DrawerSearch / DrawerParts / DrawerDanmaku / DrawerInfo。
	// 原版同一时刻只有一个抽屉（HeroUI Drawer 的 isOpen）。
	Drawer string

	// 封面圆盘的旋转角（度）。播放时按真实时间累加，暂停时停住
	// （原版是 #video-cover.record-disc 的 22s CSS 动画）。
	discDeg float32
	discAt  time.Time
	// shownCoverBmp 是当前正在显示的封面位图。新封面未加载完时先沿用旧的
	// （原版「先预载、后换源」），避免切歌闪一下空白。
	shownCoverBmp *ui.Bitmap

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
	// ShowSpeed / ShowVolume 表示播放栏的倍速、音量弹层展开着。
	ShowSpeed  bool
	ShowVolume bool

	// Mini 表示处于迷你模式（400×155 置顶小窗）。
	Mini bool
	// Pinned 表示迷你窗置顶（原版迷你模式里的图钉按钮）。
	Pinned bool

	// 显示偏好（原版 localStorage 的 coverMode / ambientBackgroundEnabled /
	// premiumTexture，落盘在本地存储里）。
	CoverMode string // "disc"（默认，转动的圆盘）或 "square"（静态方块）
	Ambient   bool   // 封面背景（氛围光）开关
	Premium   bool   // 高级质感

	// Modal 是当前显示的模态对话框（nil 表示没有）。
	Modal *Modal

	// toasts 是待弹出的 toast：后台线程通过 Notify 入队，下一帧由 Shell
	// 用 c.AddToast 弹出来（toast 只能在构建帧时添加）。
	toastMu sync.Mutex
	toasts  []ui.Toast
}

// NewApp 建一个用当前时段主题的应用。
func NewApp(repaint func()) *App {
	a := &App{
		Theme:  ThemeAt(time.Now()),
		Speed:  1,
		Volume: 1,
		Images: imagecache.New(repaint),
		// 抽屉表头的默认 tab（与原版一致：搜索按综合、热门与推荐默认热门、
		// 历史默认观看历史）。
		SortOrder: SortTotal,
		RecTab:    RecHot,
		HistTab:   HistHistory,
		UpTab:     UpTabVideos,
		// 显示偏好默认值与原版一致：碟片模式、封面背景开、高级质感开。
		CoverMode: "disc",
		Ambient:   true,
		Premium:   true,
		// 播放列表默认：顺序播放、我的列表页签。
		PlayMode:    PlayModeSequence,
		PlaylistTab: ListUser,
		// 弹幕/评论抽屉默认弹幕 tab、自动跟随。
		DanmakuTab:        TabDanmaku,
		DanmakuAutoScroll: true,
	}
	a.discAt = time.Now()
	return a
}

// Notify 弹一个 toast（等价原版的 toast({ content })，由视图下一帧显示）。
// 可以从任意 goroutine 调用。
func (a *App) Notify(message string) {
	a.notifyToast(ui.Toast{Title: message})
}

// NotifyType 弹一个带类型（error / warning / success）的 toast。
func (a *App) NotifyType(typ, message string) {
	a.notifyToast(ui.Toast{Title: message, Type: typ})
}

func (a *App) notifyToast(t ui.Toast) {
	if t.Title == "" {
		return
	}
	a.toastMu.Lock()
	a.toasts = append(a.toasts, t)
	a.toastMu.Unlock()
	if a.Win != nil {
		a.Win.Invalidate()
	}
}

// drainToasts 在构建帧时把排队的 toast 交给框架显示。
func (a *App) drainToasts(c *ui.Context) {
	a.toastMu.Lock()
	list := a.toasts
	a.toasts = nil
	a.toastMu.Unlock()
	for _, t := range list {
		c.AddToast(t)
	}
}

// ListFor 返回某个抽屉的列表状态，没有就建一个。
// 会在界面线程与网络 goroutine 里同时调用，所以对 map 加锁；返回的 *List 字段
// 仍约定只在 Win.Update 回调和界面线程里改。
func (a *App) ListFor(key string) *List {
	a.listsMu.Lock()
	defer a.listsMu.Unlock()
	if a.Lists == nil {
		a.Lists = map[string]*List{}
	}
	l := a.Lists[key]
	if l == nil {
		l = &List{}
		a.Lists[key] = l
	}
	return l
}

// list 返回当前抽屉的列表状态。
func (a *App) list() *List { return a.ListFor(a.Drawer) }

// SectionIndex 返回分区 key 的下标，找不到时返回 0。
func SectionIndex(key string) int {
	for i, s := range Sections {
		if s.Key == key {
			return i
		}
	}
	return 0
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

// canNavigate 报告上一集/下一集是否有意义（原版 canNavigateNext）：
// 播放列表模式看来源列表长度，否则看当前队列长度。
func (a *App) canNavigate() bool {
	if a.PlayingPlaylist != "" {
		return len(a.playlistActive(a.PlayingPlaylist)) > 1
	}
	return len(a.Queue) > 1
}

// Current 返回正在播放的曲目（可能为 nil）。
func (a *App) Current() *Track {
	if a.Index < 0 || a.Index >= len(a.Queue) {
		return nil
	}
	return &a.Queue[a.Index]
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

	// 搜索栏右侧的四个内容入口（对应原版 home-global-actions 里的
	// ShareSys / ChartRing / WeixinFavorites / History）。
	iconFeed     = icon(`<path d="M13.4 8A5.4 5.4 0 1 1 8 2.6"/><path d="M10.8 8A2.8 2.8 0 1 1 8 5.2"/><circle cx="8" cy="8" r="1"/>`)
	iconPopular  = icon(`<circle cx="8" cy="8" r="6"/><circle cx="8" cy="8" r="2.1"/><path d="M12.6 3.4l1.2-1.2"/>`)
	iconFavorite = icon(`<path d="M8 1.9l5.3 2.9v6.4L8 14.1 2.7 11.2V4.8z"/><path d="M2.7 4.8L8 7.7l5.3-2.9"/><path d="M8 7.7v6.4"/>`)
	iconHistory  = icon(`<path d="M2.7 8a5.3 5.3 0 1 0 1.7-3.9"/><path d="M2.3 2.6v2.7H5"/><path d="M8 5.3V8l2.1 1.3"/>`)
	// 搜索药丸里的放大镜（提交键）。
	iconMagnifier = icon(`<circle cx="7.2" cy="7.2" r="4.3"/><path d="M10.4 10.4L13.8 13.8"/>`)
	// 操作行里的「浏览器打开」与工具条里的「播放列表」。
	iconBrowser   = icon(`<rect x="1.8" y="3" width="12.4" height="10" rx="1.6"/><path d="M1.8 6.2h12.4"/><circle cx="4.3" cy="4.6" r="0.5"/><circle cx="6.3" cy="4.6" r="0.5"/>`)
	iconMusicList = icon(`<path d="M2.5 4h7M2.5 7.5h7M2.5 11h4"/><path d="M12 4.6v6.2"/><circle cx="10.6" cy="11.6" r="1.6"/>`)
	// 迷你模式的两个窗口控制：还原大窗 / 置顶。
	iconRestore = icon(`<rect x="2.2" y="2.2" width="8" height="8" rx="1.4"/><rect x="5.8" y="5.8" width="8" height="8" rx="1.4"/>`)
	iconPin     = icon(`<path d="M6.2 1.8h3.6l-.7 3.6 2.5 2.5H4.4l2.5-2.5z"/><path d="M8 7.9v6.3"/>`)
	// 标题栏的「切换到迷你模式」（原版 #switch-window-mode 用的 ZoomInternal）。
	iconMini = icon(`<rect x="1.8" y="2.6" width="12.4" height="10.8" rx="1.6"/><rect x="8" y="8" width="5" height="4" rx="1"/>`)
	// 抽屉表头的刷新键。
	// 历史抽屉表头的「隐身」开关（原版用 MaskOne）。
	iconMask    = icon(`<path d="M1.8 8h2.6a2 2 0 0 1 0 4H1.8z"/><path d="M14.2 8h-2.6a2 2 0 0 0 0 4h2.6z"/><path d="M6 10.4h4"/>`)
	iconRefresh = icon(`<path d="M13.2 8a5.2 5.2 0 1 1-1.6-3.8"/><path d="M13.6 2.4v2.8h-2.8"/>`)
	// 封面背景（原版用 icon-park 的 Halo）与高级质感（Sparkles）。
	iconHalo     = icon(`<circle cx="8" cy="8" r="3.2"/><path d="M8 1.4v2M8 12.6v2M1.4 8h2M12.6 8h2M3.3 3.3l1.4 1.4M11.3 11.3l1.4 1.4M12.7 3.3l-1.4 1.4M4.7 11.3l-1.4 1.4"/>`)
	iconSparkles = icon(`<path d="M6.4 10.2A1.5 1.5 0 0 0 5.3 9.1L1.6 8a.4.4 0 0 1 0-.7l3.7-1.1A1.5 1.5 0 0 0 6.4 5.1l1.1-3.7a.4.4 0 0 1 .7 0l1.1 3.7a1.5 1.5 0 0 0 1.1 1.1l3.7 1.1a.4.4 0 0 1 0 .7l-3.7 1.1a1.5 1.5 0 0 0-1.1 1.1l-1.1 3.7a.4.4 0 0 1-.7 0z"/><path d="M13 2v2.4M14.2 3.2h-2.4"/>`)
	// 工具条的「合集」（原版用 icon-park 的 Layers）。
	iconSeries = icon(`<path d="M8 1.9l6.2 3.2L8 8.3 1.8 5.1z"/><path d="M2.6 8.2l5.4 2.8 5.4-2.8"/><path d="M2.6 11.2l5.4 2.8 5.4-2.8"/>`)
	// UP 空间抽屉里的关注 / 已关注。
	iconFollow   = icon(`<path d="M8 3.6v8.8M3.6 8h8.8"/>`)
	iconFollowed = icon(`<path d="M3.4 8.4l3 3 6.2-6.8"/>`)

	// 播放列表：定位当前 / 播放模式 / 删除 / 添加 / 已添加 / 上移下移。
	iconLocate  = icon(`<circle cx="8" cy="8" r="3"/><path d="M8 1.4v2.2M8 12.4v2.2M1.4 8h2.2M12.4 8h2.2"/>`)
	iconOrder   = icon(`<path d="M2.5 4h11M2.5 8h7M2.5 12h11"/>`)
	iconLoop    = icon(`<path d="M3 8a5 5 0 0 1 8.5-3.5L13 6"/><path d="M13 2.6V6H9.6"/><path d="M13 8a5 5 0 0 1-8.5 3.5L3 10"/><path d="M3 13.4V10h3.4"/>`)
	iconLoopOne = icon(`<path d="M3 8a5 5 0 0 1 8.5-3.5L13 6"/><path d="M13 2.6V6H9.6"/><path d="M13 8a5 5 0 0 1-8.5 3.5L3 10"/><path d="M3 13.4V10h3.4"/><path d="M7 6.4v3.2"/>`)
	iconShuffle = icon(`<path d="M2.5 4.5h2.2l6.8 7h2"/><path d="M2.5 11.5h2.2l2.3-2.4"/><path d="M9 6.9l2.5-2.4h2"/><path d="M11.4 2.6L13.5 4.5l-2.1 1.9M11.4 9.6l2.1 1.9-2.1 1.9"/>`)
	iconDelete  = icon(`<path d="M3 4.5h10M6.2 4.5V3h3.6v1.5M4.4 4.5l.6 8.2a1 1 0 0 0 1 .9h4a1 1 0 0 0 1-.9l.6-8.2"/>`)
	iconAdd     = icon(`<path d="M8 3.4v9.2M3.4 8h9.2"/>`)
	iconCheck   = icon(`<path d="M3.2 8.4l3.2 3.2 6.4-7"/>`)
	iconUp      = icon(`<path d="M8 12.5V3.5M4.2 7.3L8 3.5l3.8 3.8"/>`)
	iconDown    = icon(`<path d="M8 3.5v9M11.8 8.7L8 12.5 4.2 8.7"/>`)
)

// pick 是三元表达式的泛型版。
func pick[T any](cond bool, a, b T) T {
	if cond {
		return a
	}
	return b
}

// compactCount 把数量转成「1.2万」这样的短文本（原版 formatCompactCount）。
func compactCount(n int64) string {
	switch {
	case n >= 100000000:
		return fmt.Sprintf("%.1f亿", float64(n)/1e8)
	case n >= 10000:
		return fmt.Sprintf("%.1f万", float64(n)/1e4)
	default:
		return fmt.Sprint(n)
	}
}
