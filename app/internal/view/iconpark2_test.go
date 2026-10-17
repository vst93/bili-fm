package view

import (
	"bytes"
	"os"
	"testing"

	"github.com/egoist/mygo/ui"
)

// TestIconParkSecondBatchPathsVerbatim 逐字钉住本轮新换的 icon-park 原始 path
// （标题栏 / 抽屉表头 / 播放列表 / 模态 / toast / 迷你窗 / 封面开关一组）。
// 数据来自 @icon-park/react 1.4.2 的 icons/*.js（48×48 画布、描边 4）。
func TestIconParkSecondBatchPathsVerbatim(t *testing.T) {
	cases := []struct{ name, got, want string }{
		{"Left", parkLeft, `<path d="M31 36L19 24L31 12"/>`},
		{"Right", parkRight, `<path d="M19 12L31 24L19 36"/>`},
		{"Close", parkClose, `<path d="M8 8L40 40"/><path d="M8 40L40 8"/>`},
		{"CloseSmall", parkCloseSmall, `<path d="M14 14L34 34"/><path d="M14 34L34 14"/>`},
		{"Check", parkCheck, `<path d="M43 11L16.875 37L5 25.1818"/>`},
		{"Refresh", parkRefresh, `<path d="M42 8V24"/><path d="M6 24L6 40"/><path d="M42 24C42 14.0589 33.9411 6 24 6C18.9145 6 14.3216 8.10896 11.0481 11.5M6 24C6 33.9411 14.0589 42 24 42C28.8556 42 33.2622 40.0774 36.5 36.9519"/>`},
		{"Text", parkText, `<rect x="6" y="6" width="36" height="36" rx="3"/><path d="M16 19V16H32V19"/><path d="M22 34H26"/><path d="M24 18L24 34"/>`},
		{"Connection", parkConnection, `<path d="M39 34L44 39L39 44"/><path d="M8 12C10.2091 12 12 10.2091 12 8C12 5.79086 10.2091 4 8 4C5.79086 4 4 5.79086 4 8C4 10.2091 5.79086 12 8 12Z"/><path d="M12 8L20 8C22.2091 8 24 9.79086 24 12V35C24 37.2091 25.7909 39 28 39H44"/>`},
		{"Delete", parkDelete, `<path d="M9 10V44H39V10H9Z"/><path d="M20 20V33"/><path d="M28 20V33"/><path d="M4 10H44"/><path d="M16 10L19.289 4H28.7771L32 10H16Z"/>`},
		{"Order", parkOrder, `<path d="M33.0499 7H38C39.1046 7 40 7.89543 40 9V42C40 43.1046 39.1046 44 38 44H10C8.89543 44 8 43.1046 8 42L8 9C8 7.89543 8.89543 7 10 7H16H17V10H31V7H33.0499Z"/><rect x="17" y="4" width="14" height="6"/><path d="M26.9996 19L19 27.0012H29.004L21.0003 35.0018"/>`},
		{"LoopOnce", parkLoopOnce, `<path d="M43.8233 25.2305C43.7019 25.9889 43.5195 26.727 43.2814 27.4395C42.763 28.9914 41.9801 30.4222 40.9863 31.6785C38.4222 34.9201 34.454 37 30 37H16C9.39697 37 4 31.6785 4 25C4 18.3502 9.39624 13 16 13H44"/><path d="M38 7L44 13L38 19"/>`},
		{"ShuffleOne", parkShuffleOne, `<path d="M40 33L44 37L40 41"/><path d="M40 7L44 11L40 15"/><path d="M44 11H37C29.8203 11 24 16.8203 24 24C24 31.1797 29.8203 37 37 37H44"/><path d="M4 37H11C18.1797 37 24 31.1797 24 24C24 16.8203 18.1797 11 11 11H4"/>`},
		{"FocusOne", parkFocusOne, `<path d="M16 6H8C6.89543 6 6 6.89543 6 8V16"/><path d="M16 42H8C6.89543 42 6 41.1046 6 40V32"/><path d="M32 42H40C41.1046 42 42 41.1046 42 40V32"/><path d="M32 6H40C41.1046 6 42 6.89543 42 8V16"/><path d="M24 31C27.866 31 31 27.866 31 24C31 20.134 27.866 17 24 17C20.134 17 17 20.134 17 24C17 27.866 20.134 31 24 31Z"/><path d="M24 17L24 13"/><path d="M24 35L24 31"/><path d="M35 24H31"/><path d="M17 24H13"/><path fill="currentColor" stroke="none" d="M24 26C25.1046 26 26 25.1046 26 24C26 22.8954 25.1046 22 24 22C22.8954 22 22 22.8954 22 24C22 25.1046 22.8954 26 24 26Z"/>`},
		{"MaskOne", parkMaskOne, `<path d="M7.36573 16.3443C6.48571 18.7024 6 21.2881 6 23.9998C6 26.7115 6.48571 29.2971 7.36573 31.6552M11.2721 38.1419C12.9301 39.9841 14.91 41.4686 17.1101 42.4823C19.2324 43.4601 21.5595 43.9998 24 43.9998C26.4405 43.9998 28.7676 43.4601 30.8899 42.4823C33.09 41.4686 35.0699 39.9841 36.7279 38.1419M40.6343 31.6552C41.5143 29.2971 42 26.7115 42 23.9998C42 21.2881 41.5143 18.7024 40.6343 16.3443M36.7279 9.85762C35.0699 8.01539 33.09 6.53089 30.8899 5.51723C28.7676 4.53943 26.4405 3.99976 24 3.99976C19.0294 3.99976 14.5294 6.23833 11.2721 9.85762"/><path d="M18 22.9998C19.6569 22.9998 21 21.6566 21 19.9998C21 18.3429 19.6569 16.9998 18 16.9998C16.3431 16.9998 15 18.3429 15 19.9998C15 21.6566 16.3431 22.9998 18 22.9998Z"/><path d="M30 22.9998C31.6569 22.9998 33 21.6566 33 19.9998C33 18.3429 31.6569 16.9998 30 16.9998C28.3431 16.9998 27 18.3429 27 19.9998C27 21.6566 28.3431 22.9998 30 22.9998Z"/><path d="M24 34.9998C26.7614 34.9998 29 33.6566 29 31.9998C29 30.3429 26.7614 28.9998 24 28.9998C21.2386 28.9998 19 30.3429 19 31.9998C19 33.6566 21.2386 34.9998 24 34.9998Z"/>`},
		{"AddOne", parkAddOne, `<path d="M24 44C35.0457 44 44 35.0457 44 24C44 12.9543 35.0457 4 24 4C12.9543 4 4 12.9543 4 24C4 35.0457 12.9543 44 24 44Z"/><path d="M24 16V32"/><path d="M16 24L32 24"/>`},
		{"Add", parkAdd, `<rect x="6" y="6" width="36" height="36" rx="3"/><path d="M24 16V32"/><path d="M16 24L32 24"/>`},
		{"Pushpin", parkPushpin, `<path d="M32 4H16L20 7L16 20C16 20 10 24 10 28H20L24 44L28 28H38C38 24 34 21.1667 32 20L28 7L32 4Z"/>`},
		{"Peoples", parkPeoples, `<path d="M19 20C22.866 20 26 16.866 26 13C26 9.13401 22.866 6 19 6C15.134 6 12 9.13401 12 13C12 16.866 15.134 20 19 20Z"/><path d="M32.6077 7C34.6405 8.2249 36.0001 10.4537 36.0001 13C36.0001 15.5463 34.6405 17.7751 32.6077 19"/><path d="M4 40.8V42H34V40.8C34 36.3196 34 34.0794 33.1281 32.3681C32.3611 30.8628 31.1372 29.6389 29.6319 28.8719C27.9206 28 25.6804 28 21.2 28H16.8C12.3196 28 10.0794 28 8.36808 28.8719C6.86278 29.6389 5.63893 30.8628 4.87195 32.3681C4 34.0794 4 36.3196 4 40.8Z"/><path d="M43.9999 42.0001V40.8001C43.9999 36.3197 43.9999 34.0795 43.128 32.3682C42.361 30.8629 41.1371 29.6391 39.6318 28.8721"/>`},
		{"ZoomInternal", parkZoomInternal, `<path d="M44 4H4V44H44V4Z"/><path d="M16 4V16H4"/><path d="M36 24V36H24"/><path d="M36 36L24 24"/><path d="M4 6V26"/><path d="M7 4H27"/>`},
		{"Minus", parkMinus, `<path d="M10.5 24L38.5 24"/>`},
		{"Halo", parkHalo, `<path d="M24 28C26.2091 28 28 26.2091 28 24C28 21.7909 26.2091 20 24 20C21.7909 20 20 21.7909 20 24C20 26.2091 21.7909 28 24 28Z"/><path d="M17.3644 14C16.6922 14.4469 16.0676 14.9598 15.5 15.5295C14.9041 16.1274 14.3709 16.7878 13.9111 17.5"/><path d="M12.1975 26.1802C12.0678 25.4732 12 24.7447 12 24.0002C12 23.2558 12.0678 22.5272 12.1975 21.8203"/><path d="M17.3644 34C16.6922 33.5531 16.0676 33.0402 15.5 32.4705C14.9041 31.8726 14.3709 31.2122 13.9111 30.5"/><path d="M26.3598 35.7681C25.5968 35.9202 24.8077 36 24 36C23.1922 36 22.4031 35.9202 21.6401 35.7681"/><path d="M30.6357 34C31.3079 33.5531 31.9325 33.0402 32.5002 32.4705C33.096 31.8726 33.6292 31.2122 34.089 30.5"/><path d="M35.8027 26.1802C35.9325 25.4732 36.0003 24.7447 36.0003 24.0002C36.0003 23.2558 35.9325 22.5272 35.8027 21.8203"/><path d="M30.6357 14C31.3079 14.4469 31.9325 14.9598 32.5002 15.5295C33.096 16.1274 33.6292 16.7878 34.089 17.5"/><path d="M21.6401 12.232C22.4031 12.0798 23.1922 12 23.9999 12C24.8077 12 25.5968 12.0798 26.3598 12.232"/><path d="M44 24C44 23.325 43.9666 22.6578 43.9012 22C43.834 21.3228 43.733 20.6556 43.5999 20M4 24C4 23.325 4.03344 22.6578 4.09875 22C4.16598 21.3228 4.26699 20.6556 4.40007 20M38.646 37.6198C38.1967 38.1027 37.7238 38.5634 37.2291 39C36.7864 39.3908 36.3262 39.7623 35.8498 40.1132M9.35403 37.6198C9.80332 38.1027 10.2762 38.5634 10.7709 39C11.2136 39.3908 11.6739 39.7623 12.1502 40.1132M15.7124 5.79255C15.0569 6.09142 14.4203 6.42465 13.805 6.78994C13.2345 7.12862 12.6823 7.49486 12.1502 7.88683M32.2876 5.79255C32.9431 6.09142 33.5797 6.42465 34.195 6.78994C34.7655 7.12862 35.3177 7.49486 35.8498 7.88683M26.2465 4.12477C25.5091 4.04233 24.7595 4 24 4C23.2405 4 22.4909 4.04233 21.7535 4.12477M26.2465 43.8752C26.999 43.7911 27.7389 43.6653 28.4638 43.5C29.1569 43.342 29.8364 43.148 30.5 42.92M21.7535 43.8752C21.001 43.7911 20.2611 43.6653 19.5362 43.5C18.8431 43.342 18.1636 43.148 17.5 42.92M8.15495 11.7945C7.77626 12.2854 7.41988 12.7944 7.08732 13.3199C6.78763 13.7935 6.50729 14.2805 6.2474 14.7799M39.8451 11.7945C40.2237 12.2854 40.5801 12.7944 40.9127 13.3199C41.2124 13.7935 41.4927 14.2805 41.7526 14.7799M6.2474 33.2201C5.95677 32.6617 5.69172 32.0878 5.45379 31.5C5.2355 30.9607 5.04005 30.4098 4.86863 29.8483M41.7526 33.2201C42.0432 32.6617 42.3083 32.0878 42.5462 31.5C42.7645 30.9607 42.96 30.4098 43.1314 29.8483"/>`},
		{"CheckOne", parkCheckOne, `<path d="M24 44C29.5228 44 34.5228 41.7614 38.1421 38.1421C41.7614 34.5228 44 29.5228 44 24C44 18.4772 41.7614 13.4772 38.1421 9.85786C34.5228 6.23858 29.5228 4 24 4C18.4772 4 13.4772 6.23858 9.85786 9.85786C6.23858 13.4772 4 18.4772 4 24C4 29.5228 6.23858 34.5228 9.85786 38.1421C13.4772 41.7614 18.4772 44 24 44Z"/><path d="M16 24L22 30L34 18"/>`},
		{"CloseOne", parkCloseOne, `<path d="M24 44C35.0457 44 44 35.0457 44 24C44 12.9543 35.0457 4 24 4C12.9543 4 4 12.9543 4 24C4 35.0457 12.9543 44 24 44Z"/><path d="M29.6567 18.3432L18.343 29.6569"/><path d="M18.3433 18.3432L29.657 29.6569"/>`},
		{"Attention", parkAttention, `<path d="M24 44C29.5228 44 34.5228 41.7614 38.1421 38.1421C41.7614 34.5228 44 29.5228 44 24C44 18.4772 41.7614 13.4772 38.1421 9.85786C34.5228 6.23858 29.5228 4 24 4C18.4772 4 13.4772 6.23858 9.85786 9.85786C6.23858 13.4772 4 18.4772 4 24C4 29.5228 6.23858 34.5228 9.85786 38.1421C13.4772 41.7614 18.4772 44 24 44Z"/><path fill="currentColor" stroke="none" d="M24 37C25.3807 37 26.5 35.8807 26.5 34.5C26.5 33.1193 25.3807 32 24 32C22.6193 32 21.5 33.1193 21.5 34.5C21.5 35.8807 22.6193 37 24 37Z"/><path d="M24 12V28"/>`},
		{"Info", parkInfo, `<path d="M24 44C29.5228 44 34.5228 41.7614 38.1421 38.1421C41.7614 34.5228 44 29.5228 44 24C44 18.4772 41.7614 13.4772 38.1421 9.85786C34.5228 6.23858 29.5228 4 24 4C18.4772 4 13.4772 6.23858 9.85786 9.85786C6.23858 13.4772 4 18.4772 4 24C4 29.5228 6.23858 34.5228 9.85786 38.1421C13.4772 41.7614 18.4772 44 24 44Z"/><path fill="currentColor" stroke="none" d="M24 11C25.3807 11 26.5 12.1193 26.5 13.5C26.5 14.8807 25.3807 16 24 16C22.6193 16 21.5 14.8807 21.5 13.5C21.5 12.1193 22.6193 11 24 11Z"/><path d="M24.5 34V20H23.5H22.5"/><path d="M21 34H28"/>`},
	}
	for _, c := range cases {
		if c.got != c.want {
			t.Errorf("%s 的 path 与原版 icon-park 不一致：\n got %q\nwant %q", c.name, c.got, c.want)
		}
	}
}

// TestIconParkSecondBatchMatchesOriginal 每枚图标变量都等于 iconPark(原始 path)
// 渲染出来的位图。
func TestIconParkSecondBatchMatchesOriginal(t *testing.T) {
	cases := []struct {
		name string
		got  *ui.SVG
		want string
	}{
		{"左箭头", iconLeft, parkLeft},
		{"右箭头", iconRight, parkRight},
		{"关闭", iconClose, parkClose},
		{"关闭(小)", iconCloseSmall, parkCloseSmall},
		{"对勾", iconCheck, parkCheck},
		{"刷新", iconRefresh, parkRefresh},
		{"文字", iconText, parkText},
		{"连接", iconConnection, parkConnection},
		{"删除", iconDelete, parkDelete},
		{"顺序", iconOrder, parkOrder},
		{"单曲", iconLoopOne, parkLoopOnce},
		{"随机", iconShuffle, parkShuffleOne},
		{"定位", iconLocate, parkFocusOne},
		{"面具", iconMask, parkMaskOne},
		{"添加", iconAdd, parkAddOne},
		{"关注(Add)", iconFollow, parkAdd},
		{"已关注(Close)", iconFollowed, parkClose},
		{"图钉", iconPin, parkPushpin},
		{"合作UP", iconPeoples, parkPeoples},
		{"缩窗", iconMini, parkZoomInternal},
		{"还原", iconRestore, parkZoomInternal},
		{"Halo", iconHalo, parkHalo},
		{"成功", iconCheckOne, parkCheckOne},
		{"失败", iconCloseOne, parkCloseOne},
		{"警告", iconWarn, parkAttention},
		{"信息", iconInfo, parkInfo},
		{"实心图钉", iconPinFilled, `<path fill="currentColor" d="M32 4H16L20 7L16 20C16 20 10 24 10 28H20L24 44L28 28H38C38 24 34 21.1667 32 20L28 7L32 4Z"/>`},
	}
	for _, c := range cases {
		if !sameImage(renderIconAt(c.got, 40), renderIconAt(iconPark(c.want), 40)) {
			t.Errorf("%s 不是 iconPark 原始 path 渲染出来的", c.name)
		}
	}
}

// TestIconParkSecondBatchNotOldHandDrawn 反证这些图标确实换掉了旧的 16 画布近似。
func TestIconParkSecondBatchNotOldHandDrawn(t *testing.T) {
	cases := []struct {
		name string
		old  *ui.SVG
		now  *ui.SVG
		size int
	}{
		{"左箭头", icon(`<path d="M10 3.2L5.2 8l4.8 4.8"/>`), iconLeft, 16},
		{"右箭头", icon(`<path d="M6 3.2L10.8 8L6 12.8"/>`), iconRight, 16},
		{"关闭", icon(`<path d="M4 4l8 8M12 4l-8 8"/>`), iconClose, 14},
		{"对勾", icon(`<path d="M3.2 8.4l3.2 3.2 6.4-7"/>`), iconCheck, 14},
		{"刷新", icon(`<path d="M13.2 8a5.2 5.2 0 1 1-1.6-3.8"/><path d="M13.6 2.4v2.8h-2.8"/>`), iconRefresh, 20},
		{"文字", icon(`<path d="M3.2 3.8h9.6"/><path d="M8 3.8v8.4"/>`), iconText, 14},
		{"连接", icon(`<path d="M6.6 9.4l2.8-2.8"/><path d="M5.6 7.1l-1 1a2.7 2.7 0 0 0 3.8 3.8l1-1"/><path d="M10.4 8.9l1-1a2.7 2.7 0 0 0-3.8-3.8l-1 1"/>`), iconConnection, 18},
		{"删除", icon(`<path d="M3 4.5h10M6.2 4.5V3h3.6v1.5M4.4 4.5l.6 8.2a1 1 0 0 0 1 .9h4a1 1 0 0 0 1-.9l.6-8.2"/>`), iconDelete, 18},
		{"顺序", icon(`<path d="M2.5 4h11M2.5 8h7M2.5 12h11"/>`), iconOrder, 18},
		{"单曲", icon(`<path d="M3 8a5 5 0 0 1 8.5-3.5L13 6"/><path d="M13 2.6V6H9.6"/><path d="M13 8a5 5 0 0 1-8.5 3.5L3 10"/><path d="M3 13.4V10h3.4"/><path d="M7 6.4v3.2"/>`), iconLoopOne, 18},
		{"随机", icon(`<path d="M2.5 4.5h2.2l6.8 7h2"/><path d="M2.5 11.5h2.2l2.3-2.4"/><path d="M9 6.9l2.5-2.4h2"/><path d="M11.4 2.6L13.5 4.5l-2.1 1.9M11.4 9.6l2.1 1.9-2.1 1.9"/>`), iconShuffle, 18},
		{"定位", icon(`<circle cx="8" cy="8" r="3"/><path d="M8 1.4v2.2M8 12.4v2.2M1.4 8h2.2M12.4 8h2.2"/>`), iconLocate, 18},
		{"面具", icon(`<path d="M1.8 8h2.6a2 2 0 0 1 0 4H1.8z"/><path d="M14.2 8h-2.6a2 2 0 0 0 0 4h2.6z"/><path d="M6 10.4h4"/>`), iconMask, 15},
		{"添加", icon(`<path d="M8 3.4v9.2M3.4 8h9.2"/>`), iconAdd, 16},
		{"关注", icon(`<path d="M8 3.6v8.8M3.6 8h8.8"/>`), iconFollow, 14},
		{"已关注", icon(`<path d="M3.4 8.4l3 3 6.2-6.8"/>`), iconFollowed, 14},
		{"图钉", icon(`<path d="M6.2 1.8h3.6l-.7 3.6 2.5 2.5H4.4l2.5-2.5z"/><path d="M8 7.9v6.3"/>`), iconPin, 16},
		{"合作UP", icon(`<circle cx="6" cy="5.6" r="2.3"/><path d="M2.2 13.2c0-2.1 1.7-3.7 3.8-3.7s3.8 1.6 3.8 3.7"/><circle cx="11.6" cy="4.6" r="1.7"/><path d="M10.9 10c1.6.2 2.9 1.6 2.9 3.2"/>`), iconPeoples, 18},
		{"缩窗", icon(`<rect x="1.8" y="2.6" width="12.4" height="10.8" rx="1.6"/><rect x="8" y="8" width="5" height="4" rx="1"/>`), iconMini, 15},
		{"还原", icon(`<rect x="2.2" y="2.2" width="8" height="8" rx="1.4"/><rect x="5.8" y="5.8" width="8" height="8" rx="1.4"/>`), iconRestore, 16},
		{"Halo", icon(`<circle cx="8" cy="8" r="3.2"/><path d="M8 1.4v2M8 12.6v2M1.4 8h2M12.6 8h2M3.3 3.3l1.4 1.4M11.3 11.3l1.4 1.4M12.7 3.3l-1.4 1.4M4.7 11.3l-1.4 1.4"/>`), iconHalo, 14},
		{"成功", icon(`<path d="M3.2 8.4l3.2 3.2 6.4-7"/>`), iconCheckOne, 28},
		{"失败", icon(`<path d="M4 4l8 8M12 4l-8 8"/>`), iconCloseOne, 28},
		{"信息", icon(`<circle cx="8" cy="8" r="6"/><path d="M8 7.4v3.8"/><path d="M8 5.1v.1" stroke-width="2"/>`), iconInfo, 20},
		{"警告", icon(`<path d="M8 2.2l6.1 10.8H1.9z"/><path d="M8 6.6v3.2"/><path d="M8 11.7v.1" stroke-width="2"/>`), iconWarn, 20},
	}
	for _, c := range cases {
		diff, total := iconPixelDiff(c.old, c.now, c.size)
		if total == 0 {
			t.Fatalf("%s：渲染失败", c.name)
		}
		t.Logf("%s：旧近似 vs 现图 %d/%d 像素不同", c.name, diff, total)
		if diff < total/100 {
			t.Errorf("%s 与旧的 16 画布近似几乎一样（仅 %d/%d 不同）—— 没换成 icon-park？", c.name, diff, total)
		}
	}
}

// TestIconParkSecondBatchNotBlank 反证这些图标在实际使用尺寸下画得出墨。
func TestIconParkSecondBatchNotBlank(t *testing.T) {
	cases := []struct {
		name string
		ic   *ui.SVG
		size int
	}{
		{"左箭头", iconLeft, 14},
		{"右箭头", iconRight, 14},
		{"关闭", iconClose, 14},
		{"关闭(小)", iconCloseSmall, 18},
		{"对勾", iconCheck, 14},
		{"刷新", iconRefresh, 20},
		{"文字", iconText, 14},
		{"连接", iconConnection, 18},
		{"删除", iconDelete, 18},
		{"顺序", iconOrder, 18},
		{"单曲", iconLoopOne, 18},
		{"随机", iconShuffle, 18},
		{"定位", iconLocate, 18},
		{"面具", iconMask, 15},
		{"面具(实心)", iconMaskFilled, 15},
		{"添加", iconAdd, 16},
		{"已添加", iconCheck, 14},
		{"关注", iconFollow, 14},
		{"已关注", iconFollowed, 14},
		{"图钉", iconPin, 16},
		{"图钉(实心)", iconPinFilled, 16},
		{"合作UP", iconPeoples, 18},
		{"缩窗", iconMini, 15},
		{"还原", iconRestore, 16},
		{"Halo", iconHalo, 14},
		{"成功", iconCheckOne, 28},
		{"失败", iconCloseOne, 28},
		{"警告", iconWarn, 20},
		{"信息", iconInfo, 20},
	}
	for _, c := range cases {
		img := renderIconAt(c.ic, c.size)
		ink := 0
		for i := 0; i < len(img.Pix); i += 4 {
			if img.Pix[i] != 0xff || img.Pix[i+1] != 0xff || img.Pix[i+2] != 0xff {
				ink++
			}
		}
		if ink == 0 {
			t.Errorf("%s 在 %dpx 下渲染为空白（path 写错？）", c.name, c.size)
		}
	}
}

// TestMaskFilledHasSolidFaces 反证隐身开启时的 filled 变体确实把眼/嘴三块填成
// 实心（MaskOne 的 filled 主题只把 colors[1]（内部填充）取成 currentColor，
// 外圈仍是描边）——实心版的墨量必定多于空心版，且两枚不是同一张图。
func TestMaskFilledHasSolidFaces(t *testing.T) {
	inkCount := func(ic *ui.SVG, size int) int {
		img := renderIconAt(ic, size)
		n := 0
		for i := 0; i < len(img.Pix); i += 4 {
			if img.Pix[i] != 0xff || img.Pix[i+1] != 0xff || img.Pix[i+2] != 0xff {
				n++
			}
		}
		return n
	}
	for _, size := range []int{15, 40, 96} {
		outline, solid := inkCount(iconMask, size), inkCount(iconMaskFilled, size)
		t.Logf("面具 %dpx：空心墨 %d、实心墨 %d", size, outline, solid)
		if solid < outline {
			t.Errorf("面具 %dpx：实心墨量 %d 少于空心 %d", size, solid, outline)
		}
		if size == 96 && solid <= outline {
			t.Errorf("面具 %dpx：实心墨量 %d 不多于空心 %d（filled 没填上眼/嘴？）", size, solid, outline)
		}
	}
}

// TestPlayFilledIsTwoColor 反证合集「播放全部」/播放列表当前行的 Play 是
// filled 主题的两层色：圆盘跟 TextColor 走、三角挖白。单色 Icon 画不出。
func TestPlayFilledIsTwoColor(t *testing.T) {
	img := ui.Render(func(c *ui.Context) {
		c.Root().Background(ui.Hex("#ffffff"))
		ui.Column(c).Padding(4).Children(func() {
			ui.Image(c, iconPlayFilled).Size(40, 40).TextColor(ui.Hex("#3b82f6"))
		})
	}, 48, 48, 1)
	blues, whites := 0, 0
	for y := 0; y < 48; y++ {
		for x := 0; x < 48; x++ {
			i := img.PixOffset(x, y)
			r, g, b, a := img.Pix[i], img.Pix[i+1], img.Pix[i+2], img.Pix[i+3]
			if a == 0 || (r == 0xff && g == 0xff && b == 0xff && a == 0xff) {
				continue
			}
			switch {
			case b > 200 && r < 120:
				blues++
			case r > 200 && g > 200 && b > 200:
				whites++
			}
		}
	}
	t.Logf("Play filled：蓝盘 %d、白三角 %d", blues, whites)
	if blues < 200 {
		t.Errorf("圆盘没有跟 TextColor 上色（蓝像素 %d）", blues)
	}
	if whites == 0 {
		t.Error("三角没有挖白（纯白像素 0）—— 还是单色图？")
	}
	// 三角内部（48 画布上的 (26,24)）必须是实心白——只描边不留白的话这里仍是蓝的。
	cx, cy := 25, 24
	if c := img.RGBAAt(cx, cy); c.R < 200 || c.G < 200 || c.B < 200 {
		t.Errorf("三角内部 (%d,%d) = %v，不是实心白（只描边？）", cx, cy, c)
	}
	// 圆盘环带（48 画布上的 (9,24)）必须是蓝（盘身跟 TextColor 走）。
	if c := img.RGBAAt(9, 24); c.B < 150 || c.R > 150 {
		t.Errorf("圆盘环带 (9,24) = %v，不是蓝色", c)
	}
}

// TestIconParkSecondBatchWiring 接线反证：新图标真的接在了对应控件上。
func TestIconParkSecondBatchWiring(t *testing.T) {
	cases := []struct {
		file string
		want string
		why  string
	}{
		{"shell.go", "ui.Icon(c, iconMini).Size(15, 15)", "标题栏「切换到迷你模式」（用户 2026-10-18 要求恢复）"},
		{"mini.go", `"mini-restore", "切换到窗口模式", iconRestore`, "迷你窗「切换到窗口模式」"},
		{"mini.go", "miniPinIcon(a.Pinned)", "迷你窗「窗口置顶」的 filled 变体"},
		{"home.go", "iconHalo", "封面背景开关（Halo）"},
		{"drawer.go", "pick(a.Incognito, iconMaskFilled, iconMask)", "隐身开关的 filled 变体"},
		{"drawer.go", "icon := iconViews", "卡片 meta 播放量图标"},
		{"playlist.go", "iconPlayFilled", "播放列表当前行的 filled Play"},
		{"drawer.go", "iconPlayFilled", "合集「播放全部」的 filled Play"},
		{"drawer.go", "pick(inList, iconCheck, iconAdd)", "选集卡片已添加/添加"},
		{"drawer.go", "iconAdd).Size(16, 16)", "「全部添加」用 AddOne"},
		{"toast.go", "iconCheckOne", "toast 成功图标"},
		{"toast.go", "iconCloseOne", "toast 失败图标"},
		{"modal.go", "Icon: iconCheckOne", "对话框成功图标"},
		{"modal.go", "Icon: iconCloseOne", "对话框失败图标"},
		{"modal.go", "ui.Icon(c, iconCloseSmall)", "登录面板关闭用 CloseSmall"},
	}
	for _, c := range cases {
		src, err := os.ReadFile(c.file)
		if err != nil {
			t.Fatalf("读 %s 失败：%v", c.file, err)
		}
		if !bytes.Contains(src, []byte(c.want)) {
			t.Errorf("%s 没接上 %q（%s）", c.file, c.want, c.why)
		}
	}
	// 旧的 16 画布手抄不能再出现在这些位置。
	for _, bad := range []string{
		`iconPlay = icon(`,
		`iconSolid(`,
	} {
		src, err := os.ReadFile("state.go")
		if err != nil {
			t.Fatalf("读 state.go 失败：%v", err)
		}
		if bytes.Contains(src, []byte(bad)) {
			t.Errorf("state.go 里仍有 %q", bad)
		}
	}
}
