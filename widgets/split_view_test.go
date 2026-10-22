package widgets

import (
	"image/color"
	"math"
	"testing"

	"github.com/golang-gui/goui/core/geometry"
	"github.com/golang-gui/goui/gui"
	"github.com/golang-gui/goui/layout"
	"github.com/golang-gui/goui/platform/events"
	"github.com/golang-gui/goui/platform/graphics"
	"github.com/golang-gui/goui/style"
	"github.com/golang-gui/goui/theme/modern"
	widgetstyle "github.com/golang-gui/goui/widgets/style"
)

// 确定性逻辑与真实事件派发路径，不创建原生窗口、不依赖字体。
type splitTestLayout struct {
	desired  geometry.Size
	measures int
}

func (l *splitTestLayout) Measure(_ []layout.Child, c layout.Constraint) layout.Measurement {
	l.measures++
	return layout.Measured(c.Clamp(l.desired))
}
func (*splitTestLayout) Arrange([]layout.Child, geometry.Rectangle) {}
func splitTestChild(w, h float32) *gui.WidgetBase {
	child := new(gui.WidgetBase)
	child.SetLayoutManager(&splitTestLayout{desired: geometry.Size{Width: w, Height: h}})
	return child
}
func splitFixture(direction layout.Direction) *SplitView {
	v := NewSplitView(direction)
	v.SetStartChild(splitTestChild(240, 120))
	v.SetEndChild(splitTestChild(560, 280))
	return v
}
func arrangeSplit(v *SplitView, main, cross float32) {
	size := v.size(main, cross)
	v.Measure(layout.Tight(size))
	v.Arrange(geometry.Rectangle{Size: size})
}
func checkSplit(t *testing.T, v *SplitView, start, end float32) {
	t.Helper()
	a, b := v.Sizes()
	if math.Abs(float64(a-start)) > .001 || math.Abs(float64(b-end)) > .001 {
		t.Fatalf("sizes=(%g,%g), want (%g,%g)", a, b, start, end)
	}
}

func TestSplitViewContentRatioAndMeasurement(t *testing.T) {
	for _, direction := range []layout.Direction{layout.DirectionHorizontal, layout.DirectionVertical} {
		v := splitFixture(direction)
		if v.MainWeight() != 1 {
			t.Fatal("split must share parent's remaining space")
		}
		want := v.size(801, 280)
		if direction == layout.DirectionVertical {
			want = v.size(401, 560)
		}
		measured := v.Measure(layout.Unbounded())
		if measured.Size != want || v.preference.mode != splitAuto {
			t.Fatalf("natural measurement=%v, want %v; Measure froze ratio", measured.Size, want)
		}
		checkSplit(t, v, 0, 0)
		arrangeSplit(v, 801, 200)
		checkSplit(t, v, 240, 560)
		// Once initialized, Arrange must not switch back to a loose natural
		// constraint and invalidate unchanged content's final-size cache.
		startLayout := v.StartChild().LayoutManager().(*splitTestLayout)
		endLayout := v.EndChild().LayoutManager().(*splitTestLayout)
		beforeStart, beforeEnd := startLayout.measures, endLayout.measures
		v.Arrange(geometry.Rectangle{Size: v.size(801, 200)})
		if startLayout.measures != beforeStart || endLayout.measures != beforeEnd {
			t.Fatal("unchanged arrangement remeasured natural content")
		}
		child := v.StartChild().(*gui.WidgetBase)
		child.LayoutManager().(*splitTestLayout).desired = v.size(600, 50)
		child.RequestLayout()
		arrangeSplit(v, 1001, 200)
		checkSplit(t, v, 300, 700)
	}
	v := NewSplitView(layout.DirectionHorizontal)
	v.SetStartChild(splitTestChild(0, 0))
	v.SetEndChild(splitTestChild(0, 0))
	arrangeSplit(v, 0, 0)
	if v.preference.mode != splitAuto {
		t.Fatal("empty arrangement froze ratio")
	}
	arrangeSplit(v, 101, 40)
	checkSplit(t, v, 50, 50)
}

func TestSplitViewSizingConstraintsAndRestoration(t *testing.T) {
	v := splitFixture(layout.DirectionHorizontal)
	notifications := 0
	v.ConnectResize(func(float32, float32) { notifications++ })
	v.SetStartSize(240)
	v.EndChild().SetMinSize(geometry.Size{Width: 100})
	arrangeSplit(v, 301, 60)
	checkSplit(t, v, 200, 100)
	arrangeSplit(v, 801, 60)
	checkSplit(t, v, 240, 560)
	v.SetEndSize(150)
	arrangeSplit(v, 801, 60)
	checkSplit(t, v, 650, 150)
	v.SetRatio(.5)
	arrangeSplit(v, 801, 60)
	checkSplit(t, v, 400, 400)
	v.SetRatio(2)
	arrangeSplit(v, 801, 60)
	checkSplit(t, v, 700, 100)
	v.SetRatio(float32(math.NaN()))
	v.SetStartSize(float32(math.Inf(1)))
	arrangeSplit(v, 801, 60)
	checkSplit(t, v, 700, 100)
	v.EndChild().SetMinSize(geometry.Size{})
	v.SetStartSize(-1)
	arrangeSplit(v, 801, 60)
	checkSplit(t, v, 0, 800)
	if notifications != 0 {
		t.Fatal("layout/setters emitted user resize")
	}
	for _, tc := range []struct {
		available float32
		s, e      splitLimit
		lo, hi    float32
	}{
		{800, splitLimit{100, 300}, splitLimit{200, 600}, 200, 300},
		{300, splitLimit{200, layout.Inf}, splitLimit{400, layout.Inf}, 100, 100},
		{800, splitLimit{100, 200}, splitLimit{100, 200}, 100, 700},
		{0, splitLimit{0, layout.Inf}, splitLimit{0, layout.Inf}, 0, 0},
	} {
		lo, hi := splitBounds(tc.available, tc.s, tc.e)
		if lo != tc.lo || hi != tc.hi {
			t.Fatalf("bounds %v: %g..%g", tc, lo, hi)
		}
	}
}

func TestSplitViewHiddenReplacementAndSnapshot(t *testing.T) {
	v := splitFixture(layout.DirectionHorizontal)
	v.SetRatio(.25)
	arrangeSplit(v, 801, 200)
	v.Arrange(geometry.Rect(10, 20, 801, 200))
	info := v.Snapshot()
	if info.Role != RoleSplitView || len(info.Children) != 3 {
		t.Fatalf("semantic subtree=%+v", info)
	}
	handle := info.Children[2]
	if handle.Role != RoleSeparator || handle.Bounds != geometry.Rect(206, 20, 9, 200) || handle.Range.Value != 200 || handle.Range.Max != 800 || len(handle.Actions) != 1 {
		t.Fatalf("separator=%+v", handle)
	}
	if gui.Pick(v, geometry.Point{X: 197, Y: 50}) != v.handle {
		t.Fatal("wide separator did not win picking")
	}
	if gui.Pick(v, geometry.Point{X: 190, Y: 50}) == v.handle {
		t.Fatal("separator stole pane input")
	}
	end := v.EndChild()
	end.SetVisible(false)
	arrangeSplit(v, 801, 200)
	checkSplit(t, v, 801, 0)
	if v.handle.Visible() || len(v.Snapshot().Children) != 2 {
		t.Fatal("hidden pane left active separator")
	}
	end.SetVisible(true)
	arrangeSplit(v, 801, 200)
	checkSplit(t, v, 200, 600)
	start := v.StartChild()
	v.SetEndChild(start)
	if v.EndChild() != end {
		t.Fatal("same child occupied both slots")
	}
	v.SetStartChild(v)
	if v.StartChild() != start {
		t.Fatal("cyclic parent accepted")
	}
	v.SetStartChild(nil)
	if start.Parent() != nil {
		t.Fatal("replaced child remained attached")
	}
	arrangeSplit(v, 801, 200)
	checkSplit(t, v, 0, 801)
	v.SetStartChild(start)
	arrangeSplit(v, 801, 200)
	checkSplit(t, v, 200, 600)
	nested := splitFixture(layout.DirectionVertical)
	v.SetEndChild(nested)
	arrangeSplit(v, 801, 401)
	checkSplit(t, nested, 120, 280)
	if nested.handle.Cursor() != gui.CursorResizeVertical {
		t.Fatal("wrong vertical cursor")
	}
}

func TestSplitViewDragRearrangeKeyboardAndCancel(t *testing.T) {
	for _, direction := range []layout.Direction{layout.DirectionHorizontal, layout.DirectionVertical} {
		v := splitFixture(direction)
		arrangeSplit(v, 801, 200)
		host := &tabInputHost{root: v}
		dispatcher := new(gui.EventDispatcher)
		point := func(kind events.EventType, main float32) {
			x, y := main, float32(50)
			if direction == layout.DirectionVertical {
				x, y = y, x
			}
			dispatchTabPointer(t, dispatcher, host, kind, x, y)
		}
		notifications := 0
		v.ConnectResize(func(float32, float32) { notifications++ })
		point(events.PointerDown, 242)
		if host.focus != v.handle || !v.handle.dragging {
			t.Fatal("handle did not focus/capture")
		}
		point(events.PointerMove, 292)
		checkSplit(t, v, 290, 510)
		arrangeSplit(v, 801, 200)
		point(events.PointerMove, 312)
		checkSplit(t, v, 310, 490)
		arrangeSplit(v, 801, 200)
		point(events.PointerUp, 312)
		if v.handle.dragging || notifications != 2 {
			t.Fatalf("drag state/count=%v/%d", v.handle.dragging, notifications)
		}
		key := func(key gui.Key, mods gui.KeyModifiers) {
			k, _ := key.Resolve()
			m, _ := mods.Resolve()
			handled := false
			if err := dispatcher.DispatchEvent(host, events.KeyEvent{EventType: events.KeyDown, Key: k, Modifiers: m, Handled: &handled}); err != nil {
				t.Fatal(err)
			}
			if handled == (mods == gui.ModControl) {
				t.Fatal("consumed keyboard adjustment did not suppress native default, or unrelated key was consumed")
			}
		}
		increase := gui.KeyArrowRight
		if direction == layout.DirectionVertical {
			increase = gui.KeyArrowDown
		}
		key(increase, 0)
		checkSplit(t, v, 311, 489)
		key(increase, gui.ModShift)
		checkSplit(t, v, 321, 479)
		key(increase, gui.ModControl)
		checkSplit(t, v, 321, 479)
		key(gui.KeyHome, 0)
		checkSplit(t, v, 0, 800)
		key(gui.KeyEnd, 0)
		checkSplit(t, v, 800, 0)
		v.SetStartSize(240)
		arrangeSplit(v, 801, 200)
		point(events.PointerDown, 240)
		point(events.PointerMove, 340)
		arrangeSplit(v, 801, 200)
		key(gui.KeyEscape, 0)
		checkSplit(t, v, 240, 560)
		if v.preference != (splitPreference{mode: splitStart, value: 240}) || v.handle.dragging {
			t.Fatal("Esc failed to restore preferred policy")
		}
		point(events.PointerUp, 340)
		checkSplit(t, v, 240, 560)
		arrangeSplit(v, 1001, 200)
		checkSplit(t, v, 240, 760)
	}
}

func TestSplitViewCancellationDirectionAndChildChange(t *testing.T) {
	v := splitFixture(layout.DirectionHorizontal)
	v.SetEndSize(560)
	arrangeSplit(v, 801, 200)
	host := &tabInputHost{root: v}
	d := new(gui.EventDispatcher)
	dispatchTabPointer(t, d, host, events.PointerDown, 240, 50)
	dispatchTabPointer(t, d, host, events.PointerMove, 340, 50)
	v.SetEndSize(460)
	if !v.handle.dragging {
		t.Fatal("identical preference canceled drag")
	}
	v.SetDirection(layout.DirectionVertical)
	if v.handle.dragging || v.preference.value != 560 {
		t.Fatal("direction change did not cancel")
	}
	arrangeSplit(v, 801, 200)
	dispatchTabPointer(t, d, host, events.PointerDown, 50, 240)
	dispatchTabPointer(t, d, host, events.PointerMove, 50, 340)
	v.SetStartChild(nil)
	if v.handle.dragging || v.preference.value != 560 {
		t.Fatal("child change did not restore preference")
	}
}

func TestSplitViewLifecycleInvalidation(t *testing.T) {
	v := splitFixture(layout.DirectionHorizontal)
	owner := gui.NewPopover(nil, nil)
	owner.SetWidget(v)
	arrangeSplit(v, 801, 200)
	host := &tabInputHost{root: v}
	d := new(gui.EventDispatcher)
	dispatchTabPointer(t, d, host, events.PointerDown, 240, 50)
	dispatchTabPointer(t, d, host, events.PointerMove, 340, 50)
	owner.SetWidget(nil)
	checkSplit(t, v, 240, 560)
	if v.handle.dragging || v.Root() != nil {
		t.Fatal("unmount failed to cancel drag")
	}
	// Complete the physical button sequence after cancellation before starting
	// a new drag; the dispatcher must not receive two downs without an up.
	dispatchTabPointer(t, d, host, events.PointerUp, 340, 50)
	owner.SetWidget(v)
	arrangeSplit(v, 801, 200)
	calls := 0
	v.ConnectResize(func(float32, float32) { owner.Destroy() })
	v.ConnectResize(func(float32, float32) { calls++ })
	dispatchTabPointer(t, d, host, events.PointerDown, 240, 50)
	dispatchTabPointer(t, d, host, events.PointerMove, 340, 50)
	// Destroying a lazy Popover detaches its tree; final Widget destruction is
	// the owning Window's responsibility. Either invalidation stops this emit.
	if v.Root() != nil || calls != 0 {
		t.Fatalf("resize host destruction: root=%v later callbacks=%d", v.Root(), calls)
	}
}

type splitStyleApp struct {
	gui.Application
	sheet style.StyleSheet
}

func (a *splitStyleApp) StyleSheet() style.StyleSheet { return a.sheet }

type splitPaintRecorder struct {
	gui.Painter
	rect  geometry.Rectangle
	brush graphics.Brush
}

func (p *splitPaintRecorder) FillRect(r geometry.Rectangle, b graphics.Brush) { p.rect, p.brush = r, b }
func TestSplitViewHandleIndependentStyleAndGeometry(t *testing.T) {
	old := gui.App
	defer func() { gui.App = old }()
	for _, dark := range []bool{false, true} {
		app := &splitStyleApp{sheet: modern.Sheet(modern.Options{Dark: dark})}
		gui.App = app
		v := splitFixture(layout.DirectionHorizontal)
		v.SetStyleName("unrelated-parent")
		arrangeSplit(v, 801, 100)
		p := new(splitPaintRecorder)
		v.handle.Paint(p)
		ink, _ := app.sheet.Resolve(style.Sel{Name: "split-handle"}).ForegroundColor()
		if p.rect != geometry.Rect(4, 0, 1, 100) || p.brush != graphics.ColorOf(ink) {
			t.Fatalf("normal separator paint=%v %v", p.rect, p.brush)
		}
		v.handle.hovered = true
		v.handle.Paint(p)
		active, _ := app.sheet.Resolve(style.Sel{Name: "split-handle", State: style.Hovered}).ForegroundColor()
		if p.rect != geometry.Rect(4, 0, 1, 100) || p.brush != graphics.ColorOf(active) || !colorEqual(ink, active) {
			t.Fatal("default hover changed the neutral 1 DIP separator")
		}
		v.SetRatio(0)
		arrangeSplit(v, 801, 100)
		v.handle.hovered = false
		v.handle.Paint(p)
		if v.handle.Rect().Width != 5 || p.rect.X != 0 {
			t.Fatal("edge hit clipping moved visible gap center")
		}
	}
}

// Drag release and pointer exit must remove pointer feedback without removing
// keyboard focus. Exercise the standard dispatcher, not private state setters.
func TestSplitViewHandlePointerFeedbackAndFocus(t *testing.T) {
	old := gui.App
	defer func() { gui.App = old }()
	for _, theme := range []struct {
		name  string
		rules []style.Rule
	}{
		{"modern-light", modern.Rules(modern.Options{})},
		{"modern-dark", modern.Rules(modern.Options{Dark: true})},
		{"fallback", append(gui.DefaultStyleRules(), widgetstyle.Rules()...)},
	} {
		for _, direction := range []layout.Direction{layout.DirectionHorizontal, layout.DirectionVertical} {
			for _, custom := range []bool{false, true} {
				name := theme.name + "/horizontal"
				if direction == layout.DirectionVertical {
					name = theme.name + "/vertical"
				}
				if custom {
					name += "/custom-highlight"
				}
				t.Run(name, func(t *testing.T) {
					rules := append([]style.Rule(nil), theme.rules...)
					hover, pressed := color.RGBA{R: 180, A: 255}, color.RGBA{G: 180, A: 255}
					if custom {
						rules = append(rules,
							style.Name("split-handle").State(style.Hovered).ForegroundColor(hover),
							style.Name("split-handle").State(style.Pressed).ForegroundColor(pressed),
							// Focus is deliberately not a paint state for the separator.
							style.Name("split-handle").State(style.Focused).ForegroundColor(color.Black))
					}
					app := &splitStyleApp{sheet: style.Sheet(rules...)}
					gui.App = app
					v := splitFixture(direction)
					owner := gui.NewPopover(nil, nil) // Lazy root: no native window.
					defer owner.Destroy()
					owner.SetWidget(v)
					dispatcher := new(gui.EventDispatcher)
					host := owner.(gui.EventTarget)
					arrangeSplit(v, 801, 100)
					ink, _ := app.sheet.Resolve(style.Sel{Name: "split-handle"}).ForegroundColor()
					if !custom {
						hover, pressed = color.RGBAModel.Convert(ink).(color.RGBA), color.RGBAModel.Convert(ink).(color.RGBA)
					}
					paint := func(want color.Color) {
						t.Helper()
						p := new(splitPaintRecorder)
						v.handle.Paint(p)
						wantRect := geometry.Rect(4, 0, 1, 100)
						if direction == layout.DirectionVertical {
							wantRect = geometry.Rect(0, 4, 100, 1)
						}
						if p.rect != wantRect || p.brush != graphics.ColorOf(want) {
							t.Fatalf("separator paint=%v %v, want 1 DIP color=%v", p.rect, p.brush, want)
						}
					}
					point := func(kind events.EventType, main float32, down bool) {
						t.Helper()
						buttons := events.PointerButtons(0)
						if down {
							buttons = events.PointerButtonLeftDown
						}
						position := geometry.Point{X: main, Y: 50}
						if direction == layout.DirectionVertical {
							position = geometry.Point{X: 50, Y: main}
						}
						if err := dispatcher.DispatchEvent(host, events.PointerEvent{EventType: kind, Position: position,
							Button: events.PointerButtonLeft, Buttons: buttons}); err != nil {
							t.Fatal(err)
						}
					}
					paint(ink)
					point(events.PointerMove, 240, false)
					if !v.handle.hovered {
						t.Fatal("pointer entry did not set hover")
					}
					paint(hover)
					point(events.PointerDown, 240, true)
					if err := dispatcher.DispatchEvent(host, events.FocusEvent{Focused: true}); err != nil {
						t.Fatal(err)
					}
					point(events.PointerMove, 280, true)
					arrangeSplit(v, 801, 100)
					if !v.handle.dragging || !v.handle.Focused() {
						t.Fatal("drag did not retain keyboard focus")
					}
					paint(pressed)
					point(events.PointerUp, 280, false)
					paint(hover)
					point(events.PointerMove, 600, false)
					if v.handle.hovered || v.handle.dragging || !v.handle.Focused() || !v.handle.Snapshot().Focused {
						t.Fatal("pointer exit did not clear feedback while retaining semantic focus")
					}
					paint(ink)
					if v.axis(v.handle.Rect().Size) != 9 {
						t.Fatal("1 DIP separator changed the 9 DIP hit area")
					}
					key := events.KeyArrowRight
					if direction == layout.DirectionVertical {
						key = events.KeyArrowDown
					}
					if err := dispatcher.DispatchEvent(host, events.KeyEvent{EventType: events.KeyDown, Key: key}); err != nil {
						t.Fatal(err)
					}
					checkSplit(t, v, 281, 519)
				})
			}
		}
	}
}
func colorEqual(a, b color.Color) bool {
	ar, ag, ab, aa := a.RGBA()
	br, bg, bb, ba := b.RGBA()
	return ar == br && ag == bg && ab == bb && aa == ba
}
