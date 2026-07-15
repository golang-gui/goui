package ui

import "github.com/golang-gui/goui/gui"

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

// viewBase is the non-generic core every View carries: the shared modifier state
// (id, visibility, style) plus the base() seal that forces embedding. The
// reconciler reads it via View.base after each Update and writes it onto the
// mounted widget, so controls never apply these themselves.
type viewBase struct {
	name                string
	hidden              bool
	styleName           string  // semantic style name (Sel.Name); "" reverts to the widget's type default
	minWidth, minHeight float32 // size preference; 0 = no min
	maxWidth, maxHeight float32 // size preference; 0 = unbounded
	mainWeight          float32 // main-axis extra-space share; 0 = hug
}

func (b *viewBase) base() *viewBase { return b }

// apply writes the shared modifier state onto a mounted widget. The
// reconciler calls it after every Update, so no control does id/visibility/style
// wiring itself.
func (b *viewBase) apply(widget gui.Widget) {
	widget.SetID(b.name)
	widget.SetVisible(!b.hidden)
	widget.SetStyleName(b.styleName) // "" reverts to the widget's type default
	widget.SetMinWidth(b.minWidth)
	widget.SetMaxWidth(b.maxWidth)
	widget.SetMinHeight(b.minHeight)
	widget.SetMaxHeight(b.maxHeight)
	widget.SetMainWeight(b.mainWeight)
}

// ViewBase is embedded as ViewBase[ConcreteView] by every declarative view. Its
// chain methods return the concrete *T (through Self) so shared modifiers
// compose with control-specific ones. The concrete constructor must set Self;
// forgetting it makes the guarded panic instead of returning a nil view.
type ViewBase[T any] struct {
	Self *T
	viewBase
}

func (b *ViewBase[T]) self() *T {
	if b.Self != nil {
		return b.Self
	}
	panic("ui: view not initialized via its constructor (ViewBase.Self is nil)")
}

func (b *ViewBase[T]) Name(name string) *T {
	b.name = name
	return b.self()
}

func (b *ViewBase[T]) Visible(visible bool) *T {
	b.hidden = !visible
	return b.self()
}

func (b *ViewBase[T]) Hidden(hidden bool) *T {
	b.hidden = hidden
	return b.self()
}

// Style selects a semantic style name (= SetStyleName); the theme's sheet turns
// it into concrete visuals. Local code names intent only, never sets values.
func (b *ViewBase[T]) Style(name string) *T {
	b.styleName = name
	return b.self()
}

// Size preference (min/max). There is no fixed Width/Height: these are
// preferences the parent constraint can override.
func (b *ViewBase[T]) MinWidth(v float32) *T  { b.minWidth = v; return b.self() }
func (b *ViewBase[T]) MaxWidth(v float32) *T  { b.maxWidth = v; return b.self() }
func (b *ViewBase[T]) MinHeight(v float32) *T { b.minHeight = v; return b.self() }
func (b *ViewBase[T]) MaxHeight(v float32) *T { b.maxHeight = v; return b.self() }

func (b *ViewBase[T]) MinSize(w, h float32) *T {
	b.minWidth, b.minHeight = w, h
	return b.self()
}

func (b *ViewBase[T]) MaxSize(w, h float32) *T {
	b.maxWidth, b.maxHeight = w, h
	return b.self()
}

// MainWeight sets this view's share of leftover main-axis space in a linear
// parent (0 = hug). Two siblings with weights 1 and 2 split the free space 1:2.
func (b *ViewBase[T]) MainWeight(w float32) *T {
	b.mainWeight = w
	return b.self()
}
