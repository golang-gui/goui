package opengl

import (
	"errors"

	. "github.com/golang-gui/goui/platform/darwin/frameworks/appkit"
	. "github.com/golang-gui/goui/platform/darwin/frameworks/foundation"
	. "github.com/golang-gui/goui/platform/darwin/frameworks/opengl"
)

type nsglContext struct {
	object      NSOpenGLContext
	pixelFormat NSOpenGLPixelFormat
}

func newContext(win NativeWindow, share Context, config Config) (_ Context, err error) {
	attribs := make([]NSOpenGLPixelFormatAttribute, 0, 40)
	addAttrib := func(attr NSOpenGLPixelFormatAttribute) {
		attribs = append(attribs, attr)
	}
	setAttrib := func(attr NSOpenGLPixelFormatAttribute, value int) {
		attribs = append(attribs, attr, NSOpenGLPixelFormatAttribute(value))
	}

	addAttrib(NSOpenGLPFAAccelerated)
	addAttrib(NSOpenGLPFAClosestPolicy)

	setAttrib(NSOpenGLPFAOpenGLProfile, NSOpenGLProfileVersion3_2Core)

	if config.PixelFormat.RedBits != DontCare &&
		config.PixelFormat.GreenBits != DontCare &&
		config.PixelFormat.BlueBits != DontCare {

		colorBits := config.PixelFormat.RedBits + config.PixelFormat.GreenBits + config.PixelFormat.BlueBits
		if colorBits == 0 {
			colorBits = 24
		} else if colorBits < 15 {
			colorBits = 15
		}
		setAttrib(NSOpenGLPFAColorSize, colorBits)
	}

	if config.PixelFormat.AlphaBits != DontCare {
		setAttrib(NSOpenGLPFAAlphaSize, config.PixelFormat.AlphaBits)
	}

	if config.PixelFormat.DepthBits != DontCare {
		setAttrib(NSOpenGLPFADepthSize, config.PixelFormat.DepthBits)
	}

	if config.PixelFormat.StencilBits != DontCare {
		setAttrib(NSOpenGLPFAStencilSize, config.PixelFormat.StencilBits)
	}

	if config.PixelFormat.Stereo {
		return nil, errors.New("NSGL: Stereo rendering is deprecated")
	}

	if config.PixelFormat.DoubleBuffer {
		addAttrib(NSOpenGLPFADoubleBuffer)
	}

	if config.PixelFormat.Samples != DontCare {
		if config.PixelFormat.Samples == 0 {
			setAttrib(NSOpenGLPFASampleBuffers, 0)
		} else {
			setAttrib(NSOpenGLPFASampleBuffers, 1)
			setAttrib(NSOpenGLPFASamples, config.PixelFormat.Samples)
		}
	}

	addAttrib(0)

	pixelFormat := NSOpenGLPixelFormatClassId.Alloc().InitWithAttributes(attribs)
	if pixelFormat.IsNil() {
		return nil, errors.New("NSGL: Failed to find a suitable pixel format")
	}

	var shareContext NSOpenGLContext
	if share != nil {
		shareContext = share.(nsglContext).object
	}

	object := NSOpenGLContextClassId.Alloc().InitWithFormat(pixelFormat, shareContext)
	if object.IsNil() {
		pixelFormat.Release()
		return nil, errors.New("NSGL: Failed to create OpenGL contex")
	}

	var window NSWindow
	window.ID = ID(win.NativeHandle())
	object.SetView(window.ContentView())

	return nsglContext{
		object:      object,
		pixelFormat: pixelFormat,
	}, nil
}

func (c nsglContext) Name() string {
	return "NSGL"
}

func (c nsglContext) Destroy() {
	AutoReleasePool(func() {
		if !c.pixelFormat.IsNil() {
			c.pixelFormat.Release()
			c.pixelFormat.ID = 0
		}
		if !c.object.IsNil() {
			c.object.Release()
			c.object.ID = 0
		}
	})
}

func (c nsglContext) MakeCurrent() error {
	AutoReleasePool(func() {
		c.object.MakeCurrentContext()
		c.object.Update()
	})
	return nil
}

func (c nsglContext) ClearCurrent() error {
	AutoReleasePool(func() {
		NSOpenGLContextClassId.ClearCurrentContext()
	})
	return nil
}

func (c nsglContext) SwapBuffers() error {
	AutoReleasePool(func() {
		// TODO: occluded state check?
		c.object.FlushBuffer()
	})
	return nil
}

func (c nsglContext) SwapInterval(interval int) error {
	AutoReleasePool(func() {
		c.object.SetValue(interval, NSOpenGLContextParameterSwapInterval)
	})
	return nil
}

func (c nsglContext) GetProcAddress(name string) (uintptr, error) {
	return GetProcAddress(name)
}

func (c nsglContext) GetExtensions() string {
	return ""
}
