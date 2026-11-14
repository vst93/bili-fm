package main

// 视频窗弹幕的接线：设置落盘 / 视频窗同步 / 按 cid 取数据。
//
// 数据不在这里重新拉：internal/danmaku.Store 与弹幕抽屉、热力条共用同一次
// 请求与同一份缓存（见 startDanmakuFetch / startDanmakuHeat）。这里只做
// 「拿到的条目交给视频窗」与「设置变化时通知已开着的窗口」两件事。

import (
	"log"
	"strconv"

	"github.com/vst93/bili-fm/app/internal/danmaku"
)

// loadDanmakuPrefs 读回弹幕设置。所有值都经 Normalize 夹取：手改 KV / 旧
// 版本写过的值最多退回默认，不会让排版拿到 0 字号或负上限。
func (c *controller) loadDanmakuPrefs() {
	s := danmaku.DefaultSettings()
	if v := c.kv.String(prefDanmakuOn); v != "" {
		s.On = v != "false"
	}
	if v := parseFloat(c.kv.String(prefDanmakuOpacity)); v >= 0.3 && v <= 1 {
		s.Opacity = v
	}
	if v, err := strconv.Atoi(c.kv.String(prefDanmakuFontSize)); err == nil && v >= 0 && v <= 2 {
		s.Size = v
	}
	if v := c.kv.String(prefDanmakuArea); v == "half" {
		s.Area = 0.5
	}
	if v, err := strconv.Atoi(c.kv.String(prefDanmakuMaxOnScreen)); err == nil && v >= 10 && v <= 200 {
		s.Max = v
	}
	c.applyDanmakuStyle(s)
}

// applyDanmakuStyle 把设置写进 App 并同步给视频管理器（已开着的窗口下一帧
// 就按新设置画）。不落盘 —— 启动读回时也走它。
func (c *controller) applyDanmakuStyle(s danmaku.Settings) {
	if c.app == nil {
		return
	}
	c.app.DanmakuStyle = s.Normalize()
	if c.vid != nil {
		c.vid.SetDanmakuOn(c.app.DanmakuStyle.On)
		c.vid.SetDanmakuSettings(c.app.DanmakuStyle)
	}
}

// setDanmakuStyle 是设置菜单与视频窗开关的统一入口：立即生效 + 落盘。
func (c *controller) setDanmakuStyle(s danmaku.Settings) {
	c.applyDanmakuStyle(s)
	s = c.app.DanmakuStyle
	if c.kv != nil {
		_ = c.kv.SetString(prefDanmakuOn, strconv.FormatBool(s.On))
		_ = c.kv.SetString(prefDanmakuOpacity, strconv.FormatFloat(s.Opacity, 'f', -1, 64))
		_ = c.kv.SetString(prefDanmakuFontSize, strconv.Itoa(s.Size))
		area := "full"
		if s.Area <= 0.75 {
			area = "half"
		}
		_ = c.kv.SetString(prefDanmakuArea, area)
		_ = c.kv.SetString(prefDanmakuMaxOnScreen, strconv.Itoa(s.Max))
	}
	if c.menuDanmaku != nil {
		c.menuDanmaku.SetChecked(s.On)
	}
	if c.app.Win != nil {
		c.app.Win.Update(func() {})
	}
}

// startVideoDanmaku 把当前这一集的弹幕交给视频窗。数据走 Store 的缓存：
// 抽屉 / 热力已经拉过就立即命中，没有才发一次请求（同 cid 并发还会去重）。
func (c *controller) startVideoDanmaku(cid int64) {
	if cid == 0 || c.dm == nil || c.vid == nil {
		return
	}
	reqID := c.videoDmReqID.Add(1)
	go func() {
		items, err := c.dm.Get(cid)
		if err != nil {
			log.Printf("视频弹幕: cid=%d 拉取失败: %v", cid, err)
			return
		}
		runOnUI(c.app, !c.app.Hidden(), func() {
			if c.videoDmReqID.Load() != reqID {
				return // 已经换到别的集，旧结果丢弃
			}
			c.vid.SetDanmaku(cid, items)
		})
	}()
}
