package main

import (
	"changeme/service"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"
)

var LoginStatus = false
var SESSDATA_KEY = "SESSDATA"
var Ticket = ""
var QrCocdeKey = ""

type BL struct {
}

func NewBL() *BL {
	return &BL{}
}

func (bl *BL) HmacSha256(key string, data string) string {
	h := hmac.New(sha256.New, []byte(key))
	h.Write([]byte(data))
	return hex.EncodeToString(h.Sum(nil))
}

func Timestamp2Date(timestamp int64) string {
	date := time.Unix(timestamp, 0)
	year := date.Year()
	month := fmt.Sprintf("%02d", date.Month())
	day := fmt.Sprintf("%02d", date.Day())
	hour := fmt.Sprintf("%02d", date.Hour())
	minute := fmt.Sprintf("%02d", date.Minute())
	second := fmt.Sprintf("%02d", date.Second())
	return fmt.Sprintf("%d-%s-%s %s:%s:%s", year, month, day, hour, minute, second)
}

func (bl *BL) GetLoginStatus() bool {
	return LoginStatus
}

func (bl *BL) SetLoginStatus(status bool) {
	LoginStatus = status
}

func (bl *BL) GetSESSDATA() string {
	val := GetItem(SESSDATA_KEY)
	if val == nil {
		return ""
	}
	return string(GetItem(SESSDATA_KEY).(string))
}

func (bl *BL) SetSESSDATA(data string) {
	SetItem(SESSDATA_KEY, data)
}

type QRCodeResponse struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
	Data    struct {
		QRCodeKey string `json:"qrcode_key"`
		URL       string `json:"url"`
	} `json:"data"`
}

func (bl *BL) GetLoginQRCode() (string, error) {
	fmt.Println("获取登录二维码")
	client := &http.Client{}
	req, err := http.NewRequest("GET", "https://passport.bilibili.com/x/passport-login/web/qrcode/generate", nil)
	if err != nil {
		return "", err
	}
	req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/58.0.3029.110 Safari/537.36")

	resp, err := client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}

	var qrcodeResp QRCodeResponse
	err = json.Unmarshal(body, &qrcodeResp)
	if err != nil {
		return "", err
	}

	if qrcodeResp.Code == 0 {
		QrCocdeKey = qrcodeResp.Data.QRCodeKey
		return qrcodeResp.Data.URL, nil
	}

	return "", fmt.Errorf("failed to get QR code: %s", qrcodeResp.Message)
}

func (bl *BL) GetBiliTicket(csrf string) (string, error) {
	if Ticket != "" {
		return Ticket, nil
	}

	ts := strconv.FormatInt(time.Now().Unix(), 10)
	hexSign := bl.HmacSha256("XgwSnGZ1p", "ts"+ts)

	params := url.Values{}
	params.Add("key_id", "ec02")
	params.Add("hexsign", hexSign)
	params.Add("context[ts]", ts)
	params.Add("csrf", csrf)

	url := "https://api.bilibili.com/bapis/bilibili.api.ticket.v1.Ticket/GenWebTicket"
	req, err := http.NewRequest("POST", url+"?"+params.Encode(), nil)
	if err != nil {
		return "", err
	}

	req.Header.Set("User-Agent", "Mozilla/5.0 (X11; Linux x86_64; rv:109.0) Gecko/20100101 Firefox/115.0")

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("HTTP error! status: %d", resp.StatusCode)
	}

	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}

	var data struct {
		Code int `json:"code"`
		Data struct {
			Ticket string `json:"ticket"`
		} `json:"data"`
	}
	err = json.Unmarshal(body, &data)
	if err != nil {
		return "", err
	}

	if data.Code != 0 {
		return "", fmt.Errorf("API error! code: %d", data.Code)
	}

	Ticket = data.Data.Ticket
	return Ticket, nil
}

type SearchResult struct {
	PictureURL string `json:"picture_url"`
	URL        string `json:"url"`
	Title      string `json:"title"`
	Views      string `json:"views"`
	DanmuCount int    `json:"danmuCount"`
	Author     string `json:"author"`
	Date       string `json:"date"`
}

func (bl *BL) SearchVideo(keyword, order string) (res []SearchResult) {
	var orderType string
	switch order {
	case "click":
		orderType = "click"
	case "update":
		orderType = "pubdate"
	default:
		orderType = "totalrank"
	}

	ticket, err := bl.GetBiliTicket("")
	if err != nil {
		return
	}

	encodedKeyword := url.QueryEscape(keyword)
	url := fmt.Sprintf("https://api.bilibili.com/x/web-interface/wbi/search/type?search_type=video&page=1&page_size=50&order=%s&keyword=%s", orderType, encodedKeyword)

	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return
	}

	req.Header.Set("authority", "api.bilibili.com")
	req.Header.Set("accept", "*/*")
	req.Header.Set("accept-language", "zh-CN,zh;q=0.9")
	req.Header.Set("origin", "https://search.bilibili.com")
	req.Header.Set("referer", "https://search.bilibili.com/video")
	req.Header.Set("user-agent", "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/116.0.0.0 Safari/537.36 Edg/116.0.1938.62")
	req.Header.Set("Cookie", "bili_ticket="+ticket)

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return
	}
	defer resp.Body.Close()

	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return
	}

	var apiResponse struct {
		Data struct {
			Result []struct {
				Pic         string `json:"pic"`
				Bvid        string `json:"bvid"`
				Title       string `json:"title"`
				Play        int    `json:"play"`
				VideoReview int    `json:"video_review"`
				Author      string `json:"author"`
				Pubdate     int64  `json:"pubdate"`
			} `json:"result"`
		} `json:"data"`
	}

	err = json.Unmarshal(body, &apiResponse)
	if err != nil {
		return
	}

	var results []SearchResult
	for _, item := range apiResponse.Data.Result {
		title := item.Title
		// title = title.replace("<em class=\"keyword\">", "").replace("</em>", "")
		title = strings.ReplaceAll(title, "<em class=\"keyword\">", "")
		title = strings.ReplaceAll(title, "</em>", "")
		views := strconv.Itoa(item.Play)
		if item.Play > 1000000 {
			views = fmt.Sprintf("%.1f万", float64(item.Play)/10000)
		}
		date := time.Unix(item.Pubdate, 0).Format("2006-01-02")

		results = append(results, SearchResult{
			PictureURL: fmt.Sprintf("https:%s", item.Pic),
			URL:        fmt.Sprintf("https://www.bilibili.com/video/%s", item.Bvid),
			Title:      title,
			Views:      views,
			DanmuCount: item.VideoReview,
			Author:     item.Author,
			Date:       date,
		})
	}

	return results
}

// ----------- begin - getCList -----------
type VideoInfo struct {
	Bvid      string `json:"bvid"`
	Aid       int    `json:"aid"`
	Title     string `json:"title"`
	Desc      string `json:"desc"`
	Videos    int    `json:"videos"`
	Pic       string `json:"pic"`
	OwnerMid  int    `json:"owner_mid"`
	OwnerName string `json:"owner_name"`
	OwnerFace string `json:"owner_face"`
	Pages     []Page `json:"pages"`
}

type Page struct {
	Cid       int    `json:"cid"`
	Page      int    `json:"page"`
	From      string `json:"from"`
	Part      string `json:"part"`
	Duration  int    `json:"duration"`
	Vid       string `json:"vid"`
	Weblink   string `json:"weblink"`
	Dimension struct {
		Width  int `json:"width"`
		Height int `json:"height"`
		Rotate int `json:"rotate"`
	} `json:"dimension"`
	FirstFrame string `json:"first_frame"`
}

func (bl *BL) GetCList(bvid string) (videoInfo VideoInfo) {
	if bvid == "" {
		return
	}
	url := "https://api.bilibili.com/x/web-interface/view?bvid=" + bvid

	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Get(url)
	if err != nil {
		fmt.Println("Error fetching data:", err)
		return
	}
	defer resp.Body.Close()

	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		fmt.Println("Error reading response body:", err)
		return
	}

	var result struct {
		Code int `json:"code"`
		Data struct {
			Bvid   string `json:"bvid"`
			Aid    int    `json:"aid"`
			Title  string `json:"title"`
			Desc   string `json:"desc"`
			Videos int    `json:"videos"`
			Pic    string `json:"pic"`
			Owner  struct {
				Mid  int    `json:"mid"`
				Name string `json:"name"`
				Face string `json:"face"`
			} `json:"owner"`
			Pages []Page `json:"pages"`
		} `json:"data"`
	}

	err = json.Unmarshal(body, &result)
	if err != nil {
		fmt.Println("Error unmarshalling JSON:", err)
		return
	}

	if result.Code != 0 {
		fmt.Println("未找到相关视频")
		return
	}

	videoInfo = VideoInfo{
		Bvid:      result.Data.Bvid,
		Aid:       result.Data.Aid,
		Title:     result.Data.Title,
		Desc:      result.Data.Desc,
		Videos:    result.Data.Videos,
		Pic:       result.Data.Pic,
		OwnerMid:  result.Data.Owner.Mid,
		OwnerName: result.Data.Owner.Name,
		OwnerFace: result.Data.Owner.Face,
		Pages:     result.Data.Pages,
	}

	return videoInfo
}

// ----------- end - getCList -----------

// ----------- begin - getUrlByCid -----------

type PlayURLInfo struct {
	URL string `json:"url"`
}

func (bl *BL) GetUrlByCid(aid int, cid int) (ret PlayURLInfo) {
	if cid == 0 {
		return
	}
	url := fmt.Sprintf("https://api.bilibili.com/x/player/playurl?avid=%d&cid=%d&qn=0&type=json&platform=html5", aid, cid)

	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Get(url)
	if err != nil {
		return
	}
	defer resp.Body.Close()

	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return
	}

	var result struct {
		Code int `json:"code"`
		Data struct {
			Durl []struct {
				URL string `json:"url"`
			} `json:"durl"`
		} `json:"data"`
	}

	err = json.Unmarshal(body, &result)
	if err != nil {
		return
	}

	if result.Code != 0 {
		return
	}

	if len(result.Data.Durl) == 0 {
		return
	}

	return PlayURLInfo{URL: result.Data.Durl[0].URL}
}

// ----------- end - getUrlByCid -----------

// ----------- begin - getLoginQRCodeStatus -----------
type QRCodeStatusResponse struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
	Data    struct {
		Code   int    `json:"code"`
		Cookie string `json:"cookie"`
	} `json:"data"`
}

func (bl *BL) GetLoginQRCodeStatus() (status bool) {
	if !LoginStatus {
		return false
	}

	options := struct {
		Method string
		URL    string
		Header http.Header
	}{
		Method: "GET",
		URL:    "https://passport.bilibili.com/x/passport-login/web/qrcode/poll?qrcode_key=" + QrCocdeKey,
		Header: http.Header{
			"User-Agent": []string{"Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/58.0.3029.110 Safari/537.36"},
		},
	}

	// 创建一个 channel 来模拟 Promise 的 resolve/reject
	client := &http.Client{}
	req, err := http.NewRequest(options.Method, options.URL, nil)
	if err != nil {
		return
	}

	for key, values := range options.Header {
		for _, value := range values {
			req.Header.Add(key, value)
		}
	}

	resp, err := client.Do(req)
	if err != nil {
		return
	}
	defer resp.Body.Close()

	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return
	}

	var qrCodeStatus QRCodeStatusResponse
	err = json.Unmarshal(body, &qrCodeStatus)
	if err != nil {
		return
	}

	switch qrCodeStatus.Data.Code {
	case 0:
		cookie := strings.Join(resp.Header["Set-Cookie"], "; ")
		fmt.Println("二维码扫描成功，Cookie:", cookie)
		bl.SetSESSDATA(cookie)
		return true
		// 在实际应用中，你可能需要将 cookie 存储到某个地方，这里只是打印出来
	case 86038:
		fmt.Println("二维码已失效")
		time.Sleep(5 * time.Second)
		return false
		// 在这里调用 getLoginQRCode 函数（假设它已经定义）
		// getLoginQRCode()
	default:
		fmt.Println("二维码扫描中...")
		time.Sleep(2 * time.Second)
		return false
	}
}

// ----------- end - getLoginQRCodeStatus -----------

// ----------- begin - getBLUserInfo -----------
type UserInfo struct {
	Uname string `json:"uname"`
	Face  string `json:"face"`
	Mid   int    `json:"mid"`
}

func (bl *BL) GetBLUserInfo() *UserInfo {
	// getBLUserInfo 是一个 Go 函数，用于获取 Bilibili 用户信息
	cookie := bl.GetSESSDATA()
	if cookie == "" {
		return nil
	}

	url := "https://api.bilibili.com/x/web-interface/nav"
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil
	}

	req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/58.0.3029.110 Safari/537.36")
	req.Header.Set("Cookie", cookie)

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return nil
	}
	defer resp.Body.Close()

	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return nil
	}

	var apiResponse struct {
		Code int      `json:"code"`
		Data UserInfo `json:"data"`
	}

	err = json.Unmarshal(body, &apiResponse)
	if err != nil {
		return nil
	}

	if apiResponse.Code != 0 {
		return nil
	}

	userInfo := &UserInfo{
		Uname: apiResponse.Data.Uname,
		Face:  apiResponse.Data.Face,
		Mid:   apiResponse.Data.Mid,
	}

	SetItem("uname", userInfo.Uname)
	SetItem("face", userInfo.Face)
	SetItem("mid", userInfo.Mid)

	return userInfo
}

// ----------- end - getBLUserInfo -----------

// ----------- begin - getBLFeedList -----------
type FeedList struct {
	Items   []interface{} `json:"items"`
	HasMore bool          `json:"has_more"`
	Offset  string        `json:"offset"`
}

func (bl *BL) GetBLFeedList(offset string) (*FeedList, error) {
	cookie := bl.GetSESSDATA()
	if cookie == "" {
		return &FeedList{
			Items:   []interface{}{},
			HasMore: false,
			Offset:  "",
		}, nil
	}

	apiUrl := "https://api.bilibili.com/x/polymer/web-dynamic/v1/feed/all"
	params := url.Values{}
	params.Add("type", "video")
	if len(offset) > 0 {
		params.Add("offset", offset)
	}

	req, err := http.NewRequest("GET", apiUrl+"?"+params.Encode(), nil)
	if err != nil {
		return nil, err
	}

	req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/58.0.3029.110 Safari/537.36")
	req.Header.Set("Cookie", cookie)

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	var apiResponse struct {
		Code int `json:"code"`
		Data struct {
			Items   []interface{} `json:"items"`
			HasMore bool          `json:"has_more"`
			Offset  string        `json:"offset"`
		} `json:"data"`
	}

	err = json.Unmarshal(body, &apiResponse)
	if err != nil {
		return nil, err
	}

	if apiResponse.Code != 0 {
		return nil, errors.New("API returned non-zero code")
	}

	feedList := &FeedList{
		Items:   apiResponse.Data.Items,
		HasMore: apiResponse.Data.HasMore,
		Offset:  apiResponse.Data.Offset,
	}

	return feedList, nil
}

// ----------- end - getBLFeedList -----------

// ----------- begin - getBLRCMDList -----------
type RCMDList struct {
	Items   []interface{} `json:"items"`
	HasMore bool          `json:"has_more"`
	Page    int           `json:"page"`
}

func (bl *BL) GetBLRCMDList(page int) (*RCMDList, error) {
	cookie := bl.GetSESSDATA()
	if cookie == "" {
		return &RCMDList{
			Items:   []interface{}{},
			HasMore: false,
			Page:    page,
		}, nil
	}

	apiUrl := "https://api.bilibili.com/x/web-interface/wbi/index/top/feed/rcmd"
	params := url.Values{}
	// 由于 theParams 在原始 JavaScript 代码中为空，这里不需要添加任何参数

	req, err := http.NewRequest("GET", apiUrl+"?"+params.Encode(), nil)
	if err != nil {
		return nil, err
	}

	req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/58.0.3029.110 Safari/537.36")
	req.Header.Set("Cookie", cookie)

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	var apiResponse struct {
		Code int `json:"code"`
		Data struct {
			Item []interface{} `json:"item"`
		} `json:"data"`
	}

	err = json.Unmarshal(body, &apiResponse)
	if err != nil {
		return nil, err
	}

	if apiResponse.Code != 0 {
		return nil, errors.New("API returned non-zero code")
	}

	rcmdList := &RCMDList{
		Items:   apiResponse.Data.Item,
		HasMore: page < 10,
		Page:    page,
	}

	return rcmdList, nil
}

// ----------- end - getBLRCMDList -----------

// ----------- begin - getBLFavFolderList -----------
type FolderListResponse struct {
	Code int `json:"code"`
	Data struct {
		List []interface{} `json:"list"`
	} `json:"data"`
}

func (bl *BL) GetBLFavFolderList() ([]interface{}, error) {
	cookie := bl.GetSESSDATA()
	if cookie == "" {
		return nil, nil
	}

	mid, _ := service.NumberToString(GetItem("mid"))
	baseURL := "https://api.bilibili.com/x/v3/fav/folder/created/list-all"
	params := url.Values{}
	params.Add("up_mid", mid) // Replace with actual logic to get mid
	params.Add("type", "2")

	client := &http.Client{}
	req, err := http.NewRequest("GET", baseURL+"?"+params.Encode(), nil)
	if err != nil {
		return nil, err
	}

	req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/58.0.3029.110 Safari/537.36")
	req.Header.Set("Cookie", cookie)

	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	var folderListResponse FolderListResponse
	err = json.Unmarshal(body, &folderListResponse)
	if err != nil {
		return nil, err
	}

	if folderListResponse.Code != 0 {
		return nil, errors.New("failed to fetch folder list")
	}

	return folderListResponse.Data.List, nil
}

// ----------- end - getBLFavFolderList -----------

// ----------- begin - getBLFavFolderListDetail -----------
type FolderDetailResponse struct {
	Code int `json:"code"`
	Data struct {
		Medias []interface{} `json:"medias"`
	} `json:"data"`
}

func (bl *BL) GetBLFavFolderListDetail(fid int, page int) ([]interface{}, error) {
	cookie := bl.GetSESSDATA()
	if cookie == "" {
		return nil, nil
	}

	baseURL := "https://api.bilibili.com/x/v3/fav/resource/list"
	params := url.Values{}
	params.Add("media_id", fmt.Sprintf("%d", fid))
	params.Add("type", "0")
	params.Add("ps", "21")
	params.Add("pn", fmt.Sprintf("%d", page))

	client := &http.Client{}
	req, err := http.NewRequest("GET", baseURL+"?"+params.Encode(), nil)
	if err != nil {
		return nil, err
	}

	req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/58.0.3029.110 Safari/537.36")
	req.Header.Set("Cookie", cookie)

	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	var folderDetailResponse FolderDetailResponse
	err = json.Unmarshal(body, &folderDetailResponse)
	if err != nil {
		return nil, err
	}

	if folderDetailResponse.Code != 0 {
		return nil, errors.New("failed to fetch folder detail")
	}

	return folderDetailResponse.Data.Medias, nil
}

// ----------- end - getBLFavFolderListDetail -----------

// ----------- begin - likeVideo -----------
func (bl *BL) LikeVideo(bid string, like int) (bool, error) {
	cookie := bl.GetSESSDATA()
	if cookie == "" {
		return false, errors.New("未登录")
	}

	ticket, err := bl.GetBiliTicket("")
	if err != nil {
		return false, err
	}

	if bid == "" {
		return false, errors.New("未选择作品")
	}

	// 从cookie中获取bili_jct作为csrf
	csrfStart := strings.Index(cookie, "bili_jct=")
	if csrfStart == -1 {
		return false, errors.New("无法获取csrf")
	}
	csrfStart += 9 // "bili_jct="的长度
	csrfEnd := strings.Index(cookie[csrfStart:], ";")
	if csrfEnd == -1 {
		csrfEnd = len(cookie[csrfStart:])
	}
	csrfEnd = csrfEnd + csrfStart
	csrf := cookie[csrfStart:csrfEnd]

	baseURL := "https://api.bilibili.com/x/web-interface/archive/like"
	params := url.Values{}
	// 去掉bid中的开头的BV
	bid = strings.Replace(bid, "BV", "", 1)

	fmt.Println(bid, like, csrf)
	params.Add("bvid", bid)
	params.Add("like", fmt.Sprintf("%d", like)) // 1: 点赞, 2: 取消点赞
	params.Add("csrf", csrf)

	client := &http.Client{}
	req, err := http.NewRequest("POST", baseURL+"?"+params.Encode(), nil)
	if err != nil {
		return false, err
	}

	req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/58.0.3029.110 Safari/537.36")
	req.Header.Set("Cookie", cookie)
	req.Header.Set("bili_ticket", ticket)

	resp, err := client.Do(req)
	if err != nil {
		return false, err
	}
	defer resp.Body.Close()

	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return false, err
	}

	var response struct {
		Code    int    `json:"code"`
		Message string `json:"message"`
	}
	err = json.Unmarshal(body, &response)
	fmt.Println(response)
	if err != nil {
		return false, err
	}

	if response.Code != 0 {
		return false, errors.New(response.Message)
	}

	return true, nil
}

// ----------- end - likeVideo -----------

// ----------- begin - hasLiked -----------
func (bl *BL) HasLiked(bid string) (bool, error) {
	cookie := bl.GetSESSDATA()
	if cookie == "" {
		return false, errors.New("未登录")
	}

	if bid == "" {
		return false, errors.New("未选择作品")
	}

	baseURL := "https://api.bilibili.com/x/web-interface/archive/has/like"
	params := url.Values{}
	// 去掉bid中的开头的BV
	bid = strings.Replace(bid, "BV", "", 1)
	params.Add("bvid", bid)

	client := &http.Client{}
	req, err := http.NewRequest("GET", baseURL+"?"+params.Encode(), nil)
	if err != nil {
		return false, err
	}

	req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/58.0.3029.110 Safari/537.36")
	req.Header.Set("Cookie", cookie)

	resp, err := client.Do(req)
	if err != nil {
		return false, err
	}
	defer resp.Body.Close()

	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return false, err
	}

	var response struct {
		Code    int    `json:"code"`
		Data    int    `json:"data"`
		Message string `json:"message"`
	}
	err = json.Unmarshal(body, &response)
	if err != nil {
		return false, err
	}

	if response.Code != 0 {
		return false, errors.New(response.Message)
	}

	return response.Data == 1, nil
}

// ----------- end - hasLiked -----------

// ----------- begin - coinVideo -----------
func (bl *BL) CoinVideo(bid string, multiply int) (bool, error) {
	cookie := bl.GetSESSDATA()
	if cookie == "" {
		return false, errors.New("未登录")
	}

	if bid == "" {
		return false, errors.New("未选择作品")
	}

	// 从cookie中获取bili_jct作为csrf
	csrfStart := strings.Index(cookie, "bili_jct=")
	if csrfStart == -1 {
		return false, errors.New("无法获取csrf")
	}
	csrfStart += 9 // "bili_jct="的长度
	csrfEnd := strings.Index(cookie[csrfStart:], ";")
	if csrfEnd == -1 {
		csrfEnd = len(cookie[csrfStart:])
	}
	csrfEnd = csrfEnd + csrfStart
	csrf := cookie[csrfStart:csrfEnd]

	baseURL := "https://api.bilibili.com/x/web-interface/coin/add"
	params := url.Values{}
	// 去掉bid中的开头的BV
	bid = strings.Replace(bid, "BV", "", 1)
	params.Add("bvid", bid)
	params.Add("multiply", fmt.Sprintf("%d", multiply)) // 投币数量
	params.Add("csrf", csrf)
	params.Add("select_like", "0") // 是否同时点赞，0：不点赞，1：同时点赞

	client := &http.Client{}
	req, err := http.NewRequest("POST", baseURL+"?"+params.Encode(), nil)
	if err != nil {
		return false, err
	}

	req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/58.0.3029.110 Safari/537.36")
	req.Header.Set("Cookie", cookie)

	resp, err := client.Do(req)
	if err != nil {
		return false, err
	}
	defer resp.Body.Close()

	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return false, err
	}

	var response struct {
		Code    int    `json:"code"`
		Message string `json:"message"`
	}
	err = json.Unmarshal(body, &response)
	if err != nil {
		return false, err
	}

	if response.Code != 0 {
		return false, errors.New(response.Message)
	}

	return true, nil
}

// ----------- end - coinVideo -----------

// ----------- begin - hasCoin -----------
func (bl *BL) HasCoin(bid string) (int, error) {
	cookie := bl.GetSESSDATA()
	if cookie == "" {
		return 0, errors.New("未登录")
	}

	if bid == "" {
		return 0, errors.New("未选择作品")
	}

	baseURL := "https://api.bilibili.com/x/web-interface/archive/coins"
	params := url.Values{}
	// 去掉bid中的开头的BV
	bid = strings.Replace(bid, "BV", "", 1)
	params.Add("bvid", bid)

	client := &http.Client{}
	req, err := http.NewRequest("GET", baseURL+"?"+params.Encode(), nil)
	if err != nil {
		return 0, err
	}

	req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/58.0.3029.110 Safari/537.36")
	req.Header.Set("Cookie", cookie)

	resp, err := client.Do(req)
	if err != nil {
		return 0, err
	}
	defer resp.Body.Close()

	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return 0, err
	}

	var response struct {
		Code    int    `json:"code"`
		Message string `json:"message"`
		Data    struct {
			Multiply int `json:"multiply"`
		} `json:"data"`
	}
	err = json.Unmarshal(body, &response)
	if err != nil {
		return 0, err
	}

	if response.Code != 0 {
		return 0, errors.New(response.Message)
	}

	return response.Data.Multiply, nil
}

// ----------- end - hasCoin -----------
