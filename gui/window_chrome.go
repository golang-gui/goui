package gui

import (
	"errors"
	"fmt"
	"log"
	"math"
	"time"

	"github.com/golang-gui/goui/core/geometry"
	"github.com/golang-gui/goui/core/signal"
	"github.com/golang-gui/goui/platform"
	"github.com/golang-gui/goui/platform/events"
)

// WindowChromeMode is GUI creation policy, not a native decoration observation.
type WindowChromeMode uint8

const (
	WindowChromeNative WindowChromeMode = iota
	WindowChromeIntegrated
	// WindowChromeNone is for special-purpose windows. Prefer Native or
	// Integrated; callers define drag/resize regions and their cursor feedback.
	WindowChromeNone
)

// WindowOptions contains initial preferences, not enforced window geometry.
// Zero size components use the GUI default. On X11, Integrated uses GUI-drawn
// decorations over a transparent None surface. If that capability is absent,
// it falls back to Native. No Widget root is inserted.
// Sizes use DIP under the backend's existing extent convention; layout always
// uses the actual client size, not the requested extent.
type WindowOptions struct {
	Size   geometry.Size
	Chrome WindowChromeMode
	// Transparent requires a native alpha surface; failure is returned, never
	// downgraded to opaque. The window style independently controls its background.
	Transparent bool
}

var defaultWindowOptions = WindowOptions{
	Size:   geometry.Size{Width: 800, Height: 600},
	Chrome: WindowChromeNative,
}

func (o WindowOptions) Validate() error {
	for _, v := range []float32{o.Size.Width, o.Size.Height} {
		if v < 0 || math.IsNaN(float64(v)) || math.IsInf(float64(v), 0) {
			return fmt.Errorf("invalid initial window size: %v", o.Size)
		}
	}
	if o.Chrome > WindowChromeNone {
		return fmt.Errorf("invalid window chrome mode: %d", o.Chrome)
	}
	return nil
}

func (o WindowOptions) normalized() WindowOptions {
	defaults := defaultWindowOptions
	if o.Size.Width == 0 {
		o.Size.Width = defaults.Size.Width
	}
	if o.Size.Height == 0 {
		o.Size.Height = defaults.Size.Height
	}
	return o
}

// ChromeRegion is a GUI role, independent of native hit-test constants.
type ChromeRegion uint8

const (
	// Default leaves existing window behavior (including native edges) intact.
	ChromeRegionDefault ChromeRegion = iota
	// Client explicitly keeps this region in ordinary GUI input.
	ChromeRegionClient
	ChromeRegionCaption
	ChromeRegionMinimize
	ChromeRegionMaximize
	ChromeRegionClose
	// Resize roles start an interactive resize. None callers own cursor feedback.
	ChromeRegionTop
	ChromeRegionBottom
	ChromeRegionLeft
	ChromeRegionRight
	ChromeRegionTopLeft
	ChromeRegionTopRight
	ChromeRegionBottomLeft
	ChromeRegionBottomRight
)

type ChromeControlsMode uint8

const (
	ChromeControlsNone ChromeControlsMode = iota
	ChromeControlsCustom
	ChromeControlsNative
)

// ChromeInfo is a value snapshot. Mode is the selected GUI policy. Enabled
// means region collaboration (Integrated or desktop None), not titlebar visibility.
// ControlsBounds is the occupied button union in client DIP for either
// presentation. A temporary native query failure preserves the last reported
// bounds without an extra notification. Disabled integration has no bounds.
type ChromeInfo struct {
	Mode           WindowChromeMode
	Enabled        bool
	Controls       ChromeControlsMode
	ControlsBounds geometry.Rectangle
}

// ChromeControls asks for the height of the available top row in DIP.
// Windows custom buttons fill this height; native and Linux circular controls
// keep their intrinsic size and are centered vertically.
// Zero means no row: native positioning resets, custom controls use their own
// height. Negative/non-finite answers are ignored.
type ChromeControls struct {
	Height float32
}

// WindowChrome is a window-owned signal service, not a Widget. Window owns
// its controls; Chrome coordinates them and HeaderBars through signals.
// All methods and callbacks run on the GUI thread.
type WindowChrome interface {
	// ConnectInfo connects, then synchronously supplies current info once.
	// Later emissions occur only on changes. After destruction it is inert.
	ConnectInfo(func(info ChromeInfo)) signal.Handle
	// ConnectQueryRegion participates in a synchronous client-DIP query.
	// Result starts at Default; callbacks run in connection order and later
	// callbacks may overwrite earlier answers. A non-hit leaves it untouched.
	// Read completed layout only: do not mutate, dispatch, destroy, or retain
	// the result pointer. No early-out or hidden subscriber priority exists.
	ConnectQueryRegion(func(point geometry.Point, region *ChromeRegion)) signal.Handle
	// ConnectQueryControls runs once after content layout to query the height
	// of the top row. HeaderBars answer from their completed allocations.
	// The same synchronous, ordered, borrowed-result rules apply.
	ConnectQueryControls(func(*ChromeControls)) signal.Handle
}

type chromeSignals struct {
	changed signal.Signal1[ChromeInfo]
	region  signal.Signal2[geometry.Point, *ChromeRegion]
	layout  signal.Signal1[*ChromeControls]
}

type windowChrome struct {
	window          *window
	native          platform.DesktopWindow
	info            ChromeInfo
	signals         *chromeSignals
	closed          bool
	nativeHit       bool
	querying        bool
	syncing         bool
	placementDirty  bool
	controlsHeight  float32         // last valid query answer, not a window property
	lastPosition    *geometry.Point // last native request, including failures
	nativeAvailable bool
	positioned      bool
}

func (w *window) Chrome() WindowChrome {
	if w.chrome == nil {
		w.chrome = &windowChrome{window: w, signals: &chromeSignals{}, closed: w.destroyed}
	}
	return w.chrome
}

func (c *windowChrome) live() bool {
	return !c.closed && c.window != nil && !c.window.destroyed
}

func (c *windowChrome) ConnectInfo(fn func(ChromeInfo)) signal.Handle {
	if !c.live() || fn == nil {
		return signal.Handles(nil)
	}
	h := c.signals.changed.Connect(func(info ChromeInfo) {
		if c.live() {
			fn(info)
		}
	})
	fn(c.info)
	return h
}

func (c *windowChrome) ConnectQueryRegion(fn func(geometry.Point, *ChromeRegion)) signal.Handle {
	if !c.live() || fn == nil {
		return signal.Handles(nil)
	}
	return c.signals.region.Connect(func(p geometry.Point, result *ChromeRegion) {
		if c.live() {
			fn(p, result)
		}
	})
}

func (c *windowChrome) ConnectQueryControls(fn func(*ChromeControls)) signal.Handle {
	if !c.live() || fn == nil {
		return signal.Handles(nil)
	}
	return c.signals.layout.Connect(func(result *ChromeControls) {
		if c.live() {
			fn(result)
		}
	})
}

func (c *windowChrome) initialize(mode WindowChromeMode) error {
	c.info.Mode = mode
	if mode == WindowChromeNative {
		return nil
	}
	native, err := c.window.desktopWindow()
	if err != nil {
		if mode == WindowChromeNone {
			return nil
		}
		return err
	}
	nativeChrome := native.Chrome()
	// X11 decoration hints are a request to the WM, not an observable promise;
	// its Chrome() correctly returns Unknown. Do not turn GUI policy into a
	// fabricated native observation.
	if mode == WindowChromeIntegrated && nativeChrome != platform.WindowChromeIntegrated &&
		!(c.window.clientChrome && (nativeChrome == platform.WindowChromeNone || nativeChrome == platform.WindowChromeUnknown)) {
		return fmt.Errorf("native window did not establish Integrated chrome")
	}
	c.native = native
	c.info.Enabled = true
	if c.window.clientChrome {
		// First subscriber supplies the frame default; later user subscribers
		// may override it, including returning Client to lock resizing.
		c.ConnectQueryRegion(c.queryFrameRegion)
	}
	if mode == WindowChromeIntegrated {
		c.info.Controls = ChromeControlsCustom
		// Probe an actual capability, not GOOS. This new window has no custom
		// position to reset; native geometry may still be unavailable before Show.
		err = native.SetControlsPosition(nil)
		if err == nil || errors.Is(err, platform.ErrUnavailable) {
			c.info.Controls = ChromeControlsNative
		} else if !errors.Is(err, platform.ErrUnsupported) {
			return fmt.Errorf("native controls capability: %w", err)
		}
	}
	err = native.SetHitTest(c.nativeRegion)
	if err == nil {
		c.nativeHit = true
	} else if !errors.Is(err, platform.ErrUnsupported) {
		return fmt.Errorf("native chrome input: %w", err)
	}
	c.window.dispatcher.hostController = &chromeInteractionController{chrome: c}
	c.refresh()
	return nil
}

func (c *windowChrome) refresh() {
	if !c.live() || !c.info.Enabled {
		return
	}
	if c.info.Controls != ChromeControlsNative {
		return
	}
	next := c.info
	r, err := c.native.ControlsRect()
	available := err == nil
	if available && !c.nativeAvailable {
		c.placementDirty = true
	}
	c.nativeAvailable = available
	if err == nil {
		next.ControlsBounds = r
	}
	c.publish(next.ControlsBounds)
}

func (c *windowChrome) publish(bounds geometry.Rectangle) {
	if bounds != c.info.ControlsBounds {
		c.info.ControlsBounds = bounds
		c.signals.changed.Emit(c.info)
	}
}

func (c *windowChrome) nativeChanged() {
	if !c.live() {
		return
	}
	c.placementDirty = true
	c.refresh()
	if c.live() && c.window.clientChrome {
		c.window.RequestLayout()
	}
}

func (c *windowChrome) queryRegion(p geometry.Point) ChromeRegion {
	if !c.live() || !c.info.Enabled || c.querying {
		return ChromeRegionDefault
	}
	w := c.window
	if w.modalTarget != nil || w.dispatcher.captureTarget != nil {
		return ChromeRegionClient
	}
	c.querying = true
	defer func() { c.querying = false }()
	result := ChromeRegionDefault
	c.signals.region.Emit(p, &result)
	if !c.live() || result > ChromeRegionBottomRight {
		return ChromeRegionDefault
	}
	return result
}

func (c *windowChrome) nativeRegion(p geometry.Point) platform.WindowHit {
	if !c.live() || c.syncing || c.window.layoutDirty || c.window.layingOut {
		return platform.WindowHitDefault
	}
	switch c.queryRegion(p) {
	case ChromeRegionClient:
		return platform.WindowHitClient
	case ChromeRegionCaption:
		return platform.WindowHitCaption
	case ChromeRegionMinimize:
		return platform.WindowHitMinimize
	case ChromeRegionMaximize:
		return platform.WindowHitMaximize
	case ChromeRegionClose:
		return platform.WindowHitClose
	case ChromeRegionTop:
		return platform.WindowHitTop
	case ChromeRegionBottom:
		return platform.WindowHitBottom
	case ChromeRegionLeft:
		return platform.WindowHitLeft
	case ChromeRegionRight:
		return platform.WindowHitRight
	case ChromeRegionTopLeft:
		return platform.WindowHitTopLeft
	case ChromeRegionTopRight:
		return platform.WindowHitTopRight
	case ChromeRegionBottomLeft:
		return platform.WindowHitBottomLeft
	case ChromeRegionBottomRight:
		return platform.WindowHitBottomRight
	default:
		return platform.WindowHitDefault
	}
}

// beforeLayout seeds the horizontal reservation before HeaderBars are
// arranged. The height query itself must wait for their actual allocations.
func (c *windowChrome) beforeLayout() {
	if !c.live() || !c.info.Enabled || c.syncing {
		return
	}
	c.syncing = true
	defer func() { c.syncing = false }()
	c.refresh()
	if c.live() && c.info.Controls == ChromeControlsCustom {
		c.placeCustom()
	}
}

// afterLayout returns whether the newly resolved region changed. Window may
// then perform one bounded reservation pass, never a recursive height query.
func (c *windowChrome) afterLayout() bool {
	if !c.live() || !c.info.Enabled || c.syncing {
		return false
	}
	c.syncing = true
	defer func() { c.syncing = false }()
	before := c.info.ControlsBounds
	var result ChromeControls
	c.signals.layout.Emit(&result)
	if !c.live() || result.Height < 0 || math.IsNaN(float64(result.Height)) || math.IsInf(float64(result.Height), 0) {
		return false
	}
	c.controlsHeight = result.Height
	if c.info.Controls == ChromeControlsCustom {
		c.placeCustom()
	} else if c.nativeAvailable {
		var position *geometry.Point
		size := c.info.ControlsBounds.Size
		if result.Height > 0 && size.Width > 0 && size.Height > 0 {
			position = &geometry.Point{X: 12, Y: max(0, (result.Height-size.Height)/2)}
		}
		same := position == nil && c.lastPosition == nil ||
			position != nil && c.lastPosition != nil && *position == *c.lastPosition
		if c.placementDirty || !same {
			c.placementDirty = false
			c.lastPosition = position
			if position != nil || c.positioned {
				err := c.native.SetControlsPosition(position)
				if err == nil {
					c.positioned = position != nil
				} else if !errors.Is(err, platform.ErrUnavailable) && !errors.Is(err, platform.ErrUnsupported) {
					log.Printf("goui: native controls placement: %v", err)
				}
				if !c.live() {
					return false
				}
				c.refresh()
			}
		}
	}
	return c.live() && c.info.ControlsBounds != before
}

func (c *windowChrome) placeCustom() {
	bounds := geometry.Rectangle{}
	if c.window.State() != WindowStateFullscreen {
		width := captionButtonWidth * 3
		height := c.controlsHeight
		if height <= 0 {
			height = captionButtonHeight
		}
		if c.window.clientChrome {
			width = circularControlSize*3 + circularControlGap*2
			if c.controlsHeight <= 0 {
				height = 40
			}
			bounds = geometry.Rect(max(0, c.window.width-width-circularControlInset), max(0, (height-circularControlSize)/2), width, circularControlSize).
				Intersect(geometry.Rect(0, 0, c.window.width, c.window.height))
		} else {
			bounds = geometry.Rect(max(0, c.window.width-width), 0, width, height).
				Intersect(geometry.Rect(0, 0, c.window.width, c.window.height))
		}
	}
	c.publish(bounds)
}

func (c *windowChrome) destroy() {
	if c.closed {
		return
	}
	c.closed = true
	if c.nativeHit {
		_ = c.native.SetHitTest(nil)
	}
	c.window.dispatcher.hostController = nil
	c.native = nil
	c.signals = nil // in-flight emissions retain their own snapshots; guards skip them
	c.window = nil
}

// This is a host EventController, not a native-event interception shortcut.
type chromeInteractionController struct {
	EventControllerBase
	chrome     *windowChrome
	pressed    bool
	clickAt    time.Time
	clickPos   geometry.Point
	lastClick  time.Time
	lastPos    geometry.Point
	suppressUp bool
}

// Private GUI policy until double-click preferences are exposed by Settings.
const captionClickInterval = 500 * time.Millisecond
const captionClickDistance float32 = 4

func captionNear(a, b geometry.Point) bool {
	return math.Abs(float64(a.X-b.X)) <= float64(captionClickDistance) && math.Abs(float64(a.Y-b.Y)) <= float64(captionClickDistance)
}

func (controller *chromeInteractionController) Reset() {
	controller.pressed, controller.suppressUp = false, false
	controller.lastClick = time.Time{}
}

func (controller *chromeInteractionController) HandleEvent(ctx EventContext) {
	controller.handleAt(ctx, time.Now())
}

func (controller *chromeInteractionController) handleAt(ctx EventContext, now time.Time) {
	c := controller.chrome
	if !c.live() || c.nativeHit || !c.info.Enabled {
		controller.Reset()
		return
	}
	event, ok := ctx.Event().(events.PointerEvent)
	if !ok {
		controller.Reset()
		return
	}
	if event.EventType == events.PointerUp && event.Button == events.PointerButtonLeft && controller.suppressUp {
		controller.suppressUp = false
		ctx.StopPropagation()
		return
	}
	if c.syncing || c.window.layoutDirty || c.window.layingOut {
		controller.Reset()
		return
	}
	if controller.pressed {
		if c.window.modalTarget != nil || c.window.dispatcher.captureTarget != nil {
			controller.Reset()
			return
		}
		if event.EventType == events.PointerMove {
			ctx.StopPropagation()
			if event.Buttons&events.PointerButtonLeftDown == 0 {
				controller.Reset()
				return
			}
			if !captionNear(controller.clickPos, event.Position) {
				controller.pressed = false
				controller.lastClick = time.Time{}
				controller.suppressUp = true
				// The press belongs to Caption even if the native move is refused;
				// never send content a release for a press it did not receive.
				_ = c.native.BeginMove()
			}
			return
		}
		if event.EventType == events.PointerUp && event.Button == events.PointerButtonLeft {
			controller.pressed = false
			ctx.StopPropagation()
			if now.Sub(controller.clickAt) <= captionClickInterval && captionNear(controller.clickPos, event.Position) && c.queryRegion(event.Position) == ChromeRegionCaption {
				controller.lastClick, controller.lastPos = controller.clickAt, event.Position
			} else {
				controller.lastClick = time.Time{}
			}
			return
		}
	}
	if event.EventType != events.PointerDown {
		return
	}
	controller.pressed, controller.suppressUp = false, false
	if event.Button != events.PointerButtonLeft {
		controller.Reset()
		return
	}
	region := c.queryRegion(event.Position)
	if !c.live() {
		return
	}
	var err error
	switch {
	case region == ChromeRegionCaption:
		ctx.StopPropagation()
		if !controller.lastClick.IsZero() && now.Sub(controller.lastClick) >= 0 && now.Sub(controller.lastClick) <= captionClickInterval && captionNear(controller.lastPos, event.Position) {
			controller.lastClick = time.Time{}
			controller.suppressUp = true
			switch c.window.State() {
			case WindowStateNormal:
				c.window.RequestState(WindowStateMaximized)
			case WindowStateMaximized:
				c.window.RequestState(WindowStateNormal)
			}
		} else {
			controller.lastClick = time.Time{}
			controller.pressed = true
			controller.clickAt, controller.clickPos = now, event.Position
		}
		return
	case region >= ChromeRegionTop && region <= ChromeRegionBottomRight:
		controller.Reset()
		err = c.native.BeginResize(platform.WindowEdge(region - ChromeRegionTop))
	default:
		controller.Reset()
		return
	}
	if err == nil {
		ctx.StopPropagation()
	}
}
