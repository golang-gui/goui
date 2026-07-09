package ui

import (
	"github.com/golang-gui/goui/gui"
	"github.com/golang-gui/goui/style"
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

// viewBase is the non-generic core every View carries: the shared modifier state
// (id, visibility, style) plus the base() seal that forces embedding. The
// reconciler reads it via View.base after each Update and writes it onto the
// mounted widget, so controls never apply these themselves.
type viewBase struct {
	name   string
	hidden bool
	rules  []style.Rule
}

func (b *viewBase) base() *viewBase { return b }

// apply writes the shared modifier state onto a mounted widget. The
// reconciler calls it after every Update, so no control does id/visibility/style
// wiring itself.
func (b *viewBase) apply(widget gui.Widget) {
	widget.SetID(b.name)
	widget.SetVisible(!b.hidden)
	widget.SetStyleRules(b.rules...)
}

// ViewBase is embedded as ViewBase[ConcreteView] by every declarative view. Its
// chain methods return the concrete *T (through Self) so shared modifiers
// compose with control-specific ones. The concrete constructor must set Self;
// forgetting it makes the guarded self() panic instead of returning a nil view.
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

func (b *ViewBase[T]) Style(rules ...style.Rule) *T {
	b.rules = rules
	return b.self()
}
