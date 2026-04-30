package opengl

import (
	"github.com/ebitengine/purego/objc"
	"github.com/goexlib/cgo"
	"github.com/golang-gui/goui/platform/cocoa/frameworks/appkit"
	"github.com/golang-gui/goui/platform/cocoa/frameworks/common"
	"github.com/golang-gui/goui/platform/cocoa/frameworks/foundation"
)

var handle uintptr

func Init(load common.LoadFunc) (err error) {
	handle, err = load("OpenGL")
	if err != nil {
		return
	}

	initNSOpenGLContext()
	initNSOpenGLPixelFormat()
	return nil
}

func GetProcAddress(name string) (fn uintptr, err error) {
	return cgo.Dlsym(handle, name)
}

func initNSOpenGLContext() {
	NSOpenGLContextClassId.Class = objc.GetClass("NSOpenGLContext")
	NSOpenGLContextSel.InitWithFormat = objc.RegisterName("initWithFormat:shareContext:")
	NSOpenGLContextSel.GetValues = objc.RegisterName("getValues:forParameter:")
	NSOpenGLContextSel.SetValues = objc.RegisterName("setValues:forParameter:")
	NSOpenGLContextSel.SetView = objc.RegisterName("setView:")
	NSOpenGLContextSel.Update = objc.RegisterName("update")
	NSOpenGLContextSel.FlushBuffer = objc.RegisterName("flushBuffer")
	NSOpenGLContextSel.MakeCurrentContext = objc.RegisterName("makeCurrentContext")
	NSOpenGLContextSel.ClearCurrentContext = objc.RegisterName("clearCurrentContext")
}

var (
	NSOpenGLContextClassId NSOpenGLContextClass
	NSOpenGLContextSel     struct {
		InitWithFormat      objc.SEL
		GetValues           objc.SEL
		SetValues           objc.SEL
		SetView             objc.SEL
		Update              objc.SEL
		FlushBuffer         objc.SEL
		MakeCurrentContext  objc.SEL
		ClearCurrentContext objc.SEL
	}
)

type (
	NSOpenGLContextClass struct{ foundation.NSObjectClass }
	NSOpenGLContext      struct{ foundation.NSObject }
)

func (c NSOpenGLContextClass) Alloc() (inst NSOpenGLContext) {
	inst.NSObject = c.NSObjectClass.Alloc()
	return
}

func (c NSOpenGLContextClass) ClearCurrentContext() {
	c.Send(NSOpenGLContextSel.ClearCurrentContext)
}

func (self NSOpenGLContext) InitWithFormat(pixelFormat NSOpenGLPixelFormat, shareContext NSOpenGLContext) (inst NSOpenGLContext) {
	inst.ID = self.Send(NSOpenGLContextSel.InitWithFormat, pixelFormat, shareContext)
	return
}

func (self NSOpenGLContext) GetValue(forParameter NSOpenGLContextParameter) (value int) {
	self.Send(NSOpenGLContextSel.GetValues, &value, forParameter)
	return
}

func (self NSOpenGLContext) SetValue(value int, forParameter NSOpenGLContextParameter) {
	self.Send(NSOpenGLContextSel.SetValues, &value, forParameter)
	return
}

func (self NSOpenGLContext) SetView(view appkit.NSView) {
	self.Send(NSOpenGLContextSel.SetView, view)
	return
}

func (self NSOpenGLContext) Update() {
	self.Send(NSOpenGLContextSel.Update)
	return
}

func (self NSOpenGLContext) FlushBuffer() {
	self.Send(NSOpenGLContextSel.FlushBuffer)
	return
}

func (self NSOpenGLContext) MakeCurrentContext() {
	self.Send(NSOpenGLContextSel.MakeCurrentContext)
	return
}

func initNSOpenGLPixelFormat() {
	NSOpenGLPixelFormatClassId.Class = objc.GetClass("NSOpenGLPixelFormat")
	NSOpenGLPixelFormatSel.InitWithAttributes = objc.RegisterName("initWithAttributes:")
}

var (
	NSOpenGLPixelFormatClassId NSOpenGLPixelFormatClass
	NSOpenGLPixelFormatSel     struct {
		InitWithAttributes objc.SEL
	}
)

type (
	NSOpenGLPixelFormatClass struct{ foundation.NSObjectClass }
	NSOpenGLPixelFormat      struct{ foundation.NSObject }
)

func (c NSOpenGLPixelFormatClass) Alloc() (inst NSOpenGLPixelFormat) {
	inst.NSObject = c.NSObjectClass.Alloc()
	return
}

func (self NSOpenGLPixelFormat) InitWithAttributes(attribs []NSOpenGLPixelFormatAttribute) (inst NSOpenGLPixelFormat) {
	inst.ID = self.Send(NSOpenGLPixelFormatSel.InitWithAttributes, attribs)
	return
}

type NSOpenGLContextParameter foundation.NSInteger

const (
	NSOpenGLContextParameterSwapInterval           NSOpenGLContextParameter = 222
	NSOpenGLContextParameterSurfaceOrder           NSOpenGLContextParameter = 235
	NSOpenGLContextParameterSurfaceOpacity         NSOpenGLContextParameter = 236
	NSOpenGLContextParameterSurfaceBackingSize     NSOpenGLContextParameter = 304
	NSOpenGLContextParameterReclaimResources       NSOpenGLContextParameter = 308
	NSOpenGLContextParameterCurrentRendererID      NSOpenGLContextParameter = 309
	NSOpenGLContextParameterGPUVertexProcessing    NSOpenGLContextParameter = 310
	NSOpenGLContextParameterGPUFragmentProcessing  NSOpenGLContextParameter = 311
	NSOpenGLContextParameterHasDrawable            NSOpenGLContextParameter = 314
	NSOpenGLContextParameterMPSwapsInFlight        NSOpenGLContextParameter = 315
	NSOpenGLContextParameterSwapRectangle          NSOpenGLContextParameter = 200
	NSOpenGLContextParameterSwapRectangleEnable    NSOpenGLContextParameter = 201
	NSOpenGLContextParameterRasterizationEnable    NSOpenGLContextParameter = 221
	NSOpenGLContextParameterStateValidation        NSOpenGLContextParameter = 301
	NSOpenGLContextParameterSurfaceSurfaceVolatile NSOpenGLContextParameter = 306
)

type NSOpenGLPixelFormatAttribute uint32

const (
	NSOpenGLPFAAllRenderers          NSOpenGLPixelFormatAttribute = 1
	NSOpenGLPFATripleBuffer          NSOpenGLPixelFormatAttribute = 3
	NSOpenGLPFADoubleBuffer          NSOpenGLPixelFormatAttribute = 5
	NSOpenGLPFAAuxBuffers            NSOpenGLPixelFormatAttribute = 7
	NSOpenGLPFAColorSize             NSOpenGLPixelFormatAttribute = 8
	NSOpenGLPFAAlphaSize             NSOpenGLPixelFormatAttribute = 11
	NSOpenGLPFADepthSize             NSOpenGLPixelFormatAttribute = 12
	NSOpenGLPFAStencilSize           NSOpenGLPixelFormatAttribute = 13
	NSOpenGLPFAAccumSize             NSOpenGLPixelFormatAttribute = 14
	NSOpenGLPFAMinimumPolicy         NSOpenGLPixelFormatAttribute = 51
	NSOpenGLPFAMaximumPolicy         NSOpenGLPixelFormatAttribute = 52
	NSOpenGLPFASampleBuffers         NSOpenGLPixelFormatAttribute = 55
	NSOpenGLPFASamples               NSOpenGLPixelFormatAttribute = 56
	NSOpenGLPFAAuxDepthStencil       NSOpenGLPixelFormatAttribute = 57
	NSOpenGLPFAColorFloat            NSOpenGLPixelFormatAttribute = 58
	NSOpenGLPFAMultisample           NSOpenGLPixelFormatAttribute = 59
	NSOpenGLPFASupersample           NSOpenGLPixelFormatAttribute = 60
	NSOpenGLPFASampleAlpha           NSOpenGLPixelFormatAttribute = 61
	NSOpenGLPFARendererID            NSOpenGLPixelFormatAttribute = 70
	NSOpenGLPFANoRecovery            NSOpenGLPixelFormatAttribute = 72
	NSOpenGLPFAAccelerated           NSOpenGLPixelFormatAttribute = 73
	NSOpenGLPFAClosestPolicy         NSOpenGLPixelFormatAttribute = 74
	NSOpenGLPFABackingStore          NSOpenGLPixelFormatAttribute = 76
	NSOpenGLPFAScreenMask            NSOpenGLPixelFormatAttribute = 84
	NSOpenGLPFAAllowOfflineRenderers NSOpenGLPixelFormatAttribute = 96
	NSOpenGLPFAAcceleratedCompute    NSOpenGLPixelFormatAttribute = 97
	NSOpenGLPFAOpenGLProfile         NSOpenGLPixelFormatAttribute = 99
	NSOpenGLPFAVirtualScreenCount    NSOpenGLPixelFormatAttribute = 128
	NSOpenGLPFAStereo                NSOpenGLPixelFormatAttribute = 6
	NSOpenGLPFAOffScreen             NSOpenGLPixelFormatAttribute = 53
	NSOpenGLPFAFullScreen            NSOpenGLPixelFormatAttribute = 54
	NSOpenGLPFASingleRenderer        NSOpenGLPixelFormatAttribute = 71
	NSOpenGLPFARobust                NSOpenGLPixelFormatAttribute = 75
	NSOpenGLPFAMPSafe                NSOpenGLPixelFormatAttribute = 78
	NSOpenGLPFAWindow                NSOpenGLPixelFormatAttribute = 80
	NSOpenGLPFAMultiScreen           NSOpenGLPixelFormatAttribute = 81
	NSOpenGLPFACompliant             NSOpenGLPixelFormatAttribute = 83
	NSOpenGLPFAPixelBuffer           NSOpenGLPixelFormatAttribute = 90
	NSOpenGLPFARemotePixelBuffer     NSOpenGLPixelFormatAttribute = 91
)

const (
	NSOpenGLProfileVersionLegacy  = 0x1000
	NSOpenGLProfileVersion3_2Core = 0x3200
	NSOpenGLProfileVersion4_1Core = 0x4100
)
