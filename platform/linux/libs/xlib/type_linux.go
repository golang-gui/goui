package xlib

import "github.com/goexlib/cgo"

type ID uintptr

type Bool int32

const (
	False Bool = iota
	True
)

type Status int32

type Display ID

type Atom ID

const (
	AtomNone               = 0
	AtomAny                = 0
	AtomPrimary            = 1
	AtomSecondary          = 2
	AtomArc                = 3
	AtomAtom               = 4
	AtomBitmap             = 5
	AtomCardinal           = 6
	AtomColormap           = 7
	AtomCursor             = 8
	AtomCutBuffer0         = 9
	AtomCutBuffer1         = 10
	AtomCutBuffer2         = 11
	AtomCutBuffer3         = 12
	AtomCutBuffer4         = 13
	AtomCutBuffer5         = 14
	AtomCutBuffer6         = 15
	AtomCutBuffer7         = 16
	AtomDrawable           = 17
	AtomFont               = 18
	AtomInteger            = 19
	AtomPixmap             = 20
	AtomPoint              = 21
	AtomRectangle          = 22
	AtomResourceManager    = 23
	AtomRgbColorMap        = 24
	AtomRgbBestMap         = 25
	AtomRgbBlueMap         = 26
	AtomRgbDefaultMap      = 27
	AtomRgbGrayMap         = 28
	AtomRgbGreenMap        = 29
	AtomRgbRedMap          = 30
	AtomString             = 31
	AtomVisualid           = 32
	AtomWindow             = 33
	AtomWmCommand          = 34
	AtomWmHints            = 35
	AtomWmClientMachine    = 36
	AtomWmIconName         = 37
	AtomWmIconSize         = 38
	AtomWmName             = 39
	AtomWmNormalHints      = 40
	AtomWmSizeHints        = 41
	AtomWmZoomHints        = 42
	AtomMinSpace           = 43
	AtomNormSpace          = 44
	AtomMaxSpace           = 45
	AtomEndSpace           = 46
	AtomSuperscriptX       = 47
	AtomSuperscriptY       = 48
	AtomSubscriptX         = 49
	AtomSubscriptY         = 50
	AtomUnderlinePosition  = 51
	AtomUnderlineThickness = 52
	AtomStrikeoutAscent    = 53
	AtomStrikeoutDescent   = 54
	AtomItalicAngle        = 55
	AtomXHeight            = 56
	AtomQuadWidth          = 57
	AtomWeight             = 58
	AtomPointSize          = 59
	AtomResolution         = 60
	AtomCopyright          = 61
	AtomNotice             = 62
	AtomFontName           = 63
	AtomFamilyName         = 64
	AtomFullName           = 65
	AtomCapHeight          = 66
	AtomWmClass            = 67
	AtomWmTransientFor     = 68
)

type Screen struct {
	ExtData       uintptr
	Display       Display
	Root          Window
	Width         int32
	Height        int32
	MWidth        int32
	MHeight       int32
	NDepths       int32
	Depths        *Depth
	RootDepth     int32
	RootVisual    *Visual
	DefaultGC     GC
	CMap          Colormap
	WhitePixel    uint64
	BlackPixel    uint64
	MaxMaps       int32
	MinMaps       int32
	BackingStore  int32
	SaveUnders    Bool
	RootInputMask uint64
}

type Window ID

type KeyCode byte

type VisualID ID

type Visual struct {
	ExtData    uintptr
	VisualId   VisualID
	Class      int32
	RedMask    uint64
	GreenMask  uint64
	BlueMask   uint64
	BitsPerRgb int32
	MapEntries int32
}

type VisualInfo struct {
	Visual       *Visual
	VisualId     VisualID
	Screen       int32
	Depth        int32
	Class        int32
	RedMask      uint64
	GreenMask    uint64
	BlueMask     uint64
	ColormapSize int32
	BitsPerRgb   int32
}

type Depth struct {
	Depth    int32
	NVisuals int32
	Visuals  *Visual
}

type SetWindowAttributes struct {
	BackgroundPixmap   Pixmap
	BackgroundPixel    uint64
	BorderPixmap       Pixmap
	BorderPixel        uint64
	BitGravity         int32
	WinGravity         int32
	BackingStore       int32
	BackingPlanes      uint64
	BackingPixel       uint64
	SaveUnder          Bool
	EventMask          uint64
	DoNotPropagateMask uint64
	OverrideRedirect   Bool
	Colormap           Colormap
	Cursor             Cursor
}

const (
	WindowClassCopyFromParent = 0
	WindowClassInputOutput    = 1
	WindowClassInputOnly      = 2
)

type PropertyChangeMode byte

const (
	PropModeReplace PropertyChangeMode = 0
	PropModePrepend PropertyChangeMode = 1
	PropModeAppend  PropertyChangeMode = 2
)

type GC ID

type Drawable ID

type GCValues struct {
	Function          int32
	PlaneMask         uint64
	Foreground        uint64
	Background        uint64
	LineWidth         int32
	lineStyle         int32
	CapStyle          int32
	JoinStyle         int32
	FillStyle         int32
	FillRule          int32
	ArcMode           int32
	Tile              Pixmap
	Stipple           Pixmap
	TsXOrigin         int32
	TsYOrigin         int32
	Font              Font
	SubWindowMode     int32
	GraphicsExposures Bool
	ClipXOrigin       int32
	ClipYOrigin       int32
	ClipMask          Pixmap
	DashOffset        int32
	dashes            uint8
}

type Font ID

type Pixmap ID

type Image struct {
	Width          int32
	Height         int32
	XOffset        int32
	Format         int32
	Data           cgo.Pointer
	ByteOrder      int32
	BitmapUnit     int32
	BitmapBitOrder int32
	BitmapPad      int32
	Depth          int32
	BytesPerLine   int32
	BitsPerPixel   int32
	RedMask        uint64
	GreenMask      uint64
	BlueMask       uint64
	Obdata         uintptr
	Funcs          struct {
		CreateImage  uintptr // Image* (Display, Visual*, uint depth, int format, int offset, char* data, uint width, uint height, int bitmapPad, int bytesPerLine)
		DestroyImage uintptr // int(Image*)
		GetPixel     uintptr // uint64(Image*, int x, int y)
		PutPixel     uintptr // int(Image*, int x, int y, uint64 pixel)
		SubImage     uintptr // Image* (Image*, int x, int y, uint width, uint height)
		AddPixel     uintptr // int(Image*, long)
	}
}

const (
	ImageFormatXYBitmap = 0
	ImageFormatXYPixmap = 1
	ImageFormatZPixmap  = 2
)

const (
	ImageOrderLSBFirst = 0
	ImageOrderMSBFirst = 1
)

type Colormap ID

type ColormapAlloc int

const (
	ColormapAllocNone ColormapAlloc = iota
	ColormapAllocAll
)

type Cursor ID

type Time int

type KeySym ID

type ModifierKeymap struct {
	MaxKeypermod int32
	Modifiermap  *KeyCode
}

func (m *ModifierKeymap) Keycodes() []KeyCode {
	if m == nil || m.Modifiermap == nil || m.MaxKeypermod <= 0 {
		return nil
	}
	return cgo.GoSliceN[KeyCode](cgo.Pointer(m.Modifiermap), int(m.MaxKeypermod)*8)
}

type Event struct {
	Type    EventType
	padding [32]uintptr
}

func (e *Event) AnyEvent() *AnyEvent {
	return (*AnyEvent)(cgo.Pointer(e))
}

func (e *Event) ExposeEvent() *ExposeEvent {
	return (*ExposeEvent)(cgo.Pointer(e))
}

func (e *Event) ConfigureEvent() *ConfigureEvent {
	return (*ConfigureEvent)(cgo.Pointer(e))
}

func (e *Event) PropertyEvent() *PropertyEvent {
	return (*PropertyEvent)(cgo.Pointer(e))
}

func (e *Event) ClientMessageEvent() *ClientMessageEvent {
	return (*ClientMessageEvent)(cgo.Pointer(e))
}

func (e *Event) KeyEvent() *KeyEvent {
	return (*KeyEvent)(cgo.Pointer(e))
}

func (e *Event) ButtonEvent() *ButtonEvent {
	return (*ButtonEvent)(cgo.Pointer(e))
}

func (e *Event) MotionEvent() *MotionEvent {
	return (*MotionEvent)(cgo.Pointer(e))
}

func (e *Event) CrossingEvent() *CrossingEvent {
	return (*CrossingEvent)(cgo.Pointer(e))
}

type AnyEvent struct {
	Type      EventType
	Serial    uint64
	SendEvent Bool
	Display   Display
	Window    Window
}

type ExposeEvent struct {
	Type      EventType
	Serial    uint64
	SendEvent Bool
	Display   Display
	Window    Window
	X         int32
	Y         int32
	Width     int32
	Height    int32
	Count     int32
}

type PropertyEvent struct {
	Type      EventType
	Serial    uint64
	SendEvent Bool
	Display   Display
	Window    Window
	Atom      Atom
	Time      Time
	State     int32
}

type ConfigureEvent struct {
	Type             EventType
	Serial           uint64
	SendEvent        Bool
	Display          Display
	Event            Window
	Window           Window
	X                int32
	Y                int32
	Width            int32
	Height           int32
	BorderWidth      int32
	Above            Window
	OverrideRedirect Bool
}

type KeyEvent struct {
	Type       EventType
	Serial     uint64
	SendEvent  Bool
	Display    Display
	Window     Window
	Root       Window
	SubWindow  Window
	Time       Time
	X          int32
	Y          int32
	XRoot      int32
	YRoot      int32
	State      uint32
	KeyCode    uint32
	SameScreen Bool
}

type ButtonEvent struct {
	Type       EventType
	Serial     uint64
	SendEvent  Bool
	Display    Display
	Window     Window
	Root       Window
	SubWindow  Window
	Time       Time
	X          int32
	Y          int32
	XRoot      int32
	YRoot      int32
	State      uint32
	Button     uint32
	SameScreen Bool
}

type MotionEvent struct {
	Type       EventType
	Serial     uint64
	SendEvent  Bool
	Display    Display
	Window     Window
	Root       Window
	SubWindow  Window
	Time       Time
	X          int32
	Y          int32
	XRoot      int32
	YRoot      int32
	State      uint32
	IsHint     byte
	SameScreen Bool
}

type CrossingEvent struct {
	Type       EventType
	Serial     uint64
	SendEvent  Bool
	Display    Display
	Window     Window
	Root       Window
	SubWindow  Window
	Time       Time
	X          int32
	Y          int32
	XRoot      int32
	YRoot      int32
	Mode       int32
	Detail     int32
	SameScreen Bool
	Focus      Bool
	State      uint32
}

type ClientMessageEvent struct {
	Type        EventType
	Serial      uint64
	SendEvent   Bool
	Display     Display
	Window      Window
	MessageType Atom
	Format      int32
	L           [5]int64
}

type EventType int32

const (
	KeyPress         EventType = 2
	KeyRelease       EventType = 3
	ButtonPress      EventType = 4
	ButtonRelease    EventType = 5
	MotionNotify     EventType = 6
	EnterNotify      EventType = 7
	LeaveNotify      EventType = 8
	FocusIn          EventType = 9
	FocusOut         EventType = 10
	KeymapNotify     EventType = 11
	Expose           EventType = 12
	GraphicsExpose   EventType = 13
	NoExpose         EventType = 14
	VisibilityNotify EventType = 15
	CreateNotify     EventType = 16
	DestroyNotify    EventType = 17
	UnmapNotify      EventType = 18
	MapNotify        EventType = 19
	MapRequest       EventType = 20
	ReparentNotify   EventType = 21
	ConfigureNotify  EventType = 22
	ConfigureRequest EventType = 23
	GravityNotify    EventType = 24
	ResizeRequest    EventType = 25
	CirculateNotify  EventType = 26
	CirculateRequest EventType = 27
	PropertyNotify   EventType = 28
	SelectionClear   EventType = 29
	SelectionRequest EventType = 30
	SelectionNotify  EventType = 31
	ColormapNotify   EventType = 32
	ClientMessage    EventType = 33
	MappingNotify    EventType = 34
	GenericEvent     EventType = 35
)

const (
	ShiftMask   = 1 << 0
	LockMask    = 1 << 1
	ControlMask = 1 << 2
	Mod1Mask    = 1 << 3
	Mod2Mask    = 1 << 4
	Mod3Mask    = 1 << 5
	Mod4Mask    = 1 << 6
	Mod5Mask    = 1 << 7
	Button1Mask = 1 << 8
	Button2Mask = 1 << 9
	Button3Mask = 1 << 10
)

const (
	XK_Scroll_Lock  KeySym = 0xff14
	XK_Pause        KeySym = 0xff13
	XK_BackSpace    KeySym = 0xff08
	XK_Tab          KeySym = 0xff09
	XK_Return       KeySym = 0xff0d
	XK_Escape       KeySym = 0xff1b
	XK_Print        KeySym = 0xff61
	XK_Break        KeySym = 0xff6b
	XK_Delete       KeySym = 0xffff
	XK_Home         KeySym = 0xff50
	XK_Left         KeySym = 0xff51
	XK_Up           KeySym = 0xff52
	XK_Right        KeySym = 0xff53
	XK_Down         KeySym = 0xff54
	XK_Page_Up      KeySym = 0xff55
	XK_Page_Down    KeySym = 0xff56
	XK_End          KeySym = 0xff57
	XK_Insert       KeySym = 0xff63
	XK_Num_Lock     KeySym = 0xff7f
	XK_KP_Enter     KeySym = 0xff8d
	XK_KP_Home      KeySym = 0xff95
	XK_KP_Left      KeySym = 0xff96
	XK_KP_Up        KeySym = 0xff97
	XK_KP_Right     KeySym = 0xff98
	XK_KP_Down      KeySym = 0xff99
	XK_KP_Page_Up   KeySym = 0xff9a
	XK_KP_Page_Down KeySym = 0xff9b
	XK_KP_End       KeySym = 0xff9c
	XK_KP_Begin     KeySym = 0xff9d
	XK_KP_Insert    KeySym = 0xff9e
	XK_KP_Delete    KeySym = 0xff9f
	XK_KP_Multiply  KeySym = 0xffaa
	XK_KP_Add       KeySym = 0xffab
	XK_KP_Subtract  KeySym = 0xffad
	XK_KP_Decimal   KeySym = 0xffae
	XK_KP_Divide    KeySym = 0xffaf
	XK_KP_0         KeySym = 0xffb0
	XK_KP_1         KeySym = 0xffb1
	XK_KP_2         KeySym = 0xffb2
	XK_KP_3         KeySym = 0xffb3
	XK_KP_4         KeySym = 0xffb4
	XK_KP_5         KeySym = 0xffb5
	XK_KP_6         KeySym = 0xffb6
	XK_KP_7         KeySym = 0xffb7
	XK_KP_8         KeySym = 0xffb8
	XK_KP_9         KeySym = 0xffb9
	XK_F1           KeySym = 0xffbe
	XK_F24          KeySym = 0xffd5
	XK_Shift_L      KeySym = 0xffe1
	XK_Shift_R      KeySym = 0xffe2
	XK_Control_L    KeySym = 0xffe3
	XK_Control_R    KeySym = 0xffe4
	XK_Caps_Lock    KeySym = 0xffe5
	XK_Alt_L        KeySym = 0xffe9
	XK_Alt_R        KeySym = 0xffea
	XK_Super_L      KeySym = 0xffeb
	XK_Super_R      KeySym = 0xffec
)

const (
	Button1 = 1
	Button2 = 2
	Button3 = 3
	Button4 = 4
	Button5 = 5
	Button6 = 6
	Button7 = 7
	Button8 = 8
	Button9 = 9
)

const (
	EventMaskNoEvent              = 0
	EventMaskKeyPress             = 1
	EventMaskKeyRelease           = 2
	EventMaskButtonPress          = 4
	EventMaskButtonRelease        = 8
	EventMaskEnterWindow          = 16
	EventMaskLeaveWindow          = 32
	EventMaskPointerMotion        = 64
	EventMaskPointerMotionHint    = 128
	EventMaskButton1Motion        = 256
	EventMaskButton2Motion        = 512
	EventMaskButton3Motion        = 1024
	EventMaskButton4Motion        = 2048
	EventMaskButton5Motion        = 4096
	EventMaskButtonMotion         = 8192
	EventMaskKeymapState          = 16384
	EventMaskExposure             = 32768
	EventMaskVisibilityChange     = 65536
	EventMaskStructureNotify      = 131072
	EventMaskResizeRedirect       = 262144
	EventMaskSubstructureNotify   = 524288
	EventMaskSubstructureRedirect = 1048576
	EventMaskFocusChange          = 2097152
	EventMaskPropertyChange       = 4194304
	EventMaskColorMapChange       = 8388608
	EventMaskOwnerGrabButton      = 16777216
)

const (
	CwBackPixmap       = 1
	CwBackPixel        = 2
	CwBorderPixmap     = 4
	CwBorderPixel      = 8
	CwBitGravity       = 16
	CwWinGravity       = 32
	CwBackingStore     = 64
	CwBackingPlanes    = 128
	CwBackingPixel     = 256
	CwOverrideRedirect = 512
	CwSaveUnder        = 1024
	CwEventMask        = 2048
	CwDontPropagate    = 4096
	CwColormap         = 8192
	CwCursor           = 16384
)

const (
	ConfigWindowX           = 1
	ConfigWindowY           = 2
	ConfigWindowWidth       = 4
	ConfigWindowHeight      = 8
	ConfigWindowBorderWidth = 16
	ConfigWindowSibling     = 32
	ConfigWindowStackMode   = 64
)
