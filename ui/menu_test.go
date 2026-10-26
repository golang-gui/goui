package ui

import (
	"errors"
	"testing"

	"github.com/golang-gui/goui/gui"
)

func TestMenuConvertsDeclarationsWithoutExecutingActions(t *testing.T) {
	called := 0
	item := MenuItem("Open", func() { called++ }).Enabled(false)
	menu := Menu(nil, item, MenuSeparator())
	if menu.ItemsCount() != 2 || menu.ItemAt(0).Label() != "Open" || menu.ItemAt(0).Enabled() || !menu.ItemAt(1).Separator() || called != 0 {
		t.Fatal("menu conversion lost metadata or executed an action")
	}
	item.Enabled(true)
	if menu.ItemAt(0).Enabled() || Menu(item).ItemAt(0) == menu.ItemAt(0) {
		t.Fatal("conversion did not copy declaration values into independent items")
	}
	menu.ItemAt(0).Action()()
	if called != 1 || Menu().ItemsCount() != 0 {
		t.Fatal("action was lost or empty declaration produced items")
	}
}

func TestMenuButtonMountsAndBuildsMenu(t *testing.T) {
	root := newRoot()

	widget := root.update(MenuButton("File").Menu(
		MenuItem("Open", func() {}),
		MenuItem("Close", func() {}).Enabled(false),
		MenuSeparator(),
		MenuItem("Quit", func() {}),
	)).(*gui.MenuButton)

	model := widget.Menu()
	if model == nil {
		t.Fatal("MenuButton should have a menu model")
	}
	if got := model.ItemsCount(); got != 4 {
		t.Fatalf("menu item count = %d, want 4", got)
	}
	if it := model.ItemAt(0); it.Label() != "Open" || !it.Enabled() {
		t.Fatalf("item 0 unexpected: label=%q enabled=%v", it.Label(), it.Enabled())
	}
	if it := model.ItemAt(1); it.Label() != "Close" || it.Enabled() {
		t.Fatalf("item 1 unexpected: label=%q enabled=%v", it.Label(), it.Enabled())
	}
	if it := model.ItemAt(2); !it.Separator() {
		t.Fatal("item 2 should be a separator")
	}
}

func TestMenuButtonUpdatesChildLabel(t *testing.T) {
	root := newRoot()
	button := root.update(MenuButton("File")).(*gui.MenuButton)

	children := button.Children()
	if len(children) != 1 || children[0].(*gui.Label).Text() != "File" {
		t.Fatalf("unexpected button child: %v", children)
	}
	child := children[0]

	root.update(MenuButton("Edit"))
	children = button.Children()
	if len(children) != 1 || children[0] != child || children[0].(*gui.Label).Text() != "Edit" {
		t.Fatalf("button child was not updated in place: %v", children)
	}
}

func TestMenuButtonCustomChild(t *testing.T) {
	root := newRoot()
	widget := root.update(MenuButton().Child(Label("icon")).Menu(
		MenuItem("Open", func() {}),
	)).(*gui.MenuButton)

	children := widget.Children()
	if len(children) != 1 || children[0].(*gui.Label).Text() != "icon" {
		t.Fatalf("custom child not applied: %v", children)
	}
}

func TestMenuButtonFingerprintSkipsUnchangedMenu(t *testing.T) {
	root := newRoot()
	first := root.update(MenuButton("File").Menu(
		MenuItem("Open", func() {}),
		MenuItem("Close", func() {}),
	)).(*gui.MenuButton)
	firstModel := first.Menu()
	if firstModel == nil {
		t.Fatal("expected a menu model")
	}

	// Same items (rebuilt descriptors) must not rebuild the model (fingerprint
	// unchanged), leaving the existing model pointer intact.
	root.update(MenuButton("File").Menu(
		MenuItem("Open", func() {}),
		MenuItem("Close", func() {}),
	))
	if got := first.Menu(); got != firstModel {
		t.Fatal("unchanged menu should not be rebuilt")
	}

	// Changing an item rebuilds the model.
	root.update(MenuButton("File").Menu(
		MenuItem("Open", func() {}),
		MenuItem("Close", func() {}).Enabled(false),
	))
	if got := first.Menu(); got == firstModel {
		t.Fatal("changed menu should rebuild the model")
	}
}

func TestMenuButtonEmptyMenuClearsModel(t *testing.T) {
	root := newRoot()
	button := root.update(MenuButton("File").Menu(MenuItem("a", func() {}))).(*gui.MenuButton)
	if button.Menu() == nil {
		t.Fatal("expected a menu model")
	}
	root.update(MenuButton("File"))
	if got := button.Menu(); got != nil && got.ItemsCount() != 0 {
		t.Fatalf("expected cleared/empty menu, got %d items", got.ItemsCount())
	}
}

func TestMenuButtonResetMenuFromEmpty(t *testing.T) {
	root := newRoot()
	button := root.update(MenuButton("File")).(*gui.MenuButton)
	if button.Menu() != nil {
		t.Fatal("empty button should have no menu")
	}
	root.update(MenuButton("File").Menu(MenuItem("Open", func() {})))
	if got := button.Menu(); got == nil || got.ItemsCount() != 1 {
		t.Fatalf("expected a rebuilt menu, got %v", got)
	}
}

// 显式组声明没有视觉包装；同名分支、重建与移除各自保持明确归属。
func TestMenuButtonGroupDeclarationsAndLatestActions(t *testing.T) {
	r := newRoot()
	t.Cleanup(r.unmountWindow)
	generation, got := 1, 0
	build := func() View {
		captured := generation
		return VBox(
			MenuButtonGroup("menus", HBox(
				MenuButton("File").ID("file").Group("menus").Menu(MenuItem("Open", func() { got = captured })),
				MenuButton("Edit").ID("edit").Group("menus").Menu(MenuItem("Undo", nil)),
				Button("Run"),
			)),
			VBox(MenuButtonGroup("menus", MenuButton("View").Group("menus").Menu(MenuItem("Zoom", nil)))).ID("second"),
		)
	}
	root := r.update(build()).(*gui.LinearBox)
	box := root.Children()[0].(*gui.LinearBox)
	file, edit := box.Children()[0].(*gui.MenuButton), box.Children()[1].(*gui.MenuButton)
	other := root.Children()[1].(*gui.LinearBox).Children()[0].(*gui.MenuButton)
	group := file.Group()
	model := file.Menu()
	if group == nil || edit.Group() != group || other.Group() == group || box.ID() != "" || len(box.Children()) != 3 {
		t.Fatal("组声明改变了实际树或混淆同名分支")
	}
	generation = 2
	r.update(build())
	if root.Children()[0] != box || box.Children()[0] != file || file.Group() != group || file.Menu() != model {
		t.Fatal("无关重建替换了 Widget、组或菜单模型")
	}
	model.ItemAt(0).Action()()
	if got != 2 {
		t.Fatal("稳定菜单模型仍使用旧动作闭包", got)
	}
	r.update(MenuButtonGroup("menus", MenuButton("File").Group("menus").Menu(MenuItem("Open", nil))))
	if file.Group() != nil || edit.Group() != nil || other.Group() != nil {
		t.Fatal("移除声明后成员仍关联旧组")
	}
	got = 0
	model.ItemAt(0).Action()()
	if got != 0 {
		t.Fatal("卸载后旧模型仍持有声明动作")
	}
}

func TestMenuButtonGroupRequiresExplicitCompatibleAncestor(t *testing.T) {
	for _, view := range []View{
		MenuButton("Missing").Group("menus"),
		MenuButtonGroup("menus", MenuButton("Wrong").Group("other")),
		objectScope("menus", MenuButton("Wrong type").Group("menus"), func() any { return new(int) }),
	} {
		func() {
			r := newRoot()
			defer r.unmountWindow()
			defer func() {
				if recover() == nil {
					t.Fatal("接受了不存在或类型错误的显式组")
				}
			}()
			r.update(view)
		}()
	}
}

func TestMenuButtonGroupTransferUpdatesAncestorWithoutReplacingState(t *testing.T) {
	for _, fail := range []bool{false, true} {
		t.Run(map[bool]string{false: "成功", true: "回滚"}[fail], func(t *testing.T) {
			a, b := transferRoots()
			t.Cleanup(a.unmountWindow)
			t.Cleanup(b.unmountWindow)
			generation, called := 1, 0
			declaration := func() *MenuButtonView {
				captured := generation
				return MenuButton("Moving").ID("moving").Group("menus").Menu(
					MenuItem("Action", func() { called = captured }),
				)
			}
			source := a.update(MenuButtonGroup("menus", VBox(declaration()))).(*gui.LinearBox)
			target := b.update(MenuButtonGroup("menus", VBox(
				MenuButton("Resident").ID("resident").Group("menus").Menu(MenuItem("Other", nil)),
			))).(*gui.LinearBox)
			moving := source.Children()[0].(*gui.MenuButton)
			old, destination := moving.Group(), target.Children()[0].(*gui.MenuButton).Group()
			model := moving.Menu()
			failure := errors.New("移交失败")
			err := (&Coordinator{owner: a.root.body}).TransferChild(moving, target, func() error {
				if fail {
					return failure
				}
				source.RemoveChild(moving)
				target.AddChild(moving)
				return nil
			}, func() {
				if moving.Group() != destination || moving.Menu() != model {
					t.Fatal("移交完成前没有刷新祖先关联，或替换了菜单模型")
				}
			})
			if fail {
				if err != failure || moving.Group() != old || moving.Menu() != model {
					t.Fatal("失败回滚改变了组或模型", err)
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			generation = 2
			b.update(MenuButtonGroup("menus", VBox(
				MenuButton("Resident").ID("resident").Group("menus").Menu(MenuItem("Other", nil)),
				declaration(),
			)))
			a.unmountWindow()
			if moving.Group() != destination || moving.Menu() != model || target.Children()[1] != moving {
				t.Fatal("移交后重建或源 Root 卸载改变了组、模型或 Widget")
			}
			model.ItemAt(0).Action()()
			if called != 2 {
				t.Fatal("移交后仍执行源声明闭包", called)
			}
			b.unmountWindow()
			if moving.Group() != nil {
				t.Fatal("目标释放后残留组关联")
			}
		})
	}
}

func TestMenuButtonGroupDeclarationTransfersWithContent(t *testing.T) {
	a, b := transferRoots()
	t.Cleanup(a.unmountWindow)
	t.Cleanup(b.unmountWindow)
	source := a.update(VBox(MenuButtonGroup("menus", HBox(
		MenuButton("File").Group("menus").Menu(MenuItem("Open", nil)),
		MenuButton("Edit").Group("menus").Menu(MenuItem("Undo", nil)),
	)))).(*gui.LinearBox)
	target := b.update(VBox()).(*gui.LinearBox)
	content := source.Children()[0]
	file, edit := content.Children()[0].(*gui.MenuButton), content.Children()[1].(*gui.MenuButton)
	group := file.Group()
	if err := (&Coordinator{owner: a.root}).TransferChild(content, target, func() error {
		source.RemoveChild(content)
		target.AddChild(content)
		return nil
	}, nil); err != nil {
		t.Fatal(err)
	}
	a.unmountWindow()
	if file.Group() != group || edit.Group() != group || target.Children()[0] != content {
		t.Fatal("完整对象声明移交后组实例被重新创建或释放")
	}
	b.unmountWindow()
	if file.Group() != nil || edit.Group() != nil {
		t.Fatal("完整组声明未在目标 Root 释放")
	}
}
