package gui

import (
	"errors"
	"fmt"
	"reflect"
	"runtime"
	"testing"
	"time"

	"github.com/golang-gui/goui/core/geometry"
	"github.com/golang-gui/goui/internal/diagnostic"
	"github.com/golang-gui/goui/platform"
	"github.com/golang-gui/goui/platform/events"
	"github.com/golang-gui/goui/platform/typography"
	"github.com/xuges/gothread"
)

// quitCountLoop is a minimal EventLoop that only records Quit calls; removeWindow
// touches nothing else on the loop.
type quitCountLoop struct {
	platform.EventLoop
	quits int
}

func (l *quitCountLoop) Quit()          { l.quits++ }
func (l *quitCountLoop) Post(fn func()) { fn() }

func TestApplicationQuitsOnLastWindowClosed(t *testing.T) {
	loop := &quitCountLoop{}
	w1, w2 := testWindowWithSurface(&window{}), testWindowWithSurface(&window{})
	a := &application{loop: loop, windows: []*window{w1, w2}, quitOnLastWindowClosed: true}
	q := newTimerTestQueue(t)
	a.timers = q.s
	timer := startTestTimer(t, q, time.Second, true, func() {})

	a.removeWindow(w1)
	if loop.quits != 0 || !timer.Active() {
		t.Fatalf("must not quit while a window remains, quits=%d", loop.quits)
	}
	a.removeWindow(w2)
	if loop.quits != 1 || timer.Active() {
		t.Fatalf("must quit when the last window closes, quits=%d", loop.quits)
	}
}

func TestApplicationDoesNotQuitWhenDisabled(t *testing.T) {
	loop := &quitCountLoop{}
	w := testWindowWithSurface(&window{})
	a := &application{loop: loop, windows: []*window{w}, quitOnLastWindowClosed: false}

	a.removeWindow(w)
	if loop.quits != 0 {
		t.Fatalf("must not quit when QuitOnLastWindowClosed is off, quits=%d", loop.quits)
	}
}

func TestApplicationShutdownStopsTimersBeforeNotification(t *testing.T) {
	for _, nativeExit := range []bool{false, true} {
		for _, withWindow := range []bool{false, true} {
			t.Run(fmt.Sprintf("native=%t/window=%t", nativeExit, withWindow), func(t *testing.T) {
				runtime.LockOSThread()
				defer runtime.UnlockOSThread()
				app, _, makeWindow := applicationWindowsFixture(t, false)
				loop := &activationTestLoop{tasks: make(chan func(), 16)}
				app.loop, app.uiThread = loop, gothread.GetId()
				app.timers = newTimerScheduler(loop.Post, time.Now)
				t.Cleanup(app.timers.finish)
				timer := app.NewTimer()
				if err := timer.Start(time.Hour); err != nil {
					t.Fatal(err)
				}
				var root Widget
				if withWindow {
					root = makeWindow(false).Widget()
				}
				shutdowns, settingsStops := 0, 0
				app.stopSettings = func() { settingsStops++ }
				app.ConnectShutdown(func() {
					shutdowns++
					if timer.Active() || timer.Start(time.Hour) == nil {
						t.Error("Shutdown 通知时定时器仍可运行或重新启动")
					}
					if settingsStops != 1 || gothread.GetId() != app.uiThread {
						t.Error("设置监听未停止或清理不在 UI 线程")
					}
					app.Post(func() { t.Error("退出后仍接收应用任务") })
					app.Quit() // 清理回调中的退出不能重复清理。
					app.shutdown()
				})
				loop.Post(func() {
					if nativeExit {
						loop.Quit()
					} else {
						app.Quit()
					}
				})
				if err := app.Run(); err != nil {
					t.Fatal(err)
				}
				app.Quit()
				app.shutdown()
				if shutdowns != 1 || settingsStops != 1 || len(app.Windows()) != 0 || root != nil && !root.base().destroyed {
					t.Fatal("两条退出路径未完成一次完整清理", shutdowns, settingsStops)
				}
			})
		}
	}
}

// 使用现有后端逐次提供独立窗口替身；退出仍走 GUI 生命周期，不创建原生窗口。
func applicationWindowsFixture(t *testing.T, autoQuit bool) (*application, *quitCountLoop, func(bool) *window) {
	t.Helper()
	loop := new(quitCountLoop)
	backend := &chromeTestPlatform{creationNotifications: true}
	app := &application{platform: backend, loop: loop, servicesReady: true, quitOnLastWindowClosed: autoQuit}
	t.Cleanup(app.shutdown)
	return app, loop, func(noKeepRunning bool) *window {
		t.Helper()
		backend.native = &desktopTestWindow{state: WindowStateHidden}
		public, err := app.NewWindow(&WindowOptions{NoKeepRunning: noKeepRunning})
		if err != nil {
			t.Fatal(err)
		}
		w := public.(*window)
		w.SetWidget(newTestWidget())
		return w
	}
}

func TestApplicationNoKeepRunningWindows(t *testing.T) {
	app, loop, makeWindow := applicationWindowsFixture(t, true)
	first, last, tool := makeWindow(false), makeWindow(false), makeWindow(true)
	if len(app.Windows()) != 3 || len(app.Surfaces()) != 3 || len(app.Snapshot().Surfaces) != 3 {
		t.Fatal("不保活窗口丢失应用归属或语义快照")
	}
	toolRoot, nativeTool := tool.Widget().(*testWidget), tool.platformWindow.(*desktopTestWindow)
	closed := 0
	tool.ConnectDestroy(func() { closed++ })
	// 隐藏和最小化只影响呈现，普通窗口仍保活。
	for _, state := range []WindowState{WindowStateHidden, WindowStateMinimized} {
		last.platformWindow.(*desktopTestWindow).state = state
		if err := last.DispatchEvent(events.StateEvent{State: state}); err != nil {
			t.Fatal(err)
		}
	}
	if err := first.RequestClose(); err != nil || loop.quits != 0 || first.PlatformWindow() != nil {
		t.Fatal("其他普通窗口仍存活时退出，或关闭未完成", err, loop.quits)
	}
	prevent := true
	last.ConnectCloseRequest(func(allow *bool) { *allow = !prevent })
	if err := last.RequestClose(); err != nil || last.destroyed || loop.quits != 0 {
		t.Fatal("关闭否决未保留最后一个普通窗口", err)
	}
	prevent = false
	if err := last.RequestClose(); err != nil {
		t.Fatal(err)
	}
	if loop.quits != 1 || !toolRoot.Destroyed() || !nativeTool.destroyed || closed != 1 || len(app.Windows()) != 0 || len(app.Surfaces()) != 0 {
		t.Fatalf("不保活窗口阻止退出或未清理：quit=%d destroy=%d windows=%d", loop.quits, closed, len(app.Windows()))
	}
	tool.Destroy()
	app.Quit()
	if closed != 1 || loop.quits != 1 {
		t.Fatal("重复销毁或退出产生了重复通知")
	}
}

func TestApplicationNoKeepRunningDoesNotRequestExit(t *testing.T) {
	for _, mode := range []string{"仅不保活窗口", "仍有普通窗口", "关闭自动退出", "系统Surface保活"} {
		t.Run(mode, func(t *testing.T) {
			app, loop, makeWindow := applicationWindowsFixture(t, mode != "关闭自动退出")
			tool := makeWindow(true)
			if mode == "仍有普通窗口" || mode == "关闭自动退出" || mode == "系统Surface保活" {
				ordinary := makeWindow(false)
				if mode == "系统Surface保活" {
					app.systemSurface = newSystemSurface(app)
					app.systemSurface.claimed = true
				}
				if mode != "仍有普通窗口" {
					ordinary.Destroy()
				}
			}
			if err := tool.RequestClose(); err != nil || loop.quits != 0 || app.quitting.Load() {
				t.Fatal("不保活窗口关闭触发了自动退出", err, loop.quits)
			}
			app.Quit()
			if loop.quits != 1 || len(app.Windows()) != 0 || len(app.Surfaces()) != 0 {
				t.Fatal("显式退出没有清理全部宿主")
			}
		})
	}
}

func TestApplicationLastWindowDestroyCanCreateReplacement(t *testing.T) {
	for _, excluded := range []bool{false, true} {
		t.Run(fmt.Sprint("replacementNoKeepRunning=", excluded), func(t *testing.T) {
			_, loop, makeWindow := applicationWindowsFixture(t, true)
			old := makeWindow(false)
			var replacement *window
			old.ConnectDestroy(func() { replacement = makeWindow(excluded) })
			old.Destroy()
			if excluded {
				if loop.quits != 1 || !replacement.destroyed {
					t.Fatal("回调中新建不保活窗口延长了应用运行")
				}
			} else {
				if loop.quits != 0 || replacement.destroyed {
					t.Fatal("未考虑销毁回调中新建的普通窗口")
				}
				replacement.Destroy()
				if loop.quits != 1 {
					t.Fatal("替代普通窗口关闭后未退出")
				}
			}
		})
	}
}

func TestApplicationShutdownDestroysRemainingWindows(t *testing.T) {
	app, loop, makeWindow := applicationWindowsFixture(t, false)
	first, other, tool := makeWindow(false), makeWindow(false), makeWindow(true)
	windows := []*window{first, other, tool}
	roots := []*testWidget{first.Widget().(*testWidget), other.Widget().(*testWidget), tool.Widget().(*testWidget)}
	natives := []*desktopTestWindow{first.platformWindow.(*desktopTestWindow), other.platformWindow.(*desktopTestWindow), tool.platformWindow.(*desktopTestWindow)}
	destroys, shutdowns := 0, 0
	for _, w := range windows {
		w.ConnectCloseRequest(func(allow *bool) { *allow = false })
		w.ConnectDestroy(func() {
			destroys++
			if shutdowns != 1 {
				t.Fatal("窗口兜底清理早于退出通知")
			}
			if _, err := app.NewWindow(nil); !errors.Is(err, platform.ErrUnavailable) {
				t.Fatal("退出期间仍可创建窗口", err)
			}
			app.Quit()
		})
	}
	// 模拟 UI 释放部分宿主，以及窗口销毁回调重入销毁另一个宿主。
	app.ConnectShutdown(func() { shutdowns++; first.Destroy() })
	other.ConnectDestroy(tool.Destroy)
	app.Quit()
	app.shutdown()
	if loop.quits != 1 || shutdowns != 1 || destroys != 3 || len(app.Windows()) != 0 {
		t.Fatalf("退出重入或列表修改导致漏清理：quit=%d shutdown=%d destroy=%d", loop.quits, shutdowns, destroys)
	}
	for i, w := range windows {
		if !roots[i].Destroyed() || !natives[i].destroyed || w.PlatformWindow() != nil {
			t.Fatalf("窗口%d仍保留控件或原生资源", i)
		}
	}
}

func TestApplicationFailedWindowCreationDoesNotRequestExit(t *testing.T) {
	app, loop, _ := applicationWindowsFixture(t, true)
	backend := app.platform.(*chromeTestPlatform)
	backend.native = new(desktopTestWindow)
	backend.painterError = errors.New("绘制器创建失败")
	if _, err := app.NewWindow(nil); !errors.Is(err, backend.painterError) || loop.quits != 0 || app.quitting.Load() || len(app.Windows()) != 0 {
		t.Fatal("未注册窗口的失败清理触发了自动退出", err, loop.quits)
	}
}

func TestApplicationQuitDuringWindowCreation(t *testing.T) {
	app, loop, _ := applicationWindowsFixture(t, true)
	backend := app.platform.(*chromeTestPlatform)
	native := new(desktopTestWindow)
	backend.native, backend.onPainter = native, app.Quit
	if _, err := app.NewWindow(&WindowOptions{NoKeepRunning: true}); !errors.Is(err, platform.ErrUnavailable) || loop.quits != 1 || !native.destroyed || len(app.Windows()) != 0 {
		t.Fatal("创建回调中退出后仍登记窗口或泄漏原生资源", err, loop.quits)
	}
}

func TestApplicationShutdownDuringWindowFrame(t *testing.T) {
	app, loop, makeWindow := applicationWindowsFixture(t, true)
	w := makeWindow(true)
	root := w.Widget().(*testWidget)
	native := w.platformWindow.(*desktopTestWindow)
	backend := new(lifecyclePainter)
	w.painter = backend
	native.state = WindowStateNormal
	if err := w.DispatchEvent(events.StateEvent{State: WindowStateNormal}); err != nil {
		t.Fatal(err)
	}
	w.ConnectAfterPaint(func(Painter, geometry.Rectangle) { app.Quit() })
	w.paint()
	if loop.quits != 1 || !native.destroyed || !root.Destroyed() || len(app.Windows()) != 0 || !reflect.DeepEqual(backend.calls, []string{"begin", "end", "destroy"}) {
		t.Fatal("帧内退出未推迟资源销毁，或退出后仍有资源", loop.quits, backend.calls)
	}
}

// activationTestLoop 在独立的固定线程执行任务，模拟移动 Run 只等待的契约。
// 这里只验证 Go 生命周期，不依赖桌面或原生窗口。
type activationTestLoop struct {
	platform.EventLoop
	tasks   chan func()
	native  bool
	stopped bool
	skip    bool
	thread  int
}

func (l *activationTestLoop) Post(fn func()) { l.tasks <- fn }
func (l *activationTestLoop) Quit()          { l.stopped = true }
func (*activationTestLoop) Destroy()         {}
func (l *activationTestLoop) Run() {
	if l.skip {
		return
	}
	run := func() {
		runtime.LockOSThread()
		defer runtime.UnlockOSThread()
		l.thread = gothread.GetId()
		for !l.stopped {
			(<-l.tasks)()
		}
	}
	if !l.native {
		run()
		return
	}
	done := make(chan struct{})
	go func() { defer close(done); run() }()
	<-done
}

type activationTestPlatform struct {
	platform.Platform
	loop      *activationTestLoop
	failure   error
	calls     []string
	offThread bool
}

func (p *activationTestPlatform) record(name string) {
	p.calls = append(p.calls, name)
	if p.loop.native && gothread.GetId() != p.loop.thread {
		p.offThread = true
	}
}
func (p *activationTestPlatform) NewEventLoop() (platform.EventLoop, error) { return p.loop, nil }
func (*activationTestPlatform) Destroy()                                    {}

type activationTestTypography struct{ typography.Context }

func (p *activationTestPlatform) NewTypography() (typography.Context, error) {
	p.record("typography")
	if p.failure != nil {
		return nil, p.failure
	}
	return &activationTestTypography{}, nil
}
func (p *activationTestPlatform) NewSettings() (platform.Settings, error) {
	p.record("settings")
	return nil, platform.ErrUnsupported
}
func (p *activationTestPlatform) NewClipboard() (platform.Clipboard, error) {
	p.record("clipboard")
	return nil, platform.ErrUnsupported
}
func (p *activationTestPlatform) NewFileDialog() (platform.FileDialog, error) {
	p.record("dialog")
	return nil, platform.ErrUnsupported
}

type activationTestSource struct{ *activationTestPlatform }

func (*activationTestSource) SetWindowHandler(func(platform.Window) platform.EventHandler) error {
	return nil
}

func TestApplicationActivationThreadAndOrdering(t *testing.T) {
	for _, native := range []bool{false, true} {
		t.Run(fmt.Sprintf("system-host=%t", native), func(t *testing.T) {
			runtime.LockOSThread()
			defer runtime.UnlockOSThread()
			startup := gothread.GetId()
			loop := &activationTestLoop{tasks: make(chan func(), 16), native: native}
			backend := &activationTestPlatform{loop: loop}
			var platformBackend platform.Platform = backend
			if native {
				platformBackend = &activationTestSource{backend}
			}
			a, err := prepareApplication(platformBackend)
			if err != nil {
				t.Fatal(err)
			}
			if native && (a.servicesReady || len(backend.calls) != 0) {
				t.Fatal("准备阶段访问了移动原生服务")
			}
			if !native && !a.servicesReady {
				t.Fatal("桌面迁移兼容路径未初始化")
			}
			if native {
				if _, err := a.NewWindow(nil); !errors.Is(err, ErrAppNotReady) {
					t.Fatalf("激活前窗口创建错误=%v", err)
				}
			}
			var order []string
			a.ConnectActivate(func() {
				if !a.servicesReady || a.Typography() == nil || a.Settings() == nil || a.FileDialog() == nil || a.Clipboard() == nil {
					t.Error("激活早于服务就绪")
				}
				if native && gothread.GetId() == startup {
					t.Error("移动激活仍在启动线程")
				}
				order = append(order, "activate")
			})
			a.ConnectShutdown(func() {
				if gothread.GetId() != loop.thread {
					t.Error("退出回调不在 UI 线程")
				}
				order = append(order, "shutdown")
			})
			a.Post(func() { order = append(order, "posted"); a.Quit(); a.Quit() })
			if len(order) != 0 {
				t.Fatal("Run 前同步触发激活或 Post")
			}
			if err := a.Run(); err != nil {
				t.Fatal(err)
			}
			if backend.offThread || len(backend.calls) != 4 || !reflect.DeepEqual(order, []string{"activate", "posted", "shutdown"}) {
				t.Fatalf("线程/顺序错误：native=%v calls=%v order=%v", native, backend.calls, order)
			}
			if err := a.Run(); !errors.Is(err, ErrAppRunOnce) {
				t.Fatalf("重复 Run 错误=%v", err)
			}
		})
	}
}

func TestApplicationActivationFailureAndQuit(t *testing.T) {
	for _, mode := range []string{"initialize failure", "quit before run", "quit during activate"} {
		t.Run(mode, func(t *testing.T) {
			runtime.LockOSThread()
			defer runtime.UnlockOSThread()
			loop := &activationTestLoop{tasks: make(chan func(), 16), native: true}
			backend := &activationTestPlatform{loop: loop}
			failure := errors.New("排版创建失败")
			if mode == "initialize failure" {
				backend.failure = failure
			}
			a, err := prepareApplication(&activationTestSource{backend})
			if err != nil {
				t.Fatal(err)
			}
			activated, shutdown := 0, 0
			a.ConnectActivate(func() { activated++; a.Quit() })
			a.ConnectActivate(func() { t.Error("Quit 后继续激活其他监听者") })
			a.ConnectShutdown(func() { shutdown++ })
			if mode == "quit before run" {
				a.Quit()
			}
			err = a.Run()
			if mode == "initialize failure" && !errors.Is(err, failure) || mode != "initialize failure" && err != nil {
				t.Fatalf("启动错误未返回：%v", err)
			}
			want := 0
			if mode == "quit during activate" {
				want = 1
			}
			if activated != want || shutdown != 1 || backend.offThread {
				t.Fatalf("激活/退出错误：%d/%d, offThread=%v", activated, shutdown, backend.offThread)
			}
		})
	}
}

func TestApplicationRejectsMissingNativeShutdown(t *testing.T) {
	loop := &activationTestLoop{tasks: make(chan func(), 8), native: true, skip: true}
	backend := &activationTestPlatform{loop: loop}
	app, err := prepareApplication(&activationTestSource{backend})
	if err != nil {
		t.Fatal(err)
	}
	app.ConnectShutdown(func() { t.Error("启动线程伪装原生 UI 退出回调") })
	if err = app.Run(); err == nil || !app.quitting.Load() || len(backend.calls) != 0 {
		t.Fatalf("缺失原生退出未报告或访问了服务：err=%v calls=%v", err, backend.calls)
	}
}

// testWindowWithSurface 为独立测试状态绑定真实的公共 Root 身份。
func testWindowWithSurface(w *window) *window { w.host = w; w.claimed = true; return w }

func TestApplicationStopsDiagnosticsOnUIThread(t *testing.T) {
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()
	loop := &activationTestLoop{tasks: make(chan func(), 16), native: true}
	backend := &activationTestPlatform{loop: loop}
	app, err := prepareApplication(&activationTestSource{backend})
	if err != nil {
		t.Fatal(err)
	}
	session, err := diagnostic.StartCapture(app, diagnostic.Options{})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(session.Stop)
	stopped, wrongThread := 0, false
	session.ConnectStop(func() { stopped++; wrongThread = gothread.GetId() != loop.thread })
	app.ConnectActivate(app.Quit)
	if err := app.Run(); err != nil {
		t.Fatal(err)
	}
	if stopped != 1 || wrongThread {
		t.Fatalf("诊断资源不在 UI 退出阶段释放：count=%d wrongThread=%v", stopped, wrongThread)
	}
}

func TestNewApplicationReturnsExistingSingleton(t *testing.T) {
	oldApp := App
	t.Cleanup(func() { App = oldApp })
	existing := &application{}
	App = existing
	for _, id := range []string{"", "com.example.Editor", "com.example.Other", "invalid"} {
		got, err := NewApplication(id)
		if err != nil || got != existing || App != existing {
			t.Fatalf("existing application with argument %q: %v, %v", id, got, err)
		}
	}
}

func TestInvalidApplicationIdentityBeforePlatformCreation(t *testing.T) {
	oldApp := App
	App = nil
	t.Cleanup(func() { App = oldApp })
	if got, err := NewApplication("bad id"); got != nil || err == nil {
		t.Fatalf("invalid identity: %v, %v", got, err)
	}
}
