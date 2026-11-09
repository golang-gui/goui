// UIKit 绑定与 iOS 宿主生命周期验收；依赖真实 UIKit 主线程和原生事件循环。
// 环境、构建、初始状态、操作、断言、差异、副作用与复位详见 README.md。
// 启动后自动检查协议、可选方法、参数和 BOOL 返回，再检查延迟原生回调及
// 交付/目标撤销回调中的销毁。仅写自身 Documents/IOSBindings，不使用 GUI/DevServer。
package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"github.com/ebitengine/purego/objc"
	"github.com/golang-gui/goui/platform"
	. "github.com/golang-gui/goui/platform/darwin/frameworks/foundation"
	ui "github.com/golang-gui/goui/platform/darwin/frameworks/uikit"
	"github.com/golang-gui/goui/platform/darwin/libs/dispatch"
	"github.com/golang-gui/goui/platform/events"
	"github.com/golang-gui/goui/platform/graphics"
)

var painterName = "software"

type application struct {
	backend                       platform.Platform
	loop                          platform.EventLoop
	source                        platform.WindowSource
	window                        platform.Window
	painter                       graphics.Painter
	phase                         string
	deliveries, destroyed, frames int
	size                          events.SizeEvent
	checks                        []string
}

func must(err error) {
	if err != nil {
		panic(err)
	}
}
func require(ok bool, message string) {
	if !ok {
		panic(message)
	}
}

func main() {
	require(!dispatch.IsMainThread(), "启动 goroutine 不得占用原生主线程")
	must(os.Setenv("GOUI_PLAT_PAINTER", painterName))
	a := &application{}
	var err error
	a.backend, err = platform.NewPlatform(platform.DefaultName(), "org.golanggui.IOSBindings")
	must(err)
	a.loop, err = a.backend.NewEventLoop()
	must(err)
	a.source = a.backend.(platform.WindowSource)
	must(a.source.SetWindowHandler(a.start))
	a.loop.Run()
	a.loop.Destroy()
	require(a.phase == "complete" && a.deliveries == 3 && a.destroyed == 2 && a.frames >= 2, "验收阶段未全部完成")
	a.save(true)
}

func (a *application) start(w platform.Window) platform.EventHandler {
	require(dispatch.IsMainThread(), "宿主交付不在主线程")
	a.deliveries++
	if a.deliveries == 1 {
		w.Destroy()
		a.checks = append(a.checks, "destroy-during-delivery")
		a.loop.Post(func() { must(a.source.SetWindowHandler(a.start)) })
		return func(platform.Event) { panic("已销毁的交付仍安装事件处理函数") }
	}
	require(a.deliveries <= 3, "同一宿主被意外重复交付")
	a.window, a.painter, a.size = w, nil, events.SizeEvent{}
	a.phase = "painting"
	return a.event
}

func (a *application) event(event platform.Event) {
	require(dispatch.IsMainThread(), "原生观察或绘制不在主线程")
	switch e := event.(type) {
	case events.SizeEvent:
		a.size = e
	case events.DestroyEvent:
		a.destroyed++
	case events.SurfaceEvent:
		if !e.State.Ready && (a.phase == "destroy-on-revoke" || a.phase == "destroy-platform-on-revoke") {
			if a.painter != nil {
				a.painter.Destroy()
				a.painter = nil
			}
			if a.phase == "destroy-platform-on-revoke" {
				a.backend.Destroy()
			} else {
				a.window.Destroy()
			}
		}
	case events.PaintEvent:
		if a.painter == nil {
			var err error
			a.painter, err = a.backend.NewPainter(a.window)
			must(err)
			want := "Software"
			if painterName == "opengl" {
				want = "OpenGL"
			}
			require(a.painter.Name() == want, "指定绘制后端被替换")
		}
		a.painter.Begin(a.size.PixelWidth, a.size.PixelHeight, a.size.PixelWidth/a.size.Width)
		a.painter.Clear(graphics.RGB(244, 247, 250))
		a.painter.FillRect(graphics.Rect(24, 80, 160, 80), graphics.RGB(32, 144, 96))
		a.painter.End()
		a.frames++
		if a.phase == "painting" {
			a.phase = "checking"
			a.loop.Post(a.check)
		}
	}
}

func (a *application) check() {
	require(dispatch.IsMainThread(), "验收不在主线程")
	view := objc.ID(a.window.NativeHandle())
	nativeWindow := view.Send(objc.RegisterName("window"))
	scene := nativeWindow.Send(objc.RegisterName("windowScene"))
	delegate := scene.Send(objc.RegisterName("delegate"))
	require(view != 0 && scene != 0 && delegate != 0, "缺少原生 Scene 或 delegate")

	if a.deliveries == 2 {
		a.checks = append(a.checks, checkBindings()...)
		before, eventsBefore := a.window.SurfaceState(), a.destroyed
		// nil 是明确不属于当前宿主的来源。旧实现丢弃来源参数，会撤销甚至释放
		// 当前窗口；这里经真实 Objective-C 方法桥接，不能被 GUI 事件注入替代。
		for _, selector := range []objc.SEL{
			ui.UIWindowSceneDelegateSel.SceneWillEnterForeground,
			ui.UIWindowSceneDelegateSel.SceneDidEnterBackground,
			ui.UIWindowSceneDelegateSel.SceneDidBecomeActive,
			ui.UIWindowSceneDelegateSel.SceneWillResignActive,
			ui.UIWindowSceneDelegateSel.SceneDidDisconnect,
		} {
			delegate.Send(selector, objc.ID(0))
		}
		require(a.window.SurfaceState() == before && a.destroyed == eventsBefore, "无关 Scene 回调修改了当前目标")
		// 旧 controller 的 layout 回调也不能重新发布当前宿主的目标。
		class := ui.UIViewControllerClass{}
		class.Class = objc.GetClass("GoUIContent")
		old := class.Alloc().Init()
		old.Send(ui.UIViewControllerSel.ViewDidLayoutSubviews)
		old.Release()
		require(a.window.SurfaceState() == before, "旧 controller 的 layout 修改了当前目标")
		a.checks = append(a.checks, "unrelated-scene-and-controller-ignored")
		a.phase = "destroy-on-revoke"
		delegate.Send(ui.UIWindowSceneDelegateSel.SceneWillEnterForeground, scene)
		require(!a.window.SurfaceState().Ready && a.destroyed == 1, "撤销回调中的窗口销毁未完成")
		a.checks = append(a.checks, "destroy-during-target-revoke")
		a.loop.Post(func() { must(a.source.SetWindowHandler(a.start)) })
	} else {
		a.phase = "destroy-platform-on-revoke"
		delegate.Send(ui.UIWindowSceneDelegateSel.SceneDidEnterBackground, scene)
		require(!a.window.SurfaceState().Ready && a.destroyed == 2, "撤销回调中的平台销毁未完成")
		a.checks = append(a.checks, "rebind-and-platform-destroy")
		a.phase = "complete"
		a.save(false)
		a.loop.Quit()
	}
}

func checkBindings() []string {
	require(dispatch.IsMainThread(), "UIKit 绑定调用不在主线程")
	application := ui.UIApplicationClassId.SharedApplication()
	dictionary := Cast[NSDictionary](objc.ID(objc.GetClass("NSDictionary")).Send(objc.RegisterName("dictionary")))
	launches := 0
	var receiver ID
	appClass, err := ui.ImplementUIApplicationDelegate("GouiTestApplicationDelegate", ui.UIApplicationDelegateOverride{
		ApplicationDidFinishLaunchingWithOptions: func(self ui.UIApplicationDelegate, app ui.UIApplication, options NSDictionary) bool {
			require(self.ID == receiver && app.ID == application.ID && options.ID == dictionary.ID, "应用回调参数丢失")
			launches++
			return launches == 2
		},
	})
	must(err)
	app := appClass.Alloc()
	app.Send(NSObjectSel.Init)
	defer app.Release()
	receiver = app.ID
	conforms := objc.RegisterName("conformsToProtocol:")
	require(objc.Send[bool](app.ID, conforms, objc.GetProtocol("UIApplicationDelegate")), "应用 delegate 未声明协议")
	for _, expected := range []bool{false, true} {
		result := objc.Send[bool](app.ID, ui.UIApplicationDelegateSel.ApplicationDidFinishLaunchingWithOptions, application.ID, dictionary.ID)
		require(result == expected, "应用回调 BOOL 返回不正确")
	}
	emptyAppClass, err := ui.ImplementUIApplicationDelegate("GouiTestEmptyApplicationDelegate", ui.UIApplicationDelegateOverride{})
	must(err)
	emptyApp := emptyAppClass.Alloc()
	emptyApp.Send(NSObjectSel.Init)
	defer emptyApp.Release()
	require(!emptyApp.RespondsToSelector("application:didFinishLaunchingWithOptions:"), "nil override 不应安装可选方法")

	// 仅验证原生消息到 Go 回调的身份传递，不将这些合成参数当作系统 Scene 事件。
	token := NSObjectClassId.Alloc()
	token.Send(NSObjectSel.Init)
	defer token.Release()
	sceneID := token.ID
	calls := 0
	var sceneReceiver ID
	onScene := func(self ui.UIWindowSceneDelegate, scene ui.UIScene) {
		require(self.ID == sceneReceiver && scene.ID == sceneID, "Scene 回调身份丢失")
		calls++
	}
	sceneClass, err := ui.ImplementUIWindowSceneDelegate("GouiTestSceneDelegate", ui.UIWindowSceneDelegateOverride{
		SceneWillConnectToSessionOptions: func(self ui.UIWindowSceneDelegate, scene ui.UIScene, session ui.UISceneSession, options ui.UISceneConnectionOptions) {
			onScene(self, scene)
			require(session.ID == dictionary.ID && options.ID == application.ID, "Scene 连接参数丢失")
		},
		SceneWillEnterForeground: onScene,
		SceneDidEnterBackground:  onScene,
		SceneDidBecomeActive:     onScene,
		SceneWillResignActive:    onScene,
		SceneDidDisconnect:       onScene,
	})
	must(err)
	sceneDelegate := sceneClass.Alloc()
	sceneDelegate.Send(NSObjectSel.Init)
	defer sceneDelegate.Release()
	sceneReceiver = sceneDelegate.ID
	require(objc.Send[bool](sceneDelegate.ID, conforms, objc.GetProtocol("UIWindowSceneDelegate")), "Scene delegate 未声明协议")
	sceneDelegate.Send(ui.UIWindowSceneDelegateSel.SceneWillConnectToSessionOptions, sceneID, dictionary.ID, application.ID)
	for _, selector := range []objc.SEL{
		ui.UIWindowSceneDelegateSel.SceneWillEnterForeground,
		ui.UIWindowSceneDelegateSel.SceneDidEnterBackground,
		ui.UIWindowSceneDelegateSel.SceneDidBecomeActive,
		ui.UIWindowSceneDelegateSel.SceneWillResignActive,
		ui.UIWindowSceneDelegateSel.SceneDidDisconnect,
	} {
		sceneDelegate.Send(selector, sceneID)
	}
	require(calls == 6, "Scene 回调未完整执行")
	emptySceneClass, err := ui.ImplementUIWindowSceneDelegate("GouiTestEmptySceneDelegate", ui.UIWindowSceneDelegateOverride{})
	must(err)
	emptyScene := emptySceneClass.Alloc()
	emptyScene.Send(NSObjectSel.Init)
	defer emptyScene.Release()
	for _, selector := range []string{"scene:willConnectToSession:options:", "sceneWillEnterForeground:", "sceneDidEnterBackground:", "sceneDidBecomeActive:", "sceneWillResignActive:", "sceneDidDisconnect:"} {
		require(!emptyScene.RespondsToSelector(selector), "nil Scene override 不应安装方法："+selector)
	}
	layouts := 0
	var controllerID ID
	controllerClass, err := ui.ImplementUIViewController("GouiTestViewController", ui.UIViewControllerOverride{
		ViewDidLayoutSubviews: func(self ui.UIViewController) {
			require(self.ID == controllerID, "controller 回调接收者丢失")
			layouts++
			objc.SendSuper[uintptr](self.ID, ui.UIViewControllerSel.ViewDidLayoutSubviews)
		},
	})
	must(err)
	controller := controllerClass.Alloc().Init()
	defer controller.Release()
	controllerID = controller.ID
	controller.Send(ui.UIViewControllerSel.ViewDidLayoutSubviews)
	require(layouts == 1, "layout 回调应只执行一次")
	_, err = ui.ImplementUIViewController("GouiTestViewController", ui.UIViewControllerOverride{})
	require(err != nil, "同名类重复注册应返回错误")
	return []string{"application-arguments-and-bool", "scene-arguments-and-protocols", "nil-optional-methods", "layout-and-super", "duplicate-class-error"}
}

func (a *application) save(completed bool) {
	home, err := os.UserHomeDir()
	must(err)
	dir := filepath.Join(home, "Documents", "IOSBindings")
	must(os.MkdirAll(dir, 0700))
	data, err := json.MarshalIndent(map[string]any{
		"completed": completed, "phase": a.phase, "painter": painterName,
		"deliveries": a.deliveries, "destroyed": a.destroyed, "frames": a.frames,
		"checks": a.checks,
	}, "", "  ")
	must(err)
	must(os.WriteFile(filepath.Join(dir, "bindings.json"), data, 0600))
	fmt.Printf("IOSBindings completed=%v deliveries=%d destroyed=%d checks=%d\n", completed, a.deliveries, a.destroyed, len(a.checks))
}
