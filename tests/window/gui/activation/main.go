//go:build android || ios

// 移动 GUI Surface 验收，具体环境、构建、操作、预期、复位与退出见 README.md。
// 控件通过普通 GUI 事件路径接受测试输入；本例不宣称已接入原生触摸或 IME。
package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"sync/atomic"
	"time"

	"github.com/golang-gui/goui/core/geometry"
	"github.com/golang-gui/goui/gui"
	"github.com/golang-gui/goui/gui/diagnostics"
	"github.com/golang-gui/goui/layout"
	"github.com/golang-gui/goui/platform"
	"github.com/golang-gui/goui/platform/events"
)

var painterName = "software"
var viewMode = "gui"

type application struct {
	app                                                                                  gui.Application
	host                                                                                 gui.Surface
	native                                                                               platform.Window
	input                                                                                *gui.TextInput
	button                                                                               *gui.Button
	label                                                                                *gui.Label
	timer                                                                                *gui.Timer
	dir                                                                                  string
	nonce                                                                                int64
	startup, ui                                                                          uint64
	activations, shutdowns, deliveries, frames, posts, count, mounts, unmounts, destroys int
	backend                                                                              string
	capture                                                                              *diagnostics.Capture
	scale                                                                                float32
	exercised, gc, stopped                                                               bool
	exited                                                                               atomic.Bool
}

func must(err error) {
	if err != nil {
		panic(err)
	}
}
func main() {
	if viewMode == "ui" {
		mainUI()
		return
	}
	if mainThread() {
		panic("启动 goroutine 不应阻塞原生 UI 线程")
	}
	must(setPainter())
	a := &application{nonce: time.Now().UnixNano(), startup: threadID(), dir: reportDirectory()}
	must(prepareReports(a.dir))
	var err error
	a.app, err = gui.NewApplication("org.golanggui.MobileActivation")
	must(err)
	if a.app.Typography() != nil {
		panic("移动准备阶段提前初始化原生服务")
	}
	a.app.ConnectActivate(a.start)
	a.app.ConnectShutdown(a.stop)
	must(a.app.Run())
	if !a.exited.Load() {
		panic("Run 早于 UI 清理回调返回")
	}
	write(filepath.Join(a.dir, "completed.json"), map[string]any{"nonce": a.nonce, "callbackReturned": true, "frames": a.frames})
}
func (a *application) checkThread() {
	if !mainThread() || threadID() == a.startup || a.ui != 0 && threadID() != a.ui {
		panic("GUI 操作没有归属原生 UI 主线程")
	}
	a.ui = threadID()
}
func (a *application) start() {
	a.checkThread()
	a.activations++
	var err error
	a.host, err = a.app.NewSurface()
	must(err)
	a.deliveries++
	if _, managed := a.host.(gui.Window); managed {
		panic("系统 Surface 冒充管理窗口")
	}
	a.host.SetID("main")
	a.capture, err = diagnostics.Start(a.app, diagnostics.Options{})
	must(err)
	root := gui.NewLinearBox(layout.DirectionVertical)
	// 测试显式留白，当前不把它当作 SafeArea 的实现。
	root.SetPadding(geometry.Insets{Left: 20, Top: 64, Right: 20, Bottom: 20})
	root.SetSpacing(12)
	root.AddChild(gui.NewLabel("GOUI Surface / main thread"))
	root.AddChild(gui.NewLabel("中文 日本語 한국어 العربية"))
	a.input = gui.NewTextInput()
	a.input.SetID("editor")
	a.input.SetText("initial")
	a.input.SetMinSize(geometry.Size{Width: 240, Height: 36})
	a.input.ConnectMount(func() { a.mounts++ })
	a.input.ConnectUnmount(func() { a.unmounts++ })
	root.AddChild(a.input)
	a.button = gui.NewButton()
	a.button.SetID("action")
	a.button.SetChild(gui.NewLabel("Increment"))
	a.button.SetMinSize(geometry.Size{Width: 180, Height: 40})
	a.label = gui.NewLabel("Count: 0")
	a.button.ConnectClicked(func() { a.checkThread(); a.count++; a.label.SetText(fmt.Sprintf("Count: %d", a.count)) })
	root.AddChild(a.button)
	root.AddChild(a.label)
	a.host.ConnectDestroy(func() { a.checkThread(); a.destroys++ })
	a.host.SetWidget(root)
	if a.input.Root() != a.host || a.input.Surface() != a.host || a.input.Window() != nil {
		panic("Widget 宿主身份错误")
	}
	a.host.ConnectAfterPaint(func(p gui.Painter, _ geometry.Rectangle) { a.checkThread(); a.frames++; a.scale = p.PixelScale() })
	a.timer = a.app.NewTimer()
	a.timer.ConnectTimeout(a.tick)
	must(a.timer.Start(100 * time.Millisecond))
}
func (a *application) tick() {
	a.checkThread()
	if _, err := os.Stat(filepath.Join(a.dir, "stop")); err == nil {
		a.app.Quit()
		return
	}
	a.posts++
	if a.frames > 1 {
		for _, root := range a.capture.Snapshot().Roots {
			if root.Kind == "surface" && root.ID == "main" {
				a.backend = root.Backend
			}
		}
		wanted := "Software"
		if painterName == "opengl" {
			wanted = "OpenGL"
		}
		if a.backend != wanted {
			panic("实际绘制后端/Surface 诊断元数据错误：" + a.backend)
		}
	}
	if native := a.host.PlatformWindow(); native != nil {
		if a.native == nil {
			a.native = native
		} else if native != a.native {
			panic("绘制目标替换却更换逻辑原生宿主")
		}
	}
	if a.frames >= 2 && a.host.Focused() && !a.exercised {
		if !a.host.SetFocusedWidget(a.input) {
			panic("Surface 无法接收逻辑焦点")
		}
		a.input.SetSelection(gui.TextSelection{Anchor: len(a.input.Text()), Caret: len(a.input.Text())})
		must(a.host.CommitText(" committed"))
		point := a.button.Snapshot().Bounds.Center()
		must(a.host.DispatchEvent(events.PointerEvent{EventType: events.PointerDown, Button: events.PointerButtonLeft, Buttons: events.PointerButtonLeftDown, Position: point}))
		must(a.host.DispatchEvent(events.PointerEvent{EventType: events.PointerUp, Button: events.PointerButtonLeft, Position: point}))
		if a.count != 1 || a.input.Text() != "initial committed" {
			panic("普通输入分发未更新按钮/文本")
		}
		a.exercised = true
	}
	if a.exercised && (a.input.Text() != "initial committed" || a.count != 1 || gui.FindWidget(a.host, "editor") != a.input || a.mounts != 1 || a.unmounts != 0) {
		panic("目标或可见性变化破坏了 Widget 状态")
	}
	if a.frames > 0 && !a.gc {
		runtime.GC()
		a.checkThread()
		a.gc = true
	}
	must(a.host.RequestPaint())
	a.save()
}
func (a *application) stop() {
	a.checkThread()
	a.stopped = true
	a.shutdowns++
	defer a.exited.Store(true)
	if a.timer != nil {
		a.timer.Stop()
	}
	if a.host != nil {
		a.host.Destroy()
	}
	if a.host != nil && (a.unmounts != 1 || a.destroys != 1 || !a.input.Destroyed()) {
		panic("Surface 没有最终释放 Widget 树")
	}
	a.save()
	if a.capture != nil {
		a.capture.Stop()
	}
}
func (a *application) save() {
	state := platform.SurfaceState{}
	bounds := geometry.Rectangle{}
	focused := false
	if a.host != nil {
		bounds = a.host.Snapshot().Bounds
		focused = a.host.Focused()
		if native := a.host.PlatformWindow(); native != nil {
			state = native.SurfaceState()
		}
	}
	write(filepath.Join(a.dir, "dispatch.json"), map[string]any{
		"nonce": a.nonce, "startupThread": a.startup, "uiThread": a.ui, "mainThread": mainThread(),
		"frames": a.frames, "posts": a.posts, "visible": a.host != nil && a.host.Visible(), "stopped": a.stopped, "focused": focused,
		"surface": state, "size": map[string]any{"Width": bounds.Width, "Height": bounds.Height, "PixelWidth": bounds.Width * a.scale, "PixelHeight": bounds.Height * a.scale},
		"viewMode": viewMode, "painter": painterName, "backend": a.backend, "gc": a.gc, "activations": a.activations, "shutdowns": a.shutdowns, "deliveries": a.deliveries,
		"inputVerified": a.exercised, "count": a.count, "mounts": a.mounts, "unmounts": a.unmounts, "destroys": a.destroys,
	})
}
func write(path string, value any) {
	data, err := json.MarshalIndent(value, "", "  ")
	must(err)
	must(os.WriteFile(path+".tmp", data, 0600))
	must(os.Rename(path+".tmp", path))
}

func setPainter() error               { return os.Setenv("GOUI_PLAT_PAINTER", painterName) }
func prepareReports(dir string) error { return os.MkdirAll(dir, 0700) }
