package bilibili

import (
	"crypto/md5"
	"fmt"
	"net/url"
	"path"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"
)

// B 站 WBI 签名。player/wbi/v2 与搜索等接口都要求 wts + w_rid；不带签名
// 时有时会返回公开结果，有时会命中旧缓存/空结果，字幕列表因此不稳定。
//
// 算法来自 bilibili-API-collect / bpi-rs：
//  1. 从 nav 的 wbi_img.img_url / sub_url 取文件名（去掉扩展名）；
//  2. img_key+sub_key 按固定 64 项表重排，取前 32 个字符作为 mixinKey；
//  3. 参数按 key 排序、过滤 !'()*、加 wts，md5(query+mixinKey) 得 w_rid。
var wbiMixinTab = [64]int{
	46, 47, 18, 2, 53, 8, 23, 32, 15, 50, 10, 31, 58, 3, 45, 35,
	27, 43, 5, 49, 33, 9, 42, 19, 29, 28, 14, 39, 12, 38, 41, 13,
	37, 48, 7, 16, 24, 55, 40, 61, 26, 17, 0, 1, 60, 51, 30, 4,
	22, 25, 54, 21, 56, 59, 6, 63, 57, 62, 11, 36, 20, 34, 44, 52,
}

type wbiKeys struct {
	img, sub string
	at       time.Time
}

var wbiCache struct {
	sync.Mutex
	keys wbiKeys
}

func wbiMixinKey(img, sub string) string {
	combined := img + sub
	var b strings.Builder
	for _, i := range wbiMixinTab {
		if i < len(combined) && b.Len() < 32 {
			b.WriteByte(combined[i])
		}
	}
	return b.String()
}

func wbiFilter(v string) string {
	return strings.Map(func(r rune) rune {
		if strings.ContainsRune("!'()*", r) {
			return -1
		}
		return r
	}, v)
}

func wbiEncode(v string) string {
	return strings.ReplaceAll(url.QueryEscape(wbiFilter(v)), "+", "%20")
}

// signWBIAt 拆成纯函数，便于用固定时间戳钉住签名。
func signWBIAt(values url.Values, img, sub string, ts int64) url.Values {
	out := make(url.Values, len(values)+2)
	for k, vs := range values {
		for _, v := range vs {
			out.Add(k, wbiFilter(v))
		}
	}
	out.Set("wts", strconv.FormatInt(ts, 10))
	keys := make([]string, 0, len(out))
	for k := range out {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	var b strings.Builder
	for i, k := range keys {
		if i > 0 {
			b.WriteByte('&')
		}
		b.WriteString(wbiEncode(k))
		b.WriteByte('=')
		b.WriteString(wbiEncode(out.Get(k)))
	}
	sum := md5.Sum([]byte(b.String() + wbiMixinKey(img, sub)))
	out.Set("w_rid", fmt.Sprintf("%x", sum))
	return out
}

func (bl *BL) wbiKeys() (wbiKeys, error) {
	wbiCache.Lock()
	if wbiCache.keys.img != "" && time.Since(wbiCache.keys.at) < 6*time.Hour {
		k := wbiCache.keys
		wbiCache.Unlock()
		return k, nil
	}
	wbiCache.Unlock()

	v, err := doJSON("GET", "https://api.bilibili.com/x/web-interface/nav", bl.GetSESSDATA())
	if err != nil {
		return wbiKeys{}, err
	}
	imgURL := strAt(mapAt(dataOf(v), "wbi_img"), "img_url")
	subURL := strAt(mapAt(dataOf(v), "wbi_img"), "sub_url")
	img := wbiURLStem(imgURL)
	sub := wbiURLStem(subURL)
	if img == "" || sub == "" {
		return wbiKeys{}, fmt.Errorf("nav 没有返回 WBI key")
	}
	k := wbiKeys{img: img, sub: sub, at: time.Now()}
	wbiCache.Lock()
	wbiCache.keys = k
	wbiCache.Unlock()
	return k, nil
}

func wbiURLStem(raw string) string {
	if raw == "" {
		return ""
	}
	if u, err := url.Parse(raw); err == nil && u.Path != "" {
		raw = u.Path
	}
	name := path.Base(raw)
	if i := strings.LastIndexByte(name, '.'); i > 0 {
		name = name[:i]
	}
	return name
}

// signedWBIURL 给 wbi 端点参数签名；取 key 失败时返回错误，让调用方回退
// 到 player/v2，而不是发一个可能命中错误缓存的裸请求。
func (bl *BL) signedWBIURL(base string, values url.Values) (string, error) {
	k, err := bl.wbiKeys()
	if err != nil {
		return "", err
	}
	signed := signWBIAt(values, k.img, k.sub, time.Now().Unix())
	return base + "?" + signed.Encode(), nil
}
