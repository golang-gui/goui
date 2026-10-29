package widgets

import (
	"testing"

	"github.com/golang-gui/goui/core/geometry"
	"github.com/golang-gui/goui/gui"
	"github.com/golang-gui/goui/layout"
	"github.com/golang-gui/goui/platform/events"
)

// 跨用例文件共享的固定布局与输入派发；不创建原生窗口。

func dragTestBar() (*TabBar, *TabView, []*TabPage) {
	view := NewTabView()
	pages := []*TabPage{NewTabPage("A", nil), NewTabPage("B", nil), NewTabPage("C", nil)}
	for _, page := range pages {
		page.SetClosable(true)
		view.AppendPage(page)
	}
	bar := NewTabBar()
	bar.SetView(view)
	bar.SetReorderable(true)
	// Fixed equal slots isolate input geometry from font/desktop measurement.
	bar.SetTabWidthRange(100, 100)
	bar.Measure(layout.Loose(geometry.Size{Width: 400, Height: 40}))
	bar.Arrange(geometry.Rect(0, 0, 400, 40))
	return bar, view, pages
}

type tabInputHost struct {
	root, focus gui.Widget
}

func (h *tabInputHost) Widget() gui.Widget                 { return h.root }
func (h *tabInputHost) FocusedWidget() gui.Widget          { return h.focus }
func (h *tabInputHost) SetFocusedWidget(w gui.Widget) bool { h.focus = w; return true }

func dispatchTabPointer(t *testing.T, d *gui.EventDispatcher, host *tabInputHost, kind events.EventType, x, y float32) {
	t.Helper()
	buttons := events.PointerButtonLeftDown
	if kind == events.PointerUp {
		buttons = 0
	}
	if err := d.DispatchEvent(host, events.PointerEvent{
		EventType: kind, Position: geometry.Point{X: x, Y: y},
		Button: events.PointerButtonLeft, Buttons: buttons,
	}); err != nil {
		t.Fatal(err)
	}
}

// 通过正常 crossing 路径设置指针事实；不向生产 API 增加测试专用 setter。
func testWidgetHover(t *testing.T, widget gui.Widget, hover bool) {
	t.Helper()
	host := &tabInputHost{root: widget}
	dispatcher := new(gui.EventDispatcher)
	point := widget.Rect().Pos.Add(geometry.Point{X: 1, Y: 1})
	if err := dispatcher.DispatchEvent(host, events.PointerEvent{EventType: events.PointerMove, Position: point}); err != nil {
		t.Fatal(err)
	}
	if !hover {
		if err := dispatcher.DispatchEvent(host, events.PointerEvent{EventType: events.PointerLeave, Position: point}); err != nil {
			t.Fatal(err)
		}
	}
}
