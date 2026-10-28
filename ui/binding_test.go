package ui

import (
	"fmt"
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
	fake := newWindowTestApplication()
	rt := newApp(fake, build)
	t.Cleanup(func() { rt.stop(); rt.destroyAll() })
	if err := rt.rebuild(); err != nil {
		t.Fatal(err)
	}
	return rt, fake
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
