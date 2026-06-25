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
	window       NSWindow
	delegate     NSWindowDelegate
	view         NSView
	trackingArea NSTrackingArea
	onEvent      events.EventHandler
	parent       common.Window
	buttons      events.PointerButtons
	modifiers    events.Modifiers
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
	win.updateTrackingArea()
	win.sendCreatedEvents()
	return win, nil
}

func (w *Window) NativeHandle() uintptr {
	return uintptr(w.window.ID)
}

func (w *Window) Destroy() {
	if !w.window.Valid() {
		return
	}

	AutoReleasePool(func() {
		delete(windowMap, w.window)
		if w.trackingArea.Valid() {
			w.view.RemoveTrackingArea(w.trackingArea)
			w.trackingArea = NSTrackingArea{}
		}
		w.window.OrderOut(0)
		w.window.SetDelegate(NSWindowDelegate{})
		w.delegate.Release()
		w.view.Release()
		w.window.Close()
		w.window = NSWindow{}
		w.delegate = NSWindowDelegate{}
		w.view = NSView{}
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
		NSApp.ActivateIgnoringOtherApps(true)
		w.window.MakeKeyAndOrderFront(0)
		w.window.MakeFirstResponder(w.view.NSResponder)
	})
	return nil
}

func (w *Window) RequestClose() error {
	if !w.window.Valid() {
		return nil
	}

	AutoReleasePool(func() {
		w.window.PerformClose(0)
	})
	return nil
}

func (w *Window) RequestPaint() error {
	if !w.window.Valid() {
		return nil
	}

	AutoReleasePool(func() {
		w.view.SetNeedsDisplay(true)
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
		WindowShouldClose:  windowShouldClose,
		WindowDidResize:    windowDidResize,
		WindowDidBecomeKey: windowDidBecomeKey,
		WindowDidResignKey: windowDidResignKey,
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
		UpdateTrackingAreas:            updateTrackingAreas,
		MouseEntered:                   mouseEntered,
		MouseExited:                    mouseExited,
		MouseMoved:                     mouseMoved,
		MouseDragged:                   mouseDragged,
		MouseDown:                      mouseDown,
		MouseUp:                        mouseUp,
		RightMouseDown:                 rightMouseDown,
		RightMouseUp:                   rightMouseUp,
		RightMouseDragged:              rightMouseDragged,
		OtherMouseDown:                 otherMouseDown,
		OtherMouseUp:                   otherMouseUp,
		OtherMouseDragged:              otherMouseDragged,
		ScrollWheel:                    scrollWheel,
		KeyDown:                        keyDown,
		KeyUp:                          keyUp,
		FlagsChanged:                   flagsChanged,
	})
	if err != nil {
		return fmt.Errorf("implement NSView err: %v", err)
	}

	return
}

func windowShouldClose(self NSWindowDelegate, sender NSWindow) bool {
	self.Retain()
	defer self.Release()

	if window, has := windowMap[sender]; has {
		window.onEvent(events.CloseEvent{})
		return false
	}
	return true
}

func windowDidResize(self NSWindowDelegate, notification NSNotification) {
	self.Retain()
	defer self.Release()

	if window, has := windowMap[Cast[NSWindow](notification.Object())]; has {
		rect := window.view.Frame()
		window.onEvent(events.SizeEvent{
			Width:  uint(rect.Size.Width),
			Height: uint(rect.Size.Height),
		})
	}
}

func windowDidBecomeKey(self NSWindowDelegate, notification NSNotification) {
	self.Retain()
	defer self.Release()

	if window, has := windowMap[Cast[NSWindow](notification.Object())]; has {
		window.onEvent(events.FocusEvent{Focused: true})
	}
}

func windowDidResignKey(self NSWindowDelegate, notification NSNotification) {
	self.Retain()
	defer self.Release()

	if window, has := windowMap[Cast[NSWindow](notification.Object())]; has {
		window.onEvent(events.FocusEvent{Focused: false})
	}
}

func viewDidChangeBackingProperties(self NSView) {
	self.Retain()
	defer self.Release()

	if window, has := windowMap[self.Window()]; has {
		rect := self.Frame()
		fbRect := self.ConvertRectToBacking(rect)
		scaleFactor := fbRect.Size.Width / rect.Size.Width

		window.onEvent(events.ScaleEvent{
			ScaleFactor: scaleFactor,
		})
	}
}

func drawRect(self NSView, rect NSRect) {
	self.Retain()
	defer self.Release()

	if window, has := windowMap[self.Window()]; has {
		window.onEvent(events.PaintEvent{})
	}
}

func (w *Window) sendCreatedEvents() {
	rect := w.view.Frame()
	fbRect := w.view.ConvertRectToBacking(rect)

	scaleFactor := fbRect.Size.Width / rect.Size.Width
	w.onEvent(events.ScaleEvent{
		ScaleFactor: scaleFactor,
	})
	w.onEvent(events.SizeEvent{
		Width:  uint(rect.Size.Width),
		Height: uint(rect.Size.Height),
	})
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
