package media

import "testing"

// TestPipelineSpeedChangeLatency 钉住「改倍速只影响贴近声卡的那一小段」：
//
// 旧实现里 raw 队列存的是**已经按旧倍速变速过**的 5 秒数据，用户调倍速后
// 要等这 5 秒播完才听得见（反馈「调完卡了好几秒才生效」）。现在 raw 存解码
// 后的源采样，WSOLA 在 finish 段按当前倍速推进，ready 只保留 200ms ——
// 所以清空 ready 后重新补满时，源游标必须按**新倍速**前进。
//
// 反证：若 WSOLA 又回到 pump / 大缓冲之前，清空 ready 不会让源游标前进，
// 本测试的「源消费 ≥ 2×输出」断言会红。
func TestPipelineSpeedChangeLatency(t *testing.T) {
	p := eqTestPipeline(t)
	const frames = 2 * 48000 // 2 秒源采样
	feedSine(p, frames, 0.5)
	waitReady(t, p, p.eqTargetBytes())

	ws := p.wsNow()
	// 先把已就绪的 200ms 清掉（模拟声卡把它播完）。
	p.mu.Lock()
	ready := len(p.ready) - p.readyAt
	p.mu.Unlock()
	p.take(make([]byte, ready))

	before := ws.SourcePosition()
	ws.SetSpeed(3)
	waitReady(t, p, p.eqTargetBytes())
	after := ws.SourcePosition()

	p.mu.Lock()
	ready = len(p.ready) - p.readyAt
	p.mu.Unlock()
	outFrames := ready / (p.channels * 2)
	srcFrames := after - before
	if outFrames <= 0 {
		t.Fatal("清空 ready 后没有补出新数据")
	}
	// 3x 下应消费约 3 倍源采样；留出 WSOLA 预热/边界的余量，只要求 >2 倍。
	if srcFrames < 2*outFrames {
		t.Fatalf("改倍速后补 %d 帧输出只消费了 %d 帧源采样（应约 3 倍）——"+
			"WSOLA 不在贴近声卡的输出段，改倍速仍要等大缓冲播完",
			outFrames, srcFrames)
	}
}

// TestPipelinePositionStaysNearAudio 钉住「进度只领先声卡约一个 ready 段」：
//
// WSOLA 现在在 finish 段推进，只预产 eqTargetBytes（约 200ms）的输出；
// 之前它在 pump 里提前变速 5 秒，SourcePosition 会领先实际听到的声音约
// 5 秒（进度条/SponsorBlock/断点上报都跟着偏）。这里喂 5 秒源采样后只
// 应消费 ~200ms。
func TestPipelinePositionStaysNearAudio(t *testing.T) {
	p := eqTestPipeline(t)
	feedSine(p, 5*48000, 0.5)
	waitReady(t, p, p.eqTargetBytes())
	pos := p.position()
	if pos > 1.0 {
		t.Fatalf("ready 只有 200ms 时播放位置 = %.2fs（应 ≈0.2s）；"+
			"WSOLA 又跑到大缓冲前面了，进度会领先声音数秒", pos)
	}
	if pos <= 0 {
		t.Fatalf("播放位置没有前进：%v", pos)
	}
}
