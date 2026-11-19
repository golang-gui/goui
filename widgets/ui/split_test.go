package ui

import (
	"testing"

	"github.com/golang-gui/goui/core/geometry"
	"github.com/golang-gui/goui/gui"
	"github.com/golang-gui/goui/layout"
	"github.com/golang-gui/goui/platform/events"
	baseui "github.com/golang-gui/goui/ui"
	"github.com/golang-gui/goui/widgets"
)

func TestSplitViewBindingsAndStableSlots(t *testing.T) {
	c := new(tabContext)
	v := HSplit(baseui.Label("start"), baseui.Label("end")).Ratio(.5).StartSize(120)
	w := v.Mount(c).(*widgets.SplitView)
	v.Update(c, w)
	state := c.State().(*splitState)
	start, end := w.StartChild(), w.EndChild()
	if start == nil || end == nil || start == end || len(c.children) != 2 {
		t.Fatal("two independent slots not mounted")
	}
	// 总宽403：默认3 DIP分隔区域之外保留原有400 DIP面板分配断言。
	size := geometry.Size{Width: 403, Height: 100}
	w.Measure(layout.Tight(size))
	w.Arrange(geometry.Rectangle{Size: size})
	if s, e := w.Sizes(); s != 120 || e != 280 {
		t.Fatalf("last sizing declaration lost: %g,%g", s, e)
	}
	v = VSplit(baseui.Label("new start"), nil).EndSize(70)
	v.Update(c, w)
	if c.State() != state || w.StartChild() != start || w.EndChild() != nil || end.Parent() != nil || len(c.children) != 1 {
		t.Fatal("slot update lost identity or nil failed to detach")
	}
	v.Unmount(c, w)
}

type splitUIHost struct{ root, focus gui.Widget }

func (h *splitUIHost) Widget() gui.Widget                 { return h.root }
func (h *splitUIHost) FocusedWidget() gui.Widget          { return h.focus }
func (h *splitUIHost) SetFocusedWidget(w gui.Widget) bool { h.focus = w; return true }
func TestSplitViewUncontrolledAndControlledUpdates(t *testing.T) {
	c := new(tabContext)
	old, latest := 0, 0
	v := HSplit(baseui.Label("a"), baseui.Label("b")).Ratio(.5).OnResize(func(float32, float32) { old++ })
	w := v.Mount(c).(*widgets.SplitView)
	v.Update(c, w)
	// 固定布局尺寸，避免将绑定测试变成字体排版测试。
	w.StartChild().SetMinSize(geometry.Size{Width: 1, Height: 1})
	w.EndChild().SetMinSize(geometry.Size{Width: 1, Height: 1})
	size := geometry.Size{Width: 403, Height: 100}
	arrange := func() { w.Measure(layout.Tight(size)); w.Arrange(geometry.Rectangle{Size: size}) }
	arrange()
	v = HSplit(baseui.Label("a"), baseui.Label("b")).OnResize(func(float32, float32) { latest++ })
	v.Update(c, w)
	host := &splitUIHost{root: w}
	dispatcher := new(gui.EventDispatcher)
	point := func(kind events.EventType, x float32) {
		buttons := events.PointerButtonLeftDown
		if kind == events.PointerUp {
			buttons = 0
		}
		if err := dispatcher.DispatchEvent(host, events.PointerEvent{EventType: kind, Position: geometry.Point{X: x, Y: 50}, Button: events.PointerButtonLeft, Buttons: buttons}); err != nil {
			t.Fatal(err)
		}
	}
	point(events.PointerDown, 200)
	point(events.PointerMove, 250)
	point(events.PointerUp, 250)
	if old != 0 || latest != 1 {
		t.Fatalf("stale callback: old=%d latest=%d", old, latest)
	}
	v.Update(c, w)
	arrange()
	if s, _ := w.Sizes(); s != 250 {
		t.Fatal("uncontrolled rebuild reset adjustment")
	}
	v = HSplit(baseui.Label("a"), baseui.Label("b")).Ratio(.5)
	v.Update(c, w)
	arrange()
	if s, _ := w.Sizes(); s != 200 {
		t.Fatal("controlled ratio not reapplied")
	}
	v.Unmount(c, w)
	point(events.PointerDown, 200)
	point(events.PointerMove, 220)
	point(events.PointerUp, 220)
	if latest != 1 {
		t.Fatal("unmounted adapter still invoked callback")
	}
}

func TestSplitViewHandleSizeBindingAndReset(t *testing.T) {
	c := new(tabContext)
	v := HSplit(baseui.Label("a"), baseui.Label("b")).StartSize(100).HandleSize(9)
	w := v.Mount(c).(*widgets.SplitView)
	v.Update(c, w)
	arrange := func() {
		size := geometry.Size{Width: 209, Height: 80}
		w.Measure(layout.Tight(size))
		w.Arrange(geometry.Rectangle{Size: size})
	}
	arrange()
	if a, b := w.Sizes(); w.HandleSize() != 9 || a != 100 || b != 100 {
		t.Fatal("声明厚度未生效")
	}
	v = HSplit(baseui.Label("a"), baseui.Label("b")).StartSize(100)
	v.Update(c, w)
	arrange()
	if a, b := w.Sizes(); w.HandleSize() != 3 || a != 100 || b != 106 {
		t.Fatal("移除声明没有恢复默认，或改写面板尺寸偏好")
	}
	v = VSplit(baseui.Label("a"), baseui.Label("b")).HandleSize(5)
	v.Update(c, w)
	if w.Direction() != layout.DirectionVertical || w.HandleSize() != 5 {
		t.Fatal("方向／厚度未同步")
	}
	v.Unmount(c, w)
}
