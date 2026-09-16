package gui

import (
	"math"
	"slices"

	"github.com/golang-gui/goui/core/geometry"
	"github.com/golang-gui/goui/core/signal"
	"github.com/golang-gui/goui/layout"
)

// Overlay lays out one main child and any number of floating children. Only
// the main child contributes to measurement and the baseline. Floating children
// paint above it in insertion order and, by default, keep their measured size.
//
// All children belong to the ordinary Widget tree. Hit testing, clipping,
// focus and event delivery are unchanged: a floating child's entire rectangle
// participates in picking, including its padding and otherwise empty space.
// Overlay does not provide keyboard modality or automatically manage focus.
type Overlay struct {
	WidgetBase
	child    Widget
	overlays map[Widget]*overlayChild
}

func NewOverlay() *Overlay { return new(Overlay) }

// Child returns the main child, or nil if it has been removed or reparented.
func (o *Overlay) Child() Widget {
	if o.child != nil && o.child.Parent() == o && !o.child.base().destroyed {
		return o.child
	}
	return nil
}

// SetChild replaces the main child; nil clears it. The main child always paints
// below floating children, even when installed after them. An existing floating
// child may become the main child; its overlay settings and queries are removed.
func (o *Overlay) SetChild(child Widget) {
	if o.destroyed || (child != nil && !o.canAdopt(child)) || o.Child() == child {
		return
	}
	o.RemoveChild(o.Child())
	o.child = child
	if child != nil {
		delete(o.overlays, child)
		child.base().setParent(child, o)
		// Keep the canonical tree order, not just a separate layout order.
		if index := slices.Index(o.children, child); index > 0 {
			copy(o.children[1:index+1], o.children[:index])
			o.children[0] = child
		}
	}
	o.RequestLayout()
}

// AddOverlay appends a floating child above the current children. Adding the
// main child or an already attached floating child is a no-op. Newly attached
// children start with Fill=false and position (0, 0).
func (o *Overlay) AddOverlay(child Widget) {
	if !o.canAdopt(child) || child == o.Child() || o.overlay(child) != nil {
		return
	}
	if o.overlays == nil {
		o.overlays = make(map[Widget]*overlayChild)
	}
	o.overlays[child] = new(overlayChild)
	// Like ScrollView's structural children, these are outside the public Bin
	// slot. Use the existing mounting primitive without the AddChild Bin guard.
	child.base().setParent(child, o)
	if child.Parent() != o {
		delete(o.overlays, child)
	}
}

// RemoveOverlay detaches a floating child without destroying it. Its fill
// setting and position queries no longer apply; re-adding starts afresh.
// Removing the main child through this method is a no-op; use SetChild(nil).
func (o *Overlay) RemoveOverlay(child Widget) {
	if child != nil && child != o.Child() {
		o.RemoveChild(child)
	}
}

// RemoveChild detaches either kind of child and clears its layout metadata.
func (o *Overlay) RemoveChild(child Widget) {
	if child == nil {
		return
	}
	if o.child == child {
		o.child = nil
	}
	delete(o.overlays, child)
	o.WidgetBase.RemoveChild(child)
}

// OverlayFill reports whether the floating child is forced to fill both axes.
// It returns false for children not currently attached as floating children.
func (o *Overlay) OverlayFill(child Widget) bool {
	entry := o.overlay(child)
	return entry != nil && entry.fill
}

// SetOverlayFill changes a floating child's measurement from Loose to Tight.
// The parent's tight allocation wins over the child's MinSize/MaxSize, as in
// other layouts. This only controls size; position queries still apply.
// Calls for the main child or an unattached child are ignored.
func (o *Overlay) SetOverlayFill(child Widget, fill bool) {
	if entry := o.overlay(child); entry != nil && entry.fill != fill {
		entry.fill = fill
		o.RequestLayout()
	}
}

// ConnectOverlayPosition queries a floating child's position after measurement
// during Arrange. available is the Overlay's allocated size; size is the
// child's measured size. All values are in Overlay-local DIP. Position starts
// at (0, 0); callbacks run in connection order and may override previous results.
// Negative and out-of-bounds positions are allowed and use normal tree clipping;
// non-finite coordinates become zero.
//
// AddOverlay must precede this call. Invalid targets and nil callbacks return
// inert handles. Hiding a child preserves its queries; removing it does not.
// Queries are synchronous: do not retain position, mutate the tree or recursively
// lay out widgets. After changing captured state or blocking/disconnecting a
// handle, call RequestLayout to apply the change. Connecting requests layout.
func (o *Overlay) ConnectOverlayPosition(child Widget,
	fn func(available, size geometry.Size, position *geometry.Point),
) signal.Handle {
	entry := o.overlay(child)
	if entry == nil || fn == nil {
		return signal.Handles(nil)
	}
	handle := entry.position.Connect(func(available, size geometry.Size, position *geometry.Point) {
		// A preceding callback may have destroyed the window or detached the
		// child. Do not invoke subsequent user code on stale layout state.
		if o.overlay(child) == entry && o.Visible() && child.Visible() {
			fn(available, size, position)
		}
	})
	o.RequestLayout()
	return handle
}

func (o *Overlay) Measure(c layout.Constraint) layout.Measurement {
	o.pruneChildren()
	if o.destroyed || !o.Visible() {
		return layout.Measurement{}
	}
	c = o.layoutConstraint(c)
	var measured layout.Measurement
	if child := o.Child(); child != nil && child.Visible() {
		measured = measureWidget(child, c)
	}
	return measured.Constrain(c)
}

func (o *Overlay) Arrange(rect geometry.Rectangle) {
	o.rect = rect
	o.pruneChildren()
	if o.destroyed || !o.Visible() {
		return
	}
	if child := o.Child(); child != nil && child.Visible() {
		measureWidget(child, layout.Tight(rect.Size))
		if !o.destroyed && o.Child() == child && child.Visible() {
			child.Arrange(geometry.Rectangle{Size: rect.Size})
		}
	}
	for _, child := range o.Children() {
		entry := o.overlay(child)
		if entry == nil || !child.Visible() {
			continue
		}
		constraint := layout.Loose(rect.Size)
		if entry.fill {
			constraint = layout.Tight(rect.Size)
		}
		measured := measureWidget(child, constraint)
		if o.overlay(child) != entry || !child.Visible() {
			continue
		}
		position := geometry.Point{}
		entry.position.Emit(rect.Size, measured.Size, &position)
		if o.overlay(child) != entry || !o.Visible() || !child.Visible() {
			continue
		}
		child.Arrange(geometry.Rectangle{
			Pos:  geometry.Point{X: finiteOverlayPosition(position.X), Y: finiteOverlayPosition(position.Y)},
			Size: measured.Size,
		})
	}
}

func (o *Overlay) canAdopt(child Widget) bool {
	return !o.destroyed && child != nil && !child.base().destroyed &&
		!o.isDescendant(o, child)
}

func (o *Overlay) overlay(child Widget) *overlayChild {
	if o.destroyed || child == nil || child.base().destroyed || child.Parent() != o || child == o.child {
		return nil
	}
	return o.overlays[child]
}

// Reparenting through another container bypasses our setters. Detachment already
// requests layout; discard the old parent's metadata on that layout pass.
func (o *Overlay) pruneChildren() {
	o.child = o.Child()
	for child := range o.overlays {
		if o.overlay(child) == nil {
			delete(o.overlays, child)
		}
	}
}

// overlayChild holds layout metadata, not a second child tree. WidgetBase's
// children remain the authoritative paint, input and lifetime order.
type overlayChild struct {
	fill     bool
	position signal.Signal3[geometry.Size, geometry.Size, *geometry.Point]
}

func finiteOverlayPosition(value float32) float32 {
	if math.IsNaN(float64(value)) || math.IsInf(float64(value), 0) {
		return 0
	}
	return value
}
