package mygo

import (
	"testing"
	"time"
)

// TestLoopInsideQueuedFunction checks that a function on the main thread
// running a loop of its own, as a context menu does, runs the functions
// queued behind it, which it may be waiting for.
func TestLoopInsideQueuedFunction(t *testing.T) {
	done := make(chan struct{})
	waited := make(chan bool, 1)
	// Both at once, as when a test asks for the menu just as it opens.
	loop.mu.Lock()
	first := len(loop.queue) == 0
	loop.queue = append(loop.queue, func() {
		for deadline := time.Now().Add(2 * time.Second); time.Now().Before(deadline); {
			select {
			case <-done:
				waited <- true
				return
			default:
			}
			backend().Step()
		}
		waited <- false
	}, func() { close(done) })
	loop.mu.Unlock()
	if first {
		backend().Signal()
	}
	if !<-waited {
		t.Fatal("the function queued behind one running a loop did not run")
	}
}
