package gui

import (
	"image"
	"math"
	"testing"

	"github.com/golang-gui/goui/core/geometry"
	"github.com/golang-gui/goui/layout"
	"github.com/golang-gui/goui/platform/events"
	"github.com/golang-gui/goui/platform/graphics"
	"github.com/golang-gui/goui/platform/typography"
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
	paintWidget(button, newPainter(backend, geometry.Rect(0, 0, 80, 30)))
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

func (p *testButtonBackgroundPainter) Save()    {}
func (p *testButtonBackgroundPainter) Restore() {}

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
