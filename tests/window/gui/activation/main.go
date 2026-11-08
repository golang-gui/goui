//go:build android || ios

// GUI Activate/Shutdown 移动线程验收，不依赖 DevServer 或测试原生宿主。
// 环境：Android API29+ ARM64；iOS ARM64 Simulator/device。启动、操作、逐项预期、
// 差异和复位见 README.md。通过 cmd/goui 构建，默认 Software；可切换 OpenGL ES。
// 初始：多语言文字、绿色块和蓝色图片。线程错误或资源创建失败立即 panic。
// 周期投递仅为验收负载，不作为生产帧调度。输入、IME、Popup 不属于本例范围。
package main

import (
	"encoding/json"
	"fmt"
	"image"
	"image/color"
	"os"
	"path/filepath"
	"runtime"
	"sync/atomic"
	"time"

	"github.com/golang-gui/goui/gui"
	"github.com/golang-gui/goui/platform"
	"github.com/golang-gui/goui/platform/events"
	"github.com/golang-gui/goui/platform/graphics"
	"github.com/golang-gui/goui/platform/typography"
)

// 使用 -ldflags '-X main.painterName=opengl' 构建 GLES 版本。
var painterName = "software"

type application struct {
	backend     platform.Platform
	app         gui.Application
	timer       *gui.Timer
	activations int
	shutdowns   int
	window      platform.Window
	text        typography.Context
	layout      typography.TextLayout
	painter     graphics.Painter
	image       graphics.Image
	dir         string
	nonce       int64
	startup     uint64
	ui          uint64
	size        events.SizeEvent
	frames      int
	posts       int
	lost        int
	visible     bool
	focused     bool
	stopped     bool
	gc          bool
	deliveries  int
	exited      atomic.Bool
}

func must(err error) {
	if err != nil {
		panic(err)
	}
}

func main() {
	if mainThread() {
		panic("启动入口不应阻塞原生 UI 主线程")
	}
	must(os.Setenv("GOUI_PLAT_PAINTER", painterName))
	a := &application{nonce: time.Now().UnixNano(), startup: threadID()}
	a.dir = reportDirectory()
	must(os.MkdirAll(a.dir, 0700))
	var err error
	a.app, err = gui.NewApplication("org.golanggui.MobileActivation")
	must(err)
	a.backend = a.app.Platform()
	if a.app.Typography() != nil {
		panic("移动准备阶段提前创建排版服务")
	}
	a.app.ConnectActivate(func() {
		a.checkThread()
		a.activations++
		if a.activations != 1 || a.app.Typography() == nil {
			panic("激活次数或服务就绪错误")
		}
		source := a.backend.(platform.WindowSource)
		must(source.SetWindowHandler(a.start))
	})
	a.app.ConnectShutdown(a.stop)
	must(a.app.Run())
	if !a.exited.Load() {
		panic("Run 在原生停止回调返回前结束")
	}
	write(filepath.Join(a.dir, "completed.json"), map[string]any{
		"nonce": a.nonce, "callbackReturned": true, "frames": a.frames,
	})
}

func (a *application) checkThread() {
	if !mainThread() || threadID() == a.startup || (a.ui != 0 && a.ui != threadID()) {
		panic("GUI 操作没有归属原生 UI 主线程")
	}
	a.ui = threadID()
}

func (a *application) start(window platform.Window) platform.EventHandler {
	a.checkThread()
	a.deliveries++
	if a.deliveries != 1 {
		panic("同一逻辑宿主被重复交付")
	}
	a.window = window
	var err error
	a.text = a.app.Typography()
	a.layout, err = a.text.NewTextLayout("GOUI Activate / Shutdown\n中文 日本語 한국어 العربية", typography.TextFormat{
		Font: typography.FontInfo{Size: 18}, TextColor: color.RGBA{R: 24, G: 40, B: 56, A: 255}, WrapMode: typography.WrapWordChar,
	}, 320, 100)
	must(err)
	if _, managed := a.window.(platform.WindowManagement); managed {
		panic("系统宿主不应伪造窗口管理能力")
	}
	// 更换交付函数不得重新交付已有宿主，也不得更换现有事件处理函数。
	source := a.backend.(platform.WindowSource)
	must(source.SetWindowHandler(func(platform.Window) platform.EventHandler {
		panic("重复交付现有宿主")
	}))
	a.save()
	a.timer = a.app.NewTimer()
	a.timer.ConnectTimeout(a.tick)
	must(a.timer.Start(100 * time.Millisecond))
	return a.event
}

func (a *application) event(event platform.Event) {
	a.checkThread()
	if a.stopped {
		return
	}
	switch e := event.(type) {
	case events.SizeEvent:
		a.size = e
		a.layout.SetSize(max(1, e.Width-32), 100)
	case events.VisibilityEvent:
		a.visible = e.Visible
	case events.FocusEvent:
		a.focused = e.Focused
	case events.SurfaceEvent:
		if !e.State.Ready {
			a.lost++
		}
	case events.PaintEvent:
		a.paint()
	}
	a.save()
}

func (a *application) paint() {
	if a.size.Width <= 0 || !a.window.SurfaceState().Ready {
		panic("绘制先于有效尺寸或原生目标")
	}
	if a.painter == nil {
		var err error
		a.painter, err = a.backend.NewPainter(a.window)
		must(err)
		wantName := "Software"
		if painterName == "opengl" {
			wantName = "OpenGL"
		}
		if a.painter.Name() != wantName {
			panic(fmt.Sprintf("绘制后端意外降级：%s", a.painter.Name()))
		}
		pixels := image.NewRGBA(image.Rect(0, 0, 8, 8))
		for y := range 8 {
			for x := range 8 {
				pixels.SetRGBA(x, y, color.RGBA{R: 48, G: 128, B: 224, A: 255})
			}
		}
		a.image, err = a.painter.NewImage(pixels)
		must(err)
	}
	a.painter.Begin(a.size.PixelWidth, a.size.PixelHeight, a.size.PixelWidth/a.size.Width)
	a.painter.Clear(graphics.RGB(244, 247, 250))
	a.painter.DrawTextLayout(graphics.Point{X: 16, Y: 40}, a.layout)
	a.painter.FillRect(graphics.Rect(16, 150, 100, 60), graphics.RGB(32, 144, 96))
	a.painter.DrawImage(graphics.Rect(136, 150, 60, 60), a.image)
	a.painter.End()
	a.frames++
}

func (a *application) tick() {
	a.checkThread()
	if _, err := os.Stat(filepath.Join(a.dir, "stop")); err == nil {
		a.app.Quit()
		return
	}
	a.posts++
	if a.frames > 0 && !a.gc {
		runtime.GC()
		a.checkThread()
		a.gc = true
	}
	must(a.window.RequestPaint())
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
	if a.image != nil {
		a.image.Destroy()
	}
	if a.painter != nil {
		a.painter.Destroy()
	}
	if a.layout != nil {
		a.layout.Destroy()
	}
	if a.window != nil {
		a.window.Destroy()
	}
	a.save()
}

func (a *application) save() {
	state := platform.SurfaceState{}
	if a.window != nil {
		state = a.window.SurfaceState()
	}
	write(filepath.Join(a.dir, "dispatch.json"), map[string]any{
		"nonce": a.nonce, "startupThread": a.startup, "uiThread": a.ui, "mainThread": mainThread(),
		"frames": a.frames, "posts": a.posts, "lost": a.lost, "visible": a.visible, "stopped": a.stopped,
		"surface": state, "size": a.size, "painter": painterName,
		"activations": a.activations, "shutdowns": a.shutdowns, "gc": a.gc, "deliveries": a.deliveries, "focused": a.focused,
	})
}

func write(path string, value any) {
	data, err := json.MarshalIndent(value, "", "  ")
	must(err)
	must(os.WriteFile(path+".tmp", data, 0600))
	must(os.Rename(path+".tmp", path))
}
