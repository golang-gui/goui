package x11

import (
	"errors"
	"github.com/goexlib/cgo"
	"github.com/golang-gui/goui/platform/common"
	"github.com/golang-gui/goui/platform/events"
	"github.com/golang-gui/goui/platform/x11/libs/xlib"
)

type Window struct {
	wid     xlib.Window
	parent  common.Window
	onEvent events.EventHandler
	width   int32
	height  int32
	title   string
	gc      xlib.GC
}

func newWindow(onEvent events.EventHandler) (w common.Window, err error) {
	win := &Window{
		onEvent: onEvent,
	}

	screen := platform.defScreen

	attr := xlib.SetWindowAttributes{
		EventMask: xlib.EventMaskStructureNotify | xlib.EventMaskExposure | xlib.EventMaskPropertyChange,
	}

	win.wid = platform.display.CreateWindow(screen.Root,
		0, 0, 1600, 1200, 0,
		int(screen.RootDepth), xlib.WindowClassInputOutput, screen.RootVisual, xlib.CwEventMask, &attr)

	if win.wid == 0 {
		return nil, errors.New("create x11 window failed")
	}

	// declare WM protocols
	if platform.atoms.WM_PROTOCOLS != 0 {
		platform.display.SetWMProtocols(win.wid, []xlib.Atom{platform.atoms.WM_DELETE_WINDOW})
	}

	windowMap[win.wid] = win
	return win, nil
}

func (w *Window) NativeHandle() uintptr {
	return uintptr(w.wid)
}

func (w *Window) Destroy() {
	platform.display.DestroyWindow(w.wid)
}

func (w *Window) Parent() common.Window {
	return w.parent
}

func (w *Window) SetParent(parent common.Window) error {
	if parent != nil {
		w.parent = parent
		platform.display.SetTransientForHint(w.wid, xlib.Window(parent.NativeHandle()))
	} else {
		w.parent = nil
		platform.display.DeleteProperty(w.wid, xlib.AtomWmTransientFor)
	}
	return nil
}

func (w *Window) Title() string {
	return w.title
}

func (w *Window) SetTitle(title string) (err error) {
	if len(title) != 0 {
		w.title = title
		cTitle := cgo.CString(title)
		platform.display.ChangeProperty(w.wid, platform.atoms._NET_WM_NAME, platform.atoms.UTF8_STRING, 8,
			xlib.PropModeReplace, cTitle, len(title))
		platform.display.StoreName(w.wid, cgo.GoStringNTemp(cTitle, len(title)+1))
	} else {
		w.title = ""
		platform.display.DeleteProperty(w.wid, platform.atoms._NET_WM_NAME)
		platform.display.DeleteProperty(w.wid, xlib.AtomWmName)
	}
	return nil
}

func (w *Window) Show() error {
	platform.display.MapWindow(w.wid)
	platform.display.Flush()
	return nil
}

func (w *Window) Hide() error {
	platform.display.UnmapWindow(w.wid)
	platform.display.Flush()
	return nil
}

func (w *Window) Close() error {
	var event Event
	closeEvent := event.Event.ClientMessageEvent()
	closeEvent.Type = xlib.ClientMessage
	closeEvent.MessageType = platform.atoms.WM_PROTOCOLS
	closeEvent.L[0] = int64(platform.atoms.WM_DELETE_WINDOW)
	w.onEvent(&events.CloseEvent{
		WindowEventBase: events.WindowEventBase{
			Window: w,
			Native: &event,
		},
	})
	return nil
}

func (w *Window) Draw(img common.Image) error {
	return w.drawImage(common.ToBGRAImage(img))
}

func (w *Window) ScaleFactor() (float64, error) {
	panic("TODO impl")
}

var windowMap = map[xlib.Window]*Window{}

// TODO: process window event
func handleEvent(event xlib.Event) {
	nativeEvent := &Event{
		Event: event,
	}
	switch event.Type {
	case xlib.ClientMessage:
		ev := event.ClientMessageEvent()
		if ev.MessageType == platform.atoms.WM_PROTOCOLS && ev.L[0] != 0 {
			if xlib.Atom(ev.L[0]) == platform.atoms.WM_DELETE_WINDOW {
				if window, ok := windowMap[ev.Window]; ok {
					closeEvent := &events.CloseEvent{
						WindowEventBase: events.WindowEventBase{
							Window: window,
							Native: nativeEvent,
						},
					}
					window.onEvent(closeEvent)
				}
			}
		}
	// ping dnd
	case xlib.ConfigureNotify:
		ev := event.ConfigureEvent()
		if window, ok := windowMap[ev.Window]; ok {
			if ev.Width != window.width || ev.Height != window.height {
				window.width, window.height = ev.Width, ev.Height
				sizeEvent := &events.SizeEvent{
					WindowEventBase: events.WindowEventBase{
						Window: window,
						Native: nativeEvent,
					},
					Width:  uint(ev.Width),
					Height: uint(ev.Height),
				}
				window.onEvent(sizeEvent)
			}
		}

	case xlib.Expose:
		ev := event.ExposeEvent()
		if window, ok := windowMap[ev.Window]; ok {
			paintEvent := &events.PaintEvent{
				WindowEventBase: events.WindowEventBase{
					Window: window,
					Native: nativeEvent,
				},
			}
			window.onEvent(paintEvent)
		}
	case xlib.PropertyNotify:
		// state
	}
}

func (w *Window) drawImage(img *common.BGRAImage) (err error) {
	if w.gc == 0 {
		w.gc = platform.display.CreateGC(xlib.Drawable(w.wid), 0, nil)
		if w.gc == 0 {
			return errors.New("create GC failed")
		}
	}

	width, height := img.Bounds().Dx(), img.Bounds().Dy()

	image := platform.display.CreateImage(platform.defScreen.RootVisual, int(platform.defScreen.RootDepth), xlib.ImageFormatZPixmap, 0, cgo.CSlice(img.Pix), width, height, 32, img.Stride)
	if image == nil {
		return errors.New("create XImage failed")
	}
	defer image.Destroy()

	platform.display.PutImage(xlib.Drawable(w.wid), w.gc, image, 0, 0, 0, 0, width, height)
	image.Data = nil
	return nil
}
