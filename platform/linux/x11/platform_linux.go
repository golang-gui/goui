package x11

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"github.com/golang-gui/goui/core/geometry"
	"github.com/golang-gui/goui/platform/internal/desktopopen"
	"github.com/golang-gui/goui/platform/linux/libs/glib"

	"github.com/golang-gui/goui/platform/common"
	"github.com/golang-gui/goui/platform/events"
	"github.com/golang-gui/goui/platform/graphics"
	"github.com/golang-gui/goui/platform/graphics/opengl"
	"github.com/golang-gui/goui/platform/graphics/software"
	"github.com/golang-gui/goui/platform/linux/libs/libc"
	"github.com/golang-gui/goui/platform/linux/libs/xlib"
	"github.com/golang-gui/goui/platform/linux/libs/xsync"
	"github.com/golang-gui/goui/platform/typography"
	"github.com/golang-gui/goui/platform/typography/pango"
)

type Platform struct {
	appId        string
	instanceName string
	display      xlib.Display
	atoms        struct {
		UTF8_STRING                  xlib.Atom
		WM_STATE                     xlib.Atom
		WM_PROTOCOLS                 xlib.Atom
		WM_DELETE_WINDOW             xlib.Atom
		_NET_SUPPORTED               xlib.Atom
		_NET_WM_STATE                xlib.Atom
		_NET_WM_MOVERESIZE           xlib.Atom
		_NET_WM_STATE_MAXIMIZED_HORZ xlib.Atom
		_NET_WM_STATE_MAXIMIZED_VERT xlib.Atom
		_NET_WM_STATE_FULLSCREEN     xlib.Atom
		_MOTIF_WM_HINTS              xlib.Atom
		_NET_WM_SYNC_REQUEST         xlib.Atom
		_NET_WM_SYNC_REQUEST_COUNTER xlib.Atom
		_NET_WM_NAME                 xlib.Atom
		_NET_WM_ICON                 xlib.Atom
		_NET_WM_ICON_NAME            xlib.Atom
		CLIPBOARD                    xlib.Atom
		TARGETS                      xlib.Atom
		GOUI_CLIPBOARD               xlib.Atom
	}
	defScreen           *xlib.Screen
	helper              xlib.Window
	clipboard           *clipboard
	numLockMask         uint32
	im                  xlib.XIM // display input method; 0 when none is available
	resizeSyncAvailable bool
	eventLoop           *EventLoop
	cursorTheme         *cursorTheme
}

var platform *Platform

func NewPlatform(appId string) (_ *Platform, err error) {
	if platform != nil {
		if platform.appId != appId {
			return nil, fmt.Errorf("x11: platform already created with application ID %q, requested %q", platform.appId, appId)
		}
		return platform, nil
	}

	executable, err := os.Executable()
	if err != nil {
		return nil, fmt.Errorf("x11: application executable: %w", err)
	}
	p := &Platform{appId: appId, instanceName: filepath.Base(executable)}
	p.display = xlib.OpenDisplay("")
	if p.display == 0 {
		return nil, errors.New("can not open display")
	}

	// intern atoms
	p.atoms.UTF8_STRING = p.display.InternAtom("UTF8_STRING", false)
	p.atoms.WM_STATE = p.display.InternAtom("WM_STATE", false)
	p.atoms.WM_PROTOCOLS = p.display.InternAtom("WM_PROTOCOLS", false)
	p.atoms.WM_DELETE_WINDOW = p.display.InternAtom("WM_DELETE_WINDOW", false)
	p.atoms._NET_SUPPORTED = p.display.InternAtom("_NET_SUPPORTED", false)
	p.atoms._NET_WM_STATE = p.display.InternAtom("_NET_WM_STATE", false)
	p.atoms._NET_WM_MOVERESIZE = p.display.InternAtom("_NET_WM_MOVERESIZE", false)
	p.atoms._NET_WM_STATE_MAXIMIZED_HORZ = p.display.InternAtom("_NET_WM_STATE_MAXIMIZED_HORZ", false)
	p.atoms._NET_WM_STATE_MAXIMIZED_VERT = p.display.InternAtom("_NET_WM_STATE_MAXIMIZED_VERT", false)
	p.atoms._NET_WM_STATE_FULLSCREEN = p.display.InternAtom("_NET_WM_STATE_FULLSCREEN", false)
	p.atoms._MOTIF_WM_HINTS = p.display.InternAtom("_MOTIF_WM_HINTS", false)
	p.atoms._NET_WM_SYNC_REQUEST = p.display.InternAtom("_NET_WM_SYNC_REQUEST", false)
	p.atoms._NET_WM_SYNC_REQUEST_COUNTER = p.display.InternAtom("_NET_WM_SYNC_REQUEST_COUNTER", false)
	p.atoms._NET_WM_NAME = p.display.InternAtom("_NET_WM_NAME", false)
	p.atoms._NET_WM_ICON = p.display.InternAtom("_NET_WM_ICON", false)
	p.atoms._NET_WM_ICON_NAME = p.display.InternAtom("_NET_WM_ICON_NAME", false)
	p.atoms.CLIPBOARD = p.display.InternAtom("CLIPBOARD", false)
	p.atoms.TARGETS = p.display.InternAtom("TARGETS", false)
	p.atoms.GOUI_CLIPBOARD = p.display.InternAtom("GOUI_CLIPBOARD", false)

	p.defScreen = p.display.DefaultScreenOfDisplay()
	p.resizeSyncAvailable = xsync.Initialize(p.display) == nil
	p.numLockMask = p.detectNumLockMask()

	// Input method (IME): set the C locale from the environment, wire the
	// XMODIFIERS-based input-method selection, then open the display's IM. A nil
	// IM leaves physical key events available, but does not synthesize text.
	libc.SetLocale(libc.LC_CTYPE, "")
	xlib.SetLocaleModifiers("")
	p.im = xlib.OpenIM(p.display)
	if p.im != 0 && !p.im.SetDestroyCallback(inputMethodDestroyCallback) {
		// Do not retain an IM whose lifetime we cannot observe safely.
		p.im.Close()
		p.im = 0
	}

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
	loop, err := newEventLoop()
	if err != nil {
		return nil, err
	}
	p.eventLoop = loop
	return loop, nil
}

func (p *Platform) NewWindow(size geometry.Size, handler events.EventHandler, options common.WindowOptions) (common.Window, error) {
	return newWindow(size, handler, options)
}

func (p *Platform) NewPopup(owner common.Window, size geometry.Size, handler events.EventHandler, options common.PopupOptions) (common.Popup, error) {
	return newPopup(owner, size, handler, options)
}

func (p *Platform) NewImage(width, height uint) (common.Image, error) {
	return graphics.MakeBitmap(0, 0, int(width), int(height), graphics.PixelFormatBGRA, nil), nil
}

func (p *Platform) NewTypography() (typography.Context, error) {
	return pango.NewContext()
}

func (p *Platform) NewPainter(surface common.Surface) (painter graphics.Painter, err error) {
	switch common.GetPreferPainter() {
	case "software":
		return software.NewPainter(surface)
	default:
		if painter, err = opengl.NewPainter(surface); err != nil {
			return software.NewPainter(surface)
		}
		return
	}
}

func (p *Platform) NewInputMethod(window common.Window, handler common.InputMethodHandler) (common.InputMethod, error) {
	im, err := p.newInputMethod(window, handler)
	if err != nil {
		return nil, err
	}
	return im, nil
}

// NewCursor creates an x11 cursor capability for window.
func (p *Platform) NewCursor(window common.Window) (common.Cursor, error) {
	return p.newCursor(window)
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

func (p *Platform) NewFileDialog() (common.FileDialog, error) {
	return newFileDialog()
}

func (p *Platform) OpenURL(rawURL string) error {
	if err := desktopopen.ValidateURL(rawURL); err != nil {
		return err
	}
	if err := glib.AppInfoLaunchDefaultForURI(rawURL, 0); err != nil {
		return fmt.Errorf("open URL: %w", err)
	}
	return nil
}

func (p *Platform) OpenPath(path string) error {
	abs, err := desktopopen.AbsolutePath(path)
	if err != nil {
		return err
	}
	uri, err := glib.FilenameToURI(abs)
	if err != nil {
		return fmt.Errorf("open path: %w", err)
	}
	if err := glib.AppInfoLaunchDefaultForURI(uri, 0); err != nil {
		return fmt.Errorf("open path: %w", err)
	}
	return nil
}
