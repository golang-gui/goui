package test

import (
	"os"
	"runtime"
	"sync"
	"testing"

	"github.com/golang-gui/goui/platform"
)

var mainThreadTasks = make(chan func())

var (
	testPlatform     platform.Platform
	testPlatformErr  error
	testPlatformOnce sync.Once
)

func init() {
	runtime.LockOSThread()
}

func TestMain(m *testing.M) {
	exit := make(chan int, 1)
	go func() {
		exit <- m.Run()
	}()

	for {
		select {
		case task := <-mainThreadTasks:
			task()
		case code := <-exit:
			if testPlatform != nil {
				testPlatform.Destroy()
			}
			os.Exit(code)
		}
	}
}

func runOnMainThread(task func()) {
	done := make(chan struct{})
	mainThreadTasks <- func() {
		defer close(done)
		task()
	}
	<-done
}

func getPlatform() (platform.Platform, error) {
	runOnMainThread(func() {
		testPlatformOnce.Do(func() {
			testPlatform, testPlatformErr = platform.NewPlatform(platform.DefaultName())
		})
	})
	return testPlatform, testPlatformErr
}

func skipWithoutDisplay(t *testing.T) {
	t.Helper()
	if runtime.GOOS == "linux" && os.Getenv("DISPLAY") == "" {
		t.Skip("DISPLAY is not set")
	}
}
