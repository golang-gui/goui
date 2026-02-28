package x11

import (
	"sync"

	"github.com/golang-gui/goui/platform/common"
	"github.com/golang-gui/goui/platform/events"

	"github.com/jezek/xgb/xproto"
)

type Window struct {
	wid     xproto.Window
	parent  common.Window
	onEvent events.EventHandler
}

func newWindow(onEvent events.EventHandler) (w common.Window, err error) {
	win := &Window{
		onEvent: onEvent,
	}

	win.wid, err = xproto.NewWindowId(platform.xConn)
	if err != nil {
		return nil, err
	}

	screen := platform.setup.DefaultScreen(platform.xConn)
	xproto.CreateWindow(platform.xConn, screen.RootDepth, win.wid, screen.Root, 0, 0, 800, 600, 0, xproto.WindowClassInputOutput, screen.RootVisual, 0, []uint32{})

	return win, nil
}

func (w *Window) NativeHandle() uintptr {
	return uintptr(w.wid)
}

func (w *Window) Destroy() {
	xproto.DestroyWindow(platform.xConn, w.wid)
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
	return xproto.MapWindowChecked(platform.xConn, w.wid).Check()
}

func (w *Window) Close() error {
	panic("impl")
}

var (
	initOnce  sync.Once
	windowMap map[xproto.Window]*Window
)

// TODO: process window event
