# PLAN / CSS 渲染压力优化（不增加渲染压力的前提下的样式优化）

> 背景：Liquid Glass 风格已定（凸起玻璃 / 内凹井 / 同心圆角），本轮目标是在**不增加**渲染开销的前提下继续优化，
> 其中第 1、2 项预期是净降低。方案已与使用者确认（2025 选定 1/2/3），本轮已按评估结果实施。
> 行号基于当前工作区快照（globals.css，约 5000 行），行号会漂移，**以选择器文本为准**。

---

## 任务 1：hover 的 box-shadow 过渡改伪元素 opacity（降 paint）

### 原理
`transition: box-shadow` 每帧触发重绘；把 hover 阴影画到 `::after` 上、静止 `opacity: 0`、hover 只过渡
`opacity`（合成器处理），hover 期间的逐帧 paint 消失，视觉零差异。

### 实施配方（每个按钮类照抄）
```css
/* 元素本体 */
position: relative;
isolation: isolate;            /* 多数按钮已有 */
transition: color/background-color/opacity/transform ...;  /* 从 transition 里删掉 box-shadow */

/* 新增阴影载体 */
.btn-class::after {
    content: "";
    position: absolute;
    inset: 0;
    border-radius: inherit;
    box-shadow: <原 hover 态阴影>;
    opacity: 0;
    pointer-events: none;
    z-index: 0;                /* 图标子元素已是 z-index:1，不会被盖住 */
    transition: opacity 0.18s ease;
}
.btn-class:hover::after { opacity: 1; }
```

### 注意事项
- `.home-user-avatar` 是 `<img>`（HeroUI Avatar），**伪元素对 img 无效**，标记为例外：保留其
  box-shadow 过渡（低频 hover，可接受），或后续给 HeroUI Avatar 传 wrapper class 再改。
- `:active` 的按压阴影（inset）保留现状：按压是瞬态，不必优化。
- `.top-tool-btn > *` 的 `transition: filter`（drop-shadow）顺手改成同 recipe（或直接删，
  视觉差异 1px 级）。
- 加 `::after` 前确认该类没有已占用的伪元素：`.nav-icon-btn` / `.search-submit-btn` / `.top-tool-btn` /
  `.home-searchbar` 均未占用（`.home-searchbar` 的 ::before/::after 被玻璃层占用，见下）。
- `.home-searchbar` / `.home-global-actions` 的 ::before/::after 已被玻璃材质层占用，若要改其
  hover lifted-shadow，需另加一层真实子元素或保留现状（低频，建议保留现状）。

### 改造清单（按 hover 频率排优先级）
P0（高频：搜索框模块 + 信息面板，本轮必做）：
- [x] `.nav-icon-btn`（改为 `::after` 阴影 + opacity 过渡）
- [x] `.search-submit-btn`（改为 `::after` 阴影 + opacity 过渡）
- [x] `.top-tool-btn`（改为 `::after` 阴影 + opacity 过渡，移除图标 filter hover）
- [x] `.video-content-actions` / `.video-context-dock` 内的按钮走 `.nav-icon-btn` 改造自动受益

P1（中频：窗口控制与封面）：
- [x] `.mini-window-controls button`（改为 `::after` 阴影 + opacity 过渡）
- [x] `.app-title-bar-btn`（改为 `::after` 阴影 + opacity 过渡）
- [x] `#switch-window-mode`（改为 `::after` 阴影 + opacity 过渡）
- [ ] `#video-cover`（跳过：自身 `overflow:hidden` 会裁掉伪元素外部阴影，且播放态/hover 态规则有覆盖关系）
- [ ] `#video-owner-face`（跳过：`RetryImg` 最终渲染 `<img>`，图片伪元素无效）

P2（低频：页面级控件，批量套同一配方）：
- [ ] `[data-slot="wrapper"]` 系列：跳过，卡片已占用 `::before` 指针光且阴影由动态 `--hover-strength` 驱动，批量替换会改变层叠/视觉
- [ ] `.history-incognito-switch`、`.danmaku-time-label`、`.danmaku-message-row`：跳过，hover 不改变 box-shadow，现有过渡用于状态切换
- [ ] `.liquid-glass` 与 `.video-part-pill`：跳过，前者 `::after` 已占用 rim 光，后者没有独立 hover 阴影规则
- [ ] 跳过：`#player input.player-volume-slider::-webkit-slider-thumb`（2589，input 伪元素受限，收益小）
- [ ] 跳过：`#settings-menu`（1510 行 transition?）、菜单/弹层类，出现频率低

---

## 任务 2：backdrop-filter 审计（降 GPU，每处一个离屏合成）

### 判定原则
- 元素悬浮在**会变化的内容**（视频、列表滚动区）之上 → 保留 blur
- 元素底下是**静态面板/纯色**，或父级已有一层 blur → 去掉 blur，换成略提高不透明度的
  纯色/静态渐变（视觉几乎无差）
- `none` 覆盖型（本来就是优化）→ 不动

### 逐项判断（按当前选择器核对，旧行号仅作参考）
保留：
- [x] `.home-searchbar::before`（`var(--liquid-blur)`）— 核心玻璃面，底下是动态背景
- [x] `#player .player-controls` — 悬浮在视频画面上
- [x] `#player .player-volume-popover`、`.player-speed-popover` — 弹层在视频上
- [x] `body.mini-mode #player .player-controls` — 同上（mini 模式）
- [x] `[data-slot="wrapper"] > section::before` — 播放列表页侧栏，底下是滚动内容
- [x] `.lazy-drawer-loading-card` — 加载卡浮在内容上（瞬态）
- [x] `.liquid-dialog` — 模态浮在任意内容上
- [x] `.login-glass` — 覆盖层保留，28px 未做可选降级
- [x] `.toast-glass` — toast 浮在内容上
- [x] `#settings-menu` — 菜单浮在内容上
- [x] `none` 覆盖型规则 — 不动

去掉（换成静态底色/半透明，父级已有 blur 或底下是静态面板）：
- [x] `.watchlater-remove-btn`（移除 blur 8px，静态底色不透明度 0.56 → 0.62）
- [x] `.video-owner-picker-menu .video-owner-picker-item`（核对后当前无 `backdrop-filter` 规则，标记为无需修改；菜单容器自身 blur 保留）
- [x] `.login-close-btn`、`.login-qr-card`（移除子层 blur，父级 `.login-glass` 保留）
- [x] `[data-slot="wrapper"] header .part-search::before`、`header [data-slot="tab"]::before`
      （移除内层 blur，保留静态底色/渐变）

### 验证
每处去掉 blur 前后截图对比；若发现透出内容导致可读性下降，回退该项。

---

## 任务 3：contain 隔离（零成本）

- [x] `.video-part-pill` 加 `contain: layout paint`（自身已 overflow:hidden，无溢出子元素，安全）
- [x] `.video-content-actions`、`.video-context-dock`、`.home-global-actions` 加 `contain: layout`
  - **不要加 paint**：dock 的 `.nav-badge` 有 `-4px` 外溢、按钮 hover 有 `translateY(-1px)`，
    paint containment 会把溢出部分裁掉
- [x] `.home-searchbar` 加 `contain: layout`（同理不加 paint，rim/shadow 需要完整绘制）

---

## 明确不做（会显著增加渲染压力）
- 加大 blur 半径、新增 backdrop-filter 层
- 全窗口 SVG displacement 滤镜模拟 lensing
- 给大面板加 `will-change`（常驻合成层内存）

## 验证步骤（每完成任务后）
1. `npm run build` 通过
2. Playwright 截图对比（改前先存 baseline）：搜索框模块、信息面板、dock hover 态、菜单、登录页
3. 手动 Chrome DevTools → Rendering → Paint flashing：hover 搜索按钮/图标按钮，
   确认不再出现大面积绿色重绘闪烁
4. 任务 2 每删一处 blur：截图对比该元素的静态观感与可读性

## 建议实施顺序
任务 3（10 分钟，零风险）→ 任务 1 的 P0（核心收益）→ 任务 2 的 4 处"双重模糊"（收益明确）→
任务 2 其余 → 任务 1 的 P1/P2 分批。

## 本轮评估与实施结果（2026-09-09）

- 任务 3：照做。四个布局容器已增加 containment，`.video-part-pill` 使用 `layout paint`，其余只使用 `layout`，未裁切外溢绘制。
- 任务 1：P0 三类按钮及窗口控制按钮照做，hover 阴影移到 `::after` 并只过渡 opacity；`.top-tool-btn` 的图标 filter hover 已移除。`#video-cover` 因 `overflow:hidden` 会裁掉伪元素外部阴影且播放态规则覆盖 hover，保留原 box-shadow 过渡；`#video-owner-face` 为图片元素，跳过伪元素方案。P2 选择器或已有伪元素/状态阴影，或没有 hover 阴影变化，均跳过以避免层叠和视觉风险。
- 任务 2：移除 `.watchlater-remove-btn`、登录关闭按钮、二维码卡片以及抽屉 header 搜索框/Tab 的内层 blur；菜单 item 选择器当前没有 `backdrop-filter`，仅记录核对结果，菜单容器 blur 保留。播放器、抽屉主体、toast、dialog、登录主面板等仍覆盖动态内容，继续保留 blur；登录主面板 28px → 20px 的可选调整未采用。
- 每个实施阶段均运行 `npm run build`，结果通过。
