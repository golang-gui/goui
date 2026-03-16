package cocoa

import (
	"fmt"
	"github.com/golang-gui/goui/platform/cocoa/frameworks/appkit"
	"github.com/golang-gui/goui/platform/cocoa/frameworks/core_foundation"
	"github.com/golang-gui/goui/platform/cocoa/frameworks/core_graphics"
	"github.com/golang-gui/goui/platform/cocoa/frameworks/foundation"
	"github.com/golang-gui/goui/platform/cocoa/frameworks/objcrt"
	"github.com/golang-gui/goui/platform/common"
	"github.com/golang-gui/goui/platform/events"
)

type Window struct {
	window   appkit.NSWindow
	delegate appkit.NSWindowDelegate
	view     appkit.NSView
	onEvent  events.EventHandler
}

func newWindow(onEvent events.EventHandler) (w *Window, err error) {
	win := &Window{
		onEvent: onEvent,
	}
	foundation.AutoReleasePool(func() {
		win.delegate = delegateClass.Alloc()
		win.view = viewClass.Alloc().Init()
		win.window = windowClass.Alloc().InitWith(foundation.NSMakeRect(0, 0, 300, 200),
			appkit.NSWindowStyleMaskTitled|appkit.NSWindowStyleMaskClosable|appkit.NSWindowStyleMaskResizable,
			appkit.NSBackingStoreBuffered, false)
		win.window.SetDelegate(win.delegate)
		win.window.SetContentView(win.view)
		win.window.MakeFirstResponder(appkit.NSResponder(win.view))
	})
	windowMap[win.window] = win
	win.sendCreatedEvents()
	return win, nil
}

func (w *Window) NativeHandle() uintptr {
	return uintptr(w.window)
}

func (w *Window) Destroy() {
	foundation.AutoReleasePool(func() {
		w.window.OrderOut(objcrt.Nil)
		w.window.SetDelegate(objcrt.Nil)
		objcrt.NSObject(w.delegate).Release()
		objcrt.NSObject(w.view).Release()
		w.window.Close()
	})
}

func (w *Window) Parent() common.Window {
	panic("impl")
}

func (w *Window) SetParent(parent common.Window) error {
	panic("impl")
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
	panic("impl")
}

func (w *Window) ScaleFactor() (float64, error) {
	panic("impl")
}

func (w *Window) Draw(img common.Image) error {
	return w.drawImage(img.(*common.RGBAImage))
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
	if window, has := windowMap[appkit.NSWindow(notification.Object())]; has {
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

func (w *Window) drawImage(img *common.RGBAImage) (err error) {
	if width, height := img.Rect.Dx(), img.Rect.Dy(); width != 0 && height != 0 {
		context := appkit.NSGraphicsContextClassId.CurrentContext().CGContext()

		data := core_foundation.CFDataCreate(0, img.Pix)
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
