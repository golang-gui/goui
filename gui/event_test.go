package gui

import (
	"fmt"
	"testing"

	"github.com/golang-gui/goui/core/geometry"
	"github.com/golang-gui/goui/layout"
	"github.com/golang-gui/goui/platform/events"
)

func TestEventDispatcherTabOrder(t *testing.T) {
	win := &window{}
	root, first, group, nested, hidden, last := newTestWidget(), newTestWidget(), newTestWidget(), newTestWidget(), newTestWidget(), newTestWidget()
	root.AddChild(first)
	root.AddChild(group)
	group.AddChild(nested)
	root.AddChild(hidden)
	hidden.AddChild(newTestWidget())
	root.AddChild(last)
	for _, w := range []Widget{first, nested, hidden, hidden.Children()[0], last} {
		w.SetFocusable(true)
	}
	hidden.SetVisible(false)
	win.SetWidget(root)
	t.Cleanup(func() { win.SetWidget(nil) })

	press := func(mods events.Modifiers, want Widget) {
		t.Helper()
		handled := false
		_ = win.DispatchEvent(events.KeyEvent{EventType: events.KeyDown, Key: events.KeyTab, Modifiers: mods, Handled: &handled})
		if !handled || win.FocusedWidget() != want || !want.Focused() || !root.ContainsFocus() {
			t.Fatalf("Tab(%v): focus=%p want=%p handled=%v", mods, win.FocusedWidget(), want, handled)
		}
		if !win.Snapshot().Widget.ContainsFocus {
			t.Fatal("focus missing from semantic snapshot")
		}
	}
	press(0, first)
	press(0, nested) // A non-focusable parent does not exclude its descendants.
	press(0, last)
	press(0, first) // Wrap within this host.
	press(events.ModifierShift, last)
	press(events.ModifierShift, nested)
	_ = win.SetFocusedWidget(nil)
	press(events.ModifierShift, last)

	// Candidates are rebuilt from the current tree, not a persistent chain.
	last.SetFocusable(false)
	group.SetVisible(false)
	press(0, first)
	root.MoveChildBefore(last, first)
	last.SetFocusable(true)
	press(events.ModifierShift, last)
	root.RemoveChild(last)
	press(0, first)
	group.SetVisible(true)
	group.SetFocusable(true)
	press(0, group)
	press(0, nested) // Focusable containers also retain child traversal.
}

func TestEventDispatcherTabFallback(t *testing.T) {
	for _, test := range []struct {
		name           string
		event          events.KeyEvent
		initialHandled bool
	}{
		{"keyup", events.KeyEvent{EventType: events.KeyUp, Key: events.KeyTab}, false},
		{"control", shortcutPress(events.KeyTab, events.ModifierControl), false},
		{"alt", shortcutPress(events.KeyTab, events.ModifierAlt), false},
		{"command", shortcutPress(events.KeyTab, events.ModifierCommand), false},
		{"option", shortcutPress(events.KeyTab, events.ModifierOption), false},
		{"win", shortcutPress(events.KeyTab, events.ModifierWin), false},
		{"super", shortcutPress(events.KeyTab, events.ModifierSuper), false},
		{"altgraph", shortcutPress(events.KeyTab, events.ModifierAltGraph), false},
		{"other", shortcutPress(events.KeyEnter, 0), false},
		{"handled", shortcutPress(events.KeyTab, 0), true},
	} {
		t.Run(test.name, func(t *testing.T) {
			root, child := newTestWidget(), newTestWidget()
			child.SetFocusable(true)
			root.AddChild(child)
			win := &window{}
			win.SetWidget(root)
			handled := test.initialHandled
			test.event.Handled = &handled
			_ = win.DispatchEvent(test.event)
			if win.FocusedWidget() != nil || handled != test.initialHandled || win.focusVisible {
				t.Fatal("non-navigation key changed focus or native default")
			}
		})
	}
	for _, phase := range []PropagationPhase{PhaseCapture, PhaseTarget, PhaseBubble} {
		for _, mode := range []string{"controller", "shortcut", "prevent-default"} {
			t.Run(fmt.Sprintf("%s/%d", mode, phase), func(t *testing.T) {
				root, child := newTestWidget(), newTestWidget()
				child.SetFocusable(true)
				root.AddChild(child)
				win := &window{}
				win.SetWidget(root)
				calls := 0
				if mode == "shortcut" {
					s := NewShortcut(KeyGesture{Key: KeyTab})
					s.ConnectActivate(func() { calls++ })
					win.Shortcuts().SetPhase(phase)
					win.Shortcuts().AddShortcut(s)
				} else {
					root.AddEventController(newRecordingController("consumer", phase, new([]string), func(ctx EventContext) {
						calls++
						if mode == "controller" {
							ctx.StopPropagation()
						} else {
							ctx.Event().(events.KeyEvent).PreventDefault()
						}
					}))
				}
				handled := false
				e := shortcutPress(events.KeyTab, 0)
				e.Handled = &handled
				_ = win.DispatchEvent(e)
				if calls != 1 || win.FocusedWidget() != nil || win.focusVisible {
					t.Fatal("default navigation ran before input consumer")
				}
			})
		}
	}
}

func TestEventDispatcherTabEmptyAndSingle(t *testing.T) {
	root := newTestWidget()
	win := &window{}
	win.SetWidget(root)
	handled := false
	e := shortcutPress(events.KeyTab, 0)
	e.Handled = &handled
	_ = win.DispatchEvent(e)
	if handled || win.FocusedWidget() != nil || win.focusVisible {
		t.Fatal("empty focus list consumed Tab")
	}
	root.SetFocusable(true)
	changes := 0
	root.ConnectFocused(func(bool) { changes++ })
	for i := 0; i < 3; i++ {
		handled = false
		_ = win.DispatchEvent(e)
		if !handled || win.FocusedWidget() != root {
			t.Fatal("single candidate did not consume Tab")
		}
	}
	if changes != 1 {
		t.Fatalf("same focus emitted %d signals, want 1", changes)
	}
}

func TestEventDispatcherTabFocusCallbacks(t *testing.T) {
	for _, when := range []string{"leave", "enter", "contains"} {
		for _, action := range []string{"remove", "redirect", "destroy-target", "destroy-host"} {
			t.Run(when+"/"+action, func(t *testing.T) {
				root, a, branch, b, c := newTestWidget(), newTestWidget(), newTestWidget(), newTestWidget(), newTestWidget()
				root.AddChild(a)
				root.AddChild(branch)
				branch.AddChild(b)
				branch.AddChild(c)
				for _, w := range []Widget{a, b, c} {
					w.SetFocusable(true)
				}
				win := &window{}
				win.SetWidget(root)
				win.SetFocusedWidget(a)
				change := func() {
					switch action {
					case "remove":
						branch.RemoveChild(b)
					case "redirect":
						win.SetFocusedWidget(c)
					case "destroy-target":
						b.base().destroy(b)
					case "destroy-host":
						win.Destroy()
					}
				}
				switch when {
				case "leave":
					a.ConnectFocused(func(focused bool) {
						if !focused {
							change()
						}
					})
				case "enter":
					b.ConnectFocused(func(focused bool) {
						if focused {
							change()
						}
					})
				case "contains":
					b.ConnectContainsFocus(func(focused bool) {
						if focused {
							change()
						}
					})
				}
				_ = win.DispatchEvent(shortcutPress(events.KeyTab, 0))
				if pathTarget(win.dispatcher.focusPath) != win.FocusedWidget() {
					t.Fatal("dispatcher retained a superseded focus path")
				}
				if b.Focused() || a.Focused() {
					t.Fatal("stale focus notification after callback")
				}
				if action == "redirect" {
					if win.FocusedWidget() != c || !c.Focused() || !c.FocusVisible() || !branch.ContainsFocus() || !root.ContainsFocus() {
						t.Fatal("reentrant focus lost state")
					}
				} else if win.FocusedWidget() != nil || root.ContainsFocus() || branch.ContainsFocus() || b.FocusVisible() {
					t.Fatal("detached subtree retained focus")
				}
			})
		}
	}
}

func TestEventDispatcherFocusVisible(t *testing.T) {
	root, first, second := newTestWidget(), newTestWidget(), newTestWidget()
	root.AddChild(first)
	root.AddChild(second)
	for _, child := range []Widget{first, second} {
		child.SetFocusable(true)
	}
	root.Arrange(geometry.Rect(0, 0, 100, 100))
	first.Arrange(geometry.Rect(0, 0, 100, 40))
	second.Arrange(geometry.Rect(0, 50, 100, 40))
	win := &window{}
	win.SetWidget(root)
	t.Cleanup(func() { win.SetWidget(nil) })
	changes := 0
	first.ConnectFocused(func(bool) { changes++ })
	win.SetFocusedWidget(first)
	if first.FocusVisible() || first.Snapshot().FocusVisible {
		t.Fatal("initial programmatic focus enabled keyboard hints")
	}
	second.ConnectFocused(func(focused bool) {
		if focused && (!second.FocusVisible() || !second.Snapshot().FocusVisible) {
			t.Fatal("Tab callback observed the old input mode")
		}
	})
	_ = win.DispatchEvent(shortcutPress(events.KeyTab, 0))
	if !second.FocusVisible() || first.FocusVisible() || root.FocusVisible() {
		t.Fatal("keyboard hint was not limited to the focused target")
	}
	_ = win.DispatchEvent(shortcutPress(events.KeyTab, events.ModifierShift))
	if !first.FocusVisible() || second.FocusVisible() {
		t.Fatal("reverse navigation lost keyboard hints")
	}
	win.SetFocusedWidget(nil)
	if first.FocusVisible() || second.FocusVisible() {
		t.Fatal("cleared focus kept a visible hint")
	}
	win.SetFocusedWidget(first)
	if !first.FocusVisible() {
		t.Fatal("programmatic focus did not preserve keyboard mode")
	}
	before := changes
	_ = win.DispatchEvent(events.PointerEvent{EventType: events.PointerMove, Position: geometry.Point{X: 10, Y: 10}})
	if !first.FocusVisible() {
		t.Fatal("pointer movement hid the keyboard hint")
	}
	win.paintDirty, win.layoutDirty = false, false
	_ = win.DispatchEvent(events.PointerEvent{EventType: events.PointerDown, Position: geometry.Point{X: 10, Y: 10}, Button: events.PointerButtonLeft})
	if first.FocusVisible() || first.Snapshot().FocusVisible || !first.Focused() || changes != before {
		t.Fatal("pointer press did not hide hints independently of logical focus")
	}
	if !win.paintDirty || win.layoutDirty {
		t.Fatal("hint-only change must repaint without requesting layout")
	}
	// The same candidate still enables hints, without repeating Focused signals.
	second.SetFocusable(false)
	_ = win.DispatchEvent(shortcutPress(events.KeyTab, 0))
	if !first.FocusVisible() || changes != before {
		t.Fatal("single-candidate Tab did not enable hints independently")
	}
	root.RemoveChild(first)
	if first.FocusVisible() || first.Snapshot().FocusVisible {
		t.Fatal("detached widget retained keyboard hints")
	}
}

func TestEventDispatcherTabDecorationTree(t *testing.T) {
	win := &window{}
	decoration := newTestWidget()
	decoration.SetFocusable(true)
	adoptWidget(decoration, win)
	win.dispatcher.decoration = decoration
	_ = win.DispatchEvent(shortcutPress(events.KeyTab, 0))
	if win.FocusedWidget() != decoration || !decoration.Focused() {
		t.Fatal("decoration-only host could not focus")
	}
	root := newTestWidget()
	root.SetFocusable(true)
	win.SetWidget(root)
	_ = win.DispatchEvent(shortcutPress(events.KeyTab, 0))
	if win.FocusedWidget() != root || decoration.Focused() {
		t.Fatal("decoration did not wrap to content")
	}
	_ = win.DispatchEvent(shortcutPress(events.KeyTab, 0))
	if win.FocusedWidget() != decoration || root.Focused() {
		t.Fatal("content did not visit decoration")
	}
}

func TestEventDispatcherTabRevealsNestedScrollViews(t *testing.T) {
	outer, inner := NewScrollView(), NewScrollView()
	outerContent := &mockWidget{size: geometry.Size{Width: 300, Height: 600}}
	innerContent := &mockWidget{size: geometry.Size{Width: 200, Height: 400}}
	first, last := newTestWidget(), newTestWidget()
	first.SetFocusable(true)
	last.SetFocusable(true)
	innerContent.WidgetBase.AddChild(innerContent, first)
	innerContent.WidgetBase.AddChild(innerContent, last)
	first.Arrange(geometry.Rect(0, 0, 30, 20))
	last.Arrange(geometry.Rect(150, 350, 30, 20))
	inner.SetChild(innerContent)
	outerContent.WidgetBase.AddChild(outerContent, inner)
	outer.SetChild(outerContent)
	win := &window{}
	win.SetWidget(outer)
	outer.Measure(layout.Loose(geometry.Size{Width: 120, Height: 100}))
	outer.Arrange(geometry.Rect(0, 0, 120, 100))
	inner.Measure(layout.Loose(geometry.Size{Width: 100, Height: 80}))
	inner.Arrange(geometry.Rect(170, 400, 100, 80))
	win.SetFocusedWidget(first)
	_ = win.DispatchEvent(shortcutPress(events.KeyTab, 0))
	// Independent expected geometry, including the 8 DIP scrollbar space:
	// inner viewport 92x72, outer viewport 112x92, target size 30x20.
	if inner.ScrollX() != 88 || inner.ScrollY() != 298 || outer.ScrollX() != 150 || outer.ScrollY() != 380 {
		t.Fatalf("inner=(%g,%g), outer=(%g,%g)", inner.ScrollX(), inner.ScrollY(), outer.ScrollX(), outer.ScrollY())
	}
	if win.FocusedWidget() != last || !last.Focused() {
		t.Fatal("reveal changed focus")
	}
	_ = win.DispatchEvent(shortcutPress(events.KeyTab, events.ModifierShift))
	if win.FocusedWidget() != first || inner.ScrollX() != 0 || inner.ScrollY() != 0 || outer.ScrollX() != 150 || outer.ScrollY() != 380 {
		t.Fatal("reverse traversal did not reveal the first nested field minimally")
	}
}

type tabScrollableContent struct {
	mockScrollContent
	target Widget
}

func (m *tabScrollableContent) LayoutVisible(viewport geometry.Size, offset geometry.Point) {
	m.mockScrollContent.LayoutVisible(viewport, offset)
	m.target.Arrange(geometry.Rect(-offset.X, 350-offset.Y, 30, 20))
}

func TestEventDispatcherTabRevealsScrollableChild(t *testing.T) {
	child := newTestWidget()
	child.SetFocusable(true)
	content := &tabScrollableContent{mockScrollContent: mockScrollContent{height: 500}, target: child}
	content.WidgetBase.AddChild(content, child)
	scroll := NewScrollView()
	scroll.SetChild(content)
	win := &window{}
	win.SetWidget(scroll)
	scroll.Measure(layout.Loose(geometry.Size{Width: 200, Height: 100}))
	scroll.Arrange(geometry.Rect(0, 0, 200, 100))
	scroll.SetScrollY(80)
	_ = win.DispatchEvent(shortcutPress(events.KeyTab, 0))
	// The field's unscrolled bottom is 370, and the viewport is 100 DIP.
	if win.FocusedWidget() != child || scroll.ScrollY() != 270 || child.Rect().Y != 80 {
		t.Fatalf("focus=%p scroll=%g child=%+v", win.FocusedWidget(), scroll.ScrollY(), child.Rect())
	}
}

func TestEventDispatcherDispatchesPointerEventThroughThreePhases(t *testing.T) {
	root := newTestWidget()
	parent := newTestWidget()
	target := newTestWidget()
	root.SetID("root")
	parent.SetID("parent")
	target.SetID("target")
	root.Arrange(geometry.Rect(0, 0, 100, 100))
	parent.Arrange(geometry.Rect(10, 10, 80, 80))
	target.Arrange(geometry.Rect(5, 5, 30, 30))
	root.AddChild(parent)
	parent.AddChild(target)

	var calls []string
	root.AddEventController(newRecordingController("root-capture", PhaseCapture, &calls, nil))
	parent.AddEventController(newRecordingController("parent-capture", PhaseCapture, &calls, nil))
	target.AddEventController(newRecordingController("target-capture", PhaseCapture, &calls, nil))
	target.AddEventController(newRecordingController("target", PhaseTarget, &calls, nil))
	target.AddEventController(newRecordingController("target-bubble", PhaseBubble, &calls, nil))
	parent.AddEventController(newRecordingController("parent-bubble", PhaseBubble, &calls, nil))
	root.AddEventController(newRecordingController("root-bubble", PhaseBubble, &calls, nil))

	win := &window{root: root}
	event := events.PointerEvent{
		EventType: events.PointerDown,
		Position:  geometry.Point{X: 20, Y: 20},
	}

	if err := win.DispatchEvent(event); err != nil {
		t.Fatal(err)
	}

	want := []string{
		"root-capture phase=0 type=6",
		"parent-capture phase=0 type=6",
		"target-capture phase=0 type=6",
		"target phase=1 type=6",
		"target-bubble phase=2 type=6",
		"parent-bubble phase=2 type=6",
		"root-bubble phase=2 type=6",
	}
	assertStrings(t, calls, want)
}

func TestEventDispatcherStopsPropagation(t *testing.T) {
	root := newTestWidget()
	parent := newTestWidget()
	target := newTestWidget()
	root.SetID("root")
	parent.SetID("parent")
	target.SetID("target")
	root.Arrange(geometry.Rect(0, 0, 100, 100))
	parent.Arrange(geometry.Rect(10, 10, 80, 80))
	target.Arrange(geometry.Rect(5, 5, 30, 30))
	root.AddChild(parent)
	parent.AddChild(target)

	var calls []string
	root.AddEventController(newRecordingController("root-capture", PhaseCapture, &calls, nil))
	parent.AddEventController(newRecordingController("parent-capture", PhaseCapture, &calls, func(ctx EventContext) {
		ctx.StopPropagation()
	}))
	target.AddEventController(newRecordingController("target", PhaseTarget, &calls, nil))
	root.AddEventController(newRecordingController("root-bubble", PhaseBubble, &calls, nil))

	win := &window{root: root}
	event := events.PointerEvent{
		EventType: events.PointerDown,
		Position:  geometry.Point{X: 20, Y: 20},
	}

	if err := win.DispatchEvent(event); err != nil {
		t.Fatal(err)
	}

	want := []string{
		"root-capture phase=0 type=6",
		"parent-capture phase=0 type=6",
	}
	assertStrings(t, calls, want)
}

func TestEventDispatcherHitTestUsesLastVisibleChild(t *testing.T) {
	root := newTestWidget()
	bottom := newTestWidget()
	top := newTestWidget()
	root.SetID("root")
	bottom.SetID("bottom")
	top.SetID("top")
	root.Arrange(geometry.Rect(0, 0, 100, 100))
	bottom.Arrange(geometry.Rect(10, 10, 40, 40))
	top.Arrange(geometry.Rect(10, 10, 40, 40))
	root.AddChild(bottom)
	root.AddChild(top)

	var calls []string
	bottom.AddEventController(newRecordingController("bottom", PhaseTarget, &calls, nil))
	top.AddEventController(newRecordingController("top", PhaseTarget, &calls, nil))

	win := &window{root: root}
	event := events.PointerEvent{
		EventType: events.PointerDown,
		Position:  geometry.Point{X: 20, Y: 20},
	}

	if err := win.DispatchEvent(event); err != nil {
		t.Fatal(err)
	}

	assertStrings(t, calls, []string{
		"top phase=1 type=6",
	})

	top.SetVisible(false)
	calls = nil

	if err := win.DispatchEvent(event); err != nil {
		t.Fatal(err)
	}

	assertStrings(t, calls, []string{
		"bottom phase=1 type=6",
	})
}

func TestEventDispatcherDispatchesWheelByPosition(t *testing.T) {
	root := newTestWidget()
	child := newTestWidget()
	root.SetID("root")
	child.SetID("child")
	root.Arrange(geometry.Rect(0, 0, 100, 100))
	child.Arrange(geometry.Rect(10, 10, 40, 40))
	root.AddChild(child)

	var calls []string
	child.AddEventController(newRecordingController("child", PhaseTarget, &calls, nil))

	win := &window{root: root}
	event := events.WheelEvent{
		Position: geometry.Point{X: 20, Y: 20},
		DeltaY:   -1,
		Mode:     events.WheelDeltaLine,
	}

	if err := win.DispatchEvent(event); err != nil {
		t.Fatal(err)
	}

	assertStrings(t, calls, []string{
		"child phase=1 type=8",
	})
}

func TestEventContextPositionIsLocalToCurrentWidget(t *testing.T) {
	root := newTestWidget()
	child := newTestWidget()
	root.Arrange(geometry.Rect(0, 0, 100, 100))
	child.Arrange(geometry.Rect(10, 20, 40, 40))
	root.AddChild(child)

	var positions []geometry.Point
	child.AddEventController(newRecordingController("child", PhaseTarget, new([]string), func(ctx EventContext) {
		position, ok := ctx.Position()
		if !ok {
			t.Fatal("pointer event should provide a position")
		}
		positions = append(positions, position)
	}))

	win := &window{root: root}
	if err := win.DispatchEvent(events.PointerEvent{
		EventType: events.PointerDown,
		Position:  geometry.Point{X: 15, Y: 27},
	}); err != nil {
		t.Fatal(err)
	}

	if len(positions) != 1 || positions[0] != (geometry.Point{X: 5, Y: 7}) {
		t.Fatalf("unexpected local positions: %+v", positions)
	}
}

func TestEventContextPositionIsAbsentForKeyEvent(t *testing.T) {
	root := newTestWidget()
	root.Arrange(geometry.Rect(0, 0, 100, 100))

	root.AddEventController(newRecordingController("root", PhaseTarget, new([]string), func(ctx EventContext) {
		if _, ok := ctx.Position(); ok {
			t.Fatal("key event should not provide a position")
		}
	}))

	win := &window{root: root}
	if err := win.DispatchEvent(events.KeyEvent{EventType: events.KeyDown}); err != nil {
		t.Fatal(err)
	}
}

func TestEventDispatcherDoesNotPropagatePointerCrossingAsPlatformEvents(t *testing.T) {
	root := newTestWidget()
	first := newTestWidget()
	second := newTestWidget()
	root.SetID("root")
	first.SetID("first")
	second.SetID("second")
	root.Arrange(geometry.Rect(0, 0, 100, 100))
	first.Arrange(geometry.Rect(0, 0, 40, 40))
	second.Arrange(geometry.Rect(50, 0, 40, 40))
	root.AddChild(first)
	root.AddChild(second)

	var calls []string
	root.AddEventController(newRecordingController("root", PhaseTarget, &calls, nil))
	first.AddEventController(newRecordingController("first", PhaseTarget, &calls, nil))
	second.AddEventController(newRecordingController("second", PhaseTarget, &calls, nil))

	win := &window{root: root}
	if err := win.DispatchEvent(events.PointerEvent{
		EventType: events.PointerMove,
		Position:  geometry.Point{X: 10, Y: 10},
	}); err != nil {
		t.Fatal(err)
	}

	assertStrings(t, calls, []string{
		"first phase=1 type=5",
	})

	calls = nil
	if err := win.DispatchEvent(events.PointerEvent{
		EventType: events.PointerMove,
		Position:  geometry.Point{X: 60, Y: 10},
	}); err != nil {
		t.Fatal(err)
	}

	assertStrings(t, calls, []string{
		"second phase=1 type=5",
	})

	calls = nil
	if err := win.DispatchEvent(events.PointerEvent{
		EventType: events.PointerLeave,
		Position:  geometry.Point{X: 60, Y: 10},
	}); err != nil {
		t.Fatal(err)
	}

	assertStrings(t, calls, nil)
}

func TestEventDispatcherUpdatesMotionHoverStates(t *testing.T) {
	root := newTestWidget()
	parent := newTestWidget()
	child := newTestWidget()
	root.Arrange(geometry.Rect(0, 0, 100, 100))
	parent.Arrange(geometry.Rect(10, 10, 80, 80))
	child.Arrange(geometry.Rect(10, 10, 20, 20))
	root.AddChild(parent)
	parent.AddChild(child)

	parentMotion := NewMotionEventController()
	childMotion := NewMotionEventController()
	parent.AddEventController(parentMotion)
	child.AddEventController(childMotion)

	win := &window{root: root}
	if err := win.DispatchEvent(events.PointerEvent{
		EventType: events.PointerMove,
		Position:  geometry.Point{X: 15, Y: 15},
	}); err != nil {
		t.Fatal(err)
	}
	if !parentMotion.Hover() || !parentMotion.ContainsHover() {
		t.Fatalf("parent should be directly hovered: is=%v contains=%v", parentMotion.Hover(), parentMotion.ContainsHover())
	}
	if childMotion.Hover() || childMotion.ContainsHover() {
		t.Fatalf("child should not be hovered: is=%v contains=%v", childMotion.Hover(), childMotion.ContainsHover())
	}

	if err := win.DispatchEvent(events.PointerEvent{
		EventType: events.PointerMove,
		Position:  geometry.Point{X: 25, Y: 25},
	}); err != nil {
		t.Fatal(err)
	}
	if parentMotion.Hover() || !parentMotion.ContainsHover() {
		t.Fatalf("parent should contain hover through child: is=%v contains=%v", parentMotion.Hover(), parentMotion.ContainsHover())
	}
	if !childMotion.Hover() || !childMotion.ContainsHover() {
		t.Fatalf("child should be directly hovered: is=%v contains=%v", childMotion.Hover(), childMotion.ContainsHover())
	}

	if err := win.DispatchEvent(events.PointerEvent{
		EventType: events.PointerMove,
		Position:  geometry.Point{X: 95, Y: 95},
	}); err != nil {
		t.Fatal(err)
	}
	if parentMotion.Hover() || parentMotion.ContainsHover() || childMotion.Hover() || childMotion.ContainsHover() {
		t.Fatalf("hover states were not cleared: parent is=%v contains=%v child is=%v contains=%v",
			parentMotion.Hover(), parentMotion.ContainsHover(), childMotion.Hover(), childMotion.ContainsHover())
	}
}

func TestEventDispatcherHoverStateIgnoresStoppedPointerMove(t *testing.T) {
	root := newTestWidget()
	child := newTestWidget()
	root.Arrange(geometry.Rect(0, 0, 100, 100))
	child.Arrange(geometry.Rect(10, 10, 40, 40))
	root.AddChild(child)

	motion := NewMotionEventController()
	child.AddEventController(motion)
	child.AddEventController(newRecordingController("stop", PhaseTarget, new([]string), func(ctx EventContext) {
		ctx.StopPropagation()
	}))

	win := &window{root: root}
	if err := win.DispatchEvent(events.PointerEvent{
		EventType: events.PointerMove,
		Position:  geometry.Point{X: 20, Y: 20},
	}); err != nil {
		t.Fatal(err)
	}
	if !motion.Hover() || !motion.ContainsHover() {
		t.Fatalf("hover state should update before propagation stop: is=%v contains=%v", motion.Hover(), motion.ContainsHover())
	}
}

func TestEventDispatcherDispatchesKeyToFocusedWidget(t *testing.T) {
	root := newTestWidget()
	child := newTestWidget()
	root.SetID("root")
	child.SetID("child")
	root.Arrange(geometry.Rect(0, 0, 100, 100))
	child.Arrange(geometry.Rect(10, 10, 40, 40))
	root.AddChild(child)
	child.SetFocusable(true)

	var calls []string
	root.AddEventController(newRecordingController("root", PhaseTarget, &calls, nil))
	child.AddEventController(newRecordingController("child", PhaseTarget, &calls, nil))

	win := &window{}
	win.SetWidget(root)
	win.SetFocusedWidget(child)
	event := events.KeyEvent{
		EventType: events.KeyDown,
		Key:       events.KeyA,
	}

	if err := win.DispatchEvent(event); err != nil {
		t.Fatal(err)
	}

	assertStrings(t, calls, []string{
		"child phase=1 type=9",
	})
}

func TestEventDispatcherDispatchesKeyToRootWithoutFocusedWidget(t *testing.T) {
	root := newTestWidget()
	child := newTestWidget()
	root.SetID("root")
	child.SetID("child")
	root.Arrange(geometry.Rect(0, 0, 100, 100))
	child.Arrange(geometry.Rect(10, 10, 40, 40))
	root.AddChild(child)

	var calls []string
	root.AddEventController(newRecordingController("root", PhaseTarget, &calls, nil))
	child.AddEventController(newRecordingController("child", PhaseTarget, &calls, nil))

	win := &window{root: root}
	event := events.KeyEvent{
		EventType: events.KeyDown,
		Key:       events.KeyA,
	}

	if err := win.DispatchEvent(event); err != nil {
		t.Fatal(err)
	}

	assertStrings(t, calls, []string{
		"root phase=1 type=9",
	})
}

func TestEventDispatcherDispatchesFocusCrossing(t *testing.T) {
	root := newTestWidget()
	parent := newTestWidget()
	child := newTestWidget()
	root.SetID("root")
	parent.SetID("parent")
	child.SetID("child")
	child.SetFocusable(true)
	root.AddChild(parent)
	parent.AddChild(child)

	var calls []string
	root.AddEventController(newCrossingRecordingController("root", &calls))
	parent.AddEventController(newCrossingRecordingController("parent", &calls))
	child.AddEventController(newCrossingRecordingController("child", &calls))

	win := &window{}
	win.SetWidget(root)
	if !win.SetFocusedWidget(child) {
		t.Fatal("focusable child should accept focus")
	}

	assertStrings(t, calls, []string{
		"root type=1 mode=1 direction=0 pos=false",
		"parent type=1 mode=1 direction=0 pos=false",
		"child type=1 mode=1 direction=0 pos=false",
		"child type=1 mode=0 direction=0 pos=false",
	})
	if root.Focused() || !root.ContainsFocus() || parent.Focused() || !parent.ContainsFocus() || !child.Focused() || !child.ContainsFocus() {
		t.Fatalf("unexpected focus state: root=%v/%v parent=%v/%v child=%v/%v",
			root.Focused(), root.ContainsFocus(), parent.Focused(), parent.ContainsFocus(), child.Focused(), child.ContainsFocus())
	}

	calls = nil
	win.SetFocusedWidget(nil)
	assertStrings(t, calls, []string{
		"child type=1 mode=0 direction=1 pos=false",
		"child type=1 mode=1 direction=1 pos=false",
		"parent type=1 mode=1 direction=1 pos=false",
		"root type=1 mode=1 direction=1 pos=false",
	})
}

func TestEventDispatcherPointerDownFocusesNearestFocusableWidget(t *testing.T) {
	root := newTestWidget()
	parent := newTestWidget()
	child := newTestWidget()
	root.Arrange(geometry.Rect(0, 0, 100, 100))
	parent.Arrange(geometry.Rect(10, 10, 80, 80))
	child.Arrange(geometry.Rect(5, 5, 30, 30))
	root.AddChild(parent)
	parent.AddChild(child)
	parent.SetFocusable(true)

	win := &window{}
	win.SetWidget(root)

	if err := win.DispatchEvent(events.PointerEvent{
		EventType: events.PointerDown,
		Position:  geometry.Point{X: 20, Y: 20},
	}); err != nil {
		t.Fatal(err)
	}

	if win.FocusedWidget() != parent {
		t.Fatalf("pointer down focused %v, want parent", win.FocusedWidget())
	}
	if !parent.Focused() {
		t.Fatal("parent focused state was not set")
	}

	if err := win.DispatchEvent(events.PointerEvent{
		EventType: events.PointerDown,
		Position:  geometry.Point{X: 1, Y: 1},
	}); err != nil {
		t.Fatal(err)
	}
	if win.FocusedWidget() != parent {
		t.Fatalf("click on a non-focusable target should leave focus unchanged, got %v (want parent)", win.FocusedWidget())
	}
}

func TestEventDispatcherNonFocusableTargetLeavesFocusForMenuBarScenario(t *testing.T) {
	// Regression: a menu-bar row of MenuButtons with Focusable(false) must not
	// steal focus from an editable field (e.g. a TextInput) when clicked to open
	// a menu. GTK4/Flutter semantics: a click on a non-focusable surface does not
	// clear or move window focus; the popover routes keyboard via the modal target.
	win := &window{}
	root := newTestWidget()
	root.Arrange(geometry.Rect(0, 0, 100, 100))
	win.SetWidget(root)

	// A real editable takes focus first.
	input := newTestWidget()
	input.Arrange(geometry.Rect(10, 10, 40, 20))
	input.SetFocusable(true)
	root.AddChild(input)
	if !win.SetFocusedWidget(input) {
		t.Fatal("setup: input did not take focus")
	}
	if win.FocusedWidget() != input {
		t.Fatal("setup: input is not the focused widget")
	}

	// Clicking a non-focusable button (the menu row) must leave input focused.
	button := newTestWidget()
	button.Arrange(geometry.Rect(10, 40, 40, 20))
	button.SetFocusable(false)
	root.AddChild(button)

	if err := win.DispatchEvent(events.PointerEvent{
		EventType: events.PointerDown,
		Position:  geometry.Point{X: 20, Y: 50},
	}); err != nil {
		t.Fatal(err)
	}
	if win.FocusedWidget() != input {
		t.Fatalf("clicking a non-focusable menu button stole focus: got %v want input", win.FocusedWidget())
	}
}

func TestEventDispatcherIgnoresEventsWithoutTarget(t *testing.T) {
	root := newTestWidget()
	root.Arrange(geometry.Rect(0, 0, 100, 100))

	var calls []string
	root.AddEventController(newRecordingController("root", PhaseTarget, &calls, nil))

	win := &window{root: root}
	event := events.PointerEvent{
		EventType: events.PointerDown,
		Position:  geometry.Point{X: 120, Y: 20},
	}

	if err := win.DispatchEvent(event); err != nil {
		t.Fatal(err)
	}
	if len(calls) != 0 {
		t.Fatalf("unexpected calls: %v", calls)
	}
}

type recordingController struct {
	name   string
	phase  PropagationPhase
	calls  *[]string
	handle func(ctx EventContext)
}

func newRecordingController(name string, phase PropagationPhase, calls *[]string, handle func(ctx EventContext)) *recordingController {
	return &recordingController{
		name:   name,
		phase:  phase,
		calls:  calls,
		handle: handle,
	}
}

func (c *recordingController) Phase() PropagationPhase {
	return c.phase
}

func (c *recordingController) Reset() {}

func (c *recordingController) HandleEvent(ctx EventContext) {
	*c.calls = append(*c.calls, fmt.Sprintf(
		"%s phase=%d type=%d",
		c.name,
		c.phase,
		ctx.Event().Type(),
	))
	if c.handle != nil {
		c.handle(ctx)
	}
}

func (c *recordingController) HandleCrossing(ctx CrossingContext) {}

type crossingRecordingController struct {
	name  string
	calls *[]string
}

func newCrossingRecordingController(name string, calls *[]string) *crossingRecordingController {
	return &crossingRecordingController{name: name, calls: calls}
}

func (c *crossingRecordingController) Phase() PropagationPhase {
	return PhaseTarget
}

func (c *crossingRecordingController) Reset() {}

func (c *crossingRecordingController) HandleEvent(ctx EventContext) {}

func (c *crossingRecordingController) HandleCrossing(ctx CrossingContext) {
	_, hasPosition := ctx.Position()
	*c.calls = append(*c.calls, fmt.Sprintf(
		"%s type=%d mode=%d direction=%d pos=%v",
		c.name,
		ctx.Type(),
		ctx.Mode(),
		ctx.Direction(),
		hasPosition,
	))
}

func assertStrings(t *testing.T, got, want []string) {
	t.Helper()
	if len(got) != len(want) {
		t.Fatalf("unexpected call count:\ngot  %v\nwant %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("unexpected call %d:\ngot  %q\nwant %q\nall got: %v", i, got[i], want[i], got)
		}
	}
}

func TestEventDispatcherCaptureRoutesMoveToCapturedWidget(t *testing.T) {
	root := newTestWidget()
	left := newTestWidget()
	right := newTestWidget()
	root.SetID("root")
	left.SetID("left")
	right.SetID("right")
	root.Arrange(geometry.Rect(0, 0, 100, 100))
	left.Arrange(geometry.Rect(0, 0, 40, 100))
	right.Arrange(geometry.Rect(60, 0, 40, 100))
	root.AddChild(left)
	root.AddChild(right)

	drag := NewDragEventController()
	var updates []geometry.Point
	drag.ConnectUpdate(func(p geometry.Point, _ events.Modifiers) {
		updates = append(updates, p)
	})
	left.AddEventController(drag)

	win := &window{root: root}

	// Press on left widget → capture starts.
	if err := win.DispatchEvent(events.PointerEvent{
		EventType: events.PointerDown,
		Button:    events.PointerButtonLeft,
		Position:  geometry.Point{X: 20, Y: 50},
	}); err != nil {
		t.Fatal(err)
	}
	if !drag.Dragging() {
		t.Fatal("drag should be active after pointer down")
	}

	// Move over the right widget — should still go to left (captured).
	if err := win.DispatchEvent(events.PointerEvent{
		EventType: events.PointerMove,
		Button:    events.PointerButtonLeft,
		Position:  geometry.Point{X: 70, Y: 50},
	}); err != nil {
		t.Fatal(err)
	}
	if len(updates) != 1 {
		t.Fatalf("expected 1 drag-update on captured widget, got %d", len(updates))
	}
}

func TestEventDispatcherCaptureReleasesOnPointerUp(t *testing.T) {
	root := newTestWidget()
	left := newTestWidget()
	right := newTestWidget()
	root.Arrange(geometry.Rect(0, 0, 100, 100))
	left.Arrange(geometry.Rect(0, 0, 40, 100))
	right.Arrange(geometry.Rect(60, 0, 40, 100))
	root.AddChild(left)
	root.AddChild(right)

	drag := NewDragEventController()
	ends := 0
	drag.ConnectEnd(func(geometry.Point, events.Modifiers) { ends++ })
	left.AddEventController(drag)

	win := &window{root: root}

	// Press on left → capture.
	_ = win.DispatchEvent(events.PointerEvent{
		EventType: events.PointerDown,
		Button:    events.PointerButtonLeft,
		Position:  geometry.Point{X: 20, Y: 50},
	})

	// Move over right (captured).
	_ = win.DispatchEvent(events.PointerEvent{
		EventType: events.PointerMove,
		Button:    events.PointerButtonLeft,
		Position:  geometry.Point{X: 70, Y: 50},
	})

	// Release over right.
	_ = win.DispatchEvent(events.PointerEvent{
		EventType: events.PointerUp,
		Button:    events.PointerButtonLeft,
		Position:  geometry.Point{X: 70, Y: 50},
	})
	if ends != 1 {
		t.Fatalf("expected 1 drag-end, got %d", ends)
	}

	// After release, subsequent move should go through normal hit test (to right).
	var calls []string
	right.AddEventController(newRecordingController("right", PhaseTarget, &calls, nil))
	_ = win.DispatchEvent(events.PointerEvent{
		EventType: events.PointerMove,
		Position:  geometry.Point{X: 70, Y: 50},
	})

	assertStrings(t, calls, []string{"right phase=1 type=5"})
}

func TestEventDispatcherCaptureClearsOnWidgetHide(t *testing.T) {
	root := newTestWidget()
	left := newTestWidget()
	right := newTestWidget()
	root.Arrange(geometry.Rect(0, 0, 100, 100))
	left.Arrange(geometry.Rect(0, 0, 40, 100))
	right.Arrange(geometry.Rect(60, 0, 40, 100))
	root.AddChild(left)
	root.AddChild(right)

	drag := NewDragEventController()
	left.AddEventController(drag)

	win := &window{root: root}

	// Press on left → capture.
	_ = win.DispatchEvent(events.PointerEvent{
		EventType: events.PointerDown,
		Button:    events.PointerButtonLeft,
		Position:  geometry.Point{X: 20, Y: 50},
	})
	if win.dispatcher.captureTarget != left {
		t.Fatal("captureTarget should be set to left")
	}

	// Hide the captured widget → capture should clear.
	left.SetVisible(false)
	_ = win.DispatchEvent(events.PointerEvent{
		EventType: events.PointerMove,
		Button:    events.PointerButtonLeft,
		Position:  geometry.Point{X: 70, Y: 50},
	})
	if win.dispatcher.captureTarget != nil {
		t.Fatal("captureTarget should be cleared when widget is hidden")
	}
}

func TestEventDispatcherHoverUpdatesDuringCapture(t *testing.T) {
	root := newTestWidget()
	left := newTestWidget()
	right := newTestWidget()
	root.Arrange(geometry.Rect(0, 0, 100, 100))
	left.Arrange(geometry.Rect(0, 0, 40, 100))
	right.Arrange(geometry.Rect(60, 0, 40, 100))
	root.AddChild(left)
	root.AddChild(right)

	drag := NewDragEventController()
	left.AddEventController(drag)

	rightMotion := NewMotionEventController()
	right.AddEventController(rightMotion)

	win := &window{root: root}

	// Press on left → capture.
	_ = win.DispatchEvent(events.PointerEvent{
		EventType: events.PointerDown,
		Button:    events.PointerButtonLeft,
		Position:  geometry.Point{X: 20, Y: 50},
	})

	// Move over right — drag-update goes to left (captured), but hover should
	// still update to right.
	_ = win.DispatchEvent(events.PointerEvent{
		EventType: events.PointerMove,
		Button:    events.PointerButtonLeft,
		Position:  geometry.Point{X: 70, Y: 50},
	})

	if !rightMotion.Hover() {
		t.Fatal("right widget should be hovered even during left's capture")
	}
}
