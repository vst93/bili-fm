package store

import (
	"fmt"
	"sync"
	"testing"
)

func TestRoundTrip(t *testing.T) {
	kv, err := Open(t.TempDir(), false)
	if err != nil {
		t.Fatal(err)
	}
	if err := kv.SetString("greeting", "hello"); err != nil {
		t.Fatal(err)
	}
	if got := kv.String("greeting"); got != "hello" {
		t.Errorf("String = %q", got)
	}
	if kv.String("missing") != "" {
		t.Error("missing key should be empty")
	}
	if err := kv.Set("num", 42); err != nil {
		t.Fatal(err)
	}
	if got := kv.Get("num"); got != float64(42) {
		t.Errorf("Get(num) = %#v", got)
	}
	kv.Del("greeting")
	if kv.String("greeting") != "" {
		t.Error("deleted key should be empty")
	}
}

func TestReadonly(t *testing.T) {
	dir := t.TempDir()
	kv, err := Open(dir, false)
	if err != nil {
		t.Fatal(err)
	}
	_ = kv.SetString("k", "v")

	ro, err := Open(dir, true)
	if err != nil {
		t.Fatal(err)
	}
	if ro.String("k") != "v" {
		t.Error("readonly should read")
	}
	if err := ro.SetString("k", "x"); err == nil {
		t.Error("readonly should refuse writes")
	}
}

// TestConcurrentAccess 断言多 goroutine 并发读写不会 panic、也不会读到半个文件。
func TestConcurrentAccess(t *testing.T) {
	kv, err := Open(t.TempDir(), false)
	if err != nil {
		t.Fatal(err)
	}
	var wg sync.WaitGroup
	for i := 0; i < 8; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			for j := 0; j < 60; j++ {
				_ = kv.SetString("k", fmt.Sprintf("%d-%d", i, j))
				if v := kv.String("k"); v == "" {
					t.Errorf("读到了空值（半个文件？）")
					return
				}
			}
		}(i)
	}
	wg.Wait()
}
