package gui

import (
	"fmt"

	"github.com/golang-gui/goui/core/geometry"
	"github.com/golang-gui/goui/core/signal"
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

	RequestClose() error
	Destroy()

	Snapshot() WindowInfo
	DispatchEvent(event events.Event) error

	ConnectCloseRequest(func(*bool)) signal.Handle
	ConnectDestroy(func()) signal.Handle
	ConnectFocusChanged(func(bool)) signal.Handle
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
	destroyed      bool
	popover        *popover // the one open popover this window hosts (v1: single)
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
	return win, nil
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
			if win, ok := oldRoot.(*window); ok && win.focusWithinWidget(widget) {
				win.SetFocusedWidget(nil)
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
	if w.forwardToPopover(event) {
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

// setPopover records the window's open popover, superseding any previous one.
func (w *window) setPopover(p *popover) {
	if w.popover != nil && w.popover != p {
		w.popover.dismissRequest.Emit()
	}
	w.popover = p
}

func (w *window) clearPopover(p *popover) {
	if w.popover == p {
		w.popover = nil
	}
}

// forwardToPopover routes the window's own input to its open popover (§7): the
// popover has no native focus, so keyboard is forwarded here; an outside click
// (the owner only ever receives clicks outside the popover) / Esc / focus loss
// requests dismissal. Returns true when it consumes the event.
func (w *window) forwardToPopover(event events.Event) bool {
	if w.popover == nil || !w.popover.visible {
		return false
	}
	switch e := event.(type) {
	case events.KeyEvent:
		if e.EventType == events.KeyDown && e.Key == events.KeyEscape {
			w.popover.dismissRequest.Emit()
			return true
		}
		_ = w.popover.dispatcher.DispatchEvent(w.popover, event)
		return true
	case events.PointerEvent:
		if e.EventType == events.PointerDown {
			w.popover.dismissRequest.Emit()
		}
		return true // swallow the window's own pointer while a popover is open
	case events.FocusEvent:
		if !e.Focused {
			w.popover.dismissRequest.Emit()
		}
		return false // window still handles its own focus normally
	}
	return false
}

func (w *window) paint() {
	if w.painter == nil || w.root == nil {
		return
	}

	size := geometry.Size{
		Width:  w.width,
		Height: w.height,
	}
	if w.layoutDirty {
		w.root.Measure(size)
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
	_ = w.dispatcher.DispatchEvent(w, events.FocusEvent{Focused: w.focused})
	w.requestPaint()
}

func (w *window) focusWithinWidget(widget Widget) bool {
	return widgetContains(w.focusedWidget, widget)
}

func (w *window) focusWithinBase(base *WidgetBase) bool {
	for widget := w.focusedWidget; widget != nil; widget = widget.Parent() {
		if widget.base() == base {
			return true
		}
	}
	return false
}

func (w *window) focusIsBase(base *WidgetBase) bool {
	return w.focusedWidget != nil && w.focusedWidget.base() == base
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

func widgetContains(widget, ancestor Widget) bool {
	for widget != nil {
		if widget == ancestor {
			return true
		}
		widget = widget.Parent()
	}
	return false
}
