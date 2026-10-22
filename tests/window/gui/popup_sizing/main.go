// 弹窗物理尺寸对齐回归。需要真实桌面、原生字体和绘制后端。
// 启动：go run ./tests/window/gui/popup_sizing；Linux 无 IME 服务时设置
// XMODIFIERS=@im=none。分别用 GOUI_PLAT_SCALE=1/1.25/1.5/1.75/2 运行；
// Linux 分别检查 Software、OpenGL，其他平台另行原生运行。
// 初始为 Integrated 窗口，自动打开有分数 DIP 宽高的无框浮层，再验证首次
// 创建、变大、缩小与再次变大：实际正文不得小于请求，多余空间小于一物理像素。
// 随后打开短菜单，检查没有滚动条；动态改长标签后仍撑开，显式限高时仅出现
// 纵条；标签超过工作区时双条可见且滚轮有效；恢复短菜单并重新打开后双条
// 消失。键盘通过正常 Window.DispatchEvent 激活菜单，断言只执行一次并关闭。
// 每步等待真实布局/绘制帧，超时或提前关闭返回非零，成功自动关闭。
// -inspect 在通过后保留正常短菜单，供人工检查框、阴影及滚动条；关闭退出。
// 不读写剪贴板、文件或系统设置；重启复位。强制缩放不代替真实混合 DPI 验收。
package main

import (
	"flag"
	"fmt"
	"image"
	"log"
	"runtime"
	"strings"
	"time"

	"github.com/golang-gui/goui/core/geometry"
	"github.com/golang-gui/goui/gui"
	"github.com/golang-gui/goui/layout"
	"github.com/golang-gui/goui/platform/events"
	"github.com/golang-gui/goui/platform/graphics"
	"github.com/golang-gui/goui/style"
	"github.com/golang-gui/goui/theme/modern"
)

func must(err error) {
	if err != nil {
		panic(err)
	}
}
func check(ok bool, message string) {
	if !ok {
		panic(message)
	}
}

type sizedContent struct {
	gui.WidgetBase
	preferred geometry.Size
	scale     float32
	painter   string
}

func (w *sizedContent) Measure(c layout.Constraint) layout.Measurement {
	return layout.Measured(c.Clamp(w.preferred))
}
func (w *sizedContent) Paint(p gui.Painter) {
	w.scale = p.PixelScale()
	if w.painter == "" {
		i, err := p.NewImage(image.NewRGBA(image.Rect(0, 0, 1, 1)))
		must(err)
		w.painter = fmt.Sprintf("%T", i)
		i.Destroy()
	}
	p.FillRect(geometry.Rectangle{Size: w.Rect().Size}, graphics.RGB(225, 238, 250))
}

func bars(w gui.Widget) (horizontal, vertical bool) {
	if b, ok := w.(*gui.ScrollBar); ok && b.Visible() {
		return b.Orientation() == layout.DirectionHorizontal, b.Orientation() == layout.DirectionVertical
	}
	for _, child := range w.Children() {
		h, v := bars(child)
		horizontal, vertical = horizontal || h, vertical || v
	}
	return
}
func scrolling(w gui.WidgetInfo) gui.WidgetInfo {
	if w.Role == gui.RoleScrollView {
		return w
	}
	for _, child := range w.Children {
		if found := scrolling(child); found.Role != "" {
			return found
		}
	}
	return gui.WidgetInfo{}
}

func main() {
	runtime.LockOSThread()
	inspect := flag.Bool("inspect", false, "keep the verified menu open")
	flag.Parse()
	app, err := gui.NewApplication("org.golang-gui.PopupSizing")
	must(err)
	rules := modern.Rules(modern.Options{})
	rules = append(rules, style.Name("sizing-surface").BackgroundColor(graphics.RGB(225, 238, 250)).BorderWidth(0).Radius(0).Shadow(style.Shadow{}))
	app.SetStyleSheet(style.Sheet(rules...))
	win, err := app.NewWindow(&gui.WindowOptions{Size: geometry.Size{Width: 760, Height: 420}, Chrome: gui.WindowChromeIntegrated})
	must(err)
	defer win.Destroy()
	win.SetMinSize(geometry.Size{Width: 480, Height: 240})
	must(win.SetTitle("Popup sizing — native pixel alignment"))
	anchor := gui.NewButton()
	anchor.SetChild(gui.NewLabel("Popup sizing regression"))
	box := gui.NewLinearBox(layout.DirectionVertical)
	box.SetCrossAlign(layout.CrossStretch)
	header := gui.NewHeaderBar()
	header.SetChild(gui.NewLabel("Popup sizing"))
	box.AddChild(header)
	box.AddChild(anchor)
	win.SetWidget(box)
	must(win.Show())
	win.SetFocusedWidget(anchor)
	content := &sizedContent{preferred: geometry.Size{Width: 227, Height: 123.25}}
	popup := gui.NewPopover(anchor, nil)
	defer popup.Destroy()
	popup.SetStyleName("sizing-surface")
	popup.SetWidget(content)
	popup.SetPosition(geometry.Point{X: 20, Y: 36})
	model := gui.NewMenu()
	activated := 0
	model.Append("First", func() { activated++ })
	long := model.Append("Duplicate Lines Down", nil)
	model.AppendSeparator()
	for i := 0; i < 4; i++ {
		model.Append(fmt.Sprintf("Other item %d", i), nil)
	}
	menu := gui.NewPopoverMenu(anchor)
	menu.SetMenu(model)
	var menuPopup gui.Popover
	frames, usedFrame, stage := 0, 0, 0
	presentation := 0
	observe := func(p gui.Popover) {
		p.ConnectFrame(func(time.Time) { frames++ })
		presentation = frames
	}
	observe(popup)
	key := func(k events.Key) {
		for _, kind := range []events.EventType{events.KeyDown, events.KeyUp} {
			must(win.DispatchEvent(events.KeyEvent{EventType: kind, Key: k}))
		}
	}
	verifySize := func() {
		got, want, scale := content.Rect().Size, content.preferred, content.scale
		check(scale > 0, "popup has not painted")
		for _, pair := range [][2]float32{{got.Width, want.Width}, {got.Height, want.Height}} {
			// 0.001 DIP 只容纳 float32 除法误差；没有吞掉不足一像素的裁切。
			check(pair[0]+.001 >= pair[1] && pair[0]-pair[1] < 1/scale+.001, fmt.Sprintf("native extent does not minimally contain request: actual=%v request=%v scale=%g", got, want, scale))
		}
		log.Printf("native size=%v requested=%v scale=%g painter=%s", got, want, scale, content.painter)
	}
	verifyMenu := func(h, v bool) {
		gotH, gotV := bars(menuPopup.Widget())
		check(gotH == h && gotV == v, fmt.Sprintf("unexpected menu bars: got=%t/%t want=%t/%t", gotH, gotV, h, v))
		info := scrolling(menuPopup.Widget().Snapshot())
		check(info.Role == gui.RoleScrollView && (info.MaxScrollX > 0) == h && (info.MaxScrollY > 0) == v, "menu ranges disagree with bars")
		if !h && !v {
			natural := menuPopup.Widget().Measure(layout.Unbounded()).Size
			actual := menuPopup.Widget().Rect().Size
			check(actual.Width+.001 >= natural.Width && actual.Height+.001 >= natural.Height, "menu is smaller than its natural content")
		}
	}
	done := false
	deadline := time.Now().Add(30 * time.Second)
	timer := app.NewTimer()
	defer timer.Stop()
	timer.ConnectTimeout(func() {
		check(time.Now().Before(deadline), fmt.Sprintf("timeout at stage %d", stage))
		if stage == 0 {
			must(popup.Show())
			stage++
			return
		}
		// ConnectFrame 在布局前发出，跨两个实际绘制帧再读取最终分配。
		if stage != 12 {
			active := popup
			if stage >= 5 {
				active = menuPopup
			}
			must(active.RequestPaint())
			if frames <= presentation+1 || frames == usedFrame {
				return
			}
			usedFrame = frames
		}
		switch stage {
		case 1, 2, 3:
			verifySize()
			sizes := []geometry.Size{{Width: 330.25, Height: 180.5}, {Width: 91.25, Height: 61.125}, {Width: 227, Height: 123.25}}
			content.preferred = sizes[stage-1]
			content.RequestLayout()
		case 4:
			verifySize()
			popup.Hide()
			must(menu.ShowAt(geometry.Point{X: 20, Y: 36}))
			check(len(win.Popovers()) == 1, "menu did not replace size probe")
			menuPopup = win.Popovers()[0]
			observe(menuPopup)
		case 5:
			verifyMenu(false, false)
			long.SetLabel("Duplicate selected lines below the current line")
		case 6:
			verifyMenu(false, false)
			menu.SetMaxHeight(80)
		case 7:
			verifyMenu(false, true)
			long.SetLabel(strings.Repeat("wide-label-", 200))
		case 8:
			verifyMenu(true, true)
			info := scrolling(menuPopup.Widget().Snapshot())
			must(menuPopup.DispatchEvent(events.WheelEvent{Position: geometry.Point{X: info.Bounds.X + 20, Y: info.Bounds.Y + 20}, DeltaY: 50}))
		case 9:
			check(scrolling(menuPopup.Widget().Snapshot()).ScrollY > 0, "constrained menu did not scroll")
			long.SetLabel("Duplicate Lines Down")
			menu.SetMaxHeight(0)
		case 10:
			verifyMenu(false, false)
			menu.Hide()
			must(menu.ShowAt(geometry.Point{X: 20, Y: 36}))
		case 11:
			verifyMenu(false, false)
			key(events.KeyHome)
			key(events.KeyEnter)
		case 12:
			check(activated == 1 && !menu.Visible(), "menu did not activate and close exactly once")
			done = true
			log.Print("PASS popup creation/grow/shrink, natural menu, label updates, height/workarea limits, wheel, restore, reopen and keyboard activation")
			if *inspect {
				must(menu.ShowAt(geometry.Point{X: 20, Y: 36}))
				timer.Stop()
				return
			}
			win.Destroy()
		}
		presentation = frames
		stage++
	})
	must(timer.Start(100 * time.Millisecond))
	app.Run()
	check(done, "popup sizing verification aborted")
}
