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

	"github.com/goexlib/mathx"
)

type Window struct {
	window       NSWindow
	delegate     NSWindowDelegate
	view         NSView
	trackingArea NSTrackingArea
	onEvent      events.EventHandler

	parent    common.Window
	buttons   events.PointerButtons
	modifiers events.Modifiers
	minWidth  float32
	minHeight float32

	im     *inputMethod // this window's IME (nil when none); keyDown routes to it
	cursor *cursor      // this window's cursor capability (nil when none)
	state  common.WindowState

	controlsPosition             *windowControlsPosition // value-only preference/defaults; no retained NSButton references
	controlsFullscreenTransition bool                    // native will/did/failure notifications, not requested state
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
		if styleMask&NSWindowStyleMaskFullSizeContentView != 0 {
			// Keep the native frame and traffic lights. Only the title/background
			// are removed; the ordinary content view paints underneath them.
			win.window.SetTitleVisibility(NSWindowTitleHidden)
			win.window.SetTitlebarAppearsTransparent(true)
		}

		win.window.SetContentView(win.view)
		// Window creation takes GOUI logical units. Match its point size to the
		// effective scale before publishing the initial backing dimensions.
		if styleMask&NSWindowStyleMaskFullSizeContentView != 0 || pointsPerLogicalUnit(win.window) != 1 {
			// Full-size content includes the titlebar area. Set the content view
			// size explicitly; do not add a guessed titlebar height to the request.
			win.window.SetContentSize(logicalContentSize(win.window, float32(rect.Size.Width), float32(rect.Size.Height)))
		}
		win.window.MakeFirstResponder(win.view.NSResponder)
		win.window.SetDelegate(win.delegate)
		// NSTrackingMouseMoved delivers motion to the tracking-area owner.
		// Leave acceptsMouseMovedEvents at its default (false): enabling the
		// separate first-responder path duplicates in-view motion and can also
		// deliver motion outside the view. Mouse-down/drag/up do not need it.
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
	w.controlsPosition = nil
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
		w.updateControlsPosition()
	})
	return nil
}

func (w *Window) Show() error {
	AutoReleasePool(func() {
		NSApp.ActivateIgnoringOtherApps(true)
		w.window.MakeKeyAndOrderFront(0)
		w.updateControlsPosition()
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
		WindowDidChangeOcclusionState:  windowDidChangeManagement,
		WindowShouldClose:              windowShouldClose,
		WindowDidResize:                windowDidResize,
		WindowDidBecomeKey:             windowDidBecomeKey,
		WindowDidResignKey:             windowDidResignKey,
		WindowDidMiniaturize:           windowDidChangeManagement,
		WindowDidDeminiaturize:         windowDidChangeManagement,
		WindowWillEnterFullScreen:      windowWillChangeFullscreen,
		WindowWillExitFullScreen:       windowWillChangeFullscreen,
		WindowDidEnterFullScreen:       windowDidChangeFullscreen,
		WindowDidExitFullScreen:        windowDidChangeFullscreen,
		WindowDidFailToEnterFullScreen: windowDidFailChangeFullscreen,
		WindowDidFailToExitFullScreen:  windowDidFailChangeFullscreen,
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
		// Content input belongs to the caller, including under an Integrated
		// titlebar. Starting a native drag requires an explicit BeginMove;
		// otherwise one press could both move the window and activate a Widget.
		MouseDownCanMoveWindow:         func(NSView) bool { return false },
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
		window.updateControlsPosition()
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
		window.updateControlsPosition()
		window.emitEvent(events.FocusEvent{Focused: true})
	}
}

func windowDidResignKey(self NSWindowDelegate, notification NSNotification) {
	self.Retain()
	defer self.Release()

	if window, has := windowMap[Cast[NSWindow](notification.Object())]; has {
		window.updateControlsPosition()
		window.emitEvent(events.FocusEvent{Focused: false})
	}
}

func viewDidChangeBackingProperties(self NSView) {
	self.Retain()
	defer self.Release()

	if window, has := windowMap[self.Window()]; has {
		window.SetMinSize(window.minWidth, window.minHeight)
		window.updateControlsPosition()
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
	if options.Chrome == common.WindowChromeIntegrated {
		style |= NSWindowStyleMaskFullSizeContentView
	}
	return style
}

func (w *Window) Chrome() common.WindowChrome {
	if !w.window.Valid() {
		return common.WindowChromeUnknown
	}
	style := w.window.StyleMask()
	if style&NSWindowStyleMaskTitled == 0 {
		return common.WindowChromeNone
	}
	if style&NSWindowStyleMaskFullSizeContentView != 0 {
		if w.window.TitleVisibility() == NSWindowTitleHidden && w.window.TitlebarAppearsTransparent() {
			return common.WindowChromeIntegrated
		}
		return common.WindowChromeUnknown
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
		window.updateControlsPosition()
		window.notifyState()
	}
}

type windowControlsPosition struct {
	position      geometry.Point // copied GOUI DIP preference
	defaultOrigin NSPoint        // native top-left point offset, independent of GOUI scale
	defaultHeight CGFloat        // original titlebar-container height in native points
}

func (w *Window) SetControlsPosition(position *geometry.Point) (err error) {
	if !w.window.Valid() {
		return common.ErrUnavailable
	}
	if w.Chrome() != common.WindowChromeIntegrated {
		return common.ErrUnsupported
	}
	if position != nil {
		if err := validateControlsPosition(*position); err != nil {
			return err
		}
		if w.controlsPlacementSuspended() {
			return common.ErrUnavailable
		}
	}
	AutoReleasePool(func() {
		if position == nil {
			if w.controlsPosition == nil {
				return
			}
			if !w.controlsPlacementSuspended() {
				err = w.restoreControlsPosition()
				if err != nil {
					return // a failed reset keeps the previous preference
				}
			}
			w.controlsPosition = nil
			return
		}
		var controls nativeWindowControls
		controls, err = w.nativeControls()
		if err != nil {
			return
		}
		var next windowControlsPosition
		if w.controlsPosition != nil {
			next = *w.controlsPosition
		} else {
			bounds, group := w.view.Bounds(), controls.bounds(w.view)
			next.defaultOrigin = NSPoint{X: group.Origin.X - bounds.Origin.X,
				Y: bounds.Origin.Y + bounds.Size.Height - group.Origin.Y - group.Size.Height}
			next.defaultHeight = controls.container.Frame().Size.Height
		}
		next.position = *position
		origin := controlsPositionInPoints(next.position, pointsPerLogicalUnit(w.window))
		height := max(next.defaultHeight, controls.bounds(w.view).Size.Height+2*origin.Y)
		if err = controls.place(w.view, origin, height); err == nil {
			w.controlsPosition = &next
		}
	})
	return err
}

func validateControlsPosition(position geometry.Point) error {
	for _, value := range []float32{position.X, position.Y} {
		if value < 0 || mathx.IsNaN(value) || mathx.IsInf(value, 0) {
			return fmt.Errorf("invalid controls position: %v", position)
		}
	}
	return nil
}

func (w *Window) controlsPlacementSuspended() bool {
	return w.controlsFullscreenTransition || w.window.StyleMask()&NSWindowStyleMaskFullScreen != 0
}

func (w *Window) updateControlsPosition() {
	if w.controlsPosition == nil || !w.window.Valid() || w.controlsPlacementSuspended() {
		return
	}
	// Native layout may be temporarily unavailable during show/miniaturization.
	// Keep the preference for the next native update; ControlsRect still reads
	// the real buttons, never this stored value. Do not reposition from a getter.
	if controls, err := w.nativeControls(); err == nil {
		origin := controlsPositionInPoints(w.controlsPosition.position, pointsPerLogicalUnit(w.window))
		height := max(w.controlsPosition.defaultHeight, controls.bounds(w.view).Size.Height+2*origin.Y)
		_ = controls.place(w.view, origin, height)
	}
}

func (w *Window) restoreControlsPosition() error {
	controls, err := w.nativeControls()
	if err != nil {
		return err
	}
	return controls.place(w.view, w.controlsPosition.defaultOrigin, w.controlsPosition.defaultHeight)
}

// Native buttons remain in AppKit's hierarchy and retain their actions,
// accessibility and tracking. As in Electron, only their container frame and
// origins are adjusted; there are no private selectors or replacement buttons.
// Rediscover these borrowed views each time: AppKit may recreate the controls.
type nativeWindowControls struct {
	buttons   [3]NSButton
	parent    NSView
	container NSView
}

func (w *Window) nativeControls() (nativeWindowControls, error) {
	var controls nativeWindowControls
	for i, kind := range [...]NSWindowButton{NSWindowCloseButton, NSWindowMiniaturizeButton, NSWindowZoomButton} {
		button := w.window.StandardWindowButton(kind)
		if !button.Valid() || button.Window() != w.window || button.IsHiddenOrHasHiddenAncestor() {
			return controls, common.ErrUnavailable
		}
		if i == 0 {
			controls.parent = button.Superview()
		} else if button.Superview() != controls.parent {
			return controls, common.ErrUnavailable
		}
		if bounds := button.Bounds(); bounds.Size.Width <= 0 || bounds.Size.Height <= 0 {
			return controls, common.ErrUnavailable
		}
		controls.buttons[i] = button
	}
	if !controls.parent.Valid() {
		return controls, common.ErrUnavailable
	}
	controls.container = controls.parent.Superview()
	if !controls.container.Valid() || !controls.container.Superview().Valid() ||
		controls.container.Window() != w.window || controls.container == w.view || controls.parent == w.view {
		return controls, common.ErrUnavailable
	}
	return controls, nil
}

func (c nativeWindowControls) bounds(view NSView) NSRect {
	result := c.buttons[0].ConvertRectToView(c.buttons[0].Bounds(), view)
	for _, button := range c.buttons[1:] {
		r := button.ConvertRectToView(button.Bounds(), view)
		x, y := min(result.Origin.X, r.Origin.X), min(result.Origin.Y, r.Origin.Y)
		result = NSMakeRect(x, y,
			max(result.Origin.X+result.Size.Width, r.Origin.X+r.Size.Width)-x,
			max(result.Origin.Y+result.Size.Height, r.Origin.Y+r.Size.Height)-y)
	}
	return result
}

func (c nativeWindowControls) place(view NSView, origin NSPoint, height CGFloat) error {
	bounds, group := view.Bounds(), c.bounds(view)
	// Do not silently clamp an impossible request or extend a titlebar container
	// beyond the client. A later resize can make an accepted preference unfit;
	// the caller can observe that via ControlsRect and choose a new position.
	if origin.X+group.Size.Width > bounds.Size.Width || height > bounds.Size.Height {
		return fmt.Errorf("controls position does not fit client: %w", common.ErrUnavailable)
	}
	var frames [3]NSRect
	for i, button := range c.buttons {
		frames[i] = button.Frame()
	}
	from := c.parent.ConvertPointFromView(group.Origin, view)
	frame := c.container.Frame()
	next := controlsContainerFrame(frame, height)
	if next != frame {
		c.container.SetFrame(next)
	}
	// The container may autoresize its descendants. Translate the saved button
	// frames, preserving their sizes and gaps even if native autoresizing moved
	// them. The destination conversion must use the new container geometry.
	target := controlsTargetOrigin(bounds, group.Size, origin)
	to := c.parent.ConvertPointFromView(target, view)
	dx, dy := to.X-from.X, to.Y-from.Y
	for i, button := range c.buttons {
		r := frames[i]
		r.Origin.X += dx
		r.Origin.Y += dy
		if r != button.Frame() {
			button.SetFrame(r)
		}
	}
	return nil
}

func controlsPositionInPoints(position geometry.Point, pointsPerDIP CGFloat) NSPoint {
	return NSPoint{X: CGFloat(position.X) * pointsPerDIP, Y: CGFloat(position.Y) * pointsPerDIP}
}

func controlsTargetOrigin(bounds NSRect, size NSSize, topLeft NSPoint) NSPoint {
	return NSPoint{X: bounds.Origin.X + topLeft.X, Y: bounds.Origin.Y + bounds.Size.Height - topLeft.Y - size.Height}
}

func controlsContainerFrame(frame NSRect, height CGFloat) NSRect {
	frame.Origin.Y += frame.Size.Height - height // keep the native top edge fixed
	frame.Size.Height = height
	return frame
}

func windowWillChangeFullscreen(self NSWindowDelegate, notification NSNotification) {
	self.Retain()
	defer self.Release()
	if w := windowMap[Cast[NSWindow](notification.Object())]; w != nil {
		if w.controlsPosition != nil && !w.controlsPlacementSuspended() {
			_ = w.restoreControlsPosition()
		}
		w.controlsFullscreenTransition = true
	}
}

func windowDidChangeFullscreen(self NSWindowDelegate, notification NSNotification) {
	self.Retain()
	defer self.Release()
	if w := windowMap[Cast[NSWindow](notification.Object())]; w != nil {
		w.controlsFullscreenTransition = false
		w.updateControlsPosition()
		w.notifyState()
	}
}

func windowDidFailChangeFullscreen(self NSWindowDelegate, window NSWindow) {
	self.Retain()
	defer self.Release()
	if w := windowMap[window]; w != nil {
		w.controlsFullscreenTransition = false
		w.updateControlsPosition()
		w.notifyState()
	}
}

var _ common.DesktopWindow = (*Window)(nil)
