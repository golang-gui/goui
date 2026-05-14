package glx

import (
	"github.com/goexlib/cgo"
	"github.com/golang-gui/goui/platform/x11/libs/xlib"
	"runtime"
)

var (
	libgl = cgo.NewLazyLibrary("libGL.so.1")

	glXGetFBConfigs          = libgl.NewSymbol("glXGetFBConfigs")
	glXGetFBConfigAttrib     = libgl.NewSymbol("glXGetFBConfigAttrib")
	glXGetClientString       = libgl.NewSymbol("glXGetClientString")
	glXQueryExtension        = libgl.NewSymbol("glXQueryExtension")
	glXQueryVersion          = libgl.NewSymbol("glXQueryVersion")
	glXDestroyContext        = libgl.NewSymbol("glXDestroyContext")
	glXMakeCurrent           = libgl.NewSymbol("glXMakeCurrent")
	glXSwapBuffers           = libgl.NewSymbol("glXSwapBuffers")
	glXQueryExtensionsString = libgl.NewSymbol("glXQueryExtensionsString")
	glXCreateNewContext      = libgl.NewSymbol("glXCreateNewContext")
	glXCreateWindow          = libgl.NewSymbol("glXCreateWindow")
	glXDestroyWindow         = libgl.NewSymbol("glXDestroyWindow")
	glXGetVisualFromFBConfig = libgl.NewSymbol("glXGetVisualFromFBConfig")
	glXGetProcAddress        = libgl.NewSymbol("glXGetProcAddress")
	glXGetProcAddressARB     = libgl.NewSymbol("glXGetProcAddressARB")

	glXSwapIntervalEXT         cgo.Symbol
	glXSwapIntervalSGI         cgo.Symbol
	glXSwapIntervalMESA        cgo.Symbol
	glXCreateContextAttribsARB cgo.Symbol
)

func init() {
	glXSwapIntervalEXT = getProcAddress("glXSwapIntervalEXT")
	glXSwapIntervalSGI = getProcAddress("glXSwapIntervalSGI")
	glXSwapIntervalMESA = getProcAddress("glXSwapIntervalMESA")
	glXCreateContextAttribsARB = getProcAddress("glXCreateContextAttribsARB")
}

func GetFBConfigs(display xlib.Display, screen int) []FBConfig {
	//FBConfig*(Display dpy, int screen, int* count)
	count := 0
	configs, _, _ := glXGetFBConfigs.CallRaw(uintptr(display), uintptr(screen), uintptr(cgo.Pointer(&count)))
	if configs != 0 {
		return cgo.GoSliceN[FBConfig](cgo.Pointer(configs), count)
	}
	return nil
}

func GetFBConfigAttrib(display xlib.Display, config FBConfig, attr int) (value int, ok bool) {
	//int(Display dpy, FBConfig config, int attr, int* value)
	ret, _, err := glXGetFBConfigAttrib.CallRaw(uintptr(display), uintptr(config), uintptr(attr), uintptr(cgo.Pointer(&value)))
	ok = err == nil && ret == 0
	return
}

func GetClientString(display xlib.Display, name int) string {
	//const char*(Display dpy, int name)
	ret, _, _ := glXGetClientString.CallRaw(uintptr(display), uintptr(name))
	if ret != 0 {
		return cgo.GoString(cgo.Pointer(ret))
	}
	return ""
}

func QueryExtension(display xlib.Display) (support bool, errorBase, eventBase int) {
	//int(Display dpy, int* errorBase, int* eventBase)
	ret, _, _ := glXQueryExtension.CallRaw(uintptr(display), uintptr(cgo.Pointer(&errorBase)), uintptr(cgo.Pointer(&eventBase)))
	support = ret != 0
	return
}

func QueryVersion(display xlib.Display) (ok bool, major, minor int) {
	//bool(Display dpy, int* major, int* minor)
	ret, _, _ := glXQueryVersion.CallRaw(uintptr(display), uintptr(cgo.Pointer(&major)), uintptr(cgo.Pointer(&minor)))
	ok = ret != 0
	return
}

func DestroyContext(display xlib.Display, ctx Context) {
	//void(Display dpy, Context ctx)
	glXDestroyContext.CallRaw(uintptr(display), uintptr(ctx))
}

func MakeCurrent(display xlib.Display, drawable xlib.Drawable, ctx Context) bool {
	//bool(Display dpy, Drawable d, Context ctx)
	ret, _, _ := glXMakeCurrent.CallRaw(uintptr(display), uintptr(drawable), uintptr(ctx))
	return ret != 0
}

func SwapBuffers(display xlib.Display, drawable xlib.Drawable) {
	//void(Display dpy, Drawable d)
	glXSwapBuffers.CallRaw(uintptr(display), uintptr(drawable))
}

func QueryExtensionsString(display xlib.Display, screen int) string {
	//const char*(Display dpy, int screen)
	ret, _, _ := glXQueryExtensionsString.CallRaw(uintptr(display), uintptr(screen))
	if ret != 0 {
		return cgo.GoString(cgo.Pointer(ret))
	}
	return ""
}

func CreateNewContext(display xlib.Display, config FBConfig, renderType int, shareList Context, direct bool) Context {
	//Context*(Display dpy, FBConfig config, int renderType, Context shareList, bool direct)
	ret, _, _ := glXCreateNewContext.CallRaw(uintptr(display), uintptr(config), uintptr(renderType), uintptr(shareList), uintptr(cgo.CBool(direct)))
	return Context(ret)
}

func CreateWindow(display xlib.Display, config FBConfig, win xlib.Window) Window {
	//Window(Display dpy, FBConfig config, Window win, const int* attrs)
	ret, _, _ := glXCreateWindow.CallRaw(uintptr(display), uintptr(config), uintptr(win), 0)
	return Window(ret)
}

func DestroyWindow(display xlib.Display, win Window) {
	//void(Display dpy, GLXWindow win)
	glXDestroyWindow.CallRaw(uintptr(display), uintptr(win))
}

func GetVisualFromFBConfig(display xlib.Display, config FBConfig) *xlib.VisualInfo {
	//XVisualInfo*(Display dpy, FBConfig config)
	ret, _, _ := glXGetVisualFromFBConfig.CallRaw(uintptr(display), uintptr(config))
	return (*xlib.VisualInfo)(cgo.Pointer(ret))
}

func SwapIntervalEXT(display xlib.Display, drawable xlib.Drawable, interval int) {
	//void(Display dpy, Drawable d, int interval)
	if glXSwapIntervalEXT != 0 {
		glXSwapIntervalEXT.CallRaw(uintptr(display), uintptr(drawable), uintptr(interval))
	}
}

func SwapIntervalSGI(interval int) int {
	//int(int)
	if glXSwapIntervalSGI != 0 {
		ret, _, _ := glXSwapIntervalSGI.CallRaw(uintptr(interval))
		return int(ret)
	}
	return 0
}

func SwapIntervalMESA(interval int) int {
	//int(int)
	if glXSwapIntervalMESA != 0 {
		ret, _, _ := glXSwapIntervalMESA.CallRaw(uintptr(interval))
		return int(ret)
	}
	return 0
}

func CreateContextAttribsARB(display xlib.Display, config FBConfig, shareList Context, direct bool, attrs []int32) Context {
	//Context(Display dpy, FBConfig config, Context shareList, bool direct, const int* attrs)
	ret, _, _ := glXCreateContextAttribsARB.CallRaw(uintptr(display), uintptr(config), uintptr(shareList), uintptr(cgo.CBool(direct)), uintptr(cgo.CSlice(attrs)))
	return Context(ret)
}

func GetProcAddress(name string) (proc uintptr, err error) {
	cName := cgo.CString(name)
	proc, _, err = glXGetProcAddress.CallRaw(uintptr(cName))
	if err != nil {
		proc, _, err = glXGetProcAddressARB.CallRaw(uintptr(cName))
		if err != nil {
			err = libgl.Load()
			if err != nil {
				return 0, err
			}
			return cgo.Dlsym(libgl.Handle(), cgo.GoStringNTemp(cName, len(name)+1))
		}
	}
	runtime.KeepAlive(cName)
	return
}

func getProcAddress(name string) cgo.Symbol {
	proc, _ := GetProcAddress(name)
	return cgo.Symbol(proc)
}
