package ui

import (
	"testing"

	"github.com/golang-gui/goui/gui"
	"github.com/golang-gui/goui/platform/events"
)

func TestCommandScopeSharesOperationAndRetainsIdentity(t *testing.T) {
	r := newRoot()
	defer r.unmountWindow()
	w := &overlayInputWindow{testWindow: newTestWindow()}
	first, current := 0, 0
	build := func(fn func(), enabled bool) *CommandScopeView {
		return CommandScope(VBox(
			MenuButton("Edit").ID("menu").Menu(MenuItem("Copy").Command("copy")),
			Label("editor"),
		).Shortcuts(Shortcut(KeyF5).Command("copy"))).Commands(Command("copy").Enabled(enabled).OnActivate(fn))
	}
	box := r.updateWindow(w, build(func() { first++ }, true))
	menu := box.Children()[0].(*gui.MenuButton)
	command := menu.Menu().ItemAt(0).Command()
	if command == nil || len(command.Shortcuts()) != 1 || first != 0 {
		t.Fatal("声明没有关联同一操作，或构建执行了回调")
	}
	press := func() {
		t.Helper()
		if err := w.DispatchEvent(events.KeyEvent{EventType: events.KeyDown, Key: events.KeyF5}); err != nil {
			t.Fatal(err)
		}
	}
	press()
	for range 3 {
		r.updateWindow(w, build(func() { current++ }, true))
	}
	if menu.Menu().ItemAt(0).Command() != command || len(command.Shortcuts()) != 1 {
		t.Fatal("重建替换操作或重复关联")
	}
	press()
	if first != 1 || current != 1 {
		t.Fatal("执行了过期或重复回调")
	}
	r.updateWindow(w, build(func() { current++ }, false))
	press()
	if command.Enabled() || current != 1 || !menu.Menu().ItemAt(0).Enabled() {
		t.Fatal("命令禁用没有生效或覆盖了菜单局部设置")
	}
	r.unmountWindow()
	if command.Enabled() || len(command.Shortcuts()) != 0 {
		t.Fatal("卸载未禁用旧操作或解除快捷键关联")
	}
	command.Activate()
	if current != 1 {
		t.Fatal("卸载后仍然执行业务回调")
	}
}

func TestCommandScopeShadowFallbackAndRemoval(t *testing.T) {
	r := newRoot()
	defer r.unmountWindow()
	makeInner := func(commands ...*CommandView) *CommandScopeView {
		return CommandScope(MenuButton("Inner").Menu(MenuItem("Copy").Command("copy"), MenuItem("Other").Command("other"))).Commands(commands...)
	}
	build := func(inner *CommandScopeView) *CommandScopeView {
		return CommandScope(inner).Commands(Command("copy"), Command("other"))
	}
	menu := r.update(build(makeInner(Command("copy")))).(*gui.MenuButton)
	inner := menu.Menu().ItemAt(0).Command()
	outer := r.root.state.(*commandScope).commands["copy"].command
	if inner == outer || inner == nil || menu.Menu().ItemAt(1).Command() == nil {
		t.Fatal("内层遮蔽或外层回退错误")
	}
	r.update(build(makeInner()))
	if menu.Menu().ItemAt(0).Command() != outer || inner.Enabled() {
		t.Fatal("移除内层定义未回退或旧命令仍启用")
	}
	r.update(CommandScope(makeInner()).Commands())
	if menu.Menu().ItemAt(0).Command().Enabled() {
		t.Fatal("缺失 ID 执行了已移除目标")
	}
	r.update(build(makeInner()))
	if !menu.Menu().ItemAt(0).Command().Enabled() {
		t.Fatal("重新定义后引用没有恢复")
	}
}

func TestCommandScopeUpdatesRetainedDescendants(t *testing.T) {
	r := newRoot()
	defer r.unmountWindow()
	menuView := MenuButton("Edit").Menu(MenuItem("Copy").Command("copy"))
	view := func(commands ...*CommandView) *CommandScopeView { return CommandScope(menuView).Commands(commands...) }
	menu := r.update(view(Command("copy"))).(*gui.MenuButton)
	original := menu.Menu().ItemAt(0).Command()
	// 相同子声明与模型保持稳定，依赖作用域通知更新引用。
	model := menu.Menu()
	r.update(view())
	if menu.Menu() != model || menu.Menu().ItemAt(0).Command() == original || menu.Menu().ItemAt(0).Command().Enabled() {
		t.Fatal("删除定义后仍使用旧引用")
	}
	r.update(view(Command("copy")))
	if menu.Menu() != model || !menu.Menu().ItemAt(0).Command().Enabled() {
		t.Fatal("恢复定义未更新稳定模型")
	}
}

func TestCommandScopeRejectsDuplicateDefinitions(t *testing.T) {
	for _, views := range [][]*CommandView{{Command("copy"), Command("copy")}, {{id: ""}}} {
		func() {
			r := newRoot()
			defer r.unmountWindow()
			defer func() {
				if recover() == nil {
					t.Error("接受了重复或空命令定义")
				}
			}()
			r.update(CommandScope(Label("child")).Commands(views...))
		}()
	}
}

func TestCommandScopeContextMenuAndDirectCallbackModes(t *testing.T) {
	r := newRoot()
	defer r.unmountWindow()
	var ctx BuildContext
	probe := objectScope("probe", Label("probe"), func() any { return new(int) })
	probe.read = func(owner BuildContext) any { ctx = owner; return new(int) }
	// 使用真实子节点上下文，查询公共解析接口，不访问另一 View 的私有 State。
	r.update(CommandScope(probe).Commands(Command("copy")))
	ctx = &buildContext{node: r.root.body}
	command, ok := FindCommand(ctx, "copy")
	if !ok {
		t.Fatal("公开查询找不到祖先命令")
	}
	model := MenuFor(ctx, MenuItem("Copy").Command("copy"))
	if model.ItemAt(0).Command() != command {
		t.Fatal("上下文菜单未解析 ID")
	}
	if Menu(MenuItem("Copy").Command("copy")).ItemAt(0).Command().Enabled() {
		t.Fatal("无上下文转换不应暴露启用的未解析命令")
	}
	if _, ok := FindCommand(ctx, "missing"); ok {
		t.Fatal("未知 ID 错误解析")
	}
	r.unmountWindow()
	if _, ok := FindCommand(ctx, "copy"); ok {
		t.Fatal("过期上下文仍可解析")
	}
}

func TestCommandScopeTransferRebindsMenuAndShortcut(t *testing.T) {
	a, b := transferRoots()
	defer a.unmountWindow()
	defer b.unmountWindow()
	first, second := 0, 0
	source := a.update(CommandScope(VBox(
		MenuButton("Moving").ID("moving").Menu(MenuItem("Run").Command("run")).Shortcuts(Shortcut(KeyF5).Command("run")),
	)).Commands(Command("run").OnActivate(func() { first++ }))).(*gui.LinearBox)
	target := b.update(CommandScope(VBox(Label("Resident"))).Commands(Command("run").OnActivate(func() { second++ }))).(*gui.LinearBox)
	moving := source.Children()[0].(*gui.MenuButton)
	old := moving.Menu().ItemAt(0).Command()
	model := moving.Menu()
	err := (&Coordinator{owner: a.root.body}).TransferChild(moving, target, func() error { source.RemoveChild(moving); target.AddChild(moving); return nil }, nil)
	if err != nil {
		t.Fatal(err)
	}
	next := moving.Menu().ItemAt(0).Command()
	if moving.Menu() != model || next == old || len(old.Shortcuts()) != 0 || len(next.Shortcuts()) != 1 {
		t.Fatal("移交未重解析菜单或共享修饰符中的快捷键")
	}
	next.Activate()
	if first != 0 || second != 1 {
		t.Fatal("移交后仍执行旧作用域操作")
	}
}

func TestCommandScopeNotifiesConsumersWithoutRebuildingThem(t *testing.T) {
	r := newRoot()
	defer r.unmountWindow()
	inner := CommandScope(MenuButton("Inner").Menu(MenuItem("Copy").Command("copy"))).Commands(Command("copy"))
	menu := r.update(CommandScope(inner).Commands(Command("copy"))).(*gui.MenuButton)
	original := menu.Menu().ItemAt(0).Command()
	outer := r.root.state.(*commandScope).commands["copy"].command
	// 仅更新作用域自己的声明，刻意不协调子节点，验证引用依赖集合通知而非重建副作用。
	ctx := &buildContext{node: r.root.body}
	CommandScope(inner.child).Build(ctx)
	if menu.Menu().ItemAt(0).Command() != outer || original.Enabled() {
		t.Fatal("未重建的后代没有跟随定义移除回退")
	}
	CommandScope(inner.child).Commands(Command("copy")).Build(ctx)
	if menu.Menu().ItemAt(0).Command() == outer {
		t.Fatal("未重建的后代没有跟随同名遮蔽恢复")
	}
}

func TestCommandContextMenuTracksScopeWhileObserved(t *testing.T) {
	r := newRoot()
	defer r.unmountWindow()
	child := Label("content")
	r.update(CommandScope(CommandScope(child).Commands(Command("copy"))).Commands(Command("copy")))
	ctx := &buildContext{node: r.root.body.body}
	model := MenuFor(ctx, MenuItem("Copy").Command("copy"))
	old := model.ItemAt(0).Command()
	n := 0
	h := model.ConnectItems(func() { n++ })
	defer h.Disconnect()
	CommandScope(child).Build(&buildContext{node: r.root.body})
	outer := r.root.state.(*commandScope).commands["copy"].command
	if old.Enabled() || model.ItemAt(0).Command() != outer || n != 1 {
		t.Fatal("已打开上下文菜单未跟随作用域移除回退")
	}
	h.Disconnect()
	CommandScope(child).Commands(Command("copy")).Build(&buildContext{node: r.root.body})
	if n != 1 {
		t.Fatal("解除模型观察后仍保留作用域通知")
	}
}

func TestCommandShortcutNotificationCanUnmountOwner(t *testing.T) {
	r := newRoot()
	defer r.unmountWindow()
	view := func(key Key) *CommandScopeView {
		return CommandScope(Label("editor").Shortcuts(Shortcut(key).Command("run"), Shortcut(KeyF6).Command("run"))).Commands(Command("run"))
	}
	r.update(view(KeyF5))
	command := r.root.state.(*commandScope).commands["run"].command
	armed := true
	command.ConnectShortcuts(func() {
		if armed {
			armed = false
			r.unmountWindow()
		}
	})
	r.update(view(KeyF7))
	if len(command.Shortcuts()) != 0 || command.Enabled() {
		t.Fatal("通知中卸载后保留了快捷键或启用的命令")
	}
}

func TestCommandReferenceLookupCanReleaseOwner(t *testing.T) {
	r := newRoot()
	defer r.unmountWindow()
	armed := false
	child := objectScope("interceptor", MenuButton("Menu").Menu(MenuItem("Run").Command("run")), func() any { return new(int) })
	child.read = func(BuildContext) any {
		if armed {
			armed = false
			r.unmountWindow()
		}
		return new(int)
	}
	r.update(CommandScope(child).Commands(Command("run")))
	ctx := &buildContext{node: r.root}
	armed = true
	// 成员通知会重新解析引用；中间 ObjectView 回调释放 owner 后必须停止旧解析。
	CommandScope(child).Build(ctx)
	if r.root != nil {
		t.Fatal("查询回调没有完成卸载")
	}
}
