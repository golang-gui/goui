package ui

import (
	"errors"
	"fmt"
	"runtime"
	"testing"

	"github.com/golang-gui/goui/core/geometry"
	"github.com/golang-gui/goui/gui"
	"github.com/golang-gui/goui/platform"
)

// 仅暴露 Surface 方法，避免测试替身偷偷提供 Window 管理能力。
type surfaceTestHost struct {
	gui.Surface
	state *testWindow
}

func newSurfaceTestHost() *surfaceTestHost {
	w := newTestWindow()
	return &surfaceTestHost{Surface: w, state: w}
}
func (s *surfaceTestHost) Destroy() {
	if s.state.destroyed {
		return
	}
	s.state.Destroy()
	s.state.widget = nil
}

type surfaceTestApplication struct {
	*windowTestApplication
	hosts   []*surfaceTestHost
	failure error
}

func (a *surfaceTestApplication) NewSurface() (gui.Surface, error) {
	if a.failure != nil {
		return nil, a.failure
	}
	for _, host := range a.hosts {
		if !host.state.destroyed {
			return nil, errors.New("测试宿主被重复领取")
		}
	}
	host := newSurfaceTestHost()
	a.hosts = append(a.hosts, host)
	return host, nil
}
func surfaceApp(t *testing.T, build func() RootView) (*app, *surfaceTestApplication) {
	t.Helper()
	runtime.LockOSThread()
	t.Cleanup(runtime.UnlockOSThread)
	backend := &surfaceTestApplication{windowTestApplication: newWindowTestApplication()}
	a := newApp(backend, build)
	t.Cleanup(func() { a.stop(); a.destroyAll() })
	if err := a.rebuild(); err != nil {
		t.Fatal(err)
	}
	return a, backend
}

func TestSurfaceDeclarationRetainsHostAndLocalState(t *testing.T) {
	count := MakeState(0)
	text := &bindingSource{value: "first"}
	global, local := 0, 0
	a, fake := surfaceApp(t, func() RootView {
		global++
		return Root().Surface(Surface("main").Content(VBox(
			Label().BindText(text).ID("bound"),
			TextInput().ID("editor"),
			Build(func() View { local++; return Label(fmt.Sprint(count.Get())).ID("count") }),
		)))
	})
	host := a.FindSurface("main")
	if _, managed := host.(gui.Window); managed || a.FindWindow("main") != nil {
		t.Fatal("系统宿主被暴露为管理窗口")
	}
	input := a.FindWidget("main", "editor").(*gui.TextInput)
	input.SetText("typed")
	initial := host.Widget()
	text.Set("second")
	count.Set(1)
	fake.runPosted()
	if global != 1 || local != 2 || a.FindWidget("main", "bound").(*gui.Label).Text() != "second" || a.FindWidget("main", "count").(*gui.Label).Text() != "1" {
		t.Fatalf("未复用局部更新：global=%d local=%d", global, local)
	}
	a.RequestUpdate()
	fake.runPosted()
	if global != 2 || len(fake.hosts) != 1 || a.FindSurface("main") != host || host.Widget() != initial || a.FindWidget("main", "editor") != input || input.Text() != "typed" {
		t.Fatal("全量声明更新重新领取宿主或丢失输入状态")
	}
	host.Destroy()
	posts := len(fake.posts)
	text.Set("late")
	count.Set(2)
	fake.runPosted()
	if a.FindSurface("main") != nil || text.disconnects != 1 || posts != 0 || global != 2 {
		t.Fatal("销毁后订阅/引用未释放")
	}
}

func TestSurfaceIdentityReplacementAndRemoval(t *testing.T) {
	id := "a"
	destroyed := 0
	a, fake := surfaceApp(t, func() RootView {
		if id == "" {
			return Root()
		}
		return Surface(id).Content(Label(id)).OnDestroy(func() { destroyed++ })
	})
	first := a.FindSurface("a")
	id = "b"
	if err := a.rebuild(); err != nil {
		t.Fatal(err)
	}
	if len(fake.hosts) != 2 || !fake.hosts[0].state.destroyed || destroyed != 1 || a.FindSurface("a") != nil || a.FindSurface("b") == first {
		t.Fatal("换 ID 未先释放旧宿主再领取新宿主")
	}
	id = ""
	if err := a.rebuild(); err != nil {
		t.Fatal(err)
	}
	if destroyed != 2 || a.surface != nil || a.FindSurface("b") != nil || fake.quits != 0 {
		t.Fatal("移除被当作应用退出或保留了旧引用")
	}
}

func TestSurfaceRejectsInvalidOrUnsupportedDeclarations(t *testing.T) {
	for _, tc := range []struct {
		name          string
		view          RootView
		failure, want error
	}{
		{name: "empty", view: Surface(""), want: ErrSurfaceIDEmpty},
		{name: "shared ID", view: Root().Surface(Surface("same")).Windows(Window("same"))},
		{name: "unsupported", view: Surface("main"), failure: platform.ErrUnsupported, want: platform.ErrUnsupported},
	} {
		t.Run(tc.name, func(t *testing.T) {
			runtime.LockOSThread()
			defer runtime.UnlockOSThread()
			fake := &surfaceTestApplication{windowTestApplication: newWindowTestApplication(), failure: tc.failure}
			a := newApp(fake, func() RootView { return tc.view })
			defer a.stop()
			defer a.destroyAll()
			err := a.rebuild()
			if err == nil || tc.want != nil && !errors.Is(err, tc.want) {
				t.Fatalf("声明错误没有返回：%v", err)
			}
			if len(fake.hosts) != 0 || len(fake.windows) != 0 {
				t.Fatal("无效声明创建了原生对象或回退到窗口")
			}
		})
	}
}

func TestSurfaceSharesCommandsCallbacksAndBatchCompletion(t *testing.T) {
	calls, painted, completed := 0, 0, 0
	var a *app
	probe := &afterUpdateProbe{callback: func(gui.Widget) {}}
	probe.Self = probe
	a, fake := surfaceApp(t, func() RootView {
		return Root().Surface(Surface("main").Content(probe).
			Commands(Command("run").OnActivate(func() { calls++ })).
			Shortcuts(Shortcut(KeyF5).Command("run")).
			OnAfterPaint(func(gui.Painter, geometry.Rectangle) { painted++ })).
			Windows(Window("other").Content(Label("other")))
	})
	// 首轮构建完成后再安装查询回调；它验证下一批所有根均已协调。
	probe.callback = func(gui.Widget) {
		completed++
		if a.reconcilingHosts || a.FindSurface("main") == nil || a.FindWindow("other") == nil {
			t.Fatal("完成通知早于其他根挂载")
		}
	}
	if err := a.rebuild(); err != nil {
		t.Fatal(err)
	}
	command := a.FindCommand("main", "run")
	if command == nil || len(command.Shortcuts()) != 1 {
		t.Fatal("Surface 未建立默认命令组")
	}
	command.Activate()
	fake.hosts[0].state.afterPaint.Emit(nil, geometry.Rectangle{})
	if calls != 1 || painted != 1 || completed != 1 || fake.hosts[0].state.shows != 0 {
		t.Fatal("重复连接、未完成批次或主动显示系统宿主")
	}
	if a.FindSurface("other") != a.FindWindow("other") {
		t.Fatal("通用宿主查询遗漏管理窗口")
	}
	native := &dialogTestService{}
	dialog := FileDialog{current: a, dlg: native}
	if err := dialog.OpenFile("main", DialogOptions{}, func(_ []FileAccess, err error) {
		if err != nil {
			t.Fatal(err)
		}
	}); err != nil {
		t.Fatal(err)
	}
	if native.owner != a.FindSurface("main") {
		t.Fatal("文件对话框未使用公共宿主 owner")
	}
	a.release()
	if command.Enabled() || a.FindCommand("main", "run") != nil {
		t.Fatal("退出后命令仍可执行或可查询")
	}
	a.destroyAll()
}

func TestSurfaceDestroyDuringCompletion(t *testing.T) {
	var a *app
	probe := &afterUpdateProbe{callback: func(gui.Widget) {}}
	probe.Self = probe
	a, fake := surfaceApp(t, func() RootView { return Surface("main").Content(probe) })
	destroyed := fake.hosts[0]
	probe.callback = func(gui.Widget) { destroyed.Destroy() }
	if err := a.rebuild(); err != nil {
		t.Fatal(err)
	}
	if a.FindSurface("main") != nil || a.surface != nil || !destroyed.state.destroyed {
		t.Fatal("完成回调销毁宿主后又复活")
	}
}

func TestSurfaceRunReturnsCreationFailure(t *testing.T) {
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()
	failure := errors.New("测试系统宿主领取失败")
	backend := &surfaceTestApplication{windowTestApplication: newWindowTestApplication(), failure: failure}
	a := newApp(backend, func() RootView { return Surface("main").Content(Label("unreachable")) })
	if err := a.run(); !errors.Is(err, failure) {
		t.Fatalf("Run 未返回创建失败：%v", err)
	}
	if backend.quits != 1 || a.FindSurface("main") != nil || len(backend.windows) != 0 || len(backend.hosts) != 0 {
		t.Fatal("失败后仍有宿主或静默创建了窗口")
	}
}

func TestSurfaceCoordinatorFindsCommonRoots(t *testing.T) {
	a, _ := surfaceApp(t, func() RootView {
		return Root().Surface(Surface("main").Content(VBox(Label("source").ID("moving")))).
			Windows(Window("other").Content(VBox(Label("target"))))
	})
	moving := a.FindWidget("main", "moving")
	fromWindow := &Coordinator{owner: windowContentNode(a.windows["other"].root)}
	found := fromWindow.find(moving)
	if found == nil || found.widget != moving || found.root != a.surface.root {
		t.Fatal("从管理窗口协调时遗漏系统 Surface 子树")
	}
	target := a.FindWindow("other").Widget()
	fromSurface := &Coordinator{owner: windowContentNode(a.surface.root)}
	if found := fromSurface.find(target); found == nil || found.root != a.windows["other"].root {
		t.Fatal("从系统 Surface 协调时遗漏管理窗口子树")
	}
}
