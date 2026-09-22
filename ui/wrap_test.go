package ui

import (
	"github.com/golang-gui/goui/core/geometry"
	"github.com/golang-gui/goui/gui"
	"github.com/golang-gui/goui/layout"
	"math"
	"testing"
)

func TestWrapUpdatesAndRestoresDefaults(t *testing.T) {
	root := newRoot()
	t.Cleanup(root.unmountWindow)
	box := root.update(HWrap(Label("first"), nil).Spacing(8).LineSpacing(6).Padding(12).MainAlign(layout.MainEnd).CrossAlign(layout.CrossBaseline)).(*gui.WrapBox)
	if box.Spacing() != 8 || box.LineSpacing() != 6 || box.Padding() != 12 || box.MainAlign() != layout.MainEnd || box.CrossAlign() != layout.CrossBaseline || len(box.Children()) != 1 {
		t.Fatal("modifiers not applied")
	}
	child := box.Children()[0]
	updated := root.update(VWrap(Label("second")))
	if updated != box || box.Children()[0] != child || child.(*gui.Label).Text() != "second" {
		t.Fatal("widget identity not preserved")
	}
	if box.Direction() != layout.DirectionVertical || box.Spacing() != 0 || box.LineSpacing() != 0 || box.Padding() != 0 || box.MainAlign() != layout.MainStart || box.CrossAlign() != layout.CrossDefault {
		t.Fatal("defaults not restored")
	}
	root.update(HWrap().Children(Label("third"), Label("fourth")))
	if len(box.Children()) != 2 || box.Children()[0] != child {
		t.Fatal("Children did not reconcile")
	}
	root.update(HWrap())
	if len(box.Children()) != 0 || child.Parent() != nil {
		t.Fatal("children not removed")
	}
}

func TestWrapLayoutAndSnapshot(t *testing.T) {
	root := newRoot()
	t.Cleanup(root.unmountWindow)
	box := root.update(HWrap(VBox().MinSize(40, 10), VBox().MinSize(40, 20), VBox().MinSize(40, 10)).Spacing(5).LineSpacing(3)).(*gui.WrapBox)
	box.Arrange(geometry.Rect(5, 6, 90, 100))
	info := box.Snapshot()
	if info.Role != gui.RoleWrapBox || len(info.Children) != 3 || info.Children[2].Bounds != geometry.Rect(5, 29, 40, 10) {
		t.Fatalf("%+v", info)
	}
	root.update(VWrap().Spacing(-1).LineSpacing(float32(math.NaN())).Padding(float32(math.Inf(1))))
	if box.Spacing() != 0 || box.LineSpacing() != 0 || box.Padding() != 0 {
		t.Fatal("invalid geometry not normalized")
	}
}
