package gui

import (
	"fmt"
	"image"
	"strings"
	"testing"

	"github.com/golang-gui/goui/core/geometry"
	"github.com/golang-gui/goui/platform/graphics"
	"github.com/golang-gui/goui/platform/typography"
)

func TestPainterUsesWidgetLocalCoordinates(t *testing.T) {
	backend := new(recordingPainterBackend)
	bounds := geometry.Rect(0, 0, 200, 100)
	widget := newPainterTestWidget(func(p Painter) {
		p.SetClipRect(geometry.Rect(4, 6, 20, 10))
		p.FillRect(geometry.Rect(1, 2, 3, 4), graphics.RGB(1, 2, 3))
	})
	widget.Arrange(geometry.Rect(10, 5, 100, 80))

	paintWidget(widget, newPainter(backend, bounds))

	if len(backend.fills) != 1 {
		t.Fatalf("expected one fill, got %d", len(backend.fills))
	}
	fill := backend.fills[0]
	if fill.rect != geometry.Rect(1, 2, 3, 4) {
		t.Fatalf("draw coordinates must remain Widget-local, got %+v", fill.rect)
	}
	if fill.clip != geometry.Rect(14, 11, 20, 10) {
		t.Fatalf("unexpected translated clip: %+v", fill.clip)
	}
	if fill.transform != geometry.Translate(10, 5) {
		t.Fatalf("unexpected Widget transform: %+v", fill.transform)
	}
	if backend.clip != (geometry.Rectangle{}) {
		t.Fatalf("subtree exit must restore the full target clip, got %+v", backend.clip)
	}
	if backend.transform != geometry.Identity() {
		t.Fatalf("subtree exit must restore the identity transform, got %+v", backend.transform)
	}
}

func TestPainterAutomaticallyPaintsChildrenInTreeOrder(t *testing.T) {
	backend := new(recordingPainterBackend)
	bounds := geometry.Rect(0, 0, 200, 100)
	parent := newPainterTestWidget(fillAt(1))
	first := newPainterTestWidget(fillAt(2))
	hidden := newPainterTestWidget(fillAt(3))
	last := newPainterTestWidget(fillAt(4))
	parent.Arrange(bounds)
	first.Arrange(geometry.Rect(0, 0, 20, 20))
	hidden.Arrange(geometry.Rect(20, 0, 20, 20))
	last.Arrange(geometry.Rect(40, 0, 20, 20))
	hidden.SetVisible(false)
	parent.AddChild(first)
	parent.AddChild(hidden)
	parent.AddChild(last)

	paintWidget(parent, newPainter(backend, bounds))

	if parent.paints != 1 || first.paints != 1 || hidden.paints != 0 || last.paints != 1 {
		t.Fatalf("unexpected paint counts: parent=%d first=%d hidden=%d last=%d",
			parent.paints, first.paints, hidden.paints, last.paints)
	}
	if len(backend.fills) != 3 {
		t.Fatalf("expected three visible fills, got %d", len(backend.fills))
	}
	for i, wantX := range []float32{1, 2, 4} {
		if backend.fills[i].rect.X != wantX {
			t.Fatalf("fill %d has X=%v, want %v", i, backend.fills[i].rect.X, wantX)
		}
	}
}

func TestWidgetBaseDefaultPaintStillTraversesChildren(t *testing.T) {
	backend := new(recordingPainterBackend)
	bounds := geometry.Rect(0, 0, 100, 100)
	parent := new(painterTestContainer)
	child := newPainterTestWidget(fillAt(7))
	parent.Arrange(bounds)
	child.Arrange(geometry.Rect(5, 6, 20, 20))
	parent.AddChild(child)

	paintWidget(parent, newPainter(backend, bounds))

	if child.paints != 1 || len(backend.fills) != 1 {
		t.Fatalf("WidgetBase.Paint should be a no-op without suppressing descendants: paints=%d fills=%d",
			child.paints, len(backend.fills))
	}
}

func TestPainterRestoresParentStateBeforePaintingChild(t *testing.T) {
	backend := new(recordingPainterBackend)
	bounds := geometry.Rect(0, 0, 200, 100)
	parent := newPainterTestWidget(func(p Painter) {
		p.SetClipRect(geometry.Rect(1, 1, 5, 5))
		p.SetTransform(geometry.Scale(2, 2))
	})
	child := newPainterTestWidget(func(p Painter) {
		p.FillRect(geometry.Rect(0, 0, 30, 20), graphics.RGB(1, 2, 3))
	})
	parent.Arrange(geometry.Rect(5, 5, 100, 80))
	child.Arrange(geometry.Rect(20, 10, 30, 20))
	parent.AddChild(child)

	paintWidget(parent, newPainter(backend, bounds))

	if len(backend.fills) != 1 {
		t.Fatalf("expected child fill, got %d", len(backend.fills))
	}
	fill := backend.fills[0]
	if fill.clip != geometry.Rect(25, 15, 30, 20) {
		t.Fatalf("parent explicit clip leaked to child: %+v", fill.clip)
	}
	if fill.transform != geometry.Translate(25, 15) {
		t.Fatalf("parent transform leaked to child: %+v", fill.transform)
	}
}

func TestPainterStructuralViewportClipDoesNotMoveWithContent(t *testing.T) {
	backend := new(recordingPainterBackend)
	bounds := geometry.Rect(0, 0, 200, 200)
	root := newPainterTestWidget(nil)
	viewport := newPainterTestWidget(nil)
	content := newPainterTestWidget(func(p Painter) {
		p.FillRect(geometry.Rect(0, 0, 100, 300), graphics.RGB(1, 2, 3))
	})
	root.Arrange(bounds)
	viewport.Arrange(geometry.Rect(20, 20, 100, 100))
	content.Arrange(geometry.Rect(0, -50, 100, 300))
	root.AddChild(viewport)
	viewport.AddChild(content)

	paintWidget(root, newPainter(backend, bounds))

	if len(backend.fills) != 1 {
		t.Fatalf("expected one fill, got %d", len(backend.fills))
	}
	fill := backend.fills[0]
	if fill.clip != geometry.Rect(20, 20, 100, 100) {
		t.Fatalf("viewport clip moved with content: %+v", fill.clip)
	}
	if fill.transform != geometry.Translate(20, -30) {
		t.Fatalf("unexpected scrolled content transform: %+v", fill.transform)
	}
}

func TestPainterSuppressesEmptyClipAndCanSetAnotherClip(t *testing.T) {
	backend := new(recordingPainterBackend)
	bounds := geometry.Rect(0, 0, 100, 100)
	widget := newPainterTestWidget(func(p Painter) {
		p.SetClipRect(geometry.Rect(200, 200, 10, 10))
		p.FillRect(bounds, graphics.RGB(1, 2, 3))

		// SetClipRect replaces the explicit clip within this Widget's structural
		// boundary; an empty preceding clip does not make later clips empty.
		p.SetClipRect(geometry.Rect(10, 10, 20, 20))
		p.FillRect(bounds, graphics.RGB(4, 5, 6))
	})
	widget.Arrange(bounds)

	paintWidget(widget, newPainter(backend, bounds))

	if len(backend.fills) != 1 {
		t.Fatalf("expected drawing to resume with a non-empty clip, got %d fills", len(backend.fills))
	}
	if backend.fills[0].clip != geometry.Rect(10, 10, 20, 20) {
		t.Fatalf("unexpected replacement clip: %+v", backend.fills[0].clip)
	}
}

func TestPainterSaveRestoreRestoresClipAndTransform(t *testing.T) {
	backend := new(recordingPainterBackend)
	bounds := geometry.Rect(0, 0, 100, 100)
	widget := newPainterTestWidget(func(p Painter) {
		p.FillRect(geometry.Rect(1, 0, 1, 1), graphics.RGB(1, 2, 3))
		p.Save()
		p.SetClipRect(geometry.Rect(2, 3, 10, 11))
		p.FillRect(geometry.Rect(2, 0, 1, 1), graphics.RGB(1, 2, 3))
		p.Save()
		p.SetTransform(geometry.Scale(2, 3))
		p.FillRect(geometry.Rect(3, 0, 1, 1), graphics.RGB(1, 2, 3))
		p.Restore()
		p.FillRect(geometry.Rect(4, 0, 1, 1), graphics.RGB(1, 2, 3))
		p.Restore()
		p.FillRect(geometry.Rect(5, 0, 1, 1), graphics.RGB(1, 2, 3))
	})
	widget.Arrange(geometry.Rect(10, 5, 80, 70))

	paintWidget(widget, newPainter(backend, bounds))

	if len(backend.fills) != 5 {
		t.Fatalf("expected five fills, got %d", len(backend.fills))
	}
	defaultClip := geometry.Rect(10, 5, 80, 70)
	defaultTransform := geometry.Translate(10, 5)
	if backend.fills[0].clip != defaultClip || backend.fills[0].transform != defaultTransform {
		t.Fatalf("unexpected initial state: %+v", backend.fills[0])
	}
	narrowClip := geometry.Rect(12, 8, 10, 11)
	if backend.fills[1].clip != narrowClip || backend.fills[1].transform != defaultTransform {
		t.Fatalf("unexpected outer saved state: %+v", backend.fills[1])
	}
	wantTransform := defaultTransform.Multiply(geometry.Scale(2, 3))
	if backend.fills[2].clip != narrowClip || backend.fills[2].transform != wantTransform {
		t.Fatalf("unexpected nested saved state: %+v", backend.fills[2])
	}
	if backend.fills[3].clip != narrowClip || backend.fills[3].transform != defaultTransform {
		t.Fatalf("inner Restore did not restore the outer state: %+v", backend.fills[3])
	}
	if backend.fills[4].clip != defaultClip || backend.fills[4].transform != defaultTransform {
		t.Fatalf("outer Restore did not restore initial state: %+v", backend.fills[4])
	}
}

func TestPainterRejectsRestoreAcrossWidgetBoundary(t *testing.T) {
	backend := new(recordingPainterBackend)
	bounds := geometry.Rect(0, 0, 100, 100)
	widget := newPainterTestWidget(func(p Painter) { p.Restore() })
	widget.SetID("broken")
	widget.Arrange(bounds)

	message := recoverMessage(func() {
		paintWidget(widget, newPainter(backend, bounds))
	})
	if !strings.Contains(message, "without matching Save") || !strings.Contains(message, `id "broken"`) {
		t.Fatalf("unexpected panic: %q", message)
	}
	if backend.clip != (geometry.Rectangle{}) || backend.transform != geometry.Identity() {
		t.Fatalf("failed Paint must restore backend state: clip=%+v transform=%+v", backend.clip, backend.transform)
	}
}

func TestPainterReportsUnbalancedSaveAndRestoresState(t *testing.T) {
	backend := new(recordingPainterBackend)
	bounds := geometry.Rect(0, 0, 100, 100)
	p := newPainter(backend, bounds)
	broken := newPainterTestWidget(func(p Painter) {
		p.Save()
		p.SetClipRect(geometry.Rect(1, 2, 3, 4))
	})
	broken.SetID("broken")
	broken.Arrange(bounds)

	message := recoverMessage(func() { paintWidget(broken, p) })
	if !strings.Contains(message, "1 unbalanced Painter.Save") || !strings.Contains(message, `id "broken"`) {
		t.Fatalf("unexpected panic: %q", message)
	}

	healthy := newPainterTestWidget(func(p Painter) {
		p.FillRect(bounds, graphics.RGB(1, 2, 3))
	})
	healthy.Arrange(bounds)
	paintWidget(healthy, p)
	if len(backend.fills) != 1 || backend.fills[0].clip != (geometry.Rectangle{}) {
		t.Fatalf("painter was not reusable after an unbalanced Save: %+v", backend.fills)
	}
}

func TestPainterRestoresStateAfterWidgetPanic(t *testing.T) {
	backend := new(recordingPainterBackend)
	bounds := geometry.Rect(0, 0, 100, 100)
	p := newPainter(backend, bounds)
	broken := newPainterTestWidget(func(p Painter) {
		p.Save()
		p.SetClipRect(geometry.Rect(1, 2, 3, 4))
		p.SetTransform(geometry.Scale(2, 2))
		panic("paint failed")
	})
	broken.Arrange(bounds)

	message := recoverMessage(func() { paintWidget(broken, p) })
	if message != "paint failed" {
		t.Fatalf("original Paint panic was not preserved: %q", message)
	}
	if backend.clip != (geometry.Rectangle{}) || backend.transform != geometry.Identity() {
		t.Fatalf("panic did not restore backend state: clip=%+v transform=%+v", backend.clip, backend.transform)
	}

	healthy := newPainterTestWidget(fillAt(9))
	healthy.Arrange(bounds)
	paintWidget(healthy, p)
	if len(backend.fills) != 1 || backend.fills[0].rect.X != 9 {
		t.Fatalf("painter was not reusable after panic: %+v", backend.fills)
	}
}

func recoverMessage(fn func()) (message string) {
	defer func() {
		if value := recover(); value != nil {
			message = fmt.Sprint(value)
		}
	}()
	fn()
	return ""
}

func fillAt(x float32) func(Painter) {
	return func(p Painter) {
		p.FillRect(geometry.Rect(x, 0, 1, 1), graphics.RGB(1, 2, 3))
	}
}

type painterTestWidget struct {
	WidgetBase
	paint  func(Painter)
	paints int
}

type painterTestContainer struct {
	WidgetBase
}

func (w *painterTestContainer) AddChild(child Widget) {
	w.WidgetBase.AddChild(w, child)
}

func newPainterTestWidget(paint func(Painter)) *painterTestWidget {
	return &painterTestWidget{paint: paint}
}

func (w *painterTestWidget) AddChild(child Widget) {
	w.WidgetBase.AddChild(w, child)
}

func (w *painterTestWidget) Paint(p Painter) {
	w.paints++
	if w.paint != nil {
		w.paint(p)
	}
}

type recordedFill struct {
	rect      geometry.Rectangle
	clip      geometry.Rectangle
	transform geometry.Transform
}

type recordingPainterBackend struct {
	clip      geometry.Rectangle
	transform geometry.Transform
	fills     []recordedFill
}

func (p *recordingPainterBackend) Name() string { return "recording" }
func (p *recordingPainterBackend) Destroy()     {}
func (p *recordingPainterBackend) NewImage(src image.Image) (graphics.Image, error) {
	return newTestNativeImage(src), nil
}
func (p *recordingPainterBackend) Begin(width, height, scale float32) {}
func (p *recordingPainterBackend) End()                               {}
func (p *recordingPainterBackend) SetClipRect(rect graphics.Rectangle) {
	p.clip = rect
}
func (p *recordingPainterBackend) Clear(color graphics.Color) {}
func (p *recordingPainterBackend) DrawBoxShadow(rect graphics.Rectangle, radius float32, shadow graphics.BoxShadow) {
}
func (p *recordingPainterBackend) FillRect(rect graphics.Rectangle, brush graphics.Brush) {
	p.fills = append(p.fills, recordedFill{rect: rect, clip: p.clip, transform: p.transform})
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
func (p *recordingPainterBackend) DrawImage(rect graphics.Rectangle, img graphics.Image) {}
func (p *recordingPainterBackend) SetTransform(matrix geometry.Transform) {
	p.transform = matrix
}
