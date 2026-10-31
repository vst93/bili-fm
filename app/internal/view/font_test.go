package view

import (
	"strings"
	"testing"

	"github.com/vst93/bili-fm/app/internal/view/fonts"
)

// TestUnifiedFontStackWired 钉住「三平台统一字体」这件事本身：
//
//   - 拉丁走内置 Inter（注册失败时 fontStack 会退成系统栈，这里必须拦住）；
//   - 主题把字体写进 ui.Theme.Font，所有元素继承；
//   - 中文回退顺序以 macOS 的 PingFang SC 打头（基准），其后是 Windows /
//     Linux 的等价界面字体。
func TestUnifiedFontStackWired(t *testing.T) {
	if !fonts.Registered() {
		t.Fatal("内置 Inter 没注册成功：中文以外的字体会回到各平台系统字体")
	}
	if !strings.HasPrefix(fontStack, "Inter,") {
		t.Errorf("字体栈没有以 Inter 打头：%q", fontStack)
	}
	for _, family := range []string{"PingFang SC", "Microsoft YaHei UI", "Noto Sans CJK SC"} {
		if !strings.Contains(fontStack, family) {
			t.Errorf("字体栈缺少中文回退 %q：%q", family, fontStack)
		}
	}
	if got := ThemeFor(PeriodAt(12)).uiTheme().Font; got != fontStack {
		t.Errorf("ui.Theme.Font = %q，want 统一字体栈 %q", got, fontStack)
	}
}

// TestCJKLineBoxTallerThanLatin 钉住中文行盒与拉丁行盒的区别：Inter 没有汉字，
// 中文由回退字体重排，行盒取回退字体的 ascent+descent，比拉丁行盒高。固定高度
// 容器（桌面字幕窗、多行标题落点）按拉丁度量算就会裁字，所以必须能分出两者。
func TestCJKLineBoxTallerThanLatin(t *testing.T) {
	for _, size := range []float32{12, 14, 20, 30} {
		latin, cjk := naturalLineBox(size), cjkLineBox(size)
		if cjk <= latin {
			t.Errorf("%vpx：中文行盒 %v 不大于拉丁行盒 %v，说明中文字体没接上（渲染会退成豆腐块或同款 Inter）", size, cjk, latin)
		}
	}
	if cjkLineBox(0) != 0 {
		t.Errorf("cjkLineBox(0) = %v，want 0", cjkLineBox(0))
	}
}

// TestFontOfCarriesStack 钉住自绘文本（弹幕/多行标题/量宽）用的是统一字体栈：
// 裸 ui.Font{Size: n} 不带 Family，会悄悄退回系统字体，量出来的宽度和画出来
// 的字形就和别处不是同一个。
func TestFontOfCarriesStack(t *testing.T) {
	f := fontOf(13, 650)
	if f.Family != fontStack {
		t.Errorf("fontOf 没带字体栈：%q", f.Family)
	}
	if f.Size != 13 || f.Weight != 650 {
		t.Errorf("fontOf 丢掉了字号/字重：%v %v", f.Size, f.Weight)
	}
}
