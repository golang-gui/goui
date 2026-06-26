package gui

import (
	"testing"

	"github.com/golang-gui/goui/core/geometry"
	"github.com/golang-gui/goui/layout"
)

func TestLinearBoxDefaultsToLinearLayout(t *testing.T) {
	box := NewLinearBox(layout.DirectionHorizontal)
	first := newSizedWidget(geometry.Size{Width: 10, Height: 20})
	second := newSizedWidget(geometry.Size{Width: 30, Height: 15})
	box.SetSpacing(2)
	box.AddChild(first)
	box.AddChild(second)

	size := box.Measure(geometry.Size{Width: 100, Height: 50})
	if size != (geometry.Size{Width: 42, Height: 20}) {
		t.Fatalf("unexpected measured size: %+v", size)
	}

	box.Arrange(geometry.Rect(0, 0, 100, 40))
	if first.Rect() != geometry.Rect(0, 0, 10, 40) {
		t.Fatalf("unexpected first rect: %+v", first.Rect())
	}
	if second.Rect() != geometry.Rect(12, 0, 30, 40) {
		t.Fatalf("unexpected second rect: %+v", second.Rect())
	}
}

func TestLinearBoxDirectionAndSpacingRequestLayout(t *testing.T) {
	win := &window{}
	box := NewLinearBox(layout.DirectionHorizontal)
	win.SetWidget(box)

	win.layoutDirty = false
	win.paintDirty = false
	box.SetDirection(layout.DirectionHorizontal)
	box.SetSpacing(0)

	if win.layoutDirty || win.paintDirty {
		t.Fatal("setting unchanged layout properties should not request layout")
	}

	box.SetDirection(layout.DirectionVertical)
	if box.Direction() != layout.DirectionVertical {
		t.Fatalf("unexpected direction: %v", box.Direction())
	}
	if !win.layoutDirty || !win.paintDirty {
		t.Fatal("setting direction did not request layout and paint")
	}

	win.layoutDirty = false
	win.paintDirty = false
	box.SetSpacing(6)
	if box.Spacing() != 6 {
		t.Fatalf("unexpected spacing: %v", box.Spacing())
	}
	if !win.layoutDirty || !win.paintDirty {
		t.Fatal("setting spacing did not request layout and paint")
	}
}

func TestLinearBoxRejectsNonLinearLayoutManager(t *testing.T) {
	box := NewLinearBox(layout.DirectionHorizontal)
	manager := box.LayoutManager()

	box.SetLayoutManager(layout.NewFillLayout())

	if box.LayoutManager() != manager {
		t.Fatal("linear box accepted non-linear layout manager")
	}
}

func TestLinearBoxSnapshot(t *testing.T) {
	box := NewLinearBox(layout.DirectionHorizontal)
	box.SetID("content")

	info := box.Snapshot()
	if info.ID != "content" {
		t.Fatalf("unexpected snapshot id: %q", info.ID)
	}
	if info.Role != RoleBox {
		t.Fatalf("unexpected snapshot role: %q", info.Role)
	}
}

type sizedWidget struct {
	WidgetBase
	size geometry.Size
}

func newSizedWidget(size geometry.Size) *sizedWidget {
	w := &sizedWidget{
		size: size,
	}
	w.Init(w)
	return w
}

func (w *sizedWidget) Measure(available geometry.Size) geometry.Size {
	return w.size
}
