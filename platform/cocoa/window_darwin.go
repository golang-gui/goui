package cocoa

import (
	"fmt"
	"image"

	"github.com/golang-gui/goui/platform/common"
	"github.com/golang-gui/goui/platform/events"
	"github.com/golang-gui/goui/platform/graphics"

	"github.com/golang-gui/goui/platform/cocoa/frameworks/appkit"
	"github.com/golang-gui/goui/platform/cocoa/frameworks/core_foundation"
	"github.com/golang-gui/goui/platform/cocoa/frameworks/core_graphics"
	"github.com/golang-gui/goui/platform/cocoa/frameworks/foundation"
)

type Window struct {
	window   appkit.NSWindow
	delegate appkit.NSWindowDelegate
	view     appkit.NSView
	onEvent  events.EventHandler
	parent   common.Window
}

func newWindow(onEvent events.EventHandler) (w *Window, err error) {
	win := &Window{
		onEvent: onEvent,
	}
	foundation.AutoReleasePool(func() {
		win.delegate = delegateClass.Alloc()
		win.view = viewClass.Alloc().Init()

		styleMask := appkit.NSWindowStyleMaskMiniaturizable |
			appkit.NSWindowStyleMaskTitled |
			appkit.NSWindowStyleMaskClosable |
			appkit.NSWindowStyleMaskResizable

		win.window = windowClass.Alloc().InitWith(foundation.NSMakeRect(0, 0, 300, 200), styleMask,
			appkit.NSBackingStoreBuffered, false)

		win.window.SetCollectionBehavior(appkit.NSWindowCollectionBehaviorFullScreenPrimary | appkit.NSWindowCollectionBehaviorManaged)
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
	foundation.AutoReleasePool(func() {
		w.window.OrderOut(0)
		w.window.SetDelegate(appkit.NSWindowDelegate{})
		w.delegate.Release()
		w.view.Release()
		w.window.Close()
	})
}

func (w *Window) Parent() common.Window {
	return w.parent
}

func (w *Window) SetParent(parent common.Window) error {
	foundation.AutoReleasePool(func() {
		if w.parent != parent {
			if w.parent != nil {
				var window appkit.NSWindow
				window.ID = foundation.ID(w.parent.NativeHandle())
				window.RemoveChildWindow(w.window)
			}
			if parent != nil {
				var window appkit.NSWindow
				window.ID = foundation.ID(parent.NativeHandle())
				window.AddChildWindow(w.window, appkit.NSWindowAbove)
			}
			w.parent = parent
		}
	})
	return nil
}

func (w *Window) Title() (v string) {
	foundation.AutoReleasePool(func() {
		v = w.window.Title()
	})
	return
}

func (w *Window) SetTitle(title string) (err error) {
	foundation.AutoReleasePool(func() {
		w.window.SetTitle(title)
	})
	return nil
}

func (w *Window) Show() error {
	foundation.AutoReleasePool(func() {
		w.window.OrderFront(0)
	})
	return nil
}

func (w *Window) Close() error {
	foundation.AutoReleasePool(func() {
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
	windowClass   appkit.NSWindowClass
	delegateClass appkit.NSWindowDelegateClass
	viewClass     appkit.NSViewClass
	windowMap     = map[appkit.NSWindow]*Window{}
)

func initWindowClass() (err error) {
	windowClass, err = appkit.ImplementNSWindow("GouiWindow", appkit.NSWindowOverride{
		CanBecomeKeyWindow: func(self appkit.NSWindow) bool {
			return true
		},
		CanBecomeMainWindow: func(self appkit.NSWindow) bool {
			return true
		},
	})
	if err != nil {
		return fmt.Errorf("implement NSWindow err: %v", err)
	}

	delegateClass, err = appkit.ImplementNSWindowDelegate("GouiWindowDelegate", appkit.NSWindowDelegateOverride{
		WindowShouldClose: windowShouldClose,
		WindowDidResize:   windowDidResize,
	})
	if err != nil {
		return fmt.Errorf("implement NSWindowDelegate err: %v", err)
	}

	viewClass, err = appkit.ImplementNSView("GouiContentView", appkit.NSViewOverride{
		CanBecomeKeyView: func(self appkit.NSView) bool {
			return true
		},
		AcceptsFirstResponder: func(self appkit.NSView) bool {
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

func windowShouldClose(self appkit.NSWindowDelegate, sender appkit.NSWindow) bool {
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

func windowDidResize(self appkit.NSWindowDelegate, notification foundation.NSNotification) {
	if window, has := windowMap[foundation.Cast[appkit.NSWindow](notification.Object())]; has {
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

func viewDidChangeBackingProperties(self appkit.NSView) {
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

func drawRect(self appkit.NSView, rect foundation.NSRect) {
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
		context := appkit.NSGraphicsContextClassId.CurrentContext().CGContext()

		data := core_foundation.CFDataCreate(0, img.Pixels)
		defer core_foundation.CFRelease(data)

		dataProvider := core_graphics.CGDataProviderCreateWithCFData(data)
		defer core_graphics.CGDataProviderRelease(dataProvider)

		colorSpace := core_graphics.CGColorSpaceCreateDeviceRGB()
		defer core_graphics.CGColorSpaceRelease(colorSpace)

		bitmapInfo := core_graphics.CGImageAlphaLast
		cgImage := core_graphics.CGImageCreate(uint(width), uint(height), 8, 32, uint(img.Stride), colorSpace, bitmapInfo, dataProvider, nil, false, core_graphics.CGRenderingIntentDefault)
		if cgImage != 0 {
			defer core_graphics.CGImageRelease(cgImage)
			core_graphics.CGContextDrawImage(context, foundation.NSMakeRect(0, 0, core_graphics.CGFloat(width), core_graphics.CGFloat(height)), cgImage)
		}
	}
	return nil
}
