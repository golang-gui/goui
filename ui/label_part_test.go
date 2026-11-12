package ui

import (
	"github.com/golang-gui/goui/gui"
	"testing"
)

func TestLabelStylePartReconciliation(t *testing.T) {
	r := newRoot()
	t.Cleanup(r.unmountHost)
	l := r.update(Label("name").Style("orders").StylePart("header-text")).(*gui.Label)
	if l.StyleName() != "orders" || l.StylePart() != "header-text" {
		t.Fatal("声明式Label未传递Name/Part")
	}
	if next := r.update(Label("other").Style("orders")); next != l || l.StylePart() != "" || l.Text() != "other" {
		t.Fatal("省略Part未在原Label上恢复默认")
	}
}
