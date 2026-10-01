package widgets

import (
	"slices"
	"time"

	"github.com/golang-gui/goui/core/geometry"
	"github.com/golang-gui/goui/gui"
)

func (b *TabBar) beginDrag(page *TabPage, point geometry.Point) {
	if !b.reorderable || b.view == nil || page.view != b.view || b.items[page] == nil {
		return
	}
	b.closeContextMenu()
	b.cancelDetach()
	b.dragPage, b.lifted = page, page
	b.dragOrder = b.view.Pages()
	b.insertAt = slices.Index(b.dragOrder, page)
	b.grabX = point.X - b.viewport.Rect().X - b.items[page].Rect().X
	b.grabY = point.Y - b.viewport.Rect().Y
	b.raiseLifted()
	b.updateDrag(page, point)
}

func (b *TabBar) updateDrag(page *TabPage, point geometry.Point) {
	if b.dragPage != page {
		return
	}
	b.pointer = point
	b.recalculateInsert()
	b.positionTabs(b.viewport.Rect().Height)
	b.RequestPaint()
}

func (b *TabBar) draggedX() float32 {
	item := b.items[b.dragPage]
	return b.scroll + max(0, min(b.pointer.X-b.viewport.Rect().X-b.grabX, b.viewportWidth-item.width))
}

// Test against target slots, never against animated rectangles. The leading
// edge crosses a neighbor's midpoint before the placeholder changes position;
// the resulting hysteresis avoids oscillation when the pointer stays still.
func (b *TabBar) recalculateInsert() {
	if b.dragPage == nil {
		return
	}
	others := make([]*tabItem, 0, len(b.items)-1)
	for _, page := range b.dragOrder {
		if page != b.dragPage {
			others = append(others, b.items[page])
		}
	}
	left := b.draggedX()
	width := b.items[b.dragPage].width
	gap := float32(0)
	for _, item := range others[:b.insertAt] {
		gap += item.width + tabSpacing
	}
	for b.insertAt > 0 && left < gap-tabSpacing-others[b.insertAt-1].width/2 {
		b.insertAt--
		gap -= others[b.insertAt].width + tabSpacing
	}
	for b.insertAt < len(others) && left+width > gap+width+tabSpacing+others[b.insertAt].width/2 {
		gap += others[b.insertAt].width + tabSpacing
		b.insertAt++
	}
}

// positionTabs is the viewport's normal Arrange path and is also used by its
// timer. The actual widget rectangles carry visual positions, so painting,
// picking and Snapshot share coordinates without a second rendering tree.
func (b *TabBar) positionTabs(height float32) {
	pages := b.viewPages()
	if b.dragPage != nil {
		from := slices.Index(pages, b.dragPage)
		pages = slices.Delete(pages, from, from+1)
		pages = slices.Insert(pages, b.insertAt, b.dragPage)
	}
	x := float32(0)
	preview := b.validIncoming(b.incoming)
	index := 0
	for _, page := range pages {
		item := b.items[page]
		if preview && page == b.incoming.page {
			// Re-entering this view moves its existing slot instead of adding
			// a second one. The original page and content remain source-owned.
			item.motion.place(float32(b.incomingAt)*(b.tabWidth+tabSpacing), b.previewMotion)
		} else {
			if preview && index == b.incomingAt {
				x += b.tabWidth + tabSpacing
			}
			if page == b.dragPage {
				item.motion.place(b.draggedX(), false)
			} else {
				item.motion.place(x, b.lifted != nil || b.previewMotion)
			}
			x += item.width + tabSpacing
			index++
		}
		item.widthMotion.place(item.width, b.previewMotion)
		item.Arrange(geometry.Rect(item.motion.x-b.scroll, 0, item.widthMotion.x, height))
	}
	b.updateTimer()
}

func (b *TabBar) endDrag(page *TabPage, point geometry.Point) {
	if b.dragPage != page {
		return
	}
	b.updateDrag(page, point)
	index, view := b.insertAt, b.view
	viewport := b.viewport.Rect()
	inside := point.X >= viewport.X && point.X <= viewport.X+viewport.Width &&
		point.Y >= viewport.Y && point.Y <= viewport.Y+viewport.Height
	b.dragPage, b.dragOrder = nil, nil
	if inside && view != nil {
		// Clear active drag state before callbacks; they may remove this page,
		// replace the view or unmount the bar. syncItems handles each case.
		view.MovePage(page, index)
	}
	b.positionTabs(b.viewport.Rect().Height)
	b.RequestPaint()
}

func (b *TabBar) cancelDrag() {
	if b.dragPage == nil {
		return
	}
	b.dragPage, b.dragOrder = nil, nil
	b.positionTabs(b.viewport.Rect().Height)
	b.RequestPaint()
}

// Unmount, structural changes and resize end motion immediately. User cancel
// instead settles the tabs back into their original positions.
func (b *TabBar) stopDrag() {
	b.dragPage, b.dragOrder, b.lifted = nil, nil, nil
	b.previewMotion = false
	if b.timer != nil {
		b.timer.Stop()
	}
	b.lastTick = time.Time{}
	for _, item := range b.items {
		item.motion.place(item.motion.target, false)
		item.widthMotion.place(item.width, false)
	}
	b.restoreOrder()
	b.RequestLayout()
}

func (b *TabBar) raiseLifted() {
	if item := b.items[b.lifted]; item != nil {
		b.viewport.WidgetBase.MoveChildBefore(item, nil)
	}
}

func (b *TabBar) restoreOrder() {
	var previous gui.Widget
	for _, page := range b.viewPages() {
		if item := b.items[page]; item != nil {
			b.viewport.WidgetBase.MoveChildAfter(item, previous)
			previous = item
		}
	}
}

func (b *TabBar) scrollVelocity() float32 {
	viewport := b.viewport.Rect()
	if (b.dragPage == nil && b.incoming == nil) || b.viewportWidth <= 0 || b.pointer.Y < viewport.Y || b.pointer.Y > viewport.Y+viewport.Height {
		return 0
	}
	x, edge := b.pointer.X-viewport.X, min(tabDragEdge, b.viewportWidth/2)
	depth := float32(0)
	if x < edge && b.scroll > 0 {
		depth = -(edge - x) / edge
	} else if x > b.viewportWidth-edge && b.scroll < b.contentWidth-b.viewportWidth {
		depth = (x - (b.viewportWidth - edge)) / edge
	}
	return max(-1, min(1, depth)) * 240
}

func (b *TabBar) moving() bool {
	for _, item := range b.items {
		if item.motion.x != item.motion.target || item.widthMotion.x != item.widthMotion.target {
			return true
		}
	}
	return false
}

func (b *TabBar) updateTimer() {
	if !b.moving() && b.scrollVelocity() == 0 {
		b.previewMotion = false
		if b.timer != nil {
			b.timer.Stop()
		}
		if b.dragPage == nil && b.lifted != nil {
			b.lifted = nil
			b.restoreOrder()
		}
		return
	}
	if b.timer == nil && b.Root() != nil && gui.App != nil {
		b.timer = gui.App.NewTimer()
		b.timerHandle = b.timer.ConnectTimeout(func() {
			now := time.Now()
			elapsed := now.Sub(b.lastTick)
			b.lastTick = now
			b.advanceMotion(elapsed)
		})
	}
	if b.timer != nil && !b.timer.Active() {
		b.lastTick = time.Now()
		_ = b.timer.Start(16 * time.Millisecond)
	}
}

func (b *TabBar) advanceMotion(elapsed time.Duration) {
	elapsed = max(0, min(elapsed, 100*time.Millisecond))
	velocity := b.scrollVelocity()
	if velocity != 0 {
		b.scroll = max(0, min(b.scroll+velocity*float32(elapsed.Seconds()), max(0, b.contentWidth-b.viewportWidth)))
		b.recalculateInsert()
		if b.incoming != nil {
			b.updateIncomingSlot()
		}
	}
	// Apply any new placeholder targets before advancing the same tick.
	b.positionTabs(b.viewport.Rect().Height)
	for page, item := range b.items {
		if page != b.dragPage {
			item.motion.advance(elapsed)
		}
		item.widthMotion.advance(elapsed)
	}
	b.positionTabs(b.viewport.Rect().Height)
	b.RequestPaint()
}
