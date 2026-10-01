package widgets

import (
	"image/color"
	"testing"
	"time"

	"github.com/golang-gui/goui/core/geometry"
	"github.com/golang-gui/goui/gui"
	"github.com/golang-gui/goui/platform/events"
	"github.com/golang-gui/goui/platform/graphics"
	"github.com/golang-gui/goui/platform/typography"
	"github.com/golang-gui/goui/style"
	widgetstyle "github.com/golang-gui/goui/widgets/style"
)

type separatorPainter struct {
	gui.Painter
	rects []geometry.Rectangle
}

func (p *separatorPainter) FillRect(rect geometry.Rectangle, _ graphics.Brush) {
	p.rects = append(p.rects, rect)
}

type separatorApp struct {
	gui.Application
	sheet style.StyleSheet
}

func (*separatorApp) Typography() typography.Context { return nil }
func (a *separatorApp) StyleSheet() style.StyleSheet { return a.sheet }

func TestTabSeparatorStatesAndGeometry(t *testing.T) {
	old := gui.App
	app := &separatorApp{sheet: style.Sheet(append(gui.DefaultStyleRules(), widgetstyle.Rules()...)...)}
	gui.App = app
	defer func() { gui.App = old }()
	bar, view, pages := dragTestBar()
	check := func(want ...geometry.Rectangle) {
		t.Helper()
		p := new(separatorPainter)
		bar.viewport.Paint(p)
		if len(p.rects) != len(want) {
			t.Fatalf("separators=%v want=%v", p.rects, want)
		}
		for i, rect := range want {
			if p.rects[i] != rect {
				t.Fatalf("separator=%v want=%v", p.rects[i], rect)
			}
		}
	}
	check(geometry.Rect(205.5, 12, 1, 16)) // only B|C, never A|B
	if target := gui.Pick(bar, geometry.Point{X: 206, Y: 20}); target != bar.viewport {
		t.Fatal("separator changed picking")
	}
	if len(bar.Snapshot().Children) != 3 {
		t.Fatal("separator became a semantic child")
	}
	d := new(gui.EventDispatcher)
	host := &tabInputHost{root: bar}
	for _, x := range []float32{130, 230} {
		dispatchTabPointer(t, d, host, events.PointerMove, x, 20)
		check() // hovering either neighbor hides their separator
	}
	dispatchTabPointer(t, d, host, events.PointerMove, 350, 20)
	check(geometry.Rect(205.5, 12, 1, 16))
	view.SetCurrent(pages[1])
	check()
	view.SetCurrent(pages[2])
	check(geometry.Rect(101.5, 12, 1, 16))
	bar.beginDrag(pages[0], geometry.Point{X: 20, Y: 20})
	bar.updateDrag(pages[0], geometry.Point{X: 140, Y: 20})
	check()
	bar.cancelDrag()
	check() // also hidden during return animation
	bar.advanceMotion(100 * time.Millisecond)
	bar.advanceMotion(100 * time.Millisecond)
	check(geometry.Rect(101.5, 12, 1, 16))
	view.SetCurrent(pages[0])
	bar.Arrange(geometry.Rect(0, 0, 300, 40))
	bar.setScroll(56)
	bar.Arrange(bar.Rect())
	check(geometry.Rect(149.5, 12, 1, 16))
	bar.Arrange(geometry.Rect(0, 0, 300, 10))
	check(geometry.Rect(149.5, 0, 1, 10)) // short hosts cap the line height
	app.sheet = style.Sheet(style.Name("tab-bar").Part("separator").ForegroundColor(color.Transparent))
	check() // transparent style disables decoration without changing geometry
}
