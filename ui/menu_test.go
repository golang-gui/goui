package ui

import (
	"testing"

	"github.com/golang-gui/goui/gui"
)

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
