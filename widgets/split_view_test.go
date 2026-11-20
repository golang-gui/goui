package widgets

import (
	"fmt"
	"image/color"
	"math"
	"testing"

	"github.com/golang-gui/goui/core/geometry"
	"github.com/golang-gui/goui/gui"
	"github.com/golang-gui/goui/layout"
	"github.com/golang-gui/goui/platform/events"
	"github.com/golang-gui/goui/platform/graphics"
	"github.com/golang-gui/goui/platform/graphics/software"
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
	// 旧有分配公式使用显式1 DIP分隔，保留原数值断言；默认5 DIP另行独立覆盖。
	v := NewSplitView(direction)
	v.SetHandleSize(1)
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
	if handle.Role != RoleSeparator || handle.Bounds != geometry.Rect(210, 20, 1, 200) || handle.Range.Value != 200 || handle.Range.Max != 800 || len(handle.Actions) != 1 {
		t.Fatalf("separator=%+v", handle)
	}
	// 此用例显式设置1 DIP抓取宽度，恰好等于线宽。
	if gui.Pick(v, geometry.Point{X: 200, Y: 50}) != v.handle {
		t.Fatal("实际分隔线不能命中")
	}
	if gui.Pick(v, geometry.Point{X: 197, Y: 50}) == v.handle {
		t.Fatal("扩展区覆盖了子控件")
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
			// 使用线的中心，避免比例计算的 float32 边界误差。
			x, y := main+.5, float32(50)
			if direction == layout.DirectionVertical {
				x, y = y, x
			}
			dispatchTabPointer(t, dispatcher, host, kind, x, y)
		}
		notifications := 0
		v.ConnectResize(func(float32, float32) { notifications++ })
		point(events.PointerDown, 240)
		if host.focus != v.handle || !v.handle.dragging {
			t.Fatal("handle did not focus/capture")
		}
		point(events.PointerMove, 292)
		checkSplit(t, v, 292, 508)
		arrangeSplit(v, 801, 200)
		point(events.PointerMove, 312)
		checkSplit(t, v, 312, 488)
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
		checkSplit(t, v, 313, 487)
		key(increase, gui.ModShift)
		checkSplit(t, v, 323, 477)
		key(increase, gui.ModControl)
		checkSplit(t, v, 323, 477)
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

func TestSplitViewAdjacentInput(t *testing.T) {
	for _, direction := range []layout.Direction{layout.DirectionHorizontal, layout.DirectionVertical} {
		for _, childGesture := range []string{"blank", "click", "drag", "scrollbar"} {
			t.Run(fmt.Sprintf("%d/%s", direction, childGesture), func(t *testing.T) {
				v := NewSplitView(direction)
				var child gui.Widget = new(gui.WidgetBase)
				clicks, childDrags := 0, 0
				if childGesture == "click" {
					b := gui.NewButton()
					b.ConnectClicked(func() { clicks++ })
					child = b
				}
				if childGesture == "drag" {
					drag := gui.NewDragEventController()
					drag.ConnectBegin(func(gui.EventContext, gui.DragInfo) { childDrags++ })
					child.AddEventController(drag)
				}
				var scroll *gui.ScrollView
				if childGesture == "scrollbar" {
					scroll = gui.NewScrollView()
					if direction == layout.DirectionHorizontal {
						scroll.SetDirection(gui.ScrollVertical)
					} else {
						scroll.SetDirection(gui.ScrollHorizontal)
					}
					scroll.SetChild(splitTestChild(1000, 1000))
					child = scroll
				}
				v.SetStartChild(child)
				v.SetEndChild(new(gui.WidgetBase))
				v.SetStartSize(100)
				owner := gui.NewPopover(nil, nil)
				owner.SetWidget(v)
				defer owner.Destroy()
				arrangeSplit(v, 201, 100)
				d := new(gui.EventDispatcher)
				point := func(kind events.EventType, main, cross float32) {
					p := geometry.Point{X: main, Y: cross}
					if direction == layout.DirectionVertical {
						p.X, p.Y = p.Y, p.X
					}
					dispatchSplitPointer(t, d, owner.(gui.EventTarget), kind, p.X, p.Y)
				}
				// 退到抓取带之外，子控件保留自身的点击、拖动与滚动行为。
				pick := geometry.Point{X: 97, Y: 5}
				if direction == layout.DirectionVertical {
					pick.X, pick.Y = pick.Y, pick.X
				}
				if got := gui.Pick(v, pick); got == v.handle {
					t.Fatal("分隔条覆盖子控件")
				}
				point(events.PointerDown, 97, 5)
				if v.handle.dragging {
					t.Fatal("分隔区域外按下抢占了子控件")
				}
				if childGesture == "click" {
					point(events.PointerUp, 97, 5)
					if clicks != 1 {
						t.Fatal("邻近分隔区域取消了普通点击")
					}
					return
				}
				point(events.PointerMove, 118, 55)
				checkSplit(t, v, 100, 100)
				if v.handle.dragging {
					t.Fatal("分隔区域外的输入调整了分区")
				}
				if childGesture == "drag" && childDrags != 1 {
					t.Fatal("子级未接收拖动")
				}
				if scroll != nil && scroll.ScrollX()+scroll.ScrollY() <= 0 {
					t.Fatal("相邻滚动条未改变偏移")
				}
				point(events.PointerUp, 118, 55)
				if v.handle.dragging {
					t.Fatal("释放后残留拖动")
				}
			})
		}
	}
}

func TestSplitViewHandleDirectionCancellation(t *testing.T) {
	for _, move := range []bool{false, true} {
		v := splitFixture(layout.DirectionHorizontal)
		v.SetStartSize(240)
		v.SetHandleSize(3)
		owner := gui.NewPopover(nil, nil)
		owner.SetWidget(v)
		defer owner.Destroy()
		arrangeSplit(v, 801, 100)
		d := new(gui.EventDispatcher)
		host := owner.(gui.EventTarget)
		dispatchSplitPointer(t, d, host, events.PointerDown, 241, 50)
		if move {
			dispatchSplitPointer(t, d, host, events.PointerMove, 280, 50)
		}
		v.SetDirection(layout.DirectionVertical)
		dispatchSplitPointer(t, d, host, events.PointerMove, 350, 50)
		dispatchSplitPointer(t, d, host, events.PointerUp, 350, 50)
		if v.handle.dragging || v.preference != (splitPreference{mode: splitStart, value: 240}) {
			t.Fatal("方向变化未清理拖动／恢复偏好")
		}
	}
}

func TestSplitViewNestedHandleStaysIndependent(t *testing.T) {
	inner := splitFixture(layout.DirectionVertical)
	inner.SetStartSize(50)
	outer := NewSplitView(layout.DirectionHorizontal)
	outer.SetStartChild(inner)
	outer.SetEndChild(new(gui.WidgetBase))
	outer.SetStartSize(100)
	owner := gui.NewPopover(nil, nil)
	owner.SetWidget(outer)
	defer owner.Destroy()
	arrangeSplit(outer, 201, 101)
	d := new(gui.EventDispatcher)
	host := owner.(gui.EventTarget)
	dispatchSplitPointer(t, d, host, events.PointerDown, 97, 50.5)
	dispatchSplitPointer(t, d, host, events.PointerMove, 120, 70.5)
	checkSplit(t, outer, 100, 100)
	checkSplit(t, inner, 70, 30)
	if outer.handle.dragging || !inner.handle.dragging {
		t.Fatal("交点之外的内层拖动被外层抢占")
	}
	dispatchSplitPointer(t, d, host, events.PointerUp, 120, 70.5)
}

func TestSplitViewHandleMarginEscapeRestoresPreference(t *testing.T) {
	v := splitFixture(layout.DirectionHorizontal)
	v.SetStartSize(240)
	v.SetHandleSize(3)
	owner := gui.NewPopover(nil, nil)
	owner.SetWidget(v)
	defer owner.Destroy()
	arrangeSplit(v, 801, 100)
	d := new(gui.EventDispatcher)
	host := owner.(gui.EventTarget)
	dispatchSplitPointer(t, d, host, events.PointerDown, 241, 50)
	dispatchSplitPointer(t, d, host, events.PointerMove, 279, 50)
	checkSplit(t, v, 278, 522)
	if err := d.DispatchEvent(host, events.KeyEvent{EventType: events.KeyDown, Key: events.KeyEscape}); err != nil {
		t.Fatal(err)
	}
	dispatchSplitPointer(t, d, host, events.PointerUp, 279, 50)
	checkSplit(t, v, 240, 560)
	if v.handle.dragging || v.preference != (splitPreference{mode: splitStart, value: 240}) {
		t.Fatal("分隔区域留白拖动 Esc 未恢复首选尺寸")
	}
}

func TestSplitViewLifecycleInvalidation(t *testing.T) {
	v := splitFixture(layout.DirectionHorizontal)
	owner := gui.NewPopover(nil, nil)
	owner.SetWidget(v)
	arrangeSplit(v, 801, 200)
	host := &tabInputHost{root: v}
	d := new(gui.EventDispatcher)
	dispatchTabPointer(t, d, host, events.PointerDown, 240.5, 50)
	dispatchTabPointer(t, d, host, events.PointerMove, 340.5, 50)
	owner.SetWidget(nil)
	checkSplit(t, v, 240, 560)
	if v.handle.dragging || v.Root() != nil {
		t.Fatal("unmount failed to cancel drag")
	}
	// Complete the physical button sequence after cancellation before starting
	// a new drag; the dispatcher must not receive two downs without an up.
	dispatchTabPointer(t, d, host, events.PointerUp, 340.5, 50)
	owner.SetWidget(v)
	arrangeSplit(v, 801, 200)
	calls := 0
	v.ConnectResize(func(float32, float32) { owner.Destroy() })
	v.ConnectResize(func(float32, float32) { calls++ })
	dispatchTabPointer(t, d, host, events.PointerDown, 240.5, 50)
	dispatchTabPointer(t, d, host, events.PointerMove, 340.5, 50)
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
	fills []geometry.Rectangle
}

func (p *splitPaintRecorder) FillRect(r geometry.Rectangle, b graphics.Brush) {
	p.rect, p.brush = r, b
	p.fills = append(p.fills, r)
}
func TestSplitViewHandleIndependentStyleAndGeometry(t *testing.T) {
	old := gui.App
	defer func() { gui.App = old }()
	for _, dark := range []bool{false, true} {
		app := &splitStyleApp{sheet: modern.Sheet(modern.Options{Dark: dark})}
		gui.App = app
		v := splitFixture(layout.DirectionHorizontal)
		v.SetStyleName("split-view")
		arrangeSplit(v, 801, 100)
		p := new(splitPaintRecorder)
		v.separator.Paint(p)
		ink, _ := app.sheet.Resolve(style.Sel{Name: "split-view", Part: "separator"}).ForegroundColor()
		if p.rect != geometry.Rect(0, 0, 1, 100) || p.brush != graphics.ColorOf(ink) {
			t.Fatalf("normal separator paint=%v %v", p.rect, p.brush)
		}
		testWidgetHover(t, v.handle, true)
		v.separator.Paint(p)
		active, _ := app.sheet.Resolve(style.Sel{Name: "split-view", Part: "separator", State: style.Hovered}).ForegroundColor()
		if p.rect != geometry.Rect(0, 0, 1, 100) || p.brush != graphics.ColorOf(active) || !colorEqual(ink, active) {
			t.Fatal("default hover changed the neutral 1 DIP separator")
		}
		v.SetRatio(0)
		arrangeSplit(v, 801, 100)
		testWidgetHover(t, v.handle, false)
		v.separator.Paint(p)
		if v.handle.Rect().Width != 1 || p.rect.X != 0 {
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
							style.Name("split-view").Part("separator").State(style.Hovered).ForegroundColor(hover),
							style.Name("split-view").Part("separator").State(style.Pressed).ForegroundColor(pressed),
							// 普通鼠标焦点不参与绘制；键盘焦点由单独的回归覆盖。
							style.Name("split-view").Part("separator").State(style.Focused).ForegroundColor(color.Black))
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
					ink, _ := app.sheet.Resolve(style.Sel{Name: "split-view", Part: "separator"}).ForegroundColor()
					if !custom {
						hover, pressed = color.RGBAModel.Convert(ink).(color.RGBA), color.RGBAModel.Convert(ink).(color.RGBA)
					}
					paint := func(want color.Color) {
						t.Helper()
						p := new(splitPaintRecorder)
						v.separator.Paint(p)
						wantRect := geometry.Rect(0, 0, 1, 100)
						if direction == layout.DirectionVertical {
							wantRect = geometry.Rect(0, 0, 100, 1)
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
						position := geometry.Point{X: main + .5, Y: 50}
						if direction == layout.DirectionVertical {
							position = geometry.Point{X: 50, Y: main + .5}
						}
						if err := dispatcher.DispatchEvent(host, events.PointerEvent{EventType: kind, Position: position,
							Button: events.PointerButtonLeft, Buttons: buttons}); err != nil {
							t.Fatal(err)
						}
					}
					paint(ink)
					point(events.PointerMove, 240, false)
					if !v.handle.State().Has(style.Hovered) {
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
					if v.handle.State().Has(style.Hovered) || v.handle.dragging || !v.handle.Focused() || !v.handle.Snapshot().Focused {
						t.Fatal("pointer exit did not clear feedback while retaining semantic focus")
					}
					paint(ink)
					if v.axis(v.handle.Rect().Size) != 1 {
						t.Fatal("separator bounds must match the 1 DIP gap")
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

func dispatchSplitPointer(t *testing.T, d *gui.EventDispatcher, host gui.EventTarget, kind events.EventType, x, y float32) {
	t.Helper()
	buttons := events.PointerButtonLeftDown
	if kind == events.PointerUp {
		buttons = 0
	}
	if err := d.DispatchEvent(host, events.PointerEvent{EventType: kind, Position: geometry.Point{X: x, Y: y}, Button: events.PointerButtonLeft, Buttons: buttons}); err != nil {
		t.Fatal(err)
	}
}

// 默认只为1 DIP线占位；5 DIP抓取区覆盖两侧各2 DIP，语义使用抓取边界。
func TestSplitViewDefaultHandleAllocation(t *testing.T) {
	for _, direction := range []layout.Direction{layout.DirectionHorizontal, layout.DirectionVertical} {
		v := NewSplitView(direction)
		if v.HandleSize() != 5 {
			t.Fatal("默认厚度不是5 DIP")
		}
		start, end := new(gui.WidgetBase), new(gui.WidgetBase)
		start.SetLayoutManager(&splitTestLayout{desired: v.size(60, 100)})
		end.SetLayoutManager(&splitTestLayout{desired: v.size(140, 50)})
		v.SetStartChild(start)
		v.SetEndChild(end)
		v.SetStartSize(60)
		if got := v.Measure(layout.Unbounded()).Size; got != v.size(201, 100) {
			t.Fatalf("自然尺寸=%v", got)
		}
		arrangeSplit(v, 201, 100)
		checkSplit(t, v, 60, 140)
		want := geometry.Rectangle{Size: v.size(5, 100)}
		if direction == layout.DirectionHorizontal {
			want.X = 58
		} else {
			want.Y = 58
		}
		if v.handle.Rect() != want || v.start.Rect().Size != v.size(60, 100) || v.end.Rect().Size != v.size(140, 100) {
			t.Fatal("分隔区域与面板分配不一致")
		}
		info := v.Snapshot().Children[2]
		if info.Bounds != want || info.Range.Value != 60 || info.Range.Max != 200 || info.Role != RoleSeparator {
			t.Fatalf("语义边界错误：%+v", info)
		}
	}
}

func TestSplitViewHandleSizeAndFullAreaInput(t *testing.T) {
	for _, direction := range []layout.Direction{layout.DirectionHorizontal, layout.DirectionVertical} {
		for _, size := range []float32{1, 1.5, 3, 5, 9} {
			t.Run(fmt.Sprintf("%d/%g", direction, size), func(t *testing.T) {
				v := NewSplitView(direction)
				v.SetHandleSize(size)
				v.SetStartChild(new(gui.WidgetBase))
				v.SetEndChild(new(gui.WidgetBase))
				v.SetStartSize(60)
				owner := gui.NewPopover(nil, nil)
				owner.SetWidget(v)
				defer owner.Destroy()
				d := new(gui.EventDispatcher)
				host := owner.(gui.EventTarget)
				point := func(kind events.EventType, main float32) {
					p := geometry.Point{X: main, Y: 30}
					if direction == layout.DirectionVertical {
						p.X, p.Y = p.Y, p.X
					}
					dispatchSplitPointer(t, d, host, kind, p.X, p.Y)
				}
				for _, offset := range []float32{.05, size / 2, size - .05} {
					v.SetStartSize(60)
					arrangeSplit(v, 201, 80)
					left := float32(60.5) - size/2
					p := geometry.Point{X: left + offset, Y: 30}
					if direction == layout.DirectionVertical {
						p.X, p.Y = p.Y, p.X
					}
					if gui.Pick(v, p) != v.handle {
						t.Fatal("分隔区域内未命中把手")
					}
					point(events.PointerMove, left+offset)
					if !v.handle.State().Has(style.Hovered) {
						t.Fatal("留白区域没有悬停反馈")
					}
					want := gui.CursorResizeHorizontal
					if direction == layout.DirectionVertical {
						want = gui.CursorResizeVertical
					}
					if v.handle.Cursor() != want {
						t.Fatal("留白区域光标错误")
					}
					point(events.PointerDown, left+offset)
					if !v.handle.dragging || host.FocusedWidget() != v.handle {
						t.Fatal("区域内按下没有立即拖动／聚焦")
					}
					point(events.PointerMove, left+10+offset)
					point(events.PointerUp, left+10+offset)
					checkSplit(t, v, 70, 130)
				}
				for _, outside := range []float32{60.5 - size/2 - .1, 60.5 + size/2} {
					v.SetStartSize(60)
					arrangeSplit(v, 201, 80)
					point(events.PointerMove, outside)
					if v.handle.State().Has(style.Hovered) {
						t.Fatal("区域外残留悬停")
					}
					point(events.PointerDown, outside)
					point(events.PointerMove, outside+15)
					point(events.PointerUp, outside+15)
					checkSplit(t, v, 60, 140)
				}
			})
		}
	}
}

func TestSplitViewHandleConfigurationAndTinyParents(t *testing.T) {
	v := splitFixture(layout.DirectionHorizontal)
	v.SetHandleSize(7)
	for _, bad := range []float32{-1, float32(math.NaN()), float32(math.Inf(1))} {
		v.SetHandleSize(bad)
		if v.HandleSize() != 7 {
			t.Fatal("非法配置改变厚度")
		}
	}
	v.SetHandleSize(0)
	if v.HandleSize() != 5 {
		t.Fatal("零值没有恢复默认")
	}
	changes := 0
	v.ConnectResize(func(float32, float32) { changes++ })
	for _, extent := range []float32{0, .5, 2, 3, 5, 105} {
		arrangeSplit(v, extent, 40)
		a, b := v.Sizes()
		g := v.separator.Rect().Width
		h := v.handle.Rect()
		if a < 0 || b < 0 || a+b+g != extent || g != min(extent, float32(1)) || v.HandleSize() != 5 || h.X < 0 || h.X+h.Width > extent {
			t.Fatalf("极小布局错误：%g/%g/%g", a, g, b)
		}
	}
	if changes != 0 {
		t.Fatal("配置／布局发送了用户调整通知")
	}
	v.SetStartSize(40)
	arrangeSplit(v, 105, 40)
	owner := gui.NewPopover(nil, nil)
	owner.SetWidget(v)
	defer owner.Destroy()
	d := new(gui.EventDispatcher)
	host := owner.(gui.EventTarget)
	dispatchSplitPointer(t, d, host, events.PointerDown, 41, 20)
	dispatchSplitPointer(t, d, host, events.PointerMove, 51, 20)
	v.SetHandleSize(7)
	dispatchSplitPointer(t, d, host, events.PointerMove, 71, 20)
	dispatchSplitPointer(t, d, host, events.PointerUp, 71, 20)
	if v.handle.dragging || v.preference != (splitPreference{mode: splitStart, value: 40}) || v.HandleSize() != 7 {
		t.Fatal("动态厚度没有取消旧拖动并保留偏好")
	}
	arrangeSplit(v, 101, 40)
	checkSplit(t, v, 40, 60)
	v.EndChild().SetVisible(false)
	arrangeSplit(v, 101, 40)
	if v.handle.Visible() {
		t.Fatal("单侧仍显示分隔区域")
	}
	checkSplit(t, v, 101, 0)
	v.EndChild().SetVisible(true)
	arrangeSplit(v, 101, 40)
	checkSplit(t, v, 40, 60)
}

func TestSplitViewHandlePartScopeAndKeyboardFocus(t *testing.T) {
	old := gui.App
	defer func() { gui.App = old }()
	gui.App = &splitStyleApp{sheet: modern.Sheet(modern.Options{})}
	v := NewSplitView(layout.DirectionHorizontal)
	v.SetStyleName("orders-split")
	v.SetHandleSize(3)
	v.SetStartChild(new(gui.WidgetBase))
	v.SetEndChild(new(gui.WidgetBase))
	v.SetStartSize(40)
	scope := gui.NewStyleScope()
	rules := []style.Rule{
		style.Name("orders-split").Part("separator").ForegroundColor(color.RGBA{G: 200, A: 255}).BorderWidth(1),
		style.Name("orders-split").Part("separator").State(style.FocusVisible).ForegroundColor(color.Black),
		style.Name("other-split").Part("separator").ForegroundColor(color.White).BorderWidth(3),
	}
	scope.SetStyleSheet(style.Sheet(rules...))
	scope.SetChild(v)
	owner := gui.NewPopover(nil, nil)
	owner.SetWidget(scope)
	defer owner.Destroy()
	arrangeSplit(v, 103, 40)
	scope.Arrange(geometry.Rect(0, 0, 103, 40))
	// Tab 经正常焦点导航建立键盘焦点，不修改私有焦点模式。
	d := new(gui.EventDispatcher)
	host := owner.(gui.EventTarget)
	if err := d.DispatchEvent(host, events.KeyEvent{EventType: events.KeyDown, Key: events.KeyTab}); err != nil {
		t.Fatal(err)
	}
	if host.FocusedWidget() != v.handle || !v.handle.FocusVisible() {
		t.Fatal("键盘焦点不可见")
	}
	p := new(splitPaintRecorder)
	v.separator.Paint(p)
	if len(p.fills) != 1 || p.brush != graphics.ColorOf(color.Black) {
		t.Fatal("Part作用域或键盘焦点样式错误")
	}
	dispatchSplitPointer(t, d, host, events.PointerDown, 41, 20)
	dispatchSplitPointer(t, d, host, events.PointerUp, 41, 20)
	p = new(splitPaintRecorder)
	v.separator.Paint(p)
	if v.handle.FocusVisible() || p.brush != graphics.ColorOf(color.RGBA{G: 200, A: 255}) {
		t.Fatal("鼠标焦点保留了键盘高亮")
	}
	v.SetStyleName("other-split")
	arrangeSplit(v, 103, 40)
	p = new(splitPaintRecorder)
	v.separator.Paint(p)
	if p.brush != graphics.ColorOf(color.White) || p.rect.Width != 3 {
		t.Fatal("动态样式名未更新，或不同实例样式混用")
	}
}

// 使用独立的物理边界预期检查两侧面板与透明抓取区；RGBA均为预乘颜色。
// Software矩形采用最近物理像素边界，单通道允许1级整数舍入误差。
func TestSplitViewHandleSoftwarePixels(t *testing.T) {
	old := gui.App
	defer func() { gui.App = old }()
	start, end, line := color.RGBA{R: 128, A: 128}, color.RGBA{B: 128, A: 128}, color.RGBA{G: 128, A: 128}
	gui.App = &splitStyleApp{sheet: style.Sheet(style.Name("split-view").Part("separator").ForegroundColor(line).BorderWidth(1))}
	native, err := software.NewPainter(new(progressPixelSurface))
	if err != nil {
		t.Fatal(err)
	}
	defer native.Destroy()
	for _, direction := range []layout.Direction{layout.DirectionHorizontal, layout.DirectionVertical} {
		for _, scale := range []float32{1, 1.25, 1.5, 2} {
			for _, phase := range []float32{0, .25, .5} {
				v := NewSplitView(direction)
				v.SetStartChild(new(gui.WidgetBase))
				v.SetEndChild(new(gui.WidgetBase))
				v.SetStartSize(10)
				arrangeSplit(v, 21, 8)
				offset := float32(3) + phase/scale
				img, err := native.RenderImage(64, 64, scale, func() {
					native.Clear(graphics.Color{})
					native.SetTransform(geometry.Translate(offset, offset))
					native.FillRect(v.start.Rect(), graphics.ColorOf(start))
					native.FillRect(v.end.Rect(), graphics.ColorOf(end))
					v.handle.Paint(&progressPixelPainter{native: native})
					r := v.separator.Rect()
					native.SetTransform(geometry.Translate(offset+r.X, offset+r.Y))
					v.separator.Paint(&progressPixelPainter{native: native})
				})
				if err != nil {
					t.Fatal(err)
				}
				edge := func(n float32) int { return int(math.Floor(float64((offset+n)*scale) + .5)) }
				left, first, last, right, top, bottom := edge(0), edge(10), edge(11), edge(21), edge(0), edge(8)
				for y := top - 1; y <= bottom; y++ {
					for x := left - 1; x <= right; x++ {
						want := color.RGBA{}
						if x >= left && x < right && y >= top && y < bottom {
							switch {
							case x < first:
								want = start
							case x < last:
								want = line
							default:
								want = end
							}
						}
						px, py := x, y
						if direction == layout.DirectionVertical {
							px, py = py, px
						}
						got := color.RGBAModel.Convert(img.At(px, py)).(color.RGBA)
						for channel, value := range []uint8{got.R, got.G, got.B, got.A} {
							expected := [4]uint8{want.R, want.G, want.B, want.A}[channel]
							if math.Abs(float64(value)-float64(expected)) > 1 {
								t.Fatalf("%d/%gx/phase%g at(%d,%d)=%v，预期 %v", direction, scale, phase, px, py, got, want)
							}
						}
					}
				}
			}
		}
	}
}

func TestSplitViewSeparatorWidthAndGrabAreIndependent(t *testing.T) {
	old := gui.App
	defer func() { gui.App = old }()
	for _, direction := range []layout.Direction{layout.DirectionHorizontal, layout.DirectionVertical} {
		for _, width := range []float32{0, 1, 2, 9, -1, float32(math.NaN()), float32(math.Inf(1))} {
			gui.App = &splitStyleApp{sheet: style.Sheet(style.Name("split-view").Part("separator").ForegroundColor(color.Black).BorderWidth(width))}
			v := NewSplitView(direction)
			v.SetHandleSize(3)
			v.SetStartChild(new(gui.WidgetBase))
			v.SetEndChild(new(gui.WidgetBase))
			v.SetStartSize(40)
			arrangeSplit(v, 103, 30)
			gap := width
			if !splitFinite(gap) || gap < 0 {
				gap = 0
			}
			checkSplit(t, v, 40, 63-gap)
			if v.axis(v.separator.Rect().Size) != gap || v.axis(v.handle.Rect().Size) != 3 {
				t.Fatal("线宽和抓取范围互相钳制")
			}
			p := new(splitPaintRecorder)
			v.separator.Paint(p)
			if gap == 0 {
				if len(p.fills) != 0 {
					t.Fatal("隐藏线仍然绘制")
				}
			} else if len(p.fills) != 1 || p.rect.Size != v.size(gap, 30) {
				t.Fatal("线条绘制宽度错误")
			}
			grabPaint := new(splitPaintRecorder)
			v.handle.Paint(grabPaint)
			if len(grabPaint.fills) != 0 {
				t.Fatal("抓取范围仍绘制背景")
			}
		}
	}
}

func TestSplitViewHandleOwnsCoveredControls(t *testing.T) {
	for _, direction := range []layout.Direction{layout.DirectionHorizontal, layout.DirectionVertical} {
		v := NewSplitView(direction)
		child := gui.NewButton()
		clicks, drags := 0, 0
		child.ConnectClicked(func() { clicks++ })
		drag := gui.NewDragEventController()
		drag.ConnectBegin(func(gui.EventContext, gui.DragInfo) { drags++ })
		child.AddEventController(drag)
		v.SetStartChild(child)
		v.SetEndChild(new(gui.WidgetBase))
		v.SetStartSize(60)
		owner := gui.NewPopover(nil, nil)
		owner.SetWidget(v)
		defer owner.Destroy()
		arrangeSplit(v, 201, 80)
		d := new(gui.EventDispatcher)
		host := owner.(gui.EventTarget)
		point := func(kind events.EventType, main float32) {
			p := geometry.Point{X: main, Y: 30}
			if direction == layout.DirectionVertical {
				p.X, p.Y = p.Y, p.X
			}
			dispatchSplitPointer(t, d, host, kind, p.X, p.Y)
		}
		point(events.PointerMove, 58.5)
		if !v.handle.State().Has(style.Hovered) || child.State().Has(style.Hovered) {
			t.Fatal("覆盖区悬停归属错误")
		}
		point(events.PointerDown, 58.5)
		point(events.PointerMove, 68.5)
		point(events.PointerUp, 68.5)
		checkSplit(t, v, 70, 130)
		if clicks != 0 || drags != 0 {
			t.Fatal("覆盖区输入穿透到子控件")
		}
	}
}

func TestSplitViewNestedIntersectionPriorityAndCapture(t *testing.T) {
	inner := NewSplitView(layout.DirectionVertical)
	inner.SetStartChild(new(gui.WidgetBase))
	inner.SetEndChild(new(gui.WidgetBase))
	inner.SetStartSize(50)
	outer := NewSplitView(layout.DirectionHorizontal)
	outer.SetStartChild(inner)
	outer.SetEndChild(new(gui.WidgetBase))
	outer.SetStartSize(100)
	owner := gui.NewPopover(nil, nil)
	owner.SetWidget(outer)
	defer owner.Destroy()
	arrangeSplit(outer, 201, 101)
	d := new(gui.EventDispatcher)
	host := owner.(gui.EventTarget)
	dispatchSplitPointer(t, d, host, events.PointerDown, 99, 50.5)
	if !outer.handle.dragging || inner.handle.dragging {
		t.Fatal("交点未由外层把手优先认领")
	}
	dispatchSplitPointer(t, d, host, events.PointerMove, 109, 70.5)
	dispatchSplitPointer(t, d, host, events.PointerUp, 109, 70.5)
	checkSplit(t, outer, 110, 90)
	checkSplit(t, inner, 50, 50)
	outer.SetStartSize(100)
	arrangeSplit(outer, 201, 101)
	dispatchSplitPointer(t, d, host, events.PointerDown, 90, 50.5)
	dispatchSplitPointer(t, d, host, events.PointerMove, 110, 60.5)
	dispatchSplitPointer(t, d, host, events.PointerUp, 110, 60.5)
	checkSplit(t, outer, 100, 100)
	checkSplit(t, inner, 60, 40)
	if outer.handle.dragging || inner.handle.dragging {
		t.Fatal("交点释放后残留捕获")
	}
}

func TestSplitViewSeparatorFeedbackDoesNotRelayout(t *testing.T) {
	old := gui.App
	defer func() { gui.App = old }()
	gui.App = &splitStyleApp{sheet: style.Sheet(
		style.Name("split-view").Part("separator").ForegroundColor(color.Black).BorderWidth(1),
		style.Name("split-view").Part("separator").State(style.Hovered).ForegroundColor(color.White).BorderWidth(9),
	)}
	v := NewSplitView(layout.DirectionHorizontal)
	v.SetStartChild(new(gui.WidgetBase))
	v.SetEndChild(new(gui.WidgetBase))
	v.SetStartSize(40)
	arrangeSplit(v, 101, 30)
	before := v.handle.Rect()
	testWidgetHover(t, v.handle, true)
	arrangeSplit(v, 101, 30)
	checkSplit(t, v, 40, 60)
	p := new(splitPaintRecorder)
	v.separator.Paint(p)
	if v.handle.Rect() != before || p.rect.Width != 1 || p.brush != graphics.ColorOf(color.White) {
		t.Fatal("反馈改变布局或未改变线色")
	}
}
