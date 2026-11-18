// X11 原生移动/缩放与一次性按下会话验证。
// 环境：独立 X11 测试桌面 + 支持 _NET_WM_MOVERESIZE 的 WM、XTEST；不需要 Painter。
// 启动：go run ./tests/window/platform/window_interaction
// 初始：无按住的鼠标键；程序会移动真实指针，请勿同时操作。
// 操作与预期：自动等待窗口可见，注入真实 ButtonPress，移动/释放后核对原生位置或尺寸；
// 同一次按下的重复 BeginMove、投递后的 BeginMove 均须 ErrUnavailable，每项五秒超时。
// 副作用：移动窗口和指针；退出前恢复指针并释放鼠标键。复位：重启专用测试桌面。
// 退出：自动 PASS/0，断言失败非零；手动关闭或 Ctrl-C 不算通过。
package main

import (
	"errors"
	"fmt"
	"runtime"
	"time"

	"github.com/ebitengine/purego"
	"github.com/golang-gui/goui/core/geometry"
	"github.com/golang-gui/goui/platform"
	"github.com/golang-gui/goui/platform/events"
	"github.com/golang-gui/goui/platform/linux/libs/xlib"
)

func checkX11NativeWindowInteraction(plat platform.Platform) {
	xtest, err := purego.Dlopen("libXtst.so.6", purego.RTLD_NOW|purego.RTLD_LOCAL)
	if err != nil {
		panic(fmt.Errorf("XTEST unavailable: %v", err))
	}
	defer purego.Dlclose(xtest)
	var motion func(xlib.Display, int32, int32, int32, uintptr) int32
	var button func(xlib.Display, uint32, int32, uintptr) int32
	purego.RegisterLibFunc(&motion, xtest, "XTestFakeMotionEvent")
	purego.RegisterLibFunc(&button, xtest, "XTestFakeButtonEvent")
	x11, err := purego.Dlopen("libX11.so.6", purego.RTLD_NOW|purego.RTLD_LOCAL)
	if err != nil {
		panic(err)
	}
	defer purego.Dlclose(x11)
	var queryPointer func(xlib.Display, xlib.Window, *xlib.Window, *xlib.Window, *int32, *int32, *int32, *int32, *uint32) int32
	purego.RegisterLibFunc(&queryPointer, x11, "XQueryPointer")
	var display xlib.Display
	var root xlib.Window
	var pointer struct {
		root, child  xlib.Window
		x, y, wx, wy int32
		mask         uint32
	}
	var pin runtime.Pinner
	pin.Pin(&pointer)
	defer pin.Unpin()
	var pointerOK int32
	func() {
		display = xlib.OpenDisplay("")
		if display != 0 {
			root = display.DefaultRootWindow()
			pointerOK = queryPointer(display, root, &pointer.root, &pointer.child,
				&pointer.x, &pointer.y, &pointer.wx, &pointer.wy, &pointer.mask)
		}
	}()
	if display == 0 {
		panic("open test display")
	}
	defer display.Close()
	if pointerOK == 0 || pointer.mask&(xlib.Button1Mask|xlib.Button2Mask|xlib.Button3Mask) != 0 {
		panic("pointer unavailable or a mouse button is already held")
	}
	defer func() {
		button(display, xlib.Button1, 0, 0)
		motion(display, -1, pointer.x, pointer.y, 0)
		display.Flush()
	}()
	for _, resize := range []bool{false, true} {
		func() {
			fmt.Println("CHECK", fmt.Sprintf("resize=%v", resize))
			var loop platform.EventLoop
			var win platform.Window
			var desktop platform.DesktopWindow
			var setupErr, requestErr, repeatedErr, postedErr error
			presses, posts := 0, 0
			var before, after xlib.WindowAttributes
			var x0, y0, x1, y1 int
			var pressX, pressY int32
			var pressPosition geometry.Point

			func() {
				loop, setupErr = plat.NewEventLoop()
				if setupErr != nil {
					return
				}
				win, setupErr = plat.NewWindow(geometry.Size{Width: 300, Height: 200}, func(event platform.Event) {
					p, ok := event.(events.PointerEvent)
					if !ok || p.EventType != events.PointerDown || p.Button != events.PointerButtonLeft {
						return
					}
					presses++
					pressPosition = p.Position
					if resize {
						requestErr = desktop.BeginResize(platform.WindowEdgeBottomRight)
					} else {
						requestErr = desktop.BeginMove()
					}
					repeatedErr = desktop.BeginMove()
					loop.Post(func() { posts++; postedErr = desktop.BeginMove() })
				}, platform.WindowOptions{Chrome: platform.WindowChromeNone})
				if setupErr == nil {
					desktop = win.(platform.DesktopWindow)
					setupErr = win.(platform.WindowManagement).SetTitle("GOUI native window interaction test")
					if setupErr == nil {
						setupErr = win.(platform.WindowManagement).Show()
					}
				}
			}()
			defer func() {
				if win != nil {
					win.Destroy()
				}
				if loop != nil {
					loop.Destroy()
				}
			}()
			if setupErr != nil {
				panic(setupErr)
			}
			stage, motionStep := 0, int32(0)
			// 只把计时器用作轮询：按实际映射、按下回调和原生几何推进，不猜测 WM 处理耗时。
			poll := func() {
				id := xlib.Window(win.NativeHandle())
				switch stage {
				case 0:
					if display.GetWindowAttributes(id, &before) == 0 || before.MapState != xlib.IsViewable {
						return
					}
					x0, y0 = display.TranslateCoordinates(id, root, 0, 0)
					pressX, pressY = int32(x0)+before.Width/2, int32(y0)+30
					if resize {
						pressX, pressY = int32(x0)+before.Width-5, int32(y0)+before.Height-5
					}
					motion(display, -1, pressX, pressY, 0)
					button(display, xlib.Button1, 1, 0)
					display.Flush()
					stage = 1
				case 1:
					if presses == 0 || posts == 0 {
						return
					}
					if requestErr != nil {
						loop.Quit()
						return
					}
					stage = 2
				case 2:
					if display.GetWindowAttributes(id, &after) == 0 {
						return
					}
					x1, y1 = display.TranslateCoordinates(id, root, 0, 0)
					changed := x0 != x1 && y0 != y1
					if resize {
						changed = after.Width > before.Width && after.Height > before.Height
					}
					if changed {
						button(display, xlib.Button1, 0, 0)
						display.Flush()
						stage = 3
						return
					}
					// WM 接管是异步的；继续发送有界真实移动，直到观察到几何变化。
					motionStep = 1 - motionStep
					motion(display, -1, pressX+40+motionStep, pressY+30+motionStep, 0)
					display.Flush()
				case 3:
					if display.GetWindowAttributes(id, &after) == 0 {
						return
					}
					x1, y1 = display.TranslateCoordinates(id, root, 0, 0)
					stage = 4
					loop.Quit()
				}
			}
			ticker := time.NewTicker(10 * time.Millisecond)
			done, stopped := make(chan struct{}), make(chan struct{})
			go func() {
				defer close(stopped)
				for {
					select {
					case <-ticker.C:
						loop.Post(poll)
					case <-done:
						return
					}
				}
			}()
			loop.Post(poll)
			timeout := time.AfterFunc(5*time.Second, loop.Quit)
			loop.Run()
			timeout.Stop()
			close(done)
			ticker.Stop()
			<-stopped
			if stage != 4 {
				panic(fmt.Errorf("native interaction stalled at stage %d: presses=%d posts=%d request=%v", stage, presses, posts, requestErr))
			}
			if setupErr != nil {
				panic(setupErr)
			}
			if presses == 1 && errors.Is(requestErr, platform.ErrUnsupported) {
				panic("WM does not support interactive requests")
			}
			if presses != 1 || posts != 1 || requestErr != nil || !errors.Is(repeatedErr, platform.ErrUnavailable) || !errors.Is(postedErr, platform.ErrUnavailable) {
				panic(fmt.Errorf("presses=%d posts=%d request=%v repeat=%v posted=%v", presses, posts, requestErr, repeatedErr, postedErr))
			}
			if resize {
				if after.Width <= before.Width || after.Height <= before.Height || x0 != x1 || y0 != y1 {
					panic(fmt.Errorf("resize: (%d,%d %dx%d) -> (%d,%d %dx%d)", x0, y0, before.Width, before.Height, x1, y1, after.Width, after.Height))
				}
			} else if x0 == x1 || y0 == y1 || before.Width != after.Width || before.Height != after.Height {
				panic(fmt.Errorf("move: (%d,%d %dx%d) -> (%d,%d %dx%d)", x0, y0, before.Width, before.Height, x1, y1, after.Width, after.Height))
			}
			fmt.Printf("native interaction: (%d,%d %dx%d) -> (%d,%d %dx%d); press root=(%d,%d), DIP=%v; repeat and posted requests rejected\n", x0, y0, before.Width, before.Height, x1, y1, after.Width, after.Height, pressX, pressY, pressPosition)
		}()
	}
}

func main() {
	plat, err := platform.NewPlatform(platform.DefaultName(), "org.golang-gui.NativeRegression")
	if err != nil {
		panic(err)
	}
	defer plat.Destroy()
	checkX11NativeWindowInteraction(plat)
	fmt.Println("PASS window interaction")
}
