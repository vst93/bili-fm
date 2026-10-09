package imagecache

import "testing"

func TestThumbURL(t *testing.T) {
	cases := []struct {
		in   string
		want string
	}{
		{"", ""},
		{
			"https://i0.hdslb.com/bfs/archive/abc.jpg",
			"https://i0.hdslb.com/bfs/archive/abc.jpg@240w.webp",
		},
		{
			// 已经是处理过的地址，不再追加后缀。
			"https://i0.hdslb.com/bfs/archive/abc.jpg@480w.webp",
			"https://i0.hdslb.com/bfs/archive/abc.jpg@480w.webp",
		},
		{
			// 协议相对地址补 https。
			"//i1.hdslb.com/bfs/archive/x.png",
			"https://i1.hdslb.com/bfs/archive/x.png@240w.webp",
		},
		{
			// http 图床统一走 https。
			"http://i2.hdslb.com/bfs/archive/y.jpg",
			"https://i2.hdslb.com/bfs/archive/y.jpg@240w.webp",
		},
		{
			// 非 hdslb 图床原样返回。
			"https://archive.biliimg.com/bfs/archive/z.jpg",
			"https://archive.biliimg.com/bfs/archive/z.jpg",
		},
	}
	for _, c := range cases {
		if got := ThumbURL(c.in); got != c.want {
			t.Errorf("ThumbURL(%q) = %q, want %q", c.in, got, c.want)
		}
	}
}
