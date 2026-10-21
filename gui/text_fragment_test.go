package gui

import (
	"strings"
	"testing"

	"github.com/golang-gui/goui/core/geometry"
	"github.com/golang-gui/goui/gui/textedit"
)

func TestTextViewOwnsMeasuredFragmentAcrossNativeResize(t *testing.T) {
	v, _, _ := newEditorFixture(t, strings.Repeat("abcdefghijklmnopqrst", 200))
	p := v.paragraph(0)
	p.editGeometry(v.lineHeight)
	f := p.fragment
	if f.LineCount() < 100 {
		t.Fatal("fixture did not wrap across many pages", f.LineCount())
	}
	before := f.Geometry(v.lineHeight)
	pos := textedit.Position{Offset: 201, Upstream: false}
	want := before.Caret(pos)
	// Native objects can be resized independently; retained values must not
	// silently change into the new width's geometry through borrowed slices.
	p.layout.SetSize(49, 1<<20)
	p.layout.MeasureMetrics()
	after := f.Geometry(v.lineHeight)
	if got := after.Caret(pos); got != want || got.Width != 1 {
		t.Fatal("snapshot changed with native layout", got, want)
	}
	view, err := f.LineRange(32, 34)
	if err != nil {
		t.Fatal(err)
	}
	g := view.Geometry(v.lineHeight)
	if g.Hit(geometry.Point{X: 10, Y: 1}).Offset < 0 || view.Range().End <= view.Range().Start {
		t.Fatal("invalid retained line fragment")
	}
}
