package gui

import (
	"image"

	"github.com/golang-gui/goui/core/geometry"
	"github.com/golang-gui/goui/platform/graphics"
	"github.com/golang-gui/goui/platform/typography"
)

type Painter interface {
	// Begin marks the start of a painting scope. State (clip, transform) is
	// saved and restored on End.
	Begin()
	// End marks the end of a painting scope, restoring clip and transform to
	// the state captured by Begin.
	End()
	NewImage(src image.Image) (graphics.Image, error)
	SetClipRect(rect geometry.Rectangle)
	SetTransform(matrix geometry.Transform)
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
	DrawImage(rect geometry.Rectangle, img graphics.Image)
}

func (p *painter) NewImage(src image.Image) (graphics.Image, error) {
	return p.base.NewImage(src)
}

// paintWidget is the only entry point used by the GUI traversal to invoke a
// Widget's Paint method. The caller owns the Painter scope, so Widget.Paint can
// change its local clip and transform without having to restore them before
// returning.
func paintWidget(widget Widget, p Painter) {
	p.Begin()
	defer p.End()
	widget.Paint(p)
}

// painter adapts the native-surface graphics.Painter to the local, scoped
// Painter consumed by Widgets. Its rectangles are in window-local logical
// coordinates. The transform stack manages coordinate-space offsets so that
// SubPainter instances can work in widget-local coordinates without manual
// translation in each draw method.
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
	// offset is the accumulated translation from widget-local to window-local
	// coordinates, contributed by nested SubPainter Begin calls.
	offset geometry.Point
	// userXform is the transform set by the widget via SetTransform, expressed
	// in widget-local coordinates.
	userXform geometry.Transform
}

func newPainter(base graphics.Painter, bounds geometry.Rectangle) Painter {
	return &painter{
		base:   base,
		bounds: bounds,
		state: painterState{
			scopeClip: bounds,
			clip:      bounds,
			userXform: geometry.Identity(),
		},
	}
}

// rootSaver is implemented by painter to allow SubPainter to save/restore
// state without recursing through nested SubPainter.Begin calls.
type rootSaver interface {
	saveState()
	restoreState()
}

// Begin saves the current state onto the stack. Called by paintWidget at the
// outermost scope.
func (p *painter) Begin() {
	p.saveState()
	p.applyClip()
	p.applyTransform()
}

func (p *painter) saveState() {
	p.stack = append(p.stack, p.state)
	p.state.scopeClip = p.state.clip
}

// End restores the state saved by Begin.
func (p *painter) End() {
	p.restoreState()
}

func (p *painter) restoreState() {
	if len(p.stack) == 0 {
		panic("gui: unbalanced Painter.End")
	}
	last := len(p.stack) - 1
	p.state = p.stack[last]
	p.stack = p.stack[:last]
	p.applyClip()
	p.applyTransform()
}

// pushScope accumulates the widget's position into the coordinate offset and
// sets the widget's clip to its bounds (in window-local coordinates),
// intersected with the current clip. The scope clip is NOT modified here —
// it was already captured by Begin. This ensures nested SubPainter clips
// compose correctly: each widget's clip is intersected with the parent's
// scope clip (saved by Begin), not with a modified scope clip.
func (p *painter) pushScope(rectPos geometry.Point, localClip geometry.Rectangle) {
	p.state.offset = p.state.offset.Add(rectPos)
	windowClip := localClip.Translate(p.state.offset)
	p.state.clip = p.state.scopeClip.Intersect(windowClip)
	p.applyClip()
	p.applyTransform()
}

func (p *painter) SetClipRect(rect geometry.Rectangle) {
	// rect is in widget-local coordinates; translate to window-local.
	p.state.clip = p.state.scopeClip.Intersect(rect.Translate(p.state.offset))
	p.applyClip()
}

func (p *painter) SetTransform(matrix geometry.Transform) {
	p.state.userXform = matrix
	p.applyTransform()
}

// fullTransform returns the effective transform: widget-local coords → user
// transform → offset translation → window-local coords.
func (p *painter) fullTransform() geometry.Transform {
	return geometry.Translate(p.state.offset.X, p.state.offset.Y).Multiply(p.state.userXform)
}

func (p *painter) applyTransform() {
	p.base.SetTransform(p.fullTransform())
}

// --- Draw methods: coordinates are in widget-local space; the transform
// handles translation to window-local coordinates. ---

func (p *painter) Clear(color graphics.Color) {
	if p.canDraw() {
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

func (p *painter) DrawImage(rect geometry.Rectangle, img graphics.Image) {
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

// offsetPusher is implemented by painter to allow SubPainter to accumulate
// coordinate offsets and set the widget scope clip in one atomic step.
type offsetPusher interface {
	pushScope(offset geometry.Point, localClip geometry.Rectangle)
}

// SubPainter returns a Painter scoped to the given rectangle within the parent
// Painter's coordinate space. Widget-local coordinates (origin at rect.Pos)
// are automatically translated to the parent's coordinate space via the
// transform stack — draw methods receive widget-local coordinates directly.
func SubPainter(base Painter, rect geometry.Rectangle) Painter {
	return &subPainter{
		base: base,
		rect: rect,
	}
}

type subPainter struct {
	base Painter
	rect geometry.Rectangle
}

func (p *subPainter) Begin() {
	// Save state on the root painter without recursing through nested
	// SubPainter.Begin calls (which would re-push offsets).
	if saver, ok := findRootSaver(p.base); ok {
		saver.saveState()
	} else {
		p.base.Begin()
	}
	// Push this widget's offset and establish its clip.
	if op, ok := p.base.(offsetPusher); ok {
		op.pushScope(p.rect.Pos, geometry.Rect(0, 0, p.rect.Width, p.rect.Height))
	} else {
		p.base.SetClipRect(geometry.Rect(0, 0, p.rect.Width, p.rect.Height))
	}
}

func (p *subPainter) End() {
	if saver, ok := findRootSaver(p.base); ok {
		saver.restoreState()
	} else {
		p.base.End()
	}
}

// findRootSaver walks the SubPainter chain to find the underlying painter.
func findRootSaver(p Painter) (rootSaver, bool) {
	for {
		if s, ok := p.(rootSaver); ok {
			return s, true
		}
		sp, ok := p.(*subPainter)
		if !ok {
			return nil, false
		}
		p = sp.base
	}
}

func (p *subPainter) SetClipRect(rect geometry.Rectangle) {
	p.base.SetClipRect(rect)
}

func (p *subPainter) SetTransform(matrix geometry.Transform) {
	p.base.SetTransform(matrix)
}

// pushScope delegates to the underlying painter to accumulate the coordinate
// offset and set the scope clip. This ensures nested SubPainter instances
// compose their offsets correctly through the shared painter state.
func (p *subPainter) pushScope(offset geometry.Point, localClip geometry.Rectangle) {
	if op, ok := p.base.(offsetPusher); ok {
		op.pushScope(offset, localClip)
	}
}

// Delegate all draw methods to base. Coordinates are in widget-local space;
// the transform (managed by painter) handles translation to window coordinates.

func (p *subPainter) Clear(color graphics.Color) {
	p.base.Clear(color)
}

func (p *subPainter) FillRect(rect geometry.Rectangle, brush graphics.Brush) {
	p.base.FillRect(rect, brush)
}

func (p *subPainter) FillRoundRect(rect geometry.Rectangle, radius float32, brush graphics.Brush) {
	p.base.FillRoundRect(rect, radius, brush)
}

func (p *subPainter) FillEllipse(center geometry.Point, xRadius, yRadius float32, brush graphics.Brush) {
	p.base.FillEllipse(center, xRadius, yRadius, brush)
}

func (p *subPainter) FillPath(path graphics.Path, brush graphics.Brush) {
	p.base.FillPath(path, brush)
}

func (p *subPainter) DrawLine(p0, p1 geometry.Point, strokeWidth float32, brush graphics.Brush) {
	p.base.DrawLine(p0, p1, strokeWidth, brush)
}

func (p *subPainter) DrawRect(rect geometry.Rectangle, strokeWidth float32, brush graphics.Brush) {
	p.base.DrawRect(rect, strokeWidth, brush)
}

func (p *subPainter) DrawRoundRect(rect geometry.Rectangle, radius, strokeWidth float32, brush graphics.Brush) {
	p.base.DrawRoundRect(rect, radius, strokeWidth, brush)
}

func (p *subPainter) DrawEllipse(center geometry.Point, xRadius, yRadius, strokeWidth float32, brush graphics.Brush) {
	p.base.DrawEllipse(center, xRadius, yRadius, strokeWidth, brush)
}

func (p *subPainter) DrawPath(path graphics.Path, strokeWidth float32, brush graphics.Brush) {
	p.base.DrawPath(path, strokeWidth, brush)
}

func (p *subPainter) DrawTextLayout(origin geometry.Point, layout typography.TextLayout) {
	p.base.DrawTextLayout(origin, layout)
}

func (p *subPainter) NewImage(src image.Image) (graphics.Image, error) {
	return p.base.NewImage(src)
}

func (p *subPainter) DrawImage(rect geometry.Rectangle, img graphics.Image) {
	p.base.DrawImage(rect, img)
}
