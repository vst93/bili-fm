package main

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"
)

var LoginStatus = false;
var SESSDATA_KEY = "SESSDATA"
var Ticket = "";

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
  return LoginStatus;
}

func (bl *BL) SetLoginStatus(status bool) {
  LoginStatus = status;
}

func (bl *BL) GetSESSDATA() string{
	return string(GetItem(SESSDATA_KEY).([]byte))
}

func (bl *BL) SetSESSDATA(data string) {
	SetItem(SESSDATA_KEY, []byte(data))
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
		return qrcodeResp.Data.URL, nil
	}

	return "", fmt.Errorf("failed to get QR code: %s", qrcodeResp.Message)
}


func (bl *BL)  GetBiliTicket(csrf string) (string, error) {
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
		Code int    `json:"code"`
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
	PictureURL  string `json:"picture_url"`
	URL         string `json:"url"`
	Title       string `json:"title"`
	Views       string `json:"views"`
	DanmuCount  int    `json:"danmuCount"`
	Author      string `json:"author"`
	Date        string `json:"date"`
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
				Pic       string `json:"pic"`
				Bvid      string `json:"bvid"`
				Title     string `json:"title"`
				Play      int    `json:"play"`
				VideoReview int `json:"video_review"`
				Author    string `json:"author"`
				Pubdate   int64  `json:"pubdate"`
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
		title =strings.ReplaceAll(title, "<em class=\"keyword\">", "")
		title =strings.ReplaceAll(title, "</em>", "")
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