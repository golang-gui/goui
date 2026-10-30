package widgets

import (
	"image/color"
	"testing"

	"github.com/golang-gui/goui/core/geometry"
	"github.com/golang-gui/goui/gui"
	"github.com/golang-gui/goui/layout"
	"github.com/golang-gui/goui/platform/events"
	"github.com/golang-gui/goui/style"
	"github.com/golang-gui/goui/theme/basic"
	"github.com/golang-gui/goui/theme/modern"
)

func TestDialogContentLayoutAndSnapshot(t *testing.T) {
	useProgressApplication(t, &progressTestApplication{sheet: style.Sheet(
		style.Name("dialog").Part("card").BackgroundColor(color.White),
	)})
	d := NewDialog()
	child := splitTestChild(120, 60)
	d.SetChild(child)
	if d.Padding() != geometry.UniformInsets(24) || d.CloseOnOutside() || d.BackdropCaption() {
		t.Fatal("默认参数不符合确认层约定")
	}
	m := d.Measure(layout.Loose(geometry.Size{Width: 400, Height: 300}))
	if m.Size != (geometry.Size{Width: 168, Height: 108}) {
		t.Fatalf("尺寸应由内容和 Padding 决定：%+v", m)
	}
	d.Arrange(geometry.Rect(9, 17, 400, 300))
	if d.card != geometry.Rect(116, 96, 168, 108) || child.Rect() != geometry.Rect(140, 120, 120, 60) {
		t.Fatalf("卡片应在局部坐标居中：card=%+v child=%+v", d.card, child.Rect())
	}
	for _, p := range []geometry.Point{{X: 117, Y: 97}, {X: 280, Y: 200}, {X: 150, Y: 120}} {
		if !pointInRect(d.card, p) {
			t.Fatal("卡片内边距和内容区域应使用同一个命中矩形")
		}
	}
	d.Arrange(geometry.Rect(0, 0, 30, 20))
	if d.card.Width > 30 || d.card.Height > 20 || child.Rect().Width < 0 || child.Rect().Height < 0 {
		t.Fatal("父级硬约束未优先")
	}
	info := d.Snapshot()
	if info.Role != RoleDialog || len(info.Children) != 1 || len(info.Actions) != 0 {
		t.Fatal("语义不应伪造标题、动作或隐藏子树")
	}
	other := splitTestChild(20, 10)
	d.SetChild(other)
	if child.Parent() != nil || child.Destroyed() || other.Parent() != d {
		t.Fatal("替换内容不应销毁旧 Widget")
	}
}

func TestDialogRequestsCloseThroughInput(t *testing.T) {
	d := NewDialog()
	child := gui.NewButton()
	child.SetChild(gui.NewLabel("content"))
	d.SetChild(child)
	d.Arrange(geometry.Rect(0, 0, 400, 300))
	h := &tabInputHost{root: d, focus: child}
	dispatcher := new(gui.EventDispatcher)
	closed := 0
	d.ConnectCloseRequest(func() { closed++ })
	click := func(start, end geometry.Point) {
		t.Helper()
		for i, kind := range []events.EventType{events.PointerDown, events.PointerUp} {
			p := start
			if i != 0 {
				p = end
			}
			if err := dispatcher.DispatchEvent(h, events.PointerEvent{EventType: kind, Position: p, Button: events.PointerButtonLeft}); err != nil {
				t.Fatal(err)
			}
		}
	}
	outside := geometry.Point{X: 10, Y: 10}
	padding := d.card.Pos.Add(geometry.Point{X: 4, Y: 4})
	click(outside, outside)
	if closed != 0 {
		t.Fatal("默认不应点击外部关闭")
	}
	d.SetCloseOnOutside(true)
	click(padding, padding)
	if closed != 0 {
		t.Fatal("卡片内边距被当成外部点击")
	}
	click(outside, outside)
	if closed != 1 || !d.Visible() {
		t.Fatal("外部点击只应发出请求，不自动隐藏")
	}
	d.SetBackdropCaption(true)
	click(outside, outside)
	if closed != 1 {
		t.Fatal("Caption 不应同时触发关闭")
	}
	d.SetBackdropCaption(false)
	handled := false
	_ = dispatcher.DispatchEvent(h, events.KeyEvent{EventType: events.KeyDown, Key: events.KeyEscape, Handled: &handled})
	if !handled || closed != 2 {
		t.Fatal("未消费的 Escape 应触发关闭请求")
	}
	keys := gui.NewKeyEventController()
	keys.ConnectKeyDown(func(ctx gui.EventContext, e events.KeyEvent) { ctx.StopPropagation(); e.PreventDefault() })
	child.AddEventController(keys)
	_ = dispatcher.DispatchEvent(h, events.KeyEvent{EventType: events.KeyDown, Key: events.KeyEscape})
	if closed != 2 {
		t.Fatal("内容已消费的 Escape 不应再关闭 Dialog")
	}
}

func TestDialogCloseCallbacksStopAfterLifecycleChange(t *testing.T) {
	for _, destroy := range []bool{false, true} {
		d := NewDialog()
		host := gui.NewPopover(nil, nil)
		host.SetWidget(d)
		defer host.Destroy()
		called := 0
		d.ConnectCloseRequest(func() {
			if destroy {
				host.Destroy()
			} else {
				d.SetVisible(false)
			}
		})
		d.ConnectCloseRequest(func() { called++ })
		d.requestClose()
		if called != 0 {
			t.Fatal("销毁或隐藏后仍执行了剩余关闭回调")
		}
	}
}

func TestDialogToastSurfaceThemes(t *testing.T) {
	for name, sheet := range map[string]style.StyleSheet{"basic": basic.Sheet(), "light": modern.Sheet(modern.Options{}), "dark": modern.Sheet(modern.Options{Dark: true})} {
		t.Run(name, func(t *testing.T) {
			for _, sel := range []style.Sel{{Name: "dialog", Part: "card"}, {Name: "toast"}} {
				s := sheet.Resolve(sel)
				bg, ok := s.BackgroundColor()
				radius, _ := s.Radius()
				width, _ := s.BorderWidth()
				if !ok || bg == nil || radius <= 0 || width != 1 || surfaceInsets(s) == (geometry.Insets{}) {
					t.Fatalf("缺少完整外观：%s/%s", sel.Name, sel.Part)
				}
			}
			bg, ok := sheet.Resolve(style.Sel{Name: "dialog", Part: "backdrop"}).BackgroundColor()
			if !ok || bg == nil {
				t.Fatal("缺少遮罩颜色")
			}
			_, _, _, a := bg.RGBA()
			if a == 0 || a == 65535 {
				t.Fatal("遮罩应半透明")
			}
		})
	}
}
