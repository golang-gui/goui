package cocoa

import (
	"image"

	"github.com/golang-gui/goui/platform/common"
	"github.com/golang-gui/goui/platform/events"

	. "github.com/golang-gui/goui/platform/darwin/frameworks/appkit"
	. "github.com/golang-gui/goui/platform/darwin/frameworks/core_graphics"
	. "github.com/golang-gui/goui/platform/darwin/frameworks/foundation"
)

// popupWindowLevel floats the popup above ordinary windows
// (NSPopUpMenuWindowLevel), so it is not clipped by its owner.
const popupWindowLevel NSInteger = 101

// Popup is a borderless NSWindow positioned relative to its owner's content
// area. It reuses the Window machinery (event routing, draw) and only adds
// borderless creation plus owner-content-relative positioning.
type Popup struct {
	win   *Window
	owner common.Window
}

func newPopup(owner common.Window, width, height float32, onEvent events.EventHandler) (common.Popup, error) {
	// Points equal goui logical units, so create at the authoritative size
	// directly (no scale conversion); this also right-sizes the GL drawable.
	win := newNativeWindow(onEvent, NSWindowStyleMaskBorderless, NSMakeRect(0, 0, CGFloat(width), CGFloat(height)))
	AutoReleasePool(func() {
		win.window.SetLevel(popupWindowLevel) // float above ordinary windows
	})
	return &Popup{win: win, owner: owner}, nil
}

func (p *Popup) NativeHandle() uintptr      { return p.win.NativeHandle() }
func (p *Popup) Destroy()                   { p.win.Destroy() }
func (p *Popup) Hide() error                { return p.win.Hide() }
func (p *Popup) RequestPaint() error        { return p.win.RequestPaint() }
func (p *Popup) Draw(img image.Image) error { return p.win.Draw(img) }

// Show orders the popup in front of its owner without activating the app, so the
// owner keeps key focus (click-menu behavior). Unlike Window.Show it does not
// make the popup key, so it cannot delegate to p.win.Show.
func (p *Popup) Show() error {
	if !p.win.window.Valid() {
		return nil
	}
	AutoReleasePool(func() {
		p.win.window.OrderFront(0)
	})
	return nil
}

// SetPosition places the popup at an owner-content-local logical point. goui uses
// a top-left origin with y growing downward; macOS screen coordinates use a
// bottom-left origin with y growing upward, so y is flipped against the owner
// content's top edge. macOS points already equal goui logical units, so no scale
// factor is applied.
func (p *Popup) SetPosition(x, y float32) {
	if !p.win.window.Valid() {
		return
	}
	AutoReleasePool(func() {
		var ownerWin NSWindow
		ownerWin.ID = ID(p.owner.NativeHandle())
		content := ownerWin.ContentRectForFrameRect(ownerWin.Frame())
		topLeft := NSPoint{
			X: content.Origin.X + CGFloat(x),
			Y: content.Origin.Y + content.Size.Height - CGFloat(y),
		}
		p.win.window.SetFrameTopLeftPoint(topLeft)
	})
}

func (p *Popup) SetSize(width, height float32) {
	if !p.win.window.Valid() {
		return
	}
	if width < 1 {
		width = 1
	}
	if height < 1 {
		height = 1
	}
	AutoReleasePool(func() {
		p.win.window.SetContentSize(NSSize{Width: CGFloat(width), Height: CGFloat(height)})
	})
}
