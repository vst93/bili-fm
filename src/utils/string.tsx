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

export const graftingImage = (img: string, width = 400) => {
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

export const subStr = (str: string, len: number) => {
  if (str.length > len) {
    return str.slice(0, len) + '..';
  } else {
    return str;
  }
}

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

// 相对时间：今天/昨天/N天前，超过 30 天显示 yyyy-MM-dd。
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
  return `${target.getFullYear()}-${String(target.getMonth() + 1).padStart(2, "0")}-${String(target.getDate()).padStart(2, "0")}`;
};
