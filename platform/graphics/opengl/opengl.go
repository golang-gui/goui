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

func DefaultConfig() Config {
	return Config{
		PixelFormat: DefaultPixelFormat(),
	}
}

type PixelFormat struct {
	RedBits        int32
	GreenBits      int32
	BlueBits       int32
	AlphaBits      int32
	DepthBits      int32
	StencilBits    int32
	AccumRedBits   int32
	AccumGreenBits int32
	AccumBlueBits  int32
	AccumAlphaBits int32
	AuxBuffers     int32
	Samples        int32
	Stereo         bool
	DoubleBuffer   bool
	Transparent    bool
	SRGB           bool
}

func DefaultPixelFormat() PixelFormat {
	return PixelFormat{
		RedBits:      8,
		GreenBits:    8,
		BlueBits:     8,
		AlphaBits:    8,
		DepthBits:    24,
		StencilBits:  8,
		DoubleBuffer: true,
	}
}

const DontCare = -1
