package ios

import (
	"runtime"
	"sync/atomic"

	"github.com/goexlib/cgo"
	"github.com/golang-gui/goui/platform/darwin/frameworks"
	. "github.com/golang-gui/goui/platform/darwin/frameworks/foundation"
	ui "github.com/golang-gui/goui/platform/darwin/frameworks/uikit"
	"github.com/golang-gui/goui/platform/darwin/libs/dispatch"
	"github.com/golang-gui/goui/platform/internal/mobile"
	"github.com/golang-gui/goui/platform/internal/mobileentry"
)

// EventLoop dispatches Go work through UIKit's native main run loop.
type EventLoop struct{ *mobile.EventLoop }

// nativeApplication lives for the process. UIKit owns the run loop; the Go
// startup goroutine only prepares the application and waits for completion.
type nativeApplication struct {
	running     atomic.Bool
	loop        atomic.Pointer[mobile.EventLoop]
	callback    uintptr
	onHostReady func() // Main-thread notification, detached by Platform.Destroy.
}

var iosApplication nativeApplication

const applicationDelegateName = "GoUIDelegate"

func init() { mobileentry.NativeRun = runNative }

// onSystemThread anchors a native callback to its entering OS thread. It does
// not move work to the main thread; UIKit/dispatch must already call it there.
func onSystemThread(fn func()) {
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()
	fn()
}

func runNative() int {
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()
	if !dispatch.IsMainThread() {
		panic("ios: native entry must run on the system main thread")
	}
	if err := frameworks.InitIOS(); err != nil {
		panic(err)
	}
	a := &iosApplication
	a.callback = cgo.NewCallback(func(uintptr) {
		onSystemThread(func() {
			AutoReleasePool(func() {
				if loop := a.loop.Load(); loop != nil {
					loop.Dispatch()
				}
			})
		})
	})
	_, err := ui.ImplementUIApplicationDelegate(applicationDelegateName, ui.UIApplicationDelegateOverride{
		ApplicationDidFinishLaunchingWithOptions: func(ui.UIApplicationDelegate, ui.UIApplication, NSDictionary) bool {
			return mobileentry.Start()
		},
	})
	if err != nil {
		panic(err)
	}
	if err := initWindow(); err != nil {
		panic(err)
	}
	a.running.Store(true)
	var result int
	AutoReleasePool(func() {
		name := NSStringClassId.Alloc().InitWithBytes([]byte(applicationDelegateName), NSUTF8StringEncoding)
		defer name.Release()
		result = int(ui.UIApplicationMain(0, 0, NSString{}, name))
	})
	return result
}
