package opengles

import (
	"github.com/ebitengine/purego/objc"
	"github.com/goexlib/cgo"
	. "github.com/golang-gui/goui/platform/darwin/frameworks/foundation"
	"github.com/golang-gui/goui/platform/darwin/frameworks/utils"
)

var framework utils.Framework
var EAGLContextClassId EAGLContextClass
var EAGLContextSel struct{ InitWithAPI, SetCurrentContext, RenderbufferStorage, PresentRenderbuffer objc.SEL }

func InitOpenGLES() (err error) {
	framework, err = utils.LoadSystemFramework("OpenGLES")
	if err != nil {
		return
	}
	EAGLContextClassId.Class = objc.GetClass("EAGLContext")
	EAGLContextSel.InitWithAPI = objc.RegisterName("initWithAPI:")
	EAGLContextSel.SetCurrentContext = objc.RegisterName("setCurrentContext:")
	EAGLContextSel.RenderbufferStorage = objc.RegisterName("renderbufferStorage:fromDrawable:")
	EAGLContextSel.PresentRenderbuffer = objc.RegisterName("presentRenderbuffer:")
	return framework.LoadFunctions([]utils.Function{
		{Name: "glGenFramebuffers", PFunc: &GenFramebuffers}, {Name: "glBindFramebuffer", PFunc: &BindFramebuffer}, {Name: "glDeleteFramebuffers", PFunc: &DeleteFramebuffers},
		{Name: "glGenRenderbuffers", PFunc: &GenRenderbuffers}, {Name: "glBindRenderbuffer", PFunc: &BindRenderbuffer}, {Name: "glDeleteRenderbuffers", PFunc: &DeleteRenderbuffers},
		{Name: "glRenderbufferStorage", PFunc: &RenderbufferStorage}, {Name: "glGetRenderbufferParameteriv", PFunc: &GetRenderbufferParameteriv},
		{Name: "glFramebufferRenderbuffer", PFunc: &FramebufferRenderbuffer}, {Name: "glCheckFramebufferStatus", PFunc: &CheckFramebufferStatus},
		{Name: "glFinish", PFunc: &Finish}, {Name: "glGetString", PFunc: &GetString}})
}

type EAGLContextClass struct{ NSObjectClass }
type EAGLContext struct{ NSObject }

func (c EAGLContextClass) Alloc() EAGLContext { return Cast[EAGLContext](c.Send(NSObjectSel.Alloc)) }
func (c EAGLContextClass) SetCurrentContext(context EAGLContext) bool {
	return objc.Send[bool](c.ID(), EAGLContextSel.SetCurrentContext, context.ID)
}
func (c EAGLContext) InitWithAPI(api uint) EAGLContext {
	return Cast[EAGLContext](c.Send(EAGLContextSel.InitWithAPI, api))
}
func (c EAGLContext) RenderbufferStorage(target uint32, layer ID) bool {
	return objc.Send[bool](c.ID, EAGLContextSel.RenderbufferStorage, target, layer)
}
func (c EAGLContext) PresentRenderbuffer(target uint32) bool {
	return objc.Send[bool](c.ID, EAGLContextSel.PresentRenderbuffer, target)
}
func GetProcAddress(name string) (uintptr, error) { return framework.GetSymbol(name) }

var (
	GenFramebuffers            func(int32, *uint32)
	BindFramebuffer            func(uint32, uint32)
	DeleteFramebuffers         func(int32, *uint32)
	GenRenderbuffers           func(int32, *uint32)
	BindRenderbuffer           func(uint32, uint32)
	DeleteRenderbuffers        func(int32, *uint32)
	RenderbufferStorage        func(uint32, uint32, int32, int32)
	GetRenderbufferParameteriv func(uint32, uint32, *int32)
	FramebufferRenderbuffer    func(uint32, uint32, uint32, uint32)
	CheckFramebufferStatus     func(uint32) uint32
	Finish                     func()
	GetString                  func(uint32) cgo.Pointer
)
