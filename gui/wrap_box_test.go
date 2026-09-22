package gui

import (
	"github.com/golang-gui/goui/core/geometry"
	"github.com/golang-gui/goui/layout"
	"github.com/golang-gui/goui/style"
	"math"
	"testing"
)

func TestWrapBoxTreeChangesAndSnapshot(t *testing.T) {
	box := NewWrapBox(layout.DirectionHorizontal)
	box.SetSpacing(5)
	box.SetLineSpacing(3)
	a, b, c := newSizedWidget(geometry.Size{Width: 40, Height: 10}), newSizedWidget(geometry.Size{Width: 40, Height: 20}), newSizedWidget(geometry.Size{Width: 40, Height: 10})
	box.AddChild(a)
	box.AddChild(b)
	box.AddChild(c)
	bounds := geometry.Rect(7, 9, 90, 100)
	box.Arrange(bounds)
	if c.Rect() != geometry.Rect(0, 23, 40, 10) {
		t.Fatal(c.Rect())
	}
	info := box.Snapshot()
	if info.Role != RoleWrapBox || info.Bounds != bounds || len(info.Children) != 3 || info.Children[2].Bounds != geometry.Rect(7, 32, 40, 10) || len(info.Actions) != 0 {
		t.Fatalf("%+v", info)
	}
	b.SetVisible(false)
	box.Arrange(bounds)
	if c.Rect() != geometry.Rect(45, 0, 40, 10) {
		t.Fatal(c.Rect())
	}
	box.RemoveChild(a)
	box.Arrange(bounds)
	if c.Rect().X != 0 || a.Parent() != nil {
		t.Fatal("removed child still occupies layout")
	}
	b.SetVisible(true)
	box.Arrange(bounds)
	if c.Rect() != geometry.Rect(45, 5, 40, 10) {
		t.Fatal(c.Rect())
	}
	box.AddChild(a)
	box.Arrange(bounds)
	if a.Rect().Y != 23 {
		t.Fatal("added child did not reflow", a.Rect())
	}
}

func TestWrapBoxSettersInvalidateOnlyOnChange(t *testing.T) {
	box := NewWrapBox(layout.DirectionHorizontal)
	win := &window{}
	win.SetWidget(box)
	setters := []struct {
		set func(float32)
		get func() float32
	}{
		{box.SetSpacing, box.Spacing}, {box.SetLineSpacing, box.LineSpacing}, {box.SetPadding, box.Padding},
	}
	for _, s := range setters {
		for _, invalid := range []float32{-1, float32(math.NaN()), float32(math.Inf(1))} {
			win.layoutDirty = false
			s.set(invalid)
			if s.get() != 0 || win.layoutDirty {
				t.Fatal("invalid/unchanged value invalidated layout")
			}
		}
		s.set(4)
		if s.get() != 4 || !win.layoutDirty {
			t.Fatal("changed value did not invalidate layout")
		}
		win.layoutDirty = false
		s.set(4)
		if win.layoutDirty {
			t.Fatal("unchanged value invalidated layout")
		}
	}
	for _, set := range []func(){func() { box.SetDirection(layout.DirectionVertical) }, func() { box.SetMainAlign(layout.MainEnd) }, func() { box.SetCrossAlign(layout.CrossStretch) }} {
		win.layoutDirty = false
		set()
		if !win.layoutDirty {
			t.Fatal("changed policy did not invalidate layout")
		}
		win.layoutDirty = false
		set()
		if win.layoutDirty {
			t.Fatal("unchanged policy invalidated layout")
		}
	}
	manager := box.LayoutManager()
	box.SetLayoutManager(layout.NewLinearLayout(layout.DirectionHorizontal))
	if box.LayoutManager() != manager {
		t.Fatal("accepted incompatible manager")
	}
}

func TestWrapBoxStyleChangeReflows(t *testing.T) {
	app := &application{}
	useTestApplication(t, app)
	win := &window{}
	app.windows = []*window{win}
	box := NewWrapBox(layout.DirectionHorizontal)
	a, b := &styleProbe{}, &styleProbe{}
	a.SetStyleName("wrap-item")
	b.SetStyleName("wrap-item")
	box.AddChild(a)
	box.AddChild(b)
	win.SetWidget(box)
	constraint := layout.Loose(geometry.Size{Width: 60, Height: 100})
	app.SetStyleSheet(style.Sheet(style.Name("wrap-item").FontSize(20)))
	if m := measureWidget(box, constraint); m.Size != (geometry.Size{Width: 40, Height: 20}) {
		t.Fatal(m)
	}
	app.SetStyleSheet(style.Sheet(style.Name("wrap-item").FontSize(40)))
	if m := measureWidget(box, constraint); m.Size != (geometry.Size{Width: 40, Height: 80}) {
		t.Fatal(m)
	}
	box.Arrange(geometry.Rect(0, 0, 60, 100))
	if b.Rect() != geometry.Rect(0, 40, 40, 40) {
		t.Fatal(b.Rect())
	}
}

func TestWrapBoxInsideScrollView(t *testing.T) {
	box := NewWrapBox(layout.DirectionHorizontal)
	box.SetLineSpacing(5)
	// Ordinary ScrollView content is measured unbounded on BOTH axes. Limit
	// the wrapping axis explicitly; WrapBox must not inspect its parent type.
	box.SetMaxSize(geometry.Size{Width: 120})
	for i := 0; i < 10; i++ {
		box.AddChild(newSizedWidget(geometry.Size{Width: 60, Height: 20}))
	}
	sv := NewScrollView()
	sv.SetChild(box)
	sv.Measure(layout.Tight(geometry.Size{Width: 150, Height: 50}))
	sv.Arrange(geometry.Rect(0, 0, 150, 50))
	// The explicit content width admits two 60 DIP items per row.
	// Five 20 DIP rows + four 5 DIP gaps = 120 DIP content height.
	if sv.contentHeight != 120 {
		t.Fatal(sv.contentHeight)
	}
	sv.SetScrollY(1000)
	if sv.ScrollY() != 70 || box.Rect().Y != -70 {
		t.Fatalf("scroll=%v rect=%v", sv.ScrollY(), box.Rect())
	}
}

func TestWrapBoxOwnWeightIsUsedByLinearParent(t *testing.T) {
	parent := NewLinearBox(layout.DirectionHorizontal)
	box := NewWrapBox(layout.DirectionHorizontal)
	box.SetMainWeight(1)
	box.AddChild(newSizedWidget(geometry.Size{Width: 20, Height: 10}))
	parent.AddChild(newSizedWidget(geometry.Size{Width: 30, Height: 10}))
	parent.AddChild(box)
	parent.Arrange(geometry.Rect(0, 0, 100, 40))
	if box.Rect().Width != 70 {
		t.Fatal(box.Rect())
	}
}

// A wrapping layout's unbounded preferred width is not a desktop minimum.
// Hosts that need reflow currently supply an explicit window minimum. Keep
// that hint authoritative through automatic minimum derivation and resizing.
func TestWrapBoxWindowExplicitMinimumAllowsReflow(t *testing.T) {
	box := NewWrapBox(layout.DirectionHorizontal)
	box.SetSpacing(8)
	box.SetLineSpacing(6)
	for range 3 {
		box.AddChild(newSizedWidget(geometry.Size{Width: 180, Height: 24}))
	}
	if got := measureWidget(box, layout.Unbounded()).Width; got != 556 {
		t.Fatalf("unbounded preferred width = %v, want 556", got)
	}
	native := &desktopTestWindow{}
	win := &window{platformWindow: native}
	win.SetWidget(box)
	t.Cleanup(win.Destroy)
	minimum := geometry.Size{Width: 400, Height: 600}
	win.SetMinSize(minimum)
	win.updateMinSize()
	win.updateMinSize()
	if len(native.minimums) != 1 || native.minimums[0] != minimum {
		t.Fatalf("explicit minimum overwritten by single-line width: %v", native.minimums)
	}
	for _, tc := range []struct{ width, lastX, lastY float32 }{
		{780, 376, 0}, {400, 0, 30}, {780, 376, 0},
	} {
		measureWidget(box, layout.Tight(geometry.Size{Width: tc.width, Height: 600}))
		box.Arrange(geometry.Rect(0, 0, tc.width, 600))
		if got := box.Children()[2].Rect(); got != geometry.Rect(tc.lastX, tc.lastY, 180, 24) {
			t.Fatalf("width=%v last child=%v", tc.width, got)
		}
	}
}
