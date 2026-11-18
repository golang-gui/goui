// Xlib 原生查询和属性缓冲区契约验证。
// 环境：Linux/X11；不需要 WM/Painter，缩放不适用。启动：go run ./tests/window/platform/xlib_queries
// 初始：创建一个不映射的 1x1 窗口，不会显示界面；仍需要显示连接。
// 操作与预期：自动核对属性尺寸/类型/format、32位值在 native long 中的存储、
// 分段/偏移读取、类型不匹配、format-8 原始字节、delete=true 与缺失属性结果。
// 副作用：只创建私有窗口和属性；不改全局设置。复位：重启。
// 退出：全部断言满足才 PASS/0，失败非零；Ctrl-C 不算通过。
package main

import (
	"fmt"
	"reflect"
	"runtime"

	"github.com/goexlib/cgo"
	"github.com/golang-gui/goui/platform/linux/libs/xlib"
)

func checkWindowQueries() {
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()
	display := xlib.OpenDisplay("")
	if display == 0 {
		panic("cannot open test display")
	}
	defer display.Close()
	screen := display.DefaultScreenOfDisplay()
	window := display.CreateWindow(screen.Root, 0, 0, 1, 1, 0, int(screen.RootDepth),
		xlib.WindowClassInputOutput, screen.RootVisual, 0, nil)
	if window == 0 {
		panic("cannot create test window")
	}
	defer display.DestroyWindow(window)
	var attrs xlib.WindowAttributes
	if status := display.GetWindowAttributes(window, &attrs); status == 0 ||
		attrs.Width != 1 || attrs.Height != 1 || attrs.Root != screen.Root ||
		attrs.MapState != xlib.IsUnmapped || attrs.Depth != screen.RootDepth {
		panic(fmt.Errorf("XGetWindowAttributes: status=%d attrs=%+v", status, attrs))
	}

	property := display.InternAtom("GOUI_TEST_RAW_PROPERTY", false)
	values := []uintptr{1, 0x12345678, 0xffffffff}
	display.ChangeProperty(window, property, xlib.AtomCardinal, 32, xlib.PropModeReplace, cgo.CSlice(values), len(values))
	for _, tc := range []struct {
		name           string
		offset, length int
		reqType        xlib.Atom
		want           []uintptr
		bytesAfter     uint
	}{
		{"full", 0, 8, xlib.AtomCardinal, values, 0},
		{"partial", 0, 2, xlib.AtomCardinal, values[:2], 4},
		{"offset", 1, 2, xlib.AtomCardinal, values[1:], 0},
		{"type mismatch", 0, 8, xlib.AtomAtom, nil, 0},
	} {
		func() {
			fmt.Println("CHECK", tc.name)
			var actualType xlib.Atom
			var format int32
			var nitems, bytesAfter uint
			var data *byte
			status := display.GetWindowProperty(window, property, tc.offset, tc.length, false,
				tc.reqType, &actualType, &format, &nitems, &bytesAfter, &data)
			if data != nil {
				defer xlib.Free(data)
			}
			remaining := bytesAfter == tc.bytesAfter
			if tc.reqType != xlib.AtomCardinal {
				remaining = bytesAfter > 0
			}
			if status != 0 || actualType != xlib.AtomCardinal || format != 32 ||
				nitems != uint(len(tc.want)) || !remaining {
				panic(fmt.Errorf("native results: status=%d type=%d format=%d items=%d after=%d",
					status, actualType, format, nitems, bytesAfter))
			}

			for i, value := range cgo.GoSliceNTemp[uintptr](cgo.Pointer(data), int(nitems)) {
				if uint32(value) != uint32(tc.want[i]) {
					panic(fmt.Errorf("item %d: %#x want low 32 bits %#x", i, value, tc.want[i]))
				}
			}
		}()
	}

	bytes := []byte{1, 2, 3}
	display.ChangeProperty(window, property, xlib.AtomString, 8, xlib.PropModeReplace, cgo.CSlice(bytes), len(bytes))
	var actualType xlib.Atom
	var format int32
	var nitems, bytesAfter uint
	var data *byte
	status := display.GetWindowProperty(window, property, 0, 8, true, xlib.AtomAny,
		&actualType, &format, &nitems, &bytesAfter, &data)
	if data != nil {
		defer xlib.Free(data)
	}
	if status != 0 || actualType != xlib.AtomString || format != 8 || nitems != 3 || bytesAfter != 0 ||
		!reflect.DeepEqual(cgo.GoSliceNTemp[byte](cgo.Pointer(data), int(nitems)), bytes) {
		panic("format-8 property results changed")
	}

	status = display.GetWindowProperty(window, property, 0, 8, false, xlib.AtomAny,
		&actualType, &format, &nitems, &bytesAfter, &data)
	if data != nil {
		defer xlib.Free(data)
	}
	if status != 0 || actualType != xlib.AtomNone || format != 0 || nitems != 0 || bytesAfter != 0 {
		panic("missing property results changed")
	}
}

func main() { checkWindowQueries(); fmt.Println("PASS Xlib queries") }
