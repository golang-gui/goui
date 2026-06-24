package x11

import (
	"errors"
	"image"

	"github.com/golang-gui/goui/platform/common"
	"github.com/golang-gui/goui/platform/events"
	"github.com/golang-gui/goui/platform/graphics"
	"github.com/golang-gui/goui/platform/graphics/opengl"

	"github.com/golang-gui/goui/platform/linux/libs/glx"
	"github.com/golang-gui/goui/platform/linux/libs/xlib"

	"github.com/goexlib/cgo"
)

type Window struct {
	wid     xlib.Window
	fb      glx.FBConfig
	cmap    xlib.Colormap
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

	visual := platform.defScreen.RootVisual
	depth := int(platform.defScreen.RootDepth)

	fbConfig := opengl.FBConfig{
		PixelFormat: opengl.DefaultConfig.PixelFormat,
	}

	if fb, err := opengl.ChooseGLXFBConfig(fbConfig); err == nil {
		vi := glx.GetVisualFromFBConfig(platform.display, fb)
		if vi != nil {
			defer xlib.Free(vi)

			win.fb = fb
			visual = vi.Visual
			depth = int(vi.Depth)
		}
		// TODO: add error log
	}

	win.cmap = platform.display.CreateColormap(platform.defScreen.Root, visual, xlib.ColormapAllocNone)

	attr := xlib.SetWindowAttributes{
		Colormap:  win.cmap,
		EventMask: xlib.EventMaskStructureNotify | xlib.EventMaskExposure | xlib.EventMaskPropertyChange,
	}

	win.wid = platform.display.CreateWindow(platform.defScreen.Root,
		0, 0, 800, 600, 0,
		depth, xlib.WindowClassInputOutput, visual, xlib.CwColormap|xlib.CwEventMask, &attr)

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

func (w *Window) NativeFBConfig() glx.FBConfig {
	return w.fb
}

func (w *Window) Destroy() {
	if w.wid == 0 {
		return
	}

	delete(windowMap, w.wid)
	if w.gc != 0 {
		platform.display.FreeGC(w.gc)
		w.gc = 0
	}
	platform.display.DestroyWindow(w.wid)
	if w.cmap != 0 {
		platform.display.FreeColormap(w.cmap)
		w.cmap = 0
	}
	platform.display.Flush()
	w.wid = 0
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

func (w *Window) RequestClose() error {
	if w.wid == 0 {
		return nil
	}
	w.onEvent(events.CloseEvent{})
	return nil
}

func (w *Window) Draw(img image.Image) error {
	bmp, ok := graphics.ToBitmap(img, graphics.PixelFormatBGRA)
	if !ok {
		bmp = graphics.CopyToBitmap(img, graphics.PixelFormatBGRA, nil)
	}
	return w.drawImage(bmp)
}

func (w *Window) ScaleFactor() (float64, error) {
	panic("TODO impl")
}

var windowMap = map[xlib.Window]*Window{}

// TODO: process window event
func handleEvent(event xlib.Event) {
	switch event.Type {
	case xlib.ClientMessage:
		ev := event.ClientMessageEvent()
		if ev.MessageType == platform.atoms.WM_PROTOCOLS && ev.L[0] != 0 {
			if xlib.Atom(ev.L[0]) == platform.atoms.WM_DELETE_WINDOW {
				if window, ok := windowMap[ev.Window]; ok {
					window.onEvent(events.CloseEvent{})
				}
			}
		}
	// ping dnd
	case xlib.ConfigureNotify:
		ev := event.ConfigureEvent()
		if window, ok := windowMap[ev.Window]; ok {
			if ev.Width != window.width || ev.Height != window.height {
				window.width, window.height = ev.Width, ev.Height
				window.onEvent(events.SizeEvent{
					Width:  uint(ev.Width),
					Height: uint(ev.Height),
				})
			}
		}

	case xlib.Expose:
		ev := event.ExposeEvent()
		if window, ok := windowMap[ev.Window]; ok {
			window.onEvent(events.PaintEvent{})
		}
	case xlib.PropertyNotify:
		// state
	}
}

func (w *Window) drawImage(img graphics.Bitmap) (err error) {
	if w.gc == 0 {
		w.gc = platform.display.CreateGC(xlib.Drawable(w.wid), 0, nil)
		if w.gc == 0 {
			return errors.New("create GC failed")
		}
	}

	width, height := img.Bounds().Dx(), img.Bounds().Dy()

	image := platform.display.CreateImage(platform.defScreen.RootVisual, int(platform.defScreen.RootDepth), xlib.ImageFormatZPixmap, 0, cgo.CSlice(img.Pixels), width, height, 32, img.Stride)
	if image == nil {
		return errors.New("create XImage failed")
	}
	defer image.Destroy()

	platform.display.PutImage(xlib.Drawable(w.wid), w.gc, image, 0, 0, 0, 0, width, height)
	image.Data = nil
	return nil
}
