package ui

import (
	"strings"
	"testing"

	"github.com/golang-gui/goui/gui"
	"github.com/golang-gui/goui/platform/events"
)

func TestCommandGroupSharesOperationAndRetainsIdentity(t *testing.T) {
	r := newRoot()
	defer r.unmountWindow()
	w := &overlayInputWindow{testWindow: newTestWindow()}
	first, current := 0, 0
	build := func(fn func(), enabled bool) *CommandGroupView {
		return CommandGroup(VBox(
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

func TestCommandGroupShadowFallbackAndRemoval(t *testing.T) {
	r := newRoot()
	defer r.unmountWindow()
	makeInner := func(commands ...*CommandView) *CommandGroupView {
		return CommandGroup(MenuButton("Inner").Menu(MenuItem("Copy").Command("copy"), MenuItem("Other").Command("other"))).Commands(commands...)
	}
	build := func(inner *CommandGroupView) *CommandGroupView {
		return CommandGroup(inner).Commands(Command("copy"), Command("other"))
	}
	menu := r.update(build(makeInner(Command("copy")))).(*gui.MenuButton)
	inner := menu.Menu().ItemAt(0).Command()
	outer := r.root.state.(*commandGroup).commands["copy"].command
	if inner == outer || inner == nil || menu.Menu().ItemAt(1).Command() == nil {
		t.Fatal("内层遮蔽或外层回退错误")
	}
	r.update(build(makeInner()))
	if menu.Menu().ItemAt(0).Command() != outer || inner.Enabled() {
		t.Fatal("移除内层定义未回退或旧命令仍启用")
	}
	r.update(CommandGroup(makeInner()).Commands())
	if menu.Menu().ItemAt(0).Command().Enabled() {
		t.Fatal("缺失 ID 执行了已移除目标")
	}
	r.update(build(makeInner()))
	if !menu.Menu().ItemAt(0).Command().Enabled() {
		t.Fatal("重新定义后引用没有恢复")
	}
}

func TestCommandGroupUpdatesRetainedDescendants(t *testing.T) {
	r := newRoot()
	defer r.unmountWindow()
	menuView := MenuButton("Edit").Menu(MenuItem("Copy").Command("copy"))
	view := func(commands ...*CommandView) *CommandGroupView { return CommandGroup(menuView).Commands(commands...) }
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

func TestCommandGroupRejectsDuplicateDefinitions(t *testing.T) {
	for _, views := range [][]*CommandView{{Command("copy"), Command("copy")}, {{id: ""}}} {
		func() {
			r := newRoot()
			defer r.unmountWindow()
			defer func() {
				if recover() == nil {
					t.Error("接受了重复或空命令定义")
				}
			}()
			r.update(CommandGroup(Label("child")).Commands(views...))
		}()
	}
}

func TestCommandGroupContextMenuAndDirectCallbackModes(t *testing.T) {
	r := newRoot()
	defer r.unmountWindow()
	var ctx BuildContext
	probe := objectScope("probe", Label("probe"), func() any { return new(int) })
	probe.read = func(owner BuildContext) any { ctx = owner; return new(int) }
	// 使用真实子节点上下文，查询公共解析接口，不访问另一 View 的私有 State。
	r.update(CommandGroup(probe).Commands(Command("copy")))
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

func TestCommandGroupTransferRebindsMenuAndShortcut(t *testing.T) {
	a, b := transferRoots()
	defer a.unmountWindow()
	defer b.unmountWindow()
	first, second := 0, 0
	source := a.update(CommandGroup(VBox(
		MenuButton("Moving").ID("moving").Menu(MenuItem("Run").Command("run")).Shortcuts(Shortcut(KeyF5).Command("run")),
	)).Commands(Command("run").OnActivate(func() { first++ }))).(*gui.LinearBox)
	target := b.update(CommandGroup(VBox(Label("Resident"))).Commands(Command("run").OnActivate(func() { second++ }))).(*gui.LinearBox)
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

func TestCommandGroupNotifiesConsumersWithoutRebuildingThem(t *testing.T) {
	r := newRoot()
	defer r.unmountWindow()
	inner := CommandGroup(MenuButton("Inner").Menu(MenuItem("Copy").Command("copy"))).Commands(Command("copy"))
	menu := r.update(CommandGroup(inner).Commands(Command("copy"))).(*gui.MenuButton)
	original := menu.Menu().ItemAt(0).Command()
	outer := r.root.state.(*commandGroup).commands["copy"].command
	// 仅更新作用域自己的声明，刻意不协调子节点，验证引用依赖集合通知而非重建副作用。
	ctx := &buildContext{node: r.root.body}
	CommandGroup(inner.child).Build(ctx)
	r.flushAfterUpdate() // 定义在本轮协调结束后发布，保留“不重建后代”的原断言。
	if menu.Menu().ItemAt(0).Command() != outer || original.Enabled() {
		t.Fatal("未重建的后代没有跟随定义移除回退")
	}
	CommandGroup(inner.child).Commands(Command("copy")).Build(ctx)
	r.flushAfterUpdate()
	if menu.Menu().ItemAt(0).Command() == outer {
		t.Fatal("未重建的后代没有跟随同名遮蔽恢复")
	}
}

func TestCommandContextMenuTracksScopeWhileObserved(t *testing.T) {
	r := newRoot()
	defer r.unmountWindow()
	child := Label("content")
	r.update(CommandGroup(CommandGroup(child).Commands(Command("copy"))).Commands(Command("copy")))
	ctx := &buildContext{node: r.root.body.body}
	model := MenuFor(ctx, MenuItem("Copy").Command("copy"))
	old := model.ItemAt(0).Command()
	n := 0
	h := model.ConnectItems(func() { n++ })
	defer h.Disconnect()
	CommandGroup(child).Build(&buildContext{node: r.root.body})
	r.flushAfterUpdate()
	outer := r.root.state.(*commandGroup).commands["copy"].command
	if old.Enabled() || model.ItemAt(0).Command() != outer || n != 1 {
		t.Fatal("已打开上下文菜单未跟随作用域移除回退")
	}
	h.Disconnect()
	CommandGroup(child).Commands(Command("copy")).Build(&buildContext{node: r.root.body})
	r.flushAfterUpdate()
	if n != 1 {
		t.Fatal("解除模型观察后仍保留作用域通知")
	}
}

func TestCommandShortcutNotificationCanUnmountOwner(t *testing.T) {
	r := newRoot()
	defer r.unmountWindow()
	view := func(key Key) *CommandGroupView {
		return CommandGroup(Label("editor").Shortcuts(Shortcut(key).Command("run"), Shortcut(KeyF6).Command("run"))).Commands(Command("run"))
	}
	r.update(view(KeyF5))
	command := r.root.state.(*commandGroup).commands["run"].command
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
	r.update(CommandGroup(child).Commands(Command("run")))
	ctx := &buildContext{node: r.root}
	armed = true
	// 成员通知会重新解析引用；中间 ObjectView 回调释放 owner 后必须停止旧解析。
	CommandGroup(child).Build(ctx)
	r.flushAfterUpdate()
	if r.root != nil {
		t.Fatal("查询回调没有完成卸载")
	}
}

// 定义附着在已有入口上；先引用后声明以及跨分支移动在同一轮提交。
func TestCommandDescendantDefinitionsAndMoves(t *testing.T) {
	r := newRoot()
	defer r.unmountWindow()
	calls := 0
	build := func(left bool, callback func()) View {
		a, b := Label("left"), Label("right")
		definition := Shortcut(KeyF5).DefineCommand(Command("run").OnActivate(callback))
		if left {
			a.Shortcuts(definition)
		} else {
			b.Shortcuts(definition)
		}
		return CommandGroup(VBox(
			MenuButton("Run").Menu(MenuItem("Run").Command("run")),
			HBox(a).Spacing(20),
			HBox(b).Spacing(20),
		))
	}
	box := r.update(build(false, func() { calls++ })).(*gui.LinearBox)
	menu := box.Children()[0].(*gui.MenuButton)
	command, model := menu.Menu().ItemAt(0).Command(), menu.Menu()
	if !command.Enabled() || len(box.Children()) != 3 || len(box.Children()[2].Children()) != 1 {
		t.Fatal("非可视定义增加了 Widget／布局间距，或向前引用未解析")
	}
	command.Activate()
	for _, left := range []bool{true, false, true} {
		r.update(build(left, func() { calls += 10 }))
		if menu.Menu() != model || menu.Menu().ItemAt(0).Command() != command || !command.Enabled() {
			t.Fatal("同轮移动定义替换了对象或菜单")
		}
		command.Activate()
	}
	if calls != 31 {
		t.Fatalf("回调不是最新声明或重复执行：%d", calls)
	}
	// Widget 隐藏不卸载其声明；只有移除定义才注销操作。
	r.update(CommandGroup(VBox(
		MenuButton("Run").Menu(MenuItem("Run").Command("run")),
		HBox(Label("hidden").Shortcuts(Shortcut(KeyF5).DefineCommand(Command("run").OnActivate(func() { calls++ })))).Visible(false),
	)))
	if menu.Menu().ItemAt(0).Command() != command || !command.Enabled() {
		t.Fatal("隐藏子树注销了仍挂载的定义")
	}
	r.update(CommandGroup(VBox(MenuButton("Run").Menu(MenuItem("Run").Command("run")))))
	command.Activate()
	if command.Enabled() || menu.Menu().ItemAt(0).Command().Enabled() || calls != 31 {
		t.Fatal("卸载后操作或引用没有失效")
	}
}

func TestCommandDescendantDefinitionErrors(t *testing.T) {
	for _, tc := range []struct {
		name string
		view View
		want string
	}{
		{"outside-scope", MenuButton("Run").Menu(MenuItem("Run").DefineCommand(Command("run"))), "DefineCommand requires an ancestor CommandGroup"},
		{"siblings", CommandGroup(VBox(MenuButton("A").Menu(MenuItem("Run").DefineCommand(Command("run"))), MenuButton("B").Menu(MenuItem("Run").DefineCommand(Command("run"))))), `duplicate command ID "run"`},
		{"shorthand-and-descendant", CommandGroup(MenuButton("Run").Menu(MenuItem("Run").DefineCommand(Command("run")))).Commands(Command("run")), `duplicate command ID "run"`},
	} {
		t.Run(tc.name, func(t *testing.T) {
			r := newRoot()
			defer r.unmountWindow()
			defer func() {
				if message, ok := recover().(string); !ok || !strings.Contains(message, tc.want) {
					t.Fatalf("声明错误不符：%q，期望包含 %q", message, tc.want)
				}
			}()
			r.update(tc.view)
		})
	}
}

func TestCommandBindEnabledDoesNotRebuildContent(t *testing.T) {
	for _, host := range []string{"menu", "shortcut", "scope"} {
		t.Run(host, func(t *testing.T) {
			enabled, other := MakeState(true), MakeState(false)
			source := &enabled
			bound, direct := true, true
			global, content := 0, 0
			rt, fake := bindingApp(t, func() RootView {
				global++
				d := Command("run").Enabled(direct)
				if bound {
					d.BindEnabled(source)
				}
				child := Build(func() View { content++; return Label("editor") })
				item := MenuItem("Run").Command("run")
				if host == "menu" {
					item.DefineCommand(d)
				}
				box := VBox(MenuButton("Run").Menu(item), child)
				scope := CommandGroup(box)
				if host == "shortcut" {
					box.Shortcuts(Shortcut(KeyF5).DefineCommand(d))
				} else if host == "scope" {
					scope.Commands(d)
				}
				return Window("main").Content(scope)
			})
			box := fake.windows[0].Widget()
			menu := box.Children()[0].(*gui.MenuButton)
			command, model := menu.Menu().ItemAt(0).Command(), menu.Menu()
			notifications := 0
			command.ConnectEnabled(func(bool) { notifications++ })
			enabled.Set(false)
			fake.runPosted()
			if command.Enabled() || global != 1 || content != 1 || menu.Menu() != model || box.Children()[1].(*gui.Label).Text() != "editor" || notifications != 1 {
				t.Fatal("绑定没有局部更新，或重建了编辑子树／菜单")
			}
			enabled.Set(true)
			fake.runPosted()
			if !command.Enabled() || global != 1 || content != 1 || notifications != 2 {
				t.Fatal("恢复启用没有保持更新范围")
			}
			// 换源后旧来源不再投递；Enabled 恢复普通 setter 语义。
			source = &other
			if err := rt.rebuild(); err != nil {
				t.Fatal(err)
			}
			enabled.Set(false)
			if len(fake.posts) != 0 || command.Enabled() {
				t.Fatal("换源未解绑旧状态")
			}
			other.Set(true)
			fake.runPosted()
			if !command.Enabled() || global != 2 || content != 2 {
				t.Fatal("新来源未接管可用性")
			}
			bound, direct = false, false
			if err := rt.rebuild(); err != nil {
				t.Fatal(err)
			}
			other.Set(false)
			if len(fake.posts) != 0 || command.Enabled() {
				t.Fatal("取消绑定没有恢复直接声明")
			}
			rt.destroyAll()
			other.Set(true)
			if len(fake.posts) != 0 || command.Enabled() {
				t.Fatal("销毁后仍有绑定或启用对象")
			}
		})
	}
	if Command("run").BindEnabled(new(State[bool])).Enabled(false).enabledSource != nil {
		t.Fatal("Enabled 没有替换之前的 BindEnabled")
	}
}

func TestCommandDescendantLocalAddRemoveAndFallback(t *testing.T) {
	show := MakeState(false)
	global, local := 0, 0
	_, fake := bindingApp(t, func() RootView {
		global++
		return Window("main").Content(CommandGroup(CommandGroup(VBox(
			MenuButton("Run").Menu(MenuItem("Run").Command("run")),
			Build(func() View {
				local++
				view := Label("definition host")
				if show.Get() {
					view.Shortcuts(Shortcut(KeyF5).DefineCommand(Command("run")))
				}
				return view
			}),
		))).Commands(Command("run")))
	})
	menu := fake.windows[0].Widget().Children()[0].(*gui.MenuButton)
	outer := menu.Menu().ItemAt(0).Command()
	show.Set(true)
	fake.runPosted()
	inner := menu.Menu().ItemAt(0).Command()
	if inner == outer || !inner.Enabled() {
		t.Fatal("局部新增定义未遮蔽外层")
	}
	show.Set(false)
	fake.runPosted()
	if menu.Menu().ItemAt(0).Command() != outer || inner.Enabled() || global != 1 || local != 3 {
		t.Fatal("局部移除未回退，或重建了全局声明")
	}
}

func TestCommandDescendantTransferMovesDefinitionAndReferences(t *testing.T) {
	a, b := transferRoots()
	defer a.unmountWindow()
	defer b.unmountWindow()
	calls := 0
	source := a.update(CommandGroup(VBox(
		MenuButton("Source").Menu(MenuItem("Run").Command("run")),
		VBox(MenuButton("Moving").Menu(MenuItem("Run").DefineCommand(Command("run").OnActivate(func() { calls++ })))).ID("moving").Shortcuts(Shortcut(KeyF5).Command("run")),
	))).(*gui.LinearBox)
	target := b.update(CommandGroup(VBox(MenuButton("Target").Menu(MenuItem("Run").Command("run"))))).(*gui.LinearBox)
	moving := source.Children()[1]
	old := source.Children()[0].(*gui.MenuButton).Menu().ItemAt(0).Command()
	if err := (&Coordinator{owner: a.root.body}).TransferChild(moving, target, func() error {
		source.RemoveChild(moving)
		target.AddChild(moving)
		return nil
	}, nil); err != nil {
		t.Fatal(err)
	}
	next := target.Children()[0].(*gui.MenuButton).Menu().ItemAt(0).Command()
	if old.Enabled() || next == old || !next.Enabled() || len(old.Shortcuts()) != 0 || len(next.Shortcuts()) != 1 || moving.Children()[0].(*gui.MenuButton).Menu().ItemAt(0).Command() != next {
		t.Fatal("移交没有迁移定义、引用及快捷键关联")
	}
	next.Activate()
	if calls != 1 {
		t.Fatal("移交后没有使用原定义的业务回调")
	}
	// 源窗口销毁不应夺走已移交定义的所有权。
	a.unmountWindow()
	next.Activate()
	if calls != 2 || !next.Enabled() {
		t.Fatal("源窗口销毁影响了目标定义")
	}
	b.unmountWindow()
	if next.Enabled() {
		t.Fatal("目标卸载没有清理移交的操作")
	}
}

func TestCommandBatchedPublicationCanUnmountScope(t *testing.T) {
	r := newRoot()
	defer r.unmountWindow()
	build := func(enabled bool) View {
		return CommandGroup(MenuButton("Run").Menu(MenuItem("A").DefineCommand(Command("a").Enabled(enabled)), MenuItem("B").DefineCommand(Command("b").Enabled(enabled))))
	}
	r.update(build(true))
	s := r.root.state.(*commandGroup)
	for _, entry := range s.commands {
		entry.command.ConnectEnabled(func(bool) { r.unmountWindow() })
	}
	r.update(build(false))
	if r.root != nil {
		t.Fatal("命令状态通知没有完成卸载")
	}
}

func TestCommandBindEnabledFollowsTransferredDefinition(t *testing.T) {
	enabled := MakeState(true)
	rt, fake := bindingApp(t, func() RootView {
		return Root().Windows(
			Window("source").Content(CommandGroup(VBox(
				VBox(MenuButton("Moving").Menu(MenuItem("Run").DefineCommand(Command("run").BindEnabled(&enabled)))).ID("moving"),
			))),
			Window("target").Content(CommandGroup(VBox(MenuButton("Target").Menu(MenuItem("Run").Command("run"))))),
		)
	})
	a, b := rt.windows["source"].root, rt.windows["target"].root
	source, target := a.widget(), b.widget()
	moving := source.Children()[0]
	if err := (&Coordinator{owner: a.root.body}).TransferChild(moving, target, func() error {
		source.(*gui.LinearBox).RemoveChild(moving)
		target.(*gui.LinearBox).AddChild(moving)
		return nil
	}, nil); err != nil {
		t.Fatal(err)
	}
	command := target.Children()[0].(*gui.MenuButton).Menu().ItemAt(0).Command()
	fake.windows[0].Destroy()
	enabled.Set(false)
	fake.runPosted()
	if command.Enabled() || moving.Children()[0].(*gui.MenuButton).Menu().ItemAt(0).Command() != command {
		t.Fatal("移交后绑定仍写旧作用域或因源窗口销毁失效")
	}
	enabled.Set(true)
	fake.runPosted()
	if !command.Enabled() {
		t.Fatal("移交后的绑定无法恢复")
	}
}

func TestCommandGroupTransferRetainsPendingDefinitions(t *testing.T) {
	a, b := transferRoots()
	defer a.unmountWindow()
	defer b.unmountWindow()
	// 模拟初次构建已完成、作用域的批次完成钩子尚未执行即发生移交。
	a.reconcile(VBox(CommandGroup(VBox(
		MenuButton("Moving").Menu(MenuItem("Run").DefineCommand(Command("run"))),
	)).ID("scope")))
	source := a.widget().(*gui.LinearBox)
	target := b.update(VBox()).(*gui.LinearBox)
	moving := source.Children()[0]
	if err := (&Coordinator{owner: a.root}).TransferChild(moving, target, func() error {
		source.RemoveChild(moving)
		target.AddChild(moving)
		return nil
	}, nil); err != nil {
		t.Fatal(err)
	}
	a.flushAfterUpdate() // 旧 root 的回调失效，目标必须主动完成发布。
	command := moving.Children()[0].(*gui.MenuButton).Menu().ItemAt(0).Command()
	if !command.Enabled() {
		t.Fatal("作用域移交丢失待发布的定义")
	}
	b.flushAfterUpdate()
	if moving.Children()[0].(*gui.MenuButton).Menu().ItemAt(0).Command() != command {
		t.Fatal("目标后续完成钩子重复创建操作")
	}
}

// 操作是普通声明数据；菜单和快捷键均可提供唯一的定义，且不依赖打开菜单。
func TestCommandEntryDefinitionsShareOperation(t *testing.T) {
	if _, ok := any(Command("run")).(View); ok {
		t.Fatal("Command 不应作为普通 View 参与布局树")
	}
	for _, menuDefines := range []bool{false, true} {
		t.Run(map[bool]string{false: "shortcut", true: "menu"}[menuDefines], func(t *testing.T) {
			r := newRoot()
			defer r.unmountWindow()
			w := &overlayInputWindow{testWindow: newTestWindow()}
			calls := 0
			build := func(callback func(), menuEnabled, shortcutEnabled bool) View {
				definition := Command("run").OnActivate(callback)
				item := MenuItem("Run").Enabled(menuEnabled).Command("run")
				shortcut := Shortcut(KeyF5).Enabled(shortcutEnabled).Command("run")
				if menuDefines {
					item.DefineCommand(definition)
				} else {
					shortcut.DefineCommand(definition)
				}
				return CommandGroup(VBox(MenuButton("Menu").Menu(item), Label("editor")).Shortcuts(shortcut))
			}
			box := r.updateWindow(w, build(func() { calls++ }, false, true))
			button := box.Children()[0].(*gui.MenuButton)
			model := button.Menu()
			command := model.ItemAt(0).Command()
			press := func() {
				t.Helper()
				if err := w.DispatchEvent(events.KeyEvent{EventType: events.KeyDown, Key: events.KeyF5}); err != nil {
					t.Fatal(err)
				}
			}
			press()
			if command == nil || !command.Enabled() || calls != 1 || len(command.Shortcuts()) != 1 || model.ItemAt(0).Enabled() || button.Opened() {
				t.Fatal("定义未在菜单打开前注册，或入口局部禁用影响了共享命令")
			}
			r.updateWindow(w, build(func() { calls += 10 }, false, true))
			if button.Menu() != model || model.ItemAt(0).Command() != command {
				t.Fatal("最新定义重建了未变化的模型或操作")
			}
			press()
			if calls != 11 {
				t.Fatal("定义使用了过期回调或重复执行业务")
			}
			r.updateWindow(w, build(func() { calls += 100 }, true, false))
			press()
			if calls != 11 || !command.Enabled() || !button.Menu().ItemAt(0).Enabled() {
				t.Fatal("Shortcut 局部禁用影响了命令或菜单入口")
			}
		})
	}
}

func TestCommandDefinitionEntryModes(t *testing.T) {
	definition, external := Command("run"), gui.NewCommand()
	item := MenuItem("Run", func() {}).CommandRef(external).DefineCommand(definition)
	if item.definition != definition || item.commandID != "run" || item.commandRef != nil || item.action != nil {
		t.Fatal("菜单定义未替换旧来源")
	}
	item.Command("other")
	if item.definition != nil || item.commandID != "other" {
		t.Fatal("菜单 ID 未替换定义")
	}
	item.DefineCommand(definition).CommandRef(external)
	if item.definition != nil || item.commandRef != external {
		t.Fatal("菜单对象未替换定义")
	}
	item.DefineCommand(definition).DefineCommand(nil)
	if item.definition != nil || item.commandID != "" || item.commandRef != nil {
		t.Fatal("菜单空定义未清除操作")
	}
	shortcut := Shortcut(KeyF5).CommandRef(external).DefineCommand(definition)
	if shortcut.definition != definition || shortcut.commandID != "run" || shortcut.commandRef != nil || shortcut.onActivate != nil {
		t.Fatal("快捷键定义未替换旧来源")
	}
	shortcut.Command("other")
	if shortcut.definition != nil || shortcut.commandID != "other" {
		t.Fatal("快捷键 ID 未替换定义")
	}
	shortcut.DefineCommand(definition).CommandRef(external)
	if shortcut.definition != nil || shortcut.commandRef != external {
		t.Fatal("快捷键对象未替换定义")
	}
	shortcut.DefineCommand(definition).OnActivate(func() {})
	if shortcut.definition != nil || shortcut.commandID != "" || shortcut.onActivate == nil {
		t.Fatal("快捷键回调未替换定义")
	}
	shortcut.DefineCommand(definition).DefineCommand(nil)
	if shortcut.definition != nil || shortcut.commandID != "" || shortcut.onActivate != nil {
		t.Fatal("快捷键空定义未清除操作")
	}
}

func TestCommandDefinitionHostErrors(t *testing.T) {
	for _, tc := range []struct {
		name string
		run  func()
		want string
	}{
		{"temporary-menu", func() { Menu(MenuItem("Run").DefineCommand(Command("run"))) }, "temporary menus must reference a command"},
		{"temporary-context-menu", func() { MenuFor(nil, MenuItem("Run").DefineCommand(Command("run"))) }, "temporary menus must reference a command"},
		{"window-shortcut", func() {
			mount := &windowMount{window: newTestWindow()}
			defer mount.disconnect()
			_ = mount.applyWindowProperties(Window("main").Shortcuts(Shortcut(KeyF5).DefineCommand(Command("run"))))
		}, "Window.Shortcuts cannot define scoped commands"},
		{"shortcut-outside-scope", func() {
			r := newRoot()
			defer r.unmountWindow()
			r.update(Label("editor").Shortcuts(Shortcut(KeyF5).DefineCommand(Command("run"))))
		}, "DefineCommand requires an ancestor CommandGroup"},
		{"duplicate-menu-definitions", func() {
			r := newRoot()
			defer r.unmountWindow()
			r.update(CommandGroup(MenuButton("Menu").Menu(MenuItem("A").DefineCommand(Command("run")), MenuItem("B").DefineCommand(Command("run")))))
		}, `duplicate or empty command ID "run"`},
		{"duplicate-shortcut-definitions", func() {
			r := newRoot()
			defer r.unmountWindow()
			r.update(CommandGroup(Label("editor").Shortcuts(Shortcut(KeyF5).DefineCommand(Command("run")), Shortcut(KeyF6).DefineCommand(Command("run")))))
		}, `duplicate or empty command ID "run"`},
		{"duplicate-across-hosts", func() {
			r := newRoot()
			defer r.unmountWindow()
			r.update(CommandGroup(MenuButton("Menu").Menu(MenuItem("Run").DefineCommand(Command("run"))).Shortcuts(Shortcut(KeyF5).DefineCommand(Command("run")))))
		}, `duplicate command ID "run"`},
	} {
		t.Run(tc.name, func(t *testing.T) {
			defer func() {
				if message, ok := recover().(string); !ok || !strings.Contains(message, tc.want) {
					t.Fatalf("错误为 %q，期望包含 %q", message, tc.want)
				}
			}()
			tc.run()
		})
	}
}

// 祖先对象查询允许用户代码销毁当前树，定义登记不能在释放后继续。
func TestCommandDefinitionLookupCanReleaseHost(t *testing.T) {
	for _, host := range []string{"menu", "shortcut"} {
		t.Run(host, func(t *testing.T) {
			r := newRoot()
			defer r.unmountWindow()
			armed := false
			build := func() View {
				item := MenuItem("Run").Command("run")
				view := MenuButton("Menu")
				if host == "menu" {
					item.DefineCommand(Command("run"))
				} else {
					view.Shortcuts(Shortcut(KeyF5).DefineCommand(Command("run")))
				}
				view.Menu(item)
				interceptor := objectScope("interceptor", view, func() any { return new(int) })
				interceptor.read = func(BuildContext) any {
					if armed {
						armed = false
						r.unmountWindow()
					}
					return new(int)
				}
				return CommandGroup(interceptor)
			}
			r.update(build())
			armed = true
			r.update(build())
			if r.root != nil {
				t.Fatal("祖先查询没有安全释放声明宿主")
			}
		})
	}
}

func TestShortcutDefinitionBindingFollowsTransfer(t *testing.T) {
	enabled := MakeState(true)
	rt, fake := bindingApp(t, func() RootView {
		return Root().Windows(
			Window("source").Content(CommandGroup(VBox(
				VBox(MenuButton("Moving").Menu(MenuItem("Run").Command("run"))).ID("moving").Shortcuts(Shortcut(KeyF5).DefineCommand(Command("run").BindEnabled(&enabled))),
			))),
			Window("target").Content(CommandGroup(VBox(MenuButton("Target").Menu(MenuItem("Run").Command("run"))))),
		)
	})
	a, b := rt.windows["source"].root, rt.windows["target"].root
	source, target := a.widget().(*gui.LinearBox), b.widget().(*gui.LinearBox)
	moving := source.Children()[0]
	old := moving.Children()[0].(*gui.MenuButton).Menu().ItemAt(0).Command()
	if err := (&Coordinator{owner: a.root.body}).TransferChild(moving, target, func() error { source.RemoveChild(moving); target.AddChild(moving); return nil }, nil); err != nil {
		t.Fatal(err)
	}
	command := target.Children()[0].(*gui.MenuButton).Menu().ItemAt(0).Command()
	if command == old || old.Enabled() || len(command.Shortcuts()) != 1 {
		t.Fatal("快捷键定义没有重新归属")
	}
	fake.windows[0].Destroy()
	enabled.Set(false)
	fake.runPosted()
	if command.Enabled() {
		t.Fatal("快捷键定义的绑定仍停留在源窗口")
	}
	enabled.Set(true)
	fake.runPosted()
	if !command.Enabled() {
		t.Fatal("快捷键定义的绑定未在目标恢复")
	}
}
