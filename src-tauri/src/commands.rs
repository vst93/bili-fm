//! `#[tauri::command]` 桥接层 — 把 bilibili / dkv 模块暴露给前端。
//!
//! 命令名与参数遵循 .pi-task-tauri-rewrite.md 的「Wails -> Tauri」映射表。
//! 前端调用示例:
//! ```ts
//! import { invoke } from "@tauri-apps/api/core";
//! const res = await invoke("search_video", { keyword, order });
//! ```

use serde_json::Value;
use tauri::AppHandle;

use crate::bilibili;

// ---------------------------------------------------------------------------
// 搜索 / 视频信息
// ---------------------------------------------------------------------------

#[tauri::command]
pub async fn search_video(keyword: String, order: String) -> Vec<bilibili::SearchResult> {
    bilibili::search_video(&keyword, &order).await
}

#[tauri::command]
pub async fn get_clist(bvid: String) -> bilibili::VideoInfo {
    bilibili::get_clist(&bvid).await
}

#[tauri::command]
pub async fn get_url_by_cid(aid: i64, cid: i64) -> bilibili::PlayURLInfo {
    bilibili::get_url_by_cid(aid, cid).await
}

// ---------------------------------------------------------------------------
// 登录
// ---------------------------------------------------------------------------

#[tauri::command]
pub async fn get_login_qrcode() -> Result<String, String> {
    bilibili::get_login_qrcode().await
}

#[tauri::command]
pub fn get_login_status() -> bool {
    bilibili::get_login_status()
}

#[tauri::command]
pub async fn get_login_qrcode_status() -> bool {
    bilibili::get_login_qrcode_status().await
}

#[tauri::command]
pub fn set_login_status(status: bool) {
    bilibili::set_login_status(status);
}

#[tauri::command]
pub fn get_sessdata() -> String {
    bilibili::get_sessdata()
}

#[tauri::command]
pub fn set_sessdata(data: String) {
    bilibili::set_sessdata(&data);
}

#[tauri::command]
pub async fn get_user_info() -> Option<bilibili::UserInfo> {
    bilibili::get_user_info().await
}

// ---------------------------------------------------------------------------
// 动态 / 推荐 / 收藏夹 / UP主 / 历史 / 系列 / 热门
// ---------------------------------------------------------------------------

#[tauri::command]
pub async fn get_feed_list(offset: String) -> Result<bilibili::FeedList, String> {
    bilibili::get_feed_list(&offset).await
}

#[tauri::command]
pub async fn get_rcmd_list(page: i32) -> Result<bilibili::RCMDList, String> {
    bilibili::get_rcmd_list(page).await
}

#[tauri::command]
pub async fn get_fav_folder_list() -> Result<Vec<Value>, String> {
    bilibili::get_fav_folder_list().await
}

#[tauri::command]
pub async fn get_fav_folder_detail(fid: i32, page: i32) -> Result<Vec<Value>, String> {
    bilibili::get_fav_folder_detail(fid, page).await
}

#[tauri::command]
pub async fn get_up_video_list(host_mid: i32, offset: String) -> Result<bilibili::FeedList, String> {
    bilibili::get_up_video_list(host_mid, &offset).await
}

#[tauri::command]
pub async fn get_history_list(
    max: i32,
    view_at: i32,
    business: String,
    ps: i32,
) -> Result<bilibili::HistoryList, String> {
    bilibili::get_history_list(max, view_at, &business, ps).await
}

#[tauri::command]
pub async fn get_series_list(mid: i32) -> Result<Vec<Value>, String> {
    bilibili::get_series_list(mid).await
}

#[tauri::command]
pub async fn get_series_videos(
    mid: i32,
    series_id: i32,
    page_num: i32,
) -> Result<Vec<bilibili::SeriesArchive>, String> {
    bilibili::get_series_videos(mid, series_id, page_num).await
}

#[tauri::command]
pub async fn get_popular_list(page: i32) -> Result<bilibili::PopularList, String> {
    bilibili::get_popular_list(page).await
}

// ---------------------------------------------------------------------------
// 弹幕 / 评论
// ---------------------------------------------------------------------------

#[tauri::command]
pub async fn get_danmaku_list(cid: i64) -> Result<bilibili::DanmakuList, String> {
    bilibili::get_danmaku_list(cid).await
}

#[tauri::command]
pub async fn get_reply_list(oid: i64, page: i32) -> Result<bilibili::ReplyList, String> {
    bilibili::get_reply_list(oid, page).await
}

// ---------------------------------------------------------------------------
// 点赞 / 投币 / 关注 / 播放进度
// ---------------------------------------------------------------------------

#[tauri::command]
pub async fn like_video(bid: String, like: i32) -> Result<bool, String> {
    bilibili::like_video(&bid, like).await
}

#[tauri::command]
pub async fn has_liked(bid: String) -> Result<bool, String> {
    bilibili::has_liked(&bid).await
}

#[tauri::command]
pub async fn coin_video(bid: String, multiply: i32) -> Result<bool, String> {
    bilibili::coin_video(&bid, multiply).await
}

#[tauri::command]
pub async fn has_coin(bid: String) -> Result<i32, String> {
    bilibili::has_coin(&bid).await
}

#[tauri::command]
pub async fn follow(mid: i32) -> Result<bool, String> {
    bilibili::follow(mid).await
}

#[tauri::command]
pub async fn unfollow(mid: i32) -> Result<bool, String> {
    bilibili::unfollow(mid).await
}

#[tauri::command]
pub async fn is_following(mid: i32) -> Result<bilibili::FollowStatus, String> {
    bilibili::is_following(mid as i64).await
}

#[tauri::command]
pub async fn report_play_progress(aid: i32, cid: i32, progress: i32) -> Result<bool, String> {
    bilibili::report_play_progress(aid, cid, progress).await
}

// ---------------------------------------------------------------------------
// 图片
// ---------------------------------------------------------------------------

#[tauri::command]
pub fn proxy_image_url(url: String) -> String {
    bilibili::proxy_image_url(&url)
}

#[tauri::command]
pub async fn fetch_image(url: String) -> Result<String, String> {
    bilibili::fetch_image(&url).await
}

#[tauri::command]
pub fn get_image_proxy_port() -> u16 {
    bilibili::get_image_proxy_port()
}

// ---------------------------------------------------------------------------
// 播放列表持久化
// ---------------------------------------------------------------------------

#[tauri::command]
pub fn get_playlist() -> String {
    bilibili::get_playlist()
}

#[tauri::command]
pub fn set_playlist(playlist_json: String) {
    bilibili::set_playlist(&playlist_json);
}

#[tauri::command]
pub fn get_playlist_play_mode() -> String {
    bilibili::get_playlist_play_mode()
}

#[tauri::command]
pub fn set_playlist_play_mode(mode: String) {
    bilibili::set_playlist_play_mode(&mode);
}

// ---------------------------------------------------------------------------
// 应用信息 / 更新 / 退出
// ---------------------------------------------------------------------------

#[tauri::command]
pub async fn check_for_updates(is_manual: bool, git_from: String, app: AppHandle) -> bilibili::UpdateResult {
    let version = app.package_info().version.to_string();
    bilibili::check_for_updates(is_manual, &git_from, &version).await
}

#[tauri::command]
pub fn get_platform() -> String {
    bilibili::get_platform()
}

#[tauri::command]
pub fn quit_app(app: AppHandle) {
    app.exit(0);
}

// ---------------------------------------------------------------------------
// 窗口控制 (迷你模式 / 隐藏 / 最小化)
//
// 前端调用示例:
// ```ts
// import { invoke } from "@tauri-apps/api/core";
// await invoke("set_window_size", { width: 400, height: 155 }); // 迷你模式
// await invoke("hide_window");       // 关闭按钮 → 隐藏到托盘
// await invoke("minimize_window");   // 最小化到任务栏
// ```
//
// 注意: Linux 下 webkit2gtk 无边框窗口无法在运行时调整大小 (迷你模式在
// 前端已按平台禁用); 此处仍保留命令, 由前端决定是否调用。
// ---------------------------------------------------------------------------

#[tauri::command]
pub fn set_window_size(window: tauri::Window, width: u32, height: u32) -> Result<(), String> {
    window
        .set_size(tauri::LogicalSize::new(width as f64, height as f64))
        .map_err(|e| format!("set_window_size 失败: {e}"))
}

#[tauri::command]
pub fn hide_window(window: tauri::Window) -> Result<(), String> {
    window.hide().map_err(|e| format!("hide_window 失败: {e}"))
}

#[tauri::command]
pub fn minimize_window(window: tauri::Window) -> Result<(), String> {
    window
        .minimize()
        .map_err(|e| format!("minimize_window 失败: {e}"))
}

#[tauri::command]
pub fn show_window(window: tauri::Window) -> Result<(), String> {
    let _ = window.unminimize();
    window.show().map_err(|e| format!("show_window 失败: {e}"))
}

#[tauri::command]
pub fn get_app_version(app: AppHandle) -> bilibili::AppVersion {
    let version = app.package_info().version.to_string();
    bilibili::AppVersion {
        version: version.clone(),
        build: bilibili::version_to_build(&version),
    }
}
