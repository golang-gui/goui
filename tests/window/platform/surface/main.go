// 原生 Surface 与 OpenGL 绘制交互验证。
// 环境：三平台桌面，原生 OpenGL；缩放 GOUI_PLAT_SCALE=1/2。
// 启动：go run ./tests/window/platform/surface
// 初始：无装饰窗口显示绘制内容，终端记录窗口尺寸和原生操作结果。
// 操作与预期：缩放窗口后内容应覆盖整个客户区；拖动指定区域只移动窗口，
// 点击其他区域不发起移动；反复缩放和移动后仍能重绘，无上下文/资源错误。
// 平台差异：原生交互不支持时应记录返回错误。副作用：移动/缩放测试窗口。
// 复位：重启。退出：窗口管理器关闭快捷键（通常 Alt+F4 或 Command+W）或 Ctrl-C。
package main

import (
	"errors"
	"fmt"
	"log"
	"runtime"

	"github.com/golang-gui/goui/core/geometry"
	"github.com/golang-gui/goui/platform"
	"github.com/golang-gui/goui/platform/events"
	"github.com/golang-gui/goui/platform/graphics"
	"github.com/golang-gui/goui/platform/graphics/opengl"
)

func init() {
	// 创建、事件、绘制和销毁在同一主线程执行；仅同一 goroutine 不保证 OpenGL 线程上下文。
	runtime.LockOSThread()
}

func main() {
	if err := run(); err != nil {
		log.Fatal(err)
	}
}

func run() error {
	plat, err := platform.NewPlatform(platform.DefaultName(), "org.golang-gui.PlatformDemo")
	if err != nil {
		return err
	}
	defer plat.Destroy()

	loop, err := plat.NewEventLoop()
	if err != nil {
		return err
	}
	defer loop.Destroy()

	var (
		painter           graphics.Painter
		desktop           platform.DesktopWindow
		clientInteraction bool

		width, height           float32
		pixelWidth, pixelHeight float32
		scale                   float32 = 1
	)
	hitTest := func(pt geometry.Point) platform.WindowHit {
		if pt.Y < 50 {
			return platform.WindowHitCaption
		}
		if width-10 < pt.X {
			if height-10 < pt.Y {
				return platform.WindowHitBottomRight
			}
			return platform.WindowHitRight
		}
		if height-10 < pt.Y {
			return platform.WindowHitBottom
		}
		return platform.WindowHitDefault
	}
	win, err := plat.NewWindow(geometry.Size{Width: 800, Height: 600}, func(event events.Event) {
		switch event := event.(type) {
		case events.CloseEvent:
			fmt.Println("close window")
			// Run 返回后先释放 Painter，再释放原生窗口。
			loop.Quit()
		case events.SizeEvent:
			width, height = event.Width, event.Height
			pixelWidth, pixelHeight = event.PixelWidth, event.PixelHeight
			if event.Width > 0 && event.PixelWidth > 0 {
				scale = event.PixelWidth / event.Width
			}
		case events.PaintEvent:
			// 初始通知可能在 NewWindow 返回前到达。
			if painter == nil || pixelWidth <= 0 || pixelHeight <= 0 {
				return
			}
			painter.Begin(pixelWidth, pixelHeight, scale)
			painter.Clear(graphics.RGBA(0, 0, 0, 0))
			painter.FillRoundRect(geometry.Rect(0, 0, width, height), 16, graphics.RGB(180, 180, 180))
			painter.End()
		case events.StateEvent:
			fmt.Println(event.State)
		case events.PointerEvent:
			if !clientInteraction || event.EventType != events.PointerDown || event.Button != events.PointerButtonLeft {
				return
			}
			// 这是调用方的交互策略；请求必须在原生按下仍有效的当前回调发起。
			var err error
			switch hitTest(event.Position) {
			case platform.WindowHitCaption:
				err = desktop.BeginMove()
			case platform.WindowHitRight:
				err = desktop.BeginResize(platform.WindowEdgeRight)
			case platform.WindowHitBottom:
				err = desktop.BeginResize(platform.WindowEdgeBottom)
			case platform.WindowHitBottomRight:
				err = desktop.BeginResize(platform.WindowEdgeBottomRight)
			}
			if err != nil {
				log.Printf("native window interaction: %v", err)
			}
		}
	}, platform.WindowOptions{
		Chrome: platform.WindowChromeNone,
	})
	if err != nil {
		return err
	}
	defer win.Destroy()

	painter, err = opengl.NewPainter(win)
	if err != nil {
		return err
	}
	defer painter.Destroy()

	desktop = win.(platform.DesktopWindow)
	if err := desktop.SetHitTest(hitTest); errors.Is(err, platform.ErrUnsupported) {
		clientInteraction = true
	} else if err != nil {
		return err
	}

	if err := win.(platform.WindowManagement).SetTitle("Platform test"); err != nil {
		return err
	}
	if err := win.(platform.WindowManagement).Show(); err != nil {
		return err
	}

	loop.Run()
	return nil
}
