package view

import (
	"fmt"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/egoist/mygo"
	"github.com/egoist/mygo/ui"

	"github.com/vst93/bili-fm/app/internal/imagecache"
)

// Card 是列表里的一张卡片要显示的内容。
type Card struct {
	// Bvid 用于和正在播放的曲目比对，判断是不是当前这条。
	Bvid  string
	Cover string
	Title string
	// Badge 是标题前的小胶囊文案（原版 feedList 的 titlePrefix，例如
	// 「充电专属」的红色胶囊）；空则不画。
	Badge    string
	Duration string // 封面右下角角标，空则不画
	// meta 行按旧版 CardMeta 的字段类型分开存，视图按卡片宽度决定显示哪几列
	// （作者 40% / 播放量 30% / 发布时间 30%，窄了从末尾隐藏）。
	Up    string // 作者
	Views string // 播放量
	// ViewsDanmaku 为真表示 Views 展示的其实是弹幕数（播放量缺失时的回退，
	// 原版 viewsMetaField）：meta 行的前导图标要用评论图标而不是播放图标。
	ViewsDanmaku bool
	Pubdate      string // 发布时间（具体日期或短的相对形式）
	Extra        string // 附加（稍后再看的「已看 30%」等）
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
	// FirstFrame 是这一分集的封面首帧（多 P 时优先于视频封面显示）。
	FirstFrame string
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
	// Color 是弹幕颜色（B 站整数 RGB）；0 表示默认色。
	Color int
}

// Segment 是一个跳过分段（SponsorBlock），用于进度条上的广告段标记。
type Segment struct {
	Start, End float64
	// VideoDuration 非 0 时用来校验「这条标记是否属于当前这一版视频」，
	// 与实际总时长差超过 2s 就不画（原版 SPONSOR_DURATION_TOLERANCE_SECONDS）。
	VideoDuration float64
}

// Comment 是一条评论（Replies 是楼中楼预览，原版只取前几条）。
type Comment struct {
	User    string
	Avatar  string
	Content string
	Likes   int64
	Time    string
	// SendTime 是发送时间（unix 秒），用于排序（原版评论 tab 按时间倒序）。
	SendTime int64
	// Level 是 B 站用户等级（0~6，原版评论行显示 Lv.N）。
	Level   int
	Replies []Comment
	// RepliesMore 为真表示楼中楼还有更多没显示（原版「查看更多回复...」）。
	RepliesMore bool
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
	// LoadErr 为真表示上一次「翻页」加载失败。失败后不再每帧自动重试
	//（mygo 的翻页触发在渲染里，而渲染每帧都跑；不加这个闸门就会在滚到底时
	// 每次请求失败都立刻重发，形成请求/提示风暴），而是画一行
	// 「加载失败，点击重试」，对齐原版 historyList 的 loadMoreError。
	LoadErr bool
	// Scroll 记录这个抽屉滚动到哪（同时用来判断「到底了」自动翻页）。
	Scroll ui.ScrollState
	// Grid 是卡片网格（listBody）与选集网格（partsBody）的虚拟化列表状态：
	// mygo 的 ui.List 只构建可见的行，取代了之前「每帧重建整张网格」的做法
	// （见 cardGrid / partsGrid）。同一个 List 只服务一个抽屉 key，两者不会同时用。
	Grid ui.ListState
	// GridScroll 给卡片/选集网格的 ui.List 挂上 ScrollState：滚动偏移以
	// DIP 写在 GridScroll.Y，「滚动时锁住上浮」靠它感知滚动（TrackScroll
	// 只额外暴露偏移，不改变列表行为）。
	GridScroll ui.ScrollState
	// Rows 是播放列表记录行的虚拟化列表状态。播放列表可能很长（用户报
	// 「历史添加的作品左侧图标不显示」），逐行全建会让上百张 192w 缩略图
	// 每帧争抢缓存、互相把对方挤出 LRU（实测 100 行 352 次重复抓取、
	// 可见缩略图只剩 1 张）；改用 ui.List 后只建可见行，缩略图也只预取
	// 可见窗口（同 prefetchCardCovers 的思路）。
	Rows ui.ListState
	// RowsScroll 同 GridScroll，给播放列表的 ui.List 用。
	RowsScroll ui.ScrollState
	// liftHold 是卡片网格（Grid）的「滚动时锁住上浮」状态机：滚动进行中
	// renderCard 不重建 lift 动画 / 悬停光晕，停下后再恢复（见 drawer.go
	// renderCard 与 scrollHold）。
	liftHold scrollHold
}

// ResetGrid 在卡片列表被整批替换（换关键词 / 刷新 / 换抽屉）时复位网格滚动，
// 避免沿用上一批内容的行锚点。追加翻页不动它。
func (l *List) ResetGrid() { l.Grid = ui.ListState{} }

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

// HistCursor 是观看历史的游标式翻页状态（原版 historyList 的 historyCursor
// 对象：{ max, view_at, business }）。第一页取零值（max=0/view_at=0/business=""），
// 之后每页把它换成接口返回的 cursor，就能一直往前翻更早的记录。
// 原版把 business 也带上（archive = 稿件），续页游标里的 business 要与首屏一致。
type HistCursor struct {
	Max      int
	ViewAt   int
	Business string
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
	// Staff 是合作视频的参与 UP 主（只有详情页有可靠字段）；单 UP 视频为空。
	Staff []Collaborator
}

// Collaborator 是合作视频的一位参与 UP 主。
type Collaborator struct {
	Mid   int64
	Name  string
	Title string
	Face  string
}

// 抽屉的 key。Drawer 为空表示没有抽屉打开；分区抽屉直接用分区的 key。
const (
	DrawerSearch  = "search"
	DrawerParts   = "parts"
	DrawerDanmaku = "danmaku"
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
	Kind string
	// Type 是对话框类型（原版 DialogConfig.type），决定标题左侧的 28px 图标
	// 与面板玻璃的强调色：info（默认）/ success / warning / error / question /
	// loading。空与未知值都按 info。
	Type    string
	Title   string
	Message string
	// QR 是登录二维码的位图（Kind == "login"）。
	QR *ui.Bitmap
	// Link 是正文下方的可点链接（文案 + 地址）；空则不画。
	LinkLabel, LinkURL string
	// Buttons 是底部按钮（原版 DialogConfig.buttons）；留空则默认一个主按钮
	// 「确定」。
	Buttons []ModalButton
}

// ModalButton 是对话框底部的一个按钮。Label 是文案，Primary 决定它是不是
// 蓝色渐变的那个主按钮（原版 DialogButton.primary）。
type ModalButton struct {
	Label   string
	Primary bool
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
	OpenBrowser func()
	// CopyLink 复制当前视频链接到剪贴板。
	CopyLink      func()
	Play          func(index int)
	TogglePlay    func()
	Next          func()
	Prev          func()
	Seek          func(seconds float64)
	SetSpeed      func(v float64)
	ToggleEQ      func()
	ToggleSponsor func()
	SetVolume     func(v float64)
	// PersistVolume 只把音量落盘（原版注释：音量偏好**仅在拖动提交/静音
	// 切换时写**，不跟着每次 input 事件写 —— 拖一次要写几十次盘）。
	PersistVolume func(v float64)
	ToggleMute    func()
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
}

// ImageStore 是 App 取封面图 / 氛围光图的方式。*imagecache.Cache 实现它；
// 测试可注入计数 / 返回假图的实现，验证「屏外不请求」等行为。
//
// width 是服务端下采样的目标宽度，按调用点的展示尺寸传（见 imagecache.Width*）：
// 列表卡片 240、播放器封面 480、氛围光 320、播放列表缩略图 192、头像 96 ——
// 对齐 Tauri 原版 graftingImage 各调用点的显式宽度，不再一律 240。
type ImageStore interface {
	// Bitmap 取封面：命中返回位图，未命中返回 nil 并异步抓取。
	Bitmap(url string, width int) *ui.Bitmap
	// Get 只读缓存、不发起抓取（懒加载的构建路径用它）。
	Get(url string, width int) *ui.Bitmap
	// Touch 把一张图标记为「本轮可见」：fetch 队列优先服务它
	//（原版浏览器 loading=lazy 的可见优先语义；实作用不到时是空操作）。
	Touch(url string, width int)
	// Retain 把一张图标记为「常驻」：缓存淘汰永不丢它。头像这类一直挂在
	// 界面上的小图每帧重申一次，免得被列表封面的一轮抓取挤出 LRU（实作用
	// 不到时是空操作）。
	Retain(url string, width int)
	// Cancel 取消一张图的抓取请求（滚出预取窗口时）：还在排队的退队
	// 回滚并发名额，已拿到名额在途的照常完成（原版浏览器对滚出的
	// loading=lazy 图取消加载；实作用不到时是空操作）。
	Cancel(url string, width int)
	// Ambient 取氛围光用的调色小图。
	Ambient(url string, width int, blur bool) *ui.Bitmap
	// Pending 返回还在抓取的张数（占位状态 / 测试用）。
	Pending() int
}

// App 是主窗口的全部状态。
type App struct {
	Win    *mygo.Window
	Theme  Theme
	Images ImageStore
	Act    Actions

	// 导航
	Section int
	// Lists 按抽屉 key（分区 key 或 DrawerSearch）存各自的列表。
	Lists map[string]*List
	// listsMu 保护 Lists 这个 map：网络 goroutine 与界面线程都会调 ListFor，
	// 而 Go 的 map 并发读写会直接 panic。
	listsMu sync.Mutex

	// scrollHoldHeld 是「滚动时锁住选中/悬停上浮」的开关：drawerBody 每帧
	// 根据含卡容器的滚动状态更新，renderCard 据此跳过 lift 动画重建。
	scrollHoldHeld bool
	// partsHold / playlistHold 是选集网格与播放列表的滚动锁状态机
	//（它们共用 List.Grid / List.Rows 滚动，但 key 不同，状态机分开记）。
	partsHold    scrollHold
	playlistHold scrollHold
	// liftChannels 统计本帧有多少张卡把 lift 交给了 AnimateTo：无头测量
	//「滚动时锁住上浮」是否真的让动画重算清零（见 liftscroll_hold_test.go）。
	liftChannels atomic.Int64
	Status       string
	LoggedIn     bool
	UName        string
	Face         string // 登录用户头像 URL，未登录为空

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
	// UpFollowPending 为真表示「关注状态还没查回来」或「关注/取关请求在途」：
	// UP 空间表头的关注键转圈并禁用（原版 upVideoList 的 isCheckingFollow /
	// isFollowingLoading）。没有它时连点关注键会先关注再取关（每个 click 各起
	// 一个请求），用户看到的是一闪而过的错误状态。
	UpFollowPending bool
	UpFans          int64  // 粉丝数，UP 空间表头显示
	UpTab           string // UpTabVideos / UpTabSeries
	SeriesList      []Series
	SeriesID        int64
	SeriesName      string
	// SeriesPlayAllPending 为真表示「播放全部」正在把整个合集拉进播放列表
	//（原版 seriesList 的 isPlayingAll：按钮在请求期间转圈 + 禁用）。
	SeriesPlayAllPending bool

	// UpOffset 是 UP 视频列表的翻页游标（旧版是 offset 而不是页码）。
	UpOffset string
	// FeedOffset 是「动态」列表的翻页游标（同样是 offset）。
	FeedOffset string
	// HistCursor 是「观看历史」的游标式翻页状态（原版 historyList 的
	// historyCursor：max / view_at / business）。观看历史接口是游标分页而不是
	// 页码：只有第一页给 max=0/view_at=0，之后每页都要带上上一页返回的游标。
	// 此前原生版没有它，fetchSection 永远只拉第一页且 HasMore 恒为 false，
	// 用户滚到底也再加载不出更早的记录。
	HistCursor HistCursor

	// 播放
	Queue   []Track
	Index   int
	Track   *Track
	Playing bool
	Pos     float64
	Dur     float64
	// Buffered 是已预读到本地的媒体秒数（进度条上的缓冲段，原版
	// audio.buffered；0 表示未知/未开始）。
	Buffered float64
	Speed    float64
	EQ       bool
	Sponsor  bool
	Volume   float64
	// LastVolume 是「静音前」记住的音量。原版没有独立的静音布尔：静音就是
	// 把音量归零（player.tsx 的 handleVolumeChange 里 `audio.muted = value === 0`），
	// lastVolumeRef 记住最后一次 >0 的音量，解除静音时恢复。这里用同一个事实
	// （Volume==0 即静音），LastVolume 只负责恢复值。
	LastVolume float64
	// Muted 是静音状态。它**恒等于 Volume <= 0**（见 setVolume / toggleMute），
	// 保留这个字段只是让视图与视频弹窗少算一次比较。
	Muted bool

	// SponsorStatus 是 SponsorBlock 查询状态：off/loading/ok/empty/error，
	// 用于播放栏按钮上的状态点（原版解 SponsorStatusInfo）。
	SponsorStatus string
	// SponsorCount 是本次查询拿到（或确认没有）的分段数；ok 时进状态点
	// 的 tooltip（原版 sponsorStatus.count）。
	SponsorCount int
	// SponsorReason 是查询失败的原因，进状态点的 tooltip
	//（原版 sponsorStatus.reason，error 时显示）。
	SponsorReason string
	// SponsorSegments 是当前曲目的跳过分段（进度条上的广告段标记）。
	SponsorSegments []Segment
	// Buffering 表示正在起播（网络 + 解码初始化）。
	Buffering bool

	// 分集 / 弹幕 / 评论 / 详情
	//
	// Info 是**正在播放**的视频详情（原版 displayVideoInfo）：起播时写入，
	// 主区右栏（标题 / 简介 / UP 主 / 封面 / 统计 / 互动键 / 合作选择器）
	// 只读它。
	Info *Info
	// BrowseInfo 是**浏览中**的视频详情（原版 videoInfo）：列表点卡片 /
	// 粘链接（urlJump）与打开选集面板（openParts）时写入，选集抽屉与
	// 「选集」相关动作读它（见 Browse）。
	//
	// 原版把两者分开是为了「点了列表但还没播放时首页信息不变」：
	// displayVideoInfo 只跟 playingInfo。原生版此前只有一个 a.Info，被
	// urlJump 覆盖成浏览的那条，于是浏览另一条视频会把主区显示也改掉。
	BrowseInfo *Info
	// CollabMid 是合作视频里用户选中的参与 UP 主（0 = 未选，显示主 UP）。
	// CollabFor 记它属于哪条视频（bvid）：换视频后选择自动失效 —— 对应原版
	// videoInfo 在 bvid 变化时 `setSelectedOwner(null)`。
	CollabMid int64
	CollabFor string
	// CollabOpen 是合作 UP 主选择器面板是否展开（原版 videoInfo 的
	// isOwnerMenuOpen）。展开时触发键按原版 [aria-expanded] 转蓝底/蓝图标。
	// CollabOpenFor 记它是在哪条视频（bvid）上打开的：换视频后展开态自动失效
	// —— 对应原版 videoInfo 的 useEffect 在 bvid 变化时 setIsOwnerMenuOpen(false)
	// （mygo 没有 bvid 变化钩子，靠比对 bvid 等价实现）。
	CollabOpen    bool
	CollabOpenFor string
	Danmaku       []Danmaku
	Comments      []Comment
	// DanmakuCid / ReplyOid 记录当前 Danmaku / Comments 属于哪条视频
	// （原版 danmakuCid / replyOid）。打开弹幕抽屉或切 tab 时用它判断
	// 「已经是这条视频的数据了吗」，不是就重拉；没有它就只能靠
	// len(...)==0 判断，换一条视频后会继续显示上一条的弹幕 / 评论。
	DanmakuCid int64
	ReplyOid   int64

	// 弹幕/评论抽屉：DanmakuTab 是当前 tab；DanmakuAutoScroll 是自动跟随
	// 当前播放时间；Reply* 是评论分页状态。
	//
	// DanmakuLoading 是弹幕列表的拉取在途标志（原版 isLoadingDanmaku）。
	// 起播预取弹幕与抽屉里点刷新都置它；表头刷新键据此画 Spinner + 禁用。
	DanmakuTab        string
	DanmakuAutoScroll bool
	DanmakuLoading    bool
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
	// PlaybackRequestID 是起播请求的代号：慢的旧响应不得覆盖新的起播
	// （原版 playbackRequestIdRef）。
	PlaybackRequestID int

	// SearchRequestID 是搜索请求的代号：慢的旧响应不得覆盖新的搜索结果
	// （原版 searchRequestIdRef）。
	SearchRequestID int

	// InteractionReqID 是互动状态（点赞/投币/收藏/关注）请求的代号：
	// 切歌后旧响应不得覆盖新视频的状态，提交收藏也不得把上一条的状态写过来
	// （原版 favoriteRequestIdRef）。
	InteractionReqID int

	// CollectScroll 是收藏夹表头横向滚动的位置（左右键拨动它）。
	CollectScroll ui.ScrollState

	// locateNow 在点了「定位到当前」后置位，下一帧把当前行滚进视野。
	locateNow bool
	// PartSearch 是选集搜索框的输入（原版 .part-search-input）。
	PartSearch string
	// PartSearchCursor 是选集搜索的游标（连续回车跳下一个）。
	PartSearchCursor int
	// locatePartIndex 是选集搜索要跳到的分集下标（-1 表示无）。
	locatePartIndex int
	// WLRemovePending 是「从稍后再看移除」请求中的 aid，按钮置灰防连点
	// （原版 historyList 的 pendingAids）。
	WLRemovePending map[int64]bool

	// Drawer 是当前打开的抽屉："" 表示没有，否则是分区 key 或
	// DrawerSearch / DrawerParts / DrawerDanmaku。
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
	// FavPending 是收藏状态「正在查 / 正在提交」：原版 isFavoriteUpdating，
	// 期间收藏键禁用（避免用上一条视频的状态误操作 / 连点重复请求）。
	FavPending bool

	// 视频弹窗
	VideoOpen bool

	// 搜索
	Query string

	// SeekValue 是进度条滑块绑定的值：按住时由框架写（sliderInput），
	// 无人操作时由 view 写播放位置。交接规则见 playerbar.progress。
	SeekValue float64
	// SeekBusy 表示一次 seek 还在跑（重定位+重建解码器）；期间的
	// 新 seek 目标只更新 PendingSeek，等这次跑完再补一次——
	// 连续拖动不会堆叠多个重建。
	SeekBusy bool
	// wasVolHeld 是上一帧指针是否按在音量滑杆上（识别松手的下降沿，
	// 用来决定什么时候落盘）。
	wasVolHeld bool
	// wasHeld 是上一帧指针是否按在进度条上（识别松手的下降沿）。
	// 用 Dragged() 的 ok 而不是 Pressed()：Pressed() 在指针移出细轨道
	// 时就是 false，拖动中会被误判成松手。
	wasHeld bool
	// SeekStableAt 是最近一次「滑块交接点」的时间（按下/松手/seek 完成）：
	// 此后一小段时间不跟随播放位置，等引擎追上。
	SeekStableAt time.Time
	// seekBubbleLabel / seekBubbleLeft 是悬停时间气泡（原版
	// .player-seek-bubble）最后一次悬停时定下的文案与左边界。原版只在
	// handleTimelineHover 里写 textContent / style.left，移开指针后仅翻
	// data-visible，气泡停在原处淡出；缓存这两个值就是为了复刻这一点。
	seekBubbleLabel string
	seekBubbleLeft  float32
	// PendingSeek/HasPendingSeek 是「等待执行的跳转目标」：
	// 起播（Buffering）期间、或上一次 seek 还在跑时，用户的请求
	// 记在这里，等条件允许后执行一次（排队而不是丢弃）——
	// 对应原版 safeSeek 的 readyState 保护。
	PendingSeek    float64
	HasPendingSeek bool
	// ShowSpeed / ShowVolume 表示播放栏的倍速、音量弹层展开着。
	ShowSpeed  bool
	ShowVolume bool
	// ShowSettings 表示标题栏的「设置」下拉（原版 titleBar.tsx 的 showMenu）
	// 展开着。
	ShowSettings bool

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
	// discAngle / discLast / discLastPos 是唱片的旋转角度、上次推进时间，
	// 以及那时的播放位置（用来识别 seek 造成的跳变，见 advanceDisc）。
	discAngle   float32
	discLast    time.Time
	discLastPos float64
	// drawerOpenKey 记住当前抽屉，drawerEnterAt 是入场起始帧的时间戳：
	// 同一帧里框架可能把视图重建多趟（点击的 build pass），只有最后一趟会
	// 被采样；用时间戳把入场钉在「这一刻」，避免第二趟把起始 pose 覆盖成 0
	// （那样入场动画就没了，见 drawerEnterStep）。
	drawerOpenKey string
	drawerEnterAt time.Time
	// dmGroups / cmSorted 缓存弹幕分组与评论排序：滚动时视图每帧重建，
	// 每帧重算这些分组 / 排序就是卡顿的来源（见 danmakuGroups）。
	// *Src 持有缓存依据的那份切片，既是身份判据，也防止旧数组被 GC 回收后
	// 新切片复用同一地址造成误命中。
	dmGroups    []danmakuGroup
	dmGroupsSrc []Danmaku
	cmSorted    []Comment
	cmSortedSrc []Comment
	// drawerShown 是当前（或正退场的）抽屉的 key；drawerClosing / drawerExitAt
	// 记退场是否进行中、何时起步。关抽屉时 a.Drawer 先被清空，但面板要继续
	// 下滑淡出（见 drawer），所以得把最后一个 key 留住。
	drawerShown   string
	drawerClosing bool
	drawerExitAt  time.Time
	// drawerOffset 是抽屉被拖拽的下滑偏移（v0.4.5 的 ui.Draggable）：
	// 0 = 贴底完全展开，抽屉高度 = 甩到底、关闭。
	drawerOffset ui.Point
	// modalOpenedAt 同理，给模态框的缩放入场用。
	modalOpenedAt time.Time

	// bloomBmp 是 #root::before/::after 光场的烘焙位图（v0.4.7 的 ui.Canvas：
	// 画一次、每帧当图显示）。bloomSize / bloomPeriod 记住烘焙时的画布像素
	// 尺寸与时段，任一变了就重烘（几何按窗口比例算、光罩按时段取色）。
	bloomBmp    *ui.Bitmap
	bloomSize   [2]int
	bloomPeriod string

	// miniBgBmp 是迷你窗 #root 背景（145° 线性 + 两条径向光晕）的烘焙位图；
	// miniBgSize 记住烘焙时的画布尺寸，变了才重烘（见 minibg.go）。
	miniBgBmp  *ui.Bitmap
	miniBgSize [2]int

	// toasts 是待弹出的 toast：后台线程通过 Notify 入队，下一帧由 Shell
	// 用 c.AddToast 弹出来（toast 只能在构建帧时添加）。
	toastMu sync.Mutex
	toasts  []ui.Toast

	// danmakuList 是弹幕列表的虚拟化状态（mygo 的 ui.List 只构建可见行）；
	// danmakuCid 记录上次构建用的是哪条弹幕数据（DanmakuCid），换了就重置它。
	danmakuList ui.ListState
	danmakuCid  int64

	// repliesList 是评论列表的虚拟化状态（同 danmakuList）；repliesCid 记录上次
	// 构建用的是哪条评论数据（ReplyOid），换了就重置。评论列表也带 ui.List 的行键，
	// 滑动窗口从头部释放已读那批时
	// （见 main.go 的 retainComments）锚点行跟着挪下标，视口停在原评论上不跳。
	repliesList ui.ListState
	repliesCid  int64

	// grids 是每个抽屉的卡片网格状态（ui.GridView 要一个稳定的 GridState，
	// 按抽屉分开存才能各自保留滚动位置）。
	gridsMu sync.Mutex
	grids   map[string]*ui.GridState
}

// NewApp 建一个用当前时段主题的应用。
func NewApp(repaint func()) *App {
	a := &App{
		Theme:      ThemeAt(time.Now()),
		Speed:      1,
		Volume:     1,
		LastVolume: 1,
		Images:     imagecache.New(repaint),
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
	// 原版 utils/toast.ts 的默认时长是 2s，而框架默认 4s —— 不显式覆盖
	// 的话 toast 会比原版多挂一倍时间。调用点给了时长就用调用点的。
	if t.Timeout == 0 {
		t.Timeout = toastDuration
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

// CanNavigate 报告上一首/下一首是否有意义（供系统媒体中心推送 CanGoNext）。
func (a *App) CanNavigate() bool { return a.canNavigate() }

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

// iconPark 解析一枚 icon-park 的**原始**描边图标（@icon-park/react 的 SVG 逐字拷贝）。
// IconPark 的画布是 48×48、描边宽 4、圆头圆接 —— 缩到 size 18 / 20 / 15 时
// 描边换算成 1.5 / 1.67 / 1.25px。
//
// 为什么不能继续用 icon() 的 16 画布手抄：原版那些图标都来自 icon-park
// （theme="outline"），圆角、端点、弧线走向都是 48 画布上定死的；在 16 画布上
// 凭观感重画出来的近似（比如旧 iconMagnifier 的圆 + 一段斜线）在放大后线条
// 偏细、放大镜的高光弧线整个缺掉，用户一眼能看出「和以前的不一样」。
// 这里直接抄原 path，再让渲染器等比缩放 —— 与原版逐点一致。
func iconPark(inner string) *ui.SVG {
	return ui.MustParseSVG([]byte(`<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 48 48" fill="none" stroke="currentColor" stroke-width="4" stroke-linecap="round" stroke-linejoin="round">` + inner + `</svg>`))
}

// park* 是 @icon-park/react 1.4.2 的原始 path 数据，逐字拷贝（未被内联成 16 画布
// 近似）。测试 parkicon_test.go 逐字钉住，防止再被手改。
const (
	// Search（icons/Search.js）。
	parkSearch = `<path d="M21 38C30.3888 38 38 30.3888 38 21C38 11.6112 30.3888 4 21 4C11.6112 4 4 11.6112 4 21C4 30.3888 11.6112 38 21 38Z"/>` +
		`<path d="M26.657 14.3431C25.2093 12.8954 23.2093 12 21.0001 12C18.791 12 16.791 12.8954 15.3433 14.3431"/>` +
		`<path d="M33.2216 33.2217L41.7069 41.707"/>`
	// ShareSys（动态）。
	parkShareSys = `<path d="M10.3654 41.6322C6.44902 37.9812 4 32.7766 4 27C4 15.9543 12.9543 7 24 7C35.0457 7 44 15.9543 44 27C44 32.7766 41.551 37.9812 37.6346 41.6322"/>` +
		`<path d="M15.1375 36.5109C12.5919 34.1378 11 30.7548 11 27C11 19.8203 16.8203 14 24 14C31.1797 14 37 19.8203 37 27C37 30.7548 35.4081 34.1378 32.8625 36.5109"/>` +
		`<path d="M19.9096 31.3897C18.7347 30.2944 18 28.733 18 27C18 23.6863 20.6863 21 24 21C27.3137 21 30 23.6863 30 27C30 28.733 29.2653 30.2944 28.0904 31.3897"/>`
	// ChartRing（热门与推荐）。
	parkChartRing = `<path d="M43.7756 20.9938C42.4735 12.3555 35.6463 5.5277 27.0084 4.22461M20.9757 4.22702C11.3651 5.68478 4 13.9822 4 23.9998C4 34.0212 11.3705 42.321 20.9863 43.7743C21.9692 43.9228 22.9756 43.9998 24 43.9998C25.0209 43.9998 26.024 43.9233 27.0038 43.7758C35.6458 42.4741 42.4762 35.6427 43.7764 27.0003"/>` +
		`<path d="M24 16C19.5817 16 16 19.5817 16 24C16 28.4183 19.5817 32 24 32C28.4183 32 32 28.4183 32 24C32 19.5817 28.4183 16 24 16Z"/>`
	// WeixinFavorites（收藏）。
	parkWeixinFavorites = `<path d="M6 14L24 6L42 14M6 14L24 22M6 14V34L24 42M42 14L24 22M42 14V34L24 42M24 22V42"/>`
	// History（历史）。
	parkHistory = `<path d="M5.81836 6.72729V14H13.0911"/>` +
		`<path d="M4 24C4 35.0457 12.9543 44 24 44V44C35.0457 44 44 35.0457 44 24C44 12.9543 35.0457 4 24 4C16.598 4 10.1351 8.02111 6.67677 13.9981"/>` +
		`<path d="M24.005 12L24.0038 24.0088L32.4832 32.4882"/>`
	// PlayOne（播放）/ Pause（暂停）/ GoEnd（下一集）/ GoStart（上一集）。
	// 原版 player.tsx 的 .player-play-button 用 `PlayOne theme="filled"` /
	// `Pause theme="filled"`，.player-next-button 用 GoEnd outline。PlayOne 的
	// fill 在 filled 主题下就是 currentColor（单色 Icon 画得了），所以 path 上
	// 明写 fill="currentColor"。
	parkPlayOne = `<path fill="currentColor" d="M15 24V11.8756L25.5 17.9378L36 24L25.5 30.0622L15 36.1244V24Z"/>`
	parkPause   = `<path d="M16 12V36"/>` +
		`<path d="M32 12V36"/>`
	parkGoEnd = `<path d="M14 12L26 24L14 36"/>` +
		`<path d="M34 12V36"/>`
	parkGoStart = `<path d="M34 36L22 24L34 12"/>` +
		`<path d="M14 12V36"/>`
	// VolumeNotice / VolumeMute（音量键）。VolumeMute 原文件用一层 mask 把斜杠
	// 裁进 13×13 的框里，但斜杠本来就整条落在框内，遮罩不改变可见像素；单色
	// 渲染下把那层 mask 去掉，只留相同的三条描边。
	parkVolumeNotice = `<path d="M24 6V42C17 42 11.7985 32.8391 11.7985 32.8391H6C4.89543 32.8391 4 31.9437 4 30.8391V17.0108C4 15.9062 4.89543 15.0108 6 15.0108H11.7985C11.7985 15.0108 17 6 24 6Z"/>` +
		`<path d="M32 15L32 15C32.6232 15.5565 33.1881 16.1797 33.6841 16.8588C35.1387 18.8504 36 21.3223 36 24C36 26.6545 35.1535 29.1067 33.7218 31.0893C33.2168 31.7885 32.6391 32.4293 32 33"/>` +
		`<path d="M34.2359 41.1857C40.0836 37.6953 44 31.305 44 24C44 16.8085 40.2043 10.5035 34.507 6.97906"/>`
	parkVolumeMute = `<path d="M40.7348 20.2858L32.2495 28.7711"/>` +
		`<path d="M32.2496 20.2858L40.7349 28.7711"/>` +
		`<path d="M24 6V42C17 42 11.7985 32.8391 11.7985 32.8391H6C4.89543 32.8391 4 31.9437 4 30.8391V17.0108C4 15.9062 4.89543 15.0108 6 15.0108H11.7985C11.7985 15.0108 17 6 24 6Z"/>`
	// Equalizer（均衡键）：三根竖线 + 三个空心旋钮。
	parkEqualizer = `<path d="M11 16V42"/>` +
		`<path d="M24 29V42"/>` +
		`<path d="M24 19V6"/>` +
		`<path d="M37 6V32"/>` +
		`<path d="M11 16C13.7614 16 16 13.7614 16 11C16 8.23858 13.7614 6 11 6C8.23858 6 6 8.23858 6 11C6 13.7614 8.23858 16 11 16Z"/>` +
		`<path d="M24 29C26.7614 29 29 26.7614 29 24C29 21.2386 26.7614 19 24 19C21.2386 19 19 21.2386 19 24C19 26.7614 21.2386 29 24 29Z"/>` +
		`<path d="M37 42C39.7614 42 42 39.7614 42 37C42 34.2386 39.7614 32 37 32C34.2386 32 32 34.2386 32 37C32 39.7614 34.2386 42 37 42Z"/>`
	// Ad（恰饭跳过键）：圆圈 + 「广」字 + 右侧「）」块 + 两角小斜线。
	parkAd = `<circle cx="24" cy="24" r="20"/>` +
		`<path d="M38 38L35 35"/>` +
		`<path d="M10 10L13 13"/>` +
		`<path d="M14 31L14.8571 28M22 31L21.1429 28M21.1429 28L18 17L14.8571 28M21.1429 28H14.8571"/>` +
		`<path d="M35 24C35 29 31.4183 31 27 31V17C31.4183 17 35 19 35 24Z"/>`
	// PreviewOpen / Comment（列表卡片 meta 的前导图标：播放量 / 弹幕数）。
	parkPreviewOpen = `<path d="M24 36C35.0457 36 44 24 44 24C44 24 35.0457 12 24 12C12.9543 12 4 24 4 24C4 24 12.9543 36 24 36Z"/>` +
		`<path d="M24 29C26.7614 29 29 26.7614 29 24C29 21.2386 26.7614 19 24 19C21.2386 19 19 21.2386 19 24C19 26.7614 21.2386 29 24 29Z"/>`
	parkComment = `<path d="M44 6H4V36H13V41L23 36H44V6Z"/>` +
		`<path d="M14 19.5V22.5"/>` +
		`<path d="M24 19.5V22.5"/>` +
		`<path d="M34 19.5V22.5"/>`
	// 主区操作行 / 工具条的图标（原版 videoInfo.tsx）。互动三键是 ThumbsUp /
	// HandleB / Star，其余是 Browser / VideoTwo / DoubleUp / Layers / MusicList。
	// Star 的 filled 态（已收藏）就是同一条 path 加上 fill="currentColor"。
	parkThumbsUp = `<path d="M27.6002 18.5998V11.3998C27.6002 8.41743 25.1826 5.99977 22.2002 5.99977L15.0002 22.1998V41.9998H35.9162C37.7113 42.0201 39.2471 40.7147 39.5162 38.9398L42.0002 22.7398C42.1587 21.6955 41.8506 20.6343 41.1576 19.8373C40.4645 19.0403 39.4564 18.5878 38.4002 18.5998H27.6002Z"/>` +
		`<path d="M15 22.0001H10.194C8.08532 21.9628 6.2827 23.7095 6 25.7994V38.3994C6.2827 40.4894 8.08532 42.0367 10.194 41.9994H15V22.0001Z"/>`
	parkHandleB = `<circle cx="24" cy="24" r="20"/>` +
		`<path d="M24.7918 23C27.6682 23 30 20.7614 30 18C30 15.2386 27.6682 13 24.7918 13H18V23H24.7918Z"/>` +
		`<path d="M26.831 35C29.6857 35 32 32.3137 32 29C32 25.6863 29.6857 23 26.831 23H18V35H26.831Z"/>`
	parkStar       = `<path d="M23.9986 5L17.8856 17.4776L4 19.4911L14.0589 29.3251L11.6544 43L23.9986 36.4192L36.3454 43L33.9586 29.3251L44 19.4911L30.1913 17.4776L23.9986 5Z"/>`
	parkStarFilled = `<path fill="currentColor" d="M23.9986 5L17.8856 17.4776L4 19.4911L14.0589 29.3251L11.6544 43L23.9986 36.4192L36.3454 43L33.9586 29.3251L44 19.4911L30.1913 17.4776L23.9986 5Z"/>`
	parkBrowser    = `<path d="M42 18V40C42 41.1046 41.1046 42 40 42H8C6.89543 42 6 41.1046 6 40V18"/>` +
		`<path d="M6 8C6 6.89543 6.89543 6 8 6H40C41.1046 6 42 6.89543 42 8V18H6V8Z"/>` +
		`<path fill="currentColor" d="M12 14C13.1046 14 14 13.1046 14 12C14 10.8954 13.1046 10 12 10C10.8954 10 10 10.8954 10 12C10 13.1046 10.8954 14 12 14Z"/>` +
		`<path fill="currentColor" d="M18 14C19.1046 14 20 13.1046 20 12C20 10.8954 19.1046 10 18 10C16.8954 10 16 10.8954 16 12C16 13.1046 16.8954 14 18 14Z"/>` +
		`<path fill="currentColor" d="M24 14C25.1046 14 26 13.1046 26 12C26 10.8954 25.1046 10 24 10C22.8954 10 22 10.8954 22 12C22 13.1046 22.8954 14 24 14Z"/>`
	parkVideoTwo = `<path d="M39 6H9C7.34315 6 6 7.34315 6 9V39C6 40.6569 7.34315 42 9 42H39C40.6569 42 42 40.6569 42 39V9C42 7.34315 40.6569 6 39 6Z"/>` +
		`<path d="M20.5 28V21.9378L25.75 24.9689L31 28L25.75 31.0311L20.5 34.0622V28Z"/>` +
		`<path d="M6 15H42"/>` +
		`<path d="M33 6L27 15"/>` +
		`<path d="M21 6L15 15"/>`
	parkDoubleUp = `<path d="M12 24L24 12L36 24"/>` +
		`<path d="M12 36L24 24L36 36"/>`
	parkLayers = `<path d="M4 11.9143L24 19L44 11.9143L24 5L4 11.9143Z"/>` +
		`<path d="M4 20L24 27L44 20"/>` +
		`<path d="M4 28L24 35L44 28"/>` +
		`<path d="M4 36L24 43L44 36"/>`
	parkMusicList = `<path d="M24 19H40"/>` +
		`<path d="M24 10H40"/>` +
		`<path d="M8 38H40"/>` +
		`<path d="M8 28H40"/>` +
		`<path d="M8 10L16 15L8 20V10Z"/>`

	// 下面这批是标题栏 / 抽屉表头 / 播放列表 / 模态 / toast / 迷你窗 / 封面开关
	// 等处的图标，原版同样来自 @icon-park/react 1.4.2（48×48 画布、描边 4），
	// 逐字拷贝。此前它们几乎都是 16 画布手抄近似：线条粗细、端点、圆角、比例
	// 都与原版有出入（比如刷新键的箭头、叉号的端点、垃圾桶的桶盖、田字格定位框
	// 的四个角、人形图标的人数与比例等），放大到 18/20px 后一眼看得出。
	//
	// 这一批里没有该 icon-park 组件里会用到的非 currentColor 填充色：描边主题下
	// colors[1] 是 none、colors[2] 与 colors[0] 同为 currentColor，所以单色
	// Icon 渲染时只保留描边形状、丢掉 colors[1] 的填充形状（与原版一致）。
	// 唯一的例外是 Play（合集「播放全部」/ 播放列表当前行）——原版是
	// theme="filled"，圆盘填 666/蓝、三角挖白，单色 Icon 画不出，用逐色 Image
	// 补（见 playFilledIcon）。
	parkLeft         = `<path d="M31 36L19 24L31 12"/>`
	parkRight        = `<path d="M19 12L31 24L19 36"/>`
	parkClose        = `<path d="M8 8L40 40"/><path d="M8 40L40 8"/>`
	parkCloseSmall   = `<path d="M14 14L34 34"/><path d="M14 34L34 14"/>`
	parkCloseOne     = `<path d="M24 44C35.0457 44 44 35.0457 44 24C44 12.9543 35.0457 4 24 4C12.9543 4 4 12.9543 4 24C4 35.0457 12.9543 44 24 44Z"/><path d="M29.6567 18.3432L18.343 29.6569"/><path d="M18.3433 18.3432L29.657 29.6569"/>`
	parkCheck        = `<path d="M43 11L16.875 37L5 25.1818"/>`
	parkCheckOne     = `<path d="M24 44C29.5228 44 34.5228 41.7614 38.1421 38.1421C41.7614 34.5228 44 29.5228 44 24C44 18.4772 41.7614 13.4772 38.1421 9.85786C34.5228 6.23858 29.5228 4 24 4C18.4772 4 13.4772 6.23858 9.85786 9.85786C6.23858 13.4772 4 18.4772 4 24C4 29.5228 6.23858 34.5228 9.85786 38.1421C13.4772 41.7614 18.4772 44 24 44Z"/><path d="M16 24L22 30L34 18"/>`
	parkRefresh      = `<path d="M42 8V24"/><path d="M6 24L6 40"/><path d="M42 24C42 14.0589 33.9411 6 24 6C18.9145 6 14.3216 8.10896 11.0481 11.5M6 24C6 33.9411 14.0589 42 24 42C28.8556 42 33.2622 40.0774 36.5 36.9519"/>`
	parkDelete       = `<path d="M9 10V44H39V10H9Z"/><path d="M20 20V33"/><path d="M28 20V33"/><path d="M4 10H44"/><path d="M16 10L19.289 4H28.7771L32 10H16Z"/>`
	parkOrder        = `<path d="M33.0499 7H38C39.1046 7 40 7.89543 40 9V42C40 43.1046 39.1046 44 38 44H10C8.89543 44 8 43.1046 8 42L8 9C8 7.89543 8.89543 7 10 7H16H17V10H31V7H33.0499Z"/><rect x="17" y="4" width="14" height="6"/><path d="M26.9996 19L19 27.0012H29.004L21.0003 35.0018"/>`
	parkLoopOnce     = `<path d="M43.8233 25.2305C43.7019 25.9889 43.5195 26.727 43.2814 27.4395C42.763 28.9914 41.9801 30.4222 40.9863 31.6785C38.4222 34.9201 34.454 37 30 37H16C9.39697 37 4 31.6785 4 25C4 18.3502 9.39624 13 16 13H44"/><path d="M38 7L44 13L38 19"/>`
	parkShuffleOne   = `<path d="M40 33L44 37L40 41"/><path d="M40 7L44 11L40 15"/><path d="M44 11H37C29.8203 11 24 16.8203 24 24C24 31.1797 29.8203 37 37 37H44"/><path d="M4 37H11C18.1797 37 24 31.1797 24 24C24 16.8203 18.1797 11 11 11H4"/>`
	parkFocusOne     = `<path d="M16 6H8C6.89543 6 6 6.89543 6 8V16"/><path d="M16 42H8C6.89543 42 6 41.1046 6 40V32"/><path d="M32 42H40C41.1046 42 42 41.1046 42 40V32"/><path d="M32 6H40C41.1046 6 42 6.89543 42 8V16"/><path d="M24 31C27.866 31 31 27.866 31 24C31 20.134 27.866 17 24 17C20.134 17 17 20.134 17 24C17 27.866 20.134 31 24 31Z"/><path d="M24 17L24 13"/><path d="M24 35L24 31"/><path d="M35 24H31"/><path d="M17 24H13"/><path fill="currentColor" stroke="none" d="M24 26C25.1046 26 26 25.1046 26 24C26 22.8954 25.1046 22 24 22C22.8954 22 22 22.8954 22 24C22 25.1046 22.8954 26 24 26Z"/>`
	parkConnection   = `<path d="M39 34L44 39L39 44"/><path d="M8 12C10.2091 12 12 10.2091 12 8C12 5.79086 10.2091 4 8 4C5.79086 4 4 5.79086 4 8C4 10.2091 5.79086 12 8 12Z"/><path d="M12 8L20 8C22.2091 8 24 9.79086 24 12V35C24 37.2091 25.7909 39 28 39H44"/>`
	parkText         = `<rect x="6" y="6" width="36" height="36" rx="3"/><path d="M16 19V16H32V19"/><path d="M22 34H26"/><path d="M24 18L24 34"/>`
	parkMaskOne      = `<path d="M7.36573 16.3443C6.48571 18.7024 6 21.2881 6 23.9998C6 26.7115 6.48571 29.2971 7.36573 31.6552M11.2721 38.1419C12.9301 39.9841 14.91 41.4686 17.1101 42.4823C19.2324 43.4601 21.5595 43.9998 24 43.9998C26.4405 43.9998 28.7676 43.4601 30.8899 42.4823C33.09 41.4686 35.0699 39.9841 36.7279 38.1419M40.6343 31.6552C41.5143 29.2971 42 26.7115 42 23.9998C42 21.2881 41.5143 18.7024 40.6343 16.3443M36.7279 9.85762C35.0699 8.01539 33.09 6.53089 30.8899 5.51723C28.7676 4.53943 26.4405 3.99976 24 3.99976C19.0294 3.99976 14.5294 6.23833 11.2721 9.85762"/><path d="M18 22.9998C19.6569 22.9998 21 21.6566 21 19.9998C21 18.3429 19.6569 16.9998 18 16.9998C16.3431 16.9998 15 18.3429 15 19.9998C15 21.6566 16.3431 22.9998 18 22.9998Z"/><path d="M30 22.9998C31.6569 22.9998 33 21.6566 33 19.9998C33 18.3429 31.6569 16.9998 30 16.9998C28.3431 16.9998 27 18.3429 27 19.9998C27 21.6566 28.3431 22.9998 30 22.9998Z"/><path d="M24 34.9998C26.7614 34.9998 29 33.6566 29 31.9998C29 30.3429 26.7614 28.9998 24 28.9998C21.2386 28.9998 19 30.3429 19 31.9998C19 33.6566 21.2386 34.9998 24 34.9998Z"/>`
	parkAddOne       = `<path d="M24 44C35.0457 44 44 35.0457 44 24C44 12.9543 35.0457 4 24 4C12.9543 4 4 12.9543 4 24C4 35.0457 12.9543 44 24 44Z"/><path d="M24 16V32"/><path d="M16 24L32 24"/>`
	parkAdd          = `<rect x="6" y="6" width="36" height="36" rx="3"/><path d="M24 16V32"/><path d="M16 24L32 24"/>`
	parkPushpin      = `<path d="M32 4H16L20 7L16 20C16 20 10 24 10 28H20L24 44L28 28H38C38 24 34 21.1667 32 20L28 7L32 4Z"/>`
	parkPeoples      = `<path d="M19 20C22.866 20 26 16.866 26 13C26 9.13401 22.866 6 19 6C15.134 6 12 9.13401 12 13C12 16.866 15.134 20 19 20Z"/><path d="M32.6077 7C34.6405 8.2249 36.0001 10.4537 36.0001 13C36.0001 15.5463 34.6405 17.7751 32.6077 19"/><path d="M4 40.8V42H34V40.8C34 36.3196 34 34.0794 33.1281 32.3681C32.3611 30.8628 31.1372 29.6389 29.6319 28.8719C27.9206 28 25.6804 28 21.2 28H16.8C12.3196 28 10.0794 28 8.36808 28.8719C6.86278 29.6389 5.63893 30.8628 4.87195 32.3681C4 34.0794 4 36.3196 4 40.8Z"/><path d="M43.9999 42.0001V40.8001C43.9999 36.3197 43.9999 34.0795 43.128 32.3682C42.361 30.8629 41.1371 29.6391 39.6318 28.8721"/>`
	parkZoomInternal = `<path d="M44 4H4V44H44V4Z"/><path d="M16 4V16H4"/><path d="M36 24V36H24"/><path d="M36 36L24 24"/><path d="M4 6V26"/><path d="M7 4H27"/>`
	parkMinus        = `<path d="M10.5 24L38.5 24"/>`
	parkHalo         = `<path d="M24 28C26.2091 28 28 26.2091 28 24C28 21.7909 26.2091 20 24 20C21.7909 20 20 21.7909 20 24C20 26.2091 21.7909 28 24 28Z"/><path d="M17.3644 14C16.6922 14.4469 16.0676 14.9598 15.5 15.5295C14.9041 16.1274 14.3709 16.7878 13.9111 17.5"/><path d="M12.1975 26.1802C12.0678 25.4732 12 24.7447 12 24.0002C12 23.2558 12.0678 22.5272 12.1975 21.8203"/><path d="M17.3644 34C16.6922 33.5531 16.0676 33.0402 15.5 32.4705C14.9041 31.8726 14.3709 31.2122 13.9111 30.5"/><path d="M26.3598 35.7681C25.5968 35.9202 24.8077 36 24 36C23.1922 36 22.4031 35.9202 21.6401 35.7681"/><path d="M30.6357 34C31.3079 33.5531 31.9325 33.0402 32.5002 32.4705C33.096 31.8726 33.6292 31.2122 34.089 30.5"/><path d="M35.8027 26.1802C35.9325 25.4732 36.0003 24.7447 36.0003 24.0002C36.0003 23.2558 35.9325 22.5272 35.8027 21.8203"/><path d="M30.6357 14C31.3079 14.4469 31.9325 14.9598 32.5002 15.5295C33.096 16.1274 33.6292 16.7878 34.089 17.5"/><path d="M21.6401 12.232C22.4031 12.0798 23.1922 12 23.9999 12C24.8077 12 25.5968 12.0798 26.3598 12.232"/><path d="M44 24C44 23.325 43.9666 22.6578 43.9012 22C43.834 21.3228 43.733 20.6556 43.5999 20M4 24C4 23.325 4.03344 22.6578 4.09875 22C4.16598 21.3228 4.26699 20.6556 4.40007 20M38.646 37.6198C38.1967 38.1027 37.7238 38.5634 37.2291 39C36.7864 39.3908 36.3262 39.7623 35.8498 40.1132M9.35403 37.6198C9.80332 38.1027 10.2762 38.5634 10.7709 39C11.2136 39.3908 11.6739 39.7623 12.1502 40.1132M15.7124 5.79255C15.0569 6.09142 14.4203 6.42465 13.805 6.78994C13.2345 7.12862 12.6823 7.49486 12.1502 7.88683M32.2876 5.79255C32.9431 6.09142 33.5797 6.42465 34.195 6.78994C34.7655 7.12862 35.3177 7.49486 35.8498 7.88683M26.2465 4.12477C25.5091 4.04233 24.7595 4 24 4C23.2405 4 22.4909 4.04233 21.7535 4.12477M26.2465 43.8752C26.999 43.7911 27.7389 43.6653 28.4638 43.5C29.1569 43.342 29.8364 43.148 30.5 42.92M21.7535 43.8752C21.001 43.7911 20.2611 43.6653 19.5362 43.5C18.8431 43.342 18.1636 43.148 17.5 42.92M8.15495 11.7945C7.77626 12.2854 7.41988 12.7944 7.08732 13.3199C6.78763 13.7935 6.50729 14.2805 6.2474 14.7799M39.8451 11.7945C40.2237 12.2854 40.5801 12.7944 40.9127 13.3199C41.2124 13.7935 41.4927 14.2805 41.7526 14.7799M6.2474 33.2201C5.95677 32.6617 5.69172 32.0878 5.45379 31.5C5.2355 30.9607 5.04005 30.4098 4.86863 29.8483M41.7526 33.2201C42.0432 32.6617 42.3083 32.0878 42.5462 31.5C42.7645 30.9607 42.96 30.4098 43.1314 29.8483"/>`
	parkAttention    = `<path d="M24 44C29.5228 44 34.5228 41.7614 38.1421 38.1421C41.7614 34.5228 44 29.5228 44 24C44 18.4772 41.7614 13.4772 38.1421 9.85786C34.5228 6.23858 29.5228 4 24 4C18.4772 4 13.4772 6.23858 9.85786 9.85786C6.23858 13.4772 4 18.4772 4 24C4 29.5228 6.23858 34.5228 9.85786 38.1421C13.4772 41.7614 18.4772 44 24 44Z"/><path fill="currentColor" stroke="none" d="M24 37C25.3807 37 26.5 35.8807 26.5 34.5C26.5 33.1193 25.3807 32 24 32C22.6193 32 21.5 33.1193 21.5 34.5C21.5 35.8807 22.6193 37 24 37Z"/><path d="M24 12V28"/>`
	parkInfo         = `<path d="M24 44C29.5228 44 34.5228 41.7614 38.1421 38.1421C41.7614 34.5228 44 29.5228 44 24C44 18.4772 41.7614 13.4772 38.1421 9.85786C34.5228 6.23858 29.5228 4 24 4C18.4772 4 13.4772 6.23858 9.85786 9.85786C6.23858 13.4772 4 18.4772 4 24C4 29.5228 6.23858 34.5228 9.85786 38.1421C13.4772 41.7614 18.4772 44 24 44Z"/><path fill="currentColor" stroke="none" d="M24 11C25.3807 11 26.5 12.1193 26.5 13.5C26.5 14.8807 25.3807 16 24 16C22.6193 16 21.5 14.8807 21.5 13.5C21.5 12.1193 22.6193 11 24 11Z"/><path d="M24.5 34V20H23.5H22.5"/><path d="M21 34H28"/>`
)

var (
	// 播放 / 暂停 / 下一集 / 上一集：icon-park 的 PlayOne / Pause / GoEnd / GoStart。
	//
	// 原版 player.tsx 就是这几枚（48×48 画布、描边 4）：播放键 PlayOne（filled，
	// 圆角三角形）、暂停键 Pause（两根圆头竖线）、下一集 GoEnd（人字形 + 右竖线）。
	// 之前是 16 画布手抄：播放是尖角实心三角、下一集是「实心三角 + 竖线」（▶|）——
	// 与 GoEnd 的「>|」形状明显不同，暂停也比原版粗矮，用户能一眼看出差异。
	iconPlayOne = iconPark(parkPlayOne)
	iconPause   = iconPark(parkPause)
	iconNext    = iconPark(parkGoEnd)
	iconPrev    = iconPark(parkGoStart)
	// 合集「播放全部」与播放列表当前行：icon-park 的 Play（filled 主题 =
	// 实心圆盘 + 挖白的三角）。单色 Icon 的掩码只有一层色，画不出「挖白」，
	// 所以这枚走逐色 ui.Image（TextColor 给圆盘上色，三角恒为白）。
	// 用法：ui.Image(c, iconPlayFilled).Size(s, s).TextColor(ink)。
	iconPlayFilled = ui.MustParseSVG([]byte(`<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 48 48" fill="none">` +
		`<path fill="currentColor" stroke="currentColor" stroke-width="4" d="M24 44C35.0457 44 44 35.0457 44 24C44 12.9543 35.0457 4 24 4C12.9543 4 4 12.9543 4 24C4 35.0457 12.9543 44 24 44Z"/>` +
		`<path fill="#ffffff" stroke="#ffffff" stroke-width="4" d="M20 24V17.0718L26 20.5359L32 24L26 27.4641L20 30.9282V24Z"/>` +
		`</svg>`))
	// 音量：icon-park 的 VolumeNotice（喇叭 + 两道弧）/ VolumeMute（喇叭 + 斜杠）。
	iconVolume = iconPark(parkVolumeNotice)
	// 左右箭头（收藏夹表头滚动键，原版 collectList 的 Left / Right）。
	iconLeft  = iconPark(parkLeft)
	iconRight = iconPark(parkRight)
	iconMute  = iconPark(parkVolumeMute)
	// 弹幕/评论：icon-park 的 Comment（方形气泡 + 三个点）。
	iconDanmaku = iconPark(parkComment)
	// 卡片 meta 的播放量前导图标：icon-park 的 PreviewOpen（眼睛），原版 CardMeta
	// 的 views 字段用的就是它 —— 之前错用了 iconPlay 的三角。
	iconViews = iconPark(parkPreviewOpen)
	// 弹幕 tab 的文字图标（原版 danmakuList 用 icon-park 的 Text）。
	iconText = iconPark(parkText)
	// 弹幕抽屉的「自动跟随」键（原版用 icon-park 的 Connection，一条折线 + 链环）。
	iconConnection = iconPark(parkConnection)
	// 选集：icon-park 的 DoubleUp（双层人字形向上）。
	iconList   = iconPark(parkDoubleUp)
	iconSearch = iconPark(parkSearch)
	// 关窗 / 抽屉关闭 / 播放列表行删除（原版 Close）/ 登录面板关闭（原版 CloseSmall）。
	iconClose      = iconPark(parkClose)
	iconCloseSmall = iconPark(parkCloseSmall)
	// 互动：icon-park 的 ThumbsUp / HandleB（投币，圆形钱币）/ Star。
	// 收藏键的 active 态是 Star 的 filled 变体（原版
	// `theme={isFavorite ? "filled" : "outline"}`），所以两枚都留着。
	iconLike       = iconPark(parkThumbsUp)
	iconCoin       = iconPark(parkHandleB)
	iconStar       = iconPark(parkStar)
	iconStarFilled = iconPark(parkStarFilled)
	iconEQ         = iconPark(parkEqualizer)
	// 恰饭跳过：icon-park 的 Ad（圆圈 + 「广」）。
	iconSponsor = iconPark(parkAd)

	// 搜索栏右侧四个入口（原版 searchForm.tsx 用的是 icon-park 的
	// ShareSys / ChartRing / WeixinFavorites / History，theme="outline"、size 18）。
	// 这里逐条抄 icon-park 的原始 path（48 画布、描边 4），不再是 16 画布的近似。
	iconFeed     = iconPark(parkShareSys)
	iconPopular  = iconPark(parkChartRing)
	iconFavorite = iconPark(parkWeixinFavorites)
	iconHistory  = iconPark(parkHistory)
	// 放大镜（原版 icon-park 的 Search）：搜索药丸的提交键（size 18）
	// 与选集抽屉的搜索框（size 15）共用。
	iconMagnifier = iconPark(parkSearch)
	// 浏览器（Browser）/ 视频（VideoTwo）/ 播放列表（MusicList）/ 合集（Layers）：
	// 均为 icon-park 原始 path。
	iconBrowser   = iconPark(parkBrowser)
	iconVideo     = iconPark(parkVideoTwo)
	iconMusicList = iconPark(parkMusicList)
	iconSeries    = iconPark(parkLayers)
	// 迷你模式的两个窗口控制：还原大窗 / 置顶。
	// 原版 miniVideoInfo 的「切换到窗口模式」与标题栏的「切换到迷你模式」用的
	// 都是同一枚 icon-park ZoomInternal；还原键此前是两方块手抄近似，形状对不上。
	iconRestore = iconPark(parkZoomInternal)
	iconPin     = iconPark(parkPushpin)
	// iconPinFilled 是同一枚图钉的实心变体（原版 <Pushpin theme="filled">）：
	// filled 主题把 colors[1]（内部填充）也取成 currentColor，所以只是同一条 path
	// 加上 fill=currentColor —— 绝不是另一枚手绘的形状。
	iconPinFilled = iconPark(`<path fill="currentColor" d="M32 4H16L20 7L16 20C16 20 10 24 10 28H20L24 44L28 28H38C38 24 34 21.1667 32 20L28 7L32 4Z"/>`)
	// 标题栏的「切换到迷你模式」（原版 #switch-window-mode 用的 ZoomInternal）。
	iconMini = iconPark(parkZoomInternal)
	// 抽屉表头的刷新键（原版 Refresh）与历史抽屉表头的「隐身」开关（原版 MaskOne）。
	// MaskOne 开启时是 filled 主题（内部整块填墨），所以留一笔实心变体。
	iconMask       = iconPark(parkMaskOne)
	iconMaskFilled = iconPark(`<path d="M7.36573 16.3443C6.48571 18.7024 6 21.2881 6 23.9998C6 26.7115 6.48571 29.2971 7.36573 31.6552M11.2721 38.1419C12.9301 39.9841 14.91 41.4686 17.1101 42.4823C19.2324 43.4601 21.5595 43.9998 24 43.9998C26.4405 43.9998 28.7676 43.4601 30.8899 42.4823C33.09 41.4686 35.0699 39.9841 36.7279 38.1419M40.6343 31.6552C41.5143 29.2971 42 26.7115 42 23.9998C42 21.2881 41.5143 18.7024 40.6343 16.3443M36.7279 9.85762C35.0699 8.01539 33.09 6.53089 30.8899 5.51723C28.7676 4.53943 26.4405 3.99976 24 3.99976C19.0294 3.99976 14.5294 6.23833 11.2721 9.85762"/><path fill="currentColor" d="M18 22.9998C19.6569 22.9998 21 21.6566 21 19.9998C21 18.3429 19.6569 16.9998 18 16.9998C16.3431 16.9998 15 18.3429 15 19.9998C15 21.6566 16.3431 22.9998 18 22.9998Z"/><path fill="currentColor" d="M30 22.9998C31.6569 22.9998 33 21.6566 33 19.9998C33 18.3429 31.6569 16.9998 30 16.9998C28.3431 16.9998 27 18.3429 27 19.9998C27 21.6566 28.3431 22.9998 30 22.9998Z"/><path fill="currentColor" d="M24 34.9998C26.7614 34.9998 29 33.6566 29 31.9998C29 30.3429 26.7614 28.9998 24 28.9998C21.2386 28.9998 19 30.3429 19 31.9998C19 33.6566 21.2386 34.9998 24 34.9998Z"/>`)
	iconRefresh    = iconPark(parkRefresh)
	// UP 空间抽屉里的关注 / 已关注（原版 upVideoList：未关注是 Add，已关注是 Close）。
	iconFollow   = iconPark(parkAdd)
	iconFollowed = iconPark(parkClose)
	// 封面背景（Halo）与高级质感（Sparkles）。
	iconHalo     = iconPark(parkHalo)
	iconSparkles = icon(`<path d="M6.4 10.2A1.5 1.5 0 0 0 5.3 9.1L1.6 8a.4.4 0 0 1 0-.7l3.7-1.1A1.5 1.5 0 0 0 6.4 5.1l1.1-3.7a.4.4 0 0 1 .7 0l1.1 3.7a1.5 1.5 0 0 0 1.1 1.1l3.7 1.1a.4.4 0 0 1 0 .7l-3.7 1.1a1.5 1.5 0 0 0-1.1 1.1l-1.1 3.7a.4.4 0 0 1-.7 0z"/><path d="M13 2v2.4M14.2 3.2h-2.4"/>`)
	// 合作 UP 主选择键（原版 icon-park 的 Peoples）：两个人形。
	iconPeoples = iconPark(parkPeoples)
	// 定位当前（FocusOne）。
	iconLocate = iconPark(parkFocusOne)
	// 播放列表：播放模式 / 删除 / 添加 / 已添加。
	// 原版 playlist.tsx 用的是 Order / LoopOnce / Shuffle，
	// 选集页「定位当前」与「全部添加」是 FocusOne / AddOne。
	iconOrder   = iconPark(parkOrder)
	iconLoopOne = iconPark(parkLoopOnce)
	iconShuffle = iconPark(parkShuffleOne)
	iconDelete  = iconPark(parkDelete)
	iconAdd     = iconPark(parkAddOne)
	iconCheck   = iconPark(parkCheck)
	// 图标三态：成功 / 失败 / 警告 / 信息（原版 CheckOne / CloseOne / Attention / Info）。
	iconCheckOne = iconPark(parkCheckOne)
	iconCloseOne = iconPark(parkCloseOne)
	iconInfo     = iconPark(parkInfo)
	iconWarn     = iconPark(parkAttention)
)

// pick 是三元表达式的泛型版。
func pick[T any](cond bool, a, b T) T {
	if cond {
		return a
	}
	return b
}

// compactCount 把数量转成「1.2万」这样的短文本（原版 formatCompactCount）。
// compactCount 是三键数值（点赞/投币/收藏）的显示（原版
// formatCompactCount）：≥1亿 → x.x亿，≥100万 → 整数 w，≥1万 → x.x w。
func compactCount(n int64) string {
	switch {
	case n >= 100000000:
		return fmt.Sprintf("%.1f亿", float64(n)/1e8)
	case n >= 1000000:
		return fmt.Sprintf("%dw", n/10000)
	case n >= 10000:
		return fmt.Sprintf("%.1fw", float64(n)/1e4)
	default:
		return fmt.Sprint(n)
	}
}

// fanCount 是 UP 空间表头的粉丝数（原版 upVideoList 的 formatNumber）：
// ≥1万 显示「x.x万」——注意这里用的是「万」，与 compactCount 的「w」不同，
// 原版两个函数就是两套写法。
func fanCount(n int64) string {
	if n >= 10000 {
		return fmt.Sprintf("%.1f万", float64(n)/1e4)
	}
	return fmt.Sprint(n)
}

// pubdateCandidates 把卡片里的发布时间字符串展开成「具体 → 抽象」的候选阶梯
// （原版 pubdateCandidates / dateFormLadder，轮 30）。
//
// 只有完整的 yyyy-MM-DD 可能放不下、需要按列宽降级；其余形态（今天 / 昨天 /
// N天前 / MM-DD / 无法解析的原串）都足够短，作为唯一候选原样显示。
func pubdateCandidates(value string, now time.Time) []string {
	raw := strings.TrimSpace(value)
	if raw == "" {
		return nil
	}
	target, err := time.ParseInLocation("2006-01-02", raw, now.Location())
	if err != nil {
		return []string{raw} // 已是相对形态 / 短形态：原样作唯一候选
	}
	// 自然月差（当月同日没到就少算一个月），夹到 ≥1。
	monthDiff := (now.Year()-target.Year())*12 + int(now.Month()-target.Month())
	if now.Day() < target.Day() {
		monthDiff--
	}
	if monthDiff < 1 {
		monthDiff = 1
	}
	cands := []string{raw, target.Format("2006-01")}
	// N个月前 / N年前 是「必定放得下」的兜底（原版 dateFormLadder 的 floor）。
	cands = append(cands, fmt.Sprintf("%d个月前", monthDiff))
	if years := now.Year() - target.Year(); years >= 1 {
		cands = append(cands, fmt.Sprintf("%d年前", years))
	}
	return cands
}

// pickDateForm 取候选阶梯里第一个 fits 的形态；都不满足时取最后一档（兜底，
// 保证绝不返回空）。原版 pickDateForm 的 Go 版。
func pickDateForm(candidates []string, fits func(string) bool) string {
	for _, c := range candidates {
		if fits(c) {
			return c
		}
	}
	if len(candidates) == 0 {
		return ""
	}
	return candidates[len(candidates)-1]
}

// iconDiscSheen 是唱片高光：一条穿过圆心的宽光带 + 两道细弧（旋转时像
// 唱片表面在反光）。viewBox 0 0 100 100。
// 不用 opacity 属性（解析器不认），透明度用浅色模拟。
// 高光弧：整圆环但用两段三次贝塞尔（解析器不支持 A 弧命令），配合
// 旋转时一侧的粗/细不对称，看起来像唱片表面反光。为了"缺口"效果，
// 用一段 270° 弧 + 一段 90° 直线收口（全部 C 命令）。
var iconDiscSheen = ui.MustParseSVG([]byte(`<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 100 100" fill="none" stroke="currentColor" stroke-linecap="round"><path d="M50 6 C76 6 94 26 94 50 C94 76 76 94 50 94 C26 94 6 76 6 50 C6 26 26 6 50 6" stroke-width="7"/><path d="M50 6 C64 6 76 14 82 26" stroke-width="10"/></svg>`))

// progressInput 是进度条一帧的输入（都是框架/引擎给的外部状态）。
type progressInput struct {
	held    bool          // 指针是否按在进度条上（Dragged 的 ok）
	changed bool          // 框架本帧是否改了值（键盘/快速点击）
	busy    bool          // 一次 seek 还在跑
	settle  time.Duration // 距最近一次交接点（按下/松手/seek 完成）的时间
	pos     float64       // 播放位置
}

// progressPlan 是进度条每帧的决策：是否提交 seek、滑块值该是多少。
//
// 抽成纯函数是为了把下面四条不变量钉死（这块反复出过问题）：
//  1. 按住时不跟随播放位置——否则每 500ms 的进度上报会把滑块拽回去；
//  2. 松手当帧提交 seek，且**用用户放下的值**（调用方必须先读 value
//     再写新值，这里通过返回值分离做到结构上不可能写反）；
//  3. seek 在跑 / 刚过交接点时停在目标值，不跟随（Pos 慢半拍，跟随会弹回）；
//  4. 其余时间跟随播放位置。
func progressPlan(in progressInput, value float64, wasHeld bool, settle time.Duration) (seek bool, next float64) {
	next = value
	if wasHeld && !in.held {
		seek = true // 松手：拖动结束 / 点击的按下-松开
	}
	if in.changed && !in.held && !wasHeld {
		seek = true // 键盘方向键改值
	}
	if !seek && !in.held && !in.busy && in.settle > settle {
		next = in.pos // 跟随播放
	}
	return seek, next
}

// ResetSeek 把进度条的 seek 状态机复位（换歌后跟随新进度），并让唱片
// 重新取一次位置基准——否则换歌时 Pos 从旧位置跳到 0 会被当成快退。
func (a *App) ResetSeek() {
	a.wasHeld, a.SeekBusy, a.SeekStableAt = false, false, time.Time{}
	a.discLast, a.discLastPos = time.Time{}, 0
}

// sameSlice 报告两个切片是不是同一段数据：长度相同且（非空时）底层数组首
// 元素地址相同。配合调用方持有旧切片（续命，见 App.dmGroupsSrc），指针
// 身份就不会被 GC 回收后的地址复用干扰。空切片一律视为相同。
func sameSlice[T any](a, b []T) bool {
	if len(a) != len(b) {
		return false
	}
	return len(a) == 0 || &a[0] == &b[0]
}

// Seeking 报告用户是否正在拖动进度条（供 OnProgress 跳过副作用）。
// Seeking 表示用户正在操作进度条，或一次 seek 还在跑——这期间
// OnProgress 的进度副作用（断点/KV/恰饭判定）要跳过。
func (a *App) Seeking() bool { return a.wasHeld || a.SeekBusy }

// pressable 是按钮的按压反馈。原版是 `.player-button:active` 的
// `transform: scale(0.93)`，mygo 没有缩放变换（Rotate 只管角度），
// 退而求其次：按住时底色加深一档，给出「按下了」的反馈。
func pressable(e ui.Element, rest, pressed ui.Color) ui.Element {
	if e.Pressed() {
		return e.Background(pressed)
	}
	return e.Background(rest)
}

// pressFeedback 是按钮的 hover/press 组合反馈：rest 平时、hover 悬停、
// pressed 按住（mygo 无缩放变换，用底色表达「按下」，见 pressable）。
func pressFeedback(e ui.Element, rest, hover, pressed ui.Color) ui.Color {
	switch {
	case e.Pressed():
		return pressed
	case e.Hovered():
		return hover
	}
	return rest
}

// scrollHoldEnabled 是「滚动时锁住上浮」的总开关。生产恒为 true；测试把它
// 置 false 复现改动前的行为（每帧都重建 lift 动画），供无头计数器做改前/
// 改后对照（见 liftscroll_hold_test.go）。测试改完必须还原。
var scrollHoldEnabled = true

// scrollHold 是「滚动进行中锁住选中/悬停上浮」的状态机：滚动期间不渲染
// 卡片上浮（renderCard 不再重建 lift 动画 / 悬停光晕）。位置一变就按住；
// 位置连续不变的第一帧就松开，本帧 renderCard 把上浮画回来并结束求帧。
//
// 连续滚动（滚轮/触摸板每帧都有位移）时位置每帧都变，锁一直按住；只在
// 真正停下后的那一帧恢复。不设「落定窗口」是为了不空转帧：停下后既能
// 立刻恢复上浮，又只需多求一帧。
type scrollHold struct {
	last float64 // 上一帧的滚动位置（float64：长列表首行 * 2^20 + 偏移不失真）
	set  bool    // last 是否有效（容器上一帧在不在视图里）
	held bool    // 当前是否按住（滚动中）
}

// observe 上报这一帧容器的滚动位置与可见性，返回 locked（这一帧要不要锁
// 住上浮）与 needFrame（是否要再求一帧）。
//
// 位置一变就立刻锁住：滚轮/拖拽的偏移在事件里已同步写回 ScrollState，
// 本帧构建就能读到（见 ui.scrollBy → state.scrollTo），所以滚动的第一帧
// 也拿得到锁定，无需再等一帧。位置恢复不变的那一帧返回 unlocked 并停帧，
// 让 renderCard 在同一帧把上浮画回去。
//
// needFrame 为真时调用方必须 c.AnimationFrame()：位移帧要主动求下一帧，
// 否则滚动停下后没有自然帧来到，锁会挂住不解除。
func (s *scrollHold) observe(offset float64, shown bool) (locked, needFrame bool) {
	locked = s.held
	if !shown {
		// 容器不在视图里（抽屉关着 / 换 tab）：位置观测无效，但保持锁定
		// 状态 —— 下次出现时若位置没变就当从未滚动，变了立刻按住。
		s.set = false
		return locked, false
	}
	switch {
	case !s.set:
		// 首次观测只建基线，不算滚动。
		s.last, s.set = offset, true
	case offset != s.last:
		// 位移发生的本帧就锁住，并求下一帧来判定「停下 & 恢复」。
		s.last, s.held = offset, true
		locked, needFrame = true, true
	case s.held:
		// 位置不再变：滚动停下，本帧就恢复渲染上浮（不再求帧）。
		s.held = false
		locked = false
	}
	return locked, needFrame
}

// holding 报告状态机当前是否按住（供测试断言）。
func (s *scrollHold) holding() bool { return s.held }
