package ui

import (
	"errors"
	"testing"

	"github.com/golang-gui/goui/gui"
	"github.com/golang-gui/goui/platform/events"
	"github.com/golang-gui/goui/widgets"
)

// 使用真实协调器及 GUI 组；此对象声明替身不创建窗口或占位 Widget。
type testObjectState struct{ value any }

type testObjectView struct {
	ComposeBase[testObjectView]
	child    View
	create   func() any
	read     func(BuildContext) any
	release  func(any)
	onMove   func(BuildContext)
	created  *int
	released *int
}

func objectScope(id string, child View, create func() any) *testObjectView {
	v := &testObjectView{child: child, create: create}
	v.Self = v
	return v.ID(id)
}

func groupScope(id string, child View) *testObjectView {
	return objectScope(id, child, func() any { return widgets.NewCheckButtonGroup() })
}

func (v *testObjectView) Build(ctx BuildContext) View {
	if ctx.State() == nil {
		if v.created != nil {
			*v.created++
		}
		ctx.SetState(&testObjectState{value: v.create()})
	}
	return v.child
}

func (v *testObjectView) Object(ctx BuildContext) any {
	if v.read != nil {
		return v.read(ctx)
	}
	return ctx.State().(*testObjectState).value
}

func (v *testObjectView) Release(ctx BuildContext) {
	if v.released != nil {
		*v.released++
	}
	if v.release != nil {
		if state, ok := ctx.State().(*testObjectState); ok {
			v.release(state.value)
		}
	}
}

func (v *testObjectView) Reparented(ctx BuildContext) {
	if v.onMove != nil {
		v.onMove(ctx)
	}
}

type groupedCheckView struct {
	ViewBase[groupedCheckView]
	name    string
	checked bool
	saved   *BuildContext
}

func namedCheck(id, name string, checked bool) *groupedCheckView {
	v := &groupedCheckView{name: name, checked: checked}
	v.Self = v
	return v.ID(id)
}

func (v *groupedCheckView) Mount(ctx BuildContext) gui.Widget {
	b := widgets.NewCheckButton()
	ctx.SetState(b)
	if v.saved != nil {
		*v.saved = ctx
	}
	return b
}

func (v *groupedCheckView) Update(ctx BuildContext, widget gui.Widget) {
	v.Reparented(ctx)
	widget.(*widgets.CheckButton).SetChecked(v.checked)
}

func (*groupedCheckView) Unmount(_ BuildContext, widget gui.Widget) {
	widget.(*widgets.CheckButton).SetGroup(nil)
}

func (v *groupedCheckView) Reparented(ctx BuildContext) {
	b := ctx.State().(*widgets.CheckButton)
	var group *widgets.CheckButtonGroup
	if v.name != "" {
		var ok bool
		group, ok = FindAncestor[*widgets.CheckButtonGroup](ctx, v.name)
		if !ok {
			panic("test: missing explicit group " + v.name)
		}
	}
	b.SetGroup(group)
}

func TestObjectGroupReconcilesAcrossContainersAndIsolatesDeclarations(t *testing.T) {
	r, other := newRoot(), newRoot()
	build := func(selected bool) View {
		return groupScope("choices", VBox(
			HBox(namedCheck("a", "choices", selected)),
			VBox(namedCheck("b", "choices", !selected)),
			namedCheck("independent", "", true),
		))
	}
	box := r.update(build(true))
	a := box.Children()[0].Children()[0].(*widgets.CheckButton)
	b := box.Children()[1].Children()[0].(*widgets.CheckButton)
	g := a.Group()
	if g == nil || b.Group() != g || g.Checked() != a || box.Children()[2].(*widgets.CheckButton).Group() != nil {
		t.Fatal("跨容器的显式分组或独立按钮不正确")
	}
	if box.ID() != "" || len(box.Children()) != 3 {
		t.Fatal("对象声明向 GUI 树添加了包装节点或转发了声明 ID")
	}
	foreign := other.update(groupScope("choices", namedCheck("foreign", "choices", true))).(*widgets.CheckButton)
	if foreign.Group() == g || !foreign.Checked() || g.Checked() != a {
		t.Fatal("不同声明树的同名组相互干扰")
	}
	changes := 0
	a.ConnectChange(func(widgets.CheckState) { changes++ })
	b.ConnectChange(func(widgets.CheckState) { changes++ })
	r.update(build(false))
	if a.Group() != g || b.Group() != g || g.Checked() != b || changes != 0 || box.Children()[0].Children()[0] != a {
		t.Fatal("重建替换了组/控件或静默同步发出了 Change")
	}
	r.unmountWindow()
	if a.Group() != nil || b.Group() != nil || g.Checked() != nil || !foreign.Checked() {
		t.Fatal("卸载保留旧关联或影响其他声明树")
	}
	other.unmountWindow()
}

func TestObjectLookupUsesOwnerContextAndNearestID(t *testing.T) {
	r := newRoot()
	var ctx BuildContext
	check := namedCheck("check", "shared", true)
	check.saved = &ctx
	outer := objectScope("shared", groupScope("shared", check), func() any { return new(int) })
	r.update(outer)
	if _, ok := FindAncestor[*int](ctx, "shared"); ok {
		t.Fatal("最近的同名声明类型不符时越过它查询了外层")
	}
	if value, ok := FindAncestor[*widgets.CheckButtonGroup](ctx, "shared"); !ok || value.Checked() == nil {
		t.Fatal("未用对象所属节点的 State 查询显式组")
	}
	if _, ok := FindAncestor[any](ctx, "absent"); ok {
		t.Fatal("不存在的名字被隐式创建")
	}
	r.unmountWindow()
	if _, ok := FindAncestor[*widgets.CheckButtonGroup](ctx, "shared"); ok {
		t.Fatal("释放的上下文仍能取得祖先对象")
	}
}

func TestObjectGroupSurvivesEmptyContentAndReorder(t *testing.T) {
	r := newRoot()
	created, released := 0, 0
	makeScope := func(content View) *testObjectView {
		v := groupScope("shared", content)
		v.created, v.released = &created, &released
		return v
	}
	box := r.update(makeScope(VBox(namedCheck("a", "shared", true), namedCheck("b", "shared", false)))).(*gui.LinearBox)
	a, b := box.Children()[0].(*widgets.CheckButton), box.Children()[1].(*widgets.CheckButton)
	g := a.Group()
	r.update(makeScope(VBox(namedCheck("b", "shared", false), namedCheck("a", "shared", true))))
	if created != 1 || released != 0 || box.Children()[0] != b || box.Children()[1] != a || b.Group() != g {
		t.Fatal("排序重新创建了组或其成员")
	}
	r.update(makeScope(nil))
	if r.widget() != nil || r.root == nil || created != 1 || released != 0 || g.Checked() != nil || a.Group() != nil {
		t.Fatal("空内容删除了对象声明或未解除成员")
	}
	resumed := r.update(makeScope(namedCheck("a", "shared", true))).(*widgets.CheckButton)
	if resumed.Group() != g || created != 1 {
		t.Fatal("恢复内容没有复用对象实例")
	}
	r.unmountWindow()
	r.unmountWindow()
	if released != 1 || resumed.Group() != nil {
		t.Fatal("释放未解除成员或执行了多次")
	}
}

func TestObjectGroupReleaseFollowsMemberDisconnect(t *testing.T) {
	for _, destroy := range []bool{false, true} {
		r := newRoot()
		calls := 0
		v := groupScope("shared", VBox(namedCheck("check", "shared", true)))
		v.release = func(value any) {
			calls++
			if value.(*widgets.CheckButtonGroup).Checked() != nil {
				t.Fatal("对象释放时成员仍保持关联")
			}
		}
		box := r.update(v).(*gui.LinearBox)
		check := box.Children()[0]
		if destroy {
			r.unmountForWindowDestroy()
			if len(box.Children()) != 1 || box.Children()[0] != check {
				t.Fatal("窗口销毁的 UI 清理擅自拆除了 GUI 树")
			}
		} else {
			r.unmountWindow()
			if len(box.Children()) != 0 {
				t.Fatal("普通卸载未移除受管理的 GUI 内容")
			}
		}
		if calls != 1 {
			t.Fatal("对象释放未恰好执行一次")
		}
	}
}

func TestObjectGroupTransferRefreshesAssociationWithoutReplayingValues(t *testing.T) {
	for _, fail := range []bool{false, true} {
		a, b := transferRoots()
		source := a.update(groupScope("shared", VBox(namedCheck("moving", "shared", false)))).(*gui.LinearBox)
		target := b.update(groupScope("shared", VBox(namedCheck("resident", "shared", true)))).(*gui.LinearBox)
		moving, resident := source.Children()[0].(*widgets.CheckButton), target.Children()[0].(*widgets.CheckButton)
		old, destination := moving.Group(), resident.Group()
		win := newTestWindow()
		win.SetWidget(source)
		win.SetFocusedWidget(moving)
		if err := new(gui.EventDispatcher).DispatchEvent(win, events.KeyEvent{EventType: events.KeyDown, Key: events.KeySpace}); err != nil {
			t.Fatal(err)
		}
		failure := errors.New("move rejected")
		err := (&Coordinator{owner: a.root.body}).TransferChild(moving, target, func() error {
			if fail {
				return failure
			}
			source.RemoveChild(moving)
			target.AddChild(moving)
			return nil
		}, func() {
			if moving.Group() != destination || !moving.Checked() || resident.Checked() {
				t.Fatal("完成回调未看到新关联，或旧 Checked(false) 声明覆盖了真实输入")
			}
		})
		if fail {
			if err != failure || moving.Group() != old || old.Checked() != moving {
				t.Fatal("失败回滚改变了关联或选择")
			}
		} else if err != nil || moving.Group() != destination || old.Checked() != nil {
			t.Fatalf("移交未解除源关联：%v", err)
		}
		a.unmountWindow()
		b.unmountWindow()
		if moving.Group() != nil {
			t.Fatal("移交后的释放遗留关联")
		}
	}
}

func TestObjectDeclarationMovesWithItsProjectedWidget(t *testing.T) {
	a, b := transferRoots()
	created, released := 0, 0
	v := groupScope("owned", VBox(namedCheck("check", "owned", true)))
	v.created, v.released = &created, &released
	source := a.update(VBox(v)).(*gui.LinearBox)
	target := b.update(VBox()).(*gui.LinearBox)
	output := source.Children()[0]
	check := output.Children()[0].(*widgets.CheckButton)
	g := check.Group()
	declaration := a.root.children[0].nodes[0]
	if err := (&Coordinator{owner: a.root}).TransferChild(output, target, func() error {
		source.RemoveChild(output)
		target.AddChild(output)
		return nil
	}, nil); err != nil {
		t.Fatal(err)
	}
	if b.root.children[0].nodes[0] != declaration || check.Group() != g || created != 1 || released != 0 {
		t.Fatal("只移交了内部 Widget，遗漏组合祖先或重新创建了对象")
	}
	a.unmountWindow()
	if check.Group() != g || released != 0 {
		t.Fatal("源 Root 释放影响了已移交的对象")
	}
	b.unmountWindow()
	if released != 1 || check.Group() != nil {
		t.Fatal("目标 Root 没有正确释放完整声明")
	}
}

func TestObjectLookupRejectsResultsAfterCallbackDestruction(t *testing.T) {
	r := newRoot()
	var ctx BuildContext
	v := namedCheck("check", "", false)
	v.saved = &ctx
	scope := objectScope("object", v, func() any { return new(int) })
	r.update(scope)
	scope.read = func(owner BuildContext) any {
		value := owner.State().(*testObjectState).value
		r.unmountWindow()
		return value
	}
	if _, ok := FindAncestor[*int](ctx, "object"); ok || r.root != nil {
		t.Fatal("对象回调销毁声明后仍返回了过期对象")
	}
}

func TestObjectGroupTransferBetweenScopesInSameRoot(t *testing.T) {
	r := newRoot()
	box := r.update(HBox(
		VBox(groupScope("shared", VBox(namedCheck("moving", "shared", true)))).ID("left"),
		VBox(groupScope("shared", VBox(namedCheck("resident", "shared", true)))).ID("right"),
	)).(*gui.LinearBox)
	source := box.Children()[0].Children()[0].(*gui.LinearBox)
	target := box.Children()[1].Children()[0].(*gui.LinearBox)
	moving := source.Children()[0].(*widgets.CheckButton)
	resident := target.Children()[0].(*widgets.CheckButton)
	old, next := moving.Group(), resident.Group()
	if old == next {
		t.Fatal("同一 Root 的不同显式声明错误共享了组实例")
	}
	if err := (&Coordinator{owner: findNode(r.root, source)}).TransferChild(moving, target, func() error {
		source.RemoveChild(moving)
		target.AddChild(moving)
		return nil
	}, nil); err != nil {
		t.Fatal(err)
	}
	if moving.Group() != next || old.Checked() != nil || resident.Checked() || !moving.Checked() {
		t.Fatal("同 Root 移交没有按新的声明祖先重新关联")
	}
	r.unmountWindow()
}

func TestObjectReparentedCallbackDestructionStopsCommit(t *testing.T) {
	a, b := transferRoots()
	released := 0
	v := objectScope("moving", Label("content"), func() any { return new(int) })
	v.released = &released
	v.onMove = func(BuildContext) { b.unmountWindow() }
	source := a.update(VBox(v)).(*gui.LinearBox)
	target := b.update(VBox()).(*gui.LinearBox)
	child := source.Children()[0]
	committed := false
	err := (&Coordinator{owner: a.root}).TransferChild(child, target, func() error {
		source.RemoveChild(child)
		target.AddChild(child)
		return nil
	}, func() { committed = true })
	if err == nil || committed || released != 1 {
		t.Fatal("祖先变化回调销毁目标后仍提交成功或未释放对象")
	}
	a.unmountWindow()
}

func TestObjectLookupRejectsResultAfterCallerMovesWithinRoot(t *testing.T) {
	r := newRoot()
	defer r.unmountWindow()
	var ctx BuildContext
	v := namedCheck("moving", "", false)
	v.saved = &ctx
	oldValue, nextValue := new(int), new(int)
	left := objectScope("value", VBox(v), func() any { return oldValue })
	right := objectScope("value", VBox(), func() any { return nextValue })
	box := r.update(HBox(VBox(left).ID("left"), VBox(right).ID("right"))).(*gui.LinearBox)
	source := box.Children()[0].Children()[0].(*gui.LinearBox)
	target := box.Children()[1].Children()[0].(*gui.LinearBox)
	child := source.Children()[0]
	left.read = func(BuildContext) any {
		if err := (&Coordinator{owner: findNode(r.root, source)}).TransferChild(child, target, func() error {
			source.RemoveChild(child)
			target.AddChild(child)
			return nil
		}, nil); err != nil {
			t.Fatal(err)
		}
		return oldValue
	}
	if _, ok := FindAncestor[*int](ctx, "value"); ok {
		t.Fatal("查询期间调用者移到同 Root 的另一作用域，仍返回旧祖先对象")
	}
	if value, ok := FindAncestor[*int](ctx, "value"); !ok || value != nextValue {
		t.Fatal("移交后的查询没有使用新祖先")
	}
}
