//! 内嵌 HTTP 图片代理 (对应旧版 main.go 的 imageProxyHandler)。
//!
//! 监听 127.0.0.1:4654，转发 B 站 CDN 图片请求并附加 Referer / User-Agent /
//! Accept 等头，避免 CDN 拒绝 (403)。前端通过
//! `http://127.0.0.1:4654/image-proxy?url=<encodeURIComponent(url)>` 加载图片。
//!
//! 注意: Windows 下 WebView2 默认拦截 http 混合内容，需要在创建 webview 时
//! 附加 `--allow-running-insecure-content` 参数 (Round 2 的 lib.rs 处理)。

use std::collections::HashMap;
use std::sync::{Mutex, OnceLock};
use std::time::{Duration, Instant};

use axum::body::Body;
use axum::extract::Query;
use axum::http::{header, HeaderMap, HeaderValue, StatusCode};
use axum::response::{IntoResponse, Response};
use axum::routing::get;
use axum::Router;
use tokio::net::TcpListener;
use tokio::sync::Semaphore;

pub const IMAGE_PROXY_PORT: u16 = 4654;

/// 与旧版 main.go imageProxyHandler 相同的 Chrome 131 UA
const UA_CHROME_131: &str = "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/131.0.0.0 Safari/537.36";
const MAX_CONCURRENT_FETCHES: usize = 8;
const FAILURE_CACHE_TTL: Duration = Duration::from_secs(30);

fn fetch_limit() -> &'static Semaphore {
    static LIMIT: OnceLock<Semaphore> = OnceLock::new();
    LIMIT.get_or_init(|| Semaphore::new(MAX_CONCURRENT_FETCHES))
}

fn failure_cache() -> &'static Mutex<HashMap<String, Instant>> {
    static CACHE: OnceLock<Mutex<HashMap<String, Instant>>> = OnceLock::new();
    CACHE.get_or_init(|| Mutex::new(HashMap::new()))
}

fn client() -> &'static reqwest::Client {
    static CLIENT: OnceLock<reqwest::Client> = OnceLock::new();
    CLIENT.get_or_init(|| {
        reqwest::Client::builder()
            .connect_timeout(Duration::from_secs(15))
            .danger_accept_invalid_certs(true)
            .build()
            .expect("failed to build proxy client")
    })
}

/// 启动图片代理服务器 (阻塞直到服务器退出)。
/// 调用方应在 tauri 的 async runtime 中 spawn (Round 2 的 lib.rs)。
pub async fn start_server() -> Result<(), String> {
    let app = Router::new()
        .route("/image-proxy", get(handle_image_proxy))
        .route("/audio-proxy", get(handle_audio_proxy));
    let listener = TcpListener::bind(("127.0.0.1", IMAGE_PROXY_PORT))
        .await
        .map_err(|e| format!("图片代理端口 {IMAGE_PROXY_PORT} 绑定失败: {e}"))?;
    #[cfg(debug_assertions)]
    println!("image-proxy listening on http://127.0.0.1:{IMAGE_PROXY_PORT}");
    axum::serve(listener, app).await.map_err(|e| e.to_string())
}

async fn handle_audio_proxy(
    Query(params): Query<HashMap<String, String>>,
    headers: HeaderMap,
) -> Response {
    let Some(url) = params.get("url").filter(|url| !url.is_empty()) else {
        return (StatusCode::BAD_REQUEST, "Missing 'url' query parameter").into_response();
    };
    let mut audio_url = url.clone();
    if audio_url.starts_with("//") {
        audio_url = format!("https:{audio_url}");
    }

    let mut request = client()
        .get(&audio_url)
        .header(header::USER_AGENT, UA_CHROME_131)
        .header(header::REFERER, "https://www.bilibili.com/")
        .header(header::ACCEPT, "audio/*,*/*;q=0.8")
        .header(header::ACCEPT_LANGUAGE, "zh-CN,zh;q=0.9,en;q=0.8");
    if let Some(range) = headers.get(header::RANGE) {
        request = request.header(header::RANGE, range);
    }

    let upstream = match request.send().await {
        Ok(response) => response,
        Err(error) => {
            #[cfg(debug_assertions)]
            eprintln!("audio-proxy: failed for {audio_url}: {error}");
            return (StatusCode::BAD_GATEWAY, "Failed to fetch audio").into_response();
        }
    };
    let status = upstream.status();
    let upstream_headers = upstream.headers().clone();
    let mut response = Response::new(Body::from_stream(upstream.bytes_stream()));
    *response.status_mut() = status;

    let response_headers = response.headers_mut();
    response_headers.insert(
        header::ACCESS_CONTROL_ALLOW_ORIGIN,
        HeaderValue::from_static("*"),
    );
    response_headers.insert(
        header::ACCEPT_RANGES,
        HeaderValue::from_static("bytes"),
    );
    response_headers.insert(
        header::ACCESS_CONTROL_EXPOSE_HEADERS,
        HeaderValue::from_static("Accept-Ranges, Content-Length, Content-Range"),
    );
    for name in [
        header::CONTENT_TYPE,
        header::CONTENT_LENGTH,
        header::CONTENT_RANGE,
        header::ETAG,
        header::LAST_MODIFIED,
    ] {
        if let Some(value) = upstream_headers.get(&name) {
            response_headers.insert(name, value.clone());
        }
    }
    if !response_headers.contains_key(header::CONTENT_TYPE) {
        response_headers.insert(header::CONTENT_TYPE, HeaderValue::from_static("audio/mpeg"));
    }
    response
}

async fn handle_image_proxy(Query(params): Query<HashMap<String, String>>) -> Response {
    let Some(url) = params.get("url").filter(|u| !u.is_empty()) else {
        return (StatusCode::BAD_REQUEST, "Missing 'url' query parameter").into_response();
    };
    let mut image_url = url.clone();
    // 处理协议相对 URL (与 Go 一致)
    if image_url.starts_with("//") {
        image_url = format!("https:{image_url}");
    }

    {
        let mut cache = failure_cache().lock().unwrap();
        if let Some(failed_at) = cache.get(&image_url) {
            if failed_at.elapsed() < FAILURE_CACHE_TTL {
                return (StatusCode::BAD_GATEWAY, "Image temporarily unavailable").into_response();
            }
            cache.remove(&image_url);
        }
    }

    let Ok(_permit) = fetch_limit().acquire().await else {
        return (StatusCode::SERVICE_UNAVAILABLE, "Image proxy unavailable").into_response();
    };

    // 带重试的上游请求 (最多 3 次)，重试间隔与 Go 一致: 200ms * attempt
    for attempt in 0..3 {
        if attempt > 0 {
            tokio::time::sleep(Duration::from_millis(200 * attempt as u64)).await;
        }
        match fetch_image(&image_url).await {
            Ok((content_type, bytes)) => {
                let mut response = Response::new(axum::body::Body::from(bytes));
                *response.status_mut() = StatusCode::OK;
                let headers = response.headers_mut();
                headers.insert(
                    header::CONTENT_TYPE,
                    HeaderValue::from_str(&content_type)
                        .unwrap_or(HeaderValue::from_static("image/jpeg")),
                );
                headers.insert(
                    header::ACCESS_CONTROL_ALLOW_ORIGIN,
                    HeaderValue::from_static("*"),
                );
                headers.insert(
                    header::CACHE_CONTROL,
                    HeaderValue::from_static("public, max-age=86400"),
                );
                headers.insert(
                    header::X_CONTENT_TYPE_OPTIONS,
                    HeaderValue::from_static("nosniff"),
                );
                return response;
            }
            Err(error) => {
                #[cfg(debug_assertions)]
                eprintln!(
                    "image-proxy: attempt {} failed for {}: {error}",
                    attempt + 1,
                    image_url
                );
                if !error.retryable {
                    break;
                }
            }
        }
    }
    let mut cache = failure_cache().lock().unwrap();
    if cache.len() >= 512 {
        cache.retain(|_, failed_at| failed_at.elapsed() < FAILURE_CACHE_TTL);
        if cache.len() >= 512 {
            cache.clear();
        }
    }
    cache.insert(image_url, Instant::now());
    (StatusCode::BAD_GATEWAY, "Failed to fetch image after retries").into_response()
}

struct FetchError {
    message: String,
    retryable: bool,
}

impl std::fmt::Display for FetchError {
    fn fmt(&self, formatter: &mut std::fmt::Formatter<'_>) -> std::fmt::Result {
        formatter.write_str(&self.message)
    }
}

async fn fetch_image(url: &str) -> Result<(String, Vec<u8>), FetchError> {
    let resp = client()
        .get(url)
        .timeout(Duration::from_secs(30))
        .header(header::USER_AGENT, UA_CHROME_131)
        .header(header::REFERER, "https://www.bilibili.com/")
        .header(
            header::ACCEPT,
            "image/avif,image/webp,image/apng,image/svg+xml,image/*,*/*;q=0.8",
        )
        .header(header::ACCEPT_LANGUAGE, "zh-CN,zh;q=0.9,en;q=0.8")
        .send()
        .await
        .map_err(|error| FetchError {
            message: error.to_string(),
            retryable: true,
        })?;
    if resp.status() != StatusCode::OK {
        let status = resp.status();
        return Err(FetchError {
            message: format!("upstream returned {}", status.as_u16()),
            retryable: status == StatusCode::TOO_MANY_REQUESTS || status.is_server_error(),
        });
    }
    let content_type = resp
        .headers()
        .get(header::CONTENT_TYPE)
        .and_then(|v| v.to_str().ok())
        .unwrap_or("image/jpeg")
        .to_string();
    let bytes = resp.bytes().await.map_err(|error| FetchError {
        message: error.to_string(),
        retryable: true,
    })?;
    Ok((content_type, bytes.to_vec()))
}
