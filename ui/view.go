package ui

import (
	"github.com/golang-gui/goui/core/bits"
	"github.com/golang-gui/goui/gui"
)

type View interface {
	base() *viewBase // seal: only an embedded ViewBase/viewBase provides it

	Build() View
}

type WidgetView interface {
	View
	Mount(ctx BuildContext) gui.Widget
	Update(ctx BuildContext, widget gui.Widget)
	Unmount(ctx BuildContext, widget gui.Widget)
}

type BuildContext interface {
	State() any
	SetState(any)
	UpdateChildren(container gui.Container, children []View)
}

// ViewBase is embedded as ViewBase[ConcreteView] by every declarative view. Its
// chain methods return the concrete *T (through Self) so shared modifiers
// compose with control-specific ones. The concrete constructor must set Self;
// forgetting it makes the guarded panic instead of returning a nil view.
type ViewBase[T any] struct {
	Self *T
	viewBase
}

// viewBase is the non-generic core every View carries: the shared modifier state
// (id, visibility, style) plus the base() seal that forces embedding. The
// reconciler reads it via View.base after each Update and writes it onto the
// mounted widget, so controls never apply these themselves.
type viewBase struct {
	name       string
	styleName  string // semantic style name (Sel.Name); "" reverts to the widget's type default
	minWidth   float32
	minHeight  float32 // size preference; 0 = no min
	maxWidth   float32
	maxHeight  float32 // size preference; 0 = unbounded
	mainWeight float32 // main-axis extra-space share; 0 = hug
	hidden     bool
	cursor     Cursor

	fields bits.Bitmap[uint64]
}

const (
	viewName = iota
	viewStyleName
	viewMinWidth
	viewMinHeight
	viewMaxWidth
	viewMaxHeight
	viewMainWeight
	viewHidden
	viewCursor
)

func (b *viewBase) base() *viewBase { return b }

func (b *ViewBase[T]) self() *T {
	if b.Self != nil {
		return b.Self
	}
	panic("ui: view not initialized via its constructor (ViewBase.Self is nil)")
}

func (b *ViewBase[T]) Name(name string) *T {
	b.name = name
	b.fields.Set(viewName, true)
	return b.self()
}

func (b *ViewBase[T]) Visible(visible bool) *T {
	b.hidden = !visible
	b.fields.Set(viewHidden, true)
	return b.self()
}

func (b *ViewBase[T]) Hidden(hidden bool) *T {
	b.hidden = hidden
	b.fields.Set(viewHidden, true)
	return b.self()
}

// Style selects a semantic style name (= SetStyleName); the theme's sheet turns
// it into concrete visuals.
func (b *ViewBase[T]) Style(name string) *T {
	b.styleName = name
	b.fields.Set(viewStyleName, true)
	return b.self()
}

func (b *ViewBase[T]) MinWidth(v float32) *T {
	b.minWidth = v
	b.fields.Set(viewMinWidth, true)
	return b.self()
}

func (b *ViewBase[T]) MaxWidth(v float32) *T {
	b.maxWidth = v
	b.fields.Set(viewMaxWidth, true)
	return b.self()
}

func (b *ViewBase[T]) MinHeight(v float32) *T {
	b.minHeight = v
	b.fields.Set(viewMinHeight, true)
	return b.self()
}

func (b *ViewBase[T]) MaxHeight(v float32) *T {
	b.maxHeight = v
	b.fields.Set(viewMaxHeight, true)
	return b.self()
}

func (b *ViewBase[T]) MinSize(w, h float32) *T {
	b.minWidth, b.minHeight = w, h
	b.fields.Set(viewMinWidth, true)
	b.fields.Set(viewMinHeight, true)
	return b.self()
}

func (b *ViewBase[T]) MaxSize(w, h float32) *T {
	b.maxWidth, b.maxHeight = w, h
	b.fields.Set(viewMaxWidth, true)
	b.fields.Set(viewMaxHeight, true)
	return b.self()
}

// MainWeight sets this view's share of leftover main-axis space in a linear
// parent (0 = hug). Two siblings with weights 1 and 2 split the free space 1:2.
func (b *ViewBase[T]) MainWeight(w float32) *T {
	b.mainWeight = w
	b.fields.Set(viewMainWeight, true)
	return b.self()
}

// Cursor sets the mouse cursor shown when hovering over this view. Applies at
// next Update; gui widgets that set a cursor in their constructor (Button,
// TextInput) keep that cursor if this modifier is never called. Pass
// CursorDefault to explicitly revert to the arrow, or CursorNone to hide.
func (b *ViewBase[T]) Cursor(c Cursor) *T {
	b.cursor = c
	b.fields.Set(viewCursor, true)
	return b.self()
}

// apply writes the shared modifier state onto a mounted widget. The
// reconciler calls it after every Update, so no control does id/visibility/style
// wiring itself.
func (b *viewBase) apply(widget gui.Widget) {
	if b.fields.Check(viewName) {
		widget.SetID(b.name)
	}
	if b.fields.Check(viewStyleName) {
		widget.SetStyleName(b.styleName)
	}
	widget.SetStyleName(b.styleName)
	if b.fields.Check(viewMinWidth) {
		widget.SetMinWidth(b.minWidth)
	}
	if b.fields.Check(viewMinHeight) {
		widget.SetMinHeight(b.minHeight)
	}
	if b.fields.Check(viewMaxWidth) {
		widget.SetMaxWidth(b.maxWidth)
	}
	if b.fields.Check(viewMaxHeight) {
		widget.SetMaxHeight(b.maxHeight)
	}
	if b.fields.Check(viewMainWeight) {
		widget.SetMainWeight(b.mainWeight)
	}
	if b.fields.Check(viewHidden) {
		widget.SetVisible(!b.hidden)
	}
	if b.fields.Check(viewCursor) {
		widget.SetCursor(b.cursor)
	}
}
