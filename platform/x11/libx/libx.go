package libx

import (
	"github.com/jezek/xgb"
	"github.com/jezek/xgb/xproto"
	"net"
	"unsafe"
)

type Display struct {
	conn  *xgb.Conn
	setup *xproto.SetupInfo
}

func OpenDisplay(name string) (d *Display, err error) {
	display := new(Display)
	display.conn, err = xgb.NewConnDisplay(name)
	if err != nil {
		return nil, err
	}

	display.setup = xproto.Setup(display.conn)
	return display, nil
}

func GetEventChan(display *Display) <-chan any {
	type hackXConn struct {
		host                string
		conn                net.Conn
		display             string
		DisplayNumber       int
		DefaultScreen       int
		SetupBytes          []byte
		setupResourceIdBase uint32
		setupResourceIdMask uint32
		eventChan           chan any
	}
	conn := (*hackXConn)(unsafe.Pointer(display.conn))
	return conn.eventChan
}

type (
	Screen   = xproto.ScreenInfo
	Screenid = uint32
	Visualid = xproto.Visualid
	Atom     = xproto.Atom
	Window   = xproto.Window
	Drawable = xproto.Drawable
	Gcontext = xproto.Gcontext
	Pixmap   = xproto.Pixmap
	Colormap = xproto.Colormap
	Cursor   = xproto.Cursor
)

const XA_ATOM Atom = xproto.AtomAtom

func DefaultScreen(display *Display) Screenid {
	return uint32(display.conn.DefaultScreen)
}

func DefaultRootWindow(display *Display) Window {
	return ScreenOfDisplay(display, DefaultScreen(display)).Root
}

func ScreenOfDisplay(display *Display, scr Screenid) *Screen {
	return &display.setup.Roots[scr]
}

type Visual struct {
	Visualid
	BitsPerRGB int
}

func DefaultVisual(display *Display, scr Screenid) Visualid {
	return ScreenOfDisplay(display, scr).RootVisual
}

func DefaultDepth(display *Display, scr Screenid) int {
	return int(ScreenOfDisplay(display, scr).RootDepth)
}

type SetWindowAttributes struct {
	BackgroundPixmap   Pixmap
	BackgroundPixel    uint32
	BorderPixmap       Pixmap
	BorderPixel        uint32
	BitGravity         int32
	WinGravity         int32
	BackingStore       int32
	BackingPlanes      uint32
	BackingPixel       uint32
	SaveUnder          bool
	EventMask          uint32
	DoNotPropagateMask uint32
	OverrideRedirect   bool
	Colormap           Colormap
	Cursor             Cursor
}

func (attr SetWindowAttributes) process(mask uint32) (values []uint32) {
	values = make([]uint32, 0, 32)
	boolToUint32 := func(b bool) uint32 {
		if b {
			return 1
		} else {
			return 0
		}
	}

	if mask&CwBackPixmap != 0 {
		values = append(values, uint32(attr.BackgroundPixmap))
	}
	if mask&CwBackPixel != 0 {
		values = append(values, uint32(attr.BackgroundPixel))
	}

	if mask&CwBorderPixmap != 0 {
		values = append(values, uint32(attr.BorderPixmap))
	}
	if mask&CwBorderPixel != 0 {
		values = append(values, uint32(attr.BorderPixel))
	}

	if mask&CwBitGravity != 0 {
		values = append(values, uint32(attr.BitGravity))
	}

	if mask&CwWinGravity != 0 {
		values = append(values, uint32(attr.WinGravity))
	}

	if mask&CwBackingStore != 0 {
		values = append(values, uint32(attr.BackingStore))
	}

	if mask&CwBackingPlanes != 0 {
		values = append(values, uint32(attr.BackingPlanes))
	}

	if mask&CwBackingPixel != 0 {
		values = append(values, uint32(attr.BackingPixel))
	}

	if mask&CwOverrideRedirect != 0 {
		values = append(values, boolToUint32(attr.OverrideRedirect))
	}

	if mask&CwSaveUnder != 0 {
		values = append(values, boolToUint32(attr.SaveUnder))
	}

	if mask&CwEventMask != 0 {
		values = append(values, uint32(attr.EventMask))
	}

	if mask&CwDontPropagate != 0 {
		values = append(values, uint32(attr.DoNotPropagateMask))
	}

	if mask&CwColormap != 0 {
		values = append(values, uint32(attr.Colormap))
	}

	if mask&CwCursor != 0 {
		values = append(values, uint32(attr.Cursor))
	}

	return
}

const (
	CwBackPixmap       = xproto.CwBackPixmap
	CwBackPixel        = xproto.CwBackPixel
	CwBorderPixmap     = xproto.CwBorderPixmap
	CwBorderPixel      = xproto.CwBorderPixel
	CwBitGravity       = xproto.CwBitGravity
	CwWinGravity       = xproto.CwWinGravity
	CwBackingStore     = xproto.CwBackingStore
	CwBackingPlanes    = xproto.CwBackingPlanes
	CwBackingPixel     = xproto.CwBackingPixel
	CwOverrideRedirect = xproto.CwOverrideRedirect
	CwSaveUnder        = xproto.CwSaveUnder
	CwEventMask        = xproto.CwEventMask
	CwDontPropagate    = xproto.CwDontPropagate
	CwColormap         = xproto.CwColormap
	CwCursor           = xproto.CwCursor
)

const (
	WindowClassCopyFromParent = xproto.WindowClassCopyFromParent
	WindowClassInputOutput    = xproto.WindowClassInputOutput
	WindowClassInputOnly      = xproto.WindowClassInputOnly
)

const (
	EventMaskNoEvent              = xproto.EventMaskNoEvent
	EventMaskKeyPress             = xproto.EventMaskKeyPress
	EventMaskKeyRelease           = xproto.EventMaskKeyRelease
	EventMaskButtonPress          = xproto.EventMaskButtonPress
	EventMaskButtonRelease        = xproto.EventMaskButtonRelease
	EventMaskEnterWindow          = xproto.EventMaskEnterWindow
	EventMaskLeaveWindow          = xproto.EventMaskLeaveWindow
	EventMaskPointerMotion        = xproto.EventMaskPointerMotion
	EventMaskPointerMotionHint    = xproto.EventMaskPointerMotionHint
	EventMaskButton1Motion        = xproto.EventMaskButton1Motion
	EventMaskButton2Motion        = xproto.EventMaskButton2Motion
	EventMaskButton3Motion        = xproto.EventMaskButton3Motion
	EventMaskButton4Motion        = xproto.EventMaskButton4Motion
	EventMaskButton5Motion        = xproto.EventMaskButton5Motion
	EventMaskButtonMotion         = xproto.EventMaskButtonMotion
	EventMaskKeymapState          = xproto.EventMaskKeymapState
	EventMaskExposure             = xproto.EventMaskExposure
	EventMaskVisibilityChange     = xproto.EventMaskVisibilityChange
	EventMaskStructureNotify      = xproto.EventMaskStructureNotify
	EventMaskResizeRedirect       = xproto.EventMaskResizeRedirect
	EventMaskSubstructureNotify   = xproto.EventMaskSubstructureNotify
	EventMaskSubstructureRedirect = xproto.EventMaskSubstructureRedirect
	EventMaskFocusChange          = xproto.EventMaskFocusChange
	EventMaskPropertyChange       = xproto.EventMaskPropertyChange
	EventMaskColorMapChange       = xproto.EventMaskColorMapChange
	EventMaskOwnerGrabButton      = xproto.EventMaskOwnerGrabButton
)

func CreateWindow(display *Display, parent Window, x, y int, width, height, borderWidth uint, depth, class byte, visual Visualid, valueMask uint32, attrs SetWindowAttributes) (Window, error) {
	wid, err := xproto.NewWindowId(display.conn)
	if err != nil {
		return 0, err
	}

	err = xproto.CreateWindowChecked(display.conn, depth, wid, parent, int16(x), int16(y), uint16(width), uint16(height), uint16(borderWidth), uint16(class), visual, valueMask, attrs.process(valueMask)).Check()
	if err != nil {
		return 0, err
	}
	return wid, nil
}

func DestroyWindow(display *Display, window Window) error {
	return xproto.DestroyWindowChecked(display.conn, window).Check()
}

func ChangeWindowAttributes(display *Display, window Window, valueMask uint32, attributes SetWindowAttributes) error {
	values := attributes.process(valueMask)
	return xproto.ChangeWindowAttributesChecked(display.conn, window, valueMask, values).Check()
}

func MapWindow(display *Display, window Window) error {
	return xproto.MapWindowChecked(display.conn, window).Check()
}

func InternAtom(display *Display, name string, onlyIfExists bool) (Atom, error) {
	reply, err := xproto.InternAtom(display.conn, onlyIfExists, uint16(len(name)), name).Reply()
	if err != nil {
		return 0, err
	}
	return reply.Atom, nil
}

type ImageByteOrderType byte

const (
	ImageOrderLSBFirst ImageByteOrderType = xproto.ImageOrderLSBFirst
	ImageOrderMSBFirst ImageByteOrderType = xproto.ImageOrderMSBFirst
)

func ImageByteOrder(display *Display) ImageByteOrderType {
	return ImageByteOrderType(display.setup.ImageByteOrder)
}

type GCValues struct {
	// TODO
}

func CreateGC(display *Display, drawable Drawable, valueMask uint32, values *GCValues) (Gcontext, error) {
	gc, err := xproto.NewGcontextId(display.conn)
	if err != nil {
		return 0, err
	}
	err = xproto.CreateGCChecked(display.conn, gc, drawable, 0, nil).Check()
	if err != nil {
		return 0, err
	}
	return gc, nil
}

func FreeGC(display *Display, gc Gcontext) error {
	return xproto.FreeGCChecked(display.conn, gc).Check()
}

func CreatePixmap(display *Display, drawable Drawable, width, height, depth uint) (Pixmap, error) {
	pid, err := xproto.NewPixmapId(display.conn)
	if err != nil {
		return 0, nil
	}
	err = xproto.CreatePixmapChecked(display.conn, byte(depth), pid, drawable, uint16(width), uint16(height)).Check()
	if err != nil {
		return 0, nil
	}
	return pid, nil
}

func FreePixmap(display *Display, pixmap Pixmap) error {
	return xproto.FreePixmapChecked(display.conn, pixmap).Check()
}

func PutBGRAImage(display *Display, drawable Drawable, gc Gcontext, width, height uint, dstX, dstY int, pix []byte) error {
	return xproto.PutImageChecked(
		display.conn, xproto.ImageFormatZPixmap,
		drawable, gc,
		uint16(width), uint16(height), int16(dstX), int16(dstY),
		0, 24, pix).Check()
}

type PropertyChangeMode byte

const (
	PropModeReplace PropertyChangeMode = xproto.PropModeReplace
	PropModePrepend PropertyChangeMode = xproto.PropModePrepend
	PropModeAppend  PropertyChangeMode = xproto.PropModeAppend
)

func ChangeProperty(display *Display, window Window, property, typ Atom, format byte, mode PropertyChangeMode, data unsafe.Pointer, nelements int) error {
	dataLen := nelements * int(format)
	byteData := unsafe.Slice((*byte)(data), dataLen)
	return xproto.ChangePropertyChecked(display.conn, byte(mode), window, property, typ, format, uint32(nelements), *(*[]byte)(unsafe.Pointer(&byteData))).Check()
}

func SetWMProtocols(display *Display, window Window, protocols []Atom) error {
	prop, err := InternAtom(display, "WM_PROTOCOLS", false)
	if err != nil {
		return err
	}
	return ChangeProperty(display, window, prop, XA_ATOM, 32, PropModeReplace, unsafe.Pointer(unsafe.SliceData(protocols)), len(protocols))
}

type (
	Error                = xgb.Error
	Event                = xgb.Event
	ClientMessageEvent   = xproto.ClientMessageEvent
	ConfigureNotifyEvent = xproto.ConfigureNotifyEvent
	ExposeEvent          = xproto.ExposeEvent
	PropertyNotifyEvent  = xproto.PropertyNotifyEvent
)
