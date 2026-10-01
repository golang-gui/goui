package widgets

import (
	"runtime"

	"github.com/golang-gui/goui/core/geometry"
	"github.com/golang-gui/goui/core/signal"
	"github.com/golang-gui/goui/gui"
	"github.com/golang-gui/goui/platform/events"
)

// ConnectContextMenu queries this bar's menu for page synchronously on the GUI
// thread. The result starts nil; handlers run in connection order and may replace
// or clear it. A nil or empty model means no menu. Do not retain the result pointer.
// Models may be newly created or reused. Actions should capture page, not Current.
// Each bar has its own handlers and popup, even when bars share the same TabView.
func (b *TabBar) ConnectContextMenu(fn func(page *TabPage, menu *gui.MenuModel)) signal.Handle {
	return b.contextMenuQuery.Connect(func(page *TabPage, menu *gui.MenuModel, generation uint64) {
		if generation == b.menuGeneration && b.validMenuPage(page) {
			fn(page, menu)
		}
	})
}

func (b *TabBar) validMenuPage(page *TabPage) bool {
	return !b.Destroyed() && b.view != nil && b.view.validPage(page) && b.items[page] != nil
}

func (b *TabBar) menuForPage(page *TabPage) gui.MenuModel {
	if !b.validMenuPage(page) {
		return nil
	}
	generation := b.menuGeneration
	var menu gui.MenuModel
	b.contextMenuQuery.Emit(page, &menu, generation)
	if generation != b.menuGeneration || !b.validMenuPage(page) {
		return nil
	}
	return menu
}

// ConnectContextMenuError reports a failed input-triggered menu presentation.
// No menu or dismissal is not an error. Failures are not additionally logged.
func (b *TabBar) ConnectContextMenuError(fn func(error)) signal.Handle {
	return b.contextMenuError.Connect(fn)
}

func (b *TabBar) closeContextMenu() {
	b.menuGeneration++ // invalidate queries interrupted by unmount or rebinding
	b.menuPage = nil
	if b.contextMenu != nil {
		b.contextMenu.SetMenu(nil)
	}
}

func (b *TabBar) showContextMenu(page *TabPage, position geometry.Point) {
	view := b.view
	if b.Destroyed() || view == nil || !view.validPage(page) || b.items[page] == nil ||
		b.dragPage != nil || b.nativeDrag != nil || b.pendingDetach != nil {
		return
	}
	b.closeContextMenu()
	generation := b.menuGeneration
	menu := b.menuForPage(page)
	if generation != b.menuGeneration || b.Destroyed() || b.view != view ||
		!view.validPage(page) || b.items[page] == nil || menu == nil || menu.ItemsCount() == 0 {
		return
	}
	if b.contextMenu == nil {
		popup := gui.NewPopoverMenu(b)
		b.contextMenu = popup
		popup.ConnectClosed(func() {
			// Drop target-capturing actions as soon as the popup is dismissed.
			b.closeContextMenu()
		})
	}
	b.menuPage = page
	b.contextMenu.SetMenu(menu)
	if err := b.contextMenu.ShowAt(position); err != nil {
		b.closeContextMenu()
		b.contextMenuError.Emit(err)
	}
}

func (item *tabItem) contextMenuPosition(point geometry.Point) geometry.Point {
	return point.Add(item.Rect().Pos).Add(item.bar.viewport.Rect().Pos)
}

// Keep pointer handling on actual tabs, not on the bar/viewport: their empty
// areas must remain passive HeaderBar caption regions. Capture also prevents
// macOS Control-click from reaching the primary click, close and drag handlers.
type tabContextMenuController struct {
	gui.EventControllerBase
	item *tabItem
}

func (c *tabContextMenuController) HandleEvent(ctx gui.EventContext) {
	event, ok := ctx.Event().(events.PointerEvent)
	if !ok || event.EventType != events.PointerDown || !tabContextMenuPointer(event, runtime.GOOS) {
		return
	}
	point, ok := ctx.Position()
	if !ok {
		return
	}
	ctx.StopPropagation()
	c.item.bar.showContextMenu(c.item.page, c.item.contextMenuPosition(point))
}

func tabContextMenuPointer(event events.PointerEvent, goos string) bool {
	return event.Button == events.PointerButtonRight || goos == "darwin" &&
		event.Button == events.PointerButtonLeft && event.Modifiers&events.ModifierControl != 0
}
