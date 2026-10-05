package gui

import (
	"fmt"
	"image"
	"image/color"
	"math"
	"testing"

	"github.com/golang-gui/goui/core/geometry"
	"github.com/golang-gui/goui/layout"
	"github.com/golang-gui/goui/platform/events"
	"github.com/golang-gui/goui/platform/graphics"
	"github.com/golang-gui/goui/platform/graphics/software"
	"github.com/golang-gui/goui/platform/typography"
	"github.com/golang-gui/goui/style"
)

func TestButtonSnapshot(t *testing.T) {
	button := NewButton()
	button.SetID("confirm")
	button.Arrange(geometry.Rect(1, 2, 80, 30))

	info := button.Snapshot()
	if info.ID != "confirm" {
		t.Fatalf("unexpected snapshot id: %q", info.ID)
	}
	if info.Role != RoleButton {
		t.Fatalf("unexpected snapshot role: %q", info.Role)
	}
	if !info.Focusable {
		t.Fatal("button should be focusable")
	}
	if info.Text != "" {
		t.Fatalf("button should not own content text, got %q", info.Text)
	}
	if len(info.Actions) != 1 || info.Actions[0] != ActionClick {
		t.Fatalf("unexpected snapshot actions: %v", info.Actions)
	}
}

func TestButtonFocusVisiblePixels(t *testing.T) {
	for _, kind := range []struct {
		name string
		new  func() Bin
	}{
		{"Button", func() Bin { return NewButton() }},
		{"MenuButton", func() Bin { return NewMenuButton() }},
	} {
		for _, scale := range []float32{1, 2} {
			t.Run(fmt.Sprintf("%s/%gx", kind.name, scale), func(t *testing.T) {
				setTestApplication(t, nil)
				backend, err := software.NewPainter(&widgetRenderSurface{})
				if err != nil {
					t.Fatal(err)
				}
				defer backend.Destroy()
				button := kind.new()
				win := &window{rootBase: rootBase{painter: backend}}
				win.SetWidget(button)
				defer win.SetWidget(nil)
				button.Arrange(geometry.Rect(0, 0, 80, 30))
				softBorder := color.RGBA{R: 102, G: 147, B: 217, A: 255}
				check := func(want uint8, border color.RGBA, borderWidth int) {
					t.Helper()
					img, err := RenderWidget(button, scale)
					if err != nil {
						t.Fatal(err)
					}
					// Straight edges and interior, away from rounded corners/text.
					// Opaque premultiplied sRGB bytes: exact assertions (there is
					// no translucent tint or partial edge coverage at these points).
					body := color.RGBA{R: want, G: want, B: want, A: 255}
					pixels := int(scale)
					// Sample every physical pixel through the entire border and
					// the first interior pixel on all four sides (2/4 pixels at
					// 1x/2x for the default 2 DIP border).
					for depth := 0; depth <= borderWidth*pixels; depth++ {
						wantColor := body
						if depth < borderWidth*pixels {
							wantColor = border
						}
						for _, point := range []image.Point{{X: 40 * pixels, Y: depth}, {X: 40 * pixels, Y: 30*pixels - 1 - depth}, {X: depth, Y: 15 * pixels}, {X: 80*pixels - 1 - depth, Y: 15 * pixels}} {
							got := color.RGBAModel.Convert(img.At(point.X, point.Y)).(color.RGBA)
							if got != wantColor {
								t.Fatalf("pixel at %v (depth=%d px): %v want %v", point, depth, got, wantColor)
							}
						}
					}
					if got := color.RGBAModel.Convert(img.At(40*pixels, 15*pixels)).(color.RGBA); got != body {
						t.Fatalf("focus changed the interior: %v want %v", got, body)
					}
					if button.Rect() != geometry.Rect(0, 0, 80, 30) || img.Bounds() != image.Rect(0, 0, 80*int(scale), 30*int(scale)) {
						t.Fatal("focus changed layout or expanded rendering bounds")
					}
					if got := color.RGBAModel.Convert(img.At(0, 0)).(color.RGBA); got != (color.RGBA{}) {
						t.Fatalf("focus squared off the rounded corner: %v", got)
					}
				}
				check(210, color.RGBA{R: 210, G: 210, B: 210, A: 255}, 0)
				_ = win.DispatchEvent(shortcutPress(events.KeyTab, 0))
				check(210, softBorder, 2)
				_ = win.DispatchEvent(events.PointerEvent{EventType: events.PointerMove, Position: geometry.Point{X: 10, Y: 10}})
				check(230, softBorder, 2) // Hover background is untouched.
				_ = win.DispatchEvent(events.PointerEvent{EventType: events.PointerDown, Position: geometry.Point{X: 10, Y: 10}, Button: events.PointerButtonLeft, Buttons: events.PointerButtonLeftDown})
				if !button.Focused() || button.FocusVisible() {
					t.Fatal("pointer press did not preserve focus while hiding the hint")
				}
				check(180, color.RGBA{R: 180, G: 180, B: 180, A: 255}, 0)
				_ = win.DispatchEvent(events.PointerEvent{EventType: events.PointerUp, Position: geometry.Point{X: 100, Y: 100}, Button: events.PointerButtonLeft})
				_ = win.DispatchEvent(events.PointerEvent{EventType: events.PointerMove, Position: geometry.Point{X: 100, Y: 100}})
				check(210, color.RGBA{R: 210, G: 210, B: 210, A: 255}, 0)
				// Generic Focused rules still control pointer focus; the more
				// specific default FocusVisible border controls keyboard focus.
				rules := append(DefaultStyleRules(), style.Name("button").Part("focus").State(style.Focused).
					BorderColor(color.RGBA{R: 255, A: 255}).BorderWidth(3))
				App.SetStyleSheet(style.Sheet(rules...))
				for _, keyboard := range []bool{false, true} {
					if keyboard {
						_ = win.DispatchEvent(shortcutPress(events.KeyTab, 0))
					}
					if keyboard {
						check(210, softBorder, 2)
					} else {
						check(210, color.RGBA{R: 255, A: 255}, 3)
					}
				}
				// Explicit keyboard overrides remain available, including disabling
				// the border without changing its underlying normal background.
				App.SetStyleSheet(style.Sheet(append(rules, style.Name("button").Part("focus").State(style.FocusVisible).BorderWidth(0))...))
				check(210, color.RGBA{R: 210, G: 210, B: 210, A: 255}, 0)
				for _, width := range []int{1, 3} {
					App.SetStyleSheet(style.Sheet(append(rules, style.Name("button").Part("focus").State(style.FocusVisible).
						BorderColor(color.RGBA{R: 255, A: 255}).BorderWidth(float32(width)))...))
					check(210, color.RGBA{R: 255, A: 255}, width)
				}
				child := newPainterTestWidget(func(p Painter) {
					p.FillRect(geometry.Rect(0, 0, 8, 8), graphics.RGB(0, 0, 255))
				})
				button.SetChild(child)
				child.Arrange(geometry.Rect(36, 11, 8, 8))
				img, err := RenderWidget(button, scale)
				if err != nil {
					t.Fatal(err)
				}
				if got := color.RGBAModel.Convert(img.At(40*int(scale), 15*int(scale))).(color.RGBA); got != (color.RGBA{B: 255, A: 255}) {
					t.Fatalf("focus border covered child painting: %v", got)
				}
			})
		}
	}
}

func TestButtonUsesWidgetBaseLayoutAndPaint(t *testing.T) {
	button := NewButton()
	child := newPaintCountingWidget()
	// Sized well above the skeleton floor so this test isolates the delegate +
	// padding behavior. The default button padding is 6, added on every side.
	manager := &testLayoutManager{
		measureSize: geometry.Size{Width: 120, Height: 60},
	}
	button.SetLayoutManager(manager)
	button.SetChild(child)

	size := button.Measure(layout.Loose(geometry.Size{Width: 300, Height: 200}))
	if size.Size != (geometry.Size{Width: 132, Height: 72}) {
		t.Fatalf("unexpected measured size: %+v", size)
	}
	if len(manager.measured) != 1 || layoutChildWidget(manager.measured[0]) != child {
		t.Fatalf("layout measured unexpected children: %v", manager.measured)
	}

	button.Arrange(geometry.Rect(0, 0, 80, 30))
	if manager.arrangeRect != geometry.Rect(6, 6, 68, 18) {
		t.Fatalf("unexpected layout arrange rect: %+v", manager.arrangeRect)
	}

	backend := new(recordingPainterBackend)
	paintWidget(button, newPainter(backend, geometry.Rect(0, 0, 80, 30), 1))
	if child.paints != 1 {
		t.Fatalf("automatic traversal should paint the child, got %d", child.paints)
	}
}

func TestButtonDefaultLayoutCentersContent(t *testing.T) {
	button := NewButton()
	child := newSizedWidget(geometry.Size{Width: 20, Height: 10})
	button.SetChild(child)

	button.Arrange(geometry.Rect(0, 0, 80, 30))
	if child.Rect() != geometry.Rect(30, 10, 20, 10) {
		t.Fatalf("unexpected centered child rect: %+v", child.Rect())
	}
}

func TestButtonExplicitFillLayoutArrangesContent(t *testing.T) {
	button := NewButton()
	button.SetLayoutManager(layout.NewFillLayout())
	child := newTestWidget()
	button.SetChild(child)

	button.Arrange(geometry.Rect(0, 0, 80, 30))

	// Content is inset by the default button padding (6) on every side.
	if child.Rect() != geometry.Rect(6, 6, 68, 18) {
		t.Fatalf("unexpected child rect: %+v", child.Rect())
	}
}

func TestButtonEmptyKeepsVisibleSkeleton(t *testing.T) {
	// A button with no content must not collapse to zero; it keeps at least a
	// font-derived skeleton so it stays visible and clickable (no user SetMinSize
	// needed). The floor is a line-height square, so it never drops below the
	// line height on either axis regardless of padding.
	button := NewButton()

	size := button.Measure(layout.Loose(geometry.Size{Width: 500, Height: 500}))

	minSkeleton := textLineHeight(defaultFontSize)
	if size.Width < minSkeleton || size.Height < minSkeleton {
		t.Fatalf("empty button collapsed: %+v (want at least %v on each axis)", size, minSkeleton)
	}
}

func TestButtonMeasurePropagatesContentBaselineThroughPadding(t *testing.T) {
	button := NewButton()
	button.SetLayoutManager(&testLayoutManager{
		measureSize:     geometry.Size{Width: 30, Height: 18},
		measureBaseline: 14,
		hasBaseline:     true,
	})
	button.SetChild(newTestWidget())

	measured := button.Measure(layout.Loose(geometry.Size{Width: 300, Height: 100}))
	if !measured.HasBaseline || measured.Baseline != 20 {
		t.Fatalf("button baseline should include default padding: %+v", measured)
	}
}

func TestButtonContentBaselineMatchesArrangement(t *testing.T) {
	constructors := []struct {
		name          string
		make          func() Bin
		naturalHeight float32
	}{
		{"Button", func() Bin { return NewButton() }, max(22, textLineHeight(defaultFontSize)+12)},
		{"MenuButton", func() Bin { return NewMenuButton() }, 22},
	}
	for _, constructor := range constructors {
		t.Run(constructor.name, func(t *testing.T) {
			cases := []struct {
				name     string
				min, max geometry.Size
				parent   layout.Constraint
				want     geometry.Size
			}{
				{name: "natural", parent: layout.Unbounded(), want: geometry.Size{Width: 32, Height: constructor.naturalHeight}},
				{name: "minimum", min: geometry.Size{Width: 80, Height: 50}, parent: layout.Unbounded(), want: geometry.Size{Width: 80, Height: 50}},
				{name: "maximum", max: geometry.Size{Width: 20, Height: 16}, parent: layout.Unbounded(), want: geometry.Size{Width: 20, Height: 16}},
				{name: "max-wins", min: geometry.Size{Width: 80, Height: 50}, max: geometry.Size{Width: 20, Height: 16}, parent: layout.Unbounded(), want: geometry.Size{Width: 20, Height: 16}},
				{name: "parent-wins", max: geometry.Size{Width: 20, Height: 16}, parent: layout.Tight(geometry.Size{Width: 100, Height: 60}), want: geometry.Size{Width: 100, Height: 60}},
				{name: "smaller-than-padding", parent: layout.Tight(geometry.Size{Width: 8, Height: 6}), want: geometry.Size{Width: 8, Height: 6}},
			}
			for _, tc := range cases {
				t.Run(tc.name, func(t *testing.T) {
					button := constructor.make()
					button.SetMinSize(tc.min)
					button.SetMaxSize(tc.max)
					child := &baselineSizedWidget{measurement: layout.MeasuredWithBaseline(
						geometry.Size{Width: 20, Height: 10}, 7,
					)}
					button.SetChild(child)
					measured := button.Measure(tc.parent)
					if measured.Size != tc.want || !measured.HasBaseline {
						t.Fatalf("measurement=%+v, want size=%+v with baseline", measured, tc.want)
					}
					button.Arrange(geometry.Rectangle{Pos: geometry.Point{X: 13, Y: 17}, Size: measured.Size})
					actual := child.Rect()
					if math.Abs(float64(actual.X-(measured.Width-actual.Width)/2)) > 0.0001 ||
						math.Abs(float64(actual.Y-(measured.Height-actual.Height)/2)) > 0.0001 {
						t.Fatalf("child not centered: parent=%+v child=%+v", measured.Size, actual)
					}
					if math.Abs(float64(measured.Baseline-(actual.Y+7))) > 0.0001 {
						t.Fatalf("baseline=%v does not match child baseline=%v", measured.Baseline, actual.Y+7)
					}
				})
			}
		})
	}
}

func TestCenteredButtonParticipatesInBaselineRow(t *testing.T) {
	button := NewButton()
	button.SetMinSize(geometry.Size{Height: 50})
	content := &baselineSizedWidget{measurement: layout.MeasuredWithBaseline(geometry.Size{Width: 20, Height: 10}, 7)}
	button.SetChild(content)
	label := &baselineSizedWidget{measurement: layout.MeasuredWithBaseline(geometry.Size{Width: 30, Height: 18}, 14)}
	row := NewLinearBox(layout.DirectionHorizontal)
	row.SetCrossAlign(layout.CrossBaseline)
	row.AddChild(label)
	row.AddChild(button)
	measured := row.Measure(layout.Unbounded())
	row.Arrange(geometry.Rectangle{Size: measured.Size})
	if label.Rect().Y+14 != button.Rect().Y+content.Rect().Y+7 {
		t.Fatalf("baseline mismatch: label=%+v button=%+v content=%+v", label.Rect(), button.Rect(), content.Rect())
	}
}

func TestButtonMeasuresWrappingContentWithinOwnMaximum(t *testing.T) {
	button := NewButton()
	button.SetMaxSize(geometry.Size{Width: 52})
	child := new(wrappingButtonChild)
	button.SetChild(child)
	measured := button.Measure(layout.Unbounded())
	if measured.Size != (geometry.Size{Width: 52, Height: 32}) {
		t.Fatalf("button did not measure two lines within its maximum width: %+v", measured)
	}
	button.Arrange(geometry.Rectangle{Size: measured.Size})
	if child.Rect() != geometry.Rect(6, 6, 40, 20) || measured.Baseline != 13 {
		t.Fatalf("wrapping content changed between Measure and Arrange: child=%+v measured=%+v", child.Rect(), measured)
	}
}

type wrappingButtonChild struct{ WidgetBase }

func (w *wrappingButtonChild) Measure(c layout.Constraint) layout.Measurement {
	size := geometry.Size{Width: 80, Height: 10}
	if c.Max.Width < size.Width {
		size.Width = c.Max.Width
		size.Height = 20
	}
	return layout.MeasuredWithBaseline(c.Clamp(size), 7)
}

func TestButtonPaintsBackgroundForPointerStates(t *testing.T) {
	button := NewButton()
	button.Arrange(geometry.Rect(0, 0, 80, 30))
	win := &window{}
	win.SetWidget(button)

	painter := new(testButtonBackgroundPainter)
	button.Paint(painter)
	if painter.brush != graphics.RGB(210, 210, 210) {
		t.Fatalf("unexpected normal background: %+v", painter.brush)
	}

	if err := win.DispatchEvent(events.PointerEvent{
		EventType: events.PointerMove,
		Position:  geometry.Point{X: 10, Y: 10},
	}); err != nil {
		t.Fatal(err)
	}
	painter = new(testButtonBackgroundPainter)
	button.Paint(painter)
	if painter.brush != graphics.RGB(230, 230, 230) {
		t.Fatalf("unexpected hover background: %+v", painter.brush)
	}

	if err := win.DispatchEvent(events.PointerEvent{
		EventType: events.PointerMove,
		Position:  geometry.Point{X: 100, Y: 100},
	}); err != nil {
		t.Fatal(err)
	}
	painter = new(testButtonBackgroundPainter)
	button.Paint(painter)
	if painter.brush != graphics.RGB(210, 210, 210) {
		t.Fatalf("unexpected hover leave background: %+v", painter.brush)
	}

	if err := win.DispatchEvent(events.PointerEvent{
		EventType: events.PointerMove,
		Position:  geometry.Point{X: 10, Y: 10},
	}); err != nil {
		t.Fatal(err)
	}
	if err := win.DispatchEvent(events.PointerEvent{
		EventType: events.PointerDown,
		Position:  geometry.Point{X: 10, Y: 10},
		Button:    events.PointerButtonLeft,
		Buttons:   events.PointerButtonLeftDown,
	}); err != nil {
		t.Fatal(err)
	}
	painter = new(testButtonBackgroundPainter)
	button.Paint(painter)
	if painter.brush != graphics.RGB(180, 180, 180) {
		t.Fatalf("unexpected pressed background: %+v", painter.brush)
	}

	if err := win.DispatchEvent(events.PointerEvent{
		EventType: events.PointerMove,
		Position:  geometry.Point{X: 100, Y: 100},
	}); err != nil {
		t.Fatal(err)
	}
	painter = new(testButtonBackgroundPainter)
	button.Paint(painter)
	if painter.brush != graphics.RGB(210, 210, 210) {
		t.Fatalf("unexpected pressed leave background: %+v", painter.brush)
	}
}
func TestButtonHoverUsesContainedChildHover(t *testing.T) {
	button := NewButton()
	child := newTestWidget()
	button.SetChild(child)
	button.Arrange(geometry.Rect(0, 0, 80, 30))
	child.Arrange(geometry.Rect(0, 0, 80, 30))
	win := &window{}
	win.SetWidget(button)

	if err := win.DispatchEvent(events.PointerEvent{
		EventType: events.PointerMove,
		Position:  geometry.Point{X: 10, Y: 10},
	}); err != nil {
		t.Fatal(err)
	}

	painter := new(testButtonBackgroundPainter)
	button.Paint(painter)
	if painter.brush != graphics.RGB(230, 230, 230) {
		t.Fatalf("button should hover while child is hovered, got %+v", painter.brush)
	}
}

func TestButtonClickedSignal(t *testing.T) {
	button := NewButton()
	button.Arrange(geometry.Rect(0, 0, 80, 30))
	win := &window{}
	win.SetWidget(button)

	clicked := 0
	button.ConnectClicked(func() {
		clicked++
	})

	if err := win.DispatchEvent(events.PointerEvent{
		EventType: events.PointerDown,
		Position:  geometry.Point{X: 10, Y: 10},
		Button:    events.PointerButtonLeft,
		Buttons:   events.PointerButtonLeftDown,
	}); err != nil {
		t.Fatal(err)
	}
	if err := win.DispatchEvent(events.PointerEvent{
		EventType: events.PointerUp,
		Position:  geometry.Point{X: 10, Y: 10},
		Button:    events.PointerButtonLeft,
	}); err != nil {
		t.Fatal(err)
	}

	if clicked != 1 {
		t.Fatalf("unexpected clicked count: %d", clicked)
	}
}

func TestButtonClickedSignalThroughChildContent(t *testing.T) {
	button := NewButton()
	child := newTestWidget()
	button.SetChild(child)
	button.Arrange(geometry.Rect(0, 0, 80, 30))
	child.Arrange(geometry.Rect(0, 0, 80, 30))
	win := &window{}
	win.SetWidget(button)

	clicked := 0
	button.ConnectClicked(func() {
		clicked++
	})

	if err := win.DispatchEvent(events.PointerEvent{
		EventType: events.PointerDown,
		Position:  geometry.Point{X: 10, Y: 10},
		Button:    events.PointerButtonLeft,
		Buttons:   events.PointerButtonLeftDown,
	}); err != nil {
		t.Fatal(err)
	}
	if err := win.DispatchEvent(events.PointerEvent{
		EventType: events.PointerUp,
		Position:  geometry.Point{X: 10, Y: 10},
		Button:    events.PointerButtonLeft,
	}); err != nil {
		t.Fatal(err)
	}

	if clicked != 1 {
		t.Fatalf("unexpected clicked count: %d", clicked)
	}
}

func TestButtonClickHandlesChildDownAndButtonUp(t *testing.T) {
	button := NewButton()
	child := newTestWidget()
	button.SetChild(child)
	button.Arrange(geometry.Rect(0, 0, 80, 30))
	child.Arrange(geometry.Rect(0, 0, 80, 30))
	win := &window{}
	win.SetWidget(button)

	clicked := 0
	button.ConnectClicked(func() {
		clicked++
	})

	if err := win.DispatchEvent(events.PointerEvent{
		EventType: events.PointerDown,
		Position:  geometry.Point{X: 10, Y: 10},
		Button:    events.PointerButtonLeft,
		Buttons:   events.PointerButtonLeftDown,
	}); err != nil {
		t.Fatal(err)
	}

	child.SetVisible(false)
	if err := win.DispatchEvent(events.PointerEvent{
		EventType: events.PointerUp,
		Position:  geometry.Point{X: 10, Y: 10},
		Button:    events.PointerButtonLeft,
	}); err != nil {
		t.Fatal(err)
	}

	if clicked != 1 {
		t.Fatalf("unexpected clicked count: %d", clicked)
	}
}

func TestButtonClickIsCanceledAfterPointerLeaves(t *testing.T) {
	button := NewButton()
	button.Arrange(geometry.Rect(0, 0, 80, 30))
	win := &window{}
	win.SetWidget(button)

	clicked := 0
	button.ConnectClicked(func() {
		clicked++
	})

	if err := win.DispatchEvent(events.PointerEvent{
		EventType: events.PointerDown,
		Position:  geometry.Point{X: 10, Y: 10},
		Button:    events.PointerButtonLeft,
		Buttons:   events.PointerButtonLeftDown,
	}); err != nil {
		t.Fatal(err)
	}
	if err := win.DispatchEvent(events.PointerEvent{
		EventType: events.PointerMove,
		Position:  geometry.Point{X: 100, Y: 100},
		Buttons:   events.PointerButtonLeftDown,
	}); err != nil {
		t.Fatal(err)
	}
	if err := win.DispatchEvent(events.PointerEvent{
		EventType: events.PointerMove,
		Position:  geometry.Point{X: 10, Y: 10},
		Buttons:   events.PointerButtonLeftDown,
	}); err != nil {
		t.Fatal(err)
	}
	if err := win.DispatchEvent(events.PointerEvent{
		EventType: events.PointerUp,
		Position:  geometry.Point{X: 10, Y: 10},
		Button:    events.PointerButtonLeft,
	}); err != nil {
		t.Fatal(err)
	}

	if clicked != 0 {
		t.Fatalf("unexpected clicked count: %d", clicked)
	}
}

func TestButtonDoesNotClickWithoutPointerDown(t *testing.T) {
	button := NewButton()
	button.Arrange(geometry.Rect(0, 0, 80, 30))
	win := &window{}
	win.SetWidget(button)

	clicked := 0
	button.ConnectClicked(func() {
		clicked++
	})

	if err := win.DispatchEvent(events.PointerEvent{
		EventType: events.PointerUp,
		Position:  geometry.Point{X: 10, Y: 10},
		Button:    events.PointerButtonLeft,
	}); err != nil {
		t.Fatal(err)
	}

	if clicked != 0 {
		t.Fatalf("unexpected clicked count: %d", clicked)
	}
}

func TestButtonIgnoresNonLeftButton(t *testing.T) {
	button := NewButton()
	button.Arrange(geometry.Rect(0, 0, 80, 30))
	win := &window{}
	win.SetWidget(button)

	clicked := 0
	button.ConnectClicked(func() {
		clicked++
	})

	if err := win.DispatchEvent(events.PointerEvent{
		EventType: events.PointerDown,
		Position:  geometry.Point{X: 10, Y: 10},
		Button:    events.PointerButtonRight,
		Buttons:   events.PointerButtonRightDown,
	}); err != nil {
		t.Fatal(err)
	}
	if err := win.DispatchEvent(events.PointerEvent{
		EventType: events.PointerUp,
		Position:  geometry.Point{X: 10, Y: 10},
		Button:    events.PointerButtonRight,
	}); err != nil {
		t.Fatal(err)
	}

	if clicked != 0 {
		t.Fatalf("unexpected clicked count: %d", clicked)
	}
}

func TestButtonSetChildReplaces(t *testing.T) {
	button := NewButton()
	first := newPaintCountingWidget()
	second := newPaintCountingWidget()

	// SetChild is the single-content API; it replaces the previous content.
	button.SetChild(first)
	if button.Child() != first {
		t.Fatal("SetChild should set the content")
	}
	children := button.Children()
	if len(children) != 1 || children[0] != first {
		t.Fatalf("content should be the only child, got %v", children)
	}

	button.SetChild(second)
	if button.Child() != second {
		t.Fatal("SetChild should replace the previous content")
	}
	children = button.Children()
	if len(children) != 1 || children[0] != second {
		t.Fatalf("content should be the only child after replace, got %v", children)
	}

	// Clear via SetChild(nil).
	button.SetChild(nil)
	if button.Child() != nil {
		t.Fatal("SetChild(nil) should clear the content")
	}
	if len(button.Children()) != 0 {
		t.Fatal("children should be empty after clear")
	}
}

func TestButtonAddChildBypassIsRefused(t *testing.T) {
	button := NewButton()
	label := NewLabel("stray")

	// Direct mounting on a Bin without SetChild registration is refused: the
	// child slot stays empty and the tree stays clean.
	button.WidgetBase.AddChild(button, label)
	if button.Child() != nil {
		t.Fatal("direct AddChild must not desync the child slot")
	}
	if len(button.Children()) != 0 {
		t.Fatalf("direct AddChild must be refused, got children %v", button.Children())
	}

	// The semantic path still works after the refused bypass.
	button.SetChild(label)
	if button.Child() != label || len(button.Children()) != 1 {
		t.Fatal("SetChild should still work after a refused bypass")
	}
}

type paintCountingWidget struct {
	WidgetBase
	paints int
}

func newPaintCountingWidget() *paintCountingWidget {
	return new(paintCountingWidget)
}

func (w *paintCountingWidget) Paint(p Painter) {
	w.paints++
}

type testButtonBackgroundPainter struct {
	rect            geometry.Rectangle
	radius          float32
	brush           graphics.Brush
	drawRect        geometry.Rectangle
	drawRadius      float32
	drawStrokeWidth float32
	drawBrush       graphics.Brush
}

func (p *testButtonBackgroundPainter) Save()               {}
func (p *testButtonBackgroundPainter) Restore()            {}
func (p *testButtonBackgroundPainter) PixelScale() float32 { return 1 }

func (p *testButtonBackgroundPainter) NewImage(src image.Image) (graphics.Image, error) {
	return newTestNativeImage(src), nil
}

func (p *testButtonBackgroundPainter) SetClipRect(rect geometry.Rectangle) {}

func (p *testButtonBackgroundPainter) FillRect(rect geometry.Rectangle, brush graphics.Brush) {
	p.rect = rect
	p.brush = brush
}

func (p *testButtonBackgroundPainter) FillRoundRect(rect geometry.Rectangle, radius float32, brush graphics.Brush) {
	p.rect = rect
	p.radius = radius
	p.brush = brush
}

func (p *testButtonBackgroundPainter) FillEllipse(center geometry.Point, xRadius, yRadius float32, brush graphics.Brush) {
}

func (p *testButtonBackgroundPainter) FillPath(path graphics.Path, brush graphics.Brush) {}

func (p *testButtonBackgroundPainter) DrawLine(p0, p1 geometry.Point, strokeWidth float32, brush graphics.Brush) {
}

func (p *testButtonBackgroundPainter) DrawRect(rect geometry.Rectangle, strokeWidth float32, brush graphics.Brush) {
	p.drawRect = rect
	p.drawStrokeWidth = strokeWidth
	p.drawBrush = brush
}

func (p *testButtonBackgroundPainter) DrawRoundRect(rect geometry.Rectangle, radius, strokeWidth float32, brush graphics.Brush) {
	p.drawRect = rect
	p.drawRadius = radius
	p.drawStrokeWidth = strokeWidth
	p.drawBrush = brush
}

func (p *testButtonBackgroundPainter) DrawEllipse(center geometry.Point, xRadius, yRadius, strokeWidth float32, brush graphics.Brush) {
}

func (p *testButtonBackgroundPainter) DrawPath(path graphics.Path, strokeWidth float32, brush graphics.Brush) {
}

func (p *testButtonBackgroundPainter) DrawTextLayout(origin geometry.Point, layout typography.TextLayout) {
}

func (p *testButtonBackgroundPainter) DrawImage(rect geometry.Rectangle, img graphics.Image) {}
func (p *testButtonBackgroundPainter) SetTransform(matrix geometry.Transform)                {}
