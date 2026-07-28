package gui

import (
	"image"

	"github.com/golang-gui/goui/core/geometry"
	"github.com/golang-gui/goui/platform/graphics"
	"github.com/golang-gui/goui/platform/typography"
)

type Painter interface {
	Begin()
	End()
	SetClipRect(rect geometry.Rectangle)
	Clear(color graphics.Color)
	FillRect(rect geometry.Rectangle, brush graphics.Brush)
	FillRoundRect(rect geometry.Rectangle, radius float32, brush graphics.Brush)
	FillEllipse(center geometry.Point, xRadius, yRadius float32, brush graphics.Brush)
	FillPath(path graphics.Path, brush graphics.Brush)
	DrawLine(p0, p1 geometry.Point, strokeWidth float32, brush graphics.Brush)
	DrawRect(rect geometry.Rectangle, strokeWidth float32, brush graphics.Brush)
	DrawRoundRect(rect geometry.Rectangle, radius, strokeWidth float32, brush graphics.Brush)
	DrawEllipse(center geometry.Point, xRadius, yRadius, strokeWidth float32, brush graphics.Brush)
	DrawPath(path graphics.Path, strokeWidth float32, brush graphics.Brush)
	DrawTextLayout(origin geometry.Point, layout typography.TextLayout)
	DrawImage(rect geometry.Rectangle, img image.Image)
}

// paintWidget is the only entry point used by the GUI traversal to invoke a
// Widget's Paint method. The caller owns the Painter scope, so Widget.Paint can
// change its local clip without having to restore it before returning.
func paintWidget(widget Widget, p Painter) {
	p.Begin()
	defer p.End()
	widget.Paint(p)
}

// painter adapts the native-surface graphics.Painter to the local,
// scoped Painter consumed by Widgets. Its rectangles are in window-local
// logical coordinates; subPainter is responsible for translating Widget-local
// coordinates into this coordinate space.
type painter struct {
	base   graphics.Painter
	bounds geometry.Rectangle
	state  painterState
	stack  []painterState

	appliedClip geometry.Rectangle
	clipApplied bool
}

type painterState struct {
	// scopeClip is the inherited clip captured by Begin. SetClipRect replaces
	// the current clip within this fixed boundary rather than accumulating with
	// the preceding SetClipRect call.
	scopeClip geometry.Rectangle
	clip      geometry.Rectangle
}

func newPainter(base graphics.Painter, bounds geometry.Rectangle) Painter {
	return &painter{
		base:   base,
		bounds: bounds,
		state: painterState{
			scopeClip: bounds,
			clip:      bounds,
		},
	}
}

func (p *painter) Begin() {
	p.stack = append(p.stack, p.state)
	p.state.scopeClip = p.state.clip
	p.applyClip()
}

func (p *painter) End() {
	if len(p.stack) == 0 {
		panic("gui: unbalanced Painter.End")
	}
	last := len(p.stack) - 1
	p.state = p.stack[last]
	p.stack = p.stack[:last]
	p.applyClip()
}

func (p *painter) SetClipRect(rect geometry.Rectangle) {
	p.state.clip = p.state.scopeClip.Intersect(rect)
	p.applyClip()
}

func (p *painter) Clear(color graphics.Color) {
	if p.canDraw() {
		// A Widget clear is local and clipped. The native Painter.Clear is a
		// whole-surface operation and is used directly by Window instead.
		p.base.FillRect(p.state.clip, color)
	}
}

func (p *painter) FillRect(rect geometry.Rectangle, brush graphics.Brush) {
	if p.canDraw() {
		p.base.FillRect(rect, brush)
	}
}

func (p *painter) FillRoundRect(rect geometry.Rectangle, radius float32, brush graphics.Brush) {
	if p.canDraw() {
		p.base.FillRoundRect(rect, radius, brush)
	}
}

func (p *painter) FillEllipse(center geometry.Point, xRadius, yRadius float32, brush graphics.Brush) {
	if p.canDraw() {
		p.base.FillEllipse(center, xRadius, yRadius, brush)
	}
}

func (p *painter) FillPath(path graphics.Path, brush graphics.Brush) {
	if p.canDraw() {
		p.base.FillPath(path, brush)
	}
}

func (p *painter) DrawLine(p0, p1 geometry.Point, strokeWidth float32, brush graphics.Brush) {
	if p.canDraw() {
		p.base.DrawLine(p0, p1, strokeWidth, brush)
	}
}

func (p *painter) DrawRect(rect geometry.Rectangle, strokeWidth float32, brush graphics.Brush) {
	if p.canDraw() {
		p.base.DrawRect(rect, strokeWidth, brush)
	}
}

func (p *painter) DrawRoundRect(rect geometry.Rectangle, radius, strokeWidth float32, brush graphics.Brush) {
	if p.canDraw() {
		p.base.DrawRoundRect(rect, radius, strokeWidth, brush)
	}
}

func (p *painter) DrawEllipse(center geometry.Point, xRadius, yRadius, strokeWidth float32, brush graphics.Brush) {
	if p.canDraw() {
		p.base.DrawEllipse(center, xRadius, yRadius, strokeWidth, brush)
	}
}

func (p *painter) DrawPath(path graphics.Path, strokeWidth float32, brush graphics.Brush) {
	if p.canDraw() {
		p.base.DrawPath(path, strokeWidth, brush)
	}
}

func (p *painter) DrawTextLayout(origin geometry.Point, layout typography.TextLayout) {
	if p.canDraw() {
		p.base.DrawTextLayout(origin, layout)
	}
}

func (p *painter) DrawImage(rect geometry.Rectangle, img image.Image) {
	if p.canDraw() {
		p.base.DrawImage(rect, img)
	}
}

func (p *painter) canDraw() bool {
	return !emptyRect(p.state.clip)
}

func (p *painter) applyClip() {
	clip := p.state.clip
	// graphics.Painter uses the zero rectangle to disable native clipping. A
	// full-target GUI clip has the same native representation. An empty GUI
	// clip is also represented this way at the backend, but canDraw suppresses
	// every draw until a non-empty state is restored.
	if emptyRect(clip) || clip == p.bounds {
		clip = geometry.Rectangle{}
	}
	if p.clipApplied && p.appliedClip == clip {
		return
	}
	p.base.SetClipRect(clip)
	p.appliedClip = clip
	p.clipApplied = true
}

func emptyRect(rect geometry.Rectangle) bool {
	return rect.Width <= 0 || rect.Height <= 0
}

func SubPainter(base Painter, rect geometry.Rectangle) Painter {
	return subPainter{
		base: base,
		rect: rect,
	}
}

type subPainter struct {
	base Painter
	rect geometry.Rectangle
}

func (p subPainter) Begin() {
	p.base.Begin()
	// Establish this Widget's bounds without moving the inherited clip. The
	// base Painter captured that clip before the local origin is applied.
	p.base.SetClipRect(p.rect)
}

func (p subPainter) End() {
	p.base.End()
}

func (p subPainter) SetClipRect(rect geometry.Rectangle) {
	localBounds := geometry.Rect(0, 0, p.rect.Width, p.rect.Height)
	p.base.SetClipRect(p.translateRect(rect.Intersect(localBounds)))
}

func (p subPainter) Clear(color graphics.Color) {
	p.base.FillRect(p.rect, color)
}

func (p subPainter) FillRect(rect geometry.Rectangle, brush graphics.Brush) {
	p.base.FillRect(p.translateRect(rect), brush)
}

func (p subPainter) FillRoundRect(rect geometry.Rectangle, radius float32, brush graphics.Brush) {
	p.base.FillRoundRect(p.translateRect(rect), radius, brush)
}

func (p subPainter) FillEllipse(center geometry.Point, xRadius, yRadius float32, brush graphics.Brush) {
	p.base.FillEllipse(center.Add(p.rect.Pos), xRadius, yRadius, brush)
}

func (p subPainter) FillPath(path graphics.Path, brush graphics.Brush) {
	p.base.FillPath(p.translatePath(path), brush)
}

func (p subPainter) DrawLine(p0, p1 geometry.Point, strokeWidth float32, brush graphics.Brush) {
	p.base.DrawLine(p0.Add(p.rect.Pos), p1.Add(p.rect.Pos), strokeWidth, brush)
}

func (p subPainter) DrawRect(rect geometry.Rectangle, strokeWidth float32, brush graphics.Brush) {
	p.base.DrawRect(p.translateRect(rect), strokeWidth, brush)
}

func (p subPainter) DrawRoundRect(rect geometry.Rectangle, radius, strokeWidth float32, brush graphics.Brush) {
	p.base.DrawRoundRect(p.translateRect(rect), radius, strokeWidth, brush)
}

func (p subPainter) DrawEllipse(center geometry.Point, xRadius, yRadius, strokeWidth float32, brush graphics.Brush) {
	p.base.DrawEllipse(center.Add(p.rect.Pos), xRadius, yRadius, strokeWidth, brush)
}

func (p subPainter) DrawPath(path graphics.Path, strokeWidth float32, brush graphics.Brush) {
	p.base.DrawPath(p.translatePath(path), strokeWidth, brush)
}

func (p subPainter) DrawTextLayout(origin geometry.Point, layout typography.TextLayout) {
	p.base.DrawTextLayout(origin.Add(p.rect.Pos), layout)
}

func (p subPainter) DrawImage(rect geometry.Rectangle, img image.Image) {
	p.base.DrawImage(p.translateRect(rect), img)
}

func (p subPainter) translateRect(rect geometry.Rectangle) geometry.Rectangle {
	rect.Pos = rect.Pos.Add(p.rect.Pos)
	return rect
}

func (p subPainter) translatePath(path graphics.Path) (translated graphics.Path) {
	empty := true
	path.Range(func(op graphics.PathOperation, args []float32) (stop bool) {
		switch op {
		case graphics.PathMoveTo:
			translated = graphics.MoveTo(args[0]+p.rect.X, args[1]+p.rect.Y)
			empty = false
		case graphics.PathLineTo:
			if empty {
				translated = graphics.MoveTo(p.rect.X, p.rect.Y)
				empty = false
			}
			translated = translated.LineTo(args[0]+p.rect.X, args[1]+p.rect.Y)
		case graphics.PathArcTo:
			if empty {
				translated = graphics.MoveTo(p.rect.X, p.rect.Y)
				empty = false
			}
			translated = translated.ArcTo(args[0], args[1], args[2], args[3], args[4], args[5]+p.rect.X, args[6]+p.rect.Y)
		case graphics.PathBezierTo:
			if empty {
				translated = graphics.MoveTo(p.rect.X, p.rect.Y)
				empty = false
			}
			translated = translated.BezierTo(
				args[0]+p.rect.X, args[1]+p.rect.Y,
				args[2]+p.rect.X, args[3]+p.rect.Y,
				args[4]+p.rect.X, args[5]+p.rect.Y,
			)
		case graphics.PathClose:
			if !empty {
				translated = translated.Close()
			}
		}
		return false
	})
	return translated
}
