//go:build android || ios

// UI Surface 验收复用本目录的移动包与报告。以 -X main.viewMode=ui 构建，
// 对比普通 GUI 路径，另检查局部 State 更新与声明生命周期；详见 README.md。
package main

import (
	"fmt"
	"path/filepath"
	"runtime"
	"time"

	"github.com/golang-gui/goui/core/geometry"
	"github.com/golang-gui/goui/gui"
	"github.com/golang-gui/goui/gui/diagnostics"
	"github.com/golang-gui/goui/ui"
)

type uiApplication struct {
	application
	handle                                ui.App
	text                                  ui.State[string]
	value                                 ui.State[int]
	globalBuilds, localBuilds, uiDestroys int
}

func mainUI() {
	if mainThread() {
		panic("启动 goroutine 不应阻塞原生 UI 线程")
	}
	must(setPainter())
	a := &uiApplication{application: application{nonce: time.Now().UnixNano(), startup: threadID(), dir: reportDirectory()}, text: ui.MakeState("initial"), value: ui.MakeState(0)}
	must(prepareReports(a.dir))
	must(ui.Run("org.golanggui.MobileActivation", func(handle ui.App) ui.RootView {
		a.globalBuilds++
		return a.view(handle)
	}))
	if !a.exited.Load() || a.unmounts != 1 || a.destroys != 1 || a.uiDestroys != 1 || !a.input.Destroyed() {
		panic("UI 退出早于清理或未释放 Widget 树")
	}
	write(filepath.Join(a.dir, "completed.json"), map[string]any{"nonce": a.nonce, "callbackReturned": true, "frames": a.frames, "ui": true, "globalBuilds": a.globalBuilds, "localBuilds": a.localBuilds, "uiDestroys": a.uiDestroys})
}

// 视图仅描述内容。原生检查与计时器在首次 AfterPaint 回调中初始化。
func (a *uiApplication) view(handle ui.App) ui.SurfaceView {
	input := &trackedInput{TextInputView: ui.TextInput().BindText(&a.text).ID("editor").MinSize(240, 36), owner: a}
	return ui.Surface("main").Content(ui.VBox(
		ui.Label("GOUI UI Surface / main thread"),
		ui.Label("中文 日本語 한국어 العربية"),
		input,
		ui.Button("Increment").ID("action").MinSize(180, 40).OnClick(func() { a.checkThread(); a.count++; a.value.Set(a.count) }),
		ui.Build(func() ui.View { a.localBuilds++; return ui.Label(fmt.Sprintf("Count: %d", a.value.Get())).ID("count") }),
	).Spacing(12).Paddings(geometry.Insets{Left: 20, Top: 64, Right: 20, Bottom: 20})).
		OnAfterPaint(func(p gui.Painter, _ geometry.Rectangle) {
			a.checkThread()
			if a.handle == nil {
				a.startUI(handle)
			}
			a.frames++
			a.scale = p.PixelScale()
		}).OnDestroy(func() { a.checkThread(); a.uiDestroys++ })
}

// 挂载真实 GUI 控件前连接生命周期计数；不导出生产内核私有状态。
type trackedInput struct {
	*ui.TextInputView
	owner *uiApplication
}

func (v *trackedInput) Mount(ctx ui.BuildContext) gui.Widget {
	widget := v.TextInputView.Mount(ctx)
	widget.ConnectMount(func() { v.owner.mounts++ })
	widget.ConnectUnmount(func() { v.owner.unmounts++ })
	return widget
}

func (a *uiApplication) startUI(handle ui.App) {
	a.handle = handle
	a.app = gui.App
	a.host = handle.FindSurface("main")
	if a.host == nil || handle.FindWindow("main") != nil {
		panic("声明未领取公共 Surface")
	}
	a.activations++
	a.deliveries++
	a.input = handle.FindWidget("main", "editor").(*gui.TextInput)
	a.button = handle.FindWidget("main", "action").(*gui.Button)
	a.label = handle.FindWidget("main", "count").(*gui.Label)
	if a.input.Root() != a.host || a.input.Surface() != a.host || a.input.Window() != nil {
		panic("UI Widget 宿主身份错误")
	}
	a.host.ConnectDestroy(func() { a.checkThread(); a.destroys++ })
	var err error
	a.capture, err = diagnostics.Start(a.app, diagnostics.Options{})
	must(err)
	// UI 首先释放协调连接，随后本回调请求 GUI 宿主销毁并检查完整释放。
	a.app.ConnectShutdown(a.stop)
	a.timer = a.app.NewTimer()
	a.timer.ConnectTimeout(func() {
		wasExercised := a.exercised
		a.tick()
		if a.stopped {
			return
		}
		if wasExercised {
			if a.globalBuilds != 1 || a.localBuilds != 2 || a.text.Get() != "initial committed" || a.value.Get() != 1 || a.label.Text() != "Count: 1" {
				panic(fmt.Sprintf("局部状态未正确更新：global=%d local=%d text=%q value=%d label=%q", a.globalBuilds, a.localBuilds, a.text.Get(), a.value.Get(), a.label.Text()))
			}
		}
	})
	must(a.timer.Start(100 * time.Millisecond))
	runtime.GC()
}
