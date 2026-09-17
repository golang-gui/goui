package ui

import (
	"github.com/golang-gui/goui/core/geometry"
	"github.com/golang-gui/goui/gui"
	"testing"
)

func TestIconReconcile(t *testing.T) {
	value := 0
	draw := func(v int) IconDrawFunc { return func(Painter, geometry.Rectangle, Color) { value = v } }
	root := newRoot()
	icon := root.update(Icon(draw(1)).Size(24).Style("custom")).(*gui.Icon)
	icon.DrawFunc()(nil, geometry.Rectangle{}, Color{})
	if value != 1 || icon.Size() != 24 || icon.StyleName() != "custom" {
		t.Fatal("initial icon declaration lost")
	}
	updated := root.update(Icon(nil).DrawFunc(draw(2)).Size(32).Style("next"))
	icon.DrawFunc()(nil, geometry.Rectangle{}, Color{})
	if updated != icon || value != 2 || icon.Size() != 32 || icon.StyleName() != "next" {
		t.Fatal("icon/closure not reconciled in place")
	}
	updated = root.update(Icon(draw(3)))
	icon.DrawFunc()(nil, geometry.Rectangle{}, Color{})
	if updated != icon || value != 3 || icon.Size() != 16 || icon.StyleName() != "" {
		t.Fatal("removed modifiers did not restore defaults")
	}
	root.update(Icon(nil))
	if icon.DrawFunc() != nil {
		t.Fatal("removed content retained")
	}
}
