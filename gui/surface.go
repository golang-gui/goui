package gui

import (
	"fmt"
	"slices"

	"github.com/golang-gui/goui/core/geometry"
	"github.com/golang-gui/goui/core/signal"
	"github.com/golang-gui/goui/platform"
	"github.com/golang-gui/goui/platform/events"
	"github.com/golang-gui/goui/style"
)

// Surface owns a logical presentation host and its Widget tree. It observes
// native geometry, focus and lifecycle without requesting window management.
// Calls belong to the UI thread. Losing a drawing target does not end its life.
type Surface interface {
	Root

	ID() string

	SetID(string)

	SetWidget(Widget)

	// StyleName is the explicit host background style name; empty uses "window".
	// It is not inherited by Widgets and does not customize managed window chrome geometry.
	StyleName() string

	// SetStyleName selects this host's background and requests a repaint.
	// Only BackgroundColor is consumed; empty restores the default style name.
	SetStyleName(string)

	// Transparent reports the immutable creation configuration, not pixel opacity.
	Transparent() bool

	// Shortcuts returns the window-owned controller. Its default Bubble phase
	// runs after the focused widget's tree; modal targets receive input first.
	Shortcuts() *ShortcutController

	// Visible reports the latest native visibility observation.
	Visible() bool

	Focused() bool

	FocusedWidget() Widget

	SetFocusedWidget(Widget) bool

	// PlatformWindow returns the underlying platform window.
	PlatformWindow() platform.Window

	// SetModalTarget makes target intercept this host's input: the window
	// forwards all keys to target and calls target.RequestDismiss() on outside
	// presses or owner focus loss. Escape is interpreted by the target. A nil
	// target clears it. Any modal element can be a target; the window needs only
	// the two ModalTarget methods and never learns the concrete type.
	SetModalTarget(ModalTarget)

	// Destroy releases this host and its Widget tree. It does not request that
	// the operating system close or terminate a system-owned application.
	Destroy()

	Snapshot() SurfaceInfo

	DispatchEvent(event events.Event) error

	// CommitText submits confirmed UTF-8 text to the current editable logical
	// focus through IMContext, without changing focus or activating the host.
	// Call on the GUI thread. A blocked/unavailable target or active preedit
	// returns an error; an empty string is a no-op on a valid target. Success
	// means synchronous dispatch, not completed layout/paint or native typing.
	CommitText(text string) error

	// Popovers returns a copy of the currently shown popovers owned by this
	// window, in registration order. These are borrowed GUI-thread objects;
	// hiding/releasing a popover removes it without changing its ownership.
	Popovers() []Popover

	// ConnectDestroy runs once, after the host is marked destroyed and before
	// its resources are released. Reentrant Destroy calls do not interrupt the
	// notification; normal Handle blocking and disconnection still apply.
	ConnectDestroy(func()) signal.Handle

	// ConnectFocus receives focus changes while the host is alive.
	ConnectFocus(func(focused bool)) signal.Handle
}

type surface struct{ surfaceBase }

// surfaceBase shares presentation, input and lifetime state. Window adds only
// management and decoration. host preserves public Root identity in composition.
type surfaceBase struct {
	rootBase
	host           Surface
	claimed        bool
	visible        bool
	tooltip        tooltipState
	id             string
	styleName      string
	platformWindow platform.Window
	root           Widget
	focused        bool
	activeIM       IMContext            // the focused text widget's context bound to the native IME; nil when none
	inputMethod    platform.InputMethod // this host's native IME; nil when unsupported
	cursor         platform.Cursor      // this host's cursor capability; nil when unsupported
	lastCursor     platform.CursorShape // last shape applied to cursor; dedupes redundant native calls
	cursorPosition geometry.Point       // last client-local pointer position for chrome queries
	cursorInside   bool
	destroyed      bool
	destroyPending bool        // resource cleanup waits for a reentered frame to finish
	inputBlocked   bool        // private host input boundary during an owned native dialog
	modalTarget    ModalTarget // the modal element intercepting this host's input; nil when none
	popoversShown  uint64      // changes whenever an owned surface is shown
	popovers       []*popover  // currently shown surfaces; lifecycle stays with each popover
	destroy        signal.Signal0
	focusChanged   signal.Signal1[bool]
}

func (w *surfaceBase) owner() Surface {
	if w == nil {
		return nil
	}
	if w.host != nil {
		return w.host
	}
	return w
}
func (w *surfaceBase) surfaceCore() *surfaceBase { return w }
func surfaceCore(root Root) *surfaceBase {
	if h, ok := root.(interface{ surfaceCore() *surfaceBase }); ok {
		return h.surfaceCore()
	}
	return nil
}
func (w *surfaceBase) managed() *window { host, _ := w.host.(*window); return host }
func newSystemSurface(app *application) *surface {
	s := &surface{surfaceBase: surfaceBase{rootBase: rootBase{app: app, layoutDirty: true, paintDirty: true, frameSuspended: true}}}
	s.host = s
	return s
}
func (w *surfaceBase) nativeChanged() {
	if host := w.managed(); host != nil && host.chrome != nil {
		host.chrome.nativeChanged()
	}
}
func (w *surfaceBase) ensurePainter() error {
	if w.painter != nil || w.platformWindow == nil || !w.platformWindow.SurfaceState().Ready {
		return nil
	}
	painter, err := w.app.platform.NewPainter(w.platformWindow)
	if err != nil {
		return fmt.Errorf("create surface painter: %w", err)
	}
	if w.destroyed {
		painter.Destroy()
		return platform.ErrUnavailable
	}
	w.painter = painter
	return nil
}
func (w *surfaceBase) bind(native platform.Window) {
	if w.destroyed || w.app.quitting.Load() {
		native.Destroy()
		return
	}
	w.platformWindow, w.surface = native, native
	w.transparent = native.Transparent()
	input, _ := w.app.platform.NewInputMethod(native, w.onInputMethod)
	if w.destroyed {
		if input != nil {
			input.Destroy()
		}
		return
	}
	w.inputMethod = input
	cursor, _ := w.app.platform.NewCursor(native)
	if w.destroyed {
		if cursor != nil {
			cursor.Destroy()
		}
		return
	}
	w.cursor = cursor
	if w.focusedWidget != nil {
		w.updateInputMethod(w.focusedWidget)
	}
}
func (w *surfaceBase) paint() {
	if !w.claimed || w.destroyed || w.app != nil && w.app.quitting.Load() {
		return
	}
	defer w.finishDestroy()
	if err := w.ensurePainter(); err != nil {
		if w.destroyed {
			return
		}
		w.app.runError = err
		w.app.Quit()
		return
	}
	w.root = liveRoot(w.root)
	w.paintFrame(w.root, w.backgroundStyle())
}

// onInputMethod is the window's platform.InputMethodHandler: it routes native
// input-method output to the focused widget's IMContext (see doc/DesignIME.md §3).
func (w *surfaceBase) onInputMethod(r platform.InputMethodResult) {
	previous := w.activeIM
	ensureModal(w.owner())
	if w.destroyed || w.activeIM == nil || previous != w.activeIM {
		return
	}
	// A modal target owns keyboard input. A trailing native result must not
	// edit the underlying field; allow only the cancellation of its preedit.
	if (modalRootBlocked(w.owner()) || w.modalTarget != nil || w.focusedWidget == nil || !modalWidgetAllowed(w.focusedWidget)) && (r.Kind != platform.InputMethodPreedit || r.Text != "") {
		return
	}
	switch r.Kind {
	case platform.InputMethodCommit:
		w.activeIM.emitCommit(IMCommit{Text: r.Text, Composed: r.Composed})
	case platform.InputMethodPreedit:
		w.activeIM.emitPreedit(r.Text, r.Caret)
	}
}

func (w *surfaceBase) ID() string {
	return w.id
}

func (w *surfaceBase) SetID(id string) {
	w.id = id
}

func (w *surfaceBase) StyleName() string { return w.styleName }

func (w *surfaceBase) SetStyleName(name string) {
	if w.destroyed || w.styleName == name {
		return
	}
	w.styleName = name
	w.requestPaint()
}

func (w *surfaceBase) backgroundStyle() style.Style {
	name := w.styleName
	if name == "" {
		name = styleNameWindow
	}
	return ResolveStyle(name, style.PartDefault, style.Normal)
}

func (w *surfaceBase) Visible() bool { return !w.destroyed && w.visible }

func (w *surfaceBase) Focused() bool {
	return !w.destroyed && w.focused
}

func (w *surfaceBase) SetFocusedWidget(widget Widget) bool {
	ensureModal(w.owner())
	if widget == nil {
		w.setFocusedWidget(nil)
		return true
	}
	if w.destroyed || widget.base().destroyed || widget.Root() != w.owner() || !widget.Focusable() || !IsEnabled(widget) || !visibleInTree(widget) || !modalWidgetAllowed(widget) {
		return false
	}
	if w.focusedWidget == widget {
		w.setFocusedWidget(widget)
		return true
	}

	w.setFocusedWidget(widget)
	return true
}

func (w *surfaceBase) Widget() Widget {
	w.root = liveRoot(w.root)
	return w.root
}

func (w *surfaceBase) SetWidget(widget Widget) {
	if w.destroyed || w.root == widget {
		return
	}
	if widget != nil && widget.base().destroyed {
		return
	}
	if w.root != nil {
		w.root.base().detachRoot(w.root)
	}
	if widget != nil {
		adoptWidget(widget, w.owner())
	}
	w.root = widget
	modalChanged(w.owner())
	ensureModal(w.owner())
	w.dragControllersChanged()
	w.requestLayout()
}

func (w *surfaceBase) RequestPaint() error {
	return w.requestPaint()
}

func (w *surfaceBase) Destroy() {
	if w.destroyed {
		return
	}
	w.destroyed = true
	w.destroyPending = true
	w.destroyFrames()
	w.finishDestroy()
}

func (w *surfaceBase) finishDestroy() {
	if !w.destroyPending || w.framePainting {
		return
	}
	w.destroyPending = false
	w.tooltip.dismiss(false)
	w.cancelInput(GestureHostClosed)
	w.drag.destroy()
	if c := w.dispatcher.shortcuts; c != nil {
		c.destroyed = true
		c.Clear()
	}
	if host := w.managed(); host != nil && host.chrome != nil {
		host.chrome.destroy()
	}
	w.destroy.Emit()

	w.dispatcher.decoration = nil
	if host := w.managed(); host != nil && host.controls != nil {
		host.controls.release()
		host.controls = nil
	}

	if w.root != nil {
		root := w.root
		root.base().detachRoot(root)
		root.base().destroy(root)
	}
	if w.inputMethod != nil {
		w.inputMethod.Destroy() // release the native input context before the window
		w.inputMethod = nil
	}
	if w.cursor != nil {
		w.cursor.Destroy()
		w.cursor = nil
	}
	if w.painter != nil {
		w.painter.Destroy()
		w.painter = nil
	}
	if w.platformWindow != nil {
		w.platformWindow.Destroy()
		w.platformWindow = nil
	}
	if w.app != nil {
		w.app.removeSurface(w.owner())
	}
}

func (w *surfaceBase) Snapshot() SurfaceInfo {
	info := SurfaceInfo{
		Visible: w.Visible(), Focused: w.Focused() && !w.destroyed,
		ID: w.ID(),
		Bounds: geometry.Rect(
			0,
			0,
			w.width,
			w.height,
		),
	}
	if w.root != nil {
		info.Widget = w.root.Snapshot()
	}
	if host := w.managed(); host != nil {
		info.Title = host.Title()
		if host.controls != nil && host.controls.Visible() {
			controls := host.controls.Snapshot()
			info.Controls = &controls
		}
	}
	return info
}

func (w *surfaceBase) Popovers() []Popover {
	result := make([]Popover, 0, len(w.popovers))
	if !w.destroyed {
		for _, p := range w.popovers {
			result = append(result, p)
		}
	}
	return result
}

func (w *surfaceBase) addPopover(p *popover) {
	w.popoversShown++
	if !w.destroyed && !slices.Contains(w.popovers, p) {
		w.popovers = append(w.popovers, p)
	}
}

func (w *surfaceBase) removePopover(p *popover) {
	if i := slices.Index(w.popovers, p); i >= 0 {
		w.popovers = slices.Delete(w.popovers, i, i+1)
	}
}

func (w *surfaceBase) CommitText(text string) error {
	return commitText(w.owner(), text)
}

func (w *surfaceBase) DispatchEvent(event events.Event) error {
	if _, ok := event.(events.DestroyEvent); ok {
		w.Destroy()
		return nil
	}
	if e, ok := event.(events.DragSourceEvent); ok {
		w.app.dispatchDragSourceEvent(e)
		return nil
	}
	if w.destroyed {
		return nil
	}
	if modalRootBlocked(w.owner()) {
		switch e := event.(type) {
		case events.KeyEvent:
			modalPreventDefault(e)
			return nil
		case events.PointerEvent:
			// Preserve pointer facts without running hover or gesture controllers.
			w.cursorPosition = e.Position
			w.cursorInside = e.EventType != events.PointerLeave && containsPoint(geometry.Rect(0, 0, w.width, w.height), e.Position)
			w.dispatcher.pointerEvent = e
			w.dispatcher.pointerInside = w.cursorInside
			return nil
		case events.WheelEvent, events.CloseEvent:
			return nil
		case events.DragOfferEvent:
			if e.EventType == events.DragDrop {
				_ = e.Offer.Finish(0)
			}
			return nil
		case events.DragDataEvent:
			return nil
		}
	}
	ensureModal(w.owner())
	if w.destroyed {
		return nil
	}
	w.tooltip.dismissOnEvent(w.owner(), event)
	if w.destroyed {
		return nil
	}
	if e, ok := event.(events.DragOfferEvent); ok {
		if w.modalTarget != nil {
			if e.EventType == events.DragDrop {
				_ = e.Offer.Finish(0)
			}
			return nil
		}
		w.dispatchDragOffer(w.owner(), e)
		return nil
	}
	if e, ok := event.(events.DragDataEvent); ok {
		w.dispatchDragData(w.owner(), e)
		return nil
	}
	switch e := event.(type) {
	case events.PointerEvent:
		w.cursorPosition = e.Position
		w.cursorInside = e.EventType != events.PointerLeave && containsPoint(geometry.Rect(0, 0, w.width, w.height), e.Position)
		defer w.applyCursor()
	case events.FocusEvent:
		if !e.Focused {
			w.cursorInside = false
		}
		defer w.applyCursor()
	case events.StateEvent, events.SizeEvent:
		defer w.applyCursor()
	}
	if controller := w.dispatcher.hostController; controller != nil {
		switch event.(type) {
		case events.FocusEvent, events.StateEvent, events.SizeEvent:
			controller.Reset()
		}
		if w.modalTarget != nil {
			controller.Reset()
		}
	}
	// Only surfaces present at the start of this event may react to it. A close
	// callback can destroy the owner or install a different input target.
	modal := w.modalTarget
	if w.dismissPopovers(event) || w.destroyed {
		return nil
	}
	if w.modalTarget == modal && w.routeToModalTarget(event) || w.destroyed {
		return nil
	}
	switch event := event.(type) {
	case events.SurfaceEvent:
		w.surfaceChanged()
		if !event.State.Ready {
			w.cancelInput(GestureInterrupted)
		}
		if event.State.Ready {
			w.requestLayout()
		}
	case events.VisibilityEvent:
		w.visible = event.Visible
		if event.Visible && (w.claimed || w.managed() != nil) {
			w.resumeFrames(w.platformWindow)
		} else {
			w.suspendFrames()
			w.cancelInput(GestureInterrupted)
		}
	case events.CloseEvent:
		if host := w.managed(); host != nil {
			allow := true
			host.closeRequest.Emit(&allow)
			if allow {
				host.Destroy()
			}
		}
	case events.SizeEvent:
		w.width = event.Width
		w.height = event.Height
		w.pixelWidth = event.PixelWidth
		w.pixelHeight = event.PixelHeight
		w.requestLayout()
		w.nativeChanged()
	case events.FocusEvent:
		w.nativeChanged()
		if w.destroyed {
			return nil
		}
		w.setFocused(event.Focused)
		if w.destroyed {
			return nil
		}
		return w.dispatcher.DispatchEvent(w.owner(), event)
	case events.StateEvent:
		if w.managed() == nil {
			return nil
		}
		if event.State == WindowStateHidden || event.State == WindowStateMinimized {
			w.suspendFrames()
		} else if event.State != WindowStateUnknown && w.frameSuspended {
			w.resumeFrames(w.platformWindow)
		}
		w.nativeChanged()
		if w.destroyed {
			return nil
		}
		w.managed().stateChanged.Emit(event.State)
	case events.PaintEvent:
		if host := w.managed(); host != nil {
			host.paint()
		} else {
			w.paint()
		}
	case events.PointerEvent:
		err := w.dispatcher.DispatchEvent(w.owner(), event)
		return err
	case events.KeyEvent:
		err := w.dispatcher.DispatchEvent(w.owner(), event)
		w.applyCursor() // re-resolve in case the focused widget changed its cursor (e.g., hide-on-typing)
		return err
	default:
		return w.dispatcher.DispatchEvent(w.owner(), event)
	}
	return nil
}

func (w *surfaceBase) Shortcuts() *ShortcutController {
	if w.dispatcher.shortcuts == nil {
		w.dispatcher.shortcuts = NewShortcutController()
		w.dispatcher.shortcuts.destroyed = w.destroyed
	}
	return w.dispatcher.shortcuts
}

func (w *surfaceBase) ConnectDestroy(fn func()) signal.Handle {
	if w.destroyed {
		return signal.Handles(nil)
	}
	return w.destroy.Connect(fn)
}

func (w *surfaceBase) ConnectFocus(fn func(bool)) signal.Handle {
	if w.destroyed {
		return signal.Handles(nil)
	}
	return w.focusChanged.Connect(func(focused bool) {
		if !w.destroyed {
			fn(focused)
		}
	})
}

func (w *surfaceBase) onEvent(event events.Event) {
	_ = w.DispatchEvent(event)
}

// SetModalTarget installs (or with nil clears) the window's modal input target.
// See the Window interface.
func (w *surfaceBase) SetModalTarget(target ModalTarget) {
	if w.destroyed {
		return
	}
	if target != nil && !modalTargetAllowed(w.owner(), target) {
		return
	}
	wasModal := w.modalTarget != nil
	epoch := w.tooltip.epoch
	w.tooltip.dismiss(false)
	if w.destroyed || w.tooltip.epoch != epoch+1 {
		return
	}
	w.modalTarget = target
	if target != nil {
		w.cancelInput(GestureInterrupted)
	}
	if target != nil && !wasModal {
		w.imReset()
		if w.destroyed {
			return
		}
	}
	w.imSetEnabled(w.activeIM != nil)
	w.applyCursor()
}

// dismissPopovers observes the existing surface list without owning resources
// or replaying input. Modal input remains exclusive; modeless presses can
// continue to the original owner target after dismissal callbacks.
func (w *surfaceBase) dismissPopovers(event events.Event) bool {
	outside, escape, deactivated := false, false, false
	var point geometry.Point
	switch e := event.(type) {
	case events.PointerEvent:
		outside, point = e.EventType == events.PointerDown, e.Position
	case events.KeyEvent:
		escape = e.EventType == events.KeyDown && e.Key == events.KeyEscape
	case events.FocusEvent:
		deactivated = !e.Focused
	}
	if !outside && !escape && !deactivated || w.modalTarget != nil && !deactivated {
		return false
	}
	root := w.Widget()
	var picked Widget
	if outside {
		picked = w.dispatcher.pick(root, point)
	}
	shown := w.popoversShown
	type entry struct {
		surface   *popover
		lifecycle uint64
	}
	entries := make([]entry, 0, len(w.popovers))
	for _, p := range w.popovers {
		if p.visible && !p.modal && p.autoDismiss {
			entries = append(entries, entry{p, p.lifecycle})
		}
	}
	for i := len(entries) - 1; i >= 0; i-- {
		p := entries[i].surface
		if p.lifecycle != entries[i].lifecycle || p.owner != w.owner() || !p.visible || p.destroyed || p.modal || !p.autoDismiss {
			continue
		}
		if outside && picked != nil && p.anchor != nil && picked.base().isDescendant(picked, p.anchor) {
			continue
		}
		p.RequestDismiss()
		if escape {
			event.(events.KeyEvent).PreventDefault()
			return true
		}
		if w.destroyed {
			return true
		}
		if outside && (w.popoversShown != shown || w.Widget() != root || w.dispatcher.pick(root, point) != picked) {
			return true
		}
	}
	return false
}

// routeToModalTarget forwards the window's input to its modal target (§7): the
// target has no native focus, so the window drives the modal policy — keyboard
// including Escape goes to the target, while an outside press or owner
// deactivation requests dismissal. Returns true when it consumes the event.
func (w *surfaceBase) routeToModalTarget(event events.Event) bool {
	if w.modalTarget == nil {
		return false
	}
	switch e := event.(type) {
	case events.KeyEvent:
		e.PreventDefault() // never deliver owner text after forwarding to a popup
		// Escape is also a key: its meaning belongs to the target's policy.
		// Give it a separate result after suppressing the owner's native default.
		handled := false
		e.Handled = &handled
		_ = w.modalTarget.DispatchEvent(e)
		return true
	case events.PointerEvent:
		if e.EventType == events.PointerDown {
			setFocusVisible(w.owner(), false)
			w.modalTarget.RequestDismiss() // the owner only ever sees clicks outside the target
			w.cancelInput(GestureInterrupted)
		}
		return true // swallow the window's own pointer while a modal target is open
	case events.FocusEvent:
		if !e.Focused {
			w.modalTarget.RequestDismiss()
		}
		return false // window still handles its own focus normally
	}
	return false
}

// PlatformWindow is the escape hatch to the underlying platform window.
func (w *surfaceBase) PlatformWindow() platform.Window { return w.platformWindow }

// RequestLayout satisfies Root: schedule a relayout of this host.
func (w *surfaceBase) RequestLayout() { w.requestLayout() }

func (w *surfaceBase) requestLayout() {
	w.rootBase.requestLayout(w.platformWindow)
}

func (w *surfaceBase) requestPaint() error {
	return w.rootBase.requestPaint(w.platformWindow)
}

func (w *surfaceBase) setFocused(focused bool) {
	if w.focused == focused {
		return
	}
	w.focused = focused
	w.focusChanged.Emit(focused)
}

func (w *surfaceBase) setFocusedWidget(widget Widget) {
	if w.focusedWidget == widget {
		_ = w.dispatcher.DispatchEvent(w.owner(), events.FocusEvent{Focused: w.focused})
		return
	}
	w.focusedWidget = widget
	w.updateInputMethod(widget)
	_ = w.dispatcher.DispatchEvent(w.owner(), events.FocusEvent{Focused: w.focused})
	w.requestPaint()
}

// updateInputMethod rebinds the window's single native IME to the newly focused
// widget. The outgoing context is finished and unbound first (so a trailing
// commit lands on the old widget), then the incoming widget's IMContext — if it
// is an IMClient — is bound and the IME enabled; a non-text widget just disables
// it. Widget authors do nothing here (see doc/DesignIME.md §5).
func (w *surfaceBase) updateInputMethod(focused Widget) {
	if w.activeIM != nil {
		w.imReset()
		w.activeIM.setSurface(nil)
		w.activeIM = nil
	}
	im := imContextOf(focused)
	if im == nil {
		w.imSetEnabled(false)
		return
	}
	w.activeIM = im
	im.setSurface(w.owner().(inputMethodHost))
	w.imSetEnabled(true)
	// The caret rect is pushed by the focused widget's next paint (setFocusedWidget
	// requests one), so the window need not read it back here.
}

func (w *surfaceBase) imSetEnabled(enabled bool) {
	if w.inputMethod != nil {
		w.inputMethod.SetEnabled(enabled && !modalRootBlocked(w.owner()) && w.modalTarget == nil)
	}
}

// imSetCaretRect is called by the active IMContext: it offsets the context's
// widget-local caret rect into window coordinates and forwards it to the platform.
func (w *surfaceBase) imSetCaretRect(local geometry.Rectangle) {
	if w.inputMethod == nil || w.focusedWidget == nil {
		return
	}
	origin := w.focusedWidget.base().windowRect().Pos
	w.inputMethod.SetCaretRect(geometry.Rect(
		local.X+origin.X,
		local.Y+origin.Y,
		local.Width,
		local.Height,
	))
}

// imReset asks the platform to cancel any in-progress composition.
func (w *surfaceBase) imReset() {
	if w.inputMethod != nil {
		w.inputMethod.Reset()
	}
}

// applyCursor uses the accepted gesture's owner while captured, otherwise the
// live hover path. The innermost non-Default cursor wins. Hover still updates
// during capture; ending or handing off the gesture restores that live cursor.
func (w *surfaceBase) applyCursor() {
	if w.destroyed || w.cursor == nil {
		return
	}
	var resolved Cursor = CursorDefault
	path := w.dispatcher.hoverPath
	if capture := w.dispatcher.captureTarget; capture != nil && !capture.base().destroyed && IsEnabled(capture) && visibleInTree(capture) {
		path = widgetPath(w.dispatcher.treeRoot(w.Widget(), capture), capture)
	}
	for i := len(path) - 1; i >= 0; i-- {
		if !IsEnabled(path[i]) {
			continue
		}
		if c := path[i].Cursor(); c != nil && c != CursorDefault {
			resolved = c
			break
		}
	}
	// MVP: only CursorShape is supported; future custom cursors will need their
	// own resolution (image → fallback shape if platform rejects).
	shape, ok := resolved.(CursorShape)
	if !ok {
		shape = CursorDefault
	}
	if host := w.managed(); host != nil {
		if frame := host.frameCursor(); frame != CursorDefault {
			shape = frame
		}
	}
	if w.destroyed || w.cursor == nil {
		return
	}
	platformShape := platform.CursorShape(shape)
	if platformShape == w.lastCursor {
		return
	}
	w.lastCursor = platformShape
	w.cursor.SetShape(platformShape)
}
