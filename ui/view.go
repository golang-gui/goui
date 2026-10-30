package ui

import (
	"slices"

	"github.com/golang-gui/goui/core/bits"
	"github.com/golang-gui/goui/core/geometry"
	"github.com/golang-gui/goui/core/signal"
	"github.com/golang-gui/goui/gui"
)

type View interface {
	base() *viewBase // seal: only an embedded ViewBase/viewBase provides it
}

// ComposeView owns a persistent declarative node and builds its single content
// view. A nil result keeps this node and its State, with no widget output.
// Synchronous State.Get and Read calls in Build track dependencies of this
// mounted node. Event callbacks and widget lifecycle calls are not tracked.
type ComposeView interface {
	View
	Build(ctx BuildContext) View
}

// ObjectView is a composition that exposes one object to descendants. Object
// reads existing state without creating resources. Release runs once, after
// descendants have released their connections; ownership remains the view's.
// It also cleans up failed first builds and must tolerate partial State.
type ObjectView interface {
	ComposeView
	Object(ctx BuildContext) any
	Release(ctx BuildContext)
}

type WidgetView interface {
	View
	Mount(ctx BuildContext) gui.Widget
	Update(ctx BuildContext, widget gui.Widget)
	Unmount(ctx BuildContext, widget gui.Widget)
}

// ReparentedView refreshes ancestry-dependent connections after an explicit
// subtree transfer. The complete logical ancestry is installed before calling
// it, before the transfer's committed callback. Do not rebuild or replay other
// declarative properties here. Ordinary updates already call Build or Update.
type ReparentedView interface {
	View
	Reparented(ctx BuildContext)
}

// BuildContext borrows its View node on the GUI thread. A virtualized adapter
// may retain it for delayed row builds: child updates resolve the current Root
// and become no-ops after release. It does not extend the node's lifetime.
type BuildContext interface {
	State() any
	SetState(any)
	// RequestUpdate asynchronously updates this node. It expires on release and
	// follows explicit subtree transfers. It may be called from any goroutine.
	RequestUpdate()
	// Coordinator borrows this node's identity for adapter coordination.
	// It expires on View release and follows an explicitly transferred subtree.
	Coordinator() *Coordinator
	// AfterUpdate runs after the entire View tree has been reconciled and
	// attached to its Window, before the next layout. App rebuilds complete all
	// window trees before these callbacks and show new windows afterwards, so
	// cross-window transfers cannot be overwritten by stale declarations.
	// It is a one-shot hook.
	// For delayed builds outside a View update, callbacks are posted to the GUI
	// loop after synchronous layout/mounting finishes. Released or superseded
	// owners are skipped. Do not start nested reconciliation from this callback.
	AfterUpdate(func())
	// UpdateChild reconciles one target's content. nil clears the managed child.
	// An optional apply callback configures the mounted output, including nil,
	// after mounting and after a local composition changes that output. It is
	// replaced on the owner's next update; it must not start reconciliation.
	UpdateChild(target Bin, child View, apply ...func(gui.Widget)) gui.Widget
	// UpdateChildren reconciles one target's list in declaration order. Results
	// correspond to input indices; nil views/builds/mounts yield nil entries.
	// Compositions occupy declaration positions even with no widget output.
	// The optional apply callback has the same lifetime as UpdateChild's, and
	// receives each declaration's index and current output, including nil.
	// An empty list clears managed children. Omitting a call does not clear a
	// previously used target.
	UpdateChildren(target Container, children []View, apply ...func(int, gui.Widget)) []gui.Widget
}

// Bin is a single-child mounting target, not necessarily a Widget. Unlike
// gui.Bin it need not provide layout, events or its own widget tree. Targets
// must be stable, non-nil pointers reused across updates of the owning View.
// A target must not be shared between owners or used as both Bin and Container.
// The UI reconciler owns only the children it mounts; callers must not replace
// or reparent those children behind it. All operations run on the GUI thread.
type Bin interface {
	SetChild(gui.Widget)
}

// Container is a list mounting target, not necessarily a Widget. AddChild
// appends and RemoveChild detaches without destroying. Like Bin, targets must
// have stable pointer identity. Framework teardown removes only managed children;
// window destruction releases UI state without detaching the GUI tree.
type Container interface {
	AddChild(gui.Widget)
	RemoveChild(gui.Widget)
	// MoveChildBefore reorders an existing child without unmounting it.
	// A nil sibling moves the child to the end.
	MoveChildBefore(child, sibling gui.Widget)
}

// TransferContainer identifies the Widget owning a list adapted by a non-Widget
// mounting target. Its empty registration is retained while the owner lives so
// an explicitly transferred node can enter an empty list. Widget containers
// already identify themselves and need not implement this interface.
type TransferContainer interface {
	Container
	Widget() gui.Widget
}

// ViewBase is embedded as ViewBase[ConcreteView] by every declarative view. Its
// chain methods return the concrete *T (through Self) so shared modifiers
// compose with control-specific ones. The concrete constructor must set Self;
// forgetting it makes the guarded panic instead of returning a nil view.
type ViewBase[T any] struct {
	Self *T
	viewBase
}

// ComposeBase supplies declarative identity without widget-only modifiers.
// Embed it in a composition and initialize Self in its constructor.
type ComposeBase[T any] struct {
	Self *T
	viewBase
}

// ID identifies this declaration within its mounting slot. It also names an
// ObjectView for explicit ancestor lookup; it does not rename its widget output.
func (b *ComposeBase[T]) ID(id string) *T {
	b.id = id
	b.fields.Set(viewID, true)
	if b.Self == nil {
		panic("ui: composition not initialized (ComposeBase.Self is nil)")
	}
	return b.Self
}

// Observe replaces this composition's explicit change sources. Empty clears
// the list. Dependencies belong to each mounted node; IDs are not required.
// Sources are stable, non-nil pointers. These sources are combined with the
// State.Get/Read dependencies collected during Build; empty does not clear them.
func (b *ComposeBase[T]) Observe(sources ...Observable) *T {
	for _, source := range sources {
		validateObservable(source)
	}
	b.observables = slices.Clone(sources)
	if b.Self == nil {
		panic("ui: composition not initialized (ComposeBase.Self is nil)")
	}
	return b.Self
}

// BuildView is a composition whose content is produced by a function.
// It adds no GUI widget or layout boundary.
type BuildView struct {
	ComposeBase[BuildView]
	build func() View
}

// Build declares a function-built subtree. fn runs on the UI thread at mount,
// when the parent updates this declaration, or when a dependency changes.
// State.Get/Read calls inside fn track dependencies automatically. Values read
// before fn runs remain snapshots; Go closure capture rules are unchanged.
func Build(fn func() View) *BuildView {
	if fn == nil {
		panic("ui: Build requires a function")
	}
	v := &BuildView{build: fn}
	v.Self = v
	return v
}

func (v *BuildView) Build(BuildContext) View {
	return v.build()
}

// viewBase is the non-generic core every View carries: the shared modifier state
// (id, visibility, style) plus the base() seal that forces embedding. The
// reconciler reads it via View.base after each Update and writes it onto the
// mounted widget, so controls never apply these themselves.
type viewBase struct {
	observables   []Observable // explicit composition dependencies
	shortcuts     []*ShortcutView
	dragSource    *DragSourceView
	dropTarget    *DropTargetView
	id            string
	name          string
	tooltip       string
	styleName     string // semantic style name (Sel.Name); "" reverts to the widget's type default
	minWidth      float32
	minHeight     float32 // size preference; 0 = no min
	maxWidth      float32
	maxHeight     float32 // size preference; 0 = unbounded
	mainWeight    float32 // main-axis extra-space share; 0 = hug
	hidden        bool
	enabled       bool
	focusable     bool
	cursor        Cursor
	onFocus       func(focused bool) // fired when the mounted widget's focus state changes
	onContextMenu func(gui.EventContext)

	fields bits.Bitmap[uint64]
}

const (
	viewID = iota
	viewName
	viewTooltip
	viewStyleName
	viewMinWidth
	viewMinHeight
	viewMaxWidth
	viewMaxHeight
	viewMainWeight
	viewHidden
	viewEnabled
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
	shortcuts         shortcutBindings
	dragSource        dragSourceBinding
	dropTarget        dropTargetBinding
	handles           []signal.Handle    // cross-rebuild handles for shared widget signals
	onFocus           func(focused bool) // effective OnFocus callback, refreshed on every update
	contextMenu       *gui.ContextMenuEventController
	contextMenuHandle signal.Handle
	onContextMenu     func(gui.EventContext)

	// Snapshot of the widget's initial values, captured once in mount before
	// the first apply. Each field corresponds to a viewBase modifier; hidden
	// is stored as the widget's Visible complement.
	initID         string
	initName       string
	initTooltip    string
	initStyleName  string
	initMinSize    geometry.Size
	initMaxSize    geometry.Size
	initMainWeight float32
	initHidden     bool
	initEnabled    bool
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
	ctx.initID = w.ID()
	ctx.initName = w.Name()
	ctx.initTooltip = w.Tooltip()
	ctx.initStyleName = w.StyleName()
	ctx.initMinSize = w.MinSize()
	ctx.initMaxSize = w.MaxSize()
	ctx.initMainWeight = w.MainWeight()
	ctx.initHidden = !w.Visible()
	ctx.initEnabled = w.Enabled()
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
	ctx.onContextMenu = b.onContextMenu
	if b.onContextMenu != nil && ctx.contextMenu == nil {
		ctx.contextMenu = gui.NewContextMenuEventController()
		ctx.contextMenuHandle = ctx.contextMenu.ConnectRequest(func(event gui.EventContext) {
			if ctx.onContextMenu != nil {
				ctx.onContextMenu(event)
			}
		})
		w.AddEventController(ctx.contextMenu)
	} else if b.onContextMenu == nil && ctx.contextMenu != nil {
		w.RemoveEventController(ctx.contextMenu)
		ctx.contextMenuHandle.Disconnect()
		ctx.contextMenu, ctx.contextMenuHandle = nil, nil
	}
	ctx.dragSource.update(w, b.dragSource)
	ctx.dropTarget.update(w, b.dropTarget)
	if len(b.shortcuts) != 0 {
		controller := ctx.shortcuts.controller
		if controller == nil {
			controller = gui.NewShortcutController()
			w.AddEventController(controller)
		}
		ctx.shortcuts.update(controller, b.shortcuts)
	} else if ctx.shortcuts.controller != nil {
		w.RemoveEventController(ctx.shortcuts.controller)
		ctx.shortcuts.clear()
	}
	b.apply(ctx, w)
}

// unmount disconnects all registered shared signal handles and clears the
// context, including the snapshot.
func (b *viewBase) unmount(ctx *viewBaseContext, w gui.Widget) {
	if ctx.contextMenu != nil {
		w.RemoveEventController(ctx.contextMenu)
		ctx.contextMenuHandle.Disconnect()
		ctx.contextMenu, ctx.contextMenuHandle = nil, nil
	}
	ctx.onContextMenu = nil
	ctx.dragSource.clear(w)
	ctx.dropTarget.clear(w)
	if ctx.shortcuts.controller != nil {
		w.RemoveEventController(ctx.shortcuts.controller)
		ctx.shortcuts.clear()
	}
	for _, h := range ctx.handles {
		h.Disconnect()
	}
	ctx.handles = nil
	ctx.onFocus = nil
	ctx.initID = ""
	ctx.initName = ""
	ctx.initStyleName = ""
	ctx.initMinSize = geometry.Size{}
	ctx.initMaxSize = geometry.Size{}
	ctx.initMainWeight = 0
	ctx.initHidden = false
	ctx.initEnabled = false
	ctx.initFocusable = false
	ctx.initCursor = nil
}

func (b *ViewBase[T]) self() *T {
	if b.Self != nil {
		return b.Self
	}
	panic("ui: view not initialized via its constructor (ViewBase.Self is nil)")
}

// OnContextMenu observes a request. Consume it with StopPropagation; keyboard
// requests have no pointer position and should use the control's current item.
func (b *ViewBase[T]) OnContextMenu(fn func(gui.EventContext)) *T {
	b.onContextMenu = fn
	return b.self()
}

// ID assigns a nonempty identifier for precise lookup within this widget's Root.
// ID also identifies this node within its mounting target: the same ID and
// concrete View type retain state across reorder. Changing it replaces the node.
// Empty uses positional identity. IDs remain unique within the GUI Root, not App.
func (b *ViewBase[T]) ID(id string) *T {
	b.id = id
	b.fields.Set(viewID, true)
	return b.self()
}

// Name assigns a repeatable natural name, independent of ID and StyleName.
func (b *ViewBase[T]) Name(name string) *T {
	b.name = name
	b.fields.Set(viewName, true)
	return b.self()
}

// Tooltip assigns supplementary plain-text help. Empty removes the help;
// omitting the modifier on a later update restores the widget's initial value.
func (b *ViewBase[T]) Tooltip(text string) *T {
	b.tooltip = text
	b.fields.Set(viewTooltip, true)
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
// Negative and non-finite values are treated as zero. Unset preserves the
// widget's constructor default; an explicit zero disables expansion.
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

// Enabled restricts this widget and its subtree's user interaction. Omission
// restores the widget's initial own setting; ancestor restrictions still apply.
func (b *ViewBase[T]) Enabled(enabled bool) *T {
	b.enabled = enabled
	b.fields.Set(viewEnabled, true)
	return b.self()
}

// Focusable sets this widget's own focus capability. Omission restores its
// constructor default; disabled ancestors still prevent receiving focus.
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
	if b.fields.Check(viewID) {
		widget.SetID(b.id)
	} else {
		widget.SetID(ctx.initID)
	}
	if b.fields.Check(viewName) {
		widget.SetName(b.name)
	} else {
		widget.SetName(ctx.initName)
	}
	if b.fields.Check(viewTooltip) {
		widget.SetTooltip(b.tooltip)
	} else {
		widget.SetTooltip(ctx.initTooltip)
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
	if b.fields.Check(viewEnabled) {
		widget.SetEnabled(b.enabled)
	} else {
		widget.SetEnabled(ctx.initEnabled)
	}
	if b.fields.Check(viewCursor) {
		widget.SetCursor(b.cursor)
	} else {
		widget.SetCursor(ctx.initCursor)
	}
}
