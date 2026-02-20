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
}

func newWindow(onEvent events.EventHandler) (w common.Window, err error) {
	win := &Window{
		onEvent: onEvent,
	}

	defScreen := libx.DefaultScreen(platform.display)
	screen := libx.ScreenOfDisplay(platform.display, defScreen)
	visual := libx.DefaultVisual(platform.display, defScreen)

	attr := libx.SetWindowAttributes{
		EventMask: libx.EventMaskStructureNotify | libx.EventMaskExposure | libx.EventMaskPropertyChange,
	}

	win.wid, err = libx.CreateWindow(platform.display, screen.Root,
		0, 0, 800, 600, 0,
		screen.RootDepth, libx.WindowClassInputOutput, visual, libx.CwEventMask, attr)

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
	panic("impl")
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
					Width:  int(ev.Width),
					Height: int(ev.Height),
				}
				window.onEvent(sizeEvent)
			}
		}

	case libx.ExposeEvent:
		// paint
	case libx.PropertyNotifyEvent:
		// state
	}
}
