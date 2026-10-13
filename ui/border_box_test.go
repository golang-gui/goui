package ui

import (
	"github.com/golang-gui/goui/gui"
	"testing"
)

func TestBorderBoxReconciliation(t *testing.T) {
	root := newRoot()
	t.Cleanup(root.unmountWindow)
	b := root.update(BorderBox(Label("first"))).(*gui.BorderBox)
	label := b.Child().(*gui.Label)
	if !b.Resizable() || b.ResizeWidth() != 6 {
		t.Fatal("defaults overwritten")
	}
	updated := root.update(BorderBox(nil).Child(Label("second")).Resizable(false).ResizeWidth(10).Name("frame"))
	if updated != b || b.Child() != label || label.Text() != "second" || b.Resizable() || b.ResizeWidth() != 10 || b.Name() != "frame" {
		t.Fatal("declarative child/options were not reconciled")
	}
	root.update(BorderBox(Button("replacement")))
	if _, ok := b.Child().(*gui.Button); !ok || label.Parent() != nil || !b.Resizable() || b.ResizeWidth() != 6 {
		t.Fatal("replacement or restoring defaults failed")
	}
	root.update(BorderBox(nil))
	if b.Child() != nil || len(b.Children()) != 0 {
		t.Fatal("empty slot retained child")
	}
}
