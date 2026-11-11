package ui

import (
	"testing"

	"github.com/golang-gui/goui/core/geometry"
	"github.com/golang-gui/goui/gui"
	"github.com/golang-gui/goui/platform/events"
	baseui "github.com/golang-gui/goui/ui"
	"github.com/golang-gui/goui/widgets"
)

func TestListRowReconcileSlotsAndDefaults(t *testing.T) {
	previous := gui.App
	gui.App = nil
	t.Cleanup(func() { gui.App = previous })
	c := new(tabContext)
	old, latest := 0, 0
	v := ListRow("Title").Subtitle("Description").Prefix(baseui.Label("icon")).Suffix(baseui.Label("value")).Padding(6).Spacing(8).Activatable(true).OnActivate(func() { old++ })
	r := v.Mount(c).(*widgets.ListRow)
	v.Update(c, r)
	s := c.State().(*listRowState)
	prefix, suffix := r.Prefix(), r.Suffix()
	if r.Title() != "Title" || r.Subtitle() != "Description" || !r.Activatable() || r.Padding() != geometry.UniformInsets(6) || r.Spacing() != 8 || len(c.children) != 2 {
		t.Fatal("初次声明未同步")
	}
	v = ListRow("Updated").Prefix(baseui.Label("new icon")).Suffix(baseui.Label("new value")).Activatable(true).Padding(0).Spacing(0).OnActivate(func() { latest++ })
	v.Update(c, r)
	if r.Prefix() != prefix || r.Suffix() != suffix || c.State() != s || r.Subtitle() != "" || r.Padding() != (geometry.Insets{}) || r.Spacing() != 0 {
		t.Fatal("插槽身份、属性移除或显式零值丢失")
	}
	r.Arrange(geometry.Rect(0, 0, 320, 60))
	h := &splitUIHost{root: r}
	d := new(gui.EventDispatcher)
	click := func() {
		t.Helper()
		for _, kind := range []events.EventType{events.PointerDown, events.PointerUp} {
			if err := d.DispatchEvent(h, events.PointerEvent{EventType: kind, Position: geometry.Point{X: 2, Y: 2}, Button: events.PointerButtonLeft}); err != nil {
				t.Fatal(err)
			}
		}
	}
	click()
	if old != 0 || latest != 1 {
		t.Fatalf("激活仍调用过期闭包：old=%d latest=%d", old, latest)
	}
	ListRow("Plain").Update(c, r)
	if r.Activatable() || r.Prefix() != nil || r.Suffix() != nil || prefix.Parent() != nil || suffix.Parent() != nil || r.Padding() != s.padding || r.Spacing() != s.spacing {
		t.Fatal("移除插槽/属性没有恢复默认")
	}
	v.Update(c, r)
	v.Unmount(c, r)
	click()
	if latest != 1 {
		t.Fatal("Unmount 后声明连接仍执行")
	}
}
