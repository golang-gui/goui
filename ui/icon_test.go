package ui

import (
	"image"
	"testing"

	"github.com/golang-gui/goui/gui"
)

func TestIconReconcile(t *testing.T) {
	value := 0
	draw := func(v int) uiTestImage {
		return func(int, int, Color) image.Image { value = v; return nil }
	}
	root := newRoot()
	icon := root.update(Icon(draw(1)).Size(24).Style("custom")).(*gui.Icon)
	icon.Source().Image(1, 1, Color{})
	if value != 1 || icon.Size() != 24 || icon.StyleName() != "custom" {
		t.Fatal("initial icon declaration lost")
	}
	updated := root.update(Icon(nil).Source(draw(2)).Size(32).Style("next"))
	icon.Source().Image(1, 1, Color{})
	if updated != icon || value != 2 || icon.Size() != 32 || icon.StyleName() != "next" {
		t.Fatal("icon/closure not reconciled in place")
	}
	updated = root.update(Icon(draw(3)))
	icon.Source().Image(1, 1, Color{})
	if updated != icon || value != 3 || icon.Size() != 16 || icon.StyleName() != "" {
		t.Fatal("removed modifiers did not restore defaults")
	}
	root.update(Icon(nil))
	if icon.Source() != nil {
		t.Fatal("removed content retained")
	}
}

func TestPreparedIconSourceReconcile(t *testing.T) {
	fn := uiTestImage(func(w, h int, _ Color) image.Image { return image.NewRGBA(image.Rect(0, 0, w, h)) })
	source := &fn
	root := newRoot()
	icon := root.update(Icon(source)).(*gui.Icon)
	if root.update(Icon(source)) != icon || icon.Source() != source {
		t.Fatal("prepared source/widget was not retained")
	}
	// Reconcile the declaration even when the GUI widget was changed directly.
	// Comparing with the previous View's source would incorrectly skip this.
	icon.SetSource(nil)
	root.update(Icon(source))
	if icon.Source() != source {
		t.Fatal("declaration was not reapplied")
	}
	root.update(Icon(nil))
	if icon.Source() != nil {
		t.Fatal("removed source retained")
	}
}

type uiTestImage func(int, int, Color) image.Image

func (f uiTestImage) Image(w, h int, c Color) image.Image { return f(w, h, c) }

type nestedIconSource struct{ value any }

func (nestedIconSource) Image(w, h int, c Color) image.Image {
	return image.NewRGBA(image.Rect(0, 0, w, h))
}

func TestIconSourceSafeReconcile(t *testing.T) {
	root := newRoot()
	fn := uiTestImage(func(w, h int, c Color) image.Image { return nil })
	s := nestedIconSource{value: []int{1}}
	i := root.update(Icon(s)).(*gui.Icon)
	if root.update(Icon(s)) != i {
		t.Fatal("widget replaced")
	}
	root.update(Icon(fn))
	root.update(Icon(fn))
	root.update(Icon(nil))
	if i.Source() != nil {
		t.Fatal("nil source not cleared")
	}
}
