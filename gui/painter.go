package gui

import (
	"fmt"
	"image"

	"github.com/golang-gui/goui/core/geometry"
	"github.com/golang-gui/goui/platform/graphics"
	"github.com/golang-gui/goui/platform/typography"
)

// Painter is the widget-local drawing context supplied to Widget.Paint.
//
// A Painter is valid only for the synchronous duration of Paint. Widgets draw
// themselves in local logical coordinates; the GUI traversal owns child
// painting and restores all state before it enters a child or sibling.
type Painter interface {
	// Save pushes the current clip and transform. Every Save must be paired
	// with Restore before the current Widget.Paint call returns.
	Save()
	// Restore restores the state most recently pushed by Save. Restore panics
	// if it would cross the current Widget.Paint boundary.
	Restore()
	// NewImage snapshots src into a resource owned by the current platform
	// Painter. The returned Image follows graphics.Image's lifecycle contract.
	NewImage(src image.Image) (graphics.Image, error)
	// SetClipRect replaces the current explicit clip within the Widget's
	// structural bounds. rect is in Widget-local layout coordinates.
	SetClipRect(rect geometry.Rectangle)
	// SetTransform replaces the current Widget-local drawing transform. It
	// affects only the current Widget, not its descendants.
	SetTransform(matrix geometry.Transform)
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

// painter adapts a surface graphics.Painter to the local drawing context used
// by Widgets. There is one painter per frame traversal: widget scopes are
// represented by state, not by chains of delegating Painter objects.
type painter struct {
	base   graphics.Painter
	bounds geometry.Rectangle
	state  painterState
	saves  []painterState
	frames []painterFrame

	appliedClip  geometry.Rectangle
	clipApplied  bool
	appliedXform geometry.Transform
	xformApplied bool
}

type painterState struct {
	// scopeClip is the structural clip established by the Widget tree. A
	// Widget may replace its explicit clip within this boundary but cannot
	// widen it.
	scopeClip geometry.Rectangle
	clip      geometry.Rectangle
	// offset maps the current Widget's local origin to window coordinates.
	offset geometry.Point
	// userXform is the absolute transform set by the current Widget, expressed
	// in its local coordinates.
	userXform geometry.Transform
}

type painterFrame struct {
	widget    Widget
	saveDepth int
}

func newPainter(base graphics.Painter, bounds geometry.Rectangle) *painter {
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

// paintWidget is the single GUI entry point for painting a Widget subtree.
// Each visible Widget paints itself first; its visible children are then
// painted in tree order with independent local drawing state.
func paintWidget(widget Widget, p *painter) {
	if widget == nil || !widget.Visible() {
		return
	}
	p.paintWidget(widget)
}

func (p *painter) paintWidget(widget Widget) {
	parentState := p.state
	saveDepth := len(p.saves)
	p.frames = append(p.frames, painterFrame{widget: widget, saveDepth: saveDepth})

	rect := widget.Rect()
	offset := parentState.offset.Add(rect.Pos)
	localBounds := geometry.Rect(0, 0, rect.Width, rect.Height)
	scopeClip := parentState.scopeClip.Intersect(localBounds.Translate(offset))
	p.state = painterState{
		scopeClip: scopeClip,
		clip:      scopeClip,
		offset:    offset,
		userXform: geometry.Identity(),
	}
	p.applyState()

	defer func() {
		// Every exit path, including a panic from Widget.Paint, restores the
		// parent before control reaches a sibling or the host.
		p.saves = p.saves[:saveDepth]
		p.state = parentState
		p.frames = p.frames[:len(p.frames)-1]
		p.applyState()
	}()

	p.paintSelf(widget, saveDepth)

	// WidgetBase owns the canonical child slice. Reading it directly avoids a
	// defensive Children() copy for every Widget on every frame. Paint is
	// required not to mutate the tree.
	for _, child := range widget.base().children {
		paintWidget(child, p)
	}
}

func (p *painter) paintSelf(widget Widget, saveDepth int) {
	entryState := p.state
	defer func() {
		recovered := recover()
		unbalanced := len(p.saves) - saveDepth
		p.saves = p.saves[:saveDepth]
		p.state = entryState
		p.applyState()

		if recovered != nil {
			panic(recovered)
		}
		if unbalanced != 0 {
			panic(fmt.Sprintf(
				"gui: %s returned with %d unbalanced Painter.Save call(s)",
				widgetPaintName(widget), unbalanced,
			))
		}
	}()

	widget.Paint(p)
}

func widgetPaintName(widget Widget) string {
	if id := widget.ID(); id != "" {
		return fmt.Sprintf("%T.Paint (id %q)", widget, id)
	}
	return fmt.Sprintf("%T.Paint", widget)
}

func (p *painter) Save() {
	if len(p.frames) == 0 {
		panic("gui: Painter.Save called outside Widget.Paint")
	}
	p.saves = append(p.saves, p.state)
}

func (p *painter) Restore() {
	if len(p.frames) == 0 {
		panic("gui: Painter.Restore called outside Widget.Paint")
	}
	frame := p.frames[len(p.frames)-1]
	if len(p.saves) <= frame.saveDepth {
		panic(fmt.Sprintf(
			"gui: Painter.Restore without matching Save in %s",
			widgetPaintName(frame.widget),
		))
	}
	last := len(p.saves) - 1
	p.state = p.saves[last]
	p.saves = p.saves[:last]
	p.applyState()
}

func (p *painter) NewImage(src image.Image) (graphics.Image, error) {
	return p.base.NewImage(src)
}

func (p *painter) SetClipRect(rect geometry.Rectangle) {
	// Clip rectangles are layout-space rectangles: translate the Widget-local
	// value to window coordinates, then restrict it to the immutable tree clip.
	p.state.clip = p.state.scopeClip.Intersect(rect.Translate(p.state.offset))
	p.applyClip()
}

func (p *painter) SetTransform(matrix geometry.Transform) {
	p.state.userXform = matrix
	p.applyTransform()
}

// fullTransform maps Widget-local coordinates through the Widget's own
// transform and then into window coordinates.
func (p *painter) fullTransform() geometry.Transform {
	return geometry.Translate(p.state.offset.X, p.state.offset.Y).Multiply(p.state.userXform)
}

func (p *painter) applyState() {
	p.applyClip()
	p.applyTransform()
}

func (p *painter) applyTransform() {
	transform := p.fullTransform()
	if p.xformApplied && p.appliedXform == transform {
		return
	}
	p.base.SetTransform(transform)
	p.appliedXform = transform
	p.xformApplied = true
}

// Draw methods pass Widget-local coordinates through unchanged. The effective
// transform applies their window offset in the platform Painter.

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
