package gui

import (
	"errors"
	"fmt"
	"log"
	"math"

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
	WindowChromeNone
)

// WindowOptions contains initial preferences, not enforced window geometry.
// Zero size components use the GUI default. Integrated falls back to Native
// only when native creation reports unsupported. No Widget root is inserted.
// Sizes use DIP under the backend's existing extent convention; layout always
// uses the actual client size, not the requested extent.
type WindowOptions struct {
	Size   geometry.Size
	Chrome WindowChromeMode
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
	ChromeRegionDrag
	ChromeRegionMinimize
	ChromeRegionMaximize
	ChromeRegionClose
)

type ChromeControlsMode uint8

const (
	ChromeControlsNone ChromeControlsMode = iota
	ChromeControlsCustom
	ChromeControlsNative
)

// ChromeInfo is a value snapshot. Mode is the selected GUI policy. Enabled
// means integrated collaboration, not visibility of a titlebar in fullscreen.
// NativeBounds is the observed button union in client DIP; consult
// NativeBoundsAvailable before using it. An unavailable query is not absence.
type ChromeInfo struct {
	Mode                  WindowChromeMode
	Enabled               bool
	Controls              ChromeControlsMode
	NativeBounds          geometry.Rectangle
	NativeBoundsAvailable bool
}

// ChromeControls is the preferred native button-group origin in client
// DIP. HasPosition distinguishes no adapter/layout from a real (0,0) request.
type ChromeControls struct {
	Position    geometry.Point
	HasPosition bool
}

// WindowChrome is a window-owned signal service, not a Widget. It never owns
// HeaderBar/WindowControls objects or an independent Widget hit-test protocol.
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
	// ConnectQueryControls is emitted after layout for native controls.
	// At most one WindowControls should supply a native group per window.
	// The same synchronous, ordered, borrowed-result rules apply.
	ConnectQueryControls(func(*ChromeControls)) signal.Handle
}

type chromeSignals struct {
	changed signal.Signal1[ChromeInfo]
	region  signal.Signal2[geometry.Point, *ChromeRegion]
	layout  signal.Signal1[*ChromeControls]
}

type windowChrome struct {
	window         *window
	native         platform.DesktopWindow
	info           ChromeInfo
	signals        *chromeSignals
	closed         bool
	nativeHit      bool
	querying       bool
	syncing        bool
	placementDirty bool
	lastLayout     ChromeControls
	positioned     bool
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
	if mode != WindowChromeIntegrated {
		return nil
	}
	native, err := c.window.desktopWindow()
	if err != nil {
		return err
	}
	if native.Chrome() != platform.WindowChromeIntegrated {
		return fmt.Errorf("native window did not establish Integrated chrome")
	}
	c.native = native
	c.info.Enabled = true
	c.info.Controls = ChromeControlsCustom
	// Probe an actual capability, not GOOS. This new window has no custom
	// position to reset; native geometry may still be unavailable before Show.
	err = native.SetControlsPosition(nil)
	if err == nil || errors.Is(err, platform.ErrUnavailable) {
		c.info.Controls = ChromeControlsNative
	} else if !errors.Is(err, platform.ErrUnsupported) {
		return fmt.Errorf("native controls capability: %w", err)
	}
	err = native.SetHitTest(c.nativeRegion)
	if err == nil {
		c.nativeHit = true
	} else if !errors.Is(err, platform.ErrUnsupported) {
		return fmt.Errorf("native chrome input: %w", err)
	}
	c.window.dispatcher.hostController = &chromeMoveController{chrome: c}
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
	next.NativeBoundsAvailable = err == nil
	if err == nil {
		next.NativeBounds = r
	}
	if next != c.info {
		c.info = next
		c.signals.changed.Emit(next)
	}
}

func (c *windowChrome) nativeChanged() {
	if !c.live() {
		return
	}
	c.placementDirty = true
	c.refresh()
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
	if !c.live() || result > ChromeRegionClose {
		return ChromeRegionDefault
	}
	return result
}

func (c *windowChrome) nativeRegion(p geometry.Point) platform.WindowHit {
	if !c.live() || c.window.layoutDirty || c.window.layingOut {
		return platform.WindowHitDefault
	}
	switch c.queryRegion(p) {
	case ChromeRegionClient:
		return platform.WindowHitClient
	case ChromeRegionDrag:
		return platform.WindowHitCaption
	case ChromeRegionMinimize:
		return platform.WindowHitMinimize
	case ChromeRegionMaximize:
		return platform.WindowHitMaximize
	case ChromeRegionClose:
		return platform.WindowHitClose
	default:
		return platform.WindowHitDefault
	}
}

// afterLayout is the only source of native placement queries. Native queries
// and Widget Paint/Arrange never mutate AppKit. No recursive layout is run.
func (c *windowChrome) afterLayout() {
	if !c.live() || c.info.Controls != ChromeControlsNative || c.syncing {
		return
	}
	c.syncing = true
	defer func() { c.syncing = false }()
	var result ChromeControls
	c.signals.layout.Emit(&result)
	if !c.live() {
		return
	}
	if !c.placementDirty && result == c.lastLayout {
		return
	}
	c.placementDirty = false
	c.lastLayout = result
	var err error
	if result.HasPosition {
		err = c.native.SetControlsPosition(&result.Position)
		if err == nil {
			c.positioned = true
		}
	} else if c.positioned {
		err = c.native.SetControlsPosition(nil)
		if err == nil {
			c.positioned = false
		}
	}
	if err != nil && !errors.Is(err, platform.ErrUnavailable) && !errors.Is(err, platform.ErrUnsupported) {
		log.Printf("goui: native controls placement: %v", err)
	}
	c.refresh()
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
type chromeMoveController struct {
	EventControllerBase
	chrome *windowChrome
}

func (controller *chromeMoveController) HandleEvent(ctx EventContext) {
	c := controller.chrome
	if !c.live() || c.nativeHit || !c.info.Enabled {
		return
	}
	event, ok := ctx.Event().(events.PointerEvent)
	if !ok || event.EventType != events.PointerDown || event.Button != events.PointerButtonLeft {
		return
	}
	if c.window.layoutDirty || c.window.layingOut {
		return
	}
	if c.queryRegion(event.Position) != ChromeRegionDrag || !c.live() {
		return
	}
	if err := c.native.BeginMove(); err == nil {
		ctx.StopPropagation()
	}
}
