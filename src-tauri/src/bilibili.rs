//! B 站 API 客户端 — service/bl.go 的 Rust 移植。
//!
//! 行为与 Go 原版保持一致: 相同的端点、headers、Cookie、响应解析与
//! JSON 字段名。任务规格要求的修复:
//! - `get_login_qrcode_status` 不再 `time.Sleep` 阻塞 (改为立即返回, 由前端轮询)
//! - `get_sessdata` 只读取一次 dkv (Go 版有双重 `GetItem` 浪费)
//! - 版本比较使用正确的 semver 序, 不用字符串 `>` (如 "1.9.5" > "1.10.0")

use std::cmp::Ordering;
use std::io::Read;
use std::sync::atomic::{AtomicBool, Ordering as AtomicOrdering};
use std::sync::{Mutex, OnceLock};
use std::time::Duration;

use flate2::read::{DeflateDecoder, GzDecoder};
use hmac::{Hmac, Mac};
use reqwest::header::SET_COOKIE;
use serde::{Deserialize, Serialize};
use serde_json::{json, Value};
use sha2::Sha256;

use crate::dkv;

// ---------------------------------------------------------------------------
// 常量 (与 Go 原版逐字一致)
// ---------------------------------------------------------------------------

/// 图片代理端口 (与旧版 service/config.go IMAGE_PROXY_PROT 一致)
pub const IMAGE_PROXY_PORT: u16 = 4654;

const SESSDATA_KEY: &str = "SESSDATA";

/// bl.go 中绝大多数请求使用的 Chrome UA
const UA_CHROME: &str = "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/58.0.3029.110 Safari/537.36";
/// 获取 bili_ticket 使用的 Firefox UA
const UA_FIREFOX: &str = "Mozilla/5.0 (X11; Linux x86_64; rv:109.0) Gecko/20100101 Firefox/115.0";
/// FetchImage / 图片代理使用的 Chrome 131 UA
const UA_CHROME_131: &str = "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/131.0.0.0 Safari/537.36";

// ---------------------------------------------------------------------------
// 全局状态 (对应 Go 的包级变量 LoginStatus / Ticket / QrCocdeKey)
// ---------------------------------------------------------------------------

pub static LOGIN_STATUS: AtomicBool = AtomicBool::new(false);
static TICKET: Mutex<Option<String>> = Mutex::new(None);
static QRCODE_KEY: Mutex<String> = Mutex::new(String::new());

// ---------------------------------------------------------------------------
// 共享 HTTP 客户端 (对应 Go 的 fetchImageClient: 连接池 + 超时 + 跳过证书校验)
// ---------------------------------------------------------------------------

fn client() -> &'static reqwest::Client {
    static CLIENT: OnceLock<reqwest::Client> = OnceLock::new();
    CLIENT.get_or_init(|| {
        reqwest::Client::builder()
            .timeout(Duration::from_secs(30))
            .connect_timeout(Duration::from_secs(15))
            .pool_idle_timeout(Duration::from_secs(90))
            .danger_accept_invalid_certs(true) // 对应 Go InsecureSkipVerify
            .build()
            .expect("failed to build reqwest client")
    })
}

// ---------------------------------------------------------------------------
// 小工具
// ---------------------------------------------------------------------------

/// Go url.QueryEscape 的等价物: 编码除 unreserved (-_.~ 字母数字) 外的所有字符,
/// 空格编码为 '+' (与 Go 一致)。
fn urlencode(s: &str) -> String {
    const QUERY_ENCODE_SET: &percent_encoding::AsciiSet = &percent_encoding::CONTROLS
        .add(b' ')
        .add(b'"')
        .add(b'#')
        .add(b'%')
        .add(b'&')
        .add(b'\'')
        .add(b'+')
        .add(b',')
        .add(b'/')
        .add(b':')
        .add(b';')
        .add(b'<')
        .add(b'=')
        .add(b'>')
        .add(b'?')
        .add(b'@')
        .add(b'[')
        .add(b'\\')
        .add(b']')
        .add(b'^')
        .add(b'`')
        .add(b'{')
        .add(b'|')
        .add(b'}');
    percent_encoding::utf8_percent_encode(s, QUERY_ENCODE_SET)
        .to_string()
        .replace("%20", "+")
}

/// HmacSha256 (Go: hmac.New(sha256.New, key) + hex)
pub fn hmac_sha256_hex(key: &str, data: &str) -> String {
    let mut mac = Hmac::<Sha256>::new_from_slice(key.as_bytes()).expect("hmac accepts any key");
    mac.update(data.as_bytes());
    hex_encode(mac.finalize().into_bytes().as_slice())
}

fn hex_encode(bytes: &[u8]) -> String {
    let mut out = String::with_capacity(bytes.len() * 2);
    for b in bytes {
        out.push_str(&format!("{b:02x}"));
    }
    out
}

/// Go time.Unix(ts, 0).Format("2006-01-02") — 本地时区
fn unix_to_date(ts: i64) -> String {
    match chrono::DateTime::from_timestamp(ts, 0) {
        Some(dt) => dt
            .with_timezone(&chrono::Local)
            .format("%Y-%m-%d")
            .to_string(),
        None => String::new(),
    }
}

/// Go: play > 1000000 时格式化为 "%.1f万"
fn format_views(play: i64) -> String {
    if play > 1_000_000 {
        format!("{:.1}万", play as f64 / 10000.0)
    } else {
        play.to_string()
    }
}

/// 从 Cookie 中提取 bili_jct 作为 csrf (与 Go 各方法中重复的逻辑一致)
fn extract_csrf(cookie: &str) -> Result<String, String> {
    const MARKER: &str = "bili_jct=";
    let Some(rel) = cookie.find(MARKER) else {
        return Err("无法获取csrf".to_string());
    };
    let rest = &cookie[rel + MARKER.len()..];
    let end = rest.find(';').unwrap_or(rest.len());
    Ok(rest[..end].to_string())
}

/// Go strings.Replace(bid, "BV", "", 1) — 替换第一处出现的 "BV"
fn bid_without_prefix(bid: &str) -> String {
    bid.replacen("BV", "", 1)
}

// 标准响应包裹 {code, message, data} 的取值助手
fn code_of(v: &Value) -> i64 {
    v.get("code").and_then(|c| c.as_i64()).unwrap_or(-1)
}
fn message_of(v: &Value) -> String {
    v.get("message")
        .and_then(|m| m.as_str())
        .unwrap_or("")
        .to_string()
}
fn data_of(v: &Value) -> Value {
    v.get("data").cloned().unwrap_or(Value::Null)
}
fn arr_of(data: &Value, key: &str) -> Vec<Value> {
    data.get(key)
        .and_then(|x| x.as_array())
        .cloned()
        .unwrap_or_default()
}
fn str_of(data: &Value, key: &str) -> String {
    data.get(key)
        .and_then(|x| x.as_str())
        .unwrap_or("")
        .to_string()
}
fn int_of(data: &Value, key: &str) -> i64 {
    data.get(key).and_then(|x| x.as_i64()).unwrap_or(0)
}

/// 校验 code==0 并返回 data；失败时返回 Go 原版的错误文案。
fn check_code(v: &Value, fallback: &str) -> Result<Value, String> {
    let code = code_of(v);
    if code != 0 {
        let msg = message_of(v);
        if msg.is_empty() {
            return Err(fallback.to_string());
        }
        return Err(msg);
    }
    Ok(data_of(v))
}

// ---------------------------------------------------------------------------
// 通用请求
// ---------------------------------------------------------------------------

/// GET 请求并解析 JSON。cookie 为空时自动不携带 (对应 Go 的条件 Set)。
async fn get_json(
    url: &str,
    cookie: Option<&str>,
    headers: &[(&str, &str)],
) -> Result<Value, String> {
    let mut req = client().get(url).header("User-Agent", UA_CHROME);
    if let Some(c) = cookie {
        if !c.is_empty() {
            req = req.header("Cookie", c);
        }
    }
    for (k, v) in headers {
        req = req.header(*k, *v);
    }
    let resp = req.send().await.map_err(|e| format!("网络请求失败: {e}"))?;
    let text = resp
        .text()
        .await
        .map_err(|e| format!("读取响应失败: {e}"))?;
    serde_json::from_str(&text).map_err(|e| format!("JSON 解析失败: {e}, body: {text}"))
}

/// POST 请求 (参数在 URL 中，对应 Go 的 POST baseURL+"?"+params) 并解析 JSON。
async fn post_json(
    url: &str,
    cookie: Option<&str>,
    headers: &[(&str, &str)],
) -> Result<Value, String> {
    let mut req = client().post(url).header("User-Agent", UA_CHROME);
    if let Some(c) = cookie {
        if !c.is_empty() {
            req = req.header("Cookie", c);
        }
    }
    for (k, v) in headers {
        req = req.header(*k, *v);
    }
    let resp = req.send().await.map_err(|e| format!("网络请求失败: {e}"))?;
    let text = resp
        .text()
        .await
        .map_err(|e| format!("读取响应失败: {e}"))?;
    serde_json::from_str(&text).map_err(|e| format!("JSON 解析失败: {e}, body: {text}"))
}

// ---------------------------------------------------------------------------
// 登录态 (dkv 兼容存储)
// ---------------------------------------------------------------------------

/// 读取 SESSDATA (只读一次 dkv — 修复 Go 版双重 GetItem 的浪费)
pub fn get_sessdata() -> String {
    dkv::get_string(SESSDATA_KEY)
}

pub fn set_sessdata(data: &str) {
    if let Err(e) = dkv::set_item(SESSDATA_KEY, &json!(data)) {
        eprintln!("dkv: set SESSDATA failed: {e}");
    }
}

// ---------------------------------------------------------------------------
// 登录 QR 码
// ---------------------------------------------------------------------------

/// 生成登录二维码 (对应 Go GetLoginQRCode)
pub async fn get_login_qrcode() -> Result<String, String> {
    let resp = client()
        .get("https://passport.bilibili.com/x/passport-login/web/qrcode/generate")
        .header("User-Agent", UA_CHROME)
        .send()
        .await
        .map_err(|e| format!("网络请求失败: {e}"))?;
    let text = resp
        .text()
        .await
        .map_err(|e| format!("读取响应失败: {e}"))?;
    let v: Value =
        serde_json::from_str(&text).map_err(|e| format!("JSON 解析失败: {e}, body: {text}"))?;
    if code_of(&v) != 0 {
        return Err(format!("failed to get QR code: {}", message_of(&v)));
    }
    let key = v
        .pointer("/data/qrcode_key")
        .and_then(|k| k.as_str())
        .unwrap_or("")
        .to_string();
    let url = v
        .pointer("/data/url")
        .and_then(|u| u.as_str())
        .unwrap_or("")
        .to_string();
    *QRCODE_KEY.lock().unwrap() = key;
    Ok(url)
}

pub fn get_login_status() -> bool {
    LOGIN_STATUS.load(AtomicOrdering::SeqCst)
}

pub fn set_login_status(status: bool) {
    LOGIN_STATUS.store(status, AtomicOrdering::SeqCst);
}

/// 轮询二维码扫描状态 (对应 Go GetLoginQRCodeStatus)。
///
/// 修复: Go 版在「二维码已失效」时 `time.Sleep(5s)`、扫描中时 `time.Sleep(2s)`
/// 阻塞调用方; 新版立即返回，由前端自行控制轮询节奏。
pub async fn get_login_qrcode_status() -> bool {
    if !LOGIN_STATUS.load(AtomicOrdering::SeqCst) {
        return false;
    }
    let key = QRCODE_KEY.lock().unwrap().clone();
    let url = format!("https://passport.bilibili.com/x/passport-login/web/qrcode/poll?qrcode_key={key}");
    let Ok(resp) = client()
        .get(&url)
        .header("User-Agent", UA_CHROME)
        .send()
        .await
    else {
        return false;
    };
    // 先取 Set-Cookie (resp 随后被 text() 消费)
    let set_cookies: Vec<String> = resp
        .headers()
        .get_all(SET_COOKIE)
        .iter()
        .filter_map(|v| v.to_str().ok())
        .map(|s| s.to_string())
        .collect();
    let Ok(text) = resp.text().await else {
        return false;
    };
    let Ok(v) = serde_json::from_str::<Value>(&text) else {
        return false;
    };
    match v.pointer("/data/code").and_then(|c| c.as_i64()).unwrap_or(-1) {
        0 => {
            // 扫码成功: 拼接 Set-Cookie 存入 dkv (对应 Go strings.Join + SetSESSDATA)
            let cookie = set_cookies.join("; ");
            println!("二维码扫描成功");
            set_sessdata(&cookie);
            true
        }
        86038 => {
            println!("二维码已失效");
            false
        }
        _ => {
            println!("二维码扫描中...");
            false
        }
    }
}

// ---------------------------------------------------------------------------
// bili_ticket (对应 Go GetBiliTicket，带全局缓存)
// ---------------------------------------------------------------------------

pub async fn get_bili_ticket(csrf: &str) -> Result<String, String> {
    if let Some(t) = TICKET.lock().unwrap().clone() {
        return Ok(t);
    }
    let ts = std::time::SystemTime::now()
        .duration_since(std::time::UNIX_EPOCH)
        .map(|d| d.as_secs())
        .unwrap_or(0)
        .to_string();
    let hex_sign = hmac_sha256_hex("XgwSnGZ1p", &format!("ts{ts}"));
    // url.Values.Encode 按 key 排序: context[ts], csrf, hexsign, key_id
    let url = format!(
        "https://api.bilibili.com/bapis/bilibili.api.ticket.v1.Ticket/GenWebTicket?context%5Bts%5D={ts}&csrf={csrf}&hexsign={hex_sign}&key_id=ec02"
    );
    let resp = client()
        .post(&url)
        .header("User-Agent", UA_FIREFOX)
        .send()
        .await
        .map_err(|e| format!("网络请求失败: {e}"))?;
    if !resp.status().is_success() {
        return Err(format!("HTTP error! status: {}", resp.status().as_u16()));
    }
    let text = resp
        .text()
        .await
        .map_err(|e| format!("读取响应失败: {e}"))?;
    let v: Value = serde_json::from_str(&text).map_err(|e| format!("JSON 解析失败: {e}"))?;
    let code = code_of(&v);
    if code != 0 {
        return Err(format!("API error! code: {code}"));
    }
    let ticket = v
        .pointer("/data/ticket")
        .and_then(|t| t.as_str())
        .unwrap_or("")
        .to_string();
    if ticket.is_empty() {
        return Err("API error! empty ticket".to_string());
    }
    *TICKET.lock().unwrap() = Some(ticket.clone());
    Ok(ticket)
}

// ---------------------------------------------------------------------------
// 数据模型 (JSON 字段名与 Go struct tag 逐一对应)
// ---------------------------------------------------------------------------

#[derive(Debug, Clone, Default, Serialize, Deserialize)]
pub struct SearchResult {
    #[serde(rename = "picture_url")]
    pub picture_url: String,
    pub url: String,
    pub title: String,
    pub views: String,
    #[serde(rename = "danmuCount")]
    pub danmu_count: i64,
    pub author: String,
    pub date: String,
}

#[derive(Debug, Clone, Default, Serialize, Deserialize)]
pub struct Dimension {
    pub width: i64,
    pub height: i64,
    pub rotate: i64,
}

fn de_default<'de, D, T>(d: D) -> Result<T, D::Error>
where
    D: serde::Deserializer<'de>,
    T: Deserialize<'de> + Default,
{
    let opt = Option::<T>::deserialize(d)?;
    Ok(opt.unwrap_or_default())
}

#[derive(Debug, Clone, Default, Serialize, Deserialize)]
pub struct Page {
    pub cid: i64,
    pub page: i64,
    pub from: String,
    pub part: String,
    pub duration: i64,
    pub vid: String,
    pub weblink: String,
    #[serde(default, deserialize_with = "de_default")]
    pub dimension: Dimension,
    #[serde(rename = "first_frame")]
    pub first_frame: String,
}

#[derive(Debug, Clone, Default, Serialize, Deserialize)]
pub struct VideoInfo {
    pub bvid: String,
    pub aid: i64,
    pub title: String,
    pub desc: String,
    pub videos: i64,
    pub pic: String,
    #[serde(rename = "owner_mid")]
    pub owner_mid: i64,
    #[serde(rename = "owner_name")]
    pub owner_name: String,
    #[serde(rename = "owner_face")]
    pub owner_face: String,
    #[serde(default)]
    pub pages: Vec<Page>,
    pub cid: i64,
}

#[derive(Debug, Clone, Default, Serialize, Deserialize)]
pub struct PlayURLInfo {
    pub url: String,
}

#[derive(Debug, Clone, Default, Serialize, Deserialize)]
pub struct UserInfo {
    pub uname: String,
    pub face: String,
    pub mid: i64,
}

#[derive(Debug, Clone, Default, Serialize, Deserialize)]
pub struct FeedList {
    #[serde(default)]
    pub items: Vec<Value>,
    #[serde(rename = "has_more")]
    pub has_more: bool,
    pub offset: String,
}

#[derive(Debug, Clone, Default, Serialize, Deserialize)]
pub struct RCMDList {
    #[serde(default)]
    pub items: Vec<Value>,
    #[serde(rename = "has_more")]
    pub has_more: bool,
    pub page: i32,
}

#[derive(Debug, Clone, Default, Serialize, Deserialize)]
pub struct HistoryCursor {
    pub max: i32,
    #[serde(rename = "view_at")]
    pub view_at: i32,
    pub business: String,
}

#[derive(Debug, Clone, Default, Serialize, Deserialize)]
pub struct HistoryList {
    #[serde(default)]
    pub list: Vec<Value>,
    #[serde(default)]
    pub cursor: HistoryCursor,
}

#[derive(Debug, Clone, Default, Serialize, Deserialize)]
pub struct PopularList {
    #[serde(default)]
    pub items: Vec<Value>,
    #[serde(rename = "has_more")]
    pub has_more: bool,
    #[serde(rename = "no_more")]
    pub no_more: bool,
    pub page: i32,
}

#[derive(Debug, Clone, Default, Serialize, Deserialize)]
pub struct Stat {
    pub view: i64,
}

#[derive(Debug, Clone, Default, Serialize, Deserialize)]
pub struct SeriesArchive {
    pub aid: i64,
    pub bvid: String,
    pub title: String,
    pub pubdate: i64,
    pub duration: i64,
    pub pic: String,
    #[serde(default, deserialize_with = "de_default")]
    pub stat: Stat,
}

#[derive(Debug, Clone, Default, Serialize, Deserialize)]
pub struct FollowStatus {
    #[serde(rename = "is_following")]
    pub is_following: bool,
    pub follower: i64,
}

#[derive(Debug, Clone, Default, Serialize, Deserialize)]
pub struct DanmakuItem {
    pub content: String,
    pub time: f64,
    #[serde(rename = "type")]
    pub kind: i64,
    #[serde(rename = "fontSize")]
    pub font_size: i64,
    pub color: i64,
    #[serde(rename = "sendTime")]
    pub send_time: i64,
    pub dmid: i64,
}

#[derive(Debug, Clone, Default, Serialize, Deserialize)]
pub struct DanmakuList {
    #[serde(default)]
    pub items: Vec<DanmakuItem>,
}

#[derive(Debug, Clone, Default, Serialize, Deserialize)]
pub struct ReplyContent {
    pub message: String,
}

#[derive(Debug, Clone, Default, Serialize, Deserialize)]
pub struct ReplyItem {
    pub rpid: i64,
    pub oid: i64,
    #[serde(rename = "type")]
    pub kind: i64,
    pub mid: i64,
    pub content: ReplyContent,
    pub ctime: i64,
    pub like: i64,
    pub action: i64,
    pub member: Value,
    #[serde(default, deserialize_with = "de_default")]
    pub replies: Vec<ReplyItem>,
    pub root: i64,
    pub parent: i64,
}

#[derive(Debug, Clone, Default, Serialize, Deserialize)]
pub struct ReplyList {
    #[serde(default)]
    pub items: Vec<ReplyItem>,
    #[serde(rename = "has_more")]
    pub has_more: bool,
    pub next: i32,
    #[serde(rename = "total_count")]
    pub total_count: i32,
}

#[derive(Debug, Clone, Default, Serialize, Deserialize)]
pub struct AppVersion {
    pub version: String,
    pub build: i32,
}

#[derive(Debug, Clone, Default, Serialize, Deserialize)]
pub struct UpdateResult {
    #[serde(rename = "hasUpdate")]
    pub has_update: bool,
    #[serde(rename = "latestVersion")]
    pub latest_version: String,
    #[serde(rename = "downloadUrl")]
    pub download_url: String,
    #[serde(rename = "isLatest")]
    pub is_latest: bool,
    pub error: String,
}

// ---------------------------------------------------------------------------
// 搜索
// ---------------------------------------------------------------------------

/// 搜索视频。B 站偶尔会返回只有 v_voucher 的风控响应，因此在两个
/// 搜索端点和登录/匿名 Cookie 之间进行有限回退。
pub async fn search_video(keyword: &str, order: &str) -> Result<Vec<SearchResult>, String> {
    let order_type = match order {
        "click" => "click",
        "update" => "pubdate",
        _ => "totalrank",
    };
    let query = format!(
        "search_type=video&page=1&page_size=50&order={order_type}&keyword={}",
        urlencode(keyword)
    );
    let urls = [
        format!("https://api.bilibili.com/x/web-interface/search/type?{query}"),
        format!("https://api.bilibili.com/x/web-interface/wbi/search/type?{query}"),
    ];
    let sessdata = get_sessdata();
    let cookies = if sessdata.is_empty() {
        vec![None]
    } else {
        vec![Some(sessdata.as_str()), None]
    };
    let mut last_error = "搜索接口未返回结果".to_string();

    for url in &urls {
        for cookie in &cookies {
            let response = get_json(
                url,
                *cookie,
                &[
                    ("authority", "api.bilibili.com"),
                    ("accept", "*/*"),
                    ("accept-language", "zh-CN,zh;q=0.9"),
                    ("origin", "https://search.bilibili.com"),
                    ("referer", "https://search.bilibili.com/video"),
                    ("user-agent", UA_CHROME_131),
                ],
            )
            .await;

            match response {
                Ok(v) => {
                    let code = code_of(&v);
                    if code != 0 {
                        last_error = format!("B 站搜索失败: {} ({code})", message_of(&v));
                    } else if let Some(result) = v
                        .pointer("/data/result")
                        .and_then(|result| result.as_array())
                    {
                        return Ok(parse_search_results(result));
                    } else if v.pointer("/data/v_voucher").is_some() {
                        last_error = "B 站搜索触发临时校验".to_string();
                    } else {
                        last_error = "B 站搜索响应缺少结果数据".to_string();
                    }
                }
                Err(error) => last_error = error,
            }

            tokio::time::sleep(Duration::from_millis(250)).await;
        }
    }

    Err(last_error)
}

fn parse_search_results(result: &[Value]) -> Vec<SearchResult> {
    let mut out = Vec::with_capacity(result.len());
    for item in result {
        let title = str_of(item, "title")
            .replace("<em class=\"keyword\">", "")
            .replace("</em>", "");
        // 额外解码 HTML 实体 (如 &amp;)，搜索结果标题更干净
        let title = html_escape::decode_html_entities(&title).to_string();
        let play = int_of(item, "play");
        out.push(SearchResult {
            picture_url: format!("https:{}", str_of(item, "pic")),
            url: format!("https://www.bilibili.com/video/{}", str_of(item, "bvid")),
            title,
            views: format_views(play),
            danmu_count: int_of(item, "video_review"),
            author: str_of(item, "author"),
            date: unix_to_date(int_of(item, "pubdate")),
        });
    }
    out
}

// ---------------------------------------------------------------------------
// 视频信息与播放地址
// ---------------------------------------------------------------------------

/// 对应 Go GetCList (失败返回零值 VideoInfo)
pub async fn get_clist(bvid: &str) -> VideoInfo {
    if bvid.is_empty() {
        return VideoInfo::default();
    }
    let url = format!("https://api.bilibili.com/x/web-interface/view?bvid={bvid}");
    let Ok(resp) = client().get(&url).send().await else {
        return VideoInfo::default();
    };
    let Ok(text) = resp.text().await else {
        return VideoInfo::default();
    };
    let Ok(v) = serde_json::from_str::<Value>(&text) else {
        return VideoInfo::default();
    };
    if code_of(&v) != 0 {
        println!("未找到相关视频");
        return VideoInfo::default();
    }
    let Some(data) = v.get("data") else {
        return VideoInfo::default();
    };
    let pages: Vec<Page> = data
        .get("pages")
        .and_then(|p| serde_json::from_value(p.clone()).ok())
        .unwrap_or_default();
    VideoInfo {
        bvid: str_of(data, "bvid"),
        aid: int_of(data, "aid"),
        title: str_of(data, "title"),
        desc: str_of(data, "desc"),
        videos: int_of(data, "videos"),
        pic: str_of(data, "pic"),
        owner_mid: data.pointer("/owner/mid").and_then(|x| x.as_i64()).unwrap_or(0),
        owner_name: data
            .pointer("/owner/name")
            .and_then(|x| x.as_str())
            .unwrap_or("")
            .to_string(),
        owner_face: data
            .pointer("/owner/face")
            .and_then(|x| x.as_str())
            .unwrap_or("")
            .to_string(),
        pages,
        cid: 0, // Go 原版从不设置该字段
    }
}

/// 对应 Go GetUrlByCid (失败返回空 url)
pub async fn get_url_by_cid(aid: i64, cid: i64) -> PlayURLInfo {
    if cid == 0 {
        return PlayURLInfo::default();
    }
    let url = format!(
        "https://api.bilibili.com/x/player/playurl?avid={aid}&cid={cid}&qn=0&type=json&platform=html5"
    );
    let Ok(resp) = client()
        .get(&url)
        .header("User-Agent", UA_CHROME)
        .header("Host", "api.bilibili.com")
        .send()
        .await
    else {
        return PlayURLInfo::default();
    };
    let Ok(text) = resp.text().await else {
        return PlayURLInfo::default();
    };
    let Ok(v) = serde_json::from_str::<Value>(&text) else {
        return PlayURLInfo::default();
    };
    if code_of(&v) != 0 {
        return PlayURLInfo::default();
    }
    let url = v
        .pointer("/data/durl/0/url")
        .and_then(|u| u.as_str())
        .unwrap_or("")
        .to_string();
    PlayURLInfo { url }
}

// ---------------------------------------------------------------------------
// 用户信息
// ---------------------------------------------------------------------------

/// 对应 Go GetBLUserInfo (未登录/失败返回 None)
pub async fn get_user_info() -> Option<UserInfo> {
    let cookie = get_sessdata();
    if cookie.is_empty() {
        return None;
    }
    let resp = client()
        .get("https://api.bilibili.com/x/web-interface/nav")
        .header("User-Agent", UA_CHROME)
        .header("Cookie", &cookie)
        .send()
        .await
        .ok()?;
    let text = resp.text().await.ok()?;
    let v: Value = serde_json::from_str(&text).ok()?;
    if code_of(&v) != 0 {
        return None;
    }
    let uname = v
        .pointer("/data/uname")
        .and_then(|x| x.as_str())
        .unwrap_or("")
        .to_string();
    let face = v
        .pointer("/data/face")
        .and_then(|x| x.as_str())
        .unwrap_or("")
        .to_string();
    let mid = v.pointer("/data/mid").and_then(|x| x.as_i64()).unwrap_or(0);
    // 与 Go 一致: 把用户信息写回 dkv
    let _ = dkv::set_item("uname", &json!(uname));
    let _ = dkv::set_item("face", &json!(face));
    let _ = dkv::set_item("mid", &json!(mid));
    Some(UserInfo { uname, face, mid })
}

// ---------------------------------------------------------------------------
// 动态 / 推荐 / 收藏夹 / UP 主作品 / 历史 / 系列 / 热门
// ---------------------------------------------------------------------------

/// 对应 Go GetBLFeedList
pub async fn get_feed_list(offset: &str) -> Result<FeedList, String> {
    let cookie = get_sessdata();
    if cookie.is_empty() {
        return Ok(FeedList::default());
    }
    let mut url = String::from("https://api.bilibili.com/x/polymer/web-dynamic/v1/feed/all?type=video");
    if !offset.is_empty() {
        url.push_str(&format!("&offset={}", urlencode(offset)));
    }
    let v = get_json(&url, Some(&cookie), &[]).await?;
    let data = check_code(&v, "API returned non-zero code")?;
    Ok(FeedList {
        items: arr_of(&data, "items"),
        has_more: data.get("has_more").and_then(|x| x.as_bool()).unwrap_or(false),
        offset: str_of(&data, "offset"),
    })
}

/// 对应 Go GetBLRCMDList
pub async fn get_rcmd_list(page: i32) -> Result<RCMDList, String> {
    let cookie = get_sessdata();
    if cookie.is_empty() {
        return Ok(RCMDList::default());
    }
    let v = get_json(
        "https://api.bilibili.com/x/web-interface/wbi/index/top/feed/rcmd",
        Some(&cookie),
        &[],
    )
    .await?;
    let data = check_code(&v, "API returned non-zero code")?;
    Ok(RCMDList {
        items: arr_of(&data, "item"),
        has_more: page < 10,
        page,
    })
}

/// 对应 Go GetBLFavFolderList
pub async fn get_fav_folder_list() -> Result<Vec<Value>, String> {
    let cookie = get_sessdata();
    if cookie.is_empty() {
        return Ok(Vec::new());
    }
    let mid = dkv::get_string("mid");
    let url = format!(
        "https://api.bilibili.com/x/v3/fav/folder/created/list-all?up_mid={mid}&type=2"
    );
    let v = get_json(&url, Some(&cookie), &[]).await?;
    let data = check_code(&v, "failed to fetch folder list")?;
    Ok(arr_of(&data, "list"))
}

/// 对应 Go GetBLFavFolderListDetail
pub async fn get_fav_folder_detail(fid: i32, page: i32) -> Result<Vec<Value>, String> {
    let cookie = get_sessdata();
    if cookie.is_empty() {
        return Ok(Vec::new());
    }
    let url = format!(
        "https://api.bilibili.com/x/v3/fav/resource/list?media_id={fid}&type=0&ps=21&pn={page}"
    );
    let v = get_json(&url, Some(&cookie), &[]).await?;
    let data = check_code(&v, "failed to fetch folder detail")?;
    Ok(arr_of(&data, "medias"))
}

/// 对应 Go GetUpVideoList
pub async fn get_up_video_list(host_mid: i32, offset: &str) -> Result<FeedList, String> {
    let cookie = get_sessdata();
    if cookie.is_empty() {
        return Ok(FeedList::default());
    }
    let mut url = format!(
        "https://api.bilibili.com/x/polymer/web-dynamic/v1/feed/space?type=video&host_mid={host_mid}"
    );
    if !offset.is_empty() {
        url.push_str(&format!("&offset={}", urlencode(offset)));
    }
    let v = get_json(&url, Some(&cookie), &[]).await?;
    let data = check_code(&v, "API returned non-zero code")?;
    Ok(FeedList {
        items: arr_of(&data, "items"),
        has_more: data.get("has_more").and_then(|x| x.as_bool()).unwrap_or(false),
        offset: str_of(&data, "offset"),
    })
}

/// 对应 Go GetBLHistoryList
pub async fn get_history_list(
    max: i32,
    view_at: i32,
    business: &str,
    ps: i32,
) -> Result<HistoryList, String> {
    let cookie = get_sessdata();
    if cookie.is_empty() {
        return Ok(HistoryList::default());
    }
    let biz = if business.is_empty() { "archive" } else { business };
    let page_size = if ps > 0 { ps } else { 20 };
    let url = format!(
        "https://api.bilibili.com/x/web-interface/history/cursor?type=archive&max={max}&view_at={view_at}&business={biz}&ps={page_size}"
    );
    let v = get_json(&url, Some(&cookie), &[]).await?;
    let data = check_code(&v, "API returned non-zero code")?;
    Ok(HistoryList {
        list: arr_of(&data, "list"),
        cursor: HistoryCursor {
            max: data.pointer("/cursor/max").and_then(|x| x.as_i64()).unwrap_or(0) as i32,
            view_at: data
                .pointer("/cursor/view_at")
                .and_then(|x| x.as_i64())
                .unwrap_or(0) as i32,
            business: data
                .pointer("/cursor/business")
                .and_then(|x| x.as_str())
                .unwrap_or("")
                .to_string(),
        },
    })
}

/// 对应 Go GetSeriesList (返回 seasons_list[].meta 原始对象)
pub async fn get_series_list(mid: i32) -> Result<Vec<Value>, String> {
    let cookie = get_sessdata();
    if cookie.is_empty() {
        return Ok(Vec::new());
    }
    let url = format!(
        "https://api.bilibili.com/x/polymer/web-space/seasons_series_list?mid={mid}&page_num=1&page_size=20"
    );
    let v = get_json(&url, Some(&cookie), &[("Referer", "https://www.bilibili.com")]).await?;
    let data = check_code(&v, "failed to fetch series list")?;
    let mut out = Vec::new();
    if let Some(list) = data
        .pointer("/items_lists/seasons_list")
        .and_then(|l| l.as_array())
    {
        for item in list {
            if let Some(meta) = item.get("meta") {
                out.push(meta.clone());
            }
        }
    }
    Ok(out)
}

/// 对应 Go GetSeriesVideos
pub async fn get_series_videos(mid: i32, series_id: i32, page_num: i32) -> Result<Vec<SeriesArchive>, String> {
    let cookie = get_sessdata();
    if cookie.is_empty() {
        return Ok(Vec::new());
    }
    let page_num = if page_num < 1 { 1 } else { page_num };
    let url = format!(
        "https://api.bilibili.com/x/polymer/web-space/seasons_archives_list?mid={mid}&season_id={series_id}&page_num={page_num}&page_size=30"
    );
    let v = get_json(&url, Some(&cookie), &[]).await?;
    let data = check_code(&v, "failed to fetch series videos")?;
    let archives: Vec<SeriesArchive> = data
        .get("archives")
        .and_then(|a| serde_json::from_value(a.clone()).ok())
        .unwrap_or_default();
    Ok(archives)
}

/// 对应 Go GetBLPopularList (cookie 可选)
pub async fn get_popular_list(page: i32) -> Result<PopularList, String> {
    let cookie = get_sessdata();
    let url = format!("https://api.bilibili.com/x/web-interface/popular?pn={page}&ps=20");
    let v = get_json(&url, Some(&cookie), &[]).await?;
    let data = check_code(&v, "获取热门失败")?;
    let no_more = data.get("no_more").and_then(|x| x.as_bool()).unwrap_or(false);
    Ok(PopularList {
        items: arr_of(&data, "list"),
        has_more: !no_more,
        no_more,
        page,
    })
}

// ---------------------------------------------------------------------------
// 弹幕 (XML + gzip/deflate 解压)
// ---------------------------------------------------------------------------

fn decompress_http(body: &[u8], content_encoding: &str) -> Result<Vec<u8>, String> {
    // gzip: Content-Encoding 含 gzip 或魔数 0x1f 0x8b
    if content_encoding.contains("gzip") || (body.len() > 2 && body[0] == 0x1f && body[1] == 0x8b) {
        let mut out = Vec::new();
        GzDecoder::new(body)
            .read_to_end(&mut out)
            .map_err(|e| format!("gzip 解压失败: {e}"))?;
        Ok(out)
    } else if content_encoding.contains("deflate") || (!body.is_empty() && body[0] == 0x78) {
        let mut out = Vec::new();
        DeflateDecoder::new(body)
            .read_to_end(&mut out)
            .map_err(|e| format!("deflate 解压失败: {e}"))?;
        Ok(out)
    } else {
        Ok(body.to_vec())
    }
}

/// 解析 XML `<d p="...">内容</d>` 的 p 属性 (与 Go parseDanmakuItem 一致)
fn parse_danmaku_item(content: &str, p: &str) -> DanmakuItem {
    let mut item = DanmakuItem {
        content: content.to_string(),
        ..Default::default()
    };
    let parts: Vec<&str> = p.split(',').collect();
    if parts.len() >= 8 {
        if let Ok(t) = parts[0].parse::<f64>() {
            item.time = t;
        }
        if let Ok(t) = parts[1].parse::<i64>() {
            item.kind = t;
        }
        if let Ok(t) = parts[2].parse::<i64>() {
            item.font_size = t;
        }
        if let Ok(t) = parts[3].parse::<i64>() {
            item.color = t;
        }
        if let Ok(t) = parts[4].parse::<i64>() {
            item.send_time = t;
        }
        if let Ok(t) = parts[7].parse::<i64>() {
            item.dmid = t;
        }
    }
    item
}

/// 对应 Go GetDanmakuList
pub async fn get_danmaku_list(cid: i64) -> Result<DanmakuList, String> {
    if cid == 0 {
        return Ok(DanmakuList::default());
    }
    let url = format!("https://comment.bilibili.com/{cid}.xml");
    let resp = client()
        .get(&url)
        .header("User-Agent", UA_CHROME)
        .header("Accept-Encoding", "gzip, deflate")
        .send()
        .await
        .map_err(|e| format!("网络请求失败: {e}"))?;
    let content_encoding = resp
        .headers()
        .get("content-encoding")
        .and_then(|v| v.to_str().ok())
        .unwrap_or("")
        .to_lowercase();
    let body = resp.bytes().await.map_err(|e| format!("读取响应失败: {e}"))?;
    let xml_data = decompress_http(&body, &content_encoding)?;
    if xml_data.is_empty() {
        return Ok(DanmakuList::default());
    }
    let xml_str = std::str::from_utf8(&xml_data).map_err(|e| format!("XML 编码错误: {e}"))?;
    let doc = roxmltree::Document::parse(xml_str)
        .map_err(|e| format!("XML 解析失败: {e}, 数据长度: {}", xml_data.len()))?;
    let mut items = Vec::new();
    for node in doc.descendants() {
        if node.is_element() && node.tag_name().name() == "d" {
            let p = node.attribute("p").unwrap_or("");
            let content = node.text().unwrap_or("");
            items.push(parse_danmaku_item(content, p));
        }
    }
    Ok(DanmakuList { items })
}

// ---------------------------------------------------------------------------
// 评论
// ---------------------------------------------------------------------------

/// 对应 Go GetReplyList
pub async fn get_reply_list(oid: i64, page: i32) -> Result<ReplyList, String> {
    if oid == 0 {
        return Ok(ReplyList::default());
    }
    let url = format!("https://api.bilibili.com/x/v2/reply?pn={page}&type=1&oid={oid}&sort=2");
    let cookie = get_sessdata();
    let v = get_json(
        &url,
        Some(&cookie),
        &[
            ("Accept", "application/json, text/plain, */*"),
            ("Referer", "https://www.bilibili.com/"),
        ],
    )
    .await?;
    let data = check_code(&v, "获取评论失败")?;
    let num = data.pointer("/page/num").and_then(|x| x.as_i64()).unwrap_or(0);
    let size = data.pointer("/page/size").and_then(|x| x.as_i64()).unwrap_or(0);
    let count = data
        .pointer("/page/count")
        .and_then(|x| x.as_i64())
        .unwrap_or(0);
    let mut items: Vec<ReplyItem> = data
        .get("replies")
        .and_then(|r| serde_json::from_value(r.clone()).ok())
        .unwrap_or_default();
    // 楼中楼只保留前 3 条 (与 Go 一致)
    for reply in &mut items {
        if reply.replies.len() > 3 {
            reply.replies.truncate(3);
        }
    }
    let has_more = !items.is_empty() && num * size < count;
    Ok(ReplyList {
        items,
        has_more,
        next: (num + 1) as i32,
        total_count: count as i32,
    })
}

// ---------------------------------------------------------------------------
// 点赞 / 投币 / 关注 (需要登录 + csrf)
// ---------------------------------------------------------------------------

/// 对应 Go LikeVideo
pub async fn like_video(bid: &str, like: i32) -> Result<bool, String> {
    let cookie = get_sessdata();
    if cookie.is_empty() {
        return Err("未登录".to_string());
    }
    if bid.is_empty() {
        return Err("未选择作品".to_string());
    }
    let csrf = extract_csrf(&cookie)?;
    let ticket = get_bili_ticket("").await?;
    let bvid = bid_without_prefix(bid);
    let url = format!(
        "https://api.bilibili.com/x/web-interface/archive/like?bvid={bvid}&like={like}&csrf={csrf}"
    );
    let v = post_json(&url, Some(&cookie), &[("bili_ticket", &ticket)]).await?;
    check_code(&v, "点赞失败")?;
    Ok(true)
}

/// 对应 Go HasLiked
pub async fn has_liked(bid: &str) -> Result<bool, String> {
    let cookie = get_sessdata();
    if cookie.is_empty() {
        return Err("未登录".to_string());
    }
    if bid.is_empty() {
        return Err("未选择作品".to_string());
    }
    let bvid = bid_without_prefix(bid);
    let url = format!("https://api.bilibili.com/x/web-interface/archive/has/like?bvid={bvid}");
    let v = get_json(&url, Some(&cookie), &[]).await?;
    let data = check_code(&v, "获取点赞状态失败")?;
    Ok(data.as_i64().unwrap_or(0) == 1)
}

/// 对应 Go CoinVideo
pub async fn coin_video(bid: &str, multiply: i32) -> Result<bool, String> {
    let cookie = get_sessdata();
    if cookie.is_empty() {
        return Err("未登录".to_string());
    }
    if bid.is_empty() {
        return Err("未选择作品".to_string());
    }
    let csrf = extract_csrf(&cookie)?;
    let bvid = bid_without_prefix(bid);
    let url = format!(
        "https://api.bilibili.com/x/web-interface/coin/add?bvid={bvid}&multiply={multiply}&csrf={csrf}&select_like=0"
    );
    let v = post_json(&url, Some(&cookie), &[]).await?;
    check_code(&v, "投币失败")?;
    Ok(true)
}

/// 对应 Go HasCoin (返回 data.multiply)
pub async fn has_coin(bid: &str) -> Result<i32, String> {
    let cookie = get_sessdata();
    if cookie.is_empty() {
        return Err("未登录".to_string());
    }
    if bid.is_empty() {
        return Err("未选择作品".to_string());
    }
    let bvid = bid_without_prefix(bid);
    let url = format!("https://api.bilibili.com/x/web-interface/archive/coins?bvid={bvid}");
    let v = get_json(&url, Some(&cookie), &[]).await?;
    let data = check_code(&v, "获取投币状态失败")?;
    Ok(data
        .get("multiply")
        .and_then(|x| x.as_i64())
        .unwrap_or(0) as i32)
}

/// 对应 Go Follow / Unfollow (act: 1 关注, 2 取消关注)
async fn modify_relation(mid: i32, act: i32) -> Result<bool, String> {
    let cookie = get_sessdata();
    if cookie.is_empty() {
        return Err("未登录".to_string());
    }
    if mid == 0 {
        return Err("未指定UP主".to_string());
    }
    let csrf = extract_csrf(&cookie)?;
    let url = format!(
        "https://api.bilibili.com/x/relation/modify?fid={mid}&act={act}&csrf={csrf}"
    );
    let v = post_json(
        &url,
        Some(&cookie),
        &[("Content-Type", "application/x-www-form-urlencoded")],
    )
    .await?;
    check_code(&v, "操作失败")?;
    Ok(true)
}

pub async fn follow(mid: i32) -> Result<bool, String> {
    modify_relation(mid, 1).await
}

pub async fn unfollow(mid: i32) -> Result<bool, String> {
    modify_relation(mid, 2).await
}

/// 对应 Go IsFollowing (粉丝数不需要登录; 关注状态需要登录)
pub async fn is_following(mid: i64) -> Result<FollowStatus, String> {
    if mid == 0 {
        return Ok(FollowStatus {
            is_following: false,
            follower: 0,
        });
    }
    // 先获取粉丝数 (不需要登录)
    let mut follower: i64 = 0;
    let stat_url = format!("https://api.bilibili.com/x/relation/stat?vmid={mid}");
    if let Ok(resp) = client()
        .get(&stat_url)
        .header("User-Agent", UA_CHROME)
        .send()
        .await
    {
        if let Ok(text) = resp.text().await {
            if let Ok(v) = serde_json::from_str::<Value>(&text) {
                if code_of(&v) == 0 {
                    follower = v
                        .pointer("/data/follower")
                        .and_then(|x| x.as_i64())
                        .unwrap_or(0);
                }
            }
        }
    }
    // 是否是自己 (dkv 中的 mid)
    let current_mid: i64 = dkv::get_string("mid").parse().unwrap_or(0);
    if current_mid == mid {
        return Ok(FollowStatus {
            is_following: true,
            follower,
        });
    }
    // 查询关注状态 (attribute: 2=已关注, 6=已关注+互相关注)
    let relation_url = format!("https://api.bilibili.com/x/relation?fid={mid}");
    let cookie = get_sessdata();
    let mut req = client()
        .get(&relation_url)
        .header("User-Agent", UA_CHROME);
    if !cookie.is_empty() {
        req = req.header("Cookie", &cookie);
    }
    let Ok(resp) = req.send().await else {
        return Ok(FollowStatus {
            is_following: false,
            follower,
        });
    };
    let Ok(text) = resp.text().await else {
        return Ok(FollowStatus {
            is_following: false,
            follower,
        });
    };
    let Ok(v) = serde_json::from_str::<Value>(&text) else {
        return Ok(FollowStatus {
            is_following: false,
            follower,
        });
    };
    if code_of(&v) != 0 {
        return Ok(FollowStatus {
            is_following: false,
            follower,
        });
    }
    let attribute = v
        .pointer("/data/attribute")
        .and_then(|x| x.as_i64())
        .unwrap_or(0);
    let is_following = attribute == 2 || attribute == 6;
    Ok(FollowStatus {
        is_following,
        follower,
    })
}

/// 对应 Go ReportPlayProgress
pub async fn report_play_progress(aid: i32, cid: i32, progress: i32) -> Result<bool, String> {
    let cookie = get_sessdata();
    if cookie.is_empty() {
        return Err("未登录".to_string());
    }
    let csrf = extract_csrf(&cookie)?;
    let url = format!(
        "https://api.bilibili.com/x/v2/history/report?aid={aid}&cid={cid}&progress={progress}&csrf={csrf}"
    );
    let v = post_json(&url, Some(&cookie), &[]).await?;
    check_code(&v, "上报播放进度失败")?;
    Ok(true)
}

// ---------------------------------------------------------------------------
// 图片代理 / 图片抓取
// ---------------------------------------------------------------------------

/// 对应 Go ProxyImage — 生成指向本地代理的 URL
pub fn proxy_image_url(url: &str) -> String {
    if url.is_empty() {
        return String::new();
    }
    let full = if url.starts_with("//") {
        format!("https:{url}")
    } else {
        url.to_string()
    };
    format!(
        "http://127.0.0.1:{IMAGE_PROXY_PORT}/image-proxy?url={}",
        urlencode(&full)
    )
}

pub fn get_image_proxy_port() -> u16 {
    IMAGE_PROXY_PORT
}

// ---------------------------------------------------------------------------
// 播放列表持久化 (dkv)
// ---------------------------------------------------------------------------

/// 对应 Go GetPlaylist
pub fn get_playlist() -> String {
    dkv::get_string("playlist")
}

/// 对应 Go SetPlaylist
pub fn set_playlist(playlist_json: &str) {
    if let Err(e) = dkv::set_item("playlist", &json!(playlist_json)) {
        eprintln!("dkv: set playlist failed: {e}");
    }
}

/// 对应 Go GetPlaylistPlayMode (缺省 "sequence")
pub fn get_playlist_play_mode() -> String {
    match dkv::get_item("playlist_play_mode") {
        Some(Value::String(s)) => s,
        _ => "sequence".to_string(),
    }
}

/// 对应 Go SetPlaylistPlayMode
pub fn set_playlist_play_mode(mode: &str) {
    if let Err(e) = dkv::set_item("playlist_play_mode", &json!(mode)) {
        eprintln!("dkv: set playlist_play_mode failed: {e}");
    }
}

// ---------------------------------------------------------------------------
// 版本 / 平台 / 更新检查
// ---------------------------------------------------------------------------

/// 正确的 semver 比较 (修复 Go 的字符串 `>` 比较 bug:
/// 例如 "1.9.5" > "1.10.0" 按字符串比较会错误地返回 true)
pub fn compare_versions(a: &str, b: &str) -> Ordering {
    let a_parts: Vec<u64> = a.split('.').filter_map(|p| p.parse().ok()).collect();
    let b_parts: Vec<u64> = b.split('.').filter_map(|p| p.parse().ok()).collect();
    for i in 0..a_parts.len().max(b_parts.len()) {
        let x = a_parts.get(i).copied().unwrap_or(0);
        let y = b_parts.get(i).copied().unwrap_or(0);
        match x.cmp(&y) {
            Ordering::Equal => continue,
            o => return o,
        }
    }
    Ordering::Equal
}

/// 版本号转 build 号 (1.9.5 -> 195, 2.0.0 -> 200)，延续旧版 APP_VERSION_NO 惯例
pub fn version_to_build(version: &str) -> i32 {
    let parts: Vec<u32> = version.split('.').filter_map(|p| p.parse().ok()).collect();
    let major = parts.first().copied().unwrap_or(0);
    let minor = parts.get(1).copied().unwrap_or(0);
    let patch = parts.get(2).copied().unwrap_or(0);
    (major * 100 + minor * 10 + patch) as i32
}

#[cfg(test)]
mod tests {
    use super::*;

    /// 修复 Go 字符串 `>` 比较 bug: "1.9.5" > "1.10.0" 必须为 false
    #[test]
    fn semver_compare_fixes_go_bug() {
        assert_eq!(compare_versions("1.9.5", "1.10.0"), Ordering::Less);
        assert_eq!(compare_versions("1.10.0", "1.9.5"), Ordering::Greater);
        assert_eq!(compare_versions("2.0.0", "1.9.5"), Ordering::Greater);
        assert_eq!(compare_versions("2.0.0", "2.0.0"), Ordering::Equal);
        assert_eq!(compare_versions("2.0", "2.0.0"), Ordering::Equal);
        assert_eq!(compare_versions("1.10.5", "1.9.9"), Ordering::Greater);
    }

    #[test]
    fn build_number_continues_old_convention() {
        assert_eq!(version_to_build("1.9.5"), 195);
        assert_eq!(version_to_build("2.0.0"), 200);
    }
}

pub fn get_platform() -> String {
    match std::env::consts::OS {
        "macos" => "darwin".to_string(),
        other => other.to_string(),
    }
}

/// 对应 Go Menu.CheckForUpdates。
///
/// 逻辑与 Go 一致: 默认先查 gitee，失败回退 github；git_from=="github" 时直查
/// github。版本比较改用 `compare_versions` (semver)，而非字符串 `>`。
pub async fn check_for_updates(is_manual: bool, git_from: &str, current_version: &str) -> UpdateResult {
    const GITHUB_URL: &str = "https://api.github.com/repos/vst93/bili-fm/releases/latest";
    const GITEE_URL: &str = "https://gitee.com/api/v5/repos/vst93/bili-fm/releases/latest";
    const GITEE_HTML: &str = "https://gitee.com/vst93/bili-fm/releases/latest";

    // 候选源: (url, is_github)。Go 逻辑: 非 github 源先试 gitee，失败再试 github。
    let mut candidates: Vec<(&str, bool)> = Vec::new();
    if git_from != "github" {
        candidates.push((GITEE_URL, false));
    }
    candidates.push((GITHUB_URL, true));

    let mut network_failed = false;
    for (release_url, is_github) in candidates {
        let resp = match client()
            .get(release_url)
            .header("User-Agent", UA_CHROME)
            .header("Accept", "application/vnd.github+json")
            .send()
            .await
        {
            Ok(r) => r,
            Err(_) => {
                network_failed = true;
                continue;
            }
        };
        let Ok(text) = resp.text().await else {
            network_failed = true;
            continue;
        };
        let Ok(v) = serde_json::from_str::<Value>(&text) else {
            continue;
        };
        let tag = v
            .get("tag_name")
            .and_then(|t| t.as_str())
            .unwrap_or("")
            .to_string();
        if tag.is_empty() {
            continue;
        }
        let latest = tag.trim_start_matches('v');
        let current = current_version.trim_start_matches('v');
        if compare_versions(latest, current) == Ordering::Greater {
            // 下载地址: github 源用 release 页 URL, gitee 源用 gitee 固定 URL (与 Go 一致)
            let download_url = if is_github {
                v.get("html_url")
                    .and_then(|h| h.as_str())
                    .unwrap_or("")
                    .to_string()
            } else {
                GITEE_HTML.to_string()
            };
            return UpdateResult {
                has_update: true,
                latest_version: latest.to_string(),
                download_url,
                is_latest: false,
                error: String::new(),
            };
        }
        return UpdateResult {
            has_update: false,
            latest_version: String::new(),
            download_url: String::new(),
            is_latest: true,
            error: String::new(),
        };
    }

    // 所有候选源都失败
    if is_manual {
        let error = if network_failed {
            "网络连接失败，请检查网络后重试".to_string()
        } else {
            "获取版本信息失败，请稍后重试".to_string()
        };
        UpdateResult {
            error,
            ..Default::default()
        }
    } else {
        UpdateResult::default()
    }
}
