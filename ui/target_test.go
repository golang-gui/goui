package ui

import (
	"errors"
	"fmt"
	"strings"
	"testing"

	"github.com/golang-gui/goui/core/geometry"
	"github.com/golang-gui/goui/gui"
)

// 探针只声明目标引用；用实际协调器验证生命周期，不另建目标解析替身。
type targetTestView[T any] struct {
	ViewBase[targetTestView[T]]
	target  string
	apply   func(T)
	updates *int
	skip    bool
	after   func()
}

func targetTest[T any](id string, apply func(T)) *targetTestView[T] {
	v := &targetTestView[T]{target: id, apply: apply}
	v.Self = v
	return v
}

func (*targetTestView[T]) Mount(BuildContext) gui.Widget { return gui.NewButton() }
func (v *targetTestView[T]) Update(ctx BuildContext, _ gui.Widget) {
	if v.updates != nil {
		*v.updates++
	}
	if !v.skip {
		BindTarget(ctx, "target", v.target, v.apply)
	}
	if v.after != nil {
		ctx.AfterUpdate(v.after)
	}
}
func (*targetTestView[T]) Unmount(BuildContext, gui.Widget) {}

func TestTargetResolvesForwardOutputBeforeAfterUpdate(t *testing.T) {
	r := newRoot()
	defer r.unmountHost()
	var bound *gui.Label
	completed := false
	probe := targetTest("target", func(w *gui.Label) {
		bound = w
		if w != nil && w.Parent() == nil {
			t.Fatal("目标在输出挂载前被应用")
		}
	})
	probe.after = func() {
		completed = true
		if bound == nil || bound.Text() != "ready" {
			t.Fatal("AfterUpdate 没有看到本批目标")
		}
	}
	r.update(VBox(probe, Modify(Build(func() View {
		return Label("ready").ID("inner")
	})).ID("target")))
	if !completed || bound == nil || bound.ID() != "target" {
		t.Fatal("前向引用或组合输出的最终 ID 没有生效")
	}
}

func TestTargetFollowsLocalReplacementWithoutUpdatingConsumer(t *testing.T) {
	version := MakeState(0)
	outer, sources, consumers, applications := 0, 0, 0, 0
	var bound gui.Widget
	rt, fake := bindingApp(t, func() RootView {
		outer++
		probe := targetTest("target", func(w gui.Widget) { applications++; bound = w })
		probe.updates = &consumers
		return Window("main").Content(VBox(probe, Modify(Build(func() View {
			sources++
			v := version.Get()
			if v < 0 {
				return nil
			}
			if v == 2 {
				return Button("new type")
			}
			return Label(fmt.Sprint(v)).ID(fmt.Sprint(v))
		})).ID("target")))
	})
	first := bound
	if first == nil || applications != 1 {
		t.Fatal("首次引用没有解析")
	}
	version.Set(1)
	fake.runPosted()
	second := bound
	if second == nil || second == first || consumers != 1 || outer != 1 || sources != 2 {
		t.Fatal("替换目标要求重建引用方", consumers, outer, sources)
	}
	version.Set(-1)
	fake.runPosted()
	if bound != nil {
		t.Fatal("目标消失后仍保留旧对象")
	}
	version.Set(2)
	fake.runPosted()
	if _, ok := bound.(*gui.Button); !ok || consumers != 1 || outer != 1 {
		t.Fatal("目标晚出现或类型替换未刷新引用")
	}
	if len(rt.windows["main"].root.targets) != 1 {
		t.Fatal("局部更新累积重复关联")
	}
}

func TestTargetPreservesUnchangedResultsAndAppliesLatestDeclaration(t *testing.T) {
	r := newRoot()
	defer r.unmountHost()
	first, second := 0, 0
	makeView := func(apply func(*gui.Label)) View {
		return VBox(targetTest("target", apply), Build(func() View { return Label("value").ID("target") }))
	}
	r.update(makeView(func(w *gui.Label) {
		if w != nil {
			first++
		}
	}))
	producer := r.root.children[0].nodes[1]
	r.updateLocal(producer)
	r.flushAfterUpdate()
	if first != 1 {
		t.Fatal("目标实例未变仍重复应用关联")
	}
	r.update(makeView(func(w *gui.Label) {
		if w != nil {
			second++
		}
	}))
	if first != 1 || second != 1 {
		t.Fatal("引用方更新没有使用最新回调", first, second)
	}
}

func TestTargetClearsRemovedDeclarationsAndReleasedOwners(t *testing.T) {
	r := newRoot()
	var bound *gui.Label
	cleared := 0
	apply := func(w *gui.Label) {
		bound = w
		if w == nil {
			cleared++
		}
	}
	probe := targetTest("target", apply)
	r.update(VBox(probe, Label("value").ID("target")))
	if bound == nil {
		t.Fatal("首次引用没有解析")
	}
	probe = targetTest("target", apply)
	probe.skip = true
	r.update(VBox(probe, Label("value").ID("target")))
	if bound != nil || cleared != 1 || len(r.targets) != 0 {
		t.Fatal("省略登记没有解除引用")
	}
	r.update(VBox(targetTest("target", apply), Label("value").ID("target")))
	r.unmountHost()
	if bound != nil || cleared != 2 || len(r.targets) != 0 {
		t.Fatal("释放所有者保留了关联或重复清理")
	}
	r.flushAfterUpdate()
	if cleared != 2 {
		t.Fatal("旧完成任务复活已释放的关联")
	}
}

func TestTargetUsesWidgetIDsAndIsolatesRoots(t *testing.T) {
	a, b := newRoot(), newRoot()
	defer a.unmountHost()
	defer b.unmountHost()
	var left, right *gui.Label
	a.update(VBox(targetTest("target", func(w *gui.Label) { left = w }), Build(func() View {
		return Label("not named")
	}).ID("target")))
	if left != nil {
		t.Fatal("仅用于协调的 ComposeView ID 被当成目标名")
	}
	a.update(VBox(targetTest("target", func(w *gui.Label) { left = w }), Label("left").ID("target")))
	b.update(VBox(targetTest("target", func(w *gui.Label) { right = w }), Label("right").ID("target")))
	if left == nil || right == nil || left == right || left.Text() != "left" || right.Text() != "right" {
		t.Fatal("不同 Root 的同名目标互相污染")
	}
	a.update(VBox(targetTest("", func(w *gui.Label) { left = w }), Label("left").ID("target")))
	if left != nil || right == nil {
		t.Fatal("空 ID 未解除目标或污染其他 Root")
	}
}

func TestTargetRejectsFinalAmbiguityAndIncompatibleTypes(t *testing.T) {
	for _, test := range []struct {
		name, message string
		content       func() View
	}{
		{"duplicate", "duplicate Widget outputs", func() View {
			return VBox(VBox(Label("a").ID("target")), VBox(Label("b").ID("target")))
		}},
		{"incompatible", "want *gui.Label", func() View { return Button("wrong type").ID("target") }},
	} {
		t.Run(test.name, func(t *testing.T) {
			r := newRoot()
			defer r.unmountHost()
			applied := 0
			defer func() {
				failure := recover()
				if failure == nil || !strings.Contains(fmt.Sprint(failure), test.message) || applied != 0 {
					t.Fatalf("最终配置错误未在应用新连接前报告：%v，回调=%d", failure, applied)
				}
			}()
			r.update(VBox(
				targetTest("good", func(*gui.Label) { applied++ }), Label("good").ID("good"),
				targetTest("target", func(*gui.Label) { applied++ }), test.content(),
			))
		})
	}
	// 没有被引用的重复输出 ID 不新增全局唯一性限制。
	r := newRoot()
	defer r.unmountHost()
	r.update(VBox(VBox(Label("a").ID("local")), VBox(Label("b").ID("local"))))
}

func TestTargetStopsPendingCallbacksWhenApplicationReleasesRoot(t *testing.T) {
	r := newRoot()
	first, second := 0, 0
	r.update(VBox(
		targetTest("target", func(w *gui.Label) {
			if w != nil {
				first++
				r.unmountHost()
			}
		}),
		targetTest("target", func(w *gui.Label) {
			if w != nil {
				second++
			}
		}),
		Label("value").ID("target"),
	))
	if first != 1 || second != 0 || len(r.targets) != 0 || r.root != nil {
		t.Fatal("目标回调释放 Root 后仍执行过期任务", first, second)
	}
}

func TestTargetDelayedRowsCommitWithoutAfterUpdate(t *testing.T) {
	app := newWindowTestApplication()
	useTestApplication(t, app)
	r := newRoot()
	defer r.unmountHost()
	var bound *gui.Label
	widget := r.update(VBox(targetTest("row", func(w *gui.Label) { bound = w }),
		ListView(SliceList([]int{1}), func(int, int) View { return Label("row").ID("row") }),
	))
	list := widget.Children()[1].(*gui.ListView)
	list.LayoutVisible(geometry.Size{Width: 100, Height: 40}, geometry.Point{})
	if bound != nil {
		t.Fatal("延迟行在布局挂载结束前应用目标")
	}
	app.runPosted()
	if bound == nil || bound.Parent() == nil {
		t.Fatal("没有普通 AfterUpdate 时丢失延迟目标提交")
	}
	list.SetModel(gui.NewSliceListModel([]int{}))
	list.LayoutVisible(geometry.Size{Width: 100, Height: 40}, geometry.Point{})
	app.runPosted()
	if bound != nil {
		t.Fatal("虚拟行回收后保留目标引用")
	}
}

func TestTargetTransfersConsumerIntoNewRootNamespace(t *testing.T) {
	a, b := transferRoots()
	defer a.unmountHost()
	defer b.unmountHost()
	var bound *gui.Label
	source := a.update(VBox(targetTest("target", func(w *gui.Label) { bound = w }).ID("consumer"), Label("left").ID("target"))).(*gui.LinearBox)
	destination := b.update(VBox(Label("right").ID("target"))).(*gui.LinearBox)
	consumer := source.Children()[0]
	if err := (&Coordinator{owner: a.root}).TransferChild(consumer, destination, func() error {
		source.RemoveChild(consumer)
		destination.AddChild(consumer)
		return nil
	}, nil); err != nil {
		t.Fatal(err)
	}
	if bound == nil || bound.Text() != "right" || len(a.targets) != 0 || len(b.targets) != 1 {
		t.Fatal("移交未在完成时采用新命名空间")
	}
}

func TestTargetDeclarationChangesAndMissingReferencesDoNotRevive(t *testing.T) {
	name := MakeState("first")
	var bound *gui.Label
	consumers := 0
	rt, fake := bindingApp(t, func() RootView {
		return Window("main").Content(VBox(Build(func() View {
			probe := targetTest(name.Get(), func(w *gui.Label) { bound = w })
			probe.updates = &consumers
			return probe
		}), Label("first").ID("first"), Label("second").ID("second")))
	})
	if bound == nil || bound.Text() != "first" {
		t.Fatal("初始声明未应用")
	}
	name.Set("second")
	fake.runPosted()
	if bound == nil || bound.Text() != "second" || consumers != 2 {
		t.Fatal("局部修改 Target ID 未切换实例")
	}
	name.Set("missing")
	fake.runPosted()
	if bound != nil {
		t.Fatal("修改为缺失 ID 没有解除旧引用")
	}
	rt.windows["main"].root.unmountForHostDestroy()
	name.Set("first")
	fake.runPosted()
	if bound != nil || consumers != 3 {
		t.Fatal("宿主销毁后状态通知复活了关联")
	}
}

func TestTargetCommitsEveryRootBeforeCompletionCallbacks(t *testing.T) {
	version := MakeState(0)
	bound := make(map[string]*gui.Label)
	var rt *app
	completed := 0
	var fake *windowTestApplication
	rt, fake = bindingApp(t, func() RootView {
		view := func(id string) View {
			probe := targetTest("target", func(w *gui.Label) { bound[id] = w })
			return VBox(probe, Modify(trackingView(func(ctx BuildContext) View {
				v := version.Get()
				ctx.AfterUpdate(func() {
					if rt == nil {
						return
					}
					for _, window := range []string{"a", "b"} {
						if bound[window] == nil || bound[window].Text() != fmt.Sprint(version.Get()) {
							t.Fatal("AfterUpdate 运行时其他 Root 的 Target 尚未提交")
						}
					}
					completed++
				})
				return Label(fmt.Sprint(v)).ID(fmt.Sprint(v))
			})).ID("target"))
		}
		return Root().Windows(Window("a").Content(view("a")), Window("b").Content(view("b")))
	})
	version.Set(1)
	fake.runPosted()
	if completed != 2 {
		t.Fatal("没有执行两棵局部更新树的完成回调", completed)
	}
}

type targetProducerView struct {
	ViewBase[targetProducerView]
	unmount func()
}

func (*targetProducerView) Mount(BuildContext) gui.Widget      { return gui.NewLabel("producer") }
func (*targetProducerView) Update(BuildContext, gui.Widget)    {}
func (v *targetProducerView) Unmount(BuildContext, gui.Widget) { v.unmount() }

func TestTargetClearsBeforeProducerUnmountAndToleratesReleaseReentry(t *testing.T) {
	visible := MakeState(true)
	var r *root
	var bound *gui.Label
	cleared, unmounted := 0, 0
	producer := &targetProducerView{unmount: func() {
		unmounted++
		if bound != nil {
			t.Fatal("目标 Unmount 时引用方仍持有旧连接")
		}
	}}
	producer.Self = producer
	rt, fake := bindingApp(t, func() RootView {
		return Window("main").Content(VBox(targetTest("target", func(w *gui.Label) {
			bound = w
			if w == nil {
				cleared++
				r.unmountHost()
			}
		}), Build(func() View {
			if visible.Get() {
				return producer.ID("target")
			}
			return nil
		})))
	})
	r = rt.windows["main"].root
	// 零值回调销毁 Root，不能再次清理同一关联或跳过原生产者的卸载。
	visible.Set(false)
	fake.runPosted()
	if bound != nil || cleared != 1 || unmounted != 1 || len(r.targets) != 0 {
		t.Fatal("解除引用期间重入释放破坏生命周期", cleared, unmounted)
	}
}

func TestTargetFailedTransferRestoresReferenceOwnership(t *testing.T) {
	a, b := transferRoots()
	defer a.unmountHost()
	defer b.unmountHost()
	var bound *gui.Label
	applications := 0
	source := a.update(VBox(targetTest("target", func(w *gui.Label) {
		bound = w
		if w != nil {
			applications++
		}
	}).ID("consumer"), Label("left").ID("target"))).(*gui.LinearBox)
	destination := b.update(VBox(Label("right").ID("target"))).(*gui.LinearBox)
	consumer, original := source.Children()[0], bound
	rejected := errors.New("拒绝移交")
	err := (&Coordinator{owner: a.root}).TransferChild(consumer, destination, func() error { return rejected }, nil)
	if !errors.Is(err, rejected) || bound != original || applications != 1 || len(a.targets) != 1 || len(b.targets) != 0 {
		t.Fatal("失败移交改变了原引用或泄漏到目标 Root", err, applications)
	}
	if source.Children()[0] != consumer || consumer.Parent() != source {
		t.Fatal("失败移交没有恢复原成员")
	}
}
