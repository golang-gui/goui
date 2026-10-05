package widgets

import (
	"github.com/golang-gui/goui/core/geometry"
	"github.com/golang-gui/goui/core/signal"
	"github.com/golang-gui/goui/gui"
	"github.com/golang-gui/goui/platform/events"
)

func (v *TableView) choose(id string, primary, shift bool) {
	index, ok := v.indices[id]
	if !ok || v.Destroyed() {
		return
	}
	oldSelection, oldCurrent := v.Selection(), v.current
	v.current = id
	if v.mode == SelectionSingle || !primary && !shift {
		clear(v.selected)
		v.selected[id] = true
		v.anchor = id
	} else if shift {
		anchor, ok := v.indices[v.anchor]
		if !ok {
			anchor = index
			v.anchor = id
		}
		if !primary {
			clear(v.selected)
		}
		for i := min(anchor, index); i <= max(anchor, index); i++ {
			v.selected[v.rowIDs[i]] = true
		}
	} else {
		if v.selected[id] {
			delete(v.selected, id)
		} else {
			v.selected[id] = true
		}
		v.anchor = id
	}
	v.notify(oldSelection, oldCurrent)
}
func (v *TableView) keyDown(ctx gui.EventContext, e events.KeyEvent) {
	if v.Destroyed() || len(v.rowIDs) == 0 || len(v.columns) == 0 {
		return
	}
	primary, shift := treeModifiers(e.Modifiers)
	allowed, _ := (gui.ModPrimary | gui.ModShift).Resolve()
	if e.Modifiers & ^allowed != 0 {
		return
	}
	index, hasCurrent := v.indices[v.current]
	if !hasCurrent {
		index = 0
		if ids := v.Selection(); len(ids) > 0 {
			index = v.indices[ids[0]]
		}
	}
	target, navigation := index, true
	switch e.Key {
	case events.KeyArrowUp:
		if hasCurrent {
			target--
		}
	case events.KeyArrowDown:
		if hasCurrent {
			target++
		}
	case events.KeyHome:
		target = 0
	case events.KeyEnd:
		target = len(v.rowIDs) - 1
	case events.KeyPageUp:
		target -= max(1, len(v.list.VisibleIndexes())-1)
	case events.KeyPageDown:
		target += max(1, len(v.list.VisibleIndexes())-1)
	case events.KeySpace:
		v.choose(v.rowIDs[index], primary, false)
		navigation = false
	case events.KeyEnter:
		if !hasCurrent {
			return
		}
		v.activate.Emit(v.current, v.revision)
		navigation = false
	case events.KeyA:
		if !primary || v.mode != SelectionMultiple {
			return
		}
		v.SetSelection(v.rowIDs)
		navigation = false
	default:
		return
	}
	ctx.StopPropagation()
	e.PreventDefault()
	if navigation && !v.Destroyed() && len(v.rowIDs) > 0 {
		id := v.rowIDs[max(0, min(target, len(v.rowIDs)-1))]
		if primary && !shift {
			v.SetCurrent(id)
		} else {
			v.choose(id, primary && shift, shift)
		}
		if !v.Destroyed() {
			v.Reveal(id)
		}
	}
}

type tableMenuQuery struct {
	row, column string
	result      *gui.MenuModel
}

func (v *TableView) ConnectContextMenu(fn func(string, string, *gui.MenuModel)) signal.Handle {
	return v.menuQuery.Connect(func(query tableMenuQuery, version uint64) {
		if v.valid(version) {
			fn(query.row, query.column, query.result)
		}
	})
}
func (v *TableView) ConnectContextMenuError(fn func(error)) signal.Handle {
	return v.menuError.Connect(func(err error) {
		if !v.Destroyed() {
			fn(err)
		}
	})
}
func (v *TableView) closeMenu() {
	v.menuID, v.menuPending = "", ""
	if v.menu != nil {
		v.menu.SetMenu(nil)
	}
}
func (v *TableView) showMenu(row, column string, position geometry.Point) {
	if v.Destroyed() {
		return
	}
	v.closeMenu()
	version := v.revision
	var model gui.MenuModel
	v.menuQuery.Emit(tableMenuQuery{row, column, &model}, version)
	if !v.valid(version) || model == nil || model.ItemsCount() == 0 {
		return
	}
	if row != "" {
		if _, ok := v.indices[row]; !ok {
			return
		}
	}
	if v.menu == nil {
		v.menu = gui.NewPopoverMenu(v.body)
		v.menu.ConnectClosed(v.closeMenu)
	}
	v.menuID = row
	v.menu.SetMenu(model)
	if err := v.menu.ShowAt(position); err != nil {
		v.closeMenu()
		v.menuError.Emit(err)
	}
}
func (v *TableView) contextMenuRequested(ctx gui.EventContext) {
	point, pointer := ctx.Position()
	if !pointer {
		if v.current != "" {
			ctx.StopPropagation()
			v.menuPending = v.current
			v.Reveal(v.current)
		}
		return
	}
	// This controller belongs to the body: point excludes the fixed header and
	// scrollbars. Only the body can query a record context menu.
	ctx.StopPropagation()
	version := v.revision
	rowID, columnID := "", ""
	for _, row := range v.realized {
		r := row.Rect()
		if point.Y >= r.Y && point.Y < r.Y+r.Height {
			rowID = row.bound.ID
			break
		}
	}
	x := point.X + v.scroll.ScrollX()
	for i, c := range v.columns {
		if x >= v.columnX[i] && x < v.columnX[i]+c.width {
			columnID = c.id
			break
		}
	}
	if rowID != "" {
		if v.selected[rowID] {
			v.SetCurrent(rowID)
		} else {
			v.choose(rowID, false, false)
		}
	}
	if v.valid(version) {
		v.showMenu(rowID, columnID, point)
	}
}
