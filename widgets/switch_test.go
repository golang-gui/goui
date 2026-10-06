package widgets

import (
	"encoding/json"
	"image/color"
	"math"
	"testing"
	"time"

	"github.com/golang-gui/goui/core/geometry"
	"github.com/golang-gui/goui/gui"
	"github.com/golang-gui/goui/layout"
	"github.com/golang-gui/goui/platform/events"
	"github.com/golang-gui/goui/platform/graphics"
	"github.com/golang-gui/goui/platform/graphics/software"
	"github.com/golang-gui/goui/style"
)

func switchInput() (*Switch, *tabInputHost, *gui.EventDispatcher) {
	b := NewSwitch()
	b.Arrange(geometry.Rect(0, 0, 120, 32))
	return b, &tabInputHost{root: b, focus: b}, new(gui.EventDispatcher)
}

func switchKey(t *testing.T, d *gui.EventDispatcher, h *tabInputHost, key events.Key, repeat bool) {
	t.Helper()
	if err := d.DispatchEvent(h, events.KeyEvent{EventType: events.KeyDown, Key: key, Repeat: repeat}); err != nil {
		t.Fatal(err)
	}
}

func TestSwitchSettersAndInput(t *testing.T) {
	b, h, d := switchInput()
	changes := 0
	b.ConnectChange(func(checked bool) {
		if checked != b.Checked() {
			t.Fatal("uncommitted signal")
		}
		changes++
	})
	if b.Checked() || !b.Animated() || !b.Enabled() || !b.Focusable() || b.Padding() != 6 {
		t.Fatal("unexpected defaults")
	}
	b.SetChecked(true)
	b.SetChecked(false)
	if changes != 0 {
		t.Fatal("setter emitted Change")
	}
	// Track and empty noninteractive content region both use valid-release clicks.
	for _, x := range []float32{16, 90} {
		before := b.Checked()
		dispatchTabPointer(t, d, h, events.PointerDown, x, 16)
		if b.Checked() != before {
			t.Fatal("changed before release")
		}
		dispatchTabPointer(t, d, h, events.PointerUp, x, 16)
	}
	if b.Checked() || changes != 2 {
		t.Fatal("click sequence incorrect")
	}
	// A press starting outside the track cannot turn into a switch drag.
	dispatchTabPointer(t, d, h, events.PointerDown, 90, 16)
	dispatchTabPointer(t, d, h, events.PointerMove, 150, 16)
	dispatchTabPointer(t, d, h, events.PointerUp, 150, 16)
	if b.Checked() || changes != 2 || b.pressed {
		t.Fatal("outside release activated")
	}
	switchKey(t, d, h, events.KeySpace, false)
	switchKey(t, d, h, events.KeySpace, true)
	if !b.Checked() || changes != 3 {
		t.Fatal("key repeat toggled")
	}
	switchKey(t, d, h, events.KeyEnter, false)
	if b.Checked() || changes != 4 {
		t.Fatal("Enter failed")
	}
	b.SetEnabled(false)
	switchKey(t, d, h, events.KeySpace, false)
	if b.Checked() || !b.Focusable() || gui.IsEnabled(b) || changes != 4 {
		t.Fatal("disabled switch activated")
	}
	b.SetFocusable(false)
	b.SetEnabled(true)
	if b.Focusable() {
		t.Fatal("enabled lost focusable preference")
	}
	for _, value := range []float32{-1, float32(math.NaN()), float32(math.Inf(1))} {
		b.SetPadding(value)
		if b.Padding() != 0 {
			t.Fatal("invalid padding retained")
		}
	}
}

func TestSwitchDragAndCancellation(t *testing.T) {
	for _, tc := range []struct {
		name    string
		initial bool
		end     float32
		want    bool
		changes int
	}{
		{"on", false, 32, true, 1}, {"off", true, 16, false, 1},
		{"same-off", false, 21, false, 0}, {"same-on", true, 27, true, 0},
		{"mid-off", false, 24, false, 0}, {"mid-on", true, 24, true, 0},
		{"captured-outside", false, 150, true, 1},
	} {
		t.Run(tc.name, func(t *testing.T) {
			b, h, d := switchInput()
			b.SetAnimated(false)
			b.SetChecked(tc.initial)
			changes := 0
			b.ConnectChange(func(bool) { changes++ })
			x := float32(16)
			if tc.initial {
				x = 32
			}
			dispatchTabPointer(t, d, h, events.PointerDown, x, 16)
			dispatchTabPointer(t, d, h, events.PointerMove, tc.end, 16)
			if !b.drag.Dragging() || b.Checked() != tc.initial || changes != 0 {
				t.Fatal("drag changed committed state / failed to capture")
			}
			b.SetChecked(tc.initial) // A rebuild with the old committed value must not cancel.
			if !b.drag.Dragging() {
				t.Fatal("same setter canceled drag")
			}
			dispatchTabPointer(t, d, h, events.PointerUp, tc.end, 16)
			if b.Checked() != tc.want || changes != tc.changes || b.position != b.target() || b.drag.Dragging() {
				t.Fatalf("drag result checked=%v position=%v changes=%d", b.Checked(), b.position, changes)
			}
		})
	}
	for _, kind := range []string{"escape", "focus", "disabled", "hidden", "setter"} {
		t.Run(kind, func(t *testing.T) {
			b, h, d := switchInput()
			changes := 0
			b.ConnectChange(func(bool) { changes++ })
			dispatchTabPointer(t, d, h, events.PointerDown, 16, 16)
			dispatchTabPointer(t, d, h, events.PointerMove, 28, 16)
			switch kind {
			case "escape":
				switchKey(t, d, h, events.KeyEscape, false)
			case "focus":
				if err := d.DispatchEvent(h, events.FocusEvent{Focused: false}); err != nil {
					t.Fatal(err)
				}
			case "disabled":
				b.SetEnabled(false)
			case "hidden":
				b.SetVisible(false)
			case "setter":
				b.SetChecked(true)
			}
			dispatchTabPointer(t, d, h, events.PointerUp, 28, 16)
			if b.Checked() != (kind == "setter") || changes != 0 || b.drag.Dragging() || b.pressed || b.position != b.target() {
				t.Fatal("canceled drag committed / retained interaction")
			}
		})
	}
	// A vertical drag must remain available to a parent gesture controller.
	b, h, d := switchInput()
	parent := gui.NewLinearBox(layout.DirectionVertical)
	parent.AddChild(b)
	parent.Arrange(geometry.Rect(0, 0, 120, 32))
	b.Arrange(geometry.Rect(0, 0, 120, 32))
	h.root = parent
	outer := gui.NewDragEventController()
	outer.SetThreshold(4)
	parent.AddEventController(outer)
	dispatchTabPointer(t, d, h, events.PointerDown, 16, 16)
	dispatchTabPointer(t, d, h, events.PointerMove, 17, 25)
	if !outer.Dragging() || b.drag.Dragging() {
		t.Fatal("switch stole vertical parent drag")
	}
	dispatchTabPointer(t, d, h, events.PointerUp, 17, 25)
	if b.Checked() {
		t.Fatal("vertical drag became click")
	}
}

func TestSwitchContentAndReentry(t *testing.T) {
	b, h, d := switchInput()
	child := new(gui.WidgetBase)
	child.SetLayoutManager(checkBaselineLayout{})
	clicks := 0
	click := gui.NewClickEventController()
	click.ConnectClicked(func(gui.EventContext) { clicks++ })
	child.AddEventController(click)
	b.SetChild(child)
	b.Arrange(geometry.Rect(0, 0, 120, 32))
	dispatchTabPointer(t, d, h, events.PointerDown, 60, 16)
	dispatchTabPointer(t, d, h, events.PointerUp, 60, 16)
	if clicks != 1 || b.Checked() || b.pressed {
		t.Fatal("interactive child also activated parent")
	}
	child.RemoveEventController(click)
	dispatchTabPointer(t, d, h, events.PointerDown, 60, 16)
	dispatchTabPointer(t, d, h, events.PointerUp, 60, 16)
	if !b.Checked() {
		t.Fatal("noninteractive content failed to activate")
	}
	for _, destroy := range []bool{false, true} {
		b, h, d := switchInput()
		owner := gui.NewPopover(nil, nil)
		owner.SetWidget(b)
		late := 0
		b.ConnectChange(func(bool) {
			if destroy {
				owner.Destroy()
			} else {
				b.SetChecked(false)
			}
		})
		b.ConnectChange(func(bool) { late++ })
		dispatchTabPointer(t, d, h, events.PointerDown, 16, 16)
		dispatchTabPointer(t, d, h, events.PointerUp, 16, 16)
		if late != 0 || destroy && b.Root() != nil || !destroy && b.Checked() {
			t.Fatal("stale notification survived callback")
		}
		owner.Destroy()
	}
}

func TestSwitchLayoutAndSnapshot(t *testing.T) {
	b := NewSwitch()
	if m := b.Measure(layout.Unbounded()); m.Size != (geometry.Size{Width: 48, Height: 32}) || m.HasBaseline {
		t.Fatalf("empty measure=%+v", m)
	}
	child := new(gui.WidgetBase)
	child.SetID("content")
	child.SetLayoutManager(checkBaselineLayout{})
	b.SetChild(child)
	m := b.Measure(layout.Unbounded())
	b.Arrange(geometry.Rectangle{Size: m.Size})
	if m.Size != (geometry.Size{Width: 96, Height: 32}) || !m.HasBaseline || m.Baseline != 19 || child.Rect() != geometry.Rect(50, 10, 40, 12) {
		t.Fatalf("child layout=%+v measure=%+v", child.Rect(), m)
	}
	b.Arrange(geometry.Rect(0, 0, 200, 80))
	if r := b.trackRect(); r.Width != 36 || r.Height != 20 {
		t.Fatal("track expanded with available space")
	}
	b.SetPadding(0)
	b.Arrange(geometry.Rect(0, 0, 18, 10))
	if b.trackRect().Size != (geometry.Size{Width: 18, Height: 10}) {
		t.Fatal("tight track did not scale uniformly")
	}
	b.SetChecked(true)
	info := b.Snapshot()
	if info.Role != RoleSwitch || len(info.Children) != 1 || info.Children[0].ID != "content" || info.Selected || info.Attributes[SwitchInfoKey].(SwitchInfo).Checked != true {
		t.Fatal("incorrect semantics")
	}
	data, err := json.Marshal(info)
	if err != nil {
		t.Fatal(err)
	}
	var decoded struct {
		Attributes map[string]struct{ Checked bool }
	}
	if json.Unmarshal(data, &decoded) != nil || !decoded.Attributes[SwitchInfoKey].Checked {
		t.Fatal("nonserializable selection")
	}
	b.SetChecked(false)
	if !info.Attributes[SwitchInfoKey].(SwitchInfo).Checked {
		t.Fatal("snapshot aliases state")
	}
	b.SetEnabled(false)
	if info := b.Snapshot(); info.Enabled || len(info.Actions) != 0 || !info.Focusable {
		t.Fatal("disabled semantics advertise activation")
	}
	child.SetVisible(false)
	b.SetPadding(6)
	if m := b.Measure(layout.Unbounded()); m.HasBaseline || m.Width != 48 {
		t.Fatal("hidden child retained gap / baseline")
	}
}

func TestSwitchAnimation(t *testing.T) {
	for _, tc := range []struct {
		elapsed time.Duration
		want    float32
	}{{-time.Second, 0}, {0, 0}, {30 * time.Millisecond, .578125}, {60 * time.Millisecond, .875}, {120 * time.Millisecond, 1}, {time.Second, 1}} {
		if got := switchPosition(0, 1, tc.elapsed); got != tc.want {
			t.Fatalf("position at %s=%v want %v", tc.elapsed, got, tc.want)
		}
		if got := switchPosition(1, 0, tc.elapsed); got != 1-tc.want {
			t.Fatal("reverse animation differs")
		}
	}
	app := &progressTestApplication{}
	useProgressApplication(t, app)
	host := newProgressTestHost()
	b := NewSwitch()
	host.WidgetBase.AddChild(host, b)
	b.Arrange(geometry.Rect(0, 0, 48, 32))
	b.SetChecked(true) // The desktop-free fake cannot start a Timer: snap safely.
	if len(app.timers) != 1 || b.timer != nil || !b.started.IsZero() || b.position != 1 {
		t.Fatal("timer start failure froze transition")
	}
	// Stop samples the current transition before a new target is committed.
	b.from, b.started, b.position = 0, time.Now().Add(-60*time.Millisecond), 0
	b.stopAnimation()
	if b.position < .875 || b.position > 1 {
		t.Fatalf("lost in-flight position: %v", b.position)
	}
	b.from, b.started, b.position = 0, time.Now().Add(-60*time.Millisecond), 0
	b.SetChecked(false)
	if b.from < .875 || b.from > 1 || b.Checked() {
		t.Fatalf("retarget sampled the new endpoint instead of the old transition: %v", b.from)
	}
	b.SetChecked(true)
	b.from, b.started = 0, time.Now()
	b.SetChecked(true)
	if b.started.IsZero() {
		t.Fatal("same setter restarted / snapped animation")
	}
	b.SetAnimated(false)
	if !b.started.IsZero() || b.position != 1 {
		t.Fatal("animation disable did not finish")
	}
	b.SetAnimated(true)
	b.from, b.started = 0, time.Now()
	host.WidgetBase.RemoveChild(b)
	if b.mounted || !b.started.IsZero() || b.timer != nil || b.position != 1 {
		t.Fatal("unmount retained animation")
	}
	owner := gui.NewPopover(nil, nil)
	owner.SetWidget(b)
	b.from, b.started = 0, time.Now()
	owner.Destroy()
	if b.mounted || !b.started.IsZero() || b.timer != nil {
		t.Fatal("destroy retained animation")
	}
}

func TestSwitchSoftwarePixels(t *testing.T) {
	// Premultiplied sRGB RGBA8, interior centers away from AA boundaries.
	// Half-alpha red is (128,0,0,128); one byte rounding is allowed.
	red, blue := color.NRGBA{R: 255, A: 128}, color.RGBA{B: 255, A: 255}
	useProgressApplication(t, &progressTestApplication{sheet: style.Sheet(
		style.Name("switch").Part("track").BackgroundColor(red).BorderWidth(0).Radius(10),
		style.Name("switch").Part("track-checked").BackgroundColor(red).BorderWidth(0).Radius(10),
		style.Name("switch").Part("thumb").BackgroundColor(blue).BorderWidth(0),
		style.Name("switch").Part("thumb-checked").BackgroundColor(blue).BorderWidth(0),
	)})
	for _, scale := range []float32{1, 2} {
		b := NewSwitch()
		b.Arrange(geometry.Rect(0, 0, 48, 32))
		out := new(progressPixelSurface)
		native, err := software.NewPainter(out)
		if err != nil {
			t.Fatal(err)
		}
		draw := func(checked, transform bool) {
			b.SetChecked(checked)
			native.Begin(100*scale, 70*scale, scale)
			native.Clear(graphics.Color{})
			if transform {
				native.SetTransform(geometry.Translate(10, 8))
				native.SetClipRect(geometry.Rect(10, 8, 40, 32))
			} else {
				native.SetClipRect(geometry.Rect(0, 0, 40, 32))
			}
			b.Paint(checkPixelPainter{native: native})
			native.End()
		}
		pixel := func(x, y int) color.RGBA { return out.pixels.RGBAAt(x*int(scale), y*int(scale)) }
		draw(false, false)
		if pixel(16, 16) != blue || pixel(6, 6).A != 0 || pixel(3, 16).A != 0 {
			t.Fatal("thumb / capsule / padding geometry incorrect")
		}
		if got := pixel(32, 16); math.Abs(float64(got.R)-128) > 1 || math.Abs(float64(got.A)-128) > 1 || got.G != 0 || got.B != 0 {
			t.Fatalf("premultiplied track: %v", got)
		}
		draw(true, true)
		if pixel(42, 24) != blue || pixel(51, 24).A != 0 || pixel(13, 24).A != 0 {
			t.Fatal("translation / window clip / padding incorrect")
		}
		// A 90-degree affine rotation maps local (32,16) to window (44,42).
		native.Begin(100*scale, 70*scale, scale)
		native.Clear(graphics.Color{})
		native.SetClipRect(geometry.Rect(30, 10, 32, 40))
		native.SetTransform(geometry.Transform{A12: -1, A21: 1, TX: 60, TY: 10})
		b.Paint(checkPixelPainter{native: native})
		native.End()
		if pixel(44, 42) != blue || pixel(54, 16).A != 0 {
			t.Fatal("rotated circular thumb / capsule corner incorrect")
		}
		if got := pixel(44, 26); math.Abs(float64(got.R)-128) > 1 || math.Abs(float64(got.A)-128) > 1 || got.G != 0 || got.B != 0 {
			t.Fatalf("rotated alpha track: %v", got)
		}
		native.Destroy()
	}
}
