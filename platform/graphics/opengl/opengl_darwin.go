package opengl

import (
	"errors"
	"github.com/golang-gui/goui/platform/darwin/frameworks/appkit"
	"github.com/golang-gui/goui/platform/darwin/frameworks/foundation"
	"github.com/golang-gui/goui/platform/darwin/frameworks/opengl"
)

type nsglContext struct {
	object      opengl.NSOpenGLContext
	pixelFormat opengl.NSOpenGLPixelFormat
}

func newContext(win NativeWindow, share Context, config Config) (_ Context, err error) {
	attribs := make([]opengl.NSOpenGLPixelFormatAttribute, 0, 40)
	addAttrib := func(attr opengl.NSOpenGLPixelFormatAttribute) {
		attribs = append(attribs, attr)
	}
	setAttrib := func(attr opengl.NSOpenGLPixelFormatAttribute, value int) {
		attribs = append(attribs, attr, opengl.NSOpenGLPixelFormatAttribute(value))
	}

	addAttrib(opengl.NSOpenGLPFAAccelerated)
	addAttrib(opengl.NSOpenGLPFAClosestPolicy)

	setAttrib(opengl.NSOpenGLPFAOpenGLProfile, opengl.NSOpenGLProfileVersion3_2Core)

	if config.PixelFormat.RedBits != DontCare &&
		config.PixelFormat.GreenBits != DontCare &&
		config.PixelFormat.BlueBits != DontCare {

		colorBits := config.PixelFormat.RedBits + config.PixelFormat.GreenBits + config.PixelFormat.BlueBits
		if colorBits == 0 {
			colorBits = 24
		} else if colorBits < 15 {
			colorBits = 15
		}
		setAttrib(opengl.NSOpenGLPFAColorSize, colorBits)
	}

	if config.PixelFormat.AlphaBits != DontCare {
		setAttrib(opengl.NSOpenGLPFAAlphaSize, config.PixelFormat.AlphaBits)
	}

	if config.PixelFormat.DepthBits != DontCare {
		setAttrib(opengl.NSOpenGLPFADepthSize, config.PixelFormat.DepthBits)
	}

	if config.PixelFormat.StencilBits != DontCare {
		setAttrib(opengl.NSOpenGLPFAStencilSize, config.PixelFormat.StencilBits)
	}

	if config.PixelFormat.Stereo {
		return nil, errors.New("NSGL: Stereo rendering is deprecated")
	}

	if config.PixelFormat.DoubleBuffer {
		addAttrib(opengl.NSOpenGLPFADoubleBuffer)
	}

	if config.PixelFormat.Samples != DontCare {
		if config.PixelFormat.Samples == 0 {
			setAttrib(opengl.NSOpenGLPFASampleBuffers, 0)
		} else {
			setAttrib(opengl.NSOpenGLPFASampleBuffers, 1)
			setAttrib(opengl.NSOpenGLPFASamples, config.PixelFormat.Samples)
		}
	}

	addAttrib(0)

	pixelFormat := opengl.NSOpenGLPixelFormatClassId.Alloc().InitWithAttributes(attribs)
	if pixelFormat.IsNil() {
		return nil, errors.New("NSGL: Failed to find a suitable pixel format")
	}

	var shareContext opengl.NSOpenGLContext
	if share != nil {
		shareContext = share.(nsglContext).object
	}

	object := opengl.NSOpenGLContextClassId.Alloc().InitWithFormat(pixelFormat, shareContext)
	if object.IsNil() {
		pixelFormat.Release()
		return nil, errors.New("NSGL: Failed to create OpenGL contex")
	}

	var window appkit.NSWindow
	window.ID = foundation.ID(win.NativeHandle())
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
	foundation.AutoReleasePool(func() {
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
	foundation.AutoReleasePool(func() {
		c.object.MakeCurrentContext()
		c.object.Update()
	})
	return nil
}

func (c nsglContext) ClearCurrent() error {
	foundation.AutoReleasePool(func() {
		opengl.NSOpenGLContextClassId.ClearCurrentContext()
	})
	return nil
}

func (c nsglContext) SwapBuffers() error {
	foundation.AutoReleasePool(func() {
		// TODO: occluded state check?
		c.object.FlushBuffer()
	})
	return nil
}

func (c nsglContext) SwapInterval(interval int) error {
	foundation.AutoReleasePool(func() {
		c.object.SetValue(interval, opengl.NSOpenGLContextParameterSwapInterval)
	})
	return nil
}

func (c nsglContext) GetProcAddress(name string) (uintptr, error) {
	return opengl.GetProcAddress(name)
}

func (c nsglContext) GetExtensions() string {
	return ""
}
