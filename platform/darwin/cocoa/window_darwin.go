package cocoa

import (
	"fmt"
	"image"

	"github.com/golang-gui/goui/platform/common"
	"github.com/golang-gui/goui/platform/events"
	"github.com/golang-gui/goui/platform/graphics"

	. "github.com/golang-gui/goui/platform/darwin/frameworks/appkit"
	. "github.com/golang-gui/goui/platform/darwin/frameworks/core_foundation"
	. "github.com/golang-gui/goui/platform/darwin/frameworks/core_graphics"
	. "github.com/golang-gui/goui/platform/darwin/frameworks/foundation"
)

type Window struct {
	window   NSWindow
	delegate NSWindowDelegate
	view     NSView
	onEvent  events.EventHandler
	parent   common.Window
}

func newWindow(onEvent events.EventHandler) (w *Window, err error) {
	win := &Window{
		onEvent: onEvent,
	}
	AutoReleasePool(func() {
		win.delegate = delegateClass.Alloc()
		win.view = viewClass.Alloc().Init()

		styleMask := NSWindowStyleMaskMiniaturizable |
			NSWindowStyleMaskTitled |
			NSWindowStyleMaskClosable |
			NSWindowStyleMaskResizable

		win.window = windowClass.Alloc().InitWith(NSMakeRect(0, 0, 300, 200), styleMask,
			NSBackingStoreBuffered, false)

		win.window.SetCollectionBehavior(NSWindowCollectionBehaviorFullScreenPrimary | NSWindowCollectionBehaviorManaged)
		win.window.SetContentView(win.view)
		win.window.MakeFirstResponder(win.view.NSResponder)
		win.window.SetDelegate(win.delegate)
		win.window.SetAcceptsMouseMovedEvents(true)
		win.window.SetRestorable(false)
	})
	windowMap[win.window] = win
	win.sendCreatedEvents()
	return win, nil
}

func (w *Window) NativeHandle() uintptr {
	return uintptr(w.window.ID)
}

func (w *Window) Destroy() {
	AutoReleasePool(func() {
		w.window.OrderOut(0)
		w.window.SetDelegate(NSWindowDelegate{})
		w.delegate.Release()
		w.view.Release()
		w.window.Close()
	})
}

func (w *Window) Parent() common.Window {
	return w.parent
}

func (w *Window) SetParent(parent common.Window) error {
	AutoReleasePool(func() {
		if w.parent != parent {
			if w.parent != nil {
				var window NSWindow
				window.ID = ID(w.parent.NativeHandle())
				window.RemoveChildWindow(w.window)
			}
			if parent != nil {
				var window NSWindow
				window.ID = ID(parent.NativeHandle())
				window.AddChildWindow(w.window, NSWindowAbove)
			}
			w.parent = parent
		}
	})
	return nil
}

func (w *Window) Title() (v string) {
	AutoReleasePool(func() {
		v = w.window.Title()
	})
	return
}

func (w *Window) SetTitle(title string) (err error) {
	AutoReleasePool(func() {
		w.window.SetTitle(title)
	})
	return nil
}

func (w *Window) Show() error {
	AutoReleasePool(func() {
		w.window.OrderFront(0)
	})
	return nil
}

func (w *Window) Close() error {
	AutoReleasePool(func() {
		w.window.Close()
	})
	return nil
}

func (w *Window) ScaleFactor() (float64, error) {
	return w.window.BackingScaleFactor(), nil
}

func (w *Window) Draw(img image.Image) error {
	bmp, ok := graphics.ToBitmap(img, graphics.PixelFormatRGBA)
	if !ok {
		bmp = graphics.CopyToBitmap(img, graphics.PixelFormatRGBA, nil)
	}
	return w.drawImage(bmp)
}

var (
	windowClass   NSWindowClass
	delegateClass NSWindowDelegateClass
	viewClass     NSViewClass
	windowMap     = map[NSWindow]*Window{}
)

func initWindowClass() (err error) {
	windowClass, err = ImplementNSWindow("GouiWindow", NSWindowOverride{
		CanBecomeKeyWindow: func(self NSWindow) bool {
			return true
		},
		CanBecomeMainWindow: func(self NSWindow) bool {
			return true
		},
	})
	if err != nil {
		return fmt.Errorf("implement NSWindow err: %v", err)
	}

	delegateClass, err = ImplementNSWindowDelegate("GouiWindowDelegate", NSWindowDelegateOverride{
		WindowShouldClose: windowShouldClose,
		WindowDidResize:   windowDidResize,
	})
	if err != nil {
		return fmt.Errorf("implement NSWindowDelegate err: %v", err)
	}

	viewClass, err = ImplementNSView("GouiContentView", NSViewOverride{
		CanBecomeKeyView: func(self NSView) bool {
			return true
		},
		AcceptsFirstResponder: func(self NSView) bool {
			return true
		},
		ViewDidChangeBackingProperties: viewDidChangeBackingProperties,
		DrawRect:                       drawRect,
	})
	if err != nil {
		return fmt.Errorf("implement NSView err: %v", err)
	}

	return
}

func windowShouldClose(self NSWindowDelegate, sender NSWindow) bool {
	if window, has := windowMap[sender]; has {
		closeEvent := &events.CloseEvent{
			WindowEventBase: events.WindowEventBase{
				Window: window,
			},
		}
		window.onEvent(closeEvent)
		return closeEvent.Accepted()
	}
	return true // will close
}

func windowDidResize(self NSWindowDelegate, notification NSNotification) {
	if window, has := windowMap[Cast[NSWindow](notification.Object())]; has {
		rect := window.view.Frame()
		sizeEvent := &events.SizeEvent{
			WindowEventBase: events.WindowEventBase{
				Window: window,
			},
			Width:  uint(rect.Size.Width),
			Height: uint(rect.Size.Height),
		}
		window.onEvent(sizeEvent)
	}
}

func viewDidChangeBackingProperties(self NSView) {
	if window, has := windowMap[self.Window()]; has {
		rect := self.Frame()
		fbRect := self.ConvertRectToBacking(rect)
		scaleFactor := fbRect.Size.Width / rect.Size.Width

		scaleEvent := &events.ScaleEvent{
			WindowEventBase: events.WindowEventBase{
				Window: window,
			},
			ScaleFactor: scaleFactor,
		}
		window.onEvent(scaleEvent)
	}
}

func drawRect(self NSView, rect NSRect) {
	if window, has := windowMap[self.Window()]; has {
		paintEvent := &events.PaintEvent{
			WindowEventBase: events.WindowEventBase{
				Window: window,
			},
		}
		window.onEvent(paintEvent)
	}
}

func (w *Window) sendCreatedEvents() {
	rect := w.view.Frame()
	fbRect := w.view.ConvertRectToBacking(rect)

	scaleFactor := fbRect.Size.Width / rect.Size.Width
	scaleEvent := &events.ScaleEvent{
		WindowEventBase: events.WindowEventBase{
			Window: w,
		},
		ScaleFactor: scaleFactor,
	}
	w.onEvent(scaleEvent)

	sizeEvent := &events.SizeEvent{
		WindowEventBase: events.WindowEventBase{
			Window: w,
		},
		Width:  uint(rect.Size.Width),
		Height: uint(rect.Size.Height),
	}
	w.onEvent(sizeEvent)
}

func (w *Window) drawImage(img graphics.Bitmap) (err error) {
	if width, height := img.Width, img.Height; width != 0 && height != 0 {
		context := NSGraphicsContextClassId.CurrentContext().CGContext()

		data := CFDataCreate(0, img.Pixels)
		defer CFRelease(data)

		dataProvider := CGDataProviderCreateWithCFData(data)
		defer CGDataProviderRelease(dataProvider)

		colorSpace := CGColorSpaceCreateDeviceRGB()
		defer CGColorSpaceRelease(colorSpace)

		bitmapInfo := CGImageAlphaLast
		cgImage := CGImageCreate(uint(width), uint(height), 8, 32, uint(img.Stride), colorSpace, bitmapInfo, dataProvider, nil, false, CGRenderingIntentDefault)
		if cgImage != 0 {
			defer CGImageRelease(cgImage)
			CGContextDrawImage(context, NSMakeRect(0, 0, CGFloat(width), CGFloat(height)), cgImage)
		}
	}
	return nil
}
