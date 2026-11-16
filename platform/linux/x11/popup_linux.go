package x11

import (
	"fmt"
	"image"
	"math"

	"github.com/golang-gui/goui/core/geometry"
	"github.com/golang-gui/goui/platform/common"
	"github.com/golang-gui/goui/platform/events"
	"github.com/golang-gui/goui/platform/internal/xerrors"

	"github.com/golang-gui/goui/platform/linux/libs/glx"
	"github.com/golang-gui/goui/platform/linux/libs/xlib"
)

// Popup is a borderless override-redirect window positioned relative to its
// owner. It reuses the Window machinery (event routing, draw) and only adds
// override-redirect creation plus owner-relative positioning.
type Popup struct {
	win   *Window
	owner common.Window
}

func newPopup(owner common.Window, size geometry.Size, onEvent events.EventHandler, options common.PopupOptions) (common.Popup, error) {
	// override-redirect: borderless, topmost, no taskbar entry, WM-bypassing.
	// Create at the authoritative size so the GL context binds a correctly-sized
	// drawable (no 1x1-then-resize).
	scale := currentScale()
	win, err := newNativeWindow(onEvent, true, popupPhysicalExtent(size.Width, scale), popupPhysicalExtent(size.Height, scale), options.Transparent)
	if err != nil {
		return nil, err
	}
	return &Popup{win: win, owner: owner}, nil
}

func (p *Popup) NativeHandle() uintptr { return p.win.NativeHandle() }
func (p *Popup) Transparent() bool     { return p.win.Transparent() }

// NativeFBConfig forwards the underlying window's GLX FBConfig so the OpenGL
// painter can build a matching context for the popup (x11 duck-types this).
func (p *Popup) NativeFBConfig() glx.FBConfig { return p.win.NativeFBConfig() }

func (p *Popup) Destroy()                   { p.win.Destroy() }
func (p *Popup) Show() error                { return p.win.mapWindow() }
func (p *Popup) Hide() error                { return p.win.Hide() }
func (p *Popup) RequestPaint() error        { return p.win.RequestPaint() }
func (p *Popup) Draw(img image.Image) error { return p.win.Draw(img) }

// SetPosition places the popup at an owner-window-local logical point, converted
// to root (screen) coordinates via the owner's position.
func (p *Popup) SetPosition(x, y float32) {
	if p.win.wid == 0 {
		return
	}
	scale := currentScale()
	ownerWid := xlib.Window(p.owner.NativeHandle())
	ox, oy := platform.display.TranslateCoordinates(ownerWid, platform.defScreen.Root, 0, 0)
	platform.display.MoveWindow(p.win.wid, ox+int(x*scale), oy+int(y*scale))
	platform.display.Flush()
}

func (p *Popup) SetSize(width, height float32) {
	if p.win.wid == 0 {
		return
	}
	scale := currentScale()
	w := popupPhysicalExtent(width, scale)
	h := popupPhysicalExtent(height, scale)
	platform.display.ResizeWindow(p.win.wid, uint(w), uint(h))
	platform.display.Flush()
}

func (p *Popup) ClientSize() (logical, pixels geometry.Size, err error) {
	if p.win.wid == 0 {
		return logical, pixels, common.ErrUnavailable
	}
	var attrs xlib.WindowAttributes
	var missing bool
	trap := xerrors.Begin(platform.display, []xlib.ID{xlib.ID(p.win.wid)}, &missing, xlib.BadWindow, xlib.BadDrawable)
	status := platform.display.GetWindowAttributes(p.win.wid, &attrs)
	trap.End()
	if status == 0 || missing {
		return logical, pixels, fmt.Errorf("query popup client size: %w", common.ErrUnavailable)
	}
	// This reply is newer than queued ConfigureNotify events. Do not let those
	// old observations resize/reposition a different presenter after Show.
	p.win.geometrySerial = uint64(platform.display.LastKnownRequestProcessed())
	p.win.width, p.win.height = attrs.Width, attrs.Height
	scale := currentScale()
	return geometry.Size{Width: float32(attrs.Width) / scale, Height: float32(attrs.Height) / scale},
		geometry.Size{Width: float32(attrs.Width), Height: float32(attrs.Height)}, nil
}

func (p *Popup) Repaint() error {
	if p.win.wid == 0 {
		return common.ErrUnavailable
	}
	p.win.emitEvent(events.PaintEvent{})
	if p.win.wid == 0 {
		return common.ErrUnavailable
	}
	platform.display.Flush()
	return nil
}

// popupPhysicalExtent rounds outward so the native client area can contain
// the measured logical content. Truncation can spuriously enable both bars
// in an exact-fit menu. Creation and resizing must use the same conversion.
func popupPhysicalExtent(logical, scale float32) int {
	return max(1, int(math.Ceil(float64(logical)*float64(scale))))
}

func (p *Popup) SurfaceState() common.SurfaceState {
	return common.SurfaceState{Ready: p.NativeHandle() != 0, Generation: 1}
}
