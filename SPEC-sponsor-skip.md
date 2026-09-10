# SPEC: 自动跳过恰饭/广告片段（SponsorBlock 接入）

> 数据源：[小电视空降助手（BilibiliSponsorBlock）](https://github.com/hanydd/BilibiliSponsorBlock/wiki/API) 的公共 API `https://bsbsb.top/api/`。
> 该 API **免费、无需注册、无需 key**（2026-09-10 实测验证），CORS 全开（`access-control-allow-origin: *`），前端可直连，**不需要动 Rust 端**。
> 覆盖率说明：社区众包数据，热门视频/音乐区覆盖较好，冷门视频可能无数据（接口返回 `[]`），必须无感降级。

## 目标

播放音频时自动跳过 B 站视频中的恰饭/广告等片段，提供开关，默认开启。跳过行为只在音频播放时间线上发生，不影响 UI 其他部分。

## 接口

```
GET https://bsbsb.top/api/skipSegments?videoID={bvid}&cid={cid}&categories=["sponsor","selfpromo","interaction"]
```

- 返回 JSON 数组（无数据时 `[]`，参数错 400）：

```json
[{
  "segment": [84.672, 129.603],   // 起止秒
  "category": "sponsor",
  "actionType": "skip",           // 只处理 skip
  "UUID": "804b...",
  "videoDuration": 169.866,       // 提交时视频时长，用于过期校验（±2s 容差）
  "votes": 0,
  "locked": 0
}]
```

- GET 带 nginx 缓存，正常调用即可，**不要用 `x-skip-cache` 强刷**
- 类别字典：`sponsor`=恰饭推广、`selfpromo`=自我推广、`interaction`=求关注三连、`intro`=片头、`outro`=片尾、`preview`=预告/回顾、`music_offtopic`=非音乐段落
- `categories` 参数格式是带方括号的 URL 参数，如 `categories=%5B%22sponsor%22%5D`

## 设置项（2 个开关，localStorage，沿用现有 `=== "true"` 模式）

| key | 默认 | 控制类别 |
|---|---|---|
| `sponsorSkipEnabled` | `true` | 总开关；启用时请求 `sponsor, selfpromo, interaction` |
| `sponsorSkipExtras` | `false` | 追加 `intro, outro, preview, music_offtopic` |

设置入口放在现有设置面板中，文案：「自动跳过广告片段」「同时跳过片头片尾」。开关切换只影响下一次加载的视频，当前播放不受影响（可接受，不需要热重载 segments）。

## 实现要点

### 1. 新建 `src/lib/sponsorBlock.ts`

```ts
export type SponsorSegment = {
  segment: [number, number];
  category: string;
  actionType: string;
  UUID: string;
  videoDuration: number;
};

export async function fetchSegments(bvid: string, cid: number, signal?: AbortSignal): Promise<SponsorSegment[]>
```

- `AbortController` 超时 5s；任何失败（网络/超时/非 200）**静默返回 `[]`，绝不抛错影响播放**
- 会话级 `Map` 缓存，key `${bvid}:${cid}`，防止同一视频反复请求；不做磁盘持久化
- 原生 `fetch`，不引入新依赖

### 2. `src/components/player.tsx`

- 新增 prop `bvid?: string`（列表数据均含 bvid，由 `src/pages/index.tsx` 播放调用处传入）
- `mediaKey`（现有，`${src}:${aid}:${cid}`）变化时：开关开启且 `bvid` 存在 → `fetchSegments`，存入 state；新请求发出前 abort 上一个
- **跳过判定**（在现有 `onTimeUpdate` 内，segments 非空才进入判断）：
  - `currentTime` 落入某 segment 的 `[start - 0.25, end)` → 调现有 `safeSeek(audio, end + 0.1)`；`end + 0.1` 不在区间内，不会死循环
  - 过期校验：`audio.duration` 就绪时（`!isNaN` 且 > 0），若 `|audio.duration - seg.videoDuration| > 2` 则丢弃该 segment
  - 过滤 `actionType !== "skip"`、`end <= start`、`start >= audio.duration` 的 segment
  - 每 segment 只自动跳一次：跳过后标记已武装解除；若用户手动 seek 回到该 segment 前方 3s 以外，重新武装（允许再次跳过）
  - 首次跳过某 segment 时 Toast 提示一次（复用现有 Toast，≤3s）：`已跳过 恰饭推广 · SponsorBlock`（文案用类别字典）
- segments 为空 / 开关关闭：零开销，不进入任何判断分支

### 3. 约束

- 只改前端：`src/lib/sponsorBlock.ts`、`src/components/player.tsx`、`src/pages/index.tsx`（传 bvid）、设置面板（`src/components/titleBar.tsx` 或其设置对话框所在文件）
- 不新增依赖；不动 Rust 端；不改无关样式
- `npm run build` 必须通过
- 若 `tests/` 可 mock `fetch`，补一条「命中区间 → currentTime 越过 end」的跳过逻辑测试

## 验收清单

1. 播放 `BV1xsYn6pEw4`（84.672–129.603 有 sponsor 片段，cid 41706655566），播放跨过 84s 时自动跳到 ~129.7s，Toast 提示一次
2. 关闭总开关：不发请求、不跳过
3. 开启「片头片尾」开关：请求 categories 含 intro/outro
4. 断网播放：完全正常，无报错无卡顿
5. 多 P 视频切换分 P：重新拉取新 cid 的 segments
6. 播放无数据的冷门视频：无任何异常表现
