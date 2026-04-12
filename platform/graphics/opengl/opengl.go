package opengl

type Context interface {
	Name() string
	Destroy()
	MakeCurrent() error
	ClearCurrent() error
	SwapBuffers() error
	SwapInterval(int) error
	GetProcAddress(name string) (uintptr, error)
	GetExtensions() string
}

type NativeWindow interface {
	NativeHandle() uintptr
}

func NewContext(win NativeWindow, share Context, config Config) (Context, error) {
	return newContext(win, share, config)
}

type Config struct {
	PixelFormat PixelFormat
}

var DefaultConfig = Config{
	PixelFormat: DefaultPixelFormat,
}

type PixelFormat struct {
	RedBits        int
	GreenBits      int
	BlueBits       int
	AlphaBits      int
	DepthBits      int
	StencilBits    int
	AccumRedBits   int
	AccumGreenBits int
	AccumBlueBits  int
	AccumAlphaBits int
	AuxBuffers     int
	Samples        int
	Stereo         bool
	DoubleBuffer   bool
	Transparent    bool
	SRGB           bool
}

var DefaultPixelFormat = PixelFormat{
	RedBits:      8,
	GreenBits:    8,
	BlueBits:     8,
	AlphaBits:    8,
	DepthBits:    24,
	StencilBits:  8,
	Samples:      4,
	DoubleBuffer: true,
}

const DontCare = -1
