package x11

import (
	"errors"

	"github.com/golang-gui/goui/platform/common"
	"github.com/golang-gui/goui/platform/events"
	"github.com/golang-gui/goui/platform/graphics"
	"github.com/golang-gui/goui/platform/graphics/opengl"
	"github.com/golang-gui/goui/platform/graphics/software"
	"github.com/golang-gui/goui/platform/linux/libs/libc"
	"github.com/golang-gui/goui/platform/linux/libs/xlib"
	"github.com/golang-gui/goui/platform/typography"
	"github.com/golang-gui/goui/platform/typography/pango"
)

type Platform struct {
	display xlib.Display
	atoms   struct {
		UTF8_STRING       xlib.Atom
		WM_STATE          xlib.Atom
		WM_PROTOCOLS      xlib.Atom
		WM_DELETE_WINDOW  xlib.Atom
		_NET_WM_NAME      xlib.Atom
		_NET_WM_ICON      xlib.Atom
		_NET_WM_ICON_NAME xlib.Atom
		CLIPBOARD         xlib.Atom
		TARGETS           xlib.Atom
		GOUI_CLIPBOARD    xlib.Atom
	}
	defScreen   *xlib.Screen
	helper      xlib.Window
	clipboard   *clipboard
	numLockMask uint32
	im          xlib.XIM // display input method; 0 when none is available
}

var platform *Platform

func NewPlatform() (_ *Platform, err error) {
	if platform != nil {
		return platform, nil
	}

	p := new(Platform)
	p.display = xlib.OpenDisplay("")
	if p.display == 0 {
		return nil, errors.New("can not open display")
	}

	// intern atoms
	p.atoms.UTF8_STRING = p.display.InternAtom("UTF8_STRING", false)
	p.atoms.WM_STATE = p.display.InternAtom("WM_STATE", false)
	p.atoms.WM_PROTOCOLS = p.display.InternAtom("WM_PROTOCOLS", false)
	p.atoms.WM_DELETE_WINDOW = p.display.InternAtom("WM_DELETE_WINDOW", false)
	p.atoms._NET_WM_NAME = p.display.InternAtom("_NET_WM_NAME", false)
	p.atoms._NET_WM_ICON = p.display.InternAtom("_NET_WM_ICON", false)
	p.atoms._NET_WM_ICON_NAME = p.display.InternAtom("_NET_WM_ICON_NAME", false)
	p.atoms.CLIPBOARD = p.display.InternAtom("CLIPBOARD", false)
	p.atoms.TARGETS = p.display.InternAtom("TARGETS", false)
	p.atoms.GOUI_CLIPBOARD = p.display.InternAtom("GOUI_CLIPBOARD", false)

	p.defScreen = p.display.DefaultScreenOfDisplay()
	p.numLockMask = p.detectNumLockMask()

	// Input method (IME): set the C locale from the environment, wire the
	// XMODIFIERS-based input-method selection, then open the display's IM. A nil
	// IM (no server / unset XMODIFIERS) degrades to plain keysym translation.
	libc.SetLocale(libc.LC_CTYPE, "")
	xlib.SetLocaleModifiers("")
	p.im = xlib.OpenIM(p.display)

	p.helper = p.display.CreateWindow(p.defScreen.Root, 0, 0, 1, 1, 0,
		int(p.defScreen.RootDepth), xlib.WindowClassInputOutput, p.defScreen.RootVisual, 0, nil)

	err = opengl.InitGLX(p.display)
	if err != nil {
		// TODO: add log
	}

	platform = p
	return platform, nil
}

func (p *Platform) detectNumLockMask() uint32 {
	keycode := p.display.KeysymToKeycode(xlib.XK_Num_Lock)
	if keycode == 0 {
		return xlib.Mod2Mask
	}

	mapping := p.display.GetModifierMapping()
	if mapping == nil {
		return xlib.Mod2Mask
	}
	defer xlib.FreeModifiermap(mapping)

	keycodes := mapping.Keycodes()
	if len(keycodes) == 0 {
		return xlib.Mod2Mask
	}

	masks := [...]uint32{
		xlib.ShiftMask,
		xlib.LockMask,
		xlib.ControlMask,
		xlib.Mod1Mask,
		xlib.Mod2Mask,
		xlib.Mod3Mask,
		xlib.Mod4Mask,
		xlib.Mod5Mask,
	}
	maxKeypermod := int(mapping.MaxKeypermod)
	for modifier, mask := range masks {
		for index := 0; index < maxKeypermod; index++ {
			if keycodes[modifier*maxKeypermod+index] == keycode {
				return mask
			}
		}
	}

	return xlib.Mod2Mask
}

func (p *Platform) Name() string {
	return "x11"
}

func (p *Platform) Destroy() {
	// TODO
}

func (p *Platform) NewEventLoop() (common.EventLoop, error) {
	return newEventLoop()
}

func (p *Platform) NewWindow(width, height float32, handler events.EventHandler) (common.Window, error) {
	return newWindow(width, height, handler)
}

func (p *Platform) NewPopup(owner common.Window, width, height float32, handler events.EventHandler) (common.Popup, error) {
	return newPopup(owner, width, height, handler)
}

func (p *Platform) NewImage(width, height uint) (common.Image, error) {
	return graphics.MakeBitmap(0, 0, int(width), int(height), graphics.PixelFormatBGRA, nil), nil
}

func (p *Platform) NewTypography() (typography.Context, error) {
	return pango.NewContext()
}

func (p *Platform) NewPainter(surface common.Surface, typo typography.Context) (painter graphics.Painter, err error) {
	// TODO: error log
	painter, err = opengl.NewPainter(surface, typo)
	if err != nil {
		return software.NewPainter(surface, typo)
	}
	return
}

func (p *Platform) NewInputMethod(window common.Window, handler common.InputMethodHandler) (common.InputMethod, error) {
	return p.newInputMethod(window, handler)
}

func (p *Platform) NewSettings() (common.Settings, error) {
	return newSettings()
}

func (p *Platform) NewClipboard() (common.Clipboard, error) {
	if p.clipboard == nil {
		p.clipboard = newClipboard()
	}
	return p.clipboard, nil
}
