# PLAN / CSS 渲染压力优化（不增加渲染压力的前提下的样式优化）

> 背景：Liquid Glass 风格已定（凸起玻璃 / 内凹井 / 同心圆角），本轮目标是在**不增加**渲染开销的前提下继续优化，
> 其中第 1、2 项预期是净降低。方案已与使用者确认（2025 选定 1/2/3），尚未实施。
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
- [ ] `.nav-icon-btn`（行 2033 transition；hover 阴影在 2073 附近）
- [ ] `.search-submit-btn`（行 1357）
- [ ] `.top-tool-btn`（行 1424，含 1430 的 filter 过渡）
- [ ] `.video-content-actions` / `.video-context-dock` 内的按钮走 `.nav-icon-btn` 改造自动受益

P1（中频：窗口控制与封面）：
- [ ] `.mini-window-controls button`（行 1149）
- [ ] `.app-title-bar-btn`（行 1166）
- [ ] `#switch-window-mode`（行 2739）
- [ ] `#video-cover`（行 372 / 1595 / 4931，播放态阴影过渡）
- [ ] `#video-owner-face`（行 1798）

P2（低频：页面级控件，批量套同一配方）：
- [ ] `[data-slot="wrapper"]` 系列：2989 / 3052 / 3077 / 3114 / 3134 / 3230 / 3338 / 3450 / 3990
- [ ] `.history-incognito-switch`（3611）、`.danmaku-time-label`（3776）、`.danmaku-message-row`（3829）
- [ ] `.liquid-glass`（710）与 `.video-part-pill`（1917）：hover 只变边框色+阴影，属容器级，可保留或删
      box-shadow 过渡只留 border-color
- [ ] 跳过：`#player input.player-volume-slider::-webkit-slider-thumb`（2589，input 伪元素受限，收益小）
- [ ] 跳过：`#settings-menu`（1510 行 transition?）、菜单/弹层类，出现频率低

---

## 任务 2：backdrop-filter 审计（降 GPU，每处一个离屏合成）

### 判定原则
- 元素悬浮在**会变化的内容**（视频、列表滚动区）之上 → 保留 blur
- 元素底下是**静态面板/纯色**，或父级已有一层 blur → 去掉 blur，换成略提高不透明度的
  纯色/静态渐变（视觉几乎无差）
- `none` 覆盖型（本来就是优化）→ 不动

### 逐项判断（21 处，行号为快照）
保留：
- [ ] 1296 `.home-searchbar::before`（var(--liquid-blur)）— 核心玻璃面，底下是动态背景
- [ ] 2145 `#player .player-controls` — 悬浮在视频画面上
- [ ] 2642 `#player .player-volume-popover`、2680 `.player-speed-popover`(blur 10px) — 弹层在视频上
- [ ] 4217 `body.mini-mode #player .player-controls` — 同上（mini 模式）
- [ ] 2845 `[data-slot="wrapper"] > section::before` — 播放列表页侧栏，底下是滚动内容
- [ ] 107 `.lazy-drawer-loading-card` — 加载卡浮在内容上（瞬态，可不改）
- [ ] 4825 `.liquid-dialog` — 模态浮在任意内容上
- [ ] 4586 `.login-glass` — 覆盖层，可保留；顺手可把 28px 降到 20px（可选微调）
- [ ] 4560 `.toast-glass` — toast 浮在内容上，保留
- [ ] 1510 `#settings-menu` — 菜单浮在内容上，保留
- [ ] 223 / 2811 / 4507 — `none` 覆盖型，不动

去掉（换成静态底色/半透明，父级已有 blur 或底下是静态面板）：
- [ ] 3545 `.watchlater-remove-btn`（blur 8px，小按钮，静态底）
- [ ] 4314 `.video-owner-picker-menu .video-owner-picker-item`（blur 20px，**双重模糊**：
      菜单容器 1708 已有 blur，item 再来一层纯属浪费）
- [ ] 4615 `.login-close-btn`、4638 `.login-qr-card`（都坐在 .login-glass 已模糊的面上，双重模糊）
- [ ] 3128 `[data-slot="wrapper"] header .part-search::before`、3355 `header [data-slot="tab"]::before`
      （blur-thin 用于小控件，静态底，换半透明底色）

### 验证
每处去掉 blur 前后截图对比；若发现透出内容导致可读性下降，回退该项。

---

## 任务 3：contain 隔离（零成本）

- [ ] `.video-part-pill` 加 `contain: layout paint`（自身已 overflow:hidden，无溢出子元素，安全）
- [ ] `.video-content-actions`、`.video-context-dock`、`.home-global-actions` 加 `contain: layout`
  - **不要加 paint**：dock 的 `.nav-badge` 有 `-4px` 外溢、按钮 hover 有 `translateY(-1px)`，
    paint containment 会把溢出部分裁掉
- [ ] `.home-searchbar` 加 `contain: layout`（同理不加 paint，rim/shadow 需要完整绘制）

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
