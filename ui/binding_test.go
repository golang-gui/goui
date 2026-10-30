package ui

import (
	"fmt"
	"runtime"
	"sync"
	"testing"

	"github.com/golang-gui/goui/core/geometry"
	"github.com/golang-gui/goui/core/signal"
	"github.com/golang-gui/goui/gui"
	"github.com/golang-gui/goui/layout"
)

// 来源计数只在 UI 测试线程读取；并发通知测试使用本身同步的 State。
type bindingSource struct {
	value                        string
	changed                      signal.Signal0
	connects, disconnects, reads int
	last                         func()
}

func (s *bindingSource) Get() string { s.reads++; return s.value }
func (s *bindingSource) Connect(fn func()) signal.Handle {
	s.connects++
	s.last = fn
	return &bindingConnection{Handle: s.changed.Connect(fn), source: s}
}
func (s *bindingSource) Set(v string) { s.value = v; s.changed.Emit() }

type bindingConnection struct {
	signal.Handle
	source *bindingSource
}

func (h *bindingConnection) Disconnect() {
	h.Handle.Disconnect()
	if h.source != nil {
		h.source.disconnects++
		h.source = nil
	}
}

func bindingApp(t *testing.T, build func() RootView) (*app, *windowTestApplication) {
	t.Helper()
	// 自动读取按 OS 线程隔离；锁定真正执行构建与队列的测试 goroutine。
	runtime.LockOSThread()
	t.Cleanup(runtime.UnlockOSThread)
	fake := newWindowTestApplication()
	rt := newApp(fake, build)
	t.Cleanup(func() { rt.stop(); rt.destroyAll() })
	if err := rt.rebuild(); err != nil {
		t.Fatal(err)
	}
	return rt, fake
}

func TestBuildTracksStateReadsAndBranches(t *testing.T) {
	show, chooseLeft := MakeState(true), MakeState(true)
	left, right := MakeState("left"), MakeState("right")
	builds := 0
	rt, fake := bindingApp(t, func() RootView {
		return Window("main").Content(Build(func() View {
			builds++
			if !show.Get() {
				return nil
			}
			if chooseLeft.Get() {
				return Label(left.Get())
			}
			return Label(right.Get())
		}))
	})
	n := rt.windows["main"].root.root
	left.Set("new left")
	left.Set("latest left")
	if len(fake.posts) != 1 || builds != 1 {
		t.Fatal("自动依赖没有延迟合并通知")
	}
	fake.runPosted()
	if builds != 2 || fake.windows[0].Widget().(*gui.Label).Text() != "latest left" {
		t.Fatal("自动依赖没有更新当前分支")
	}
	chooseLeft.Set(false)
	fake.runPosted()
	left.Set("unused")
	if len(fake.posts) != 0 || len(n.tracked) != 3 {
		t.Fatal("分支切换没有解除旧依赖")
	}
	right.Set("new right")
	fake.runPosted()
	if builds != 4 || fake.windows[0].Widget().(*gui.Label).Text() != "new right" {
		t.Fatal("没有收集新分支的来源")
	}
	show.Set(false)
	fake.runPosted()
	chooseLeft.Set(true)
	left.Set("restored")
	right.Set("hidden")
	if len(fake.posts) != 0 || fake.windows[0].Widget() != nil || len(n.tracked) != 1 {
		t.Fatal("空输出仍订阅不可见分支")
	}
	show.Set(true)
	fake.runPosted()
	if builds != 6 || rt.windows["main"].root.root != n || fake.windows[0].Widget().(*gui.Label).Text() != "restored" {
		t.Fatal("空组合没有保留身份或恢复订阅")
	}
}

func TestAutomaticReadsStayWithTheirMountedComposition(t *testing.T) {
	parentValue, childValue := MakeState("parent"), MakeState("child")
	parents, children := 0, 0
	shared := Build(func() View { children++; return Label(childValue.Get()) })
	rt, fake := bindingApp(t, func() RootView {
		return Root().Windows(
			Window("parent").Content(Build(func() View {
				parents++
				return VBox(Label(parentValue.Get()), shared)
			})),
			Window("other").Content(shared),
		)
	})
	childValue.Set("both")
	fake.runPosted()
	if parents != 1 || children != 4 || fake.windows[0].Widget().Children()[1].(*gui.Label).Text() != "both" || fake.windows[1].Widget().(*gui.Label).Text() != "both" {
		t.Fatal("子构建依赖泄漏给父节点，或共享声明合并了两个挂载")
	}
	parentValue.Set("new parent")
	fake.runPosted()
	if parents != 2 || children != 5 {
		t.Fatal("父子更新范围错误")
	}
	rt.windows["parent"].window.Destroy()
	childValue.Set("survivor")
	fake.runPosted()
	if children != 6 || fake.windows[1].Widget().(*gui.Label).Text() != "survivor" || rt.windows["parent"] != nil {
		t.Fatal("一个挂载释放影响了另一个挂载的自动订阅")
	}
}

func TestBuildReadAndObserveShareConnections(t *testing.T) {
	source := new(getterNotificationSource)
	manual, builds := true, 0
	rt, fake := bindingApp(t, func() RootView {
		v := Build(func() View {
			builds++
			return Label(Read[string](source) + Read[string](source))
		})
		if manual {
			v.Observe(source, source)
		} else {
			v.Observe()
		}
		return Window("main").Content(v)
	})
	if source.connects != 1 || len(fake.posts) != 1 {
		t.Fatal("Read 未在首次 Get 前连接，或与 Observe 重复连接")
	}
	fake.runPosted()
	if builds != 2 || source.reads != 4 {
		t.Fatal("首次读取中的通知丢失")
	}
	manual = false
	rt.RequestUpdate()
	fake.runPosted()
	source.Set("automatic")
	fake.runPosted()
	if builds != 4 || source.connects != 1 || source.disconnects != 0 {
		t.Fatal("Observe() 错误清除了自动依赖，或重建重复连接")
	}
	rt.destroyAll()
	source.last()
	if source.disconnects != 1 || len(fake.posts) != 0 {
		t.Fatal("自动来源释放或迟到通知不正确")
	}
}

func TestAutomaticReadSharesPropertyBindingConnection(t *testing.T) {
	source := &bindingSource{value: "before"}
	builds, applies := 0, 0
	rt, fake := bindingApp(t, func() RootView {
		return Window("main").Content(trackingView(func(ctx BuildContext) View {
			builds++
			BindValue(ctx, "text", source, func(string) { applies++ })
			return Label(Read[string](source))
		}).Observe(source))
	})
	source.Set("after")
	fake.runPosted()
	n := rt.windows["main"].root.root
	if builds != 2 || applies != 2 || source.connects != 1 || len(n.subscriptions) != 1 || updateKind(n.subscriptions[source].kind.Load()) != updateBuild|updateBindings {
		t.Fatal("自动读取、Observe 和属性绑定重复订阅或重复应用")
	}
	for range 3 {
		rt.RequestUpdate()
		fake.runPosted()
	}
	if source.connects != 1 || source.disconnects != 0 {
		t.Fatal("完整重建没有复用同一节点的来源连接")
	}
}

func TestBuildPeekAndCapturedValuesStaySnapshots(t *testing.T) {
	value, debug, captured := MakeState("value"), MakeState("debug"), MakeState("captured")
	snapshot := captured.Get()
	builds, latestDebug := 0, ""
	_, fake := bindingApp(t, func() RootView {
		return Window("main").Content(Build(func() View {
			builds++
			latestDebug = debug.Peek()
			return Label(snapshot + value.Get())
		}))
	})
	debug.Set("new debug")
	captured.Set("new captured")
	if len(fake.posts) != 0 {
		t.Fatal("Peek 或构建前捕获值建立了自动依赖")
	}
	value.Set("new value")
	fake.runPosted()
	if builds != 2 || latestDebug != "new debug" || fake.windows[0].Widget().(*gui.Label).Text() != "capturednew value" {
		t.Fatal("Peek 未读到当前值，或改变了 Go 闭包快照语义")
	}
}

type readTrackingView struct {
	ComposeBase[readTrackingView]
	build func(BuildContext) View
}

func trackingView(build func(BuildContext) View) *readTrackingView {
	v := &readTrackingView{build: build}
	v.Self = v
	return v
}
func (v *readTrackingView) Build(ctx BuildContext) View { return v.build(ctx) }

func TestBuildCallbacksAndBindingsDoNotAddDependencies(t *testing.T) {
	bound, noise, notified := MakeState("bound"), MakeState("noise"), MakeState(0)
	builds, applies := 0, 0
	handle := notified.Connect(func() { noise.Get() })
	defer handle.Disconnect()
	_, fake := bindingApp(t, func() RootView {
		return Window("main").Content(trackingView(func(ctx BuildContext) View {
			builds++
			BindValue(ctx, "value", &bound, func(string) { applies++; noise.Get() })
			notified.Set(1)
			notified.Update(func(value int) int { noise.Get(); return value + 1 })
			return Label("composition")
		}))
	})
	noise.Set("ignored")
	if len(fake.posts) != 0 {
		t.Fatal("通知／Update 或绑定 apply 的读取污染了构建依赖")
	}
	bound.Set("new bound")
	fake.runPosted()
	if builds != 1 || applies != 2 {
		t.Fatal("绑定 getter 的 Get 同时请求了完整构建")
	}
}

func TestBuildBackgroundReadsAndWritesAreIsolated(t *testing.T) {
	value, noise := MakeState("before"), MakeState("worker")
	builds := 0
	_, fake := bindingApp(t, func() RootView {
		return Window("main").Content(Build(func() View {
			builds++
			text := value.Get()
			if builds == 1 {
				done := make(chan struct{})
				go func() {
					defer close(done)
					for range 100 {
						noise.Get()
						noise.Set("ignored")
					}
					value.Set("during build")
				}()
				<-done
				// 后台 Set 不得清除当前 UI 收集器，之后的读取仍应登记。
				value.Get()
			}
			return Label(text)
		}))
	})
	if len(fake.posts) != 1 {
		t.Fatal("构建中的后台通知丢失")
	}
	fake.runPosted()
	noise.Set("after")
	if builds != 2 || len(fake.posts) != 0 || fake.windows[0].Widget().(*gui.Label).Text() != "during build" {
		t.Fatal("后台读取污染依赖，或后台写入没有在下一批应用")
	}
}

func TestBuildPanicRestoresCaptureAndPreviousDependencies(t *testing.T) {
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()
	before, failed := &bindingSource{value: "before"}, &bindingSource{value: "failed"}
	r := newRoot()
	defer r.unmountWindow()
	v := Build(func() View { return Label(Read[string](before)) })
	r.update(v)
	n := r.root
	v.build = func() View { Read[string](failed); panic("expected") }
	func() {
		defer func() {
			if recover() != "expected" {
				t.Fatal("未保留原始构建 panic")
			}
		}()
		r.update(v)
	}()
	if activeReads.Load() != nil || len(n.tracked) != 1 || failed.disconnects != 1 || before.disconnects != 0 {
		t.Fatal("异常构建提交了半个依赖集，或未恢复收集器")
	}
	if _, ok := n.tracked[before]; !ok {
		t.Fatal("异常构建丢失上轮依赖")
	}
	v.build = func() View { return Label(Read[string](failed)) }
	r.update(v)
	if before.disconnects != 1 || failed.connects != 2 || activeReads.Load() != nil {
		t.Fatal("异常后的正常构建没有替换依赖")
	}
}

type trackingSource struct {
	bindingSource
	read                    func() string
	onConnect, onDisconnect func()
}

func (s *trackingSource) Get() string {
	if s.read != nil {
		return s.read()
	}
	return s.bindingSource.Get()
}
func (s *trackingSource) Connect(fn func()) signal.Handle {
	h := &trackingConnection{Handle: s.bindingSource.Connect(fn), disconnected: s.onDisconnect}
	if s.onConnect != nil {
		s.onConnect()
	}
	return h
}

type trackingConnection struct {
	signal.Handle
	disconnected func()
}

func (h *trackingConnection) Disconnect() {
	fn := h.disconnected
	h.disconnected = nil
	h.Handle.Disconnect()
	if fn != nil {
		fn()
	}
}

func TestReadKeepsCustomSourceAndSubscriptionCallbacksIsolated(t *testing.T) {
	show, inner, noise := MakeState(true), MakeState("inner"), MakeState("noise")
	source := &trackingSource{
		read:      func() string { return inner.Get() },
		onConnect: func() { noise.Get() }, onDisconnect: func() { noise.Get() },
	}
	builds := 0
	_, fake := bindingApp(t, func() RootView {
		return Window("main").Content(Build(func() View {
			builds++
			if !show.Get() {
				return nil
			}
			return Label(Read[string](source))
		}))
	})
	// 故意延迟外层通知，验证 Read 只依赖来源自身，不订阅 getter 内部状态。
	inner.Set("changed")
	noise.Set("ignored")
	if len(fake.posts) != 0 {
		t.Fatal("Read getter 或 Connect 回调污染了自动依赖")
	}
	source.Set("publish")
	fake.runPosted()
	if builds != 2 || fake.windows[0].Widget().(*gui.Label).Text() != "changed" {
		t.Fatal("自定义来源通知没有重新读取当前值")
	}
	show.Set(false)
	fake.runPosted()
	noise.Set("disconnected")
	if len(fake.posts) != 0 || source.connects != 1 || source.disconnects != 1 {
		t.Fatal("Disconnect 回调添加了新的依赖，或连接没有复用")
	}
}

type trackingWidget struct {
	ViewBase[trackingWidget]
	noise *State[string]
}

func (v *trackingWidget) Mount(BuildContext) gui.Widget { return gui.NewLabel(v.noise.Get()) }
func (v *trackingWidget) Update(_ BuildContext, w gui.Widget) {
	w.(*gui.Label).SetText(v.noise.Get())
}
func (v *trackingWidget) Unmount(BuildContext, gui.Widget) { v.noise.Get() }

func TestBuildNestedCoordinationAndWidgetHooksAreUntracked(t *testing.T) {
	trigger, noise := MakeState(true), MakeState("noise")
	bin := new(testBinTarget)
	builds := 0
	_, fake := bindingApp(t, func() RootView {
		return Window("main").Content(trackingView(func(ctx BuildContext) View {
			builds++
			ctx.UpdateChild(bin, Label("slot"), func(gui.Widget) { noise.Get() })
			ctx.AfterUpdate(func() { noise.Get() })
			if !trigger.Get() {
				return nil
			}
			v := &trackingWidget{noise: &noise}
			v.Self = v
			return v
		}))
	})
	noise.Set("ignored")
	if len(fake.posts) != 0 {
		t.Fatal("挂载、更新、槽位 apply 或完成回调的读取污染了父节点")
	}
	trigger.Set(false)
	fake.runPosted()
	noise.Set("after unmount")
	if builds != 2 || len(fake.posts) != 0 {
		t.Fatal("卸载回调污染了父节点的自动依赖")
	}
}

func TestReadConnectMayReleaseOwner(t *testing.T) {
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()
	r, source := newRoot(), new(trackingSource)
	defer r.unmountWindow()
	v := trackingView(func(ctx BuildContext) View {
		source.onConnect = func() { r.release(ctx.Coordinator().owner, true) }
		return Label(Read[string](source))
	})
	if got := r.update(v); got != nil || source.connects != 1 || source.disconnects != 1 || activeReads.Load() != nil {
		t.Fatal("Connect 期间释放 owner 后泄漏了新连接或收集器")
	}
}

var readBenchmarkValue int

// 使用相同空输出构建、来源规模和读取次数，对比显式 Observe 与自动 Get。
// Observe 路径用 Peek，保留旧 Get 的纯值读取成本，不叠加自动收集。
func BenchmarkCompositionReads(b *testing.B) {
	for _, automatic := range []bool{false, true} {
		mode := "Observe"
		if automatic {
			mode = "Automatic"
		}
		for _, count := range []int{1, 16} {
			b.Run(fmt.Sprintf("%s/%d", mode, count), func(b *testing.B) {
				runtime.LockOSThread()
				defer runtime.UnlockOSThread()
				values := make([]State[int], count)
				sources := make([]Observable, count)
				for i := range values {
					values[i] = MakeState(i)
					sources[i] = &values[i]
				}
				v := Build(func() View {
					total := 0
					for i := range values {
						if automatic {
							total += values[i].Get()
						} else {
							total += values[i].Peek()
						}
					}
					readBenchmarkValue = total
					return nil
				})
				if !automatic {
					v.Observe(sources...)
				}
				r := newRoot()
				r.update(v)
				b.ReportAllocs()
				b.ResetTimer()
				for range b.N {
					r.update(v)
				}
				b.StopTimer()
				r.unmountWindow()
			})
		}
	}
}

func TestAutomaticReadNotificationsFollowTransfer(t *testing.T) {
	value := MakeState("before")
	builds := 0
	rt, fake := bindingApp(t, func() RootView {
		return Root().Windows(
			Window("source").Content(VBox(Build(func() View { builds++; return Label(value.Get()) }))),
			Window("target").Content(VBox()),
		)
	})
	a, b := rt.windows["source"].root, rt.windows["target"].root
	from, to := fake.windows[0].Widget().(*gui.LinearBox), fake.windows[1].Widget().(*gui.LinearBox)
	n := a.root.children[0].nodes[0]
	label := from.Children()[0].(*gui.Label)
	value.Set("before transfer")
	if err := (&Coordinator{owner: a.root}).TransferChild(label, to, func() error {
		from.RemoveChild(label)
		to.AddChild(label)
		value.Set("during transfer")
		fake.runPosted()
		return nil
	}, nil); err != nil {
		t.Fatal(err)
	}
	rt.windows["source"].window.Destroy()
	fake.runPosted()
	if builds != 2 || n.root != b || n.released || label.Text() != "during transfer" || label.Parent() != to || len(n.tracked) != 1 {
		t.Fatal("自动来源或排队构建没有跟随节点移交")
	}
	value.Set("target")
	fake.runPosted()
	if builds != 3 || label.Text() != "target" {
		t.Fatal("目标 Root 未继续处理自动订阅")
	}
	rt.destroyAll()
	value.Set("released")
	if len(fake.posts) != 0 || n.tracked != nil {
		t.Fatal("释放移交节点后残留自动依赖")
	}
}

type modifierSizeView struct {
	ViewBase[modifierSizeView]
	size    *State[geometry.Size]
	updates *int
}

func (v *modifierSizeView) Mount(BuildContext) gui.Widget {
	w := gui.NewLabel("size")
	w.SetMinSize(v.size.Peek())
	return w
}
func (v *modifierSizeView) Update(ctx BuildContext, w gui.Widget) {
	*v.updates++
	BindValue(ctx, "size", v.size, w.SetMinSize)
}
func (*modifierSizeView) Unmount(BuildContext, gui.Widget) {}

func TestModifyBindingOverridesOnlyExplicitAxes(t *testing.T) {
	size := MakeState(geometry.Size{Width: 100, Height: 24})
	updates, builds := 0, 0
	_, fake := bindingApp(t, func() RootView {
		builds++
		v := &modifierSizeView{size: &size, updates: &updates}
		v.Self = v
		return Window("main").Content(Modify(v).MinWidth(200).Name("outer"))
	})
	w := fake.windows[0].Widget()
	size.Set(geometry.Size{Width: 300, Height: 48})
	fake.runPosted()
	if w.MinSize() != (geometry.Size{Width: 200, Height: 48}) || w.Name() != "outer" || updates != 1 || builds != 1 {
		t.Fatal("绑定覆盖了外层宽度，或重新应用默认值损坏绑定高度／执行完整更新")
	}
	size.Set(geometry.Size{Width: 60, Height: 72})
	fake.runPosted()
	if w.MinSize() != (geometry.Size{Width: 200, Height: 72}) || updates != 1 {
		t.Fatal("连续属性通知没有保持外层逐轴修饰")
	}
}

func TestBindingsStayLocalAndShareOneSubscription(t *testing.T) {
	source := &bindingSource{value: "initial"}
	global, local := 0, 0
	rt, fake := bindingApp(t, func() RootView {
		global++
		return Root().Windows(
			Window("a").Content(VBox(Label("static"), Label().ID("bound").BindText(source),
				Build(func() View { local++; return Label(source.Get()) }).Observe(source, source))),
			Window("b").Content(Label("other")),
		)
	})
	bound := fake.windows[0].Widget().Children()[1].(*gui.Label)
	source.Set("one")
	source.Set("two")
	if len(fake.posts) != 1 || bound.Text() != "initial" {
		t.Fatal("属性更新没有异步合并")
	}
	fake.runPosted()
	if global != 1 || local != 2 || bound.Text() != "two" || fake.windows[1].Widget().(*gui.Label).Text() != "other" {
		t.Fatalf("更新范围错误: global=%d local=%d text=%s", global, local, bound.Text())
	}
	if source.connects != 2 {
		t.Fatalf("同一节点重复来源没有去重: %d", source.connects)
	}
	for range 3 {
		rt.RequestUpdate()
		fake.runPosted()
	}
	if source.connects != 2 || source.disconnects != 0 {
		t.Fatal("完整更新重连了相同来源")
	}
	rt.destroyAll()
	if source.disconnects != 2 {
		t.Fatal("释放没有断开所有节点")
	}
	source.last() // 模拟 Disconnect 前已开始、随后才执行的通知。
	if len(fake.posts) != 0 {
		t.Fatal("迟到通知复活了已释放节点")
	}
}

func TestLocalBuildReplacesOutputsAndRetainsEmptySlots(t *testing.T) {
	value := MakeState(0)
	builds := 0
	_, fake := bindingApp(t, func() RootView {
		return Window("main").Content(VBox(Label("before"), Build(func() View {
			builds++
			switch value.Get() {
			case 0:
				return Label("middle")
			case 1:
				return TextInput().Text("new")
			default:
				return nil
			}
		}).Observe(&value), Label("after")))
	})
	box := fake.windows[0].Widget().(*gui.LinearBox)
	first, last := box.Children()[0], box.Children()[2]
	for _, v := range []int{1, 2, 0} {
		value.Set(v)
		fake.runPosted()
		children := box.Children()
		want := 3
		if v == 2 {
			want = 2
		}
		if len(children) != want || children[0] != first || children[len(children)-1] != last {
			t.Fatalf("输出替换破坏了相对顺序: %v", children)
		}
		if v == 1 {
			if _, ok := children[1].(*gui.TextInput); !ok {
				t.Fatal("类型替换未挂载")
			}
		}
	}
	if builds != 4 {
		t.Fatalf("空内容丢失订阅: builds=%d", builds)
	}
}

func TestLocalBuildReappliesOverlaySlot(t *testing.T) {
	show := MakeState(false)
	_, fake := bindingApp(t, func() RootView {
		return Window("main").Content(Overlay(nil,
			OverlayItem(Build(func() View {
				if show.Get() {
					return Label("floating")
				}
				return nil
			}).Observe(&show)).Fill(true)))
	})
	overlay := fake.windows[0].Widget().(*gui.Overlay)
	show.Set(true)
	fake.runPosted()
	if len(overlay.Children()) != 1 || !overlay.OverlayFill(overlay.Children()[0]) {
		t.Fatal("nil 输出恢复后丢失 Fill")
	}
	overlay.Measure(layout.Tight(geometry.Size{Width: 120, Height: 80}))
	overlay.Arrange(geometry.Rect(0, 0, 120, 80))
	if overlay.Children()[0].Rect().Size != (geometry.Size{Width: 120, Height: 80}) {
		t.Fatal("槽位配置没有参与布局")
	}
	show.Set(false)
	fake.runPosted()
	show.Set(true)
	fake.runPosted()
	if len(overlay.Children()) != 1 || !overlay.OverlayFill(overlay.Children()[0]) {
		t.Fatal("重复替换丢失槽位配置")
	}
}

func TestBindingSourceReplacementAndStaticOverride(t *testing.T) {
	old, next := &bindingSource{value: "old"}, &bindingSource{value: "new"}
	var source ReadState[string] = old
	rt, fake := bindingApp(t, func() RootView { return Window("main").Content(Label().BindText(source)) })
	label := fake.windows[0].Widget().(*gui.Label)
	old.Set("queued")
	source = next
	rt.RequestUpdate()
	fake.runPosted()
	if label.Text() != "new" || old.disconnects != 1 || next.connects != 1 {
		t.Fatal("旧请求覆盖了新来源")
	}
	old.last()
	if len(fake.posts) != 0 {
		t.Fatal("已替换来源仍安排更新")
	}
	rt.windows["main"].root.updateWindow(fake.windows[0], Label().BindText(next).Text("fixed"))
	next.Set("ignored")
	fake.runPosted()
	if label.Text() != "fixed" || next.disconnects != 1 {
		t.Fatal("Text 没有解除读绑定")
	}
}

func TestBindingUpdatesDuringApplyAreNotLost(t *testing.T) {
	source := &bindingSource{value: "first"}
	calls := 0
	_, fake := bindingApp(t, func() RootView {
		return Window("main").Content(Build(func() View {
			calls++
			v := source.Get()
			if v == "second" {
				source.Set("third")
			}
			return Label(v)
		}).Observe(source))
	})
	source.Set("second")
	fake.runPosted()
	if calls != 2 || len(fake.posts) != 1 {
		t.Fatal("构建期间的新通知应留给下一批")
	}
	fake.runPosted()
	if calls != 3 || fake.windows[0].Widget().(*gui.Label).Text() != "third" {
		t.Fatal("构建期间的下一批通知丢失")
	}
}

type retainedChildView struct {
	ViewBase[retainedChildView]
	child   View
	updates *int
}

func (v *retainedChildView) Mount(BuildContext) gui.Widget { return gui.NewButton() }
func (v *retainedChildView) Update(ctx BuildContext, w gui.Widget) {
	*v.updates++
	if ctx.State() == nil {
		ctx.SetState(true)
		ctx.UpdateChild(w.(Bin), v.child)
	}
}
func (*retainedChildView) Unmount(BuildContext, gui.Widget) {}

func TestFullUpdateDoesNotDropUnvisitedChild(t *testing.T) {
	text := MakeState("before")
	updates := 0
	rt, fake := bindingApp(t, func() RootView {
		v := &retainedChildView{child: Label().BindText(&text), updates: &updates}
		v.Self = v
		return Window("main").Content(v)
	})
	label := fake.windows[0].Widget().(*gui.Button).Child().(*gui.Label)
	text.Set("after")
	rt.RequestUpdate()
	fake.runPosted()
	if updates != 2 || label.Text() != "after" {
		t.Fatal("父 Update 未访问的子树失效被清掉")
	}
}

func TestBindingNotificationConcurrentWithRelease(t *testing.T) {
	text := MakeState("start")
	rt, fake := bindingApp(t, func() RootView { return Window("main").Content(Label().BindText(&text)) })
	var workers sync.WaitGroup
	workers.Add(1)
	go func() {
		defer workers.Done()
		for i := range 100 {
			text.Set(fmt.Sprint(i))
		}
	}()
	rt.destroyAll()
	workers.Wait()
	fake.runPosted()
	if len(rt.windows) != 0 {
		t.Fatal("后台通知复活窗口")
	}
	rt.mu.Lock()
	defer rt.mu.Unlock()
	if len(rt.dirty) != 0 {
		t.Fatal("释放请求没有回收")
	}
}

func TestSelectionBindingDoesNotReplayStaticText(t *testing.T) {
	for _, multiline := range []bool{false, true} {
		t.Run(fmt.Sprint(multiline), func(t *testing.T) {
			selection := MakeState(TextSelection{})
			_, fake := bindingApp(t, func() RootView {
				var v View = TextInput().Text("initial").BindSelection(&selection)
				if multiline {
					v = TextView().Text("initial").BindSelection(&selection)
				}
				return Window("main").Content(v)
			})
			w := fake.windows[0].Widget()
			if multiline {
				w.(*gui.TextView).Model().SetText("user text")
			} else {
				w.(*gui.TextInput).SetText("user text")
			}
			selection.Set(TextSelection{Anchor: 1, Caret: 3})
			fake.runPosted()
			if multiline {
				if w.(*gui.TextView).Model().Text() != "user text" || w.(*gui.TextView).Selection() != selection.Get() {
					t.Fatal("选区绑定重放了静态文本")
				}
			} else if w.(*gui.TextInput).Text() != "user text" || w.(*gui.TextInput).Selection() != selection.Get() {
				t.Fatal("选区绑定重放了静态文本")
			}
		})
	}
}

func TestTextBindingRemainsLocalAndDoesNotEcho(t *testing.T) {
	text, selection := MakeState("abc"), MakeState(TextSelection{Anchor: 1, Caret: 2})
	writes, global := 0, 0
	text.Connect(func() { writes++ })
	_, fake := bindingApp(t, func() RootView {
		global++
		return Window("main").Content(TextInput().BindText(&text).BindSelection(&selection))
	})
	input := fake.windows[0].Widget().(*gui.TextInput)
	text.Set("new value")
	selection.Set(TextSelection{Anchor: 2, Caret: 4})
	fake.runPosted()
	if input.Text() != "new value" || input.Selection() != selection.Get() || global != 1 || writes != 1 {
		t.Fatal("文本属性绑定重建了应用或循环写回")
	}
	input.SetText("user edit")
	fake.runPosted()
	if text.Get() != "user edit" || global != 1 {
		t.Fatal("编辑回写丢失或触发全局更新")
	}
}

type localContextView struct {
	ComposeBase[localContextView]
	context *BuildContext
	text    *string
}

func (v *localContextView) Build(ctx BuildContext) View {
	*v.context = ctx
	if *v.text == "" {
		return nil
	}
	return Label(*v.text)
}

func TestLocalContextUpdatesEmptyWindowAndExpiresOnStop(t *testing.T) {
	var ctx BuildContext
	text, builds := "", 0
	rt, fake := bindingApp(t, func() RootView {
		builds++
		v := &localContextView{context: &ctx, text: &text}
		v.Self = v
		return Window("main").Content(v)
	})
	text = "ready"
	ctx.RequestUpdate()
	fake.runPosted()
	if builds != 1 || fake.windows[0].Widget().(*gui.Label).Text() != text {
		t.Fatal("空输出根节点未独立更新")
	}
	rt.stop()
	ctx.RequestUpdate()
	if len(fake.posts) != 0 || fake.windows[0].Widget() == nil {
		t.Fatal("退出后上下文失效，GUI 仍拥有其控件树")
	}
}

func TestObservedParentDeduplicatesOnlyVisitedChildren(t *testing.T) {
	source := MakeState("initial")
	parent, child := 0, 0
	_, fake := bindingApp(t, func() RootView {
		return Window("main").Content(Build(func() View {
			parent++
			return VBox(Build(func() View { child++; return Label(source.Get()) }).Observe(&source))
		}).Observe(&source))
	})
	source.Set("after")
	fake.runPosted()
	if parent != 2 || child != 2 {
		t.Fatalf("同一批父子重复构建: parent=%d child=%d", parent, child)
	}
}

func TestPendingBindingFollowsTransferAndSourceWindowClose(t *testing.T) {
	source := &bindingSource{value: "before"}
	rt, fake := bindingApp(t, func() RootView {
		return Root().Windows(Window("source").Content(VBox(Label().BindText(source))), Window("target").Content(VBox()))
	})
	a, b := rt.windows["source"].root, rt.windows["target"].root
	from, to := fake.windows[0].Widget().(*gui.LinearBox), fake.windows[1].Widget().(*gui.LinearBox)
	label := from.Children()[0].(*gui.Label)
	source.Set("queued before transfer")
	if err := (&Coordinator{owner: a.root}).TransferChild(label, to, func() error {
		from.RemoveChild(label)
		to.AddChild(label)
		source.Set("during transfer")
		fake.runPosted() // 移交期间的嵌套原生循环不得消费失效。
		return nil
	}, nil); err != nil {
		t.Fatal(err)
	}
	rt.windows["source"].window.Destroy()
	fake.runPosted()
	if label.Text() != "during transfer" || label.Parent() != to || b.widget() != to || source.connects != 1 || source.disconnects != 0 {
		t.Fatal("排队通知没有跟随移交，或源窗口释放了目标订阅")
	}
	source.Set("destination")
	fake.runPosted()
	if label.Text() != "destination" {
		t.Fatal("目标节点未继续订阅")
	}
}

func TestAppExitDisconnectsBindingsWithoutDestroyingGUI(t *testing.T) {
	source := &bindingSource{value: "before"}
	rt, fake := bindingApp(t, func() RootView { return Window("main").Content(Label().BindText(source)) })
	w := fake.windows[0].Widget()
	rt.stop()
	rt.stop()
	source.Set("after")
	if source.disconnects != 1 || len(fake.posts) != 0 || fake.windows[0].Widget() != w || fake.windows[0].destroyed {
		t.Fatal("退出未断开绑定，或 UI 夺走了 GUI 树所有权")
	}
}

type getterNotificationSource struct {
	bindingSource
	notified bool
}

func (s *getterNotificationSource) Get() string {
	if !s.notified {
		s.notified = true
		s.Set("published inside Get")
	}
	return s.bindingSource.Get()
}

func TestBindingConnectsBeforeFirstRead(t *testing.T) {
	source := new(getterNotificationSource)
	_, fake := bindingApp(t, func() RootView { return Window("main").Content(Label().BindText(source)) })
	if source.connects != 1 || len(fake.posts) != 1 {
		t.Fatal("首次 Get 期间的通知丢失")
	}
	fake.runPosted()
	if source.reads != 2 || fake.windows[0].Widget().(*gui.Label).Text() != "published inside Get" {
		t.Fatal("首次读通知未被应用")
	}
}

func TestBindingDeclarationPrecedenceAndConfiguration(t *testing.T) {
	text := MakeState("source")
	selection := MakeState(TextSelection{})
	calls := 0
	r := newRoot()
	t.Cleanup(r.unmountWindow)
	input := r.update(TextInput().BindText(&text).OnText(func(string) { calls++ })).(*gui.TextInput)
	input.SetText("custom")
	if calls != 1 || text.Get() != "source" {
		t.Fatal("OnText 未覆盖生成的回写")
	}
	r.update(TextInput().OnText(func(string) { calls++ }).BindText(&text))
	input.SetText("write back")
	if calls != 1 || text.Get() != "write back" {
		t.Fatal("后声明的 BindText 未覆盖回调")
	}
	r.update(TextInput().BindText(&text).OnText(func(string) { calls++ }).Text("static"))
	input.SetText("custom again")
	if calls != 2 || text.Get() != "write back" {
		t.Fatal("Text 不应删除显式 OnText")
	}
	r.update(TextInput().BindText(&text).BindText(nil).BindSelection(&selection).BindSelection(nil))
	input.SetText("nil keeps binding")
	if text.Get() != "nil keeps binding" {
		t.Fatal("nil 清除了有效绑定")
	}
	for _, bad := range []func(){
		func() { Build(nil) },
		func() { Read[string](nil) },
		func() { Read[string]((*bindingSource)(nil)) },
		func() { Label("a", "b") },
		func() { Build(func() View { return nil }).Observe((*bindingSource)(nil)) },
		func() { Label().BindText((*bindingSource)(nil)) },
	} {
		func() {
			defer func() {
				if recover() == nil {
					t.Error("无效配置没有拒绝")
				}
			}()
			bad()
		}()
	}
}

type completionView struct {
	ComposeBase[completionView]
	text      *State[string]
	id        string
	completed func()
}

func (v *completionView) Build(ctx BuildContext) View {
	ctx.AfterUpdate(v.completed)
	return Label(v.text.Get()).ID(v.id)
}

func TestLocalAfterUpdateSeesAllAffectedWindows(t *testing.T) {
	text := MakeState("before")
	var rt *app
	calls := 0
	completed := func() {
		if rt == nil {
			return
		} // 首次挂载由既有全窗口测试覆盖。
		for _, id := range []string{"a", "b"} {
			if rt.windows[id].window.Widget().(*gui.Label).Text() != text.Get() {
				t.Fatal("完成回调先于其他受影响窗口")
			}
		}
		calls++
	}
	var fake *windowTestApplication
	rt, fake = bindingApp(t, func() RootView {
		view := func(id string) View {
			v := &completionView{text: &text, id: id, completed: completed}
			v.Self = v
			return v.Observe(&text)
		}
		return Root().Windows(Window("a").Content(view("a")), Window("b").Content(view("b")))
	})
	text.Set("after")
	fake.runPosted()
	if calls != 2 {
		t.Fatalf("完成回调次数=%d", calls)
	}
}

func TestSharedValueBindingsKeepIndependentDocumentsAndUndo(t *testing.T) {
	text := MakeState("ab")
	_, fake := bindingApp(t, func() RootView {
		declaration := TextView().BindText(&text)
		return Root().Windows(Window("a").Content(declaration), Window("b").Content(declaration))
	})
	a, b := fake.windows[0].Widget().(*gui.TextView), fake.windows[1].Widget().(*gui.TextView)
	if a.Model() == b.Model() {
		t.Fatal("共用声明导致两个节点共用文档")
	}
	if err := a.Model().Replace(TextRange{Start: 2, End: 2}, "c"); err != nil {
		t.Fatal(err)
	}
	fake.runPosted()
	if b.Model().Text() != "abc" || !a.Model().CanUndo() || b.Model().CanUndo() {
		t.Fatal("值绑定回放破坏了源历史或共享了目标历史")
	}
}
