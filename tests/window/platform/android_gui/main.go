//go:build android

// Android GUI 首个内容窗口验收，不依赖 DevServer。环境与维护者打包方式见 README.md。
// 初始状态：NativeActivity、Software、系统默认比例；普通 NewApplication/NewWindow，
// Label、Button、TextInput 和纯色块在 Run 前创建，未暴露任何桌面窗口模拟能力。
// 自动步骤：普通 Timer 更新文本；首帧后经 Window.DispatchEvent 点击按钮一次。
// 操作：旋转横/竖屏，Home 后重新打开；预期同 PID/GOUI 线程、同控件树、一次点击，
// 尺寸适应新表面，后台不推进 Frame，恢复显示最新计数。触摸、IME、GLES 尚未接入。
// 副作用：仅自身沙箱 go.json 和 startup.json；本例定时更新便于检查后台帧暂停。
// 复位：清除测试应用数据或 force-stop 后重新启动。退出：adb shell am force-stop
// org.golanggui.androidgui；Back 的原生结束 Activity 不等同于本例主动 Quit。
package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"

	ffi "github.com/goexlib/cgo"
	"github.com/golang-gui/goui/core/geometry"
	"github.com/golang-gui/goui/gui"
	"github.com/golang-gui/goui/layout"
	"github.com/golang-gui/goui/platform"
	"github.com/golang-gui/goui/platform/events"
	"github.com/golang-gui/goui/platform/graphics"
	"github.com/golang-gui/goui/theme/modern"
)

func must(err error) {
	if err != nil {
		panic(err)
	}
}

func symbol(name string) uintptr {
	address, err := ffi.Dlsym(ffi.RTLD_DEFAULT, name)
	must(err)
	return address
}

type colorBlock struct{ gui.WidgetBase }

func (b *colorBlock) Paint(p gui.Painter) {
	p.FillRect(geometry.Rect(0, 0, b.Rect().Width, b.Rect().Height), graphics.RGB(32, 144, 96))
}

func main() {
	gettid := symbol("gettid")
	owner := ffi.CallRet[int32](gettid)
	nativeThread := ffi.CallRet[int32](symbol("probe_native_thread"))
	directory := ffi.GoString(ffi.Pointer(ffi.CallRet[uintptr](symbol("probe_directory"))))
	if owner == nativeThread || directory == "" {
		panic("GUI 与系统线程没有分离")
	}
	write := func(name string, value any) {
		if ffi.CallRet[int32](gettid) != owner {
			panic("GUI 回调离开所属线程")
		}
		data, err := json.MarshalIndent(value, "", "  ")
		must(err)
		temporary := filepath.Join(directory, name+".tmp")
		must(os.WriteFile(temporary, data, 0600))
		must(os.Rename(temporary, filepath.Join(directory, name)))
	}
	app, err := gui.NewApplication("org.golanggui.androidgui")
	must(err)
	if platform.DefaultName() != "android" || app.Platform().Name() != "android" {
		panic("默认工厂未选择 Android")
	}
	app.SetStyleSheet(modern.FromSettings(app.Settings()))
	win, err := app.NewWindow(&gui.WindowOptions{Size: geometry.Size{Width: 320, Height: 480}, Chrome: gui.WindowChromeIntegrated})
	must(err)
	native := win.PlatformWindow()
	if _, desktop := native.(platform.WindowManagement); desktop || native.SurfaceState().Ready {
		panic("移动窗口伪造桌面能力或在 Run 前等待表面")
	}
	write("startup.json", map[string]any{"owner": owner, "nativeThread": nativeThread, "readyBeforeRun": false, "desktop": false})
	column := gui.NewLinearBox(layout.DirectionVertical)
	column.SetPadding(geometry.UniformInsets(20))
	column.SetSpacing(12)
	column.SetCrossAlign(layout.CrossStretch)
	column.SetID("content")
	heading := gui.NewLabel("GOUI Android — GUI window")
	column.AddChild(heading)
	text := gui.NewLabel("中文 日本語 한국어 العربية हिन्दी 👩‍💻")
	text.SetWrapMode(gui.WrapWordChar)
	column.AddChild(text)
	status := gui.NewLabel("Timer 0")
	column.AddChild(status)
	button := gui.NewButton()
	button.SetID("button")
	button.SetChild(gui.NewLabel("Normal Window.DispatchEvent"))
	column.AddChild(button)
	input := gui.NewTextInput()
	input.SetText("programmatic text — IME later")
	column.AddChild(input)
	block := new(colorBlock)
	block.SetID("color")
	block.SetMinSize(geometry.Size{Width: 80, Height: 32})
	column.AddChild(block)
	win.SetWidget(column)
	frames, ticks, clicks := 0, 0, 0
	var scale float32
	var rendered gui.WindowInfo
	button.ConnectClicked(func() { clicks++ })
	win.ConnectFrame(func(time.Time) { frames++ })
	win.ConnectAfterPaint(func(p gui.Painter, _ geometry.Rectangle) {
		for _, label := range []*gui.Label{heading, text, status} {
			if label.Rect().Width <= 0 || label.Rect().Height <= 0 {
				panic("独立 Label 未取得有效布局尺寸")
			}
		}
		scale = p.PixelScale()
		rendered = win.Snapshot()
	})
	timer := app.NewTimer()
	timer.ConnectTimeout(func() {
		ticks++
		status.SetText(fmt.Sprintf("Timer %d — clicks %d", ticks, clicks))
		if clicks == 0 && frames > 0 {
			point := button.Snapshot().Bounds.Center()
			must(win.DispatchEvent(events.PointerEvent{EventType: events.PointerDown, Position: point, Button: events.PointerButtonLeft, Buttons: events.PointerButtonLeftDown}))
			must(win.DispatchEvent(events.PointerEvent{EventType: events.PointerUp, Position: point, Button: events.PointerButtonLeft}))
			if clicks != 1 {
				panic("普通指针事件未激活按钮")
			}
		}
		write("go.json", map[string]any{
			"owner": owner, "nativeThread": nativeThread, "frames": frames, "ticks": ticks, "clicks": clicks,
			"root": fmt.Sprintf("%p", column), "input": input.Text(), "rendered": rendered,
			"surface": native.SurfaceState(), "scale": scale,
		})
	})
	must(timer.Start(200 * time.Millisecond))
	must(win.Show())
	app.Run()
}
