package basic

import (
	"testing"

	"github.com/golang-gui/goui/core/geometry"
	"github.com/golang-gui/goui/style"
)

func TestTreeContentPartIsAnEmptyDecoration(t *testing.T) {
	for _, state := range []style.State{style.Normal, style.Hovered, style.Pressed, style.Selected, style.Focused | style.FocusVisible} {
		s := Sheet().Resolve(style.Sel{Name: "tree-item", Part: "content", State: state})
		bg, _ := s.BackgroundColor()
		_, _, _, alpha := bg.RGBA()
		width, _ := s.BorderWidth()
		radius, _ := s.CornerRadius()
		if alpha != 0 || width != 0 || radius != (geometry.CornerRadius{}) {
			t.Fatalf("content默认装饰重复铺底或继承边框／圆角：state=%v", state)
		}
	}
}
