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

	// UpOffset 是 UP 视频列表的翻页游标（旧版是 offset 而不是页码）。
	UpOffset string
	// FeedOffset 是「动态」列表的翻页游标（同样是 offset）。
	FeedOffset string

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
	// Muted 是静音状态（原版音量弹层里的独立静音键）。
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
	Info *Info
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
	// danmakuCid 用于换集时重置它。
	danmakuList ui.ListState
	danmakuCid  int64

	// grids 是每个抽屉的卡片网格状态（ui.GridView 要一个稳定的 GridState，
	// 按抽屉分开存才能各自保留滚动位置）。
	gridsMu sync.Mutex
	grids   map[string]*ui.GridState
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

var (
	// 播放/暂停/下一首：icon-park 的 PlayOne / Pause / GoEnd。
	iconPlay  = icon(`<path d="M5.5 3.2v9.6a.8.8 0 0 0 1.22.68l7.6-4.8a.8.8 0 0 0 0-1.36l-7.6-4.8A.8.8 0 0 0 5.5 3.2z"/>`)
	iconPause = icon(`<rect x="4.4" y="3" width="2.8" height="10" rx="0.9"/><rect x="8.8" y="3" width="2.8" height="10" rx="0.9"/>`)
	// GoEnd：右三角 + 右侧竖线。
	iconNext = icon(`<path d="M3.6 3.4v9.2a.7.7 0 0 0 1.08.6l6.2-4.6a.7.7 0 0 0 0-1.2L4.68 2.8a.7.7 0 0 0-1.08.6z"/><path d="M12.2 3v10" stroke-width="1.8"/>`)
	iconPrev = icon(`<path d="M12.4 3.4v9.2a.7.7 0 0 1-1.08.6l-6.2-4.6a.7.7 0 0 1 0-1.2l6.2-4.6a.7.7 0 0 1 1.08.6z"/><path d="M3.8 3v10" stroke-width="1.8"/>`)
	// 音量：VolumeNotice（喇叭 + 两道弧）/ VolumeMute（喇叭 + 斜杠）。
	iconVolume = icon(`<path d="M3.2 6h2.1l3.3-2.7a.6.6 0 0 1 .98.46v8.48a.6.6 0 0 1-.98.46L5.3 10H3.2a.7.7 0 0 1-.7-.7V6.7a.7.7 0 0 1 .7-.7z"/><path d="M11.4 6.2a2.6 2.6 0 0 1 0 3.6"/><path d="M12.9 4.6a4.6 4.6 0 0 1 0 6.8"/>`)
	// 左右箭头（收藏夹表头滚动键）。
	iconLeft  = icon(`<path d="M10 3.2L5.2 8l4.8 4.8"/>`)
	iconRight = icon(`<path d="M6 3.2L10.8 8L6 12.8"/>`)
	iconMute  = icon(`<path d="M3.2 6h2.1l3.3-2.7a.6.6 0 0 1 .98.46v8.48a.6.6 0 0 1-.98.46L5.3 10H3.2a.7.7 0 0 1-.7-.7V6.7a.7.7 0 0 1 .7-.7z"/><path d="M10.8 6.4l3.4 3.2M14.2 6.4l-3.4 3.2" stroke-width="1.6"/>`)
	// 弹幕/评论：Comment（气泡 + 三点）。
	iconDanmaku = icon(`<path d="M2.8 4.6a1.8 1.8 0 0 1 1.8-1.8h6.8a1.8 1.8 0 0 1 1.8 1.8v4.2a1.8 1.8 0 0 1-1.8 1.8H6.4l-2.6 2.2v-2.2h-.2a1.8 1.8 0 0 1-1.8-1.8z"/><path d="M5.4 6.7h5.2M5.4 8.6h3.2" stroke-width="1.1"/>`)
	// 弹幕 tab 的文字图标（原版 danmakuList 用 icon-park 的 Text）。
	iconText = icon(`<path d="M3.2 3.8h9.6"/><path d="M8 3.8v8.4"/>`)
	// 弹幕抽屉的「自动跟随」键（原版用 icon-park 的 Connection，两段链环）。
	iconConnection = icon(`<path d="M6.6 9.4l2.8-2.8"/><path d="M5.6 7.1l-1 1a2.7 2.7 0 0 0 3.8 3.8l1-1"/><path d="M10.4 8.9l1-1a2.7 2.7 0 0 0-3.8-3.8l-1 1"/>`)
	// 选集：DoubleUp（双层人字形向上）。
	iconList   = icon(`<path d="M3.4 8.6L8 4.2l4.6 4.4M3.4 12.2L8 7.8l4.6 4.4" stroke-width="1.7"/>`)
	iconSearch = icon(`<circle cx="7" cy="7" r="4.4"/><path d="M10.4 10.4L14 14"/>`)
	iconClose  = icon(`<path d="M4 4l8 8M12 4l-8 8"/>`)
	iconMin    = icon(`<path d="M4 8h8"/>`)
	// 互动：ThumbsUp / HandleB(投币，圆形钱币) / Star。
	iconLike  = icon(`<path d="M6.4 13.6V7l2.2-4a1.1 1.1 0 0 1 1.9.9l-.4 2.7h2.8a1.2 1.2 0 0 1 1.18 1.42l-.86 4.4a1.6 1.6 0 0 1-1.57 1.3H6.4z"/><path d="M6.4 7H3.9a.9.9 0 0 0-.9.9v5a.9.9 0 0 0 .9.9h2.5"/>`)
	iconCoin  = icon(`<circle cx="8" cy="8" r="5.6"/><path d="M8 5.2v5.6M6.4 6.6c0-.8.7-1.4 1.6-1.4s1.6.6 1.6 1.4-.7 1.2-1.6 1.4c-.9.2-1.6.6-1.6 1.4s.7 1.4 1.6 1.4 1.6-.6 1.6-1.4"/>`)
	iconStar  = icon(`<path d="M8 1.9l1.9 4.2 4.6.5-3.4 3.1.9 4.5L8 11.9l-4 2.2.9-4.5L1.5 6.5l4.6-.5z"/>`)
	iconSpeed = icon(`<circle cx="8" cy="8" r="6"/><path d="M8 8l3-2.2"/>`)
	iconEQ    = icon(`<path d="M3 11V5M6.5 13V3M10 10V6M13.5 12V4"/>`)
	// 恰饭跳过：icon-park 的 Ad（方框 + "广"的简化），这里用方框 + 小三角播放。
	iconSponsor = icon(`<rect x="2" y="4.5" width="12" height="7" rx="1.2"/><path d="M6.4 6.4l3.6 1.6-3.6 1.6z"/>`)

	// 搜索栏右侧四个入口（ShareSys / ChartRing / WeixinFavorites / History）。
	iconFeed     = icon(`<path d="M13.4 8A5.4 5.4 0 1 1 8 2.6"/><path d="M10.8 8A2.8 2.8 0 1 1 8 5.2"/><circle cx="8" cy="8" r="1"/>`)
	iconPopular  = icon(`<circle cx="8" cy="8" r="6"/><circle cx="8" cy="8" r="2.1"/><path d="M12.6 3.4l1.2-1.2"/>`)
	iconFavorite = icon(`<path d="M8 1.9l5.3 2.9v6.4L8 14.1 2.7 11.2V4.8z"/><path d="M2.7 4.8L8 7.7l5.3-2.9"/><path d="M8 7.7v6.4"/>`)
	iconHistory  = icon(`<path d="M2.7 8a5.3 5.3 0 1 0 1.7-3.9"/><path d="M2.3 2.6v2.7H5"/><path d="M8 5.3V8l2.1 1.3"/>`)
	// 搜索药丸里的放大镜（提交键）。
	iconMagnifier = icon(`<circle cx="7.2" cy="7.2" r="4.3"/><path d="M10.4 10.4L13.8 13.8"/>`)
	// 浏览器（Browser）与播放列表（MusicList）。
	iconBrowser = icon(`<rect x="1.8" y="3" width="12.4" height="10" rx="1.6"/><path d="M1.8 6.2h12.4"/><circle cx="4.3" cy="4.6" r="0.5"/><circle cx="6.3" cy="4.6" r="0.5"/>`)
	// 视频播放：VideoTwo（圆角方框 + 右下播放角）。
	iconVideo     = icon(`<rect x="1.8" y="3" width="12.4" height="9" rx="1.6"/><path d="M7 6.6l3 1.4-3 1.4z"/>`)
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
	// 工具条的「合集」（原版用 icon-park 的 Layers）。
	iconSeries = icon(`<path d="M8 1.9l6.2 3.2L8 8.3 1.8 5.1z"/><path d="M2.6 8.2l5.4 2.8 5.4-2.8"/><path d="M2.6 11.2l5.4 2.8 5.4-2.8"/>`)
	// UP 空间抽屉里的关注 / 已关注。
	// 封面背景（Halo）与高级质感（Sparkles）。
	iconHalo     = icon(`<circle cx="8" cy="8" r="3.2"/><path d="M8 1.4v2M8 12.6v2M1.4 8h2M12.6 8h2M3.3 3.3l1.4 1.4M11.3 11.3l1.4 1.4M12.7 3.3l-1.4 1.4M4.7 11.3l-1.4 1.4"/>`)
	iconSparkles = icon(`<path d="M6.4 10.2A1.5 1.5 0 0 0 5.3 9.1L1.6 8a.4.4 0 0 1 0-.7l3.7-1.1A1.5 1.5 0 0 0 6.4 5.1l1.1-3.7a.4.4 0 0 1 .7 0l1.1 3.7a1.5 1.5 0 0 0 1.1 1.1l3.7 1.1a.4.4 0 0 1 0 .7l-3.7 1.1a1.5 1.5 0 0 0-1.1 1.1l-1.1 3.7a.4.4 0 0 1-.7 0z"/><path d="M13 2v2.4M14.2 3.2h-2.4"/>`)
	iconFollow   = icon(`<path d="M8 3.6v8.8M3.6 8h8.8"/>`)
	iconFollowed = icon(`<path d="M3.4 8.4l3 3 6.2-6.8"/>`)
	// 合作 UP 主选择键（原版 icon-park 的 Peoples）：两个人形。
	iconPeoples = icon(`<circle cx="6" cy="5.6" r="2.3"/><path d="M2.2 13.2c0-2.1 1.7-3.7 3.8-3.7s3.8 1.6 3.8 3.7"/><circle cx="11.6" cy="4.6" r="1.7"/><path d="M10.9 10c1.6.2 2.9 1.6 2.9 3.2"/>`)
	// 定位当前（FocusOne）。
	iconLocate = icon(`<circle cx="8" cy="8" r="3"/><path d="M8 1.4v2.2M8 12.4v2.2M1.4 8h2.2M12.4 8h2.2"/>`)
	// 播放列表：播放模式 / 删除 / 添加 / 已添加 / 上移下移。
	iconOrder   = icon(`<path d="M2.5 4h11M2.5 8h7M2.5 12h11"/>`)
	iconLoop    = icon(`<path d="M3 8a5 5 0 0 1 8.5-3.5L13 6"/><path d="M13 2.6V6H9.6"/><path d="M13 8a5 5 0 0 1-8.5 3.5L3 10"/><path d="M3 13.4V10h3.4"/>`)
	iconLoopOne = icon(`<path d="M3 8a5 5 0 0 1 8.5-3.5L13 6"/><path d="M13 2.6V6H9.6"/><path d="M13 8a5 5 0 0 1-8.5 3.5L3 10"/><path d="M3 13.4V10h3.4"/><path d="M7 6.4v3.2"/>`)
	iconShuffle = icon(`<path d="M2.5 4.5h2.2l6.8 7h2"/><path d="M2.5 11.5h2.2l2.3-2.4"/><path d="M9 6.9l2.5-2.4h2"/><path d="M11.4 2.6L13.5 4.5l-2.1 1.9M11.4 9.6l2.1 1.9-2.1 1.9"/>`)
	iconDelete  = icon(`<path d="M3 4.5h10M6.2 4.5V3h3.6v1.5M4.4 4.5l.6 8.2a1 1 0 0 0 1 .9h4a1 1 0 0 0 1-.9l.6-8.2"/>`)
	iconAdd     = icon(`<path d="M8 3.4v9.2M3.4 8h9.2"/>`)
	iconCheck   = icon(`<path d="M3.2 8.4l3.2 3.2 6.4-7"/>`)
	iconUp      = icon(`<path d="M8 12.5V3.5M4.2 7.3L8 3.5l3.8 3.8"/>`)
	// toast 用的信息 / 警告（原版 Info / Attention）。
	iconInfo = icon(`<circle cx="8" cy="8" r="6"/><path d="M8 7.4v3.8"/><path d="M8 5.1v.1" stroke-width="2"/>`)
	iconWarn = icon(`<path d="M8 2.2l6.1 10.8H1.9z"/><path d="M8 6.6v3.2"/><path d="M8 11.7v.1" stroke-width="2"/>`)
	iconDown = icon(`<path d="M8 3.5v9M11.8 8.7L8 12.5 4.2 8.7"/>`)
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
