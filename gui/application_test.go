package gui

import (
	"errors"
	"fmt"
	"reflect"
	"runtime"
	"testing"
	"time"

	"github.com/golang-gui/goui/internal/diagnostic"
	"github.com/golang-gui/goui/platform"
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
	w1, w2 := &window{}, &window{}
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
	w := &window{}
	a := &application{loop: loop, windows: []*window{w}, quitOnLastWindowClosed: false}

	a.removeWindow(w)
	if loop.quits != 0 {
		t.Fatalf("must not quit when QuitOnLastWindowClosed is off, quits=%d", loop.quits)
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
