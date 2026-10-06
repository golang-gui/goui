package cocoa

import (
	"image"
	"math"

	"github.com/golang-gui/goui/core/geometry"
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

func newPopup(owner common.Window, size geometry.Size, onEvent events.EventHandler, options common.PopupOptions) (common.Popup, error) {
	// newNativeWindow converts the requested logical content size to points.
	win := newNativeWindow(onEvent, popupClass, NSWindowStyleMaskBorderless, NSMakeRect(0, 0, CGFloat(size.Width), CGFloat(size.Height)), options.Transparent)
	AutoReleasePool(func() {
		win.window.SetLevel(popupWindowLevel) // float above ordinary windows
		// Popup decoration belongs to its content host; avoid a second OS shadow.
		win.window.SetHasShadow(false)
	})
	return &Popup{win: win, owner: owner}, nil
}

func (p *Popup) NativeHandle() uintptr      { return p.win.NativeHandle() }
func (p *Popup) Transparent() bool          { return p.win.Transparent() }
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
// content's top edge. The owner's scale maps logical coordinates to points.
func (p *Popup) SetPosition(x, y float32) {
	if !p.win.window.Valid() {
		return
	}
	AutoReleasePool(func() {
		var ownerWin NSWindow
		ownerWin.ID = ID(p.owner.NativeHandle())
		content := ownerWin.ContentRectForFrameRect(ownerWin.Frame())
		scale := pointsPerLogicalUnit(ownerWin)
		topLeft := NSPoint{
			X: content.Origin.X + CGFloat(x)*scale,
			Y: content.Origin.Y + content.Size.Height - CGFloat(y)*scale,
		}
		p.win.window.SetFrameTopLeftPoint(topLeft)
	})
}

func (p *Popup) SetSize(width, height float32) {
	if !p.win.window.Valid() {
		return
	}
	AutoReleasePool(func() {
		p.win.window.SetContentSize(popupContentSize(p.win.window, width, height))
	})
}

func popupContentSize(window NSWindow, width, height float32) NSSize {
	return popupSizeInPoints(width, height, window.BackingScaleFactor(), CGFloat(common.GetPreferScale()))
}

// popupSizeInPoints rounds outward in backing pixels, not logical units or
// AppKit points. A fractional text height must fit after native pixel alignment.
// Without an override a logical unit is an AppKit point; with one it spans
// preferScale backing pixels. The actual SizeEvent remains authoritative.
func popupSizeInPoints(width, height float32, backingScale, preferScale CGFloat) NSSize {
	scale := backingScale
	if preferScale > 0 {
		scale = preferScale
	}
	return NSSize{
		Width:  CGFloat(math.Ceil(float64(max(1, width))*float64(scale))) / backingScale,
		Height: CGFloat(math.Ceil(float64(max(1, height))*float64(scale))) / backingScale,
	}
}
