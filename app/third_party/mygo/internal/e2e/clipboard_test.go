package e2e

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"image"
	"image/color"
	"image/png"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"slices"
	"sync/atomic"
	"testing"
	"time"

	"github.com/egoist/mygo"
	"github.com/egoist/mygo/transfer"
	"github.com/egoist/mygo/ui"
)

const clipboardCustom transfer.Format = "application/vnd.mygo.clipboard-e2e"

type clipboardPeerResult struct {
	Items      int
	Values     map[transfer.Format][]byte
	Text, HTML string
	Image      []byte
	Files      []string
}

func clipboardTestData(provider func() ([]byte, error)) transfer.Data {
	img := image.NewNRGBA(image.Rect(0, 0, 2, 2))
	img.SetNRGBA(0, 0, color.NRGBA{R: 40, G: 80, B: 120, A: 255})
	var encoded bytes.Buffer
	_ = png.Encode(&encoded, img)
	return transfer.New(
		transfer.NewItem(transfer.Bytes(transfer.Text, []byte("hello 日本語")), transfer.Bytes(transfer.HTML, []byte("<b>hello 日本語</b>")), transfer.Bytes(transfer.PNG, encoded.Bytes()), transfer.Lazy(clipboardCustom, provider)),
		transfer.NewItem(transfer.Bytes(transfer.Text, []byte("second")), transfer.Bytes(clipboardCustom, []byte{3, 0})),
	)
}

// clipboardPeer runs a real, independent native clipboard client. It does
// not start any window or webview. The parent app's loop stays running while
// this process requests delayed representations or owns a foreign selection.
func clipboardPeer(mode string) {
	mygo.App.SetName("MyGoClipboardPeer")
	mygo.App.SetActivationPolicy(mygo.ActivationPolicyAccessory)
	mygo.App.WhenReady(func() {
		go func() {
			if mode == "persist-text" {
				mygo.Clipboard.WriteText("persisted clipboard 日本語")
				mygo.App.Quit()
				return
			}
			if mode == "read" {
				var formats []transfer.Format
				_ = json.Unmarshal([]byte(os.Getenv("MYGO_CLIPBOARD_FORMATS")), &formats)
				d, err := mygo.Clipboard.Read(formats...)
				if err != nil {
					fmt.Fprintln(os.Stderr, "clipboard peer read:", err)
					os.Exit(1)
				}
				r := clipboardPeerResult{Items: len(d.Items()), Values: map[transfer.Format][]byte{}, Text: mygo.Clipboard.ReadText(), HTML: mygo.Clipboard.ReadHTML(), Image: mygo.Clipboard.ReadImage()}
				r.Files, _ = d.Files()
				for _, f := range d.Formats() {
					r.Values[f], _ = d.Read(f)
				}
				b, _ := json.Marshal(r)
				fmt.Println(string(b))
				mygo.App.Quit()
				return
			}
			d := clipboardTestData(func() ([]byte, error) { return []byte{0, 1, 2, 0}, nil })
			if err := mygo.Clipboard.Write(d); err != nil {
				fmt.Fprintln(os.Stderr, err)
				os.Exit(1)
			}
			if mode == "persist" {
				mygo.App.Quit()
				return
			}
			fmt.Println("ready")
			_, _ = bufio.NewReader(os.Stdin).ReadString('\n')
			mygo.App.Quit()
		}()
	})
	if err := mygo.App.Run(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func readClipboardPeer(t *testing.T, formats ...transfer.Format) clipboardPeerResult {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	b, _ := json.Marshal(formats)
	cmd := exec.CommandContext(ctx, os.Args[0], "-test.run=^$")
	cmd.Env = append(os.Environ(), "MYGO_E2E_CLIPBOARD_PEER=read", "MYGO_CLIPBOARD_FORMATS="+string(b))
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	out, err := cmd.Output()
	if err != nil {
		t.Fatalf("clipboard peer: %v; %s", err, stderr.String())
	}
	var r clipboardPeerResult
	if err := json.Unmarshal(bytes.TrimSpace(out), &r); err != nil {
		t.Fatalf("peer reply %q: %v", out, err)
	}
	return r
}

func preserveClipboard(t *testing.T) {
	t.Helper()
	d, err := mygo.Clipboard.Read()
	t.Cleanup(func() {
		if err == nil {
			_ = mygo.Clipboard.Write(d)
			_ = mygo.Clipboard.Flush()
		} else {
			mygo.Clipboard.Clear()
		}
	})
}

func TestUnifiedClipboardNativeRoundTrip(t *testing.T) {
	preserveClipboard(t)
	var calls, releases atomic.Int32
	d := clipboardTestData(func() ([]byte, error) { calls.Add(1); return []byte{0, 1, 2, 0}, nil })
	w := newWindow(t, mygo.WindowOptions{Hidden: true, Content: ui.View(func(c *ui.Context) { ui.Box(c) })})
	if err := mygo.Clipboard.Write(d, mygo.ClipboardOptions{OnRelease: func() { releases.Add(1) }}); err != nil {
		t.Fatal(err)
	}
	if f := mygo.Clipboard.Formats(); !slices.Equal(f, d.Formats()) || calls.Load() != 0 {
		t.Fatalf("lazy discovery %v, calls %d", f, calls.Load())
	}
	w.Destroy()
	if calls.Load() != 0 || releases.Load() != 0 {
		t.Fatal("window destruction requested or released clipboard providers")
	}
	local, err := mygo.Clipboard.Read(transfer.Text)
	if err != nil || len(local.Items()) != 2 || calls.Load() != 0 {
		t.Fatalf("local items %d, calls %d: %v", len(local.Items()), calls.Load(), err)
	}
	r := readClipboardPeer(t, d.Formats()...)
	for _, f := range d.Formats() {
		want, _ := d.Read(f)
		if !bytes.Equal(r.Values[f], want) {
			t.Errorf("foreign %s: %v, want %v", f, r.Values[f], want)
		}
	}
	// One invocation for the native snapshot; d.Read above invokes its
	// independent original provider cache once as well.
	if calls.Load() != 2 {
		t.Fatalf("provider calls %d", calls.Load())
	}
	wantText := "hello 日本語\nsecond"
	if r.Text != wantText || r.HTML != "<b>hello 日本語</b>" {
		t.Fatalf("native conveniences: %q, %q", r.Text, r.HTML)
	}
	if img, err := png.Decode(bytes.NewReader(r.Image)); err != nil || img.Bounds().Dx() != 2 {
		t.Fatalf("native image: %v", err)
	}
	if runtime.GOOS == "darwin" && r.Items != 2 {
		t.Fatal("AppKit lost item boundaries")
	}
	if err := mygo.Clipboard.Flush(); err != nil && !errors.Is(err, mygo.ErrClipboardPersistence) {
		t.Fatal(err)
	}
	eventually(t, "clipboard provider release", func() bool { return releases.Load() == 1 })
	r = readClipboardPeer(t, clipboardCustom)
	if !bytes.Equal(r.Values[clipboardCustom], []byte{0, 1, 2, 0}) {
		t.Fatalf("flushed custom bytes: %v", r.Values[clipboardCustom])
	}
	mygo.Clipboard.Clear()
	mygo.RunOnMain(func() {})
	if releases.Load() != 1 {
		t.Fatal("native source released twice")
	}
	if b, err := local.Read(transfer.Text); err != nil || string(b) != "hello 日本語\nsecond" {
		t.Fatal("read snapshot depended on a closed window/clipboard owner")
	}
}

func TestUnifiedClipboardNativeFilesAndFailures(t *testing.T) {
	preserveClipboard(t)
	paths := []string{filepath.Join(t.TempDir(), "file #%.txt"), filepath.Join(t.TempDir(), "日本語.txt")}
	for _, path := range paths {
		if err := os.WriteFile(path, []byte("file"), 0600); err != nil {
			t.Fatal(err)
		}
	}
	if err := mygo.Clipboard.WriteFiles(paths...); err != nil {
		t.Fatal(err)
	}
	r := readClipboardPeer(t, transfer.FileList, transfer.URIList)
	if !slices.Equal(r.Files, paths) {
		t.Fatalf("foreign file list %q, want %q", r.Files, paths)
	}
	boom := errors.New("encode failed")
	var calls atomic.Int32
	d := transfer.New(transfer.NewItem(transfer.Bytes(transfer.Text, []byte("fallback")), transfer.Lazy(clipboardCustom, func() ([]byte, error) { calls.Add(1); return nil, boom })))
	if err := mygo.Clipboard.Write(d); err != nil {
		t.Fatal(err)
	}
	for range 2 {
		if _, err := mygo.Clipboard.ReadFormat(clipboardCustom); !errors.Is(err, boom) {
			t.Fatalf("provider read error: %v", err)
		}
		if err := mygo.Clipboard.Flush(); !errors.Is(err, boom) {
			t.Fatalf("provider flush error: %v", err)
		}
	}
	// External readers receive a failed native representation, while the
	// source retains its cached error and usable plain-text alternative.
	for range 2 {
		ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
		cmd := exec.CommandContext(ctx, os.Args[0], "-test.run=^$")
		formats, _ := json.Marshal([]transfer.Format{clipboardCustom})
		cmd.Env = append(os.Environ(), "MYGO_E2E_CLIPBOARD_PEER=read", "MYGO_CLIPBOARD_FORMATS="+string(formats))
		out, err := cmd.CombinedOutput()
		cancel()
		if err == nil || !bytes.Contains(out, []byte("clipboard peer read:")) {
			t.Fatalf("failed native provider was acknowledged: %v, %s", err, out)
		}
	}
	if calls.Load() != 1 || mygo.Clipboard.ReadText() != "fallback" {
		t.Fatal("native failure lost ownership or retried a provider")
	}
}

func TestUnifiedClipboardForeignOwnershipAndShutdown(t *testing.T) {
	preserveClipboard(t)
	var releases atomic.Int32
	if err := mygo.Clipboard.Write(transfer.TextData("old"), mygo.ClipboardOptions{OnRelease: func() { releases.Add(1) }}); err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, os.Args[0], "-test.run=^$")
	cmd.Env = append(os.Environ(), "MYGO_E2E_CLIPBOARD_PEER=own")
	stdin, err := cmd.StdinPipe()
	if err != nil {
		t.Fatal(err)
	}
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		t.Fatal(err)
	}
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	if err := cmd.Start(); err != nil {
		t.Fatal(err)
	}
	stopped := false
	t.Cleanup(func() {
		if !stopped {
			_ = stdin.Close()
			_ = cmd.Wait()
		}
	})
	ready, err := bufio.NewReader(stdout).ReadString('\n')
	if err != nil || ready != "ready\n" {
		t.Fatalf("foreign owner startup %q: %v, %s", ready, err, stderr.String())
	}
	eventually(t, "foreign clipboard ownership", func() bool { return slices.Contains(mygo.Clipboard.Formats(), clipboardCustom) })
	eventually(t, "old provider release", func() bool { return releases.Load() == 1 })
	d, err := mygo.Clipboard.Read(clipboardCustom)
	if err != nil {
		t.Fatalf("foreign read: %v; peer stderr: %s", err, stderr.String())
	}
	if b, _ := d.Read(clipboardCustom); !bytes.Equal(b, []byte{0, 1, 2, 0}) {
		t.Fatalf("foreign custom bytes %v", b)
	}
	// Flushing from a non-owner must leave the other process's data intact.
	if err := mygo.Clipboard.Flush(); err != nil {
		t.Fatal(err)
	}
	if _, err := mygo.Clipboard.ReadFormat(clipboardCustom); err != nil {
		t.Fatal("non-owner flush changed the clipboard")
	}
	_, _ = stdin.Write([]byte("\n"))
	_ = stdin.Close()
	err = cmd.Wait()
	stopped = true
	if err != nil {
		t.Fatalf("foreign owner shutdown: %v; %s", err, stderr.String())
	}
	if runtime.GOOS != "linux" {
		b, err := mygo.Clipboard.ReadFormat(clipboardCustom)
		if err != nil || !bytes.Equal(b, []byte{0, 1, 2, 0}) {
			t.Fatalf("data after source app exits: %v, %v", b, err)
		}
	}
}

func TestUnifiedClipboardNativePersistence(t *testing.T) {
	preserveClipboard(t)
	mygo.Clipboard.WriteText("persistence probe")
	if err := mygo.Clipboard.Flush(); errors.Is(err, mygo.ErrClipboardPersistence) {
		t.Skip("desktop has no clipboard manager for persistence after exit")
	} else if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, os.Args[0], "-test.run=^$")
	cmd.Env = append(os.Environ(), "MYGO_E2E_CLIPBOARD_PEER=persist-text")
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("clipboard writer shutdown: %v; %s", err, out)
	}
	if got := mygo.Clipboard.ReadText(); got != "persisted clipboard 日本語" {
		t.Fatalf("text after application exit: %q", got)
	}
}
