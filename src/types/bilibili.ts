/**
 * Bilibili API 类型定义
 *
 * 与 src-tauri/src/bilibili.rs 中的 serde struct 字段一一对应
 * (Tauri invoke 返回的 JSON 使用 Rust 侧的字段名)。
 */

export interface SearchResult {
  picture_url: string;
  url: string;
  title: string;
  views: string;
  danmuCount: number;
  author: string;
  date: string;
}

export interface Dimension {
  width: number;
  height: number;
  rotate: number;
}

export interface Page {
  cid: number;
  page: number;
  from: string;
  part: string;
  duration: number;
  vid: string;
  weblink: string;
  dimension: Dimension;
  first_frame: string;
}

export interface VideoInfo {
  bvid: string;
  aid: number;
  title: string;
  desc: string;
  videos: number;
  pic: string;
  owner_mid: number;
  owner_name: string;
  owner_face: string;
  pages: Page[];
  cid: number;
}

export interface PlayURLInfo {
  url: string;
}

export interface UserInfo {
  uname: string;
  face: string;
  mid: number;
}

export interface FeedList {
  items: any[];
  has_more: boolean;
  offset: string;
}

export interface RCMDList {
  items: any[];
  has_more: boolean;
  page: number;
}

export interface HistoryCursor {
  max: number;
  view_at: number;
  business: string;
}

export interface HistoryList {
  list: any[];
  cursor: HistoryCursor;
}

export interface PopularList {
  items: any[];
  has_more: boolean;
  no_more: boolean;
  page: number;
}

export interface Stat {
  view: number;
}

export interface SeriesArchive {
  aid: number;
  bvid: string;
  title: string;
  pubdate: number;
  duration: number;
  pic: string;
  stat: Stat;
}

export interface FollowStatus {
  is_following: boolean;
  follower: number;
}

export interface DanmakuItem {
  content: string;
  time: number;
  type: number;
  fontSize: number;
  color: number;
  sendTime: number;
  dmid: number;
}

export interface DanmakuList {
  items: DanmakuItem[];
}

export interface ReplyContent {
  message: string;
}

export interface ReplyItem {
  rpid: number;
  oid: number;
  type: number;
  mid: number;
  content: ReplyContent;
  ctime: number;
  like: number;
  action: number;
  member: any;
  replies: ReplyItem[];
  root: number;
  parent: number;
}

export interface ReplyList {
  items: ReplyItem[];
  has_more: boolean;
  next: number;
  total_count: number;
}

export interface AppVersion {
  version: string;
  build: number;
}

export interface UpdateResult {
  hasUpdate: boolean;
  latestVersion: string;
  downloadUrl: string;
  isLatest: boolean;
  error: string;
}
