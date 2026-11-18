// 原生装饰与标题栏交互验证。
// 环境：Windows/macOS 桌面，指定 GOUI_PLAT_PAINTER 与 GOUI_PLAT_SCALE=1/2。
// 启动：go run ./tests/window/platform/chrome_controls -chrome=integrated
// macOS 可加 -controls-position=16.5,18；-chrome=native 对照原生装饰。
// 初始：显示标题、原生控制按钮和客户区内容；Linux Integrated 应显式返回不支持。
// 操作与预期：拖动标题栏空白区域可移动；点击内容与原生按钮不能误拖动；
// 缩放/最大化/恢复后，客户区与绘制保持一致；macOS 指定位置只平移按钮组，
// 不改变按钮尺寸/间距；-move=false 禁止例程自行发起标题栏移动。
// 副作用：仅操作测试窗口。复位：重启。退出：关闭窗口。
// 本目录 go test 验证参数解析与命中区域，不能代替原生操作。
package main

import (
	"errors"
	"flag"
	"fmt"
	"log"
	"runtime"
	"strconv"
	"strings"

	"github.com/golang-gui/goui/core/geometry"
	"github.com/golang-gui/goui/platform"
	"github.com/golang-gui/goui/platform/events"
	"github.com/golang-gui/goui/platform/graphics"
)

func init() { runtime.LockOSThread() }

func main() {
	chromeFlag := flag.String("chrome", "integrated", "integrated or native")
	positionFlag := flag.String("controls-position", "", "macOS Integrated native buttons: x,y in client DIP; empty keeps native layout")
	moveFlag := flag.Bool("move", true, "allow explicit dragging of blank header space")
	traceFlag := flag.Bool("trace-input", false, "log pointer down/up and pressed movement")
	flag.Parse()
	chrome := platform.WindowChromeIntegrated
	switch *chromeFlag {
	case "integrated":
	case "native":
		chrome = platform.WindowChromeNative
	default:
		log.Fatal("-chrome must be integrated or native")
	}
	position, err := parseControlsPosition(*positionFlag)
	if err != nil {
		log.Fatal(err)
	}
	if err := run(chrome, position, *moveFlag, *traceFlag); err != nil {
		log.Fatal(err)
	}
}

func parseControlsPosition(value string) (*geometry.Point, error) {
	if value == "" {
		return nil, nil
	}
	x, y, ok := strings.Cut(value, ",")
	px, xErr := strconv.ParseFloat(strings.TrimSpace(x), 32)
	py, yErr := strconv.ParseFloat(strings.TrimSpace(y), 32)
	if !ok || xErr != nil || yErr != nil {
		return nil, fmt.Errorf("-controls-position must be x,y in DIP")
	}
	return &geometry.Point{X: float32(px), Y: float32(py)}, nil
}

type demo struct {
	win        platform.Window
	desktop    platform.DesktopWindow
	painter    graphics.Painter
	size       events.SizeEvent
	controls   geometry.Rectangle
	custom     bool
	clientMove bool
	allowMove  bool
	traceInput bool
	tileDown   bool
	dark       bool
	hover      platform.WindowHit
	pressed    platform.WindowHit
}

func run(chrome platform.WindowChrome, controlsPosition *geometry.Point, allowMove, traceInput bool) error {
	plat, err := platform.NewPlatform(platform.DefaultName(), "")
	if err != nil {
		return err
	}
	defer plat.Destroy()
	loop, err := plat.NewEventLoop()
	if err != nil {
		return err
	}
	defer loop.Destroy()
	d := &demo{allowMove: allowMove, traceInput: traceInput}
	d.win, err = plat.NewWindow(geometry.Size{Width: 800, Height: 560}, func(event platform.Event) {
		switch event := event.(type) {
		case events.CloseEvent:
			loop.Quit() // Run 返回后先释放 Painter，再销毁窗口。
		case events.SizeEvent:
			d.size = event
		case events.StateEvent:
			log.Printf("observed state=%v", event.State)
		case events.PaintEvent:
			d.paint()
		case events.PointerEvent:
			d.pointer(event)
		case events.KeyEvent:
			if event.EventType == events.KeyDown && event.Key == events.KeyR && !event.Repeat && event.Modifiers == 0 {
				if err := d.desktop.SetControlsPosition(nil); err != nil {
					log.Printf("reset controls: %v", err)
				} else {
					log.Print("controls position reset to native default")
					_ = d.win.RequestPaint()
				}
			}
		case events.FocusEvent:
			if !event.Focused {
				d.pressed, d.hover = platform.WindowHitDefault, platform.WindowHitDefault
				d.tileDown = false
				if d.win != nil {
					_ = d.win.RequestPaint()
				}
			}
		}
	}, platform.WindowOptions{Chrome: chrome})
	if err != nil {
		return err // Linux 明确返回不支持，不隐式回退。
	}
	defer d.win.Destroy()
	d.desktop = d.win.(platform.DesktopWindow)
	d.desktop.SetMinSize(360, 240)
	d.custom = runtime.GOOS == "windows" && d.desktop.Chrome() == platform.WindowChromeIntegrated
	if chrome == platform.WindowChromeIntegrated {
		if err := d.desktop.SetHitTest(d.hitTest); errors.Is(err, platform.ErrUnsupported) {
			d.clientMove = true
		} else if err != nil {
			return err
		}
	}
	d.painter, err = plat.NewPainter(d.win)
	if err != nil {
		return err
	}
	defer d.painter.Destroy()
	if err := d.win.(platform.WindowManagement).SetTitle("GOUI Integrated chrome"); err != nil {
		return err
	}
	if controlsPosition != nil {
		if err := d.desktop.SetControlsPosition(controlsPosition); err != nil {
			return fmt.Errorf("set controls position: %w", err)
		}
	}
	log.Printf("chrome=%v painter=%s; click the colored header tile to change theme; drag blank header space", d.desktop.Chrome(), d.painter.Name())
	log.Printf("header move enabled=%t; macOS: R restores native controls position; -move=false tests absence of implicit content dragging", allowMove)
	log.Print("Check native edges/shadow, resize/minimum size, maximize/normal, inactive appearance and HiDPI. Windows: Alt+Space, Alt+F4 and maximize-button Snap hover.")
	if err := d.win.(platform.WindowManagement).Show(); err != nil {
		return err
	}
	loop.Run()
	return nil
}

const headerHeight float32 = 52

func contains(r geometry.Rectangle, p geometry.Point) bool {
	return p.X >= r.X && p.Y >= r.Y && p.X < r.X+r.Width && p.Y < r.Y+r.Height
}

func (d *demo) tile() geometry.Rectangle {
	return geometry.Rect(max(16, d.controls.X+d.controls.Width+16), 12, 80, 28)
}

func (d *demo) buttonRect(hit platform.WindowHit) geometry.Rectangle {
	return geometry.Rect(d.size.Width-float32(platform.WindowHitClose-hit+1)*44, 12, 44, 32)
}

// 回调只读取已完成布局，不查询 AppKit，不修改窗口、不启动布局或调用 Painter。
func (d *demo) hitTest(p geometry.Point) platform.WindowHit {
	if !contains(geometry.Rect(0, 0, d.size.Width, d.size.Height), p) || p.Y < 12 {
		return platform.WindowHitDefault // 保留原生边缘，包括顶部区域。
	}
	if d.custom {
		for _, hit := range []platform.WindowHit{platform.WindowHitMinimize, platform.WindowHitMaximize, platform.WindowHitClose} {
			if contains(d.buttonRect(hit), p) {
				return hit
			}
		}
	}
	if contains(d.tile(), p) || contains(d.controls, p) {
		return platform.WindowHitClient
	}
	if d.allowMove && p.Y < headerHeight {
		return platform.WindowHitCaption
	}
	return platform.WindowHitDefault
}

func (d *demo) pointer(event events.PointerEvent) {
	if d.win == nil || d.desktop == nil {
		return
	}
	hit := d.hitTest(event.Position)
	if d.traceInput && (event.EventType == events.PointerDown || event.EventType == events.PointerUp ||
		event.EventType == events.PointerMove && event.Buttons&events.PointerButtonLeftDown != 0) {
		log.Printf("pointer: %+v; hit=%v", event, hit)
	}
	switch event.EventType {
	case events.PointerMove:
		if d.hover != hit {
			d.hover = hit
			_ = d.win.RequestPaint()
		}
	case events.PointerLeave:
		d.hover = platform.WindowHitDefault
		d.tileDown = false
		_ = d.win.RequestPaint()
	case events.PointerDown:
		if event.Button != events.PointerButtonLeft {
			return
		}
		d.tileDown = false
		if d.clientMove && hit == platform.WindowHitCaption {
			// AppKit 原始按下事件只在本回调内有效；交给系统后不能启动点击/捕获或等待释放。
			if err := d.desktop.BeginMove(); err != nil {
				log.Printf("begin move: %v", err)
			}
			return
		}
		if hit >= platform.WindowHitMinimize && hit <= platform.WindowHitClose {
			d.pressed = hit
		} else if contains(d.tile(), event.Position) {
			d.tileDown = true
		}
		_ = d.win.RequestPaint()
	case events.PointerUp:
		if event.Button != events.PointerButtonLeft {
			return
		}
		pressed := d.pressed
		d.pressed = platform.WindowHitDefault
		if d.tileDown && contains(d.tile(), event.Position) {
			d.dark = !d.dark
			log.Print("header tile clicked")
		}
		d.tileDown = false
		if pressed == hit {
			var err error
			switch hit {
			case platform.WindowHitMinimize:
				err = d.desktop.RequestState(platform.WindowStateMinimized)
			case platform.WindowHitMaximize:
				if d.desktop.State() == platform.WindowStateMaximized {
					err = d.desktop.RequestState(platform.WindowStateNormal)
				} else {
					err = d.desktop.RequestState(platform.WindowStateMaximized)
				}
			case platform.WindowHitClose:
				err = d.win.(platform.WindowManagement).RequestClose()
			}
			if err != nil {
				log.Printf("caption request: %v", err)
			}
		}
		_ = d.win.RequestPaint()
	}
}

func (d *demo) paint() {
	if d.painter == nil || d.size.Width <= 0 || d.size.Height <= 0 {
		return
	}
	if r, err := d.desktop.ControlsRect(); err == nil && r != d.controls {
		d.controls = r
		log.Printf("native controls in client DIP: %v", r)
	}
	p := d.painter
	w, h := d.size.Width, d.size.Height
	scale := d.size.PixelWidth / w
	background, header, ink := graphics.RGB(242, 245, 248), graphics.RGB(222, 230, 238), graphics.RGB(35, 45, 60)
	if d.dark {
		background, header, ink = graphics.RGB(27, 31, 38), graphics.RGB(38, 44, 54), graphics.RGB(225, 232, 240)
	}
	p.Begin(d.size.PixelWidth, d.size.PixelHeight, scale)
	defer p.End()
	// 绘制整个不透明客户区及四角；可见窗口轮廓与阴影由操作系统拥有。
	p.Clear(background)
	p.FillRect(geometry.Rect(0, 0, w, headerHeight), header)
	p.FillRoundRect(d.tile(), 6, graphics.RGB(35, 150, 125))
	if d.controls.Width > 0 && d.controls.Y >= 0 {
		p.DrawRect(d.controls.Inset(-3), 1, graphics.RGB(35, 150, 125))
	}
	for y := headerHeight + 24; y < h-24; y += 40 {
		p.DrawLine(geometry.Point{X: 24, Y: y}, geometry.Point{X: w - 24, Y: y}, 1, header)
	}
	p.DrawRect(geometry.Rect(8, headerHeight+8, w-16, h-headerHeight-16), 1, ink)
	if !d.custom {
		return // macOS 交通灯保持真实的原生按钮。
	}
	for _, hit := range []platform.WindowHit{platform.WindowHitMinimize, platform.WindowHitMaximize, platform.WindowHitClose} {
		r := d.buttonRect(hit)
		if d.hover == hit {
			color := graphics.RGB(130, 165, 180)
			if hit == platform.WindowHitClose {
				color = graphics.RGB(220, 80, 85)
			}
			if d.pressed == hit {
				color = graphics.RGB(105, 120, 140)
			}
			p.FillRoundRect(r.Inset(2), 5, color)
		}
		c := r.Center()
		switch hit {
		case platform.WindowHitMinimize:
			p.DrawLine(geometry.Point{X: c.X - 5, Y: c.Y}, geometry.Point{X: c.X + 5, Y: c.Y}, 1, ink)
		case platform.WindowHitMaximize:
			if d.desktop.State() == platform.WindowStateMaximized {
				p.DrawRect(geometry.Rect(c.X-3, c.Y-6, 9, 9), 1, ink)
			}
			p.DrawRect(geometry.Rect(c.X-5, c.Y-4, 9, 9), 1, ink)
		case platform.WindowHitClose:
			p.DrawLine(geometry.Point{X: c.X - 5, Y: c.Y - 5}, geometry.Point{X: c.X + 5, Y: c.Y + 5}, 1, ink)
			p.DrawLine(geometry.Point{X: c.X - 5, Y: c.Y + 5}, geometry.Point{X: c.X + 5, Y: c.Y - 5}, 1, ink)
		}
	}
}
