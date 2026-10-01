package widgets

import (
	"math"
	"slices"
	"time"

	"github.com/golang-gui/goui/core/geometry"
	"github.com/golang-gui/goui/core/signal"
	"github.com/golang-gui/goui/gui"
	"github.com/golang-gui/goui/layout"
	"github.com/golang-gui/goui/platform/events"
	"github.com/golang-gui/goui/platform/graphics"
	"github.com/golang-gui/goui/style"
)

const tabButtonWidth float32 = 24
const tabDragEdge float32 = 24
const tabSpacing float32 = 4
const defaultMinTabWidth float32 = 112
const defaultMaxTabWidth float32 = 240

// TabBar presents a TabView without owning its pages. Its viewport and scroll
// position are independent, so several bars may present the same view.
type TabBar struct {
	gui.WidgetBase
	view                                *TabView
	connections                         signal.Handles
	items                               map[*TabPage]*tabItem
	viewport                            *tabViewport
	previous, next                      *gui.Button
	reorderable                         bool
	minTabWidth, maxTabWidth            float32
	tabWidth                            float32
	overflow                            bool
	scroll, contentWidth, viewportWidth float32
	reveal                              bool
	lastCurrent                         *TabPage
	dragPage                            *TabPage
	lifted                              *TabPage // drawn last through drag and settling
	dragOrder                           []*TabPage
	insertAt                            int
	pointer                             geometry.Point // bar-local, independent of moving children
	grabX                               float32
	grabY                               float32
	timer                               *gui.Timer
	timerHandle                         signal.Handle
	lastTick                            time.Time
	transferable                        bool
	dropTarget                          *gui.DropTarget
	nativeDrag, incoming                *tabTransferDrag
	transferError                       signal.Signal1[error]
	detachRequest                       signal.Signal1[*TabDetachRequest]
	pendingDetach                       *TabDetachRequest
	incomingAt                          int
	previewMotion                       bool // retain animation while a vacated gap closes
	contextMenu                         *gui.PopoverMenu
	contextMenuQuery                    signal.Signal3[*TabPage, *gui.MenuModel, uint64]
	menuPage                            *TabPage
	menuGeneration                      uint64
	contextMenuError                    signal.Signal1[error]
}

func NewTabBar() *TabBar {
	b := &TabBar{items: make(map[*TabPage]*tabItem), minTabWidth: defaultMinTabWidth, maxTabWidth: defaultMaxTabWidth}
	b.viewport = &tabViewport{bar: b}
	b.viewport.SetLayoutManager(&tabStripLayout{bar: b})
	b.previous = tabArrow(-1, b)
	b.next = tabArrow(1, b)
	b.WidgetBase.AddChild(b, b.previous)
	b.WidgetBase.AddChild(b, b.viewport)
	b.WidgetBase.AddChild(b, b.next)
	b.SetLayoutManager(&tabBarLayout{bar: b})
	b.ConnectUnmount(func() {
		b.closeContextMenu()
		b.cancelDetach()
		if b.nativeDrag != nil {
			b.nativeDrag.source.Cancel()
		}
		b.clearIncoming()
		b.stopDrag()
		b.disconnect()
		if b.timerHandle != nil {
			b.timerHandle.Disconnect()
			b.timerHandle = nil
		}
		b.timer = nil
	})
	b.ConnectMount(func() { b.connect() })
	key := gui.NewKeyEventController()
	key.SetPhase(gui.PhaseBubble)
	key.ConnectKeyDown(b.onKey)
	b.AddEventController(key)
	return b
}

// Only actual tabs and buttons consume wheel input. The bar and viewport stay
// passive so HeaderBar can classify their unoccupied area as Caption.
func (b *TabBar) addWheel(widget gui.Widget) {
	wheel := gui.NewWheelEventController()
	wheel.ConnectScroll(func(_ gui.EventContext, e events.WheelEvent) {
		delta := e.DeltaX
		if delta == 0 {
			delta = e.DeltaY
		}
		if e.Mode == events.WheelDeltaLine {
			delta *= 32
		}
		b.setScroll(b.scroll + delta)
	})
	widget.AddEventController(wheel)
}

func tabArrow(direction int, bar *TabBar) *gui.Button {
	button := gui.NewButton()
	button.SetPadding(0)
	button.SetMinSize(geometry.Size{Width: tabButtonWidth, Height: tabButtonWidth})
	button.SetMaxSize(geometry.Size{Width: tabButtonWidth})
	button.SetStyleName("tab-scroll-button")
	glyph := &tabGlyph{direction: direction}
	glyph.SetMinSize(geometry.Size{Width: 16, Height: 16})
	glyph.SetStyleName("tab-scroll-button-text")
	button.SetChild(glyph)
	button.ConnectClicked(func() { bar.scrollByTab(direction) })
	bar.addWheel(button)
	return button
}

func (b *TabBar) View() *TabView { return b.view }
func (b *TabBar) SetView(view *TabView) {
	if b.view == view {
		return
	}
	if view != nil && b.Root() != nil && view.Root() != nil && b.Root() != view.Root() {
		return
	}
	b.closeContextMenu()
	b.cancelDetach()
	if b.nativeDrag != nil {
		b.nativeDrag.source.Cancel()
	}
	b.clearIncoming()
	b.stopDrag()
	b.disconnect()
	b.view = view
	b.lastCurrent = nil
	b.reveal = true
	b.connect()
	b.syncItems()
}
func (b *TabBar) Reorderable() bool { return b.reorderable }

// TabWidthRange returns the configured equal-tab width limits in DIP.
// The effective limits also reserve room for icons, close buttons and padding.
func (b *TabBar) TabWidthRange() (minWidth, maxWidth float32) {
	return b.minTabWidth, b.maxTabWidth
}

// SetTabWidthRange sets equal-tab width limits in DIP (defaults: 112, 240).
// Tabs shrink together from maxWidth to minWidth, then scroll. Equal limits
// disable shrinking. Non-positive or non-finite values restore that limit's
// default; a maximum below the minimum is raised to the minimum.
func (b *TabBar) SetTabWidthRange(minWidth, maxWidth float32) {
	if minWidth <= 0 || math.IsNaN(float64(minWidth)) || math.IsInf(float64(minWidth), 0) {
		minWidth = defaultMinTabWidth
	}
	if maxWidth <= 0 || math.IsNaN(float64(maxWidth)) || math.IsInf(float64(maxWidth), 0) {
		maxWidth = defaultMaxTabWidth
	}
	maxWidth = max(minWidth, maxWidth)
	if b.minTabWidth == minWidth && b.maxTabWidth == maxWidth {
		return
	}
	b.stopDrag()
	b.minTabWidth, b.maxTabWidth = minWidth, maxWidth
	b.reveal = true
	b.viewport.RequestLayout()
}

func (b *TabBar) effectiveTabWidthRange() (float32, float32) {
	minimum := b.minTabWidth
	pages := b.viewPages()
	if b.validIncoming(b.incoming) && b.incoming.view != b.view {
		pages = append(pages, b.incoming.page)
	}
	for _, page := range pages {
		floor := float32(20) // body horizontal padding
		if page.icon != nil {
			floor += 16 + 6
		}
		if page.closable {
			floor += 24 + 4
		}
		minimum = max(minimum, floor)
	}
	return minimum, max(minimum, b.maxTabWidth)
}

func (b *TabBar) SetReorderable(value bool) {
	if b.reorderable == value {
		return
	}
	b.reorderable = value
	if !value {
		b.stopDrag()
	}
}
func (b *TabBar) connect() {
	if b.view == nil {
		return
	}
	if b.Root() != nil && b.view.Root() != nil && b.view.Root() != b.Root() {
		b.SetView(nil)
		return
	}
	if len(b.connections) != 0 {
		return
	}
	b.connections = signal.Handles{
		b.view.changed.Connect(b.syncItems),
		b.view.metadata.Connect(func(page *TabPage) {
			if item := b.items[page]; item != nil {
				item.updateMetadata()
				b.RequestLayout()
			}
		}),
		b.view.ConnectUnmount(func() { b.SetView(nil) }),
		b.view.ConnectMount(func() { b.connect() }),
	}
}
func (b *TabBar) disconnect() { b.connections.Disconnect(); b.connections = nil }

func (b *TabBar) syncItems() {
	var pages []*TabPage
	if b.view != nil {
		pages = b.view.Pages()
	}
	if b.menuPage != nil && !slices.Contains(pages, b.menuPage) {
		b.closeContextMenu()
	}
	if b.dragPage != nil && !slices.Equal(pages, b.dragOrder) {
		b.stopDrag()
	}
	if b.lifted != nil && !slices.Contains(pages, b.lifted) {
		b.stopDrag()
	}
	for page, item := range b.items {
		if !slices.Contains(pages, page) {
			b.viewport.WidgetBase.RemoveChild(item)
			delete(b.items, page)
		}
	}
	var previous gui.Widget
	for _, page := range pages {
		item := b.items[page]
		if item == nil {
			item = newTabItem(b, page)
			b.items[page] = item
			b.viewport.WidgetBase.AddChild(b.viewport, item)
			item.updateMetadata()
		}
		item.updateSelection()
		b.viewport.WidgetBase.MoveChildAfter(item, previous)
		previous = item
	}
	b.raiseLifted()
	b.syncDragVisibility()
	var current *TabPage
	if b.view != nil {
		current = b.view.Current()
	}
	if current != b.lastCurrent {
		b.reveal = true
		b.lastCurrent = current
	}
	b.RequestLayout()
}

func (b *TabBar) Paint(p gui.Painter) {
	paintTabBox(p, geometry.Rect(0, 0, b.Rect().Width, b.Rect().Height), "tab-bar", "", style.Normal)
}
func (b *TabBar) Snapshot() gui.WidgetInfo {
	info := b.WidgetBase.Snapshot()
	info.Role = RoleTabBar
	info.Children = nil
	if b.overflow {
		info.Children = append(info.Children, b.previous.Snapshot())
	}
	// Preserve the snapshot's back-to-front contract while a tab is raised.
	// Canonical page order remains available from TabView's page subtree.
	for _, item := range b.viewport.Children() {
		info.Children = append(info.Children, item.Snapshot())
	}
	if b.overflow {
		info.Children = append(info.Children, b.next.Snapshot())
	}
	return info
}

func (b *TabBar) setScroll(value float32) {
	value = max(0, min(value, max(0, b.contentWidth-b.viewportWidth)))
	if !math.IsNaN(float64(value)) && b.scroll != value {
		b.scroll = value
		b.RequestLayout()
	}
}
func (b *TabBar) scrollByTab(direction int) {
	if direction < 0 {
		candidate := float32(0)
		for _, page := range b.viewPages() {
			child := b.items[page]
			if x := child.Rect().X + b.scroll; x < b.scroll-1 {
				candidate = x
			}
		}
		b.setScroll(candidate)
	} else {
		candidate := b.contentWidth
		for _, page := range b.viewPages() {
			child := b.items[page]
			if x := child.Rect().X + child.Rect().Width + b.scroll; x > b.scroll+b.viewportWidth+1 {
				candidate = x - b.viewportWidth
				break
			}
		}
		b.setScroll(candidate)
	}
}
func (b *TabBar) onKey(ctx gui.EventContext, e events.KeyEvent) {
	if e.Key == events.KeyEscape && b.dragPage != nil {
		b.cancelDrag()
		ctx.StopPropagation()
		e.PreventDefault()
		return
	}
	if b.view == nil || (!b.Focused() && !b.ContainsFocus()) {
		return
	}
	pages := b.view.Pages()
	if len(pages) == 0 {
		return
	}
	if e.Key == events.KeyF10 && e.Modifiers == events.ModifierShift {
		if host, ok := b.Root().(gui.EventTarget); ok {
			for _, page := range pages {
				item := b.items[page]
				if item != nil && (item.body.ContainsFocus() || item.close.ContainsFocus() || host.FocusedWidget() == item.body) {
					ctx.StopPropagation()
					e.PreventDefault()
					b.showContextMenu(page, item.contextMenuPosition(geometry.Point{Y: item.Rect().Height}))
					return
				}
			}
		}
		return
	}
	index := slices.Index(pages, b.view.Current())
	switch e.Key {
	case events.KeyArrowLeft:
		index = max(0, index-1)
	case events.KeyArrowRight:
		index = min(len(pages)-1, index+1)
	case events.KeyHome:
		index = 0
	case events.KeyEnd:
		index = len(pages) - 1
	case events.KeyEnter, events.KeySpace:
		matched := b.Focused()
		if host, ok := b.Root().(gui.EventTarget); ok {
			for _, page := range pages {
				if item := b.items[page]; item != nil && item.body == host.FocusedWidget() {
					index = slices.Index(pages, page)
					matched = true
					break
				}
			}
		}
		if !matched {
			return
		}
	default:
		return
	}
	ctx.StopPropagation()
	e.PreventDefault()
	b.view.SetCurrent(pages[index])
	if host, ok := b.Root().(gui.EventTarget); ok {
		if item := b.items[pages[index]]; item != nil {
			host.SetFocusedWidget(item.body)
		}
	}
}

type tabViewport struct {
	gui.WidgetBase
	bar *TabBar
}

func (v *tabViewport) Paint(p gui.Painter) {
	b := v.bar
	if b.view == nil || b.lifted != nil || b.nativeDrag != nil || b.incoming != nil || b.previewMotion {
		return // suppress separators throughout drag and settling
	}
	s := gui.ResolveStyle("tab-bar", "separator", style.Normal)
	color, ok := s.ForegroundColor()
	if !ok || color == nil {
		return
	}
	brush := graphics.ColorOf(color)
	if brush.A == 0 {
		return
	}
	height := min(float32(16), v.Rect().Height)
	if height <= 0 {
		return
	}
	pages := b.view.Pages()
	for i := 1; i < len(pages); i++ {
		left, right := b.items[pages[i-1]], b.items[pages[i]]
		if pages[i-1] == b.view.Current() || pages[i] == b.view.Current() || left.hovered || right.hovered {
			continue
		}
		x := left.Rect().X + left.Rect().Width + tabSpacing/2 - .5
		// The structural viewport clip is authoritative, including subpixel
		// boundaries; cull only separators completely outside it.
		if x+1 > 0 && x < v.Rect().Width {
			p.FillRect(geometry.Rect(x, (v.Rect().Height-height)/2, 1, height), brush)
		}
	}
}

type tabStripLayout struct{ bar *TabBar }

func (l *tabStripLayout) Measure(children []layout.Child, c layout.Constraint) layout.Measurement {
	var size geometry.Size
	for _, child := range children {
		m := child.Measure(layout.Unbounded())
		size.Height = max(size.Height, m.Height)
	}
	// A hidden native-drag source still reserves its measured slot, including
	// height when it is the only tab. Visibility must not collapse the strip.
	for _, item := range l.bar.items {
		if !item.Visible() {
			size.Height = max(size.Height, item.reservedHeight)
		}
	}
	_, maximum := l.bar.effectiveTabWidthRange()
	// A foreign preview affects allocation/scrolling, not the host's natural
	// size: hovering a tab must not ask the window to grow.
	count := len(l.bar.items)
	size.Width = float32(count)*maximum + float32(max(0, count-1))*tabSpacing
	return layout.Measured(c.Clamp(size))
}
func (l *tabStripLayout) Arrange(children []layout.Child, rect geometry.Rectangle) {
	l.bar.positionTabs(rect.Height)
}

type tabBarLayout struct{ bar *TabBar }

func (l *tabBarLayout) Measure(children []layout.Child, c layout.Constraint) layout.Measurement {
	if len(children) < 3 {
		return layout.Measured(c.Clamp(geometry.Size{}))
	}
	m := children[1].Measure(layout.Unbounded())
	width := m.Width
	if c.Max.Width < layout.Inf {
		width = c.Max.Width
	}
	return layout.Measured(c.Clamp(geometry.Size{Width: width, Height: max(32, m.Height)}))
}
func (l *tabBarLayout) Arrange(children []layout.Child, rect geometry.Rectangle) {
	if len(children) < 3 {
		return
	}
	b := l.bar
	minimum, maximum := b.effectiveTabWidthRange()
	count := b.slotCount()
	tabWidth, overflow := allocateTabWidth(count, rect.Width, minimum, maximum)
	b.tabWidth = tabWidth
	arrows := float32(0)
	if overflow {
		arrows = min(tabButtonWidth, max(0, rect.Width/2))
	}
	width := max(0, rect.Width-2*arrows)
	effective := float32(count)*tabWidth + float32(max(0, count-1))*tabSpacing
	changed := width != b.viewportWidth || effective != b.contentWidth || overflow != b.overflow
	if b.lifted != nil && (changed || rect.Height != b.viewport.Rect().Height) {
		b.stopDrag()
	}
	b.overflow = overflow
	b.viewportWidth = width
	for _, page := range b.viewPages() {
		if item := b.items[page]; item != nil {
			item.width = tabWidth
		}
	}
	b.contentWidth = effective
	b.scroll = max(0, min(b.scroll, max(0, effective-b.viewportWidth)))
	if (b.reveal || changed) && b.view != nil && b.incoming == nil {
		left := float32(0)
		for _, page := range b.viewPages() {
			item := b.items[page]
			if item == nil {
				continue
			}
			width := item.width
			if page == b.view.Current() {
				if width > b.viewportWidth || left < b.scroll {
					b.scroll = left
				} else if left+width > b.scroll+b.viewportWidth {
					b.scroll = left + width - b.viewportWidth
				}
				break
			}
			left += width + tabSpacing
		}
		b.scroll = max(0, min(b.scroll, max(0, effective-b.viewportWidth)))
		b.reveal = false
	}
	children[0].Arrange(geometry.Rect(0, 0, arrows, rect.Height))
	// Slot decisions use final widths and the newly allocated viewport.
	b.updateIncomingSlot()
	children[1].Arrange(geometry.Rect(arrows, 0, b.viewportWidth, rect.Height))
	children[2].Arrange(geometry.Rect(rect.Width-arrows, 0, arrows, rect.Height))
}

func (b *TabBar) viewPages() []*TabPage {
	if b.view == nil {
		return nil
	}
	return b.view.Pages()
}
