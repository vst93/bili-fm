package media

import "errors"

// Kind 是一次播放失败的**类别**，对应原版 HTML `<audio>` 的 MediaError.code。
//
// 原版 player.tsx 的 onError 就是按这个码选一条给用户看的提示：
//
//	4 → 音频格式不支持或加载失败
//	3 → 音频解码失败
//	2 → 网络错误，音频加载失败
//	其它 → 音频加载出错
//
// 原生版没有浏览器那套媒体元素，码得由引擎自己给：错误发生在哪一层就
// 带上哪一层的类别（HTTP 在 remote.go 带 KindNetwork、解封装/解码在
// player.go 带 KindUnsupported / KindDecode）。上层（main.go 的
// audioErrorMessage）只认 Kind，不用去猜错误字符串。
type Kind uint8

const (
	// KindOther 是没归类的失败（对应原版的「其它」兜底文案）。
	KindOther Kind = iota
	// KindNetwork 是取流失败：连不上、HTTP 状态码不是 200/206、读响应体中断。
	KindNetwork
	// KindDecode 是解码失败：AAC 解码器建不起来或解码中途报错。
	KindDecode
	// KindUnsupported 是这份流本身不能用：解封装读不出 MP4、声道数变了。
	KindUnsupported
	// KindOutput 是本地音频设备打不开（原版没有这一层，归到兜底文案）。
	KindOutput
)

// Error 给一个底层错误带上类别。Error()/Unwrap() 让它对上层完全透明 ——
// 日志里照旧是原来那句话，errors.Is/As 也照旧能穿透到根因。
type Error struct {
	Kind Kind
	Err  error
}

func (e *Error) Error() string { return e.Err.Error() }

func (e *Error) Unwrap() error { return e.Err }

// WrapKind 给 err 打上类别 k。链上已经有类别的就不再套一层：**根因优先**。
//
// 例：m4a.NewReader 因为一次 HTTP 403 失败，它包着的错误已经是 KindNetwork；
// 外层再套 KindUnsupported 就会让用户看到「格式不支持」而真因是网络 ——
// 与其猜，不如让最内层那个知道自己在干什么的人说话。
func WrapKind(k Kind, err error) error {
	if err == nil {
		return nil
	}
	var e *Error
	if errors.As(err, &e) {
		return err
	}
	return &Error{Kind: k, Err: err}
}

// KindOf 报告一次播放失败的类别；没带类别（或 err 为 nil）算 KindOther。
func KindOf(err error) Kind {
	var e *Error
	if errors.As(err, &e) {
		return e.Kind
	}
	return KindOther
}

// ErrHEAAC 表示这份流的音频是 HE-AAC（AAC-LC + SBR/PS），纯 Go 解码器
// （go-aac 只做 AAC-LC）解不了。
//
// 它不是「播放地址坏了」：B 站 360P 合流在部分稿件里用 HE-AAC 省码率，
// 同一集往往还有 DASH 的 AAC-LC 音轨。上层（main.go）据此换音轨重试，
// 不该直接弹「音频解码失败」。
var ErrHEAAC = errors.New("media: HE-AAC（SBR）暂不支持")

// ErrSuperseded 表示这次起播被更新的起播取代了。
//
// 建流水线要发 HTTP（秒级），期间用户切了别的曲目，这次的结果就作废。
// 它**不是失败**：调用方必须忽略它（既不能弹提示，也不能动界面状态，
// 否则会把新那一次正在进行的起播标成「失败」）。声明成导出量是为了让
// main.go 能 errors.Is 判定。
var ErrSuperseded = errors.New("media: 起播已被新的请求取代")
