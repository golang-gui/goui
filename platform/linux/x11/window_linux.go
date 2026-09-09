package x11

import (
	"errors"
	"fmt"
	"image"
	"slices"

	"github.com/golang-gui/goui/core/geometry"
	"github.com/golang-gui/goui/platform/common"
	"github.com/golang-gui/goui/platform/events"
	"github.com/golang-gui/goui/platform/graphics"
	"github.com/golang-gui/goui/platform/graphics/opengl"

	"github.com/golang-gui/goui/platform/linux/libs/glx"
	"github.com/golang-gui/goui/platform/linux/libs/xlib"

	"github.com/goexlib/cgo"
)

type Window struct {
	wid              xlib.Window
	fb               glx.FBConfig
	cmap             xlib.Colormap
	parent           common.Window
	onEvent          events.EventHandler
	width            int32
	height           int32
	title            string
	gc               xlib.GC
	buttons          events.PointerButtons
	minW             float32      // logical (DIP) minimum size; 0 = unbounded
	minH             float32      // logical (DIP) minimum size; 0 = unbounded
	im               *inputMethod // this window's IME (nil when none); the key loop consults it
	cursor           *cursor      // this window's cursor capability (nil when none)
	resizeSync       resizeSync
	paintPending     bool
	moveResize       bool // a WM interaction was requested; a raced release must cancel it
	state            common.WindowState
	overrideRedirect bool
}

// newNativeWindow creates the X11 InputOutput window shared by both top-level
// windows and popups: it picks a GL-capable visual, creates the colormap, and
// registers the window for event routing. overrideRedirect makes a borderless,
// WM-bypassing surface (popups); width/height is the initial size in pixels.
func newNativeWindow(onEvent events.EventHandler, overrideRedirect bool, width, height int) (*Window, error) {
	win := &Window{
		onEvent:          onEvent,
		state:            common.WindowStateUnknown,
		overrideRedirect: overrideRedirect,
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
		BorderPixel: 0,
		Colormap:    win.cmap,
		BitGravity:  xlib.NorthWestGravity,
		EventMask: xlib.EventMaskStructureNotify |
			xlib.EventMaskExposure |
			xlib.EventMaskPropertyChange |
			xlib.EventMaskKeyPress |
			xlib.EventMaskKeyRelease |
			xlib.EventMaskFocusChange |
			xlib.EventMaskButtonPress |
			xlib.EventMaskButtonRelease |
			xlib.EventMaskPointerMotion |
			xlib.EventMaskEnterWindow |
			xlib.EventMaskLeaveWindow,
	}

	valueMask := uint(xlib.CwBorderPixel | xlib.CwColormap | xlib.CwEventMask | xlib.CwBitGravity)
	if overrideRedirect {
		attr.OverrideRedirect = 1
		valueMask |= xlib.CwOverrideRedirect
	}

	win.wid = platform.display.CreateWindow(platform.defScreen.Root,
		0, 0, width, height, 0,
		depth, xlib.WindowClassInputOutput, visual, valueMask, &attr)

	if win.wid == 0 {
		return nil, errors.New("create x11 window failed")
	}

	windowMap[win.wid] = win
	return win, nil
}

func newWindow(size geometry.Size, onEvent events.EventHandler, options common.WindowOptions) (common.Window, error) {
	if err := common.ValidateWindowSize(size); err != nil {
		return nil, err
	}
	if err := options.Validate(); err != nil {
		return nil, err
	}
	if options.Chrome == common.WindowChromeIntegrated {
		return nil, fmt.Errorf("integrated window chrome: %w", common.ErrUnsupported)
	}
	scale := currentScale()
	win, err := newNativeWindow(onEvent, false, physical(size.Width, scale), physical(size.Height, scale))
	if err != nil {
		return nil, err
	}
	win.applyDecoration(options.Chrome)
	win.applyMinSize()

	// Declare WM protocols for top-level windows before they are mapped. The
	// resize-sync protocol is advertised only after its counter property exists.
	syncEnabled := false
	if platform.resizeSyncAvailable &&
		platform.atoms.WM_PROTOCOLS != 0 &&
		platform.atoms._NET_WM_SYNC_REQUEST != 0 &&
		platform.atoms._NET_WM_SYNC_REQUEST_COUNTER != 0 {
		syncEnabled = win.resizeSync.initialize(
			platform.display,
			win.wid,
			platform.atoms._NET_WM_SYNC_REQUEST_COUNTER,
		)
	}
	if platform.atoms.WM_PROTOCOLS != 0 {
		protocols := []xlib.Atom{platform.atoms.WM_DELETE_WINDOW}
		if syncEnabled {
			protocols = append(protocols, platform.atoms._NET_WM_SYNC_REQUEST)
		}
		platform.display.SetWMProtocols(win.wid, protocols)
	}

	return win, nil
}

func (w *Window) NativeHandle() uintptr {
	return uintptr(w.wid)
}

func (w *Window) NativeFBConfig() glx.FBConfig {
	return w.fb
}

func (w *Window) Destroy() {
	if moveResizePress.window == w {
		moveResizePress = nativePress{}
	}
	w.moveResize = false
	if w.wid == 0 {
		return
	}

	delete(windowMap, w.wid)
	if w.im != nil {
		w.im.Destroy()
	}
	if w.gc != 0 {
		platform.display.FreeGC(w.gc)
		w.gc = 0
	}
	w.resizeSync.destroy(platform.display)
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
	if w.wid == 0 {
		return common.ErrUnavailable
	}
	platform.display.MapWindow(w.wid)
	platform.display.Flush()
	return nil
}

func (w *Window) Hide() error {
	if w.wid == 0 {
		return common.ErrUnavailable
	}
	if w.overrideRedirect {
		platform.display.UnmapWindow(w.wid)
	} else if platform.display.WithdrawWindow(w.wid, platform.display.DefaultScreen()) == 0 {
		return fmt.Errorf("XWithdrawWindow failed")
	}
	platform.display.Flush()
	return nil
}

func (w *Window) RequestClose() error {
	if w.wid == 0 {
		return nil
	}
	w.emitEvent(events.CloseEvent{})
	return nil
}

func (w *Window) RequestPaint() error {
	if w.wid == 0 {
		return nil
	}
	w.schedulePaint()
	return nil
}

func (w *Window) schedulePaint() {
	if w == nil || w.wid == 0 || w.paintPending {
		return
	}
	w.paintPending = true
	paint := func() {
		w.paintPending = false
		if w.wid == 0 {
			return
		}
		w.emitEvent(events.PaintEvent{})
		// Paint callbacks are synchronous. OpenGL has swapped its buffers and the
		// software painter has presented before the callback returns, so this is
		// the common completion point for the EWMH resize handshake. A handler with
		// nothing to paint must also release the WM instead of leaving it waiting.
		w.resizeSync.complete(platform.display)
	}
	if platform.eventLoop != nil {
		platform.eventLoop.Post(paint)
	} else {
		paint()
	}
}

func (w *Window) SetMinSize(width, height float32) {
	if w.wid == 0 {
		return
	}
	// Store logical (DIP) values; physical pixels are derived from the display
	// scale when the hints are written, keeping the hint correct if the scale
	// ever changes (X11 currently uses a static display-global scale).
	w.minW, w.minH = width, height
	w.applyMinSize()
}

// applyMinSize converts the stored logical (DIP) minimum to physical pixels
// and writes the WM_NORMAL_HINTS property.
func (w *Window) applyMinSize() {
	scale := currentScale()
	if w.minW <= 0 && w.minH <= 0 {
		// Clear the hint: delete the WM_NORMAL_HINTS property entirely.
		platform.display.DeleteProperty(w.wid, xlib.AtomWmNormalHints)
	} else {
		hints := xlib.SizeHints{Flags: xlib.PMinSize}
		if w.minW > 0 {
			hints.MinWidth = int32(w.minW * scale)
		}
		if w.minH > 0 {
			hints.MinHeight = int32(w.minH * scale)
		}
		platform.display.SetWMNormalHints(w.wid, &hints)
	}
	platform.display.Flush()
}

func (w *Window) Draw(img image.Image) error {
	bmp, ok := graphics.ToBitmap(img, graphics.PixelFormatBGRA)
	if !ok {
		bmp = graphics.CopyToBitmap(img, graphics.PixelFormatBGRA, nil)
	}
	return w.drawImage(bmp)
}

var windowMap = map[xlib.Window]*Window{}

// TODO: process window event
func handleEvent(event xlib.Event) {
	// Entering a nested native dispatch expires any previous press context.
	moveResizePress = nativePress{}
	// Give the input method first refusal on every event: during composition it
	// consumes the keys it needs (candidate navigation, preedit editing) and we
	// must drop them. Unconsumed keys fall through to normal handling below.
	if platform != nil && platform.im != 0 && xlib.FilterEvent(&event, 0) {
		return
	}

	switch event.Type {
	case xlib.ClientMessage:
		ev := event.ClientMessageEvent()
		if ev.MessageType == platform.atoms.WM_PROTOCOLS && ev.L[0] != 0 {
			if xlib.Atom(ev.L[0]) == platform.atoms.WM_DELETE_WINDOW {
				if window, ok := windowMap[ev.Window]; ok {
					window.emitEvent(events.CloseEvent{})
				}
			} else if xlib.Atom(ev.L[0]) == platform.atoms._NET_WM_SYNC_REQUEST {
				if window, ok := windowMap[ev.Window]; ok {
					window.resizeSync.request(uint32(ev.L[2]), int32(ev.L[3]))
				}
			}
		}
	// ping dnd
	case xlib.ConfigureNotify:
		ev := event.ConfigureEvent()
		if window, ok := windowMap[ev.Window]; ok {
			sizeChanged := ev.Width != window.width || ev.Height != window.height
			if sizeChanged {
				window.width, window.height = ev.Width, ev.Height
				scale := currentScale()
				window.emitEvent(events.SizeEvent{
					Width:       float32(ev.Width) / scale,
					Height:      float32(ev.Height) / scale,
					PixelWidth:  float32(ev.Width),
					PixelHeight: float32(ev.Height),
				})
			}
			syncPaint := window.resizeSync.configured()
			if sizeChanged || syncPaint {
				// Full-frame painters must redraw every new drawable extent. This is
				// required even when the WM does not implement resize synchronization.
				window.schedulePaint()
			}
		}

	case xlib.Expose:
		ev := event.ExposeEvent()
		if window, ok := windowMap[ev.Window]; ok {
			window.schedulePaint()
		}
	case xlib.PropertyNotify:
		ev := event.PropertyEvent()
		if ev.Atom == platform.atoms.WM_STATE || ev.Atom == platform.atoms._NET_WM_STATE {
			if window := windowMap[ev.Window]; window != nil {
				window.notifyState()
			}
		}
	case xlib.MapNotify:
		if window := windowMap[event.MapEvent().Window]; window != nil {
			window.notifyState()
		}
	case xlib.UnmapNotify:
		if window := windowMap[event.UnmapEvent().Window]; window != nil {
			window.notifyState()
		}
	case xlib.SelectionClear:
		if platform.clipboard != nil {
			platform.clipboard.handleSelectionClear(event.SelectionClearEvent())
		}
	case xlib.SelectionRequest:
		if platform.clipboard != nil {
			platform.clipboard.handleSelectionRequest(event.SelectionRequestEvent())
		}
	case xlib.SelectionNotify:
		if platform.clipboard != nil {
			platform.clipboard.handleSelectionNotify(event.SelectionEvent())
		}
	case xlib.FocusIn:
		ev := event.AnyEvent()
		if window, ok := windowMap[ev.Window]; ok {
			window.emitEvent(events.FocusEvent{Focused: true})
		}
	case xlib.FocusOut:
		ev := event.AnyEvent()
		if window, ok := windowMap[ev.Window]; ok {
			window.emitEvent(events.FocusEvent{Focused: false})
		}
	case xlib.MotionNotify:
		ev := event.MotionEvent()
		if window, ok := windowMap[ev.Window]; ok {
			window.handlePointerMove(ev)
		}
	case xlib.EnterNotify:
		ev := event.CrossingEvent()
		if window, ok := windowMap[ev.Window]; ok {
			window.handlePointerCrossing(events.PointerEnter, ev)
		}
	case xlib.LeaveNotify:
		ev := event.CrossingEvent()
		if window, ok := windowMap[ev.Window]; ok {
			window.handlePointerCrossing(events.PointerLeave, ev)
		}
	case xlib.ButtonPress:
		ev := event.ButtonEvent()
		if window, ok := windowMap[ev.Window]; ok {
			window.handleButton(events.PointerDown, ev)
		}
	case xlib.ButtonRelease:
		ev := event.ButtonEvent()
		if window, ok := windowMap[ev.Window]; ok {
			window.handleButton(events.PointerUp, ev)
		}
	case xlib.KeyPress:
		ev := event.KeyEvent()
		if window, ok := windowMap[ev.Window]; ok {
			window.handleKey(events.KeyDown, ev)
		}
	case xlib.KeyRelease:
		ev := event.KeyEvent()
		if window, ok := windowMap[ev.Window]; ok {
			window.handleKey(events.KeyUp, ev)
		}
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

// Even None stays managed. No function mask is written: decoration must not
// disable native close, resize, minimize or maximize operations.
// Motif hints are requests, not observations of WM-owned decorations.
func (w *Window) applyDecoration(chrome common.WindowChrome) {
	if chrome != common.WindowChromeNone {
		return
	}
	hints := [5]uintptr{1 << 1} // decorations flag; hints[2] = no decorations
	platform.display.ChangeProperty(w.wid, platform.atoms._MOTIF_WM_HINTS,
		platform.atoms._MOTIF_WM_HINTS, 32, xlib.PropModeReplace, cgo.CSlice(hints[:]), len(hints))
}

func (w *Window) Chrome() common.WindowChrome {
	return common.WindowChromeUnknown
}

func (w *Window) ControlsRect() (geometry.Rectangle, error) {
	if w.wid == 0 {
		return geometry.Rectangle{}, common.ErrUnavailable
	}
	return geometry.Rectangle{}, common.ErrUnsupported
}

func (w *Window) SetControlsPosition(*geometry.Point) error {
	if w.wid == 0 {
		return common.ErrUnavailable
	}
	return common.ErrUnsupported
}

// windowProperty32 reads a bounded ATOM/CARDINAL-style property, copying
// native unsigned longs to protocol uint32s before freeing the Xlib buffer.
// A missing property returns nil, nil. Malformed, truncated or failed queries
// return an error rather than an apparently valid empty state.
func windowProperty32(d xlib.Display, w xlib.Window, property, reqType xlib.Atom, maxItems int) ([]uint32, error) {
	if maxItems <= 0 {
		return nil, fmt.Errorf("property item limit must be positive")
	}
	var (
		actualType         xlib.Atom
		actualFormat       int32
		nitems, bytesAfter uint
		prop               *byte
	)
	status := d.GetWindowProperty(w, property, 0, maxItems, false, reqType,
		&actualType, &actualFormat, &nitems, &bytesAfter, &prop)
	if prop != nil {
		defer xlib.Free(prop)
	}
	if status != 0 {
		return nil, fmt.Errorf("XGetWindowProperty: status %d", status)
	}
	if actualType == xlib.AtomNone {
		return nil, nil
	}
	if actualFormat != 32 || (reqType != xlib.AtomAny && actualType != reqType) ||
		bytesAfter != 0 || nitems > uint(maxItems) || (nitems > 0 && prop == nil) {
		return nil, fmt.Errorf("invalid or oversized X11 property %d (type %d, format %d)", property, actualType, actualFormat)
	}
	data := make([]uint32, int(nitems))
	for i, value := range cgo.GoSliceNTemp[uintptr](cgo.Pointer(prop), int(nitems)) {
		data[i] = uint32(value)
	}
	return data, nil
}

func (w *Window) SetHitTest(func(geometry.Point) common.WindowHit) error {
	if w.wid == 0 {
		return common.ErrUnavailable
	}
	return common.ErrUnsupported
}

func (w *Window) BeginMove() error {
	return w.beginMoveResize(8)
}

func (w *Window) BeginResize(edge common.WindowEdge) error {
	if w.wid == 0 {
		return common.ErrUnavailable
	}
	direction, ok := resizeDirection(edge)
	if !ok {
		return fmt.Errorf("invalid window edge: %d", edge)
	}
	return w.beginMoveResize(direction)
}

func resizeDirection(edge common.WindowEdge) (int64, bool) {
	switch edge {
	case common.WindowEdgeTopLeft:
		return 0, true
	case common.WindowEdgeTop:
		return 1, true
	case common.WindowEdgeTopRight:
		return 2, true
	case common.WindowEdgeRight:
		return 3, true
	case common.WindowEdgeBottomRight:
		return 4, true
	case common.WindowEdgeBottom:
		return 5, true
	case common.WindowEdgeBottomLeft:
		return 6, true
	case common.WindowEdgeLeft:
		return 7, true
	default:
		return 0, false
	}
}

func (w *Window) beginMoveResize(direction int64) error {
	if w.wid == 0 {
		return common.ErrUnavailable
	}
	if w.overrideRedirect {
		return common.ErrUnsupported
	}
	if moveResizePress.window != w {
		return common.ErrUnavailable
	}
	supported, err := windowProperty32(platform.display, platform.defScreen.Root,
		platform.atoms._NET_SUPPORTED, xlib.AtomAtom, 4096)
	if err != nil {
		return err
	}
	if platform.atoms._NET_WM_MOVERESIZE == 0 || !slices.Contains(supported, uint32(platform.atoms._NET_WM_MOVERESIZE)) {
		return common.ErrUnsupported
	}
	event := moveResizePress.event
	// ButtonPress holds an implicit pointer grab. Release it before asking the
	// WM to grab for its native move/resize loop, using the original press time.
	platform.display.UngrabPointer(event.Time)
	if !w.sendMoveResize(&event, direction) {
		return fmt.Errorf("send _NET_WM_MOVERESIZE failed")
	}
	moveResizePress = nativePress{}
	// The WM may consume the release. Do not keep a stale native button state.
	w.buttons &^= events.PointerButtonLeftDown
	w.moveResize = true
	return nil
}

func moveResizeMessage(window xlib.Window, atom xlib.Atom, event *xlib.ButtonEvent, direction int64) xlib.Event {
	var message xlib.Event
	*message.ClientMessageEvent() = xlib.ClientMessageEvent{
		Type: xlib.ClientMessage, Window: window, MessageType: atom, Format: 32,
		L: [5]int64{int64(event.XRoot), int64(event.YRoot), direction, int64(event.Button), 1},
	}
	return message
}

func (w *Window) sendMoveResize(event *xlib.ButtonEvent, direction int64) bool {
	message := moveResizeMessage(w.wid, platform.atoms._NET_WM_MOVERESIZE, event, direction)
	ok := platform.display.SendEvent(platform.defScreen.Root, false,
		xlib.EventMaskSubstructureRedirect|xlib.EventMaskSubstructureNotify, &message)
	platform.display.Flush()
	return ok != 0
}

func (w *Window) State() common.WindowState {
	if w.wid == 0 {
		return common.WindowStateUnknown
	}
	var attrs xlib.WindowAttributes
	if platform.display.GetWindowAttributes(w.wid, &attrs) == 0 {
		return common.WindowStateUnknown
	}
	wm, err := windowProperty32(platform.display, w.wid, platform.atoms.WM_STATE, platform.atoms.WM_STATE, 2)
	if err != nil {
		return common.WindowStateUnknown
	}
	if len(wm) == 0 || (len(wm) == 2 && wm[0] == 0) {
		if attrs.MapState == xlib.IsUnmapped {
			return common.WindowStateHidden
		}
		return common.WindowStateUnknown
	}
	if len(wm) != 2 {
		return common.WindowStateUnknown
	}
	if wm[0] == 3 {
		return common.WindowStateMinimized
	}
	if wm[0] != 1 {
		return common.WindowStateUnknown
	}
	// A normal WM_STATE can be unmapped on another workspace, not Hidden.
	states, err := windowProperty32(platform.display, w.wid, platform.atoms._NET_WM_STATE, xlib.AtomAtom, 4096)
	if err != nil {
		return common.WindowStateUnknown
	}
	return observedPresentation(states, platform.atoms._NET_WM_STATE_MAXIMIZED_HORZ,
		platform.atoms._NET_WM_STATE_MAXIMIZED_VERT, platform.atoms._NET_WM_STATE_FULLSCREEN)
}

func observedPresentation(states []uint32, horizontal, vertical, fullscreen xlib.Atom) common.WindowState {
	if states == nil {
		return common.WindowStateUnknown
	}
	if slices.Contains(states, uint32(fullscreen)) {
		return common.WindowStateFullscreen
	}
	if slices.Contains(states, uint32(horizontal)) && slices.Contains(states, uint32(vertical)) {
		return common.WindowStateMaximized
	}
	return common.WindowStateNormal
}

func (w *Window) RequestState(state common.WindowState) error {
	if w.wid == 0 {
		return common.ErrUnavailable
	}
	switch state {
	case common.WindowStateHidden:
		return w.Hide()
	case common.WindowStateMinimized:
		if w.State() == state {
			return nil
		}
		if platform.display.IconifyWindow(w.wid, platform.display.DefaultScreen()) == 0 {
			return fmt.Errorf("XIconifyWindow failed")
		}
		platform.display.Flush()
		return nil
	case common.WindowStateNormal, common.WindowStateMaximized, common.WindowStateFullscreen:
	default:
		return fmt.Errorf("invalid window state: %d", state)
	}
	supported, err := windowProperty32(platform.display, platform.defScreen.Root, platform.atoms._NET_SUPPORTED, xlib.AtomAtom, 4096)
	if err != nil {
		return err
	}
	has := func(atom xlib.Atom) bool { return atom != 0 && slices.Contains(supported, uint32(atom)) }
	maxSupported := has(platform.atoms._NET_WM_STATE) &&
		has(platform.atoms._NET_WM_STATE_MAXIMIZED_HORZ) && has(platform.atoms._NET_WM_STATE_MAXIMIZED_VERT)
	fullSupported := has(platform.atoms._NET_WM_STATE) && has(platform.atoms._NET_WM_STATE_FULLSCREEN)
	if (state == common.WindowStateMaximized && !maxSupported) ||
		(state == common.WindowStateFullscreen && !fullSupported) {
		return common.ErrUnsupported
	}
	if w.State() == state {
		return nil
	}
	// Validate support before mapping. X requests stay ordered: map/deiconify
	// first, then request the explicit target rather than historical restore.
	if err := w.Show(); err != nil {
		return err
	}
	// Leave fullscreen before setting the next presentation. A WM may restore
	// its pre-fullscreen state on exit, discarding a maximize request sent first.
	if fullSupported && state != common.WindowStateFullscreen {
		if err := w.sendWMState(false, platform.atoms._NET_WM_STATE_FULLSCREEN, 0); err != nil {
			return err
		}
	}
	if maxSupported {
		if err := w.sendWMState(state == common.WindowStateMaximized,
			platform.atoms._NET_WM_STATE_MAXIMIZED_HORZ, platform.atoms._NET_WM_STATE_MAXIMIZED_VERT); err != nil {
			return err
		}
	}
	if fullSupported && state == common.WindowStateFullscreen {
		if err := w.sendWMState(true, platform.atoms._NET_WM_STATE_FULLSCREEN, 0); err != nil {
			return err
		}
	}
	platform.display.Flush()
	return nil
}

func (w *Window) sendWMState(add bool, first, second xlib.Atom) error {
	var action int64
	if add {
		action = 1
	}
	var ev xlib.Event
	*ev.ClientMessageEvent() = xlib.ClientMessageEvent{
		Type: xlib.ClientMessage, Window: w.wid, MessageType: platform.atoms._NET_WM_STATE,
		Format: 32, L: [5]int64{action, int64(first), int64(second), 1, 0},
	}
	if platform.display.SendEvent(platform.defScreen.Root, false,
		xlib.EventMaskSubstructureRedirect|xlib.EventMaskSubstructureNotify, &ev) == 0 {
		return fmt.Errorf("send _NET_WM_STATE failed")
	}
	return nil
}

func (w *Window) notifyState() {
	state := w.State()
	if state != w.state {
		w.state = state
		w.emitEvent(events.StateEvent{State: state})
	}
}

var _ common.DesktopWindow = (*Window)(nil)
