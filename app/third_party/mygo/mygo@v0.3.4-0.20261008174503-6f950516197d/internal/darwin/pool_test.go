//go:build darwin

package darwin

import (
	"runtime"
	"sync"
	"testing"
)

// TestPoolOffMainThread pushes and pops autorelease pools from many
// goroutines, as public methods that call native code directly do
// (App.Name reads the bundle): a pool belongs to the thread that pushed
// it, and popping it on another one, where a goroutine yielding in between
// may resume, crashes.
func TestPoolOffMainThread(t *testing.T) {
	load()
	defer runtime.GOMAXPROCS(runtime.GOMAXPROCS(max(4, runtime.NumCPU())))
	var wg sync.WaitGroup
	for range 32 {
		wg.Go(func() {
			for range 100 {
				withPool(func() {
					nsString("pool")
					runtime.Gosched()
				})
				appController{}.Package()
			}
		})
	}
	wg.Wait()
}
