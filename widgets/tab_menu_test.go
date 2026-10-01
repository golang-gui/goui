package widgets

import (
	"slices"
	"testing"

	"github.com/golang-gui/goui/core/geometry"
	"github.com/golang-gui/goui/gui"
	"github.com/golang-gui/goui/platform/events"
)

func TestTabContextMenuQueriesTargetInOrderAndMayReuseModel(t *testing.T) {
	view := NewTabView()
	a, b := NewTabPage("A", nil), NewTabPage("B", nil)
	view.AppendPage(a)
	view.AppendPage(b)
	bar := NewTabBar()
	bar.SetView(view)
	menu := gui.NewMenu()
	menu.Append("Close B", func() { view.RequestClose(b) })
	var order []int
	first := bar.ConnectContextMenu(func(page *TabPage, result *gui.MenuModel) {
		if page != b || *result != nil {
			t.Fatal("query did not start with the requested inactive page and nil model")
		}
		order = append(order, 1)
		*result = menu
	})
	second := bar.ConnectContextMenu(func(page *TabPage, result *gui.MenuModel) {
		if page != b || *result != menu {
			t.Fatal("later handler did not receive previous result")
		}
		order = append(order, 2)
		*result = nil
	})
	if bar.menuForPage(b) != nil || !slices.Equal(order, []int{1, 2}) || view.Current() != a {
		t.Fatal("query order, cancellation or selection changed")
	}
	second.Disconnect()
	if bar.menuForPage(b) != menu || bar.menuForPage(b) != menu {
		t.Fatal("a shared menu model could not be reused")
	}
	first.Disconnect()
	if bar.menuForPage(b) != nil {
		t.Fatal("disconnected handler remained active")
	}
}

func TestTabContextMenuBarsSharingViewHaveIndependentQueries(t *testing.T) {
	bar, view, pages := dragTestBar()
	other := NewTabBar()
	other.SetView(view)
	first, second := gui.NewMenu(), gui.NewMenu()
	first.Append("first", nil)
	second.Append("second", nil)
	menus := []gui.MenuModel{first, second}
	calls := [2]int{}
	bar.ConnectContextMenu(func(page *TabPage, result *gui.MenuModel) {
		if page != pages[1] {
			t.Fatal("first bar queried the wrong page")
		}
		calls[0]++
		*result = menus[0]
	})
	other.ConnectContextMenu(func(page *TabPage, result *gui.MenuModel) {
		if page != pages[1] {
			t.Fatal("second bar queried the wrong page")
		}
		calls[1]++
		*result = menus[1]
	})
	if bar.menuForPage(pages[1]) != menus[0] || calls != [2]int{1, 0} {
		t.Fatal("first bar invoked or used another bar's provider")
	}
	if other.menuForPage(pages[1]) != menus[1] || calls != [2]int{1, 1} || view.Current() != pages[0] {
		t.Fatal("second bar shared another bar's menu or changed selection")
	}
	bar.SetView(nil)
	if bar.menuForPage(pages[1]) != nil || other.menuForPage(pages[1]) != menus[1] || calls != [2]int{1, 2} {
		t.Fatal("unbinding one bar invalidated another bar's provider")
	}
}

func TestTabContextMenuAndCloseRejectInvalidatedPages(t *testing.T) {
	for _, change := range []string{"remove", "transfer"} {
		t.Run(change, func(t *testing.T) {
			view, target := NewTabView(), NewTabView()
			page := NewTabPage("page", nil)
			page.SetClosable(true)
			view.AppendPage(page)
			bar := NewTabBar()
			bar.SetView(view)
			owner := gui.NewPopover(nil, nil)
			owner.SetWidget(view)
			defer owner.Destroy()
			invalidate := func() {
				switch change {
				case "remove":
					view.RemovePage(page)
				case "transfer":
					if err := view.TransferPage(page, target, 0); err != nil {
						t.Fatal(err)
					}
				}
			}
			later := 0
			bar.ConnectContextMenu(func(_ *TabPage, result *gui.MenuModel) {
				menu := gui.NewMenu()
				menu.Append("unused", nil)
				*result = menu
				invalidate()
			})
			bar.ConnectContextMenu(func(*TabPage, *gui.MenuModel) { later++ })
			if bar.menuForPage(page) != nil || later != 0 {
				t.Fatal("query returned a stale menu or invoked a handler after invalidation")
			}
			view.ConnectCloseRequest(func(*TabPage) { later++ })
			view.RequestClose(page)
			if later != 0 {
				t.Fatal("stale page sent a close request")
			}
		})
	}
}

func TestTabCloseRequestCallbacksCannotUseRemovedPage(t *testing.T) {
	view := NewTabView()
	page := NewTabPage("page", nil)
	page.SetClosable(true)
	view.AppendPage(page)
	view.ConnectCloseRequest(view.RemovePage)
	later := 0
	view.ConnectCloseRequest(func(*TabPage) { later++ })
	view.RequestClose(page)
	if later != 0 || len(view.Pages()) != 0 {
		t.Fatal("later handler used removed page")
	}
}

func TestTabContextMenuPointerUsesActualTabNotCurrentOrBlank(t *testing.T) {
	bar, view, pages := dragTestBar()
	var queries []*TabPage
	bar.ConnectContextMenu(func(page *TabPage, _ *gui.MenuModel) { queries = append(queries, page) })
	closed := 0
	view.ConnectCloseRequest(func(*TabPage) { closed++ })
	d := new(gui.EventDispatcher)
	host := &tabInputHost{root: bar}
	for _, x := range []float32{130, 102, 350, 85} {
		if err := d.DispatchEvent(host, events.PointerEvent{
			EventType: events.PointerDown, Button: events.PointerButtonRight,
			Position: geometry.Point{X: x, Y: 20}, Buttons: events.PointerButtonRightDown,
		}); err != nil {
			t.Fatal(err)
		}
		if err := d.DispatchEvent(host, events.PointerEvent{
			EventType: events.PointerUp, Button: events.PointerButtonRight,
			Position: geometry.Point{X: x, Y: 20},
		}); err != nil {
			t.Fatal(err)
		}
	}
	// B body, gaps/blank ignored, then A's visible close button.
	if !slices.Equal(queries, []*TabPage{pages[1], pages[0]}) || view.Current() != pages[0] ||
		closed != 0 || bar.dragPage != nil || bar.contextMenu != nil {
		t.Fatal("secondary click selected/closed/dragged a page or consumed blank caption space")
	}
	if info := bar.Snapshot(); len(info.Children) != 3 || info.Children[1].Text != "B" || info.Children[1].Selected {
		t.Fatal("context query changed tab snapshot")
	}
}

func TestTabContextMenuPresentationFailureAndRebinding(t *testing.T) {
	bar, _, pages := dragTestBar()
	menu := gui.NewMenu()
	menu.Append("action", nil)
	bar.ConnectContextMenu(func(_ *TabPage, result *gui.MenuModel) { *result = menu })
	errors := 0
	bar.ConnectContextMenuError(func(err error) {
		if err == nil {
			t.Fatal("nil presentation error")
		}
		errors++
	})
	// An unmounted anchor cannot create a native popup; no desktop is involved.
	bar.showContextMenu(pages[1], geometry.Point{X: 130, Y: 20})
	if errors != 1 || bar.menuPage != nil || bar.contextMenu.Visible() || bar.contextMenu.Menu() != nil {
		t.Fatal("presentation failure retained page/model or was not reported once")
	}
	bar.ConnectContextMenu(func(*TabPage, *gui.MenuModel) { bar.SetView(nil) })
	later := 0
	bar.ConnectContextMenu(func(*TabPage, *gui.MenuModel) { later++ })
	bar.showContextMenu(pages[1], geometry.Point{X: 130, Y: 20})
	if errors != 1 || bar.View() != nil || bar.menuPage != nil || later != 0 {
		t.Fatal("query continued after callback unbound its bar")
	}
}

func TestTabContextMenuEmptyModelAndDragDoNotCreatePopup(t *testing.T) {
	bar, _, pages := dragTestBar()
	calls := 0
	bar.ConnectContextMenu(func(_ *TabPage, result *gui.MenuModel) {
		calls++
		*result = gui.NewMenu()
	})
	bar.showContextMenu(pages[0], geometry.Point{})
	bar.beginDrag(pages[0], geometry.Point{X: 20, Y: 20})
	bar.showContextMenu(pages[0], geometry.Point{})
	if calls != 1 || bar.contextMenu != nil {
		t.Fatal("empty model created popup or active drag started a query")
	}
}

func TestTabContextMenuNestedQuerySupersedesOuterQuery(t *testing.T) {
	bar, _, pages := dragTestBar()
	queries, failures := 0, 0
	bar.ConnectContextMenu(func(page *TabPage, result *gui.MenuModel) {
		queries++
		menu := gui.NewMenu()
		menu.Append(page.Title(), nil)
		*result = menu
		if page == pages[0] {
			bar.showContextMenu(pages[1], geometry.Point{})
		}
	})
	bar.ConnectContextMenuError(func(error) { failures++ })
	bar.showContextMenu(pages[0], geometry.Point{})
	if queries != 2 || failures != 1 || bar.menuPage != nil || bar.contextMenu.Menu() != nil {
		t.Fatal("outer query replaced or presented after the nested query")
	}
}

func TestTabContextMenuCoordinatesAndKeyboardTarget(t *testing.T) {
	bar, view, pages := dragTestBar()
	bar.viewport.Arrange(geometry.Rect(24, 0, 200, 40))
	item := bar.items[pages[1]]
	item.Arrange(geometry.Rect(-30, 0, 100, 40))
	if got := item.contextMenuPosition(geometry.Point{X: 16, Y: 12}); got != (geometry.Point{X: 10, Y: 12}) {
		t.Fatalf("scrolled tab-local point not converted to bar DIP: %v", got)
	}
	owner := gui.NewPopover(nil, nil)
	owner.SetWidget(bar)
	defer owner.Destroy()
	host := owner.(gui.EventTarget)
	host.SetFocusedWidget(item.body)
	var requested *TabPage
	bar.ConnectContextMenu(func(page *TabPage, _ *gui.MenuModel) { requested = page })
	d := new(gui.EventDispatcher)
	if err := d.DispatchEvent(host, events.FocusEvent{Focused: true}); err != nil {
		t.Fatal(err)
	}
	if err := d.DispatchEvent(host, events.KeyEvent{EventType: events.KeyDown, Key: events.KeyF10, Modifiers: events.ModifierShift}); err != nil {
		t.Fatal(err)
	}
	if requested != pages[1] || view.Current() != pages[0] {
		t.Fatal("keyboard menu used selection instead of focused tab")
	}
}

func TestTabContextMenuQueryCannotOutliveBarMount(t *testing.T) {
	bar, _, pages := dragTestBar()
	owner := gui.NewPopover(nil, nil)
	owner.SetWidget(bar)
	defer owner.Destroy()
	bar.ConnectContextMenu(func(_ *TabPage, result *gui.MenuModel) {
		menu := gui.NewMenu()
		menu.Append("unused", nil)
		*result = menu
		owner.SetWidget(nil)
	})
	later := 0
	bar.ConnectContextMenu(func(*TabPage, *gui.MenuModel) { later++ })
	failures := 0
	bar.ConnectContextMenuError(func(error) { failures++ })
	bar.showContextMenu(pages[0], geometry.Point{})
	if bar.Root() != nil || bar.contextMenu != nil || failures != 0 || later != 0 {
		t.Fatal("unmounted bar continued the old query")
	}
}

func TestTabContextMenuPointerPlatformSemantics(t *testing.T) {
	for _, goos := range []string{"linux", "windows", "darwin"} {
		if !tabContextMenuPointer(events.PointerEvent{Button: events.PointerButtonRight}, goos) {
			t.Fatal("secondary click not recognized")
		}
		if got := tabContextMenuPointer(events.PointerEvent{Button: events.PointerButtonLeft, Modifiers: events.ModifierControl}, goos); got != (goos == "darwin") {
			t.Fatalf("Control-click incorrectly interpreted on %s", goos)
		}
		if tabContextMenuPointer(events.PointerEvent{Button: events.PointerButtonLeft}, goos) {
			t.Fatal("primary click interpreted as context menu")
		}
	}
}
