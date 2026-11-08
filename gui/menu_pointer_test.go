package gui

import (
	"image/color"
	"testing"

	"github.com/golang-gui/goui/core/geometry"
	"github.com/golang-gui/goui/layout"
	"github.com/golang-gui/goui/platform/events"
	"github.com/golang-gui/goui/platform/graphics"
	"github.com/golang-gui/goui/style"
)

func TestMenuPointerAndKeyboardShareOneActiveItem(t *testing.T) {
	normal, highlight := color.NRGBA{R: 255, G: 255, B: 255, A: 255}, color.NRGBA{B: 220, A: 255}
	useTestApplication(t, &application{typo: &styleTypography{}, style: style.Sheet(
		style.Name(styleNameMenuItemText).FontSize(12),
		style.Name(styleNameMenuItemTextDisabled).FontSize(12),
		style.Name(styleNameMenuItem).BackgroundColor(normal),
		style.Name(styleNameMenuItem).State(style.Hovered).BackgroundColor(highlight),
		style.Name(styleNameMenuItem).State(style.Pressed).BackgroundColor(highlight),
	)})
	m := NewMenu()
	first, second := m.Append("First", nil), m.Append("Second", nil)
	disabled := m.Append("Disabled", nil)
	disabled.SetEnabled(false)
	m.AppendSeparator()
	m.Append("Last", nil)
	var activated *MenuItem
	content := newMenuContent(m, 0, func(item *MenuItem) { activated = item })
	win := testWindowWithSurface(&window{})
	win.SetWidget(content)
	defer win.Destroy()
	size := content.Measure(layout.Unbounded()).Size
	content.Arrange(geometry.Rect(0, 0, size.Width, size.Height))
	row := func(index int) *menuItemRow {
		t.Helper()
		r, ok := content.list.items[index].(*menuItemRow)
		if !ok {
			t.Fatalf("row %d was not realized", index)
		}
		return r
	}
	point := func(index int, text bool) geometry.Point {
		r := row(index)
		bounds := r.Snapshot().Bounds
		p := geometry.Point{X: bounds.X + 2, Y: bounds.Y + bounds.Height/2}
		var want Widget = r
		if text {
			bounds = r.label.Snapshot().Bounds
			p = geometry.Point{X: bounds.X + bounds.Width/2, Y: bounds.Y + bounds.Height/2}
			want = r.label
		}
		if got := Pick(content, p); got != want {
			t.Fatalf("point did not hit intended target: %T, want %T", got, want)
		}
		return p
	}
	assertActive := func(want *MenuItem) {
		t.Helper()
		painted, selected := 0, 0
		for _, widget := range content.list.items {
			r := widget.(*menuItemRow)
			background, ok := r.resolvedStyle().BackgroundColor()
			lit := ok && background != nil && graphics.ColorOf(background) == graphics.ColorOf(highlight)
			if lit {
				painted++
			}
			if r.Snapshot().Selected {
				selected++
			}
			if lit != (want != nil && r.mi == want) || lit != r.Snapshot().Selected {
				t.Errorf("row %q: painted=%t selected=%t, want active=%v", r.mi.Label(), lit, r.Snapshot().Selected, want)
			}
		}
		count := 0
		if want != nil {
			count = 1
		}
		if content.active != want || painted != count || selected != count {
			t.Fatalf("active=%v, painted=%d selected=%d; want %v/%d", content.active, painted, selected, want, count)
		}
	}
	move := func(index int, text bool) {
		t.Helper()
		if err := win.DispatchEvent(events.PointerEvent{EventType: events.PointerMove, Position: point(index, text)}); err != nil {
			t.Fatal(err)
		}
	}
	press := func(key events.Key) {
		t.Helper()
		if err := win.DispatchEvent(shortcutPress(key, 0)); err != nil {
			t.Fatal(err)
		}
	}

	move(0, false)
	assertActive(first)
	if err := win.DispatchEvent(events.PointerEvent{EventType: events.PointerDown, Button: events.PointerButtonLeft, Position: point(0, false)}); err != nil {
		t.Fatal(err)
	}
	press(events.KeyEnd)
	assertActive(m.ItemAt(4))
	if err := win.DispatchEvent(events.PointerEvent{EventType: events.PointerUp, Button: events.PointerButtonLeft, Position: point(0, false)}); err != nil {
		t.Fatal(err)
	}
	// 指针不移动时，新的按下也应接管键盘活动项。
	if err := win.DispatchEvent(events.PointerEvent{EventType: events.PointerDown, Button: events.PointerButtonLeft, Position: point(0, false)}); err != nil {
		t.Fatal(err)
	}
	assertActive(first)
	if err := win.DispatchEvent(events.PointerEvent{EventType: events.PointerUp, Button: events.PointerButtonLeft, Position: point(0, false)}); err != nil {
		t.Fatal(err)
	}
	// 直接从一行留白移到另一行的 Label。
	move(1, true)
	assertActive(second)
	for range 100 {
		move(0, true)
		assertActive(first)
		move(1, true)
		assertActive(second)
	}
	press(events.KeyHome)
	assertActive(first)
	// 指针仍在 Second 的 Label 内，没有 crossing，普通移动也必须接管。
	move(1, true)
	assertActive(second)
	move(1, false)
	assertActive(second)
	move(1, true)
	assertActive(second)
	move(2, true)
	assertActive(nil)
	move(3, false)
	assertActive(nil)
	move(0, true)
	assertActive(first)
	if err := win.DispatchEvent(events.PointerEvent{EventType: events.PointerLeave}); err != nil {
		t.Fatal(err)
	}
	assertActive(nil)
	for _, widget := range content.list.items {
		if widget.Snapshot().Hovered {
			t.Fatal("离开菜单后仍有物理悬停")
		}
	}
	// 原生 Enter 只有 crossing，没有普通 Motion。
	if err := win.DispatchEvent(events.PointerEvent{EventType: events.PointerEnter, Position: point(1, true)}); err != nil {
		t.Fatal(err)
	}
	assertActive(second)
	// 顶部留白不能保留上一项高亮。
	blank := geometry.Point{X: size.Width / 2, Y: 1}
	if target := Pick(content, blank); target != content {
		t.Fatalf("留白命中 %T，预期菜单内容", target)
	}
	if err := win.DispatchEvent(events.PointerEvent{EventType: events.PointerMove, Position: blank}); err != nil {
		t.Fatal(err)
	}
	assertActive(nil)
	press(events.KeyHome)
	assertActive(first)
	press(events.KeyEnd)
	assertActive(m.ItemAt(4))
	// 指针静止时键盘活动项保持，下一次移动才接管。
	move(1, true)
	assertActive(second)
	press(events.KeyEnter)
	if activated != second {
		t.Fatal("keyboard activation ignored the pointer's active item")
	}
}

// 滚动、Reveal 和虚拟行复用不能把旧物理悬停当作新的菜单选择。
func TestMenuActiveItemAcrossScrollingAndRowReuse(t *testing.T) {
	useTestApplication(t, &application{typo: &styleTypography{}})
	model := NewMenu()
	for range 50 {
		model.Append("Item", nil)
	}
	content := newMenuContent(model, 100, func(*MenuItem) {})
	win := testWindowWithSurface(&window{})
	win.SetWidget(content)
	defer win.Destroy()
	size := content.Measure(layout.Unbounded()).Size
	content.Arrange(geometry.Rect(13, 27, size.Width, size.Height))
	if !content.sv.vbar.Visible() {
		t.Fatal("回归场景没有实际出现滚动条")
	}
	move := func(position geometry.Point) {
		t.Helper()
		if err := win.DispatchEvent(events.PointerEvent{EventType: events.PointerMove, Position: position}); err != nil {
			t.Fatal(err)
		}
	}
	first := content.list.items[0].Snapshot().Bounds
	point := geometry.Point{X: first.X + 2, Y: first.Y + first.Height/2}
	move(point)
	if content.active != model.ItemAt(0) {
		t.Fatal("未选择首行")
	}
	move(geometry.Point{X: first.X + first.Width + 1, Y: point.Y})
	if content.active != nil {
		t.Fatal("移入菜单行与滚动条之间的间距后仍有活动项")
	}
	move(point)
	bar := content.sv.vbar.Snapshot().Bounds
	move(geometry.Point{X: bar.X + bar.Width/2, Y: bar.Y + bar.Height/2})
	if content.active != nil {
		t.Fatal("移入滚动条后仍有活动项")
	}
	move(point)
	if err := win.DispatchEvent(shortcutPress(events.KeyEnd, 0)); err != nil {
		t.Fatal(err)
	}
	// Reveal 请求在下一轮布局生效；此测试没有原生帧循环，显式执行该阶段。
	content.Arrange(content.Rect())
	last := model.ItemAt(49)
	if content.active != last || content.sv.ScrollY() <= 0 || content.list.items[49] == nil {
		t.Fatalf("键盘未选中并显示末行：active=%p want=%p scroll=%g rows=%v", content.active, last, content.sv.ScrollY(), content.list.items)
	}
	if content.list.items[0] != nil || !content.list.items[49].Snapshot().Selected {
		t.Fatal("首行未回收或末行缺少活动项投影")
	}
	content.sv.SetScrollY(0)
	if content.active != last || content.list.items[0] == nil {
		t.Fatal("单纯布局和行复用改变了键盘活动项")
	}
	for _, widget := range content.list.items {
		row := widget.(*menuItemRow)
		if row.Snapshot().Selected != (row.mi == last) {
			t.Fatal("复用行保留了旧选中状态")
		}
	}
	move(point)
	if content.active != model.ItemAt(0) || !content.list.items[0].Snapshot().Selected {
		t.Fatal("滚回后指针未在原位置接管活动项")
	}
}

// 内容有窗口局部偏移时，普通移动、空白和原生 Enter 使用相同的命中坐标。
func TestMenuPointerSelectionWithContentOffset(t *testing.T) {
	useTestApplication(t, &application{typo: &styleTypography{}})
	model := NewMenu()
	first := model.Append("First", nil)
	second := model.Append("Second", nil)
	content := newMenuContent(model, 0, func(*MenuItem) {})
	win := testWindowWithSurface(&window{})
	win.SetWidget(content)
	defer win.Destroy()
	size := content.Measure(layout.Unbounded()).Size
	origin := geometry.Point{X: 19, Y: 31}
	content.Arrange(geometry.Rectangle{Pos: origin, Size: size})
	for _, eventType := range []events.EventType{events.PointerMove, events.PointerEnter} {
		for index, item := range []*MenuItem{first, second} {
			if err := win.DispatchEvent(events.PointerEvent{EventType: events.PointerLeave}); err != nil {
				t.Fatal(err)
			}
			if content.active != nil {
				t.Fatal("离开后活动项未清空")
			}
			bounds := content.list.items[index].Snapshot().Bounds
			position := geometry.Point{X: bounds.X + 2, Y: bounds.Y + bounds.Height/2}
			if err := win.DispatchEvent(events.PointerEvent{EventType: eventType, Position: position}); err != nil {
				t.Fatal(err)
			}
			if content.active != item || !content.list.items[index].Snapshot().Selected {
				t.Fatalf("事件 %v 未在有偏移的菜单选择 %q", eventType, item.Label())
			}
			if err := win.DispatchEvent(events.PointerEvent{EventType: events.PointerMove, Position: geometry.Point{X: origin.X + 1, Y: origin.Y + 1}}); err != nil {
				t.Fatal(err)
			}
			if content.active != nil || content.list.items[index].Snapshot().Selected {
				t.Fatal("移动到有偏移的菜单留白后仍有活动项")
			}
		}
	}
}
