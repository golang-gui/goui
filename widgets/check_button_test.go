package widgets

import (
	"encoding/json"
	"image/color"
	"math"
	"reflect"
	"testing"

	"github.com/golang-gui/goui/core/geometry"
	"github.com/golang-gui/goui/gui"
	"github.com/golang-gui/goui/layout"
	"github.com/golang-gui/goui/platform/events"
	"github.com/golang-gui/goui/platform/graphics"
	"github.com/golang-gui/goui/platform/graphics/software"
	"github.com/golang-gui/goui/style"
	"github.com/golang-gui/goui/theme/modern"
)

func TestCheckButtonStateAndGroup(t *testing.T) {
	a, b := NewCheckButton(), NewCheckButton()
	if a.CheckState() != CheckUnchecked || a.Checked() || a.Group() != nil || !a.Focusable() || !a.Enabled() || a.Padding() != 6 {
		t.Fatal("unexpected defaults")
	}
	changes := 0
	a.ConnectChange(func(CheckState) { changes++ })
	a.SetCheckState(CheckMixed)
	if a.Checked() || a.CheckState() != CheckMixed {
		t.Fatal("mixed was lost")
	}
	a.SetChecked(true)
	g := NewCheckGroup()
	a.SetGroup(g)
	b.SetGroup(g)
	if g.Checked() != a || !a.Checked() || b.Checked() {
		t.Fatal("joining group lost selection")
	}
	b.SetChecked(true)
	if g.Checked() != b || a.Checked() || !b.Checked() {
		t.Fatal("programmatic selection failed exclusivity")
	}
	b.SetCheckState(CheckMixed)
	if b.CheckState() != CheckUnchecked || g.Checked() != nil {
		t.Fatal("group accepted Mixed / failed to clear")
	}
	a.SetChecked(true)
	a.SetGroup(nil)
	if !a.Checked() || g.Checked() != nil {
		t.Fatal("detaching lost member state / retained group selection")
	}
	b.SetChecked(true)
	a.SetGroup(g)
	if b.Checked() || !a.Checked() || g.Checked() != a {
		t.Fatal("checked joining member failed to replace selection")
	}
	a.SetCheckState(255)
	a.SetAppearance(255)
	if a.Checked() || g.Checked() != nil || a.Appearance() != CheckAppearanceIndicator || changes != 0 {
		t.Fatal("invalid values / silent setters")
	}
	for _, invalid := range []float32{-1, float32(math.NaN()), float32(math.Inf(1))} {
		a.SetPadding(invalid)
		if a.Padding() != 0 {
			t.Fatal("invalid padding retained")
		}
	}
}

// Real dispatcher path (including gesture competition), without a native window.
func checkClick(t *testing.T, b *CheckButton) {
	t.Helper()
	b.Measure(layout.Tight(geometry.Size{Width: 120, Height: 32}))
	b.Arrange(geometry.Rect(0, 0, 120, 32))
	h := &tabInputHost{root: b}
	d := new(gui.EventDispatcher)
	dispatchTabPointer(t, d, h, events.PointerDown, 8, 16)
	dispatchTabPointer(t, d, h, events.PointerUp, 8, 16)
}

func TestCheckButtonInput(t *testing.T) {
	b := NewCheckButton()
	b.Arrange(geometry.Rect(0, 0, 120, 32))
	host, d := &tabInputHost{root: b}, new(gui.EventDispatcher)
	var states []CheckState
	b.ConnectChange(func(s CheckState) { states = append(states, s) })
	for _, x := range []float32{8, 90} { // Indicator and otherwise empty label area.
		before := b.CheckState()
		dispatchTabPointer(t, d, host, events.PointerDown, x, 16)
		if b.CheckState() != before {
			t.Fatal("changed before valid release")
		}
		dispatchTabPointer(t, d, host, events.PointerUp, x, 16)
	}
	if !reflect.DeepEqual(states, []CheckState{CheckChecked, CheckUnchecked}) {
		t.Fatalf("wrong click sequence: %v", states)
	}
	dispatchTabPointer(t, d, host, events.PointerDown, 8, 16)
	dispatchTabPointer(t, d, host, events.PointerMove, 150, 16)
	dispatchTabPointer(t, d, host, events.PointerUp, 150, 16)
	if b.Checked() || len(states) != 2 || b.pressed {
		t.Fatal("outside release activated / retained press")
	}
	dispatchTabPointer(t, d, host, events.PointerDown, 8, 16)
	if err := d.DispatchEvent(host, events.FocusEvent{Focused: false}); err != nil {
		t.Fatal(err)
	}
	dispatchTabPointer(t, d, host, events.PointerUp, 8, 16)
	if b.Checked() || len(states) != 2 || b.pressed {
		t.Fatal("lost-focus gesture activated / retained press")
	}
	b.SetCheckState(CheckMixed)
	checkClick(t, b)
	if !b.Checked() || states[2] != CheckChecked {
		t.Fatal("Mixed did not become Checked")
	}
	b.SetChecked(false)
	host.focus = b
	for _, tc := range []struct {
		key    events.Key
		repeat bool
		want   CheckState
	}{{events.KeySpace, false, CheckChecked}, {events.KeySpace, true, CheckChecked}, {events.KeyEnter, false, CheckUnchecked}} {
		handled := false
		if err := d.DispatchEvent(host, events.KeyEvent{EventType: events.KeyDown, Key: tc.key, Repeat: tc.repeat, Handled: &handled}); err != nil {
			t.Fatal(err)
		}
		if b.CheckState() != tc.want || !handled {
			t.Fatal("keyboard activation / repeat / native consumption failed")
		}
	}
	before := len(states)
	dispatchTabPointer(t, d, host, events.PointerDown, 8, 16)
	b.SetEnabled(false)
	dispatchTabPointer(t, d, host, events.PointerUp, 8, 16)
	checkClick(t, b)
	if b.Checked() || len(states) != before || !b.Focusable() || gui.IsEnabled(b) || b.Snapshot().Enabled {
		t.Fatal("disabled control activated or lost its own focus capability")
	}
	b.SetFocusable(false)
	b.SetEnabled(true)
	if b.Focusable() {
		t.Fatal("enabling discarded explicit focusability")
	}
	b.SetFocusable(true)
	checkClick(t, b)
	if !b.Checked() || !b.Focusable() || len(b.EventControllers()) != 3 {
		t.Fatal("reenable did not restore one controller per kind")
	}
	for i := 0; i < 4; i++ {
		b.SetEnabled(false)
		b.SetEnabled(true)
	}
	if len(b.EventControllers()) != 3 {
		t.Fatal("reenabling duplicated controllers")
	}
}

func TestCheckButtonChildCompetition(t *testing.T) {
	b := NewCheckButton()
	child := splitTestChild(60, 20)
	click := gui.NewClickEventController()
	clicks := 0
	click.ConnectClicked(func(ctx gui.EventContext) { clicks++; ctx.StopPropagation() })
	child.AddEventController(click)
	b.SetChild(child)
	b.Measure(layout.Tight(geometry.Size{Width: 120, Height: 32}))
	b.Arrange(geometry.Rect(0, 0, 120, 32))
	host, d := &tabInputHost{root: b}, new(gui.EventDispatcher)
	dispatchTabPointer(t, d, host, events.PointerDown, 40, 16)
	dispatchTabPointer(t, d, host, events.PointerUp, 40, 16)
	if b.Checked() || b.pressed || clicks != 1 {
		t.Fatal("child click also toggled parent / retained press")
	}
	child.RemoveEventController(click)
	dispatchTabPointer(t, d, host, events.PointerDown, 40, 16)
	dispatchTabPointer(t, d, host, events.PointerUp, 40, 16)
	if !b.Checked() || clicks != 1 {
		t.Fatal("noninteractive content did not activate parent")
	}
	b.SetChild(nil)
	if child.Parent() != nil || len(b.Children()) != 0 {
		t.Fatal("replaced child remained mounted")
	}
	b.SetChild(b)
	if b.Child() != nil {
		t.Fatal("accepted cyclic child")
	}
}

func TestCheckGroupAtomicChangeAndReentry(t *testing.T) {
	for _, reenter := range []bool{false, true} {
		t.Run(map[bool]string{false: "atomic", true: "reentry"}[reenter], func(t *testing.T) {
			a, b, c := NewCheckButton(), NewCheckButton(), NewCheckButton()
			g := NewCheckGroup()
			for _, member := range []*CheckButton{a, b, c} {
				member.SetGroup(g)
			}
			a.SetChecked(true)
			var order []string
			a.ConnectChange(func(state CheckState) {
				if state != CheckUnchecked || a.Checked() || !b.Checked() || g.Checked() != b {
					t.Fatal("callback observed uncommitted group state")
				}
				order = append(order, "old")
				if reenter {
					c.SetChecked(true)
				}
			})
			a.ConnectChange(func(CheckState) { order = append(order, "old-later") })
			b.ConnectChange(func(CheckState) { order = append(order, "new") })
			checkClick(t, b)
			want := []string{"old", "old-later", "new"}
			if reenter {
				want = []string{"old"}
			}
			if !reflect.DeepEqual(order, want) {
				t.Fatalf("notifications=%v, want %v", order, want)
			}
			if reenter && (g.Checked() != c || b.Checked()) {
				t.Fatal("reentrant selection lost")
			}
			before := len(order)
			checkClick(t, g.Checked())
			if len(order) != before {
				t.Fatal("checked member could uncheck itself")
			}
		})
	}
	b := NewCheckButton()
	called := 0
	b.ConnectChange(func(CheckState) { b.SetChecked(false) })
	b.ConnectChange(func(CheckState) { called++ })
	checkClick(t, b)
	if b.Checked() || called != 0 {
		t.Fatal("stale independent notification survived reentry")
	}
}

func TestCheckGroupSurvivesReparenting(t *testing.T) {
	a, b := NewCheckButton(), NewCheckButton()
	g := NewCheckGroup()
	a.SetGroup(g)
	b.SetGroup(g)
	a.SetChecked(true)
	old, next := new(progressTestHost), new(progressTestHost)
	old.root, next.root = &progressTestRoot{widget: old}, &progressTestRoot{widget: next}
	old.WidgetBase.AddChild(old, a)
	old.WidgetBase.RemoveChild(a)
	if a.Root() != nil || g.Checked() != a {
		t.Fatal("unmount changed logical selection")
	}
	next.WidgetBase.AddChild(next, a)
	if a.Root() != next.root || a.Group() != g || g.Checked() != a {
		t.Fatal("mount changed group identity")
	}
	b.SetChecked(true)
	if a.Checked() || g.Checked() != b {
		t.Fatal("transferred member lost exclusivity")
	}
}

type checkBaselineLayout struct{}

func (checkBaselineLayout) Measure(_ []layout.Child, c layout.Constraint) layout.Measurement {
	return layout.MeasuredWithBaseline(c.Clamp(geometry.Size{Width: 40, Height: 12}), 9)
}
func (checkBaselineLayout) Arrange([]layout.Child, geometry.Rectangle) {}

func TestCheckButtonLayoutAndSnapshot(t *testing.T) {
	b, child := NewCheckButton(), new(gui.WidgetBase)
	child.SetLayoutManager(checkBaselineLayout{})
	child.SetID("content")
	b.SetChild(child)
	m := b.Measure(layout.Unbounded())
	if m.Size != (geometry.Size{Width: 78, Height: 30}) || !m.HasBaseline || m.Baseline != 18 {
		t.Fatalf("indicator measurement/baseline=%+v", m)
	}
	b.Arrange(geometry.Rectangle{Size: m.Size})
	if child.Rect() != geometry.Rect(32, 9, 40, 12) || m.Baseline != child.Rect().Y+9 {
		t.Fatal("arranged baseline differs from measured")
	}
	b.SetAppearance(CheckAppearanceButton)
	m = b.Measure(layout.Tight(geometry.Size{Width: 100, Height: 40}))
	b.Arrange(geometry.Rectangle{Size: m.Size})
	if child.Rect() != geometry.Rect(30, 14, 40, 12) || m.Baseline != 23 {
		t.Fatal("button did not center content/baseline")
	}
	b.SetMinSize(geometry.Size{Width: 140, Height: 80})
	b.SetMaxSize(geometry.Size{Width: 150, Height: 90})
	m = b.Measure(layout.Loose(geometry.Size{Width: 10, Height: 6}))
	b.Arrange(geometry.Rectangle{Size: m.Size})
	if m.Size != (geometry.Size{Width: 10, Height: 6}) || child.Rect().Width > 10 || child.Rect().Height > 6 {
		t.Fatal("child preferences broke parent hard bounds")
	}
	child.SetVisible(false)
	b.SetMinSize(geometry.Size{})
	b.SetMaxSize(geometry.Size{})
	if b.Measure(layout.Unbounded()).HasBaseline {
		t.Fatal("hidden content leaked baseline")
	}
	b.SetVisible(false)
	if b.Measure(layout.Unbounded()).Size != (geometry.Size{}) {
		t.Fatal("hidden control did not collapse")
	}
	b.SetVisible(true)
	for _, tc := range []struct {
		appearance CheckAppearance
		group      *CheckGroup
		role       gui.Role
	}{
		{CheckAppearanceIndicator, nil, RoleCheckBox}, {CheckAppearanceIndicator, NewCheckGroup(), RoleRadioButton}, {CheckAppearanceButton, NewCheckGroup(), RoleToggleButton},
	} {
		b.SetGroup(tc.group)
		b.SetAppearance(tc.appearance)
		b.SetChecked(true)
		info := b.Snapshot()
		check := info.Attributes[CheckInfoKey].(CheckInfo)
		if info.Role != tc.role || check.State != CheckChecked || check.Grouped != (tc.group != nil) || info.Selected || len(info.Children) != 1 || info.Children[0].ID != "content" {
			t.Fatal("snapshot omitted content / confused selection with focus")
		}
		if _, err := json.Marshal(info); err != nil {
			t.Fatal(err)
		}
		b.SetChecked(false)
		if check.State != CheckChecked {
			t.Fatal("snapshot aliases mutable selection")
		}
	}
}

func TestCheckButtonFontMinimumMatchesButton(t *testing.T) {
	// System UI fonts are often smaller than Modern's default 14 pt. A fixed
	// 20 DIP toggle floor used to make identical labels taller than Button.
	for _, size := range []float32{10, 14, 22} {
		useProgressApplication(t, &progressTestApplication{sheet: modern.Sheet(modern.Options{FontSize: size})})
		ordinary, toggle := gui.NewButton(), NewCheckButton()
		left, right := new(gui.WidgetBase), new(gui.WidgetBase)
		left.SetLayoutManager(checkBaselineLayout{})
		right.SetLayoutManager(checkBaselineLayout{})
		ordinary.SetChild(left)
		toggle.SetChild(right)
		toggle.SetAppearance(CheckAppearanceButton)
		want, got := ordinary.Measure(layout.Unbounded()), toggle.Measure(layout.Unbounded())
		if got.Size != want.Size {
			t.Fatalf("font=%g pt: toggle=%v, Button=%v", size, got.Size, want.Size)
		}
	}
}

// Software pixel reference, not a copied GUI traversal. Samples are away from
// AA edges; premultiplied sRGB RGBA8 half-opacity has at most one byte rounding.
type checkPixelPainter struct {
	gui.Painter
	native graphics.Painter
}

func (p checkPixelPainter) FillRect(r geometry.Rectangle, b graphics.Brush) { p.native.FillRect(r, b) }
func (p checkPixelPainter) FillRoundRect(r geometry.Rectangle, s float32, b graphics.Brush) {
	p.native.FillRoundRect(r, s, b)
}
func (p checkPixelPainter) FillEllipse(c geometry.Point, x, y float32, b graphics.Brush) {
	p.native.FillEllipse(c, x, y, b)
}
func (p checkPixelPainter) DrawEllipse(c geometry.Point, x, y, w float32, b graphics.Brush) {
	p.native.DrawEllipse(c, x, y, w, b)
}
func (p checkPixelPainter) DrawRoundRect(r geometry.Rectangle, s, w float32, b graphics.Brush) {
	p.native.DrawRoundRect(r, s, w, b)
}
func (p checkPixelPainter) DrawRect(r geometry.Rectangle, w float32, b graphics.Brush) {
	p.native.DrawRect(r, w, b)
}
func (p checkPixelPainter) DrawPath(path graphics.Path, w float32, b graphics.Brush) {
	p.native.DrawPath(path, w, b)
}

func TestCheckButtonSoftwarePixels(t *testing.T) {
	red, blue := color.NRGBA{R: 255, A: 128}, color.RGBA{B: 255, A: 255}
	useProgressApplication(t, &progressTestApplication{sheet: style.Sheet(
		style.Name("check-button").Part("indicator-mixed").BackgroundColor(red).ForegroundColor(blue).BorderWidth(0).Radius(2),
		style.Name("check-button").Part("indicator-checked").BackgroundColor(red).ForegroundColor(blue).BorderWidth(0).Radius(2),
		style.Name("check-button").Part("radio-checked").BackgroundColor(red).ForegroundColor(blue).BorderWidth(0),
		style.Name("check-button").Part("button-checked").BackgroundColor(red).BorderWidth(0).Radius(2),
	)})
	for _, scale := range []float32{1, 2} {
		b := NewCheckButton()
		// Constrain the 18 DIP indicator to 16 DIP with explicit padding. These samples test
		// premultiplied painting, not the separately asserted default padding.
		b.SetPadding(4)
		b.SetCheckState(CheckMixed)
		b.Arrange(geometry.Rect(0, 0, 24, 24))
		out := new(progressPixelSurface)
		native, err := software.NewPainter(out)
		if err != nil {
			t.Fatal(err)
		}
		draw := func() {
			native.Begin(32*scale, 32*scale, scale)
			native.Clear(graphics.Color{})
			native.SetClipRect(geometry.Rect(0, 0, 24, 24))
			b.Paint(checkPixelPainter{native: native})
			native.End()
		}
		draw()
		pixel := func(x, y int) color.RGBA {
			return color.RGBAModel.Convert(out.pixels.At(x*int(scale), y*int(scale))).(color.RGBA)
		}
		if got := pixel(12, 7); math.Abs(float64(got.R)-128) > 1 || math.Abs(float64(got.A)-128) > 1 || got.G != 0 || got.B != 0 {
			t.Fatalf("premultiplied indicator %gx=%v", scale, got)
		}
		if pixel(12, 12) != blue || pixel(2, 12) != (color.RGBA{}) {
			t.Fatal("mixed mark / transparent padding incorrect")
		}
		b.SetChecked(true)
		draw()
		// The turn of a 1.92 DIP check stroke covers this pixel at both scales.
		// A missing mark would have B=0; require majority blue coverage, not a
		// backend-specific antialiasing value at the joint.
		if got := pixel(10, 14); got.B <= 128 {
			t.Fatalf("check mark missing at %gx: %v", scale, got)
		}
		b.SetGroup(NewCheckGroup())
		b.SetChecked(true)
		draw()
		if pixel(12, 12) != blue || pixel(4, 4) != (color.RGBA{}) {
			t.Fatal("radio mark / circular geometry incorrect")
		}
		b.SetAppearance(CheckAppearanceButton)
		draw()
		if got := pixel(12, 12); math.Abs(float64(got.R)-128) > 1 || got.B != 0 || math.Abs(float64(got.A)-128) > 1 {
			t.Fatal("button appearance retained indicator glyph")
		}
		if _, _, _, a := out.pixels.At(25*int(scale), 12*int(scale)).RGBA(); a != 0 {
			t.Fatal("drawing escaped bounds")
		}
		native.Destroy()
	}
}

func TestCheckButtonModernSoftwarePixels(t *testing.T) {
	// Opaque sRGB expectations, read at interior pixel centers away from AA
	// boundaries. Padding must stay transparent; no font/platform golden image.
	for _, tc := range []struct {
		dark bool
		fill color.RGBA
		edge color.RGBA
	}{
		{false, color.RGBA{R: 228, G: 228, B: 229, A: 255}, color.RGBA{R: 198, G: 198, B: 206, A: 255}},
		{true, color.RGBA{R: 72, G: 72, B: 78, A: 255}, color.RGBA{R: 85, G: 85, B: 95, A: 255}},
	} {
		useProgressApplication(t, &progressTestApplication{sheet: modern.Sheet(modern.Options{Dark: tc.dark})})
		for _, scale := range []float32{1, 2} {
			b := NewCheckButton()
			b.SetCheckState(CheckMixed)
			b.Arrange(geometry.Rect(0, 0, 30, 30))
			out := new(progressPixelSurface)
			native, err := software.NewPainter(out)
			if err != nil {
				t.Fatal(err)
			}
			draw := func() {
				native.Begin(30*scale, 30*scale, scale)
				native.Clear(graphics.Color{})
				b.Paint(checkPixelPainter{native: native})
				native.End()
			}
			pixel := func(x, y int) color.RGBA { return out.pixels.RGBAAt(x*int(scale), y*int(scale)) }
			draw()
			if got := pixel(15, 9); got.A != 255 || got.B <= got.G || got.G <= got.R || pixel(3, 15) != (color.RGBA{}) {
				t.Fatalf("dark=%v scale=%v mixed fill=%v; want solid blue and transparent padding", tc.dark, scale, got)
			}
			if mark := pixel(15, 15); mark != (color.RGBA{R: 255, G: 255, B: 255, A: 255}) {
				t.Fatalf("mixed mark is not white: %v", mark)
			}
			b.SetGroup(NewCheckGroup())
			b.SetChecked(true)
			draw()
			mark := pixel(15, 15)
			gap := color.RGBA{R: 255, G: 255, B: 255, A: 255}
			if tc.dark {
				gap = color.RGBA{R: 50, G: 50, B: 56, A: 255}
			}
			if mark.A != 255 || mark.B <= mark.G || mark.G <= mark.R || pixel(15, 9) != gap || pixel(6, 6) != (color.RGBA{}) {
				t.Fatal("radio lost its colored dot / circular geometry")
			}
			b.SetAppearance(CheckAppearanceButton)
			draw()
			if pixel(15, 15) != tc.fill || pixel(15, 0) != tc.edge {
				t.Fatalf("selected button: fill=%v edge=%v; want %v / %v", pixel(15, 15), pixel(15, 0), tc.fill, tc.edge)
			}
			native.Destroy()
		}
	}
}
