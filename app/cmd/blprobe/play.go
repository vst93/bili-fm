package main

// play 命令：把整条音频链路跑一遍（HTTP → AAC → WSOLA → EQ → 声卡）。
//
//	go run ./cmd/blprobe play BV1xx411c7mD 8 2.0

import (
	"fmt"
	"time"

	"github.com/vst93/bili-fm/app/internal/bilibili"
	"github.com/vst93/bili-fm/app/internal/media"
)

const ua = "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/131.0.0.0 Safari/537.36"

func playVideo(bl *bilibili.BL, bvid string, seconds float64, speed float64, eq bool) {
	info := bl.GetCList(bvid)
	if len(info.Pages) == 0 {
		fmt.Println("取分集失败")
		return
	}
	part := info.Pages[0]
	u := bl.GetUrlByCid(int(info.Aid), part.Cid)
	if u.URL == "" {
		fmt.Println("取播放地址失败")
		return
	}
	fmt.Printf("分集: %s (cid=%d)\n地址: %s…\n", part.Part, part.Cid, u.URL[:min(len(u.URL), 70)])

	p := media.NewPlayer()
	defer p.Close()
	p.SetSpeed(speed)
	p.SetEQ(eq)

	ended := make(chan struct{})
	p.OnEnded(func() { close(ended) })
	p.OnError(func(err error) { fmt.Println("播放错误:", err) })

	headers := map[string]string{
		"User-Agent": ua,
		"Referer":    "https://www.bilibili.com/",
	}
	t0 := time.Now()
	if err := p.Play(u.URL, headers); err != nil {
		fmt.Println("起播失败:", err)
		return
	}
	fmt.Printf("起播耗时 %v，总时长 %.1fs，速度 %.2fx，均衡 %v\n", time.Since(t0), p.Duration(), speed, eq)

	deadline := time.Now().Add(time.Duration(seconds * float64(time.Second)))
	last := -1.0
	for time.Now().Before(deadline) {
		time.Sleep(time.Second)
		pos := p.Position()
		fmt.Printf("  t=%5.1fs  位置 %7.2fs  推进 %+.2fs/s  播放中=%v\n",
			time.Since(t0).Seconds(), pos, pos-last, p.Playing())
		last = pos
	}
	p.Stop()
	fmt.Println("停止。")
}
