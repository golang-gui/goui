package ui

import (
	"github.com/golang-gui/goui/core/bits"
	"github.com/golang-gui/goui/core/geometry"
	"github.com/golang-gui/goui/core/signal"
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
	UpdateChildren(widget gui.Widget, children []View)
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
	focusable  bool
	cursor     Cursor
	onFocus    func(focused bool) // fired when the mounted widget's focus state changes

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
	viewFocusable
	viewCursor
	viewOnFocus
)

// viewBaseContext is the persistent lifecycle context of viewBase: root creates
// one per node on first mount and reuses it across rebuilds. It stores the
// cross-cutting widget signal handles that must survive rebuilds, the
// currently effective callbacks, and a snapshot of the widget's initial
// shared-modifier values taken right after Mount. On every Update the
// declarative view either overwrites a value (bit set) or restores the
// snapshot (bit not set), so the widget's private defaults stay private and
// a missing modifier naturally reverts. The context is private.
type viewBaseContext struct {
	handles []signal.Handle    // cross-rebuild handles for shared widget signals
	onFocus func(focused bool) // effective OnFocus callback, refreshed on every update

	// Snapshot of the widget's initial values, captured once in mount before
	// the first apply. Each field corresponds to a viewBase modifier; hidden
	// is stored as the widget's Visible complement.
	initName       string
	initStyleName  string
	initMinSize    geometry.Size
	initMaxSize    geometry.Size
	initMainWeight float32
	initHidden     bool
	initFocusable  bool
	initCursor     Cursor
}

func (b *viewBase) base() *viewBase { return b }

// mount snapshots the widget's initial shared-modifier values and registers
// the cross-cutting widget signals that survive rebuilds. The snapshot is
// taken before the first apply so a later Update with a missing modifier can
// restore the widget's private default. The focus closure reads ctx.onFocus
// at fire time, and update overlays the new view's callback into ctx, so a
// rebuild that swaps the view automatically picks up the latest callback
// without reconnecting.
func (b *viewBase) mount(ctx *viewBaseContext, w gui.Widget) {
	ctx.initName = w.ID()
	ctx.initStyleName = w.StyleName()
	ctx.initMinSize = w.MinSize()
	ctx.initMaxSize = w.MaxSize()
	ctx.initMainWeight = w.MainWeight()
	ctx.initHidden = !w.Visible()
	ctx.initFocusable = w.Focusable()
	ctx.initCursor = w.Cursor()
	ctx.onFocus = b.onFocus
	ctx.handles = append(ctx.handles, w.ConnectFocused(func(focused bool) {
		if ctx.onFocus != nil {
			ctx.onFocus(focused)
		}
	}))
}

// update refreshes the shared callbacks after each rebuild and applies the
// shared modifiers onto the widget (missing modifiers restore the snapshot).
func (b *viewBase) update(ctx *viewBaseContext, w gui.Widget) {
	ctx.onFocus = b.onFocus
	b.apply(ctx, w)
}

// unmount disconnects all registered shared signal handles and clears the
// context, including the snapshot.
func (b *viewBase) unmount(ctx *viewBaseContext, _ gui.Widget) {
	for _, h := range ctx.handles {
		h.Disconnect()
	}
	ctx.handles = nil
	ctx.onFocus = nil
	ctx.initName = ""
	ctx.initStyleName = ""
	ctx.initMinSize = geometry.Size{}
	ctx.initMaxSize = geometry.Size{}
	ctx.initMainWeight = 0
	ctx.initHidden = false
	ctx.initFocusable = false
	ctx.initCursor = nil
}

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

// Focusable sets whether the mounted widget accepts keyboard focus. Like Cursor,
// it only applies when explicitly called — gui widgets that enable focus in their
// constructor (Button, MenuButton, TextInput) keep that by default. Pass
// Focusable(false) to strip it (e.g. a menu-bar-style row of MenuButtons where
// only the open one should take focus).
func (b *ViewBase[T]) Focusable(focusable bool) *T {
	b.focusable = focusable
	b.fields.Set(viewFocusable, true)
	return b.self()
}

// OnFocus registers a callback fired when the mounted widget's keyboard focus
// state changes: true when it gains focus, false when it loses focus.
func (b *ViewBase[T]) OnFocus(fn func(focused bool)) *T {
	b.onFocus = fn
	b.fields.Set(viewOnFocus, true)
	return b.self()
}

// apply writes the shared modifier state onto a mounted widget. The
// reconciler calls it after every Update, so no control does id/visibility/style
// wiring itself. When a modifier was not set in this frame (bit not set) the
// widget is restored to the snapshot captured at mount, keeping the widget's
// private defaults private.
func (b *viewBase) apply(ctx *viewBaseContext, widget gui.Widget) {
	if b.fields.Check(viewName) {
		widget.SetID(b.name)
	} else {
		widget.SetID(ctx.initName)
	}
	if b.fields.Check(viewStyleName) {
		widget.SetStyleName(b.styleName)
	} else {
		widget.SetStyleName(ctx.initStyleName)
	}
	// MinSize: merge per-axis bits into snapshot, then set once.
	min := ctx.initMinSize
	if b.fields.Check(viewMinWidth) {
		min.Width = b.minWidth
	}
	if b.fields.Check(viewMinHeight) {
		min.Height = b.minHeight
	}
	widget.SetMinSize(min)
	// MaxSize: merge per-axis bits into snapshot, then set once.
	max := ctx.initMaxSize
	if b.fields.Check(viewMaxWidth) {
		max.Width = b.maxWidth
	}
	if b.fields.Check(viewMaxHeight) {
		max.Height = b.maxHeight
	}
	widget.SetMaxSize(max)
	if b.fields.Check(viewMainWeight) {
		widget.SetMainWeight(b.mainWeight)
	} else {
		widget.SetMainWeight(ctx.initMainWeight)
	}
	if b.fields.Check(viewHidden) {
		widget.SetVisible(!b.hidden)
	} else {
		widget.SetVisible(!ctx.initHidden)
	}
	if b.fields.Check(viewFocusable) {
		widget.SetFocusable(b.focusable)
	} else {
		widget.SetFocusable(ctx.initFocusable)
	}
	if b.fields.Check(viewCursor) {
		widget.SetCursor(b.cursor)
	} else {
		widget.SetCursor(ctx.initCursor)
	}
}
