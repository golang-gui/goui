package uikit

import (
	"fmt"

	"github.com/ebitengine/purego/objc"
	"github.com/goexlib/cgo"
	cg "github.com/golang-gui/goui/platform/darwin/frameworks/core_graphics"
	. "github.com/golang-gui/goui/platform/darwin/frameworks/foundation"
	"github.com/golang-gui/goui/platform/darwin/frameworks/utils"
)

var framework utils.Framework
var message uintptr
var fnUIApplicationMain func(int32, uintptr, ID, ID) int32

func InitUIKit() (err error) {
	framework, err = utils.LoadSystemFramework("UIKit")
	if err != nil {
		return
	}
	message, err = cgo.Dlsym(cgo.RTLD_DEFAULT, "objc_msgSend")
	if err != nil {
		return
	}
	if err = framework.LoadFunctions([]utils.Function{{Name: "UIApplicationMain", PFunc: &fnUIApplicationMain}}); err != nil {
		return
	}
	initUIResponder()
	initUIApplication()
	initUIApplicationDelegate()
	initUIWindowSceneDelegate()
	initUIView()
	initUIWindow()
	initUIViewController()
	initUIScreen()
	initUIColor()
	return
}

func UIApplicationMain(argc int32, argv uintptr, principal, delegate NSString) int32 {
	return fnUIApplicationMain(argc, argv, principal.ID, delegate.ID)
}

// UIResponder

var UIResponderClassId UIResponderClass

type UIResponderClass struct{ NSObjectClass }
type UIResponder struct{ NSObject }

func initUIResponder() { UIResponderClassId.Class = objc.GetClass("UIResponder") }

// UIApplication

var UIApplicationClassId UIApplicationClass
var UIApplicationSel struct{ SharedApplication objc.SEL }

type UIApplicationClass struct{ UIResponderClass }
type UIApplication struct{ UIResponder }

func initUIApplication() {
	UIApplicationClassId.Class = objc.GetClass("UIApplication")
	UIApplicationSel.SharedApplication = objc.RegisterName("sharedApplication")
}

func (c UIApplicationClass) SharedApplication() UIApplication {
	return Cast[UIApplication](c.Send(UIApplicationSel.SharedApplication))
}

// UIApplicationDelegate

var UIApplicationDelegateSel struct{ ApplicationDidFinishLaunchingWithOptions objc.SEL }

type UIApplicationDelegateClass struct{ UIResponderClass }
type UIApplicationDelegate struct{ UIResponder }

// UIApplicationDelegateOverride implements optional UIApplicationDelegate methods.
// Callback arguments are borrowed. Callbacks run synchronously on the native
// calling thread; the caller owns application policy and thread affinity.
type UIApplicationDelegateOverride struct {
	ApplicationDidFinishLaunchingWithOptions func(self UIApplicationDelegate, application UIApplication, options NSDictionary) bool
}

func initUIApplicationDelegate() {
	UIApplicationDelegateSel.ApplicationDidFinishLaunchingWithOptions = objc.RegisterName("application:didFinishLaunchingWithOptions:")
}

// ImplementUIApplicationDelegate registers a UIResponder subclass conforming to
// UIApplicationDelegate. Nil overrides leave optional methods unimplemented.
func ImplementUIApplicationDelegate(className string, override UIApplicationDelegateOverride) (class UIApplicationDelegateClass, err error) {
	protocol := objc.GetProtocol("UIApplicationDelegate")
	if protocol == nil {
		return class, fmt.Errorf("uikit: missing protocol UIApplicationDelegate")
	}
	methods := make([]objc.MethodDef, 0, 1)
	if override.ApplicationDidFinishLaunchingWithOptions != nil {
		methods = append(methods, objc.MethodDef{
			Cmd: UIApplicationDelegateSel.ApplicationDidFinishLaunchingWithOptions,
			Fn: func(self objc.ID, _ objc.SEL, application, options objc.ID) bool {
				return override.ApplicationDidFinishLaunchingWithOptions(Cast[UIApplicationDelegate](self), Cast[UIApplication](application), Cast[NSDictionary](options))
			},
		})
	}
	class.Class, err = objc.RegisterClass(className, UIResponderClassId.Class, []*objc.Protocol{protocol}, nil, methods)
	return
}

func (c UIApplicationDelegateClass) Alloc() UIApplicationDelegate {
	return Cast[UIApplicationDelegate](c.Send(NSObjectSel.Alloc))
}

// UIScene

// Scene callback arguments are borrowed; storing one beyond its callback needs
// an owned reference. These wrappers do not add window-management policy.
type UIScene struct{ NSObject }
type UIWindowScene struct{ UIScene }
type UISceneSession struct{ NSObject }
type UISceneConnectionOptions struct{ NSObject }

// UIWindowSceneDelegate

var UIWindowSceneDelegateSel struct {
	SceneWillConnectToSessionOptions objc.SEL
	SceneWillEnterForeground         objc.SEL
	SceneDidEnterBackground          objc.SEL
	SceneDidBecomeActive             objc.SEL
	SceneWillResignActive            objc.SEL
	SceneDidDisconnect               objc.SEL
}

type UIWindowSceneDelegateClass struct{ UIResponderClass }
type UIWindowSceneDelegate struct{ UIResponder }

// UIWindowSceneDelegateOverride includes the lifecycle methods inherited from
// UISceneDelegate. Arguments are borrowed and delivered synchronously; the
// caller decides which Scene it owns and how its lifecycle affects resources.
type UIWindowSceneDelegateOverride struct {
	SceneWillConnectToSessionOptions func(self UIWindowSceneDelegate, scene UIScene, session UISceneSession, options UISceneConnectionOptions)
	SceneWillEnterForeground         func(self UIWindowSceneDelegate, scene UIScene)
	SceneDidEnterBackground          func(self UIWindowSceneDelegate, scene UIScene)
	SceneDidBecomeActive             func(self UIWindowSceneDelegate, scene UIScene)
	SceneWillResignActive            func(self UIWindowSceneDelegate, scene UIScene)
	SceneDidDisconnect               func(self UIWindowSceneDelegate, scene UIScene)
}

func initUIWindowSceneDelegate() {
	UIWindowSceneDelegateSel.SceneWillConnectToSessionOptions = objc.RegisterName("scene:willConnectToSession:options:")
	UIWindowSceneDelegateSel.SceneWillEnterForeground = objc.RegisterName("sceneWillEnterForeground:")
	UIWindowSceneDelegateSel.SceneDidEnterBackground = objc.RegisterName("sceneDidEnterBackground:")
	UIWindowSceneDelegateSel.SceneDidBecomeActive = objc.RegisterName("sceneDidBecomeActive:")
	UIWindowSceneDelegateSel.SceneWillResignActive = objc.RegisterName("sceneWillResignActive:")
	UIWindowSceneDelegateSel.SceneDidDisconnect = objc.RegisterName("sceneDidDisconnect:")
}

// ImplementUIWindowSceneDelegate registers a UIResponder subclass conforming to
// UIWindowSceneDelegate. It does not create Scenes or retain callback arguments.
func ImplementUIWindowSceneDelegate(className string, override UIWindowSceneDelegateOverride) (class UIWindowSceneDelegateClass, err error) {
	protocol := objc.GetProtocol("UIWindowSceneDelegate")
	if protocol == nil {
		return class, fmt.Errorf("uikit: missing protocol UIWindowSceneDelegate")
	}
	methods := make([]objc.MethodDef, 0, 6)
	if override.SceneWillConnectToSessionOptions != nil {
		methods = append(methods, objc.MethodDef{
			Cmd: UIWindowSceneDelegateSel.SceneWillConnectToSessionOptions,
			Fn: func(self objc.ID, _ objc.SEL, scene, session, options objc.ID) {
				override.SceneWillConnectToSessionOptions(Cast[UIWindowSceneDelegate](self), Cast[UIScene](scene), Cast[UISceneSession](session), Cast[UISceneConnectionOptions](options))
			},
		})
	}
	if override.SceneWillEnterForeground != nil {
		methods = append(methods, objc.MethodDef{
			Cmd: UIWindowSceneDelegateSel.SceneWillEnterForeground,
			Fn: func(self objc.ID, _ objc.SEL, scene objc.ID) {
				override.SceneWillEnterForeground(Cast[UIWindowSceneDelegate](self), Cast[UIScene](scene))
			},
		})
	}
	if override.SceneDidEnterBackground != nil {
		methods = append(methods, objc.MethodDef{
			Cmd: UIWindowSceneDelegateSel.SceneDidEnterBackground,
			Fn: func(self objc.ID, _ objc.SEL, scene objc.ID) {
				override.SceneDidEnterBackground(Cast[UIWindowSceneDelegate](self), Cast[UIScene](scene))
			},
		})
	}
	if override.SceneDidBecomeActive != nil {
		methods = append(methods, objc.MethodDef{
			Cmd: UIWindowSceneDelegateSel.SceneDidBecomeActive,
			Fn: func(self objc.ID, _ objc.SEL, scene objc.ID) {
				override.SceneDidBecomeActive(Cast[UIWindowSceneDelegate](self), Cast[UIScene](scene))
			},
		})
	}
	if override.SceneWillResignActive != nil {
		methods = append(methods, objc.MethodDef{
			Cmd: UIWindowSceneDelegateSel.SceneWillResignActive,
			Fn: func(self objc.ID, _ objc.SEL, scene objc.ID) {
				override.SceneWillResignActive(Cast[UIWindowSceneDelegate](self), Cast[UIScene](scene))
			},
		})
	}
	if override.SceneDidDisconnect != nil {
		methods = append(methods, objc.MethodDef{
			Cmd: UIWindowSceneDelegateSel.SceneDidDisconnect,
			Fn: func(self objc.ID, _ objc.SEL, scene objc.ID) {
				override.SceneDidDisconnect(Cast[UIWindowSceneDelegate](self), Cast[UIScene](scene))
			},
		})
	}
	class.Class, err = objc.RegisterClass(className, UIResponderClassId.Class, []*objc.Protocol{protocol}, nil, methods)
	return
}

func (c UIWindowSceneDelegateClass) Alloc() UIWindowSceneDelegate {
	return Cast[UIWindowSceneDelegate](c.Send(NSObjectSel.Alloc))
}

// UIView

var UIViewClassId UIViewClass
var UIViewSel struct{ InitWithFrame, Bounds, Layer, SetHidden, Hidden, SetContentScaleFactor, SetBackgroundColor objc.SEL }

type UIViewClass struct{ UIResponderClass }
type UIView struct{ UIResponder }

func initUIView() {
	UIViewClassId.Class = objc.GetClass("UIView")
	UIViewSel.InitWithFrame = objc.RegisterName("initWithFrame:")
	UIViewSel.Bounds = objc.RegisterName("bounds")
	UIViewSel.Layer = objc.RegisterName("layer")
	UIViewSel.SetHidden = objc.RegisterName("setHidden:")
	UIViewSel.Hidden = objc.RegisterName("isHidden")
	UIViewSel.SetContentScaleFactor = objc.RegisterName("setContentScaleFactor:")
	UIViewSel.SetBackgroundColor = objc.RegisterName("setBackgroundColor:")
}

func (c UIViewClass) Alloc() UIView { return Cast[UIView](c.Send(NSObjectSel.Alloc)) }
func (v UIView) InitWithFrame(rect cg.CGRect) UIView {
	return Cast[UIView](cgo.CallRet[ID](message, v.ID, UIViewSel.InitWithFrame, rect))
}
func (v UIView) Bounds() cg.CGRect     { return cgo.CallRet[cg.CGRect](message, v.ID, UIViewSel.Bounds) }
func (v UIView) Layer() ID             { return v.Send(UIViewSel.Layer) }
func (v UIView) SetHidden(hidden bool) { v.Send(UIViewSel.SetHidden, hidden) }
func (v UIView) Hidden() bool          { return objc.Send[bool](v.ID, UIViewSel.Hidden) }
func (v UIView) SetContentScaleFactor(scale float64) {
	cgo.Call(message, v.ID, UIViewSel.SetContentScaleFactor, scale)
}

func (v UIView) SetBackgroundColor(color UIColor) { v.Send(UIViewSel.SetBackgroundColor, color.ID) }

// UIWindow

var UIWindowClassId UIWindowClass
var UIWindowSel struct{ InitWithWindowScene, SetRootViewController, MakeKeyAndVisible, Screen, IsKeyWindow objc.SEL }

type UIWindowClass struct{ UIViewClass }
type UIWindow struct{ UIView }

func initUIWindow() {
	UIWindowClassId.Class = objc.GetClass("UIWindow")
	UIWindowSel.InitWithWindowScene = objc.RegisterName("initWithWindowScene:")
	UIWindowSel.SetRootViewController = objc.RegisterName("setRootViewController:")
	UIWindowSel.MakeKeyAndVisible = objc.RegisterName("makeKeyAndVisible")
	UIWindowSel.Screen = objc.RegisterName("screen")
	UIWindowSel.IsKeyWindow = objc.RegisterName("isKeyWindow")
}

func (c UIWindowClass) Alloc() UIWindow { return Cast[UIWindow](c.Send(NSObjectSel.Alloc)) }
func (w UIWindow) InitWithWindowScene(scene UIWindowScene) UIWindow {
	return Cast[UIWindow](w.Send(UIWindowSel.InitWithWindowScene, scene.ID))
}
func (w UIWindow) SetRootViewController(controller UIViewController) {
	w.Send(UIWindowSel.SetRootViewController, controller.ID)
}
func (w UIWindow) MakeKeyAndVisible() { w.Send(UIWindowSel.MakeKeyAndVisible) }
func (w UIWindow) Screen() UIScreen   { return Cast[UIScreen](w.Send(UIWindowSel.Screen)) }
func (w UIWindow) IsKeyWindow() bool  { return objc.Send[bool](w.ID, UIWindowSel.IsKeyWindow) }

// UIViewController

var UIViewControllerClassId UIViewControllerClass
var UIViewControllerSel struct{ View, SetView, ViewDidLayoutSubviews objc.SEL }

type UIViewControllerClass struct{ UIResponderClass }
type UIViewController struct{ UIResponder }

// UIViewControllerOverride overrides native methods without automatically
// invoking their superclass implementation. The subclass owns that decision.
type UIViewControllerOverride struct {
	ViewDidLayoutSubviews func(self UIViewController)
}

func initUIViewController() {
	UIViewControllerClassId.Class = objc.GetClass("UIViewController")
	UIViewControllerSel.View = objc.RegisterName("view")
	UIViewControllerSel.SetView = objc.RegisterName("setView:")
	UIViewControllerSel.ViewDidLayoutSubviews = objc.RegisterName("viewDidLayoutSubviews")
}

// ImplementUIViewController registers a UIViewController subclass. Callbacks
// receive borrowed objects and execute synchronously on the native caller.
func ImplementUIViewController(className string, override UIViewControllerOverride) (class UIViewControllerClass, err error) {
	methods := make([]objc.MethodDef, 0, 1)
	if override.ViewDidLayoutSubviews != nil {
		methods = append(methods, objc.MethodDef{
			Cmd: UIViewControllerSel.ViewDidLayoutSubviews,
			Fn: func(self objc.ID, _ objc.SEL) {
				override.ViewDidLayoutSubviews(Cast[UIViewController](self))
			},
		})
	}
	class.Class, err = objc.RegisterClass(className, UIViewControllerClassId.Class, nil, nil, methods)
	return
}

func (c UIViewControllerClass) Alloc() UIViewController {
	return Cast[UIViewController](c.Send(NSObjectSel.Alloc))
}
func (v UIViewController) Init() UIViewController {
	return Cast[UIViewController](v.Send(NSObjectSel.Init))
}
func (v UIViewController) View() UIView        { return Cast[UIView](v.Send(UIViewControllerSel.View)) }
func (v UIViewController) SetView(view UIView) { v.Send(UIViewControllerSel.SetView, view.ID) }

// UIScreen

var UIScreenClassId UIScreenClass
var UIScreenSel struct{ MainScreen, Scale objc.SEL }

type UIScreenClass struct{ NSObjectClass }
type UIScreen struct{ NSObject }

func initUIScreen() {
	UIScreenClassId.Class = objc.GetClass("UIScreen")
	UIScreenSel.MainScreen = objc.RegisterName("mainScreen")
	UIScreenSel.Scale = objc.RegisterName("scale")
}

func (c UIScreenClass) MainScreen() UIScreen { return Cast[UIScreen](c.Send(UIScreenSel.MainScreen)) }
func (s UIScreen) Scale() float64            { return cgo.CallRet[float64](message, s.ID, UIScreenSel.Scale) }

// UIColor

var UIColorClassId UIColorClass
var UIColorSel struct{ BlackColor objc.SEL }

type UIColorClass struct{ NSObjectClass }
type UIColor struct{ NSObject }

func initUIColor() {
	UIColorClassId.Class = objc.GetClass("UIColor")
	UIColorSel.BlackColor = objc.RegisterName("blackColor")
}

func (c UIColorClass) BlackColor() UIColor { return Cast[UIColor](c.Send(UIColorSel.BlackColor)) }
