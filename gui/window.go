package gui

import (
	"errors"
	"fmt"
	"log"

	"github.com/golang-gui/goui/core/geometry"
	"github.com/golang-gui/goui/core/signal"
	"github.com/golang-gui/goui/layout"
	"github.com/golang-gui/goui/platform"
	"github.com/golang-gui/goui/platform/events"
	"github.com/golang-gui/goui/style"
)

// Window owns a native window and its Widget tree. Operations and callbacks run
// on the GUI thread. Connections are controlled by their returned Handles.
// Once destruction starts, Connect methods return inert Handles and ordinary
// notifications skip callbacks that have not started; destroy callbacks still run.
type Window interface {
	Root

	ID() string
	SetID(string)
	SetWidget(Widget)

	Title() string
	SetTitle(string) error
	// Transparent reports the immutable creation configuration, not pixel opacity.
	Transparent() bool
	// Chrome returns the stable, non-nil titlebar integration signal service.
	// Unsupported integration uses a service that reports disabled information.
	Chrome() WindowChrome
	// State reports one native presentation state, or Unknown. RequestState
	// is best effort; it does not promise the requested result.
	// Normal requests an ordinary window, not a historical Restore operation.
	State() WindowState
	RequestState(WindowState)

	Focused() bool
	FocusedWidget() Widget
	SetFocusedWidget(Widget) bool

	Show() error

	// PlatformWindow returns the underlying platform window.
	PlatformWindow() platform.Window

	// SetModalTarget makes target intercept this window's input: the window
	// forwards its keyboard to target and calls target.RequestDismiss() when it
	// sees a dismiss-worthy interaction (outside click / Esc / focus loss). A nil
	// target clears it. Any modal element can be a target; the window needs only
	// the two ModalTarget methods and never learns the concrete type.
	SetModalTarget(ModalTarget)

	RequestClose() error
	Destroy()

	// SetMinSize supplies a desktop WM hint in DIP; it is a no-op for non-desktop
	// hosts. It follows the native creation-size convention: Windows None and
	// Integrated use outer size, while the other current desktop modes use client
	// size. Widget layout constraints are independent of this advisory hint.
	SetMinSize(geometry.Size)

	Snapshot() WindowInfo
	DispatchEvent(event events.Event) error

	// ConnectCloseRequest lets fn veto a close request by setting *allow to false.
	// Each callback runs only while the window is alive.
	ConnectCloseRequest(func(allow *bool)) signal.Handle
	// ConnectDestroy runs once, after the window is marked destroyed and before
	// its resources are released. Reentrant Destroy calls do not interrupt the
	// notification; normal Handle blocking and disconnection still apply.
	ConnectDestroy(func()) signal.Handle
	// ConnectFocus receives focus changes while the window is alive.
	ConnectFocus(func(focused bool)) signal.Handle
	// ConnectState receives observed native transitions through
	// DispatchEvent while the window is alive. No event is synthesized just
	// because RequestState succeeds.
	ConnectState(func(state WindowState)) signal.Handle
}

// ModalTarget is a modal element a window forwards its input to (see
// Window.SetModalTarget). The window drives it: it dispatches forwarded keyboard
// via DispatchEvent and calls RequestDismiss on a dismiss-worthy interaction.
// A popover implements it, but so can any element — the window depends only on
// these two methods, never on a concrete type.
type ModalTarget interface {
	DispatchEvent(events.Event) error
	RequestDismiss()
}

type WindowState = platform.WindowState

const (
	WindowStateUnknown    = platform.WindowStateUnknown
	WindowStateNormal     = platform.WindowStateNormal
	WindowStateHidden     = platform.WindowStateHidden
	WindowStateMinimized  = platform.WindowStateMinimized
	WindowStateMaximized  = platform.WindowStateMaximized
	WindowStateFullscreen = platform.WindowStateFullscreen
)

type window struct {
	rootBase
	app            *application
	id             string
	title          string
	platformWindow platform.Window
	root           Widget
	dispatcher     EventDispatcher
	focused        bool
	activeIM       IMContext            // the focused text widget's context bound to the native IME; nil when none
	inputMethod    platform.InputMethod // this window's platform IME; nil when the platform has none
	cursor         platform.Cursor      // this window's platform cursor capability; nil when the platform has none
	lastCursor     platform.CursorShape // last shape applied to cursor; dedupes redundant native calls
	destroyed      bool
	modalTarget    ModalTarget // the modal element intercepting this window's input; nil when none
	closeRequest   signal.Signal1[*bool]
	destroy        signal.Signal0
	focusChanged   signal.Signal1[bool]
	stateChanged   signal.Signal1[WindowState]
	minSize        geometry.Size // explicit minimum from SetMinSize; zero means derive from tree
	minSizeApplied bool          // true once the first layout has derived and applied the min-size hint
	chrome         *windowChrome
	controls       *windowControls
	layingOut      bool
}

func newWindow(app *application, options WindowOptions) (*window, error) {
	win := &window{
		app:      app,
		rootBase: rootBase{layoutDirty: true, paintDirty: true, transparent: options.Transparent},
	}

	mode := options.Chrome
	nativeOptions := platform.WindowOptions{Chrome: platform.WindowChrome(mode), Transparent: options.Transparent}
	platformWindow, err := app.platform.NewWindow(options.Size, win.onEvent, nativeOptions)
	if mode == WindowChromeIntegrated && errors.Is(err, platform.ErrUnsupported) && platformWindow == nil {
		mode = WindowChromeNative
		nativeOptions.Chrome = platform.WindowChromeNative
		platformWindow, err = app.platform.NewWindow(options.Size, win.onEvent, nativeOptions)
	}
	if err != nil {
		return nil, fmt.Errorf("create platform window: %w", err)
	}
	win.platformWindow = platformWindow
	win.Chrome()
	if err := win.chrome.initialize(mode); err != nil {
		win.Destroy()
		return nil, fmt.Errorf("create window chrome: %w", err)
	}
	if win.chrome.info.Controls != ChromeControlsNone {
		win.controls = newWindowControls(win)
		win.dispatcher.decoration = win.controls
	}

	win.painter, err = app.platform.NewPainter(platformWindow)
	if err != nil {
		win.Destroy()
		return nil, fmt.Errorf("create painter: %w", err)
	}

	win.title = platformWindow.Title()

	// Text input (IME) is an optional platform capability; a nil result (the
	// platform has no input method) just means text widgets fall back to plain
	// key events. Commit/preedit are routed to the focused widget's IMContext.
	win.inputMethod, _ = app.platform.NewInputMethod(platformWindow, win.onInputMethod)

	// Cursor is an optional platform capability; nil means the platform controls
	// the cursor itself (no dynamic per-widget cursor support).
	win.cursor, _ = app.platform.NewCursor(platformWindow)

	return win, nil
}

// onInputMethod is the window's platform.InputMethodHandler: it routes native
// input-method output to the focused widget's IMContext (see doc/DesignIME.md §3).
func (w *window) onInputMethod(r platform.InputMethodResult) {
	if w.activeIM == nil {
		return
	}
	switch r.Kind {
	case platform.InputMethodCommit:
		w.activeIM.emitCommit(r.Text)
	case platform.InputMethodPreedit:
		w.activeIM.emitPreedit(r.Text, r.Caret)
	}
}

func (w *window) ID() string {
	return w.id
}

func (w *window) SetID(id string) {
	w.id = id
}

func (w *window) Title() string {
	if w.title != "" {
		return w.title
	}
	if w.platformWindow != nil {
		return w.platformWindow.Title()
	}
	return ""
}

func (w *window) SetTitle(title string) error {
	if w.platformWindow == nil {
		w.title = title
		return nil
	}
	if err := w.platformWindow.SetTitle(title); err != nil {
		return err
	}
	w.title = title
	return nil
}

func (w *window) Focused() bool {
	return w.focused
}

func (w *window) SetFocusedWidget(widget Widget) bool {
	if widget == nil {
		w.setFocusedWidget(nil)
		return true
	}
	if widget.Window() != w || !widget.Focusable() || !visibleInTree(widget) {
		return false
	}
	if w.focusedWidget == widget {
		w.setFocusedWidget(widget)
		return true
	}

	w.setFocusedWidget(widget)
	return true
}

func (w *window) Widget() Widget {
	w.root = liveRoot(w.root)
	return w.root
}

func (w *window) SetWidget(widget Widget) {
	if w.root == widget {
		return
	}
	if widget != nil && widget.base().destroyed {
		return
	}
	if w.root != nil {
		w.root.base().detachRoot(w.root)
	}
	if widget != nil {
		adoptWidget(widget, w)
	}
	w.root = widget
	w.requestLayout()
}

func (w *window) Show() error {
	if w.platformWindow == nil {
		return nil
	}
	err := w.platformWindow.Show()
	if w.chrome != nil {
		w.chrome.nativeChanged()
	}
	return err
}

func (w *window) RequestPaint() error {
	return w.requestPaint()
}

// RequestCursorUpdate re-resolves the cursor shape from the hover path and
// applies it. Widgets call this via Root when they change their cursor and are
// being hovered over.
func (w *window) RequestClose() error {
	if w.platformWindow == nil {
		return nil
	}
	return w.platformWindow.RequestClose()
}

func (w *window) Destroy() {
	if w.destroyed {
		return
	}
	w.destroyed = true
	if w.chrome != nil {
		w.chrome.destroy()
	}
	w.destroy.Emit()

	w.dispatcher.decoration = nil
	if w.controls != nil {
		w.controls.release()
		w.controls = nil
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
		w.app.removeWindow(w)
	}
}

func (w *window) Snapshot() WindowInfo {
	info := WindowInfo{
		ID:    w.ID(),
		Title: w.Title(),
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
	if w.controls != nil && w.controls.Visible() {
		controls := w.controls.Snapshot()
		info.Controls = &controls
	}
	return info
}

func (w *window) DispatchEvent(event events.Event) error {
	if w.destroyed {
		return nil
	}
	// Dismissing a modal target can destroy its owner without consuming the event.
	if w.routeToModalTarget(event) || w.destroyed {
		return nil
	}
	switch event := event.(type) {
	case events.CloseEvent:
		allow := true
		w.closeRequest.Emit(&allow)
		if allow {
			w.Destroy()
		}
	case events.SizeEvent:
		w.width = event.Width
		w.height = event.Height
		w.pixelWidth = event.PixelWidth
		w.pixelHeight = event.PixelHeight
		w.requestLayout()
		if w.chrome != nil {
			w.chrome.nativeChanged()
		}
	case events.FocusEvent:
		if w.chrome != nil {
			w.chrome.nativeChanged()
		}
		if w.destroyed {
			return nil
		}
		w.setFocused(event.Focused)
		if w.destroyed {
			return nil
		}
		return w.dispatcher.DispatchEvent(w, event)
	case events.StateEvent:
		if w.chrome != nil {
			w.chrome.nativeChanged()
		}
		if w.destroyed {
			return nil
		}
		w.stateChanged.Emit(event.State)
	case events.PaintEvent:
		w.paint()
	case events.PointerEvent:
		err := w.dispatcher.DispatchEvent(w, event)
		w.applyCursor() // re-resolve cursor after hover path changes
		return err
	case events.KeyEvent:
		err := w.dispatcher.DispatchEvent(w, event)
		w.applyCursor() // re-resolve in case the focused widget changed its cursor (e.g., hide-on-typing)
		return err
	default:
		return w.dispatcher.DispatchEvent(w, event)
	}
	return nil
}

func (w *window) ConnectCloseRequest(fn func(*bool)) signal.Handle {
	if w.destroyed {
		return signal.Handles(nil)
	}
	return w.closeRequest.Connect(func(allow *bool) {
		if !w.destroyed {
			fn(allow)
		}
	})
}

func (w *window) ConnectDestroy(fn func()) signal.Handle {
	if w.destroyed {
		return signal.Handles(nil)
	}
	return w.destroy.Connect(fn)
}

func (w *window) ConnectFocus(fn func(bool)) signal.Handle {
	if w.destroyed {
		return signal.Handles(nil)
	}
	return w.focusChanged.Connect(func(focused bool) {
		if !w.destroyed {
			fn(focused)
		}
	})
}

func (w *window) SetMinSize(size geometry.Size) {
	w.minSize = size
	if native, err := w.desktopWindow(); err == nil {
		native.SetMinSize(size.Width, size.Height)
	}
}

// updateMinSize derives and applies the window-manager min-size hint from the
// widget tree. Called on the first layout pass when no explicit SetMinSize was
// given. The content is measured with no upper bound (Loose(Inf)) to discover
// its preferred intrinsic size.
func (w *window) updateMinSize() {
	if w.minSizeApplied {
		return
	}
	w.minSizeApplied = true
	// Explicit SetMinSize wins over auto-derivation.
	if w.minSize.Width > 0 || w.minSize.Height > 0 {
		return
	}
	native, err := w.desktopWindow()
	if w.root == nil || err != nil {
		return
	}
	pref := measureWidget(w.root, layout.Unbounded()).Size
	if pref.Width > 0 || pref.Height > 0 {
		native.SetMinSize(pref.Width, pref.Height)
	}
}

func (w *window) onEvent(event events.Event) {
	_ = w.DispatchEvent(event)
}

// SetModalTarget installs (or with nil clears) the window's modal input target.
// See the Window interface.
func (w *window) SetModalTarget(target ModalTarget) {
	w.modalTarget = target
}

// routeToModalTarget forwards the window's input to its modal target (§7): the
// target has no native focus, so the window drives the modal policy — keyboard
// goes to the target, and an outside click / Esc / focus loss asks it to
// dismiss. Returns true when it consumes the event.
func (w *window) routeToModalTarget(event events.Event) bool {
	if w.modalTarget == nil {
		return false
	}
	switch e := event.(type) {
	case events.KeyEvent:
		if e.EventType == events.KeyDown && e.Key == events.KeyEscape {
			w.modalTarget.RequestDismiss()
		} else {
			_ = w.modalTarget.DispatchEvent(event)
		}
		return true
	case events.PointerEvent:
		if e.EventType == events.PointerDown {
			w.modalTarget.RequestDismiss()   // the owner only ever sees clicks outside the target
			w.dispatcher.captureTarget = nil // clear any stale capture from the window's own tree
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

func (w *window) paint() {
	w.root = liveRoot(w.root)
	w.paintDirty = false
	if w.chrome != nil {
		w.chrome.beforeLayout()
	}
	if w.destroyed {
		return
	}
	w.updateMinSize()
	w.layoutContent()
	if w.chrome != nil && w.chrome.afterLayout() && !w.destroyed {
		// Notifications only invalidate. Apply changed reservations once in
		// this frame, without asking for height again or running a fixed point.
		w.layoutContent()
	}
	if w.destroyed {
		return
	}
	var controls Widget
	if w.controls != nil {
		w.controls.layout()
		controls = w.controls
	}
	w.drawFrame(w.root, ResolveStyle(styleNameWindow, style.PartDefault, style.Normal), controls)
}

func (w *window) layoutContent() {
	w.layingOut = true
	defer func() { w.layingOut = false }()
	w.layoutFrame(w.root)
}

// PlatformWindow is the escape hatch to the underlying platform window.
func (w *window) PlatformWindow() platform.Window { return w.platformWindow }

// RequestLayout satisfies Root: schedule a relayout of this host.
func (w *window) RequestLayout() { w.requestLayout() }

func (w *window) requestLayout() {
	w.rootBase.requestLayout(w.platformWindow)
}

func (w *window) requestPaint() error {
	return w.rootBase.requestPaint(w.platformWindow)
}

func (w *window) setFocused(focused bool) {
	if w.focused == focused {
		return
	}
	w.focused = focused
	w.focusChanged.Emit(focused)
}

func (w *window) setFocusedWidget(widget Widget) {
	if w.focusedWidget == widget {
		_ = w.dispatcher.DispatchEvent(w, events.FocusEvent{Focused: w.focused})
		return
	}
	w.focusedWidget = widget
	w.updateInputMethod(widget)
	_ = w.dispatcher.DispatchEvent(w, events.FocusEvent{Focused: w.focused})
	w.requestPaint()
}

// updateInputMethod rebinds the window's single native IME to the newly focused
// widget. The outgoing context is finished and unbound first (so a trailing
// commit lands on the old widget), then the incoming widget's IMContext — if it
// is an IMClient — is bound and the IME enabled; a non-text widget just disables
// it. Widget authors do nothing here (see doc/DesignIME.md §5).
func (w *window) updateInputMethod(focused Widget) {
	if w.activeIM != nil {
		w.imReset()
		w.activeIM.setWindow(nil)
		w.activeIM = nil
	}
	im := imContextOf(focused)
	if im == nil {
		w.imSetEnabled(false)
		return
	}
	w.activeIM = im
	im.setWindow(w)
	w.imSetEnabled(true)
	// The caret rect is pushed by the focused widget's next paint (setFocusedWidget
	// requests one), so the window need not read it back here.
}

func (w *window) imSetEnabled(enabled bool) {
	if w.inputMethod != nil {
		w.inputMethod.SetEnabled(enabled)
	}
}

func imContextOf(widget Widget) IMContext {
	client, ok := widget.(IMClient)
	if !ok {
		return nil
	}
	im := client.IMContext()
	if im == nil {
		return nil
	}
	return im
}

// imSetCaretRect is called by the active IMContext: it offsets the context's
// widget-local caret rect into window coordinates and forwards it to the platform.
func (w *window) imSetCaretRect(local geometry.Rectangle) {
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
func (w *window) imReset() {
	if w.inputMethod != nil {
		w.inputMethod.Reset()
	}
}

// applyCursor resolves the cursor shape from the dispatcher's hover path
// (innermost non-Default widget wins, falling back to CursorDefault) and
// applies it. Called when the hover path changes or a hovered widget changes
// its cursor. Redundant same-shape calls are skipped.
func (w *window) applyCursor() {
	if w.cursor == nil {
		return
	}
	var resolved Cursor = CursorDefault
	path := w.dispatcher.hoverPath
	for i := len(path) - 1; i >= 0; i-- {
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
	platformShape := platform.CursorShape(shape)
	if platformShape == w.lastCursor {
		return
	}
	w.lastCursor = platformShape
	w.cursor.SetShape(platformShape)
}

func visibleInTree(widget Widget) bool {
	for widget != nil {
		if !widget.Visible() {
			return false
		}
		widget = widget.Parent()
	}
	return true
}

func (w *window) desktopWindow() (platform.DesktopWindow, error) {
	if w.destroyed || w.platformWindow == nil {
		return nil, platform.ErrUnavailable
	}
	native, ok := w.platformWindow.(platform.DesktopWindow)
	if !ok {
		return nil, platform.ErrUnsupported
	}
	return native, nil
}

func (w *window) State() WindowState {
	native, err := w.desktopWindow()
	if err != nil {
		return WindowStateUnknown
	}
	return native.State()
}

// RequestState is best effort. Only actual native events change observations.
func (w *window) RequestState(state WindowState) {
	native, err := w.desktopWindow()
	if err != nil {
		return
	}
	if err := native.RequestState(state); err != nil && !errors.Is(err, platform.ErrUnsupported) && !errors.Is(err, platform.ErrUnavailable) {
		log.Printf("goui: request window state: %v", err)
	}
}

func (w *window) ConnectState(fn func(WindowState)) signal.Handle {
	if w.destroyed {
		return signal.Handles(nil)
	}
	return w.stateChanged.Connect(func(state WindowState) {
		if !w.destroyed {
			fn(state)
		}
	})
}
