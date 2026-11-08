package gui

import (
	"errors"
	"fmt"
	"image/color"
	"log"
	"math"
	"runtime"

	"github.com/golang-gui/goui/core/geometry"
	"github.com/golang-gui/goui/core/signal"
	"github.com/golang-gui/goui/platform"
	"github.com/golang-gui/goui/platform/events"
	"github.com/golang-gui/goui/platform/graphics"
	"github.com/golang-gui/goui/style"
)

// Window owns a native window and its Widget tree. Operations and callbacks run
// on the GUI thread. Connections are controlled by their returned Handles.
// Once destruction starts, Connect methods return inert Handles and ordinary
// notifications skip callbacks that have not started; destroy callbacks still run.
type Window interface {
	Surface

	Title() string

	SetTitle(string) error

	// Chrome returns the stable, non-nil titlebar integration signal service.
	// Unsupported integration uses a service that reports disabled information.
	Chrome() WindowChrome

	// State reports one native presentation state, or Unknown. RequestState
	// is best effort; it does not promise the requested result.
	// Normal requests an ordinary window, not a historical Restore operation.
	State() WindowState

	RequestState(WindowState)

	// Show makes the window visible, restores it if minimized, and requests
	// foreground activation without changing its maximized/fullscreen mode or
	// logical widget focus. The system may refuse activation; observe Focused
	// and ConnectFocus for the actual result. A destroyed window returns
	// platform.ErrUnavailable. Call on the GUI thread, including repeated requests.
	Show() error

	// WorkAreaAt returns the work area containing point (or the nearest one),
	// in this window's client-local DIP. Unsupported hosts return ErrUnsupported.
	WorkAreaAt(point geometry.Point) (geometry.Rectangle, error)

	// SetPosition requests the client-area top-left in relativeTo's client DIP.
	// nil uses this window's current WorkAreaAt(Point{}) top-left and DIP instead.
	// This is a one-shot request, not ownership or following. Negative positions
	// are allowed; the window manager may adjust or ignore the request. It does
	// not resize or activate the window. Errors are returned to the caller.
	SetPosition(relativeTo Window, position geometry.Point) error

	// Position observes the actual client-area top-left in the same reference
	// coordinates as SetPosition. nil uses this window's current work area;
	// self-reference returns (0, 0). It may be queried before Show once native
	// client geometry exists. It does not wait for pending moves or return the
	// last requested/restored position. Unavailable native geometry and
	// unsupported hosts return errors.
	Position(relativeTo Window) (geometry.Point, error)

	RequestClose() error

	// SetMinSize supplies a desktop WM hint in DIP; it is a no-op for non-desktop
	// hosts. Like the creation size, it describes the client area irrespective
	// of Chrome. Widget layout constraints are independent of this advisory hint.
	// Only explicit calls set this hint; content measurement never supplies it.
	// Zero components clear the application constraint on those axes, preserving
	// any native system limits.
	SetMinSize(geometry.Size)

	// ConnectCloseRequest lets fn veto a close request by setting *allow to false.
	// Each callback runs only while the window is alive.
	ConnectCloseRequest(func(allow *bool)) signal.Handle

	// ConnectState receives observed native transitions through
	// DispatchEvent while the window is alive. No event is synthesized just
	// because RequestState succeeds.
	ConnectState(func(state WindowState)) signal.Handle
}

// ModalTarget is a modal element a window forwards its input to (see
// Window.SetModalTarget). The window drives it: it dispatches forwarded keyboard
// via DispatchEvent and calls RequestDismiss on a dismiss-worthy interaction.
// A popover implements it, but so can any element — the window depends only on
// these two methods, never on a concrete type.
type ModalTarget interface {
	DispatchEvent(events.Event) error
	RequestDismiss()
}

type WindowState = platform.WindowState

const (
	WindowStateUnknown    = platform.WindowStateUnknown
	WindowStateNormal     = platform.WindowStateNormal
	WindowStateHidden     = platform.WindowStateHidden
	WindowStateMinimized  = platform.WindowStateMinimized
	WindowStateMaximized  = platform.WindowStateMaximized
	WindowStateFullscreen = platform.WindowStateFullscreen
)

type window struct {
	surfaceBase
	title        string
	closeRequest signal.Signal1[*bool]
	stateChanged signal.Signal1[WindowState]
	chrome       *windowChrome
	controls     *windowControls
	layingOut    bool
	clientChrome bool // GUI-drawn Integrated over a factual None alpha surface
}

func newWindow(app *application, options WindowOptions) (*window, error) {
	win := &window{surfaceBase: surfaceBase{rootBase: rootBase{app: app, layoutDirty: true, paintDirty: true, frameSuspended: true, transparent: options.Transparent}}}

	win.host = win
	win.claimed = true
	mode := options.Chrome
	nativeOptions := platform.WindowOptions{Chrome: platform.WindowChrome(mode), Transparent: options.Transparent, SkipTaskbar: options.SkipTaskbar}
	platformWindow, err := app.platform.NewWindow(options.Size, win.onEvent, nativeOptions)
	if mode == WindowChromeIntegrated && errors.Is(err, platform.ErrUnsupported) && platformWindow == nil &&
		runtime.GOOS == "linux" && app.platform.Name() == "x11" {
		// The platform deliberately does not claim native Integrated support.
		// Only the GUI knows how to decorate a transparent, undecorated surface.
		nativeOptions.Chrome, nativeOptions.Transparent = platform.WindowChromeNone, true
		platformWindow, err = app.platform.NewWindow(options.Size, win.onEvent, nativeOptions)
		if err == nil {
			win.clientChrome, win.transparent = true, true
		} else if platformWindow == nil && !options.Transparent && errors.Is(err, platform.ErrUnavailable) {
			// No compositor: Integrated is a preference, explicit transparency is not.
			err = platform.ErrUnsupported
		}
	}
	if mode == WindowChromeIntegrated && errors.Is(err, platform.ErrUnsupported) && platformWindow == nil {
		mode = WindowChromeNative
		nativeOptions = platform.WindowOptions{Chrome: platform.WindowChromeNative, Transparent: options.Transparent, SkipTaskbar: options.SkipTaskbar}
		platformWindow, err = app.platform.NewWindow(options.Size, win.onEvent, nativeOptions)
	}
	win.surface = platformWindow
	if err != nil {
		return nil, fmt.Errorf("create platform window: %w", err)
	}
	win.platformWindow = platformWindow
	win.Chrome()
	if err := win.chrome.initialize(mode); err != nil {
		win.Destroy()
		return nil, fmt.Errorf("create window chrome: %w", err)
	}
	if win.chrome.info.Controls != ChromeControlsNone {
		win.controls = newWindowControls(win)
		win.dispatcher.decoration = win.controls
	}

	win.painter, err = app.platform.NewPainter(platformWindow)
	if err != nil {
		win.Destroy()
		return nil, fmt.Errorf("create painter: %w", err)
	}
	win.title = win.Title()
	if options.Center {
		area, err := win.WorkAreaAt(geometry.Point{})
		if err == nil {
			err = win.SetPosition(nil, geometry.Point{
				X: (area.Width - options.Size.Width) / 2,
				Y: (area.Height - options.Size.Height) / 2,
			})
		}
		if err != nil && !errors.Is(err, platform.ErrUnsupported) && !errors.Is(err, platform.ErrUnavailable) {
			win.Destroy()
			return nil, fmt.Errorf("center initial window: %w", err)
		}
	}

	// Text input is optional. Without a native input method, navigation still
	// uses KeyEvent, but GUI does not invent characters from physical keys.
	// Commit/preedit are routed to the focused widget's IMContext.
	win.inputMethod, _ = app.platform.NewInputMethod(platformWindow, win.onInputMethod)

	// Cursor is an optional platform capability; nil means the platform controls
	// the cursor itself (no dynamic per-widget cursor support).
	win.cursor, _ = app.platform.NewCursor(platformWindow)

	return win, nil
}

func (w *window) Title() string {
	if w.title != "" {
		return w.title
	}
	if native, err := w.windowManagement(); err == nil {
		return native.Title()
	}
	return ""
}

func (w *window) SetTitle(title string) error {
	if w.platformWindow == nil {
		w.title = title
		return nil
	}
	native, err := w.windowManagement()
	if err != nil {
		return err
	}
	if err := native.SetTitle(title); err != nil {
		return err
	}
	w.title = title
	return nil
}

func (w *window) Show() error {
	native, err := w.windowManagement()
	if err != nil {
		return err
	}
	err = native.Show()
	// Showing or activating can synchronously dispatch native callbacks, and
	// application handlers may destroy the window before the request returns.
	if w.destroyed || w.platformWindow == nil {
		return platform.ErrUnavailable
	}
	if err == nil {
		w.resumeFrames(w.platformWindow)
	}
	if w.chrome != nil {
		w.chrome.nativeChanged()
	}
	return err
}

// RequestCursorUpdate re-resolves the cursor shape from the hover path and
// applies it. Widgets call this via Root when they change their cursor and are
// being hovered over.
func (w *window) RequestClose() error {
	if w.platformWindow == nil {
		return nil
	}
	native, err := w.windowManagement()
	if err != nil {
		return err
	}
	return native.RequestClose()
}

func (w *window) ConnectCloseRequest(fn func(*bool)) signal.Handle {
	if w.destroyed {
		return signal.Handles(nil)
	}
	return w.closeRequest.Connect(func(allow *bool) {
		if !w.destroyed {
			fn(allow)
		}
	})
}

func (w *window) WorkAreaAt(point geometry.Point) (geometry.Rectangle, error) {
	native, err := w.windowManagement()
	if err != nil {
		return geometry.Rectangle{}, err
	}
	return native.WorkAreaAt(point)
}

func (w *window) SetPosition(relativeTo Window, position geometry.Point) error {
	native, err := w.windowManagement()
	if err != nil {
		return err
	}
	reference, err := positionReference(relativeTo)
	if err != nil {
		return err
	}
	return native.SetPosition(reference, position)
}

func (w *window) Position(relativeTo Window) (geometry.Point, error) {
	native, err := w.windowManagement()
	if err != nil {
		return geometry.Point{}, err
	}
	reference, err := positionReference(relativeTo)
	if err != nil {
		return geometry.Point{}, err
	}
	return native.Position(reference)
}

func positionReference(relativeTo Window) (platform.Window, error) {
	var reference platform.Window
	if relativeTo != nil {
		if ref, ok := relativeTo.(*window); ok && (ref == nil || ref.destroyed) {
			return nil, platform.ErrUnavailable
		}
		reference = relativeTo.PlatformWindow()
		if reference == nil {
			return nil, platform.ErrUnavailable
		}
	}
	return reference, nil
}

func (w *window) SetMinSize(size geometry.Size) {
	if native, err := w.windowManagement(); err == nil {
		native.SetMinSize(size.Width, size.Height)
	}
}

func (w *window) paint() {
	defer w.finishDestroy()
	defer w.applyCursor() // layout/state changes can move a resize region beneath a stationary pointer
	if !w.beginFrame() {
		return
	}
	defer w.endFrame()
	w.root = liveRoot(w.root)
	if w.chrome != nil {
		w.chrome.beforeLayout()
	}
	if w.destroyed {
		return
	}
	laidOut := w.layoutDirty
	w.layoutContent()
	if w.chrome != nil && w.chrome.afterLayout() && !w.destroyed {
		// Notifications only invalidate. Apply changed reservations once in
		// this frame, without asking for height again or running a fixed point.
		w.layoutContent()
	}
	if w.destroyed {
		return
	}
	var controls Widget
	if laidOut {
		w.refreshDragMotion(w)
	}
	if w.destroyed {
		return
	}
	w.tooltip.refresh(laidOut)
	if w.destroyed {
		return
	}
	if w.controls != nil {
		w.controls.layout()
		controls = w.controls
	}
	border := w.frameStyle()
	bounds := w.contentBounds()
	var finish func(Painter)
	if w.floatingFrame() {
		finish = func(p Painter) { clearRoundCorners(p, bounds, windowFrameRadius) }
	}
	w.drawSurfaceFrame(w.root, w.backgroundStyle(), border, bounds, bounds, graphics.BoxShadow{}, finish, controls)
}

func (w *window) layoutContent() {
	w.layingOut = true
	defer func() { w.layingOut = false }()
	w.layoutFrameIn(w.root, w.contentBounds())
}

// Content uses the complete client area; resize hit zones are input only.
func (w *window) contentBounds() geometry.Rectangle {
	return geometry.Rect(0, 0, max(0, w.width), max(0, w.height))
}

func imContextOf(widget Widget) IMContext {
	client, ok := widget.(IMClient)
	if !ok {
		return nil
	}
	im := client.IMContext()
	if im == nil {
		return nil
	}
	return im
}

func visibleInTree(widget Widget) bool {
	for widget != nil {
		if !widget.Visible() {
			return false
		}
		widget = widget.Parent()
	}
	return true
}

func (w *window) windowManagement() (platform.WindowManagement, error) {
	if w.destroyed || w.platformWindow == nil {
		return nil, platform.ErrUnavailable
	}
	native, ok := w.platformWindow.(platform.WindowManagement)
	if !ok {
		return nil, platform.ErrUnsupported
	}
	return native, nil
}

func (w *window) State() WindowState {
	native, err := w.windowManagement()
	if err != nil {
		return WindowStateUnknown
	}
	return native.State()
}

// RequestState is best effort. Only actual native events change observations.
func (w *window) RequestState(state WindowState) {
	native, err := w.windowManagement()
	if err != nil {
		return
	}
	if err := native.RequestState(state); err != nil && !errors.Is(err, platform.ErrUnsupported) && !errors.Is(err, platform.ErrUnavailable) {
		log.Printf("goui: request window state: %v", err)
	}
}

func (w *window) ConnectState(fn func(WindowState)) signal.Handle {
	if w.destroyed {
		return signal.Handles(nil)
	}
	return w.stateChanged.Connect(func(state WindowState) {
		if !w.destroyed {
			fn(state)
		}
	})
}

func (w *window) floatingFrame() bool {
	return w.clientChrome && w.State() != WindowStateMaximized && w.State() != WindowStateFullscreen
}

// Only GUI-drawn Integrated owns automatic resize feedback. None callers keep
// control of their cursor, and native frames keep their OS cursor behavior.
func (w *window) frameCursor() CursorShape {
	if !w.clientChrome || !w.cursorInside || w.chrome == nil || w.layoutDirty || w.layingOut || w.State() != WindowStateNormal {
		return CursorDefault
	}
	// Use the final ordered query, including Client overrides, modal input and
	// pointer capture. Do not maintain a second geometric hit test for cursors.
	switch w.chrome.queryRegion(w.cursorPosition) {
	case ChromeRegionLeft:
		return CursorResizeLeft
	case ChromeRegionRight:
		return CursorResizeRight
	case ChromeRegionTop:
		return CursorResizeTop
	case ChromeRegionBottom:
		return CursorResizeBottom
	case ChromeRegionTopLeft:
		return CursorResizeTopLeft
	case ChromeRegionTopRight:
		return CursorResizeTopRight
	case ChromeRegionBottomLeft:
		return CursorResizeBottomLeft
	case ChromeRegionBottomRight:
		return CursorResizeBottomRight
	default:
		return CursorDefault
	}
}

// The frame is drawn over full-client content without reserving layout space.
func (w *window) frameStyle() style.Style {
	if !w.floatingFrame() {
		return style.Style{}
	}
	frame := style.Default().Radius(windowFrameRadius).BorderWidth(windowFrameBorder).
		BorderColor(w.decorationPalette(true).frame).Style
	return frame
}

// resizeBorderWidth is independent of window rounding and content layout.
func (w *window) resizeBorderWidth() float32 {
	if !w.floatingFrame() {
		return 0
	}
	scale := w.frameScale()
	return float32(math.Ceil(float64(5*scale))) / scale
}

// clearRoundCorners is host policy implemented with ordinary drawing. Each
// reflected path erases only one corner; no compound path or window API is needed.
func clearRoundCorners(p Painter, rect geometry.Rectangle, radius float32) {
	radius = min(normalizeLayoutValue(radius), max(0, min(rect.Width, rect.Height)/2))
	if radius <= 0 {
		return
	}
	// Close outside the surface. Closing along its edges creates zero-width
	// cusps at the arc tangencies, whose AA fringes can erase the first row or
	// column of retained pixels. Only the closure moves; the arc stays put.
	padding := 2 / p.PixelScale()
	path := graphics.MoveTo(-padding, -padding).LineTo(radius, -padding).
		LineTo(radius, 0).ArcTo(radius, radius, 0, 0, 0, 0, radius).
		LineTo(-padding, radius).Close()
	for _, corner := range []struct{ x, y, sx, sy float32 }{
		{rect.X, rect.Y, 1, 1}, {rect.X + rect.Width, rect.Y, -1, 1},
		{rect.X + rect.Width, rect.Y + rect.Height, -1, -1}, {rect.X, rect.Y + rect.Height, 1, -1},
	} {
		p.SetTransform(geometry.Translate(corner.x, corner.y).Scale(corner.sx, corner.sy))
		p.ClearPath(path)
	}
	p.SetTransform(geometry.Identity())
}

// These are GUI resize affordances, not a native input-region mask. Transparent
// corner pixels remain inside the rectangular native input surface.
func (c *windowChrome) queryFrameRegion(p geometry.Point, result *ChromeRegion) {
	w := c.window
	if !w.floatingFrame() || !containsPoint(geometry.Rect(0, 0, w.width, w.height), p) {
		return
	}
	edge := w.resizeBorderWidth()
	const corner float32 = 16
	left, right := p.X < edge, p.X >= w.width-edge
	top, bottom := p.Y < edge, p.Y >= w.height-edge
	switch {
	case (left && p.Y < corner) || (top && p.X < corner):
		*result = ChromeRegionTopLeft
	case (right && p.Y < corner) || (top && p.X >= w.width-corner):
		*result = ChromeRegionTopRight
	case (left && p.Y >= w.height-corner) || (bottom && p.X < corner):
		*result = ChromeRegionBottomLeft
	case (right && p.Y >= w.height-corner) || (bottom && p.X >= w.width-corner):
		*result = ChromeRegionBottomRight
	case left:
		*result = ChromeRegionLeft
	case right:
		*result = ChromeRegionRight
	case top:
		*result = ChromeRegionTop
	case bottom:
		*result = ChromeRegionBottom
	}
}

const (
	windowFrameRadius float32 = 12
	windowFrameBorder float32 = 1
)

// Window decoration is GUI policy, not a set of application style selectors.
// Native decorations remain OS-owned. Only the window background is themed.
type windowDecorationPalette struct {
	frame, foreground, button, hovered, pressed color.Gray
}

func (w *window) decorationPalette(circular bool) windowDecorationPalette {
	background := w.backgroundStyle()
	if darkWindowBackground(background, w.transparent) {
		if circular {
			return windowDecorationPalette{
				frame: color.Gray{90}, foreground: color.Gray{240},
				button: color.Gray{64}, hovered: color.Gray{80}, pressed: color.Gray{48},
			}
		}
		return windowDecorationPalette{
			frame: color.Gray{90}, foreground: color.Gray{240},
			button: color.Gray{50}, hovered: color.Gray{70}, pressed: color.Gray{35},
		}
	}
	if circular {
		return windowDecorationPalette{
			frame: color.Gray{165}, foreground: color.Gray{50},
			button: color.Gray{232}, hovered: color.Gray{215}, pressed: color.Gray{195},
		}
	}
	return windowDecorationPalette{
		frame: color.Gray{165}, foreground: color.Gray{0},
		button: color.Gray{210}, hovered: color.Gray{230}, pressed: color.Gray{180},
	}
}

func darkWindowBackground(background style.Style, transparent bool) bool {
	var r, g, b, a uint32
	if bg, ok := background.BackgroundColor(); ok && bg != nil {
		r, g, b, a = bg.RGBA()
	}
	if transparent {
		// The compositor's backdrop is unknown. Use the declared RGB, with a
		// deterministic light fallback for fully transparent/missing backgrounds.
		if a == 0 {
			return false
		}
		r, g, b = r*0xffff/a, g*0xffff/a, b*0xffff/a
	}
	// An opaque host clears uncovered pixels to black; RGBA's premultiplied
	// channels already represent composition of the background over that clear.
	return 299*r+587*g+114*b < 1000*0x8000
}
