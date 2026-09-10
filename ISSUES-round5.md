# 视频播放页问题清单（round 5）

> 以下问题基于 2.0.12 预览版在 Linux 下的实测反馈。涉及文件主要在 `src/styles/globals.css`，问题 4 可能涉及 `src/components/titleBar.tsx`。

## 1. 「当前选集」胶囊内容被截断

「当前选集」玻璃胶囊（`.video-part-pill`，含「播放列表/当前选集」前缀和选集标题）在内容过长时直接被截断，标题显示不完整。

- 胶囊当前是 `overflow: visible; contain: layout; width: min(100%, 430px)`，内部标题 `.video-part-title` 带省略号规则
- 期望：内容过长时在胶囊内部以省略号优雅截断，胶囊永不溢出其父容器

## 2. 视频标题和简介被遮挡

视频标题（`#video-title`）和简介文字（`#video-desc`）看起来被部分遮挡/隐藏。

- `#video-desc` 目前是 `max-height: none; overflow: visible`，底部有一个 24px 渐隐 mask（`mask-image: linear-gradient(to bottom, #000 0, #000 calc(100% - 24px), transparent 100%)`）
- 没有 高度上限时，渐隐会直接吃掉真实文字且无法查看，长简介还会把布局撑开
- 期望：恢复合理的高度上限，同时保留底部渐隐融合效果（渐隐本身是对的，缺的是上限）

## 3. 底部操作按钮行内容被隐藏

点赞/投币/收藏等操作行（`.video-content-actions`、`.video-context-dock`，首页的 `.home-global-actions` 同构）有部分内容被隐藏。

- 这些容器上有一层 `::before` 接触淡影 overlay，以及子元素的 z-index 处理
- 期望：排查 overlay / z-index 是否盖住了可交互子元素（overlay 必须 `pointer-events: none` 且不遮挡按钮）；给操作行加上合理的宽/高上限，flex 换行时高度不能无限增长

## 4. 视频播放模式下顶栏叠加在视频上

视频播放模式下，自定义顶栏（`.app-title-bar`）以及 macOS 的系统标题栏会叠在视频窗口上。可以隐藏，但有两个硬性要求：

1. 鼠标悬浮到视频顶部时，仍要能唤出可拖动的标题栏区域
2. 唤出后整个窗口依然可以通过该区域拖动（顶栏用的是 Tauri 的 `data-tauri-drag-region`）

- 已有的 `body.video-open` 主题块（含 `.app-title-bar-close:hover::after` 特例）需要保持可用
- macOS 原生装饰路径如有平台限制，请注明

## 通用约束

- 保持液态玻璃质感，不新增 `backdrop-filter`、不加大 blur、不用 `will-change`
- 完成后 `npm run build` 需通过
