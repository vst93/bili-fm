package video

import (
	"os"
	"runtime"
	"testing"

	"github.com/ebitengine/purego"
)

func init() { runtime.LockOSThread() }

func TestMain(tests *testing.M) {
	framework, err := purego.Dlopen("/System/Library/Frameworks/CoreFoundation.framework/CoreFoundation", purego.RTLD_NOW)
	if err != nil {
		panic(err)
	}
	var runLoop func()
	purego.RegisterLibFunc(&runLoop, framework, "CFRunLoopRun")
	go func() { os.Exit(tests.Run()) }()
	for {
		runLoop()
	}
}
