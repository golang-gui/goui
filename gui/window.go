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
	ID() string
	SetID(string)

	Title() string
	SetTitle(string) error

	Widget() Widget
	SetWidget(Widget)

	Show() error
	RequestPaint() error

	RequestClose() error
	Destroy()

	Dispatcher() *EventDispatcher
	Snapshot() WindowInfo
	DispatchEvent(event events.Event) error

	ConnectCloseRequest(func(*bool)) signal.Handle
	ConnectDestroy(func()) signal.Handle
}

type window struct {
	app            *application
	id             string
	title          string
	platformWindow platform.Window
	painter        graphics.Painter
	root           Widget
	dispatcher     EventDispatcher
	width          uint
	height         uint
	scale          float64
	layoutDirty    bool
	paintDirty     bool
	destroyed      bool
	closeRequest   signal.Signal1[*bool]
	destroy        signal.Signal0
}

func newWindow(app *application) (*window, error) {
	win := &window{
		app:         app,
		scale:       1,
		layoutDirty: true,
		paintDirty:  true,
	}

	platformWindow, err := app.platform.NewWindow(win.onEvent)
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

func (w *window) Widget() Widget {
	return w.root
}

func (w *window) SetWidget(widget Widget) {
	if w.root == widget {
		return
	}
	if w.root != nil {
		w.root.Base().setWindow(nil)
	}
	w.root = widget
	if w.root != nil {
		w.root.Base().parent = nil
		w.root.Base().setWindow(w)
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
		w.root.Base().setWindow(nil)
		w.root = nil
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

func (w *window) Dispatcher() *EventDispatcher {
	return &w.dispatcher
}

func (w *window) Snapshot() WindowInfo {
	info := WindowInfo{
		ID:    w.ID(),
		Title: w.Title(),
		Bounds: geometry.Rect(
			0,
			0,
			float32(w.width),
			float32(w.height),
		),
	}
	if w.root != nil {
		info.Widget = w.root.Snapshot()
	}
	return info
}

func (w *window) DispatchEvent(event events.Event) error {
	return w.dispatcher.DispatchEvent(w, event)
}

func (w *window) ConnectCloseRequest(fn func(*bool)) signal.Handle {
	return w.closeRequest.Connect(fn)
}

func (w *window) ConnectDestroy(fn func()) signal.Handle {
	return w.destroy.Connect(fn)
}

func (w *window) onEvent(event events.Event) {
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
		w.requestLayout()
	case events.ScaleEvent:
		w.scale = event.ScaleFactor
		if w.scale == 0 {
			w.scale = 1
		}
		w.requestLayout()
	case events.PaintEvent:
		w.paint()
	default:
		_ = w.DispatchEvent(event)
	}
}

func (w *window) paint() {
	if w.painter == nil || w.root == nil {
		return
	}

	size := geometry.Size{
		Width:  float32(w.width),
		Height: float32(w.height),
	}
	if w.layoutDirty {
		w.root.Measure(size)
		w.root.Arrange(geometry.Rect(0, 0, size.Width, size.Height))
		w.layoutDirty = false
	}

	scale := float32(w.scale)
	if scale == 0 {
		scale = 1
	}

	w.painter.Begin(size.Width, size.Height, scale)
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
