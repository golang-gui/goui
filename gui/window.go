package gui

import (
	"fmt"

	"github.com/golang-gui/goui/core/geometry"
	"github.com/golang-gui/goui/core/signal"
	"github.com/golang-gui/goui/layout"
	"github.com/golang-gui/goui/platform"
	"github.com/golang-gui/goui/platform/events"
	"github.com/golang-gui/goui/platform/graphics"
)

type Window interface {
	Root

	ID() string
	SetID(string)
	SetWidget(Widget)

	Title() string
	SetTitle(string) error

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

	Snapshot() WindowInfo
	DispatchEvent(event events.Event) error

	ConnectCloseRequest(func(*bool)) signal.Handle
	ConnectDestroy(func()) signal.Handle
	ConnectFocusChanged(func(bool)) signal.Handle
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

type window struct {
	app            *application
	id             string
	title          string
	platformWindow platform.Window
	painter        graphics.Painter
	root           Widget
	dispatcher     EventDispatcher
	width          float32 // logical (DIP)
	height         float32 // logical (DIP)
	pixelWidth     float32 // physical (backing) pixels
	pixelHeight    float32 // physical (backing) pixels
	layoutDirty    bool
	paintDirty     bool
	focused        bool
	focusedWidget  Widget
	activeIM       IMContext            // the focused text widget's context bound to the native IME; nil when none
	inputMethod    platform.InputMethod // this window's platform IME; nil when the platform has none
	destroyed      bool
	modalTarget    ModalTarget // the modal element intercepting this window's input; nil when none
	closeRequest   signal.Signal1[*bool]
	destroy        signal.Signal0
	focusChanged   signal.Signal1[bool]
}

// defaultWindowWidth/Height is the preferred initial size (logical/DIP) passed
// to the platform as a hint. TODO(layout): derive from the widget tree / let the
// caller specify once the layout system threads sizing.
const (
	defaultWindowWidth  = 800
	defaultWindowHeight = 600
)

func newWindow(app *application) (*window, error) {
	win := &window{
		app:         app,
		layoutDirty: true,
		paintDirty:  true,
	}

	platformWindow, err := app.platform.NewWindow(defaultWindowWidth, defaultWindowHeight, win.onEvent)
	if err != nil {
		return nil, fmt.Errorf("create platform window: %w", err)
	}
	win.platformWindow = platformWindow

	win.painter, err = app.platform.NewPainter(platformWindow, app.typo)
	if err != nil {
		platformWindow.Destroy()
		win.platformWindow = nil
		return nil, fmt.Errorf("create painter: %w", err)
	}

	win.title = platformWindow.Title()

	// Text input (IME) is an optional platform capability; a nil result (the
	// platform has no input method) just means text widgets fall back to plain
	// key events. Commit/preedit are routed to the focused widget's IMContext.
	win.inputMethod, _ = app.platform.NewInputMethod(platformWindow, win.onInputMethod)

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

func (w *window) FocusedWidget() Widget {
	return w.focusedWidget
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
		oldRoot := widget.Root()
		if oldRoot != nil {
			widget.base().emitUnmountSubtree(widget)
			if h, ok := oldRoot.(EventTarget); ok && focusWithin(h, widget.base()) {
				h.SetFocusedWidget(nil)
			}
		}
		widget.base().detach(widget)
		w.root = widget
		widget.base().attachRoot(w, widget)
	}
	w.requestLayout()
}

func (w *window) Show() error {
	if w.platformWindow == nil {
		return nil
	}
	return w.platformWindow.Show()
}

func (w *window) RequestPaint() error {
	if w.platformWindow == nil {
		return nil
	}
	w.paintDirty = true
	return w.platformWindow.RequestPaint()
}

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
	w.destroy.Emit()

	if w.root != nil {
		root := w.root
		root.base().detachRoot(root)
		root.base().destroy(root)
	}
	if w.inputMethod != nil {
		w.inputMethod.Destroy() // release the native input context before the window
		w.inputMethod = nil
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
	return info
}

func (w *window) DispatchEvent(event events.Event) error {
	if w.routeToModalTarget(event) {
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
	case events.FocusEvent:
		w.setFocused(event.Focused)
		return w.dispatcher.DispatchEvent(w, event)
	case events.PaintEvent:
		w.paint()
	default:
		return w.dispatcher.DispatchEvent(w, event)
	}
	return nil
}

func (w *window) ConnectCloseRequest(fn func(*bool)) signal.Handle {
	return w.closeRequest.Connect(fn)
}

func (w *window) ConnectDestroy(fn func()) signal.Handle {
	return w.destroy.Connect(fn)
}

func (w *window) ConnectFocusChanged(fn func(bool)) signal.Handle {
	return w.focusChanged.Connect(fn)
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
			w.modalTarget.RequestDismiss() // the owner only ever sees clicks outside the target
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
	if w.painter == nil || w.root == nil {
		return
	}

	size := geometry.Size{
		Width:  w.width,
		Height: w.height,
	}
	if w.layoutDirty {
		w.root.Measure(layout.Tight(size)) // Window is extrinsic: root fills the window.
		w.root.Arrange(geometry.Rect(0, 0, size.Width, size.Height))
		w.layoutDirty = false
	}

	// Begin takes the physical (backing) pixel size; scale = physical / logical.
	pixelWidth, pixelHeight := w.pixelWidth, w.pixelHeight
	scale := float32(1)
	if w.width > 0 && w.pixelWidth > 0 {
		scale = w.pixelWidth / w.width
	} else {
		pixelWidth, pixelHeight = size.Width, size.Height
	}

	w.painter.Begin(pixelWidth, pixelHeight, scale)
	w.painter.Clear(graphics.RGB(255, 255, 255))
	w.root.Paint(SubPainter(w.painter, w.root.Rect()))
	w.painter.End()
	w.paintDirty = false
}

// PlatformWindow is the escape hatch to the underlying platform window.
func (w *window) PlatformWindow() platform.Window { return w.platformWindow }

// RequestLayout satisfies Root: schedule a relayout of this host.
func (w *window) RequestLayout() { w.requestLayout() }

func (w *window) requestLayout() {
	w.layoutDirty = true
	w.requestPaint()
}

func (w *window) requestPaint() {
	w.paintDirty = true
	if w.platformWindow != nil {
		_ = w.platformWindow.RequestPaint()
	}
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

func visibleInTree(widget Widget) bool {
	for widget != nil {
		if !widget.Visible() {
			return false
		}
		widget = widget.Parent()
	}
	return true
}
