export const urlToBVID = (url: string) => {
  const bvRegex = /BV[a-zA-Z0-9]+/;
  const match = url.match(bvRegex);
  return match ? match[0] : "";
};

export const bvidToUrl = (bvid: string) => {
  return `https://www.bilibili.com/video/${bvid}`;
};

export const convertToDuration = (seconds: number) => {
  // 兜底掉 undefined / NaN / 负数，避免渲染出 "NaN:NaN"
  const total = Number.isFinite(seconds) ? Math.max(0, Math.floor(seconds)) : 0;
  const minutes = Math.floor(total / 60);
  const remainingSeconds = total % 60;
  return `${minutes}:${remainingSeconds.toString().padStart(2, "0")}`;
};

// 列表封面统一走本函数拼接图片代理 URL。
//
// 修复轮 23（内存 + 用户诉求「列表页封面分辨率再降一些」）：
//   B 站 CDN 支持在路径后追加 `@<n>w.webp` 让服务端下采样。列表卡片的封面
//   展示宽度通常只有 ~130–200 CSS px（抽屉 2/3 列网格），此前默认 400w 属于
//   过采样 —— 解码后的位图是 WebView2 内存里真正的大头（每张 400w 封面约
//   0.3MB 解码内存，一整屏列表可积到几十 MB）。
//
// 修复轮 24（用户复测：「一排放 3 个卡片，封面需要的尺寸应该不用特别大」）：
//   主窗口固定 800×600，抽屉内 3 列网格的卡片宽 ≈ (800 − 2*24(mx-6) − 2*24(px-6)
//   − 2*8(gap))/3 ≈ 229 CSS px（封面盒高 100px，object-fit:cover 按宽度填充）。
//   300w 对 100% 缩放已偏过采样，对用户“不用特别大”的诉求也偏大。默认值由
//   300w 降到 240w：单张位图解码像素少 36%（240²/300² = 0.64），列表缩略图
//   观感几乎无差别；240 ≥ 229，在 100% 缩放下仍不欠采样（>125% 缩放会略软，
//   属可接受的缩略图代价，用户已授权降分辨率优先保内存）。
//   —— 主播放器封面（480）、背景光场（320）、歌单缩略图（192）、各类头像（96）
//   均按各自调用传显式宽度，**不受此默认值影响**。
export const graftingImage = (img: string, width = 240) => {
  if (!img) return img;

  let source = img.startsWith("//") ? `https:${img}` : img;
  try {
    const url = new URL(source);
    if (url.hostname.endsWith("hdslb.com") && url.pathname.includes("/bfs/") && !url.pathname.includes("@")) {
      url.pathname += `@${width}w.webp`;
      source = url.toString();
    }
  } catch {
    // Keep non-standard URLs unchanged and let the proxy handle them.
  }

  return `http://127.0.0.1:4654/image-proxy?url=${encodeURIComponent(source)}`;
};

export const formatDate = (timestamp: number) => {
  const date = new Date(timestamp * 1000);
  return `${date.getFullYear()}年${String(date.getMonth() + 1).padStart(2, "0")}月${String(date.getDate()).padStart(2, "0")}日`;
};


export const formatDatetime = (timestamp: number) => {
  const date = new Date(timestamp * 1000);
  return `${date.getFullYear()}-${String(date.getMonth() + 1).padStart(2, '0')}-${String(date.getDate()).padStart(2, '0')} ${String(date.getHours()).padStart(2, '0')}:${String(date.getMinutes()).padStart(2, '0')}`;
};

export const formatNumber = (num: number) => {
  if (num >= 10000) {
    return (num / 10000).toFixed(1) + 'W';
  } else if (num >= 1000) { 
    return (num / 1000).toFixed(1) + 'K';
  } else {
    return num;
  }
}

// 轮 24 起移除 `subStr`：作者名不再按「字符数」在应用层提前截断（7 个字符
// 会先把 4 字左右的中文名砍成省略号，而此时列宽往往还有富余）。截断统一交给
// CSS 的 text-overflow: ellipsis —— 按真实像素宽度决定，只有真正超出列宽才省略。

// 播放量/点赞量等计数：≥1万 → `x.x万`（1.1万），≥100万 → 去小数 `xx万`（112万）。
export const formatViewCount = (num: number) => {
  const value = Number.isFinite(num) ? Math.max(0, Math.floor(num)) : 0;
  if (value >= 1000000) return `${Math.floor(value / 10000)}万`;
  if (value >= 10000) return `${(value / 10000).toFixed(1)}万`;
  return String(value);
};

/** 把可能为 undefined / null / 非数字的值折成有限数；无效返回 null（区别于真实 0）。 */
const toFiniteOrNull = (num: unknown): number | null => {
  if (num === null || num === undefined || num === "") return null;
  const value = Number(num);
  return Number.isFinite(value) ? value : null;
};

/** 卡片 meta「播放量」列的选值（轮 22，问题 1 + 字段盘点）。
 *
 * 根因回顾：此前各列表写 `formatViewCount(Number(info.stat.play) || 0)`，
 * `|| 0` 把「字段缺失」与「真实 0」混为一谈，缺字段的行就渲染成假数据 `0`。
 *
 * 新规则：
 *  1. 播放量字段存在（非 null/undefined/NaN）→ 展示播放量（真实 0 也照实展示 0）；
 *  2. 播放量缺失 → 若同对象的弹幕数存在，则改展示弹幕数（icon 换成评论图标，语义不串）；
 *  3. 两者都缺 → value 为 null，CardMeta 整字段不渲染（绝不补 0）。
 */
export const viewsMetaField = (
  play: unknown,
  danmaku?: unknown,
): Array<{ kind: "views"; value: string | null; icon: "play" | "danmaku" }> => {
  const p = toFiniteOrNull(play);
  if (p !== null) return [{ kind: "views", value: formatViewCount(p), icon: "play" }];
  const d = toFiniteOrNull(danmaku);
  if (d !== null) return [{ kind: "views", value: formatViewCount(d), icon: "danmaku" }];
  return [{ kind: "views", value: null, icon: "play" }];
};

// 三键数值：≥1亿 → `x.x亿`，≥100万 → 整数万（23w），≥1万 → `x.x w`（1.1w）。
export const formatCompactCount = (num: number) => {
  const value = Number.isFinite(num) ? Math.max(0, Math.floor(num)) : 0;
  if (value >= 100000000) return `${(value / 100000000).toFixed(1)}亿`;
  if (value >= 1000000) return `${Math.floor(value / 10000)}w`;
  if (value >= 10000) return `${(value / 10000).toFixed(1)}w`;
  return String(value);
};

// 相对时间（轮 28 修订）：今天/昨天/N天前/N个月前，满一年后显示 yyyy-MM。
//
// 背景（用户实测截图）：此前阈值是 30 天，超过即回退到 10 字符的 `yyyy-MM-dd`。
// 卡片 meta 的发布时间列只占 30%（约 55px），`yyyy-MM-dd` 必然被省略号截成
// `2025-09-…` —— 既挤占昵称宽度、又几乎没有信息量。新策略让任何长度下日期列
// 都「要么短到放得下，要么信息完整」：
//   ≤ 30 天   → 今天 / 昨天 / N天前
//   31 天–1 年 → N个月前（最长「11个月前」= 5 字 ≈ 40px，30% 列宽可完整显示）
//   ≥ 1 年    → yyyy-MM（7 字符，比 yyyy-MM-dd 短 3 字符，同样能完整显示）
// 这样 8 个列表的日期列都不会再出现「被截断且无信息」的形态，也不再需要
// 为了放下完整年月日而挤压作者列。
export const formatRelativeTime = (timestamp: number) => {
  if (!Number.isFinite(timestamp) || timestamp <= 0) return "";
  const target = new Date(timestamp * 1000);
  const now = new Date();
  const startOfToday = new Date(now.getFullYear(), now.getMonth(), now.getDate()).getTime();
  const startOfTarget = new Date(
    target.getFullYear(),
    target.getMonth(),
    target.getDate(),
  ).getTime();
  const dayDiff = Math.round((startOfToday - startOfTarget) / 86400000);
  if (dayDiff <= 0) return "今天";
  if (dayDiff === 1) return "昨天";
  if (dayDiff <= 30) return `${dayDiff}天前`;
  // 自然月差（按「当月同日」对齐：还没到当月同一天就少算一个月）。
  let monthDiff =
    (now.getFullYear() - target.getFullYear()) * 12 + (now.getMonth() - target.getMonth());
  if (now.getDate() < target.getDate()) monthDiff -= 1;
  if (monthDiff < 12) {
    // dayDiff ≥ 30 但月差算成 0 的边界（如 1/1 → 1/31）夹到 1，避免「0个月前」。
    return `${Math.max(1, monthDiff)}个月前`;
  }
  return `${target.getFullYear()}-${String(target.getMonth() + 1).padStart(2, "0")}`;
};

/**
 * 卡片 meta「发布时间」列的统一入口（轮 28）。
 *
 * 把后端可能给出的「绝对日期字符串」（`yyyy-MM-dd` / `yyyy-MM-dd HH:mm:ss`，
 * 如 feedList / upVideoList 的 `pub_time`、searchList 的 `video.date`）折算成
 * formatRelativeTime 的短形态（N个月前 / yyyy-MM）；已是相对时间或无法识别的
 * 字符串原样返回（不丢信息）。与各列表已用的 formatRelativeTime 走同一套
 * 长度保证，因此日期列在任何卡片宽度下都不会被截断成无信息的 `2025-09-…`。
 */
export const formatMetaDate = (value?: string | null): string | null => {
  if (value === undefined || value === null || value === "") return null;
  const raw = String(value).trim();
  const m = raw.match(/^(\d{4})-(\d{2})-(\d{2})(?:[ T](\d{2}):(\d{2})(?::(\d{2}))?)?/);
  if (!m) return raw;
  const ts = Date.parse(
    `${m[1]}-${m[2]}-${m[3]}T${m[4] ?? "00"}:${m[5] ?? "00"}:${m[6] ?? "00"}`,
  );
  if (!Number.isFinite(ts)) return raw;
  return formatRelativeTime(ts / 1000);
};
