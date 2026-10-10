package terminal

import (
	"fmt"
	"strings"
	"testing"

	"github.com/egoist/mygo/ui"
)

// BenchmarkOutput measures how fast the terminal takes a program's output:
// lines of text with some colors.
func BenchmarkOutput(b *testing.B) {
	if err := Load(); err != nil {
		b.Skip(err)
	}
	term, err := New(Options{Conn: newPipe()})
	if err != nil {
		b.Fatal(err)
	}
	defer term.Close()
	var sb strings.Builder
	for i := range 1000 {
		fmt.Fprintf(&sb, "\x1b[32m%6d\x1b[0m  the quick brown fox jumps over the lazy dog \x1b[1;34m%x\x1b[0m\r\n", i, i*7919)
	}
	data := []byte(sb.String())
	b.SetBytes(int64(len(data)))
	for b.Loop() {
		term.write(data)
	}
}

// BenchmarkFrame measures a frame of a 1600×1000 window whose every row
// changed, as while a program scrolls.
func BenchmarkFrame(b *testing.B) {
	if err := Load(); err != nil {
		b.Skip(err)
	}
	term, err := New(Options{Conn: newPipe()})
	if err != nil {
		b.Fatal(err)
	}
	defer term.Close()
	tt := ui.NewTester(func(c *ui.Context) { View(c, term).Fill() }, 1600, 1000)
	tt.SetScale(2)
	i := 0
	for b.Loop() {
		i++
		term.write([]byte(fmt.Sprintf("\x1b[32m%6d\x1b[0m  the quick brown fox jumps over the lazy dog \x1b[1;34m%x\x1b[0m %s\r\n", i, i*7919, strings.Repeat("=", i%100))))
		for range 60 {
			term.write([]byte("line " + strings.Repeat("abc ", i%40) + "\r\n"))
		}
		tt.Frame()
	}
}
