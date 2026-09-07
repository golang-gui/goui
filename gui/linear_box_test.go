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

	size := box.Measure(layout.Loose(geometry.Size{Width: 100, Height: 50}))
	if size.Size != (geometry.Size{Width: 42, Height: 20}) {
		t.Fatalf("unexpected measured size: %+v", size)
	}

	box.Arrange(geometry.Rect(0, 0, 100, 40))
	// CrossDefault centers a horizontal row without stretching its children.
	if first.Rect() != geometry.Rect(0, 10, 10, 20) {
		t.Fatalf("unexpected first rect: %+v", first.Rect())
	}
	if second.Rect() != geometry.Rect(12, 12.5, 30, 15) {
		t.Fatalf("unexpected second rect: %+v", second.Rect())
	}
}

func TestLinearBoxPaddingInsetsContent(t *testing.T) {
	// Padding is a layout field handled on the WidgetBase common path, so a plain
	// box (no custom Measure) gets it: content is inset in Arrange, added in Measure.
	box := NewLinearBox(layout.DirectionVertical)
	box.SetPadding(10)
	child := newSizedWidget(geometry.Size{Width: 30, Height: 20})
	box.AddChild(child)

	size := box.Measure(layout.Loose(geometry.Size{Width: 500, Height: 500}))
	if size.Size != (geometry.Size{Width: 50, Height: 40}) { // 30+2*10, 20+2*10
		t.Fatalf("padding not added in measure: %+v", size)
	}

	box.Arrange(geometry.Rect(0, 0, 100, 80))
	if child.Rect() != geometry.Rect(10, 10, 30, 20) { // hugged child, offset by padding
		t.Fatalf("padding not applied in arrange: %+v", child.Rect())
	}
}

func TestLinearBoxAlignRequestLayout(t *testing.T) {
	win := &window{}
	box := NewLinearBox(layout.DirectionHorizontal)
	win.SetWidget(box)

	win.layoutDirty = false
	box.SetMainAlign(layout.MainStart)
	box.SetCrossAlign(layout.CrossDefault)
	if win.layoutDirty {
		t.Fatal("setting unchanged alignment should not request layout")
	}

	box.SetMainAlign(layout.MainCenter)
	if box.MainAlign() != layout.MainCenter || !win.layoutDirty {
		t.Fatalf("SetMainAlign did not apply/request layout: align=%v dirty=%v", box.MainAlign(), win.layoutDirty)
	}

	win.layoutDirty = false
	box.SetCrossAlign(layout.CrossStretch)
	if box.CrossAlign() != layout.CrossStretch || !win.layoutDirty {
		t.Fatalf("SetCrossAlign did not apply/request layout: align=%v dirty=%v", box.CrossAlign(), win.layoutDirty)
	}
}

func TestLinearBoxCrossStretchFillsChildren(t *testing.T) {
	box := NewLinearBox(layout.DirectionVertical)
	box.SetCrossAlign(layout.CrossStretch)
	child := newSizedWidget(geometry.Size{Width: 10, Height: 20})
	box.AddChild(child)

	box.Arrange(geometry.Rect(0, 0, 80, 40))
	if child.Rect().Width != 80 {
		t.Fatalf("CrossStretch should fill cross width, got %+v", child.Rect())
	}
}

func TestLinearBoxSingleChildCentersInAllocatedRect(t *testing.T) {
	box := NewLinearBox(layout.DirectionHorizontal)
	box.SetMainAlign(layout.MainCenter)
	box.SetCrossAlign(layout.CrossCenter)
	child := newSizedWidget(geometry.Size{Width: 20, Height: 10})
	box.AddChild(child)

	box.Arrange(geometry.Rect(0, 0, 100, 40))
	if got := child.Rect(); got != geometry.Rect(40, 15, 20, 10) {
		t.Fatalf("single-child LinearBox did not center its child: %+v", got)
	}
}

func TestLinearBoxCrossAlignIsContainerPolicy(t *testing.T) {
	parent := NewLinearBox(layout.DirectionHorizontal)
	parent.SetCrossAlign(layout.CrossStretch)
	childBox := NewLinearBox(layout.DirectionVertical)
	childBox.SetMinSize(geometry.Size{Width: 20, Height: 0})
	child := newSizedWidget(geometry.Size{Width: 4, Height: 12})
	childBox.AddChild(child)
	parent.AddChild(childBox)

	parent.Arrange(geometry.Rect(0, 0, 100, 40))

	if got := childBox.Rect(); got != geometry.Rect(0, 0, 20, 40) {
		t.Fatalf("child box CrossStretch should affect its parent placement, got %+v", got)
	}
	if got := child.Rect(); got != geometry.Rect(0, 0, 4, 12) {
		t.Fatalf("child box default CrossStart should hug its child, got %+v", got)
	}

	childBox.SetCrossAlign(layout.CrossStretch)
	childBox.Arrange(geometry.Rect(0, 0, 20, 40))
	if got := child.Rect(); got != geometry.Rect(0, 0, 20, 12) {
		t.Fatalf("child box CrossStretch should arrange its own child, got %+v", got)
	}
}

func TestLinearBoxMainWeightSharesSpace(t *testing.T) {
	box := NewLinearBox(layout.DirectionHorizontal)
	first := newSizedWidget(geometry.Size{Width: 10, Height: 20})
	second := newSizedWidget(geometry.Size{Width: 10, Height: 20})
	second.SetMainWeight(1)
	box.AddChild(first)
	box.AddChild(second)

	box.Arrange(geometry.Rect(0, 0, 100, 40))
	// first hugs (10); second takes all 80 leftover -> 90 wide, starting at 10.
	if first.Rect().Width != 10 {
		t.Fatalf("unweighted child should hug: %+v", first.Rect())
	}
	if second.Rect() != geometry.Rect(10, 10, 90, 20) {
		t.Fatalf("weighted child should only consume main-axis free space: %+v", second.Rect())
	}
}

func TestLinearBoxAlignsAndPropagatesBaseline(t *testing.T) {
	box := NewLinearBox(layout.DirectionHorizontal)
	box.SetCrossAlign(layout.CrossBaseline)
	first := &baselineSizedWidget{measurement: layout.MeasuredWithBaseline(
		geometry.Size{Width: 20, Height: 20}, 15,
	)}
	second := &baselineSizedWidget{measurement: layout.MeasuredWithBaseline(
		geometry.Size{Width: 30, Height: 30}, 20,
	)}
	box.AddChild(first)
	box.AddChild(second)

	measured := box.Measure(layout.Loose(geometry.Size{Width: 100, Height: 100}))
	if !measured.HasBaseline || measured.Baseline != 20 {
		t.Fatalf("linear box did not propagate common baseline: %+v", measured)
	}
	box.Arrange(geometry.Rect(0, 0, 100, 40))
	if first.Rect().Y+first.measurement.Baseline != second.Rect().Y+second.measurement.Baseline {
		t.Fatalf("children are not baseline-aligned: first=%+v second=%+v", first.Rect(), second.Rect())
	}
}

func TestLinearBoxWeightedChildrenSplitFreeSpace(t *testing.T) {
	box := NewLinearBox(layout.DirectionHorizontal)
	box.SetCrossAlign(layout.CrossStretch)
	first := newSizedWidget(geometry.Size{Width: 0, Height: 20})
	second := newSizedWidget(geometry.Size{Width: 0, Height: 20})
	first.SetMainWeight(1)
	second.SetMainWeight(1)
	box.AddChild(first)
	box.AddChild(second)

	box.Arrange(geometry.Rect(0, 0, 100, 40))
	// CrossStretch is the container policy; MainWeight distributes the main axis.
	if first.Rect() != geometry.Rect(0, 0, 50, 40) {
		t.Fatalf("first elastic child should take half: %+v", first.Rect())
	}
	if second.Rect() != geometry.Rect(50, 0, 50, 40) {
		t.Fatalf("second elastic child should take half: %+v", second.Rect())
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
	if info.Role != RoleHBox {
		t.Fatalf("unexpected snapshot role: %q", info.Role)
	}

	box.SetDirection(layout.DirectionVertical)
	info = box.Snapshot()
	if info.Role != RoleVBox {
		t.Fatalf("unexpected vertical snapshot role: %q", info.Role)
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
	return w
}

func (w *sizedWidget) Measure(_ layout.Constraint) layout.Measurement {
	return layout.Measured(w.size)
}

type baselineSizedWidget struct {
	WidgetBase
	measurement layout.Measurement
}

func (w *baselineSizedWidget) Measure(c layout.Constraint) layout.Measurement {
	return w.measurement.Constrain(c)
}

func TestLinearBoxCrossDefaultFollowsDirection(t *testing.T) {
	box := NewLinearBox(layout.DirectionHorizontal)
	child := newSizedWidget(geometry.Size{Width: 20, Height: 10})
	box.AddChild(child)
	win := &window{}
	win.SetWidget(box)
	cases := []struct {
		direction layout.Direction
		align     layout.CrossAlign
		want      geometry.Rectangle
	}{
		{layout.DirectionHorizontal, layout.CrossDefault, geometry.Rect(0, 15, 20, 10)},
		{layout.DirectionVertical, layout.CrossDefault, geometry.Rect(0, 0, 20, 10)},
		{layout.DirectionHorizontal, layout.CrossStart, geometry.Rect(0, 0, 20, 10)},
		{layout.DirectionVertical, layout.CrossStart, geometry.Rect(0, 0, 20, 10)},
		{layout.DirectionHorizontal, layout.CrossDefault, geometry.Rect(0, 15, 20, 10)},
	}
	for _, tc := range cases {
		box.SetDirection(tc.direction)
		box.SetCrossAlign(tc.align)
		box.Arrange(geometry.Rect(0, 0, 100, 40))
		if child.Rect() != tc.want || box.CrossAlign() != tc.align {
			t.Fatalf("direction=%v align=%v: rect=%+v config=%v; want %+v", tc.direction, tc.align, child.Rect(), box.CrossAlign(), tc.want)
		}
	}
}

func TestLinearBoxMinimumSizeIsIncludedInBaseline(t *testing.T) {
	box := NewLinearBox(layout.DirectionHorizontal)
	box.SetMinSize(geometry.Size{Width: 100, Height: 40})
	child := &baselineSizedWidget{measurement: layout.MeasuredWithBaseline(
		geometry.Size{Width: 20, Height: 10}, 7,
	)}
	box.AddChild(child)
	measured := box.Measure(layout.Unbounded())
	if measured.Size != (geometry.Size{Width: 100, Height: 40}) || !measured.HasBaseline || measured.Baseline != 22 {
		t.Fatalf("minimum height was not included in the centered baseline: %+v", measured)
	}
	box.Arrange(geometry.Rectangle{Size: measured.Size})
	if child.Rect().Y+7 != measured.Baseline {
		t.Fatalf("measured baseline=%v, child rect=%+v", measured.Baseline, child.Rect())
	}
}
