package uikit

import (
	"github.com/ebitengine/purego/objc"
	"github.com/goexlib/cgo"
	cg "github.com/golang-gui/goui/platform/darwin/frameworks/core_graphics"
	. "github.com/golang-gui/goui/platform/darwin/frameworks/foundation"
	"github.com/golang-gui/goui/platform/darwin/frameworks/utils"
)

var framework utils.Framework
var message uintptr
var fnUIApplicationMain func(int32, uintptr, ID, ID) int32
var UIViewClassId UIViewClass
var UIWindowClassId UIWindowClass
var UIViewControllerClassId UIViewControllerClass
var UIScreenClassId UIScreenClass
var UIColorClassId UIColorClass
var UIColorSel struct{ BlackColor objc.SEL }
var UIViewSel struct{ InitWithFrame, Bounds, Layer, SetHidden, Hidden, SetContentScaleFactor, SetBackgroundColor objc.SEL }
var UIWindowSel struct{ InitWithWindowScene, SetRootViewController, MakeKeyAndVisible, Screen, IsKeyWindow objc.SEL }
var UIViewControllerSel struct{ View, SetView objc.SEL }
var UIScreenSel struct{ MainScreen, Scale objc.SEL }

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
	UIViewClassId.Class = objc.GetClass("UIView")
	UIWindowClassId.Class = objc.GetClass("UIWindow")
	UIViewControllerClassId.Class = objc.GetClass("UIViewController")
	UIScreenClassId.Class = objc.GetClass("UIScreen")
	UIColorClassId.Class = objc.GetClass("UIColor")
	UIColorSel.BlackColor = objc.RegisterName("blackColor")
	UIViewSel.InitWithFrame = objc.RegisterName("initWithFrame:")
	UIViewSel.Bounds = objc.RegisterName("bounds")
	UIViewSel.Layer = objc.RegisterName("layer")
	UIViewSel.SetHidden = objc.RegisterName("setHidden:")
	UIViewSel.Hidden = objc.RegisterName("isHidden")
	UIViewSel.SetContentScaleFactor = objc.RegisterName("setContentScaleFactor:")
	UIViewSel.SetBackgroundColor = objc.RegisterName("setBackgroundColor:")
	UIWindowSel.InitWithWindowScene = objc.RegisterName("initWithWindowScene:")
	UIWindowSel.SetRootViewController = objc.RegisterName("setRootViewController:")
	UIWindowSel.MakeKeyAndVisible = objc.RegisterName("makeKeyAndVisible")
	UIWindowSel.Screen = objc.RegisterName("screen")
	UIWindowSel.IsKeyWindow = objc.RegisterName("isKeyWindow")
	UIViewControllerSel.View = objc.RegisterName("view")
	UIViewControllerSel.SetView = objc.RegisterName("setView:")
	UIScreenSel.MainScreen = objc.RegisterName("mainScreen")
	UIScreenSel.Scale = objc.RegisterName("scale")
	return
}
func UIApplicationMain(argc int32, argv uintptr, principal, delegate NSString) int32 {
	return fnUIApplicationMain(argc, argv, principal.ID, delegate.ID)
}

type UIViewClass struct{ NSObjectClass }
type UIView struct{ NSObject }
type UIWindowClass struct{ NSObjectClass }
type UIWindow struct{ UIView }
type UIViewControllerClass struct{ NSObjectClass }
type UIViewController struct{ NSObject }
type UIScreenClass struct{ NSObjectClass }
type UIScreen struct{ NSObject }

func (c UIViewClass) Alloc() UIView     { return Cast[UIView](c.Send(NSObjectSel.Alloc)) }
func (c UIWindowClass) Alloc() UIWindow { return Cast[UIWindow](c.Send(NSObjectSel.Alloc)) }
func (c UIViewControllerClass) Alloc() UIViewController {
	return Cast[UIViewController](c.Send(NSObjectSel.Alloc))
}
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
func (w UIWindow) InitWithWindowScene(scene ID) UIWindow {
	return Cast[UIWindow](w.Send(UIWindowSel.InitWithWindowScene, scene))
}
func (w UIWindow) SetRootViewController(controller UIViewController) {
	w.Send(UIWindowSel.SetRootViewController, controller.ID)
}
func (w UIWindow) MakeKeyAndVisible()          { w.Send(UIWindowSel.MakeKeyAndVisible) }
func (w UIWindow) Screen() UIScreen            { return Cast[UIScreen](w.Send(UIWindowSel.Screen)) }
func (w UIWindow) IsKeyWindow() bool           { return objc.Send[bool](w.ID, UIWindowSel.IsKeyWindow) }
func (v UIViewController) View() UIView        { return Cast[UIView](v.Send(UIViewControllerSel.View)) }
func (v UIViewController) SetView(view UIView) { v.Send(UIViewControllerSel.SetView, view.ID) }
func (c UIScreenClass) MainScreen() UIScreen   { return Cast[UIScreen](c.Send(UIScreenSel.MainScreen)) }
func (s UIScreen) Scale() float64              { return cgo.CallRet[float64](message, s.ID, UIScreenSel.Scale) }

type UIColorClass struct{ NSObjectClass }
type UIColor struct{ NSObject }

func (c UIColorClass) BlackColor() UIColor        { return Cast[UIColor](c.Send(UIColorSel.BlackColor)) }
func (v UIView) SetBackgroundColor(color UIColor) { v.Send(UIViewSel.SetBackgroundColor, color.ID) }
