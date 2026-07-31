package gui

import (
	"image"
	"testing"

	"github.com/golang-gui/goui/core/geometry"
	"github.com/golang-gui/goui/platform/graphics"
	"github.com/golang-gui/goui/platform/typography"
)

func TestSubPainterUsesWidgetLocalCoordinates(t *testing.T) {
	backend := new(recordingPainterBackend)
	bounds := geometry.Rect(0, 0, 200, 100)
	p := SubPainter(newPainter(backend, bounds), geometry.Rect(10, 5, 100, 80))

	p.Begin()
	p.SetClipRect(geometry.Rect(4, 6, 20, 10))
	p.FillRect(geometry.Rect(1, 2, 3, 4), graphics.RGB(1, 2, 3))
	p.End()

	if len(backend.fills) != 1 {
		t.Fatalf("expected one fill, got %d", len(backend.fills))
	}
	fill := backend.fills[0]
	if fill.rect != geometry.Rect(1, 2, 3, 4) {
		t.Fatalf("unexpected rect: %+v (transform handles offset)", fill.rect)
	}
	if fill.clip != geometry.Rect(14, 11, 20, 10) {
		t.Fatalf("unexpected translated clip: %+v", fill.clip)
	}
	if backend.clip != (geometry.Rectangle{}) {
		t.Fatalf("outer End should restore the unclipped target, got %+v", backend.clip)
	}
}

func TestPainterRestoresClipBetweenSiblings(t *testing.T) {
	backend := new(recordingPainterBackend)
	bounds := geometry.Rect(0, 0, 200, 100)
	root := SubPainter(newPainter(backend, bounds), bounds)
	root.Begin()

	first := SubPainter(root, geometry.Rect(0, 0, 80, 100))
	first.Begin()
	first.SetClipRect(geometry.Rect(10, 10, 20, 20))
	first.FillRect(geometry.Rect(0, 0, 80, 100), graphics.RGB(1, 2, 3))
	first.End()

	second := SubPainter(root, geometry.Rect(100, 0, 80, 100))
	second.Begin()
	second.FillRect(geometry.Rect(0, 0, 80, 100), graphics.RGB(4, 5, 6))
	second.End()
	root.End()

	if len(backend.fills) != 2 {
		t.Fatalf("expected two fills, got %d", len(backend.fills))
	}
	if backend.fills[0].clip != geometry.Rect(10, 10, 20, 20) {
		t.Fatalf("unexpected first child clip: %+v", backend.fills[0].clip)
	}
	if backend.fills[1].clip != geometry.Rect(100, 0, 80, 100) {
		t.Fatalf("first child clip leaked to sibling: %+v", backend.fills[1].clip)
	}
}

func TestScrollViewportClipDoesNotMoveWithContent(t *testing.T) {
	backend := new(recordingPainterBackend)
	bounds := geometry.Rect(0, 0, 200, 200)
	root := SubPainter(newPainter(backend, bounds), bounds)
	root.Begin()

	scroll := SubPainter(root, geometry.Rect(20, 20, 100, 100))
	scroll.Begin()
	scroll.SetClipRect(geometry.Rect(0, 0, 100, 100))

	content := SubPainter(scroll, geometry.Rect(0, -50, 100, 300))
	content.Begin()
	content.FillRect(geometry.Rect(0, 0, 100, 300), graphics.RGB(1, 2, 3))
	content.End()
	scroll.End()
	root.End()

	if len(backend.fills) != 1 {
		t.Fatalf("expected one fill, got %d", len(backend.fills))
	}
	fill := backend.fills[0]
	if fill.rect != geometry.Rect(0, 0, 100, 300) {
		t.Fatalf("content rect should be widget-local: %+v", fill.rect)
	}
	if fill.clip != geometry.Rect(20, 20, 100, 100) {
		t.Fatalf("viewport clip moved with content: %+v", fill.clip)
	}
}

func TestPainterSuppressesEmptyClipAndCanSetAnotherClip(t *testing.T) {
	backend := new(recordingPainterBackend)
	bounds := geometry.Rect(0, 0, 100, 100)
	p := SubPainter(newPainter(backend, bounds), bounds)
	p.Begin()

	p.SetClipRect(geometry.Rect(200, 200, 10, 10))
	p.FillRect(bounds, graphics.RGB(1, 2, 3))
	if len(backend.fills) != 0 {
		t.Fatal("an empty GUI clip must suppress drawing")
	}

	// SetClipRect replaces the clip within this Widget's inherited scope; an
	// empty preceding clip must not make all later clips irreversibly empty.
	p.SetClipRect(geometry.Rect(10, 10, 20, 20))
	p.FillRect(bounds, graphics.RGB(4, 5, 6))
	p.End()

	if len(backend.fills) != 1 {
		t.Fatalf("expected drawing to resume with a non-empty clip, got %d fills", len(backend.fills))
	}
	if backend.fills[0].clip != geometry.Rect(10, 10, 20, 20) {
		t.Fatalf("unexpected restored clip: %+v", backend.fills[0].clip)
	}
}

func TestPaintChildrenOwnsChildPainterScope(t *testing.T) {
	parent := new(WidgetBase)
	child := new(scopeTestWidget)
	child.Arrange(geometry.Rect(3, 4, 20, 10))
	parent.children = []Widget{child}

	p := new(scopeTestPainter)
	parent.PaintChildren(p)

	if child.paints != 1 {
		t.Fatalf("expected child to paint once, got %d", child.paints)
	}
	if p.begins != 1 || p.ends != 1 {
		t.Fatalf("parent must own one balanced child scope, begins=%d ends=%d", p.begins, p.ends)
	}
	if p.clip != geometry.Rect(0, 0, 20, 10) {
		t.Fatalf("child scope was not initialized from its rect: %+v", p.clip)
	}
}

func TestPaintWidgetRestoresScopeAfterPanic(t *testing.T) {
	widget := &scopeTestWidget{panicOnPaint: true}
	p := new(scopeTestPainter)

	func() {
		defer func() {
			if recover() == nil {
				t.Fatal("expected Widget.Paint panic")
			}
		}()
		paintWidget(widget, p)
	}()

	if p.begins != 1 || p.ends != 1 {
		t.Fatalf("Painter scope must be restored after panic, begins=%d ends=%d", p.begins, p.ends)
	}
}

type recordedFill struct {
	rect geometry.Rectangle
	clip geometry.Rectangle
}

type recordingPainterBackend struct {
	clip  geometry.Rectangle
	fills []recordedFill
}

func (p *recordingPainterBackend) Name() string { return "recording" }
func (p *recordingPainterBackend) Destroy()     {}
func (p *recordingPainterBackend) Begin(width, height, scale float32) {
}
func (p *recordingPainterBackend) End() {}
func (p *recordingPainterBackend) SetClipRect(rect graphics.Rectangle) {
	p.clip = rect
}
func (p *recordingPainterBackend) Clear(color graphics.Color) {}
func (p *recordingPainterBackend) FillRect(rect graphics.Rectangle, brush graphics.Brush) {
	p.fills = append(p.fills, recordedFill{rect: rect, clip: p.clip})
}
func (p *recordingPainterBackend) FillRoundRect(rect graphics.Rectangle, radius float32, brush graphics.Brush) {
}
func (p *recordingPainterBackend) FillEllipse(center graphics.Point, xRadius, yRadius float32, brush graphics.Brush) {
}
func (p *recordingPainterBackend) FillPath(path graphics.Path, brush graphics.Brush) {}
func (p *recordingPainterBackend) DrawLine(p0, p1 graphics.Point, strokeWidth float32, brush graphics.Brush) {
}
func (p *recordingPainterBackend) DrawRect(rect graphics.Rectangle, strokeWidth float32, brush graphics.Brush) {
}
func (p *recordingPainterBackend) DrawRoundRect(rect graphics.Rectangle, radius, strokeWidth float32, brush graphics.Brush) {
}
func (p *recordingPainterBackend) DrawEllipse(center graphics.Point, xRadius, yRadius, strokeWidth float32, brush graphics.Brush) {
}
func (p *recordingPainterBackend) DrawPath(path graphics.Path, strokeWidth float32, brush graphics.Brush) {
}
func (p *recordingPainterBackend) DrawTextLayout(origin graphics.Point, layout typography.TextLayout) {
}
func (p *recordingPainterBackend) DrawImage(rect graphics.Rectangle, img image.Image) {}
func (p *recordingPainterBackend) SetTransform(matrix geometry.Transform)             {}

type scopeTestWidget struct {
	WidgetBase
	paints       int
	panicOnPaint bool
}

func (w *scopeTestWidget) Paint(Painter) {
	w.paints++
	if w.panicOnPaint {
		panic("paint failed")
	}
}

type scopeTestPainter struct {
	Painter
	begins int
	ends   int
	clip   geometry.Rectangle
}

func (p *scopeTestPainter) Begin() {
	p.begins++
}

func (p *scopeTestPainter) End() {
	p.ends++
}

func (p *scopeTestPainter) SetClipRect(rect geometry.Rectangle) {
	p.clip = rect
}
