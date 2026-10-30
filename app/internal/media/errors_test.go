package media

import (
	"bytes"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	aacpcm "github.com/tphakala/go-aac/pcm"
)

// TestKindOfRootCauseWins 钉住类别判定的两条语义：
//  1. 没带类别的错误算 KindOther（上层会落到兜底文案）；
//  2. 链上已经有类别时，WrapKind 不再套新的 —— 根因优先。
//
// 第 2 条是这套机制的关键：一次 HTTP 403 会让 m4a.NewReader 失败，而
// 外层会把它包成「解析 MP4 失败 ⇒ 格式不支持」。如果外层硬贴 KindUnsupported，
// 用户看到的提示就和真因相反了。
func TestKindOfRootCauseWins(t *testing.T) {
	if got := KindOf(nil); got != KindOther {
		t.Errorf("KindOf(nil) = %v，want KindOther", got)
	}
	if got := KindOf(errors.New("media: 什么都没有")); got != KindOther {
		t.Errorf("没归类的错误 = %v，want KindOther", got)
	}

	netErr := WrapKind(KindNetwork, errors.New("media: 上游返回 403"))
	if got := KindOf(netErr); got != KindNetwork {
		t.Fatalf("WrapKind(KindNetwork) 后 KindOf = %v", got)
	}
	// 解码器建不起来时又套一层 KindDecode：不该盖住内层的网络类别。
	outer := WrapKind(KindDecode, fmt.Errorf("media: 打开 AAC 解码器失败: %w", netErr))
	if got := KindOf(outer); got != KindNetwork {
		t.Errorf("外层再套类别后 KindOf = %v，want KindNetwork（根因优先）", got)
	}
	// 文案/根因都还在。
	if got := outer.Error(); got == "" || !errors.Is(outer, netErr) {
		t.Errorf("包装后应保留文案与 errors.Is 通道，得到 %q", got)
	}
	if got := KindOf(WrapKind(KindDecode, errors.New("x"))); got != KindDecode {
		t.Errorf("首层类别 = %v，want KindDecode", got)
	}
}

// TestRemoteFileErrorKind 钉住「取流出错都算网络类」：这是最常见的一类
// 播放失败（B 站链接过期 / CDN 给 403），必须落到原版 code 2 的文案。
func TestRemoteFileErrorKind(t *testing.T) {
	t.Run("HTTP 非 200/206", func(t *testing.T) {
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			w.WriteHeader(http.StatusForbidden)
		}))
		defer srv.Close()
		_, err := newRemoteFile(srv.URL, nil)
		if err == nil {
			t.Fatal("403 应当报错")
		}
		if got := KindOf(err); got != KindNetwork {
			t.Errorf("HTTP 403 的类别 = %v，want KindNetwork（err=%v）", got, err)
		}
	})

	t.Run("206 缺 Content-Range 总长度", func(t *testing.T) {
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			w.WriteHeader(http.StatusPartialContent)
			_, _ = w.Write([]byte("a"))
		}))
		defer srv.Close()
		_, err := newRemoteFile(srv.URL, nil)
		if err == nil {
			t.Fatal("206 缺总长度应当报错")
		}
		if got := KindOf(err); got != KindNetwork {
			t.Errorf("206 缺总长度 = %v，want KindNetwork（err=%v）", got, err)
		}
	})

	t.Run("取块失败", func(t *testing.T) {
		// 先给一个能探出长度的 206（Content-Range 带总长），之后取块给 500。
		first := true
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if first {
				first = false
				w.Header().Set("Content-Range", "bytes 0-0/4096")
				w.WriteHeader(http.StatusPartialContent)
				_, _ = w.Write([]byte("a"))
				return
			}
			w.WriteHeader(http.StatusInternalServerError)
		}))
		defer srv.Close()
		f, err := newRemoteFile(srv.URL, nil)
		if err != nil {
			t.Fatalf("探测阶段不该失败：%v", err)
		}
		buf := make([]byte, 16)
		// 探测请求只读 Content-Range，不填缓冲，所以第一次 Read 就会去取块
		// -> 500。
		_, err = f.Read(buf)
		if err == nil {
			t.Fatal("取块 500 应当报错")
		}
		if got := KindOf(err); got != KindNetwork {
			t.Errorf("取块失败的类别 = %v，want KindNetwork（err=%v）", got, err)
		}
	})
}

// TestPipelineErrorKeepsNetworkKind 反证「pump 兜底套 KindDecode 不会吃掉
// 网络根因」：流水线中途因为取流失败而停时，上层必须仍看到 KindNetwork。
func TestPipelineErrorKeepsNetworkKind(t *testing.T) {
	p := newTestPipeline(48000)
	netErr := WrapKind(KindNetwork, errors.New("media: 取块失败，上游返回 403"))
	p.mu.Lock()
	p.err, p.eof = WrapKind(KindDecode, netErr), true
	p.mu.Unlock()
	_, err := p.state()
	if err == nil {
		t.Fatal("state() 应报出流水线错误")
	}
	if got := KindOf(err); got != KindNetwork {
		t.Errorf("流水线错误的类别 = %v，want KindNetwork", got)
	}
}

// TestErrSupersededIsExportedAndPlain 钉住「被取代」这个哨兵量：
//   - 它是导出量（main.go 要 errors.Is 判定），
//   - 它**不带**类别（它不是给用户看的失败，调用方必须整条忽略）。
func TestErrSupersededIsExportedAndPlain(t *testing.T) {
	if ErrSuperseded == nil {
		t.Fatal("ErrSuperseded 不能是 nil")
	}
	if got := KindOf(ErrSuperseded); got != KindOther {
		t.Errorf("ErrSuperseded 不该带类别，得到 %v", got)
	}
	if !errors.Is(fmt.Errorf("起播: %w", ErrSuperseded), ErrSuperseded) {
		t.Error("ErrSuperseded 应能被 errors.Is 认出来")
	}
}

// TestHEAACErrorIsTagged 钉住「换流重试」的判据：HE-AAC（SBR/PS）的解码
// 错误必须带上 ErrHEAAC（上层据此换 DASH 的 AAC-LC 音轨）；普通解码错误
// 不能误带，否则一次帧损坏也会白换一次流。
func TestHEAACErrorIsTagged(t *testing.T) {
	// 显式 HE-AAC（AOT 5）的 ASC：解码器在建解码器阶段就拒绝。
	_, sbr := aacpcm.NewDecoder(bytes.NewReader(nil), aacpcm.WithRawStream([]byte{0x2b, 0x92, 0x08, 0x00}))
	if sbr == nil || !errors.Is(sbr, aacpcm.ErrUnsupportedSBR) {
		t.Fatalf("测试前提变了：HE-AAC 的 ASC 应被解码器拒绝，得到 %v", sbr)
	}

	tagged := aacProfileErr(fmt.Errorf("media: 打开 AAC 解码器失败: %w", sbr))
	if !errors.Is(tagged, ErrHEAAC) {
		t.Errorf("HE-AAC 错误应带 ErrHEAAC，得到 %v", tagged)
	}
	// 原始错误链要保留（日志里还能看到 go-aac 的具体原因）。
	if !errors.Is(tagged, aacpcm.ErrUnsupportedSBR) {
		t.Errorf("包装后丢了原始错误链：%v", tagged)
	}
	// 类别仍然是解码失败：用户文案不变。
	if got := KindOf(WrapKind(KindDecode, tagged)); got != KindDecode {
		t.Errorf("HE-AAC 的类别 = %v，want KindDecode", got)
	}

	plain := aacProfileErr(errors.New("media: 帧损坏"))
	if errors.Is(plain, ErrHEAAC) {
		t.Errorf("普通解码错误不该带 ErrHEAAC：%v", plain)
	}
	if aacProfileErr(nil) != nil {
		t.Error("aacProfileErr(nil) 应为 nil")
	}
}
