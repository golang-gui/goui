package ui

import (
	"slices"
	"testing"

	"github.com/golang-gui/goui/gui"
)

type compositionState struct{ builds int }

type stateComposition struct {
	ComposeBase[stateComposition]
	child View
	state **compositionState
	after func(BuildContext)
}

func compose(id string, child View, state **compositionState) *stateComposition {
	v := &stateComposition{child: child, state: state}
	v.Self = v
	return v.ID(id)
}

func (v *stateComposition) Build(ctx BuildContext) View {
	if ctx.State() == nil {
		ctx.SetState(&compositionState{})
	}
	state := ctx.State().(*compositionState)
	state.builds++
	if v.state != nil {
		*v.state = state
	}
	if v.after != nil {
		ctx.AfterUpdate(func() { v.after(ctx) })
	}
	return v.child
}

func TestCompositionKeepsStateAndUpdatesRootProjection(t *testing.T) {
	r := newRoot()
	win := newTestWindow()
	var state *compositionState
	label := r.updateWindow(win, compose("component", Label("first"), &state))
	original := state
	if label.ID() != "" || win.Widget() != label || r.root.body.parent != r.root {
		t.Fatal("组合身份被转发给 Widget 或声明父子关系丢失")
	}
	if got := r.updateWindow(win, compose("component", Label("second"), &state)); got != label || state != original || state.builds != 2 {
		t.Fatal("重建没有保留组合状态及相同类型的输出")
	}
	if got := r.updateWindow(win, compose("component", nil, &state)); got != nil || win.Widget() != nil || r.root == nil || state != original {
		t.Fatal("空内容没有清空窗口输出或删除了组合节点")
	}
	button := r.updateWindow(win, compose("component", Button("new output"), &state))
	if _, ok := button.(*gui.Button); !ok || win.Widget() != button || state != original {
		t.Fatal("恢复内容未替换输出，或替换输出时重建了组合状态")
	}
	r.update(compose("replacement", nil, &state))
	if state == original || state.builds != 1 {
		t.Fatal("修改声明 ID 没有替换组合身份")
	}
	r.unmountWindow()
}

func TestCompositionProjectionUpdatesBinAndListWithoutWrapper(t *testing.T) {
	r := newRoot()
	var state *compositionState
	button := r.update(Button().Child(compose("content", Label("first"), &state))).(*gui.Button)
	label := button.Child()
	original := state
	r.update(Button().Child(compose("content", TextInput().Text("replacement"), &state)))
	if button.Child() == label || label.Parent() != nil || state != original {
		t.Fatal("单槽位组合输出替换失败")
	}
	r.update(Button().Child(compose("content", nil, &state)))
	if button.Child() != nil || state != original {
		t.Fatal("单槽位空输出没有移除 Widget 或保留状态")
	}
	r.unmountWindow()

	a, b := newRoot(), newRoot()
	defer a.unmountWindow()
	defer b.unmountWindow()
	var leftState, emptyState *compositionState
	box := a.update(VBox(compose("left", Label("left"), &leftState), compose("empty", nil, &emptyState), Label("right").ID("right"))).(*gui.LinearBox)
	left, right := box.Children()[0], box.Children()[1]
	leftOriginal, emptyOriginal := leftState, emptyState
	a.update(VBox(Label("RIGHT").ID("right"), compose("empty", nil, &emptyState), compose("left", Label("LEFT"), &leftState)))
	if !slices.Equal(box.Children(), []gui.Widget{right, left}) || leftState != leftOriginal || emptyState != emptyOriginal {
		t.Fatal("带空组合节点的排序没有保留状态与 GUI 顺序")
	}
	a.update(VBox(Label("RIGHT").ID("right"), compose("empty", Label("now visible"), &emptyState), compose("left", TextInput(), &leftState)))
	if len(box.Children()) != 3 || box.Children()[0] != right || box.Children()[1].(*gui.Label).Text() != "now visible" || left.Parent() != nil || leftState != leftOriginal || emptyState != emptyOriginal {
		t.Fatal("列表中的空/替换输出没有正确更新实际挂载目标")
	}
	// 相同声明值在不同 Root 中不能共享 State。
	b.update(compose("empty", nil, &state))
	if state == emptyState {
		t.Fatal("不同 Root 的组合共享了可变状态")
	}
}

func TestCompositionAfterUpdateUsesCompleteCurrentTree(t *testing.T) {
	r := newRoot()
	win := newTestWindow()
	calls := 0
	v := compose("component", Label("ready"), nil)
	v.after = func(ctx BuildContext) {
		calls++
		if win.Widget() != r.widget() || ctx.State().(*compositionState).builds != 1 || r.widget().(*gui.Label).Text() != "ready" {
			t.Fatal("组合完成回调观察到未挂载或过期的内容")
		}
	}
	r.updateWindow(win, v)
	if calls != 1 {
		t.Fatal("组合节点的完成回调未恰好调用一次")
	}
	r.unmountWindow()
}

func TestCompositionReleaseDuringReplacementDoesNotMountNewState(t *testing.T) {
	r := newRoot()
	created, released := 0, 0
	v := objectScope("old", Label("old"), func() any { return new(int) })
	v.release = func(any) { released++; r.unmountWindow() }
	r.update(v)
	replacement := objectScope("new", Label("new"), func() any { created++; return new(int) })
	r.update(replacement)
	if r.root != nil || created != 0 || released != 1 {
		t.Fatal("旧对象释放时销毁 Root，随后却重新挂载了新对象")
	}
}

func TestCompositionBuildDestructionReleasesInFlightObject(t *testing.T) {
	r := newRoot()
	released := 0
	v := objectScope("in-flight", Label("unused"), func() any {
		r.unmountWindow()
		return new(int)
	})
	v.released = &released
	if got := r.update(v); got != nil || r.root != nil || released != 1 {
		t.Fatal("构建回调销毁 Root 后遗留了正在创建的对象或视觉内容")
	}
}

func TestObjectMissingGroupReleasesPartialConstruction(t *testing.T) {
	r := newRoot()
	released := 0
	v := groupScope("declared", namedCheck("check", "absent", true))
	v.released = &released
	defer func() {
		if recover() == nil {
			t.Fatal("缺少显式组声明没有报告配置错误")
		}
		if released != 1 || r.root != nil || r.reconciling {
			t.Fatal("构建异常遗留了对象状态或协调标志")
		}
	}()
	r.update(v)
}

func TestObjectListFailureReleasesPreviouslyConstructedSiblings(t *testing.T) {
	r := newRoot()
	released := 0
	first := groupScope("first", Label("unattached"))
	second := groupScope("second", namedCheck("check", "absent", true))
	first.released, second.released = &released, &released
	defer func() {
		if recover() == nil {
			t.Fatal("缺少组的列表构建没有失败")
		}
		if released != 2 || r.root != nil || r.reconciling {
			t.Fatal("后一个声明失败后，先前构建的未挂载对象没有释放")
		}
	}()
	r.update(VBox(first, second))
}

type ambiguousView struct{ stateComposition }

func (*ambiguousView) Mount(BuildContext) gui.Widget    { return gui.NewLabel("") }
func (*ambiguousView) Update(BuildContext, gui.Widget)  {}
func (*ambiguousView) Unmount(BuildContext, gui.Widget) {}

func TestCompositionRejectsAmbiguousKindsAndWidgetModifiers(t *testing.T) {
	withModifier := &testCompositionView{view: Label("unused")}
	withModifier.fields.Set(viewMinWidth, true)
	for _, declaration := range []View{
		&ambiguousView{},
		withModifier,
	} {
		r := newRoot()
		original := r.update(Label("unchanged"))
		func() {
			defer func() {
				if recover() == nil {
					t.Fatal("无效的声明类型或 Widget 修饰器未被拒绝")
				}
			}()
			r.update(declaration)
		}()
		if r.widget() != original || original.(*gui.Label).Text() != "unchanged" {
			t.Fatal("声明校验失败却替换了已有内容")
		}
		r.unmountWindow()
	}
}
