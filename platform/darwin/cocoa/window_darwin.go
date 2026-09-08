package cocoa

import (
	"fmt"
	"image"

	"github.com/golang-gui/goui/core/geometry"
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
	minWidth     float32
	minHeight    float32
	im           *inputMethod // this window's IME (nil when none); keyDown routes to it
	cursor       *cursor      // this window's cursor capability (nil when none)
	state        common.WindowState
}

// newNativeWindow creates the NSWindow shared by top-level windows and popups:
// it allocates the delegate/view, initializes the window with the given style
// and rect, wires the content view and delegate, and registers it for event
// routing. Callers apply role-specific setup (collection behavior / level).
func newNativeWindow(onEvent events.EventHandler, class NSWindowClass, styleMask NSWindowStyleMask, rect NSRect) *Window {
	win := &Window{
		onEvent: onEvent,
		state:   common.WindowStateUnknown,
	}
	AutoReleasePool(func() {
		win.delegate = delegateClass.Alloc()
		win.view = viewClass.Alloc().Init()

		win.window = class.Alloc().InitWith(rect, styleMask, NSBackingStoreBuffered, false)

		win.window.SetContentView(win.view)
		// Window creation takes GOUI logical units. Match its point size to the
		// effective scale before publishing the initial backing dimensions.
		if pointsPerLogicalUnit(win.window) != 1 {
			win.window.SetContentSize(logicalContentSize(win.window, float32(rect.Size.Width), float32(rect.Size.Height)))
		}
		win.window.MakeFirstResponder(win.view.NSResponder)
		win.window.SetDelegate(win.delegate)
		win.window.SetAcceptsMouseMovedEvents(true)
		win.window.SetRestorable(false)
	})
	windowMap[win.window] = win
	win.updateTrackingArea()
	win.sendCreatedEvents()
	return win
}

func newWindow(size geometry.Size, onEvent events.EventHandler, options common.WindowOptions) (*Window, error) {
	if err := common.ValidateWindowSize(size); err != nil {
		return nil, err
	}
	if err := options.Validate(); err != nil {
		return nil, err
	}
	if options.Chrome == common.WindowChromeIntegrated {
		return nil, fmt.Errorf("integrated window chrome: %w", common.ErrUnsupported)
	}
	styleMask := windowStyle(options)

	// newNativeWindow converts the requested logical content size to points.
	win := newNativeWindow(onEvent, windowClass, styleMask, NSMakeRect(0, 0, CGFloat(size.Width), CGFloat(size.Height)))

	AutoReleasePool(func() {
		win.window.SetCollectionBehavior(NSWindowCollectionBehaviorManaged | NSWindowCollectionBehaviorFullScreenPrimary)
	})
	return win, nil
}

func (w *Window) NativeHandle() uintptr {
	return uintptr(w.window.ID)
}

func (w *Window) Destroy() {
	if movePress.window == w {
		movePress = nativePress{}
	}
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

func (w *Window) Hide() error {
	if !w.window.Valid() {
		return nil
	}
	AutoReleasePool(func() {
		w.window.OrderOut(0)
	})
	return nil
}

func (w *Window) RequestClose() error {
	if !w.window.Valid() {
		return nil
	}

	// Programmatic close still uses the vetoable notification when the native
	// close button is disabled/absent. PerformClose would only beep in that case.
	w.emitEvent(events.CloseEvent{})
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

func (w *Window) SetMinSize(width, height float32) {
	if !w.window.Valid() {
		return
	}
	w.minWidth, w.minHeight = width, height
	AutoReleasePool(func() {
		// A zero size clears the constraint.
		w.window.SetContentMinSize(logicalContentSize(w.window, width, height))
	})
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
	popupClass    NSWindowClass
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
	// Popups accept pointer input without taking native keyboard/main-window
	// status from their owner. Keyboard routing remains with the owner window.
	popupClass, err = ImplementNSWindow("GouiPopupWindow", NSWindowOverride{
		CanBecomeKeyWindow:  func(NSWindow) bool { return false },
		CanBecomeMainWindow: func(NSWindow) bool { return false },
	})
	if err != nil {
		return fmt.Errorf("implement popup NSWindow err: %v", err)
	}

	delegateClass, err = ImplementNSWindowDelegate("GouiWindowDelegate", NSWindowDelegateOverride{
		WindowDidChangeOcclusionState: windowDidChangeManagement,
		WindowShouldClose:             windowShouldClose,
		WindowDidResize:               windowDidResize,
		WindowDidBecomeKey:            windowDidBecomeKey,
		WindowDidResignKey:            windowDidResignKey,
		WindowDidMiniaturize:          windowDidChangeManagement,
		WindowDidDeminiaturize:        windowDidChangeManagement,
		WindowDidEnterFullScreen:      windowDidChangeManagement,
		WindowDidExitFullScreen:       windowDidChangeManagement,
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

	err = ImplementNSTextInputClient(viewClass, NSTextInputClientOverride{
		InsertText:                   imInsertText,
		SetMarkedText:                imSetMarkedText,
		UnmarkText:                   imUnmarkText,
		HasMarkedText:                imHasMarkedText,
		MarkedRange:                  imMarkedRange,
		SelectedRange:                imSelectedRange,
		FirstRectForCharacterRange:   imFirstRect,
		AttributedSubstring:          imAttributedSubstring,
		ValidAttributesForMarkedText: imValidAttributes,
		CharacterIndexForPoint:       imCharacterIndexForPoint,
		DoCommandBySelector:          imDoCommandBySelector,
	})
	if err != nil {
		return fmt.Errorf("implement NSTextInputClient err: %v", err)
	}

	return
}

func windowShouldClose(self NSWindowDelegate, sender NSWindow) bool {
	self.Retain()
	defer self.Release()

	if window, has := windowMap[sender]; has {
		window.emitEvent(events.CloseEvent{})
		return false
	}
	return true
}

// makeSizeEvent builds a SizeEvent carrying both the logical size and
// the physical (backing pixel) size of the view.
func makeSizeEvent(view NSView) events.SizeEvent {
	rect := view.Bounds()
	fbRect := view.ConvertRectToBacking(rect)
	// An override affects layout units, never the native drawable dimensions.
	// Inventing backing sizes here makes glViewport extend beyond the actual
	// surface and clips translated/scaled geometry out of the window.
	if scale := common.GetPreferScale(); scale > 0 {
		return events.SizeEvent{
			Width:       float32(fbRect.Size.Width) / scale,
			Height:      float32(fbRect.Size.Height) / scale,
			PixelWidth:  float32(fbRect.Size.Width),
			PixelHeight: float32(fbRect.Size.Height),
		}
	}
	return events.SizeEvent{
		Width:       float32(rect.Size.Width),
		Height:      float32(rect.Size.Height),
		PixelWidth:  float32(fbRect.Size.Width),
		PixelHeight: float32(fbRect.Size.Height),
	}
}

func windowDidResize(self NSWindowDelegate, notification NSNotification) {
	self.Retain()
	defer self.Release()

	if window, has := windowMap[Cast[NSWindow](notification.Object())]; has {
		window.notifyState()
		if !window.window.Valid() {
			return
		}
		window.emitEvent(makeSizeEvent(window.view))
	}
}

func windowDidBecomeKey(self NSWindowDelegate, notification NSNotification) {
	self.Retain()
	defer self.Release()

	if window, has := windowMap[Cast[NSWindow](notification.Object())]; has {
		window.emitEvent(events.FocusEvent{Focused: true})
	}
}

func windowDidResignKey(self NSWindowDelegate, notification NSNotification) {
	self.Retain()
	defer self.Release()

	if window, has := windowMap[Cast[NSWindow](notification.Object())]; has {
		window.emitEvent(events.FocusEvent{Focused: false})
	}
}

func viewDidChangeBackingProperties(self NSView) {
	self.Retain()
	defer self.Release()

	if window, has := windowMap[self.Window()]; has {
		window.SetMinSize(window.minWidth, window.minHeight)
		// Both the backing size and the point-to-logical conversion may change.
		window.emitEvent(makeSizeEvent(self))
	}
}

func drawRect(self NSView, rect NSRect) {
	self.Retain()
	defer self.Release()

	if window, has := windowMap[self.Window()]; has {
		window.emitEvent(events.PaintEvent{})
	}
}

func (w *Window) sendCreatedEvents() {
	w.emitEvent(makeSizeEvent(w.view))
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
			// The bitmap is in physical (backing) pixels; draw it into the view's
			// logical point rect so the retina CGContext maps it 1:1 onto the
			// backing store instead of upscaling a point-sized image.
			frame := w.view.Frame()
			CGContextDrawImage(context, NSMakeRect(0, 0, frame.Size.Width, frame.Size.Height), cgImage)
		}
	}
	return nil
}

func windowStyle(options common.WindowOptions) NSWindowStyleMask {
	style := NSWindowStyleMaskResizable | NSWindowStyleMaskClosable | NSWindowStyleMaskMiniaturizable
	if options.Chrome != common.WindowChromeNone {
		style |= NSWindowStyleMaskTitled
	}
	return style
}

func (w *Window) Chrome() common.WindowChrome {
	if !w.window.Valid() {
		return common.WindowChromeUnknown
	}
	if w.window.StyleMask()&NSWindowStyleMaskTitled == 0 {
		return common.WindowChromeNone
	}
	return common.WindowChromeNative
}

func (w *Window) ControlsRect() (geometry.Rectangle, error) {
	if !w.window.Valid() || !w.view.Valid() {
		return geometry.Rectangle{}, common.ErrUnavailable
	}
	if !w.window.IsVisible() || w.window.IsMiniaturized() {
		return geometry.Rectangle{}, common.ErrUnavailable
	}
	var result geometry.Rectangle
	bounds := w.view.Bounds()
	scale := pointsPerLogicalUnit(w.window)
	for _, kind := range [...]NSWindowButton{NSWindowCloseButton, NSWindowMiniaturizeButton, NSWindowZoomButton} {
		button := w.window.StandardWindowButton(kind)
		if !button.Valid() || button.IsHiddenOrHasHiddenAncestor() {
			continue
		}
		// AppKit can temporarily host fullscreen controls in another window.
		// Cross-window convertRect:toView: is not a valid observation.
		if button.Window() != w.window {
			return geometry.Rectangle{}, common.ErrUnavailable
		}
		r := controlsRectInDIP(button.ConvertRectToView(button.Bounds(), w.view), bounds, scale)
		if r.Width <= 0 || r.Height <= 0 {
			continue
		}
		if result.Width == 0 {
			result = r
		} else {
			x, y := min(result.X, r.X), min(result.Y, r.Y)
			result = geometry.Rect(x, y,
				max(result.X+result.Width, r.X+r.Width)-x,
				max(result.Y+result.Height, r.Y+r.Height)-y)
		}
	}
	return result, nil
}

// The content view is unflipped; GOUI uses top-left client coordinates.
func controlsRectInDIP(rect, bounds NSRect, pointsPerDIP CGFloat) geometry.Rectangle {
	return geometry.Rect(
		float32((rect.Origin.X-bounds.Origin.X)/pointsPerDIP),
		float32((bounds.Origin.Y+bounds.Size.Height-rect.Origin.Y-rect.Size.Height)/pointsPerDIP),
		float32(rect.Size.Width/pointsPerDIP), float32(rect.Size.Height/pointsPerDIP))
}

func (w *Window) SetHitTest(func(geometry.Point) common.WindowHit) error {
	if !w.window.Valid() {
		return common.ErrUnavailable
	}
	return common.ErrUnsupported
}

func (w *Window) BeginMove() error {
	if !w.window.Valid() || movePress.window != w || !movePress.event.Valid() {
		return common.ErrUnavailable
	}
	event, native := movePress.event, w.window
	movePress = nativePress{}
	w.buttons &^= events.PointerButtonLeftDown
	// AppKit may enter a modal loop, consume mouseUp and reenter Go callbacks.
	// Keep the native objects alive even if one of those callbacks destroys w.
	native.Retain()
	defer native.Release()
	event.Retain()
	defer event.Release()
	native.PerformWindowDrag(event)
	return nil
}

func (w *Window) BeginResize(common.WindowEdge) error {
	if !w.window.Valid() {
		return common.ErrUnavailable
	}
	return common.ErrUnsupported
}

func (w *Window) State() common.WindowState {
	if !w.window.Valid() {
		return common.WindowStateUnknown
	}
	if w.window.IsMiniaturized() {
		return common.WindowStateMinimized
	}
	if !w.window.IsVisible() {
		return common.WindowStateHidden
	}
	if w.window.StyleMask()&NSWindowStyleMaskFullScreen != 0 {
		return common.WindowStateFullscreen
	}
	// AppKit zoom changes ordinary window geometry; it is not maximization.
	return common.WindowStateNormal
}

func (w *Window) RequestState(state common.WindowState) error {
	if !w.window.Valid() {
		return common.ErrUnavailable
	}
	switch state {
	case common.WindowStateHidden:
		return w.Hide()
	case common.WindowStateMaximized, common.WindowStateFullscreen:
		// A fullscreen target needs in-flight/failure tracking, not a toggle.
		return common.ErrUnsupported
	case common.WindowStateNormal, common.WindowStateMinimized:
	default:
		return fmt.Errorf("invalid window state: %d", state)
	}
	if w.window.StyleMask()&NSWindowStyleMaskFullScreen != 0 {
		return common.ErrUnsupported
	}
	if w.State() == state {
		return nil
	}
	if state == common.WindowStateMinimized {
		if w.window.StyleMask()&NSWindowStyleMaskMiniaturizable == 0 {
			return common.ErrUnsupported
		}
		AutoReleasePool(func() { w.window.Miniaturize(0) })
		return nil
	}
	AutoReleasePool(func() {
		if w.window.IsMiniaturized() {
			w.window.Deminiaturize(0)
		}
	})
	return w.Show()
}

func (w *Window) notifyState() {
	state := w.State()
	if state != w.state {
		w.state = state
		w.emitEvent(events.StateEvent{State: state})
	}
}

func windowDidChangeManagement(self NSWindowDelegate, notification NSNotification) {
	self.Retain()
	defer self.Release()
	if window := windowMap[Cast[NSWindow](notification.Object())]; window != nil {
		window.notifyState()
	}
}

var _ common.DesktopWindow = (*Window)(nil)
