//! 旧版 Wails `service.SendAppStats` 的 Rust 移植。
//!
//! 应用启动时以 fire-and-forget 方式向 umami.dev 发送一次使用统计，
//! 请求失败只写入 stderr，不影响应用启动或运行。

use reqwest::header::{CONTENT_TYPE, USER_AGENT};
use serde::Serialize;

const STATS_ENDPOINT: &str = "https://api-gateway.umami.dev/api/send";
const WEBSITE: &str = "32c24ade-d689-4252-a37a-52c61aa04e5a";
const TITLE: &str = "bili-fm";

#[cfg(target_os = "macos")]
const APP_USER_AGENT: &str = "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/91.0.4472.114 Safari/537.36";
#[cfg(target_os = "windows")]
const APP_USER_AGENT: &str = "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/91.0.4472.114 Safari/537.36";
#[cfg(not(any(target_os = "macos", target_os = "windows")))]
const APP_USER_AGENT: &str = "Mozilla/5.0 (X11; Linux x86_64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/91.0.4472.114 Safari/537.36";

#[derive(Serialize)]
struct StatsEvent {
    #[serde(rename = "type")]
    event_type: &'static str,
    payload: StatsPayload,
}

#[derive(Serialize)]
struct StatsPayload {
    website: &'static str,
    screen: &'static str,
    language: &'static str,
    title: &'static str,
    hostname: &'static str,
    url: &'static str,
    referrer: &'static str,
}

/// 向 umami.dev 发送应用启动统计；错误仅记录到 stderr。
pub async fn send_app_stats() {
    let body = StatsEvent {
        event_type: "event",
        payload: StatsPayload {
            website: WEBSITE,
            screen: "",
            language: "",
            title: TITLE,
            hostname: "meimingzi.top",
            url: "https://meimingzi.top/bili-fm",
            referrer: "",
        },
    };

    let result = reqwest::Client::new()
        .post(STATS_ENDPOINT)
        .header(CONTENT_TYPE, "application/json")
        .header(USER_AGENT, APP_USER_AGENT)
        .json(&body)
        .send()
        .await
        .and_then(reqwest::Response::error_for_status);

    if let Err(error) = result {
        eprintln!("send-app-stats failed: {error}");
    }
}
