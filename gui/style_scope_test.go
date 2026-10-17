package gui

import (
	"testing"

	"github.com/golang-gui/goui/core/geometry"
	"github.com/golang-gui/goui/layout"
	"github.com/golang-gui/goui/style"
)

func TestStyleScopeIsolationNestingAndReparent(t *testing.T) {
	setTestApplication(t, nil)
	App.SetStyleSheet(style.Sheet(style.Name("text-input").FontSize(10)))
	a, b, nested := NewStyleScope(), NewStyleScope(), NewStyleScope()
	sheet := func(n float32) style.StyleSheet { return style.Sheet(style.Name("text-input").FontSize(n)) }
	a.SetStyleSheet(sheet(11))
	b.SetStyleSheet(sheet(12))
	nested.SetStyleSheet(sheet(13))
	input := NewTextInput()
	nested.SetChild(input)
	a.SetChild(nested)
	want := func(n float32) {
		t.Helper()
		got, _ := input.resolvedStyle("").FontSize()
		if got != n {
			t.Fatalf("font=%v want %v", got, n)
		}
	}
	want(13)
	nested.SetStyleSheet(nil)
	want(11)
	b.SetChild(nested)
	a.SetChild(nil)
	if a.child != nil {
		t.Fatal("detached slot retained old child")
	}
	want(12)
	a.SetStyleSheet(sheet(14))
	want(12)
	b.SetStyleSheet(nil)
	want(10)
}

type scopeProbe struct {
	WidgetBase
	notifications int
	size          float32
}

func (p *scopeProbe) StyleChanged() {
	p.notifications++
	p.size, _ = ResolveWidgetStyle(p, "probe", "", style.Normal).FontSize()
}
func (p *scopeProbe) Measure(c layout.Constraint) layout.Measurement {
	return layout.Measured(c.Clamp(geometry.Size{Width: p.size, Height: p.size}))
}
func TestStyleScopeInvalidatesThroughFramework(t *testing.T) {
	s := NewStyleScope()
	p := new(scopeProbe)
	s.SetChild(p)
	s.SetStyleSheet(style.Sheet(style.Name("probe").FontSize(11)))
	measureWidget(s, layout.Unbounded())
	s.SetStyleSheet(style.Sheet(style.Name("probe").FontSize(12)))
	s.SetStyleSheet(style.Sheet(style.Name("probe").FontSize(13)))
	if p.notifications != 1 {
		t.Fatal("setter called user code")
	}
	if got := measureWidget(s, layout.Unbounded()); got.Width != 13 || p.notifications != 2 {
		t.Fatalf("stale/coalescing: %+v %d", got, p.notifications)
	}
	s.SetChild(nil)
	s.SetStyleSheet(style.Sheet(style.Name("probe").FontSize(14)))
	if p.notifications != 2 {
		t.Fatal("detached child notified")
	}
	s.SetChild(p)
	if got := measureWidget(s, layout.Unbounded()); got.Width != 14 {
		t.Fatal("reattach stale", got)
	}
	s.destroy(s)
	s.SetStyleSheet(nil)
}

func TestStyleScopeRejectsCycles(t *testing.T) {
	a, b := NewStyleScope(), NewStyleScope()
	a.SetChild(b)
	b.SetChild(a)
	b.SetChild(b)
	if b.Child() != nil || a.Child() != b || a.Parent() != nil {
		t.Fatal("cycle changed tree")
	}
}
