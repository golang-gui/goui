package x11

import (
	"image"

	"github.com/golang-gui/goui/platform/common"
	"github.com/golang-gui/goui/platform/events"

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

func newPopup(owner common.Window, width, height float32, onEvent events.EventHandler) (common.Popup, error) {
	// override-redirect: borderless, topmost, no taskbar entry, WM-bypassing.
	// Create at the authoritative size so the GL context binds a correctly-sized
	// drawable (no 1x1-then-resize).
	scale := currentScale()
	win, err := newNativeWindow(onEvent, true, physical(width, scale), physical(height, scale))
	if err != nil {
		return nil, err
	}
	return &Popup{win: win, owner: owner}, nil
}

func (p *Popup) NativeHandle() uintptr { return p.win.NativeHandle() }

// NativeFBConfig forwards the underlying window's GLX FBConfig so the OpenGL
// painter can build a matching context for the popup (x11 duck-types this).
func (p *Popup) NativeFBConfig() glx.FBConfig { return p.win.NativeFBConfig() }

func (p *Popup) Destroy()                   { p.win.Destroy() }
func (p *Popup) Show() error                { return p.win.Show() }
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
	w := int(width * scale)
	h := int(height * scale)
	if w < 1 {
		w = 1
	}
	if h < 1 {
		h = 1
	}
	platform.display.ResizeWindow(p.win.wid, uint(w), uint(h))
	platform.display.Flush()
}
