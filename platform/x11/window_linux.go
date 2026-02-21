package x11

import (
	"github.com/golang-gui/goui/platform/common"
	"github.com/golang-gui/goui/platform/events"
	"github.com/golang-gui/goui/platform/x11/libx"
)

type Window struct {
	wid     libx.Window
	parent  common.Window
	onEvent events.EventHandler
	width   uint16
	height  uint16
	gc      libx.Gcontext
}

func newWindow(onEvent events.EventHandler) (w common.Window, err error) {
	win := &Window{
		onEvent: onEvent,
	}

	screen := platform.defScreen

	attr := libx.SetWindowAttributes{
		EventMask: libx.EventMaskStructureNotify | libx.EventMaskExposure | libx.EventMaskPropertyChange,
	}

	win.wid, err = libx.CreateWindow(platform.display, screen.Root,
		0, 0, 800, 600, 0,
		screen.RootDepth, libx.WindowClassInputOutput, screen.RootVisual, libx.CwEventMask, attr)

	if err != nil {
		return nil, err
	}

	// declare WM protocols
	if platform.atoms.WM_PROTOCOLS != 0 {
		libx.SetWMProtocols(platform.display, win.wid, []libx.Atom{platform.atoms.WM_DELETE_WINDOW})
	}

	windowMap[win.wid] = win
	return win, nil
}

func (w *Window) NativeHandle() uintptr {
	return uintptr(w.wid)
}

func (w *Window) Destroy() {
	libx.DestroyWindow(platform.display, w.wid)
}

func (w *Window) Parent() common.Window {
	panic("impl")
}

func (w *Window) SetParent(parent common.Window) error {
	panic("impl")
}

func (w *Window) Title() string {
	panic("impl")
}

func (w *Window) SetTitle(title string) (err error) {
	panic("impl")
}

func (w *Window) Show() error {
	return libx.MapWindow(platform.display, w.wid)
}

func (w *Window) Close() error {
	panic("impl")
}

func (w *Window) Draw(img common.Image) error {
	return w.drawImage(common.ToBGRAImage(img))
}

var windowMap = map[libx.Window]*Window{}

// TODO: process window event
func handleEvent(event libx.Event) {
	nativeEvent := &Event{
		Event: event,
	}
	switch ev := event.(type) {
	case libx.ClientMessageEvent:
		if ev.Type == platform.atoms.WM_PROTOCOLS && len(ev.Data.Data32) != 0 {
			if libx.Atom(ev.Data.Data32[0]) == platform.atoms.WM_DELETE_WINDOW {
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
	case libx.ConfigureNotifyEvent:
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

	case libx.ExposeEvent:
		if window, ok := windowMap[ev.Window]; ok {
			paintEvent := &events.PaintEvent{
				WindowEventBase: events.WindowEventBase{
					Window: window,
					Native: nativeEvent,
				},
			}
			window.onEvent(paintEvent)
		}
	case libx.PropertyNotifyEvent:
		// state
	}
}

func (w *Window) drawImage(img *common.BGRAImage) (err error) {
	if w.gc == 0 {
		w.gc, err = libx.CreateGC(platform.display, libx.Drawable(w.wid), 0, nil)
		if err != nil {
			return err
		}
	}

	//width, height := img.Bounds().Dx(), img.Bounds().Dy()
	//return libx.PutBGRAImage(platform.display, libx.Drawable(w.wid), w.gc, uint(width), uint(height), 0, 0, img.Pix)

	width := img.Bounds().Dx()
	data := img.Pix

	const MaxReqSize = (1 << 16) * 4
	rowsPer := (MaxReqSize - 28) / (width * 4)
	bytesPer := rowsPer * width * 4

	xpos := 0
	ypos := 0

	heightPer := 0
	start, end := 0, 0

	var toSend []byte

	for end < len(data) {
		end = start + bytesPer
		if end > len(data) {
			end = len(data)
		}

		toSend = data[start:end]
		heightPer = len(toSend) / 4 / width

		err = libx.PutBGRAImage(platform.display, libx.Drawable(w.wid), w.gc, uint(width), uint(heightPer), xpos, ypos, toSend)
		if err != nil {
			return err
		}

		start = end
		ypos += rowsPer
	}

	return nil
}
