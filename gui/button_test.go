package gui

import (
	"image"
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
	button.AddChild(child)

	size := button.Measure(layout.Loose(geometry.Size{Width: 300, Height: 200}))
	if size != (geometry.Size{Width: 132, Height: 72}) {
		t.Fatalf("unexpected measured size: %+v", size)
	}
	if len(manager.measured) != 1 || manager.measured[0] != child {
		t.Fatalf("layout measured unexpected children: %v", manager.measured)
	}

	button.Arrange(geometry.Rect(0, 0, 80, 30))
	if manager.arrangeRect != geometry.Rect(6, 6, 68, 18) {
		t.Fatalf("unexpected layout arrange rect: %+v", manager.arrangeRect)
	}

	button.Paint(new(testLabelPainter))
	if child.paints != 1 {
		t.Fatalf("expected child paint, got %d", child.paints)
	}
}

func TestButtonDefaultFillLayoutArrangesContent(t *testing.T) {
	button := NewButton()
	child := newTestWidget()
	button.AddChild(child)

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
	button.AddChild(child)
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
	button.AddChild(child)
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
	button.AddChild(child)
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

type paintCountingWidget struct {
	WidgetBase
	paints int
}

func newPaintCountingWidget() *paintCountingWidget {
	return new(paintCountingWidget)
}

func (w *paintCountingWidget) Paint(p Painter) {
	w.paints++
	w.PaintChildren(p)
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

func (p *testButtonBackgroundPainter) SetClipRect(rect geometry.Rectangle) {}

func (p *testButtonBackgroundPainter) Clear(color graphics.Color) {}

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

func (p *testButtonBackgroundPainter) DrawImage(rect geometry.Rectangle, img image.Image) {}
