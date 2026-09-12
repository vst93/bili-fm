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

// 相对时间（轮 30 修订）：发布形态回调 —— 恢复「具体日期优先」。
//
// 背景：轮 28 为了迁就 30%（约 55px）的窄列，把日期一律压到 ≤7 字符
// （N个月前 / yyyy-MM）。但用户复测后指出：这样反而丢掉了「具体是哪天」的信息，
// 而卡片 meta 列的宽度其实有冗余（其余列内容常常很短）。
//
// 轮 30 的新策略：**能放多具体就放多具体**，放不下再由渲染层按真实宽度降级：
//   ≤ 0 天   → 今天
//   1 天     → 昨天
//   2–7 天   → N天前（相对时间对「新鲜」内容最直观，保留）
//   > 7 天，当年   → MM-DD（5 字符，最常见形态，30% 列宽必定放得下）
//   > 7 天，跨年   → yyyy-MM-DD（10 字符，尽量完整展示；放不下再降级）
// 降级不是按「字符数硬猜」，而是由上文的 dateFormLadder 给出「具体 → 抽象」
// 的候选阶梯，再由 cardMeta 的 PubDateField 实测每个候选的像素宽度，挑第一个
// 目测放得下的形态（详见 pickDateForm / pubdateCandidates）。
//
// 因此本函数返回**最具体**的形态；列表调用方照常渲染即可，真正「放不下就降级」
// 发生在卡片 meta 渲染层，绝不会渲染出「2025-09-…」这类被截断的半截日期。
export type DateFormKind =
  | "today"
  | "yesterday"
  | "days"
  | "md"
  | "ymd"
  | "ym"
  | "months"
  | "years"
  | "raw";

export interface DateFormCandidate {
  kind: DateFormKind;
  text: string;
}

const pad2 = (n: number) => String(n).padStart(2, "0");

/**
 * 「具体 → 抽象」的日期形态阶梯（轮 30 的核心）。
 *
 * 返回按信息量从高到低排列的候选数组；调用方（或渲染层）取第一个「放得下」
 * 的候选即可。最后一个候选是「必定放得下」的兜底。
 *
 * 传入 `now` 便于测试构造确定性的相对时间（生产环境省略即取当前时刻）。
 */
export const dateFormLadder = (timestamp: number, now: Date = new Date()): DateFormCandidate[] => {
  if (!Number.isFinite(timestamp) || timestamp <= 0) return [];
  const target = new Date(timestamp * 1000);
  const startOfToday = new Date(now.getFullYear(), now.getMonth(), now.getDate()).getTime();
  const startOfTarget = new Date(
    target.getFullYear(),
    target.getMonth(),
    target.getDate(),
  ).getTime();
  const dayDiff = Math.round((startOfToday - startOfTarget) / 86400000);
  if (dayDiff <= 0) return [{ kind: "today", text: "今天" }];
  if (dayDiff === 1) return [{ kind: "yesterday", text: "昨天" }];
  if (dayDiff <= 7) return [{ kind: "days", text: `${dayDiff}天前` }];

  // 自然月差（按「当月同日」对齐：还没到当月同一天就少算一个月），夹到 ≥1
  // 避免出现「0个月前」。
  let monthDiff =
    (now.getFullYear() - target.getFullYear()) * 12 + (now.getMonth() - target.getMonth());
  if (now.getDate() < target.getDate()) monthDiff -= 1;
  monthDiff = Math.max(1, monthDiff);

  // 年内（含跨年但不足一年）：兜底是 `N个月前`（1..11 个月 → ≤5 字符）。
  // ≥ 1 年：额外补一档 `N年前` 作为**必定放得下**的最终兜底（`N个月前` 在
  // 很老的日期上也会长到 6–7 字符）。
  const yearDiff = now.getFullYear() - target.getFullYear();
  const floor: DateFormCandidate[] = [{ kind: "months", text: `${monthDiff}个月前` }];
  if (yearDiff >= 1) floor.push({ kind: "years", text: `${yearDiff}年前` });

  const mmdd = `${pad2(target.getMonth() + 1)}-${pad2(target.getDate())}`;
  const yyyymmdd = `${target.getFullYear()}-${mmdd}`;
  const sameYear = target.getFullYear() === now.getFullYear();

  if (sameYear) {
    // 当年：MM-DD 已经比任何相对形态都具体，且列宽必定放得下。
    return [{ kind: "md", text: mmdd }, ...floor];
  }
  // 跨年：优先完整年月日；放不下才退回 yyyy-MM / N个月前 / N年前。
  return [
    { kind: "ymd", text: yyyymmdd },
    { kind: "ym", text: `${target.getFullYear()}-${pad2(target.getMonth() + 1)}` },
    ...floor,
  ];
};

/**
 * 从候选阶梯里挑第一个「放得下」的形态（轮 30）。
 *
 * `fits(text)` 由渲染层提供（按真实像素宽度判断）；若一个都不满足，则返回
 * 阶梯的最后一档（兜底，保证绝不返回空）。纯函数，便于单测。
 */
export const pickDateForm = <T extends { text: string }>(
  candidates: T[],
  fits: (text: string) => boolean,
): T => candidates.find((candidate) => fits(candidate.text)) ?? candidates[candidates.length - 1];

/**
 * 把「已经格式化的发布日期字符串」还原成候选阶梯（轮 30，供渲染层降级用）。
 *
 * 只有「完整 yyyy-MM-DD」这类可能放不下的形态需要降级；其余形态
 * （今天 / 昨天 / N天前 / MM-DD / N个月前 / yyyy-MM / 无法识别的原串）都足够短，
 * 直接作为唯一候选原样显示。
 */
export const pubdateCandidates = (value: string): DateFormCandidate[] => {
  const raw = String(value).trim();
  const m = raw.match(/^(\d{4})-(\d{2})-(\d{2})$/);
  if (!m) return [{ kind: "raw", text: raw }];
  const ts = Date.parse(`${m[1]}-${m[2]}-${m[3]}T00:00:00`);
  if (!Number.isFinite(ts)) return [{ kind: "raw", text: raw }];
  const ladder = dateFormLadder(ts / 1000);
  return ladder.length > 0 ? ladder : [{ kind: "raw", text: raw }];
};

/** 最具体的发布日期形态（生产环境默认入口）。 */
export const formatRelativeTime = (timestamp: number, now: Date = new Date()): string => {
  const ladder = dateFormLadder(timestamp, now);
  return ladder.length > 0 ? ladder[0].text : "";
};

/**
 * 卡片 meta「发布时间」列的统一入口（轮 30）。
 *
 * 把后端可能给出的「绝对日期字符串」（`yyyy-MM-dd` / `yyyy-MM-dd HH:mm:ss`，
 * 如 feedList / upVideoList 的 `pub_time`、searchList 的 `video.date`）折算成
 * formatRelativeTime 的**最具体**形态（跨年 yyyy-MM-DD / 当年 MM-DD / 相对时间）；
 * 已是相对时间或无法识别的字符串原样返回（不丢信息）。
 * 真正的「放不下就降级」由 cardMeta 的 PubDateField 按实测像素宽度完成。
 */
export const formatMetaDate = (value?: string | null, now: Date = new Date()): string | null => {
  if (value === undefined || value === null || value === "") return null;
  const raw = String(value).trim();
  const m = raw.match(/^(\d{4})-(\d{2})-(\d{2})(?:[ T](\d{2}):(\d{2})(?::(\d{2}))?)?/);
  if (!m) return raw;
  const ts = Date.parse(
    `${m[1]}-${m[2]}-${m[3]}T${m[4] ?? "00"}:${m[5] ?? "00"}:${m[6] ?? "00"}`,
  );
  if (!Number.isFinite(ts)) return raw;
  return formatRelativeTime(ts / 1000, now);
};
