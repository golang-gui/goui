package gui

import (
	"testing"

	"github.com/golang-gui/goui/core/geometry"
	"github.com/golang-gui/goui/layout"
	"github.com/golang-gui/goui/platform"
	"github.com/golang-gui/goui/platform/events"
)

func TestHeaderBarAutomaticDragControllerPolicy(t *testing.T) {
	tests := []struct {
		name       string
		controller func() EventController
		want       platform.WindowHit
	}{
		{"none", func() EventController { return nil }, platform.WindowHitCaption},
		{"motion", func() EventController { return NewMotionEventController() }, platform.WindowHitCaption},
		{"key", func() EventController { return NewKeyEventController() }, platform.WindowHitCaption},
		{"click", func() EventController { return NewClickEventController() }, platform.WindowHitClient},
		{"drag", func() EventController { return NewDragEventController() }, platform.WindowHitClient},
		{"wheel", func() EventController { return NewWheelEventController() }, platform.WindowHitClient},
		{"custom", func() EventController { return &EventControllerBase{} }, platform.WindowHitClient},
	}
	for _, test := range tests {
		for _, location := range []string{"target", "ancestor", "header"} {
			t.Run(test.name+"/"+location, func(t *testing.T) {
				win, native := chromeFixture(t, false, true)
				header := NewHeaderBar()
				row := NewLinearBox(layout.DirectionHorizontal)
				label := NewLabel("Title")
				label.SetMinSize(geometry.Size{Width: 80, Height: 24})
				row.AddChild(label)
				button := NewButton() // unrelated interactive sibling
				row.AddChild(button)
				header.SetChild(row)
				var owner Widget = label
				if location == "ancestor" {
					owner = row
				}
				if location == "header" {
					owner = header
				}
				controller := test.controller()
				owner.AddEventController(controller)
				win.SetWidget(header)
				win.paint()
				// This fixture has no typography context. Give the label a real
				// hit area without depending on font measurement.
				label.Arrange(geometry.Rect(0, 0, 80, 24))
				button.Arrange(geometry.Rect(100, 0, 80, 24))
				point := label.windowRect().Center()
				if hitTest(header, point) != label {
					t.Fatal("fixture did not hit label")
				}
				if got := native.hitTest(point); got != test.want {
					t.Fatalf("hit=%v, want %v; label=%v row=%v target=%T default=%v", got, test.want, label.Rect(), row.Rect(), hitTest(header, point), header.defaultDragRegion(label))
				}
				owner.RemoveEventController(controller)
				if native.hitTest(point) != platform.WindowHitCaption {
					t.Fatal("controller removal was not reflected immediately")
				}
				owner.SetFocusable(true)
				if native.hitTest(point) != platform.WindowHitClient {
					t.Fatal("focusable path node became draggable")
				}
				owner.SetFocusable(false)
				if native.hitTest(point) != platform.WindowHitCaption {
					t.Fatal("focusability removal was not reflected immediately")
				}
			})
		}
	}
}

func TestHeaderBarButtonContentDoesNotDrag(t *testing.T) {
	win, native := chromeFixture(t, false, true)
	header := NewHeaderBar()
	button := NewButton()
	button.SetFocusable(false)
	label := NewLabel("Save")
	button.SetChild(label)
	header.SetChild(button)
	win.SetWidget(header)
	win.paint()
	label.Arrange(geometry.Rect(0, 0, 40, 20)) // fixture has no typography
	if hitTest(header, label.windowRect().Center()) != label {
		t.Fatal("fixture did not hit button label")
	}
	if got := native.hitTest(label.windowRect().Center()); got != platform.WindowHitClient {
		t.Fatalf("label bypassed non-focusable button: %v", got)
	}
}

func BenchmarkHeaderBarDefaultDragRegion(b *testing.B) {
	header := NewHeaderBar()
	row := NewLinearBox(layout.DirectionHorizontal)
	header.SetChild(row)
	for range 8 {
		child := NewLinearBox(layout.DirectionHorizontal)
		row.AddChild(child)
		row = child
	}
	label := NewLabel("Title")
	row.AddChild(label)
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if !header.defaultDragRegion(label) {
			b.Fatal("passive path rejected")
		}
	}
}

func TestSplitHeaderBarsAndOrderedOverride(t *testing.T) {
	win, native := chromeFixture(t, false, true)
	row := NewLinearBox(layout.DirectionHorizontal)
	left, right := NewHeaderBar(), NewHeaderBar()
	setHeaderTitle(left, "Left")
	setHeaderTitle(right, "Right")
	left.SetMainWeight(1)
	right.SetMainWeight(1)
	row.AddChild(left)
	row.AddChild(right)
	win.SetWidget(row)
	win.paint()
	for _, h := range []*HeaderBar{left, right} {
		p := h.windowRect().Center()
		if got := native.hitTest(p); got != platform.WindowHitCaption {
			t.Fatalf("split header %v: %v", p, got)
		}
	}
	p := right.windowRect().Center()
	override := win.Chrome().ConnectQueryRegion(func(_ geometry.Point, result *ChromeRegion) { *result = ChromeRegionClient })
	if native.hitTest(p) != platform.WindowHitClient {
		t.Fatal("later signal could not suppress header drag")
	}
	override.Disconnect()
	left.SetVisible(false)
	win.paint()
	if native.hitTest(right.windowRect().Center()) != platform.WindowHitCaption {
		t.Fatal("hiding one segment removed the other")
	}
	row.RemoveChild(right)
	win.paint()
	if got := native.hitTest(p); got != platform.WindowHitDefault {
		t.Fatalf("unmounted header retained query: %v", got)
	}
}

func TestHeaderBarCustomContentAndOverlay(t *testing.T) {
	win, native := chromeFixture(t, false, true)
	header := NewHeaderBar()
	setHeaderTitle(header, "Title")
	button := NewButton()
	button.SetMinSize(geometry.Size{Width: 80, Height: 32})
	header.Child().(*LinearBox).AddChild(button)
	root := newTestWidget()
	root.WidgetBase.AddChild(root, header)
	win.SetWidget(root)
	win.paint()
	header.Arrange(geometry.Rect(0, 0, 640, 48))
	if got := native.hitTest(button.windowRect().Center()); got != platform.WindowHitClient {
		t.Fatalf("application button became drag region: %v", got)
	}
	if got := native.hitTest(geometry.Point{X: 1, Y: 1}); got != platform.WindowHitCaption {
		t.Fatal("header padding did not request dragging")
	}
	overlay := newTestWidget()
	root.WidgetBase.AddChild(root, overlay)
	win.paint()
	header.Arrange(geometry.Rect(0, 0, 640, 48))
	overlay.Arrange(geometry.Rect(0, 0, 640, 48))
	if got := native.hitTest(geometry.Point{X: 100, Y: 20}); got != platform.WindowHitDefault {
		t.Fatal("standard header hit through covering Widget")
	}
}

func TestHeaderBarPaddingParticipatesInDragQuery(t *testing.T) {
	win, native := chromeFixture(t, false, true)
	root := NewLinearBox(layout.DirectionVertical)
	header := NewHeaderBar()
	header.SetPadding(16)
	header.SetMinSize(geometry.Size{Height: 64})
	button := NewButton()
	button.SetMinSize(geometry.Size{Height: 24})
	header.SetChild(button)
	root.AddChild(header)
	root.AddChild(newSizedWidget(geometry.Size{Width: 100, Height: 40}))
	win.SetWidget(root)
	win.paint()
	// All four padding strips, without claiming the custom controls at right.
	for _, point := range []geometry.Point{{X: 100, Y: 4}, {X: 100, Y: 60}, {X: 4, Y: 32}} {
		if got := native.hitTest(point); got != platform.WindowHitCaption {
			t.Fatalf("padding %v hit=%v, want Caption", point, got)
		}
	}
	if got := native.hitTest(button.windowRect().Center()); got != platform.WindowHitClient {
		t.Fatal("interactive content became draggable")
	}
	if got := native.hitTest(geometry.Point{X: 100, Y: 65}); got == platform.WindowHitCaption {
		t.Fatal("drag region escaped header allocation")
	}
	queried := false
	handle := header.ConnectDragRegion(func(local geometry.Point, drag *bool) {
		queried = local == (geometry.Point{X: 100, Y: 60})
		*drag = false
	})
	if native.hitTest(geometry.Point{X: 100, Y: 60}) != platform.WindowHitClient || !queried {
		t.Fatal("padding bypassed the application's drag-region override")
	}
	handle.Disconnect()
	// With no window-controls reservation, the right padding is background too.
	header.Arrange(geometry.Rect(20, 100, 200, 64))
	if got := win.chrome.queryRegion(geometry.Point{X: 216, Y: 132}); got != ChromeRegionCaption {
		t.Fatalf("translated right padding hit=%v, want Drag", got)
	}
}

func TestHeaderBarCrossWindowRemount(t *testing.T) {
	first, native1 := chromeFixture(t, false, true)
	second, native2 := chromeFixture(t, false, true)
	header := NewHeaderBar()
	setHeaderTitle(header, "Move between windows")
	first.SetWidget(header)
	first.paint()
	p := geometry.Point{X: 100, Y: 20}
	if native1.hitTest(p) != platform.WindowHitCaption {
		t.Fatal("initial binding missing")
	}
	first.SetWidget(nil)
	second.SetWidget(header)
	second.paint()
	first.paint()
	if native1.hitTest(p) != platform.WindowHitDefault || native2.hitTest(p) != platform.WindowHitCaption {
		t.Fatal("remount did not transfer signal connections")
	}
}

func TestHeaderBarClientMoveDoesNotClickTools(t *testing.T) {
	win, native := chromeFixture(t, false, false)
	header := NewHeaderBar()
	setHeaderTitle(header, "Title")
	button := NewButton()
	button.SetMinSize(geometry.Size{Width: 80, Height: 32})
	header.Child().(*LinearBox).AddChild(button)
	win.SetWidget(header)
	win.paint()
	clicks := 0
	button.ConnectClicked(func() { clicks++ })
	clickAt(win, button.windowRect().Center())
	if clicks != 1 || native.moveRequests != 0 {
		t.Fatal("tool click became native movement")
	}
	dragCaptionAt(win, geometry.Point{X: 100, Y: 20})
	if clicks != 1 || native.moveRequests != 1 {
		t.Fatal("blank/title region did not hand off exactly once")
	}
}

func clickAt(win *window, p geometry.Point) {
	for _, kind := range []events.EventType{events.PointerDown, events.PointerUp} {
		_ = win.DispatchEvent(events.PointerEvent{EventType: kind, Button: events.PointerButtonLeft, Position: p})
	}
}

func dragCaptionAt(win *window, p geometry.Point) {
	_ = win.DispatchEvent(events.PointerEvent{EventType: events.PointerDown, Button: events.PointerButtonLeft, Buttons: events.PointerButtonLeftDown, Position: p})
	p.X += 10
	_ = win.DispatchEvent(events.PointerEvent{EventType: events.PointerMove, Buttons: events.PointerButtonLeftDown, Position: p})
	_ = win.DispatchEvent(events.PointerEvent{EventType: events.PointerUp, Button: events.PointerButtonLeft, Position: p})
}

func setHeaderTitle(header *HeaderBar, text string) {
	content := NewLinearBox(layout.DirectionHorizontal)
	title := NewLabel(text)
	title.SetMainWeight(1)
	content.AddChild(title)
	header.SetChild(content)
}

func TestHeaderBarAutomaticControlsAvoidance(t *testing.T) {
	for _, nativeButtons := range []bool{false, true} {
		win, native := chromeFixture(t, nativeButtons, !nativeButtons)
		root := newTestWidget()
		left, right, below := NewHeaderBar(), NewHeaderBar(), NewHeaderBar()
		for _, header := range []*HeaderBar{left, right, below} {
			child := newTestWidget()
			child.SetMinSize(geometry.Size{Width: 30, Height: 20})
			header.SetChild(child)
			root.WidgetBase.AddChild(root, header)
		}
		win.SetWidget(root)
		win.paint()
		arrange := func() {
			left.Arrange(geometry.Rect(0, 0, 320, 48))
			right.Arrange(geometry.Rect(320, 0, 320, 48))
			below.Arrange(geometry.Rect(0, 100, 640, 48))
		}
		arrange()
		occupied := chromeInfo(win.Chrome()).ControlsBounds
		for _, header := range []*HeaderBar{left, right, below} {
			if !emptyRect(header.Child().base().windowRect().Intersect(occupied)) {
				t.Fatalf("child overlapped controls: %v / %v", header.Child().Rect(), occupied)
			}
			if header.Padding() != 8 {
				t.Fatal("avoidance mutated padding")
			}
		}
		if below.Child().Rect().X != 8 || below.Child().Rect().Width != 624 {
			t.Fatal("unrelated row was inset")
		}
		if nativeButtons {
			if left.Child().Rect().X != occupied.X+occupied.Width+left.Padding() || right.Child().Rect().X != 8 {
				t.Fatal("native left reservation not local to the intersecting segment")
			}
			// Padding belongs to the remaining content area, including when it
			// is larger than the controls' reservation.
			left.SetPadding(80)
			arrange()
			if left.Child().Rect().X != occupied.X+occupied.Width+80 {
				t.Fatal("controls consumed content padding")
			}
			// A transient native query failure retains the reservation.
			native.queryError = platform.ErrUnavailable
			win.chrome.refresh()
			arrange()
			if left.Child().Rect().X != occupied.X+occupied.Width+80 || left.info.ControlsBounds != occupied {
				t.Fatal("unavailable controls lost conservative reservation")
			}
		} else {
			if left.Child().Rect().Width != 304 || right.Child().Rect().Width != 320-16-captionButtonWidth*3 {
				t.Fatalf("custom right reservation: %v / %v", left.Child().Rect(), right.Child().Rect())
			}
			right.Arrange(geometry.Rect(600, 0, 20, 48))
			if right.Child().Rect().Width != 0 {
				t.Fatal("narrow header did not clamp exhausted content to zero")
			}
		}
	}
}

func TestHeaderBarSingleChildAndDragQuery(t *testing.T) {
	win, native := chromeFixture(t, false, true)
	header := NewHeaderBar()
	if header.Child() != nil || len(header.Children()) != 0 {
		t.Fatal("implicit title")
	}
	child := newTestWidget()
	child.SetMinSize(geometry.Size{Height: 24})
	child.AddEventController(&EventControllerBase{}) // unknown controller: explicit opt-in
	header.SetChild(child)
	win.SetWidget(header)
	win.paint()
	point := child.windowRect().Center()
	if native.hitTest(point) != platform.WindowHitClient {
		t.Fatal("child default was draggable")
	}
	first := header.ConnectDragRegion(func(local geometry.Point, drag *bool) {
		if Pick(header, local) != child {
			t.Fatal("local query/pick coordinates differ")
		}
		*drag = true
	})
	if native.hitTest(point) != platform.WindowHitCaption {
		t.Fatal("explicit passive child not draggable")
	}
	last := header.ConnectDragRegion(func(_ geometry.Point, drag *bool) { *drag = false })
	if native.hitTest(point) != platform.WindowHitClient {
		t.Fatal("last query did not override")
	}
	last.Disconnect()
	first.Disconnect()
	header.SetChild(nil)
	win.paint()
	if child.Window() != nil || len(header.Children()) != 0 {
		t.Fatal("child not detached")
	}
}

func TestHeaderFillsDefaultColumnWithoutStretchingMenuButton(t *testing.T) {
	win, native := chromeFixture(t, false, true)
	root := NewLinearBox(layout.DirectionVertical)
	header := NewHeaderBar()
	header.SetChild(NewLabel("Title"))
	header.ConnectDragRegion(func(_ geometry.Point, drag *bool) { *drag = true })
	root.AddChild(header)
	content := NewMenuButton()
	content.SetChild(newSizedWidget(geometry.Size{Width: 40, Height: 24}))
	natural := content.Measure(layout.Unbounded()).Size
	root.AddChild(content)
	win.SetWidget(root)
	for _, width := range []float32{640, 800, 240} {
		_ = win.DispatchEvent(events.SizeEvent{Width: width, Height: 400})
		win.paint()
		if header.Rect().Width != width {
			t.Fatalf("header width = %g, want %g", header.Rect().Width, width)
		}
		if content.Rect().Size != natural {
			t.Fatalf("menu button stretched: %v, want %v", content.Rect().Size, natural)
		}
		if win.chrome.info.ControlsBounds.Height != header.Rect().Height {
			t.Fatal("controls did not follow the default-column header")
		}
		blank := geometry.Point{X: (width - captionButtonWidth*3) / 2, Y: 24}
		if native.hitTest(blank) != platform.WindowHitCaption {
			t.Fatalf("blank header area did not drag at %v", blank)
		}
		if native.hitTest(content.windowRect().Center()) == platform.WindowHitCaption {
			t.Fatal("header claimed content below its allocation")
		}
	}
}

func TestHeaderBarMeasuresAvailableWidth(t *testing.T) {
	for _, test := range []struct {
		name                   string
		constraint             layout.Constraint
		minSize, maxSize, want geometry.Size
	}{
		{name: "bounded", constraint: layout.Loose(geometry.Size{Width: 320, Height: 200}), want: geometry.Size{Width: 320, Height: 40}},
		{name: "unbounded", constraint: layout.Unbounded(), want: geometry.Size{Width: 76, Height: 40}},
		{name: "maximum", constraint: layout.Loose(geometry.Size{Width: 320, Height: 200}), maxSize: geometry.Size{Width: 140}, want: geometry.Size{Width: 140, Height: 40}},
		{name: "minimum unbounded", constraint: layout.Unbounded(), minSize: geometry.Size{Width: 100, Height: 48}, want: geometry.Size{Width: 100, Height: 48}},
		{name: "parent wins", constraint: layout.Tight(geometry.Size{Width: 240, Height: 72}), maxSize: geometry.Size{Width: 140}, want: geometry.Size{Width: 240, Height: 72}},
		{name: "narrow", constraint: layout.Loose(geometry.Size{Width: 10, Height: 20}), want: geometry.Size{Width: 10, Height: 20}},
		{name: "zero", constraint: layout.Tight(geometry.Size{}), want: geometry.Size{}},
	} {
		t.Run(test.name, func(t *testing.T) {
			header := NewHeaderBar()
			header.SetChild(&baselineSizedWidget{measurement: layout.MeasuredWithBaseline(geometry.Size{Width: 60, Height: 20}, 14)})
			if test.minSize != (geometry.Size{}) {
				header.SetMinSize(test.minSize)
			}
			header.SetMaxSize(test.maxSize)
			got := header.Measure(test.constraint)
			if got.Size != test.want {
				t.Fatalf("measurement = %v, want %v", got.Size, test.want)
			}
			if test.want.Height >= 40 {
				header.Arrange(geometry.Rectangle{Size: got.Size})
				if !got.HasBaseline || got.Baseline != header.Child().Rect().Y+14 {
					t.Fatalf("baseline no longer matches arranged child: %+v / %v", got, header.Child().Rect())
				}
			}
		})
	}
	for _, withHiddenChild := range []bool{false, true} {
		header := NewHeaderBar()
		if withHiddenChild {
			child := newSizedWidget(geometry.Size{Width: 200, Height: 100})
			child.SetVisible(false)
			header.SetChild(child)
		}
		if got := header.Measure(layout.Loose(geometry.Size{Width: 320, Height: 200})); got.Size != (geometry.Size{Width: 320, Height: 40}) {
			t.Fatalf("empty header did not fill width: %+v", got)
		}
		header.SetVisible(false)
		if got := header.Measure(layout.Unbounded()); got != (layout.Measurement{}) {
			t.Fatalf("hidden header measured nonzero: %+v", got)
		}
	}
}

func TestWeightedHeadersFillTheirOwnHorizontalAllocation(t *testing.T) {
	row := NewLinearBox(layout.DirectionHorizontal)
	left, right := NewHeaderBar(), NewHeaderBar()
	for _, header := range []*HeaderBar{left, right} {
		header.SetChild(newSizedWidget(geometry.Size{Width: 60, Height: 20}))
		header.SetMainWeight(1)
		row.AddChild(header)
	}
	row.Measure(layout.Tight(geometry.Size{Width: 400, Height: 48}))
	row.Arrange(geometry.Rect(0, 0, 400, 48))
	if left.Rect() != geometry.Rect(0, 4, 200, 40) || right.Rect() != geometry.Rect(200, 4, 200, 40) {
		t.Fatalf("headers exceeded their assigned widths: %v / %v", left.Rect(), right.Rect())
	}
}

func TestPickUsesLocalCoordinatesAndPaintOrder(t *testing.T) {
	root, back, front := newTestWidget(), newTestWidget(), newTestWidget()
	root.WidgetBase.AddChild(root, back)
	root.WidgetBase.AddChild(root, front)
	root.Arrange(geometry.Rect(100, 100, 50, 50))
	back.Arrange(geometry.Rect(5, 5, 60, 60))
	front.Arrange(geometry.Rect(5, 5, 20, 20))
	point := geometry.Point{X: 10, Y: 10}
	if Pick(root, point) != front || hitTest(root, point.Add(root.Rect().Pos)) != front {
		t.Fatal("public and event picking differ")
	}
	front.SetVisible(false)
	if Pick(root, point) != back {
		t.Fatal("hidden front child picked")
	}
	if Pick(root, geometry.Point{X: 55, Y: 10}) != nil {
		t.Fatal("ancestor clipping ignored")
	}
	if Pick(root, geometry.Point{X: 1, Y: 1}) != root {
		t.Fatal("background did not return actual Widget")
	}
}

func TestHeaderBarHeightQueryChoosesOuterTopSegment(t *testing.T) {
	for _, nativeButtons := range []bool{false, true} {
		win, _ := chromeFixture(t, nativeButtons, !nativeButtons)
		root := NewLinearBox(layout.DirectionVertical)
		root.SetCrossAlign(layout.CrossStretch)
		row := NewLinearBox(layout.DirectionHorizontal)
		row.SetCrossAlign(layout.CrossStart)
		left, right, lower := NewHeaderBar(), NewHeaderBar(), NewHeaderBar()
		left.SetMinSize(geometry.Size{Height: 64})
		right.SetMinSize(geometry.Size{Height: 96})
		lower.SetMinSize(geometry.Size{Height: 128})
		left.SetMainWeight(1)
		right.SetMainWeight(1)
		row.AddChild(left)
		row.AddChild(right)
		root.AddChild(row)
		root.AddChild(lower) // connected last; must not override the top row
		win.SetWidget(root)
		queries := 0
		var answer float32
		observer := win.Chrome().ConnectQueryControls(func(result *ChromeControls) {
			queries++
			answer = result.Height
		})
		win.paint()
		expected := float32(96)
		if nativeButtons {
			expected = 64
		}
		bounds := chromeInfo(win.Chrome()).ControlsBounds
		if answer != expected || bounds.Center().Y != expected/2 || queries != 1 {
			t.Fatalf("wrong height source: height=%v controls=%v queries=%v", answer, bounds, queries)
		}
		if lower.Rect().Y == 0 {
			t.Fatal("test lower header was not below the top row")
		}
		override := win.Chrome().ConnectQueryControls(func(result *ChromeControls) { result.Height = 120 })
		win.paint()
		if chromeInfo(win.Chrome()).ControlsBounds.Center().Y != 60 {
			t.Fatal("later application subscriber could not override height")
		}
		override.Disconnect()
		win.paint()
		if chromeInfo(win.Chrome()).ControlsBounds.Center().Y != expected/2 {
			t.Fatal("disconnect did not restore automatic header height")
		}
		observer.Disconnect()
	}
}

func TestCoveredHeaderDoesNotSupplyHeight(t *testing.T) {
	win, _ := chromeFixture(t, false, true)
	root := newTestWidget()
	root.SetLayoutManager(layout.NewFillLayout())
	header := NewHeaderBar()
	root.AddChild(header)
	root.AddChild(newTestWidget()) // covers the header at the controls anchor
	win.SetWidget(root)
	answer := float32(-1)
	win.Chrome().ConnectQueryControls(func(result *ChromeControls) { answer = result.Height })
	win.paint()
	if answer != 0 {
		t.Fatal("covered header supplied a height")
	}
}

type headerLayoutProbe struct {
	WidgetBase
	arranges int
}

func (p *headerLayoutProbe) Arrange(rect geometry.Rectangle) {
	p.arranges++
	p.WidgetBase.Arrange(rect)
}

func TestHeaderControlsReservationUsesBoundedLayout(t *testing.T) {
	win, _ := chromeFixture(t, false, true)
	root := NewLinearBox(layout.DirectionVertical)
	root.SetCrossAlign(layout.CrossStretch)
	header := NewHeaderBar()
	child := &headerLayoutProbe{}
	child.SetMinSize(geometry.Size{Height: 16})
	header.SetChild(child)
	header.SetMinSize(geometry.Size{Height: 400})
	root.AddChild(header)
	win.SetWidget(root)
	win.paint()
	child.arranges = 0
	queries := 0
	win.Chrome().ConnectQueryControls(func(*ChromeControls) { queries++ })
	header.SetMinSize(geometry.Size{Height: 32})
	win.paint()
	// Full-height buttons keep the same horizontal reservation while the
	// header shrinks, so no second content layout is necessary.
	if queries != 1 || child.arranges != 1 || win.layoutDirty {
		t.Fatalf("unbounded or missing reservation pass: queries=%d arranges=%d dirty=%v", queries, child.arranges, win.layoutDirty)
	}
	if !emptyRect(child.windowRect().Intersect(win.chrome.info.ControlsBounds)) {
		t.Fatal("new reservation was not applied in the same frame")
	}
	win.paint()
	if child.arranges != 1 || queries != 2 || win.layoutDirty {
		t.Fatal("stable controls geometry kept invalidating layout")
	}
}

func TestHeaderHeightQueryAfterWindowShrink(t *testing.T) {
	win, _ := chromeFixture(t, false, true)
	header := NewHeaderBar()
	win.SetWidget(header) // actual height follows the client, not a preference
	win.paint()
	_ = win.DispatchEvent(events.SizeEvent{Width: 640, Height: 80})
	win.paint()
	if win.chrome.info.ControlsBounds.Center().Y != 40 {
		t.Fatal("a clipped old controls region prevented the new height query")
	}
}
