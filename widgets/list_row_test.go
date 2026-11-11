package widgets

import (
	"math"
	"testing"

	"github.com/golang-gui/goui/core/geometry"
	"github.com/golang-gui/goui/gui"
	"github.com/golang-gui/goui/layout"
	"github.com/golang-gui/goui/platform/events"
	"github.com/golang-gui/goui/style"
)

func listRowWithoutTypography(t *testing.T) {
	t.Helper()
	previous := gui.App
	gui.App = nil
	t.Cleanup(func() { gui.App = previous })
}

func TestListRowSlotsAndSnapshot(t *testing.T) {
	listRowWithoutTypography(t)
	r := NewListRow(`\{format\}\{strong:Title\}`)
	r.SetSubtitle("Description")
	if r.Activatable() || r.Focusable() || r.Prefix() != nil || r.Suffix() != nil || r.MinSize().Height != 48 {
		t.Fatal("默认行应为非激活、无插槽、48 DIP 最小高度")
	}
	prefix := gui.NewLabel("icon")
	prefix.SetStyleName("custom-icon")
	suffix := NewSwitch()
	suffix.SetChecked(true)
	r.SetPrefix(prefix)
	r.SetSuffix(suffix)
	r.SetPrefix(suffix) // 同一个 Widget 不可占据两个插槽。
	if r.Prefix() != prefix || r.Suffix() != suffix || prefix.Parent() != r || prefix.StyleName() != "custom-icon" {
		t.Fatal("插槽归属或独立样式丢失")
	}
	r.Arrange(geometry.Rect(10, 20, 320, 90))
	info := r.Snapshot()
	if info.Role != gui.RoleListItem || info.Text != "Title\nDescription" || len(info.Actions) != 0 || len(info.Children) != 2 || info.Children[1].Role != RoleSwitch {
		t.Fatalf("行快照不应暴露内部布局或丢失控件语义：%+v", info)
	}
	r.SetActivatable(true)
	if !r.Focusable() || len(r.Snapshot().Actions) != 1 {
		t.Fatal("激活行应支持键盘焦点和点击动作")
	}
	r.SetEnabled(false)
	if info := r.Snapshot(); info.Enabled || len(info.Actions) != 0 || info.Children[1].Enabled {
		t.Fatal("禁用状态未传递到尾部控件/动作")
	}
	r.SetPrefix(nil)
	if prefix.Parent() != nil || prefix.Destroyed() {
		t.Fatal("替换插槽应解绑而非销毁旧内容")
	}
	r.SetTitle("")
	r.SetSubtitle("")
	if r.text.Visible() || r.Snapshot().Text != "" {
		t.Fatal("空标题/说明仍保留文字区域")
	}
	if info.Text != "Title\nDescription" || !info.Children[1].Enabled {
		t.Fatal("既有快照随对象状态改变")
	}
}

type listRowExtent struct{ size geometry.Size }

func (l listRowExtent) Measure(_ []layout.Child, c layout.Constraint) layout.Measurement {
	return layout.Measured(c.Clamp(l.size))
}
func (listRowExtent) Arrange([]layout.Child, geometry.Rectangle) {}
func listRowFixed(width, height float32) *gui.WidgetBase {
	w := new(gui.WidgetBase)
	w.SetLayoutManager(listRowExtent{geometry.Size{Width: width, Height: height}})
	return w
}

func TestListRowBoundedSlots(t *testing.T) {
	listRowWithoutTypography(t)
	r := NewListRow("Title")
	// 固定文字测量隔离字体差异；另由原生文字用例检查真实换行和字号。
	r.text.WidgetBase.SetLayoutManager(listRowExtent{geometry.Size{Width: 150, Height: 60}})
	prefix, suffix := listRowFixed(24, 24), listRowFixed(70, 32)
	r.SetPrefix(prefix)
	r.SetSuffix(suffix)
	for _, width := range []float32{320, 160, 80, 10, 0} {
		m := r.Measure(layout.Loose(geometry.Size{Width: width, Height: 500}))
		r.Arrange(geometry.Rect(0, 0, m.Width, m.Height))
		if m.Width != width || m.Height != 80 {
			t.Fatalf("width=%g，实际测量 %v", width, m)
		}
		previous := float32(0)
		for _, child := range []gui.Widget{prefix, r.text, suffix} {
			b := child.Rect()
			if b.X < previous || b.X < 0 || b.Width < 0 || b.X+b.Width > width || b.Y < 0 || b.Y+b.Height > m.Height {
				t.Fatalf("窄行插槽重叠或越界：width=%g child=%v previous=%g", width, b, previous)
			}
			previous = b.X + b.Width
		}
		if width == 320 && (prefix.Rect().X != 12 || r.text.Rect().X != 48 || r.text.Rect().Width != 178 || suffix.Rect().X != 238) {
			t.Fatal("前后插槽和文字区未保留独立的 12 DIP 间距")
		}
	}
	r.SetMinSize(geometry.Size{})
	r.SetMaxSize(geometry.Size{Width: 100, Height: 30})
	if m := r.Measure(layout.Loose(geometry.Size{Width: 200, Height: 200})); m.Width != 100 || m.Height != 30 {
		t.Fatal("未遵守 WidgetBase 的 min/max 与父约束", m)
	}
	r.SetSpacing(float32(math.NaN()))
	r.SetPadding(geometry.Insets{Left: -1, Top: float32(math.Inf(1)), Right: 4})
	if r.Spacing() != 0 || r.Padding() != (geometry.Insets{Right: 4}) {
		t.Fatal("布局参数未归一化")
	}
}

func TestListRowInputIsolation(t *testing.T) {
	listRowWithoutTypography(t)
	for _, kind := range []string{"switch", "button", "text-input"} {
		t.Run(kind, func(t *testing.T) {
			r := NewListRow("Action")
			r.SetActivatable(true)
			rowCount, childCount := 0, 0
			r.ConnectActivated(func() { rowCount++ })
			var child gui.Widget
			switch kind {
			case "switch":
				b := NewSwitch()
				b.SetAnimated(false)
				b.ConnectChange(func(bool) { childCount++ })
				child = b
			case "button":
				b := gui.NewButton()
				b.ConnectClicked(func() { childCount++ })
				child = b
			default:
				child = gui.NewTextInput()
			}
			child.SetMinSize(geometry.Size{Width: 60, Height: 32})
			child.SetMaxSize(geometry.Size{Width: 60, Height: 32})
			r.SetSuffix(child)
			r.Arrange(geometry.Rect(0, 0, 320, 60))
			h := &tabInputHost{root: r}
			d := new(gui.EventDispatcher)
			click := func(x, y float32) {
				dispatchTabPointer(t, d, h, events.PointerDown, x, y)
				dispatchTabPointer(t, d, h, events.PointerUp, x, y)
			}
			click(20, 20)
			if rowCount != 1 {
				t.Fatal("行主体没有激活")
			}
			b := child.Rect()
			click(b.X+20, b.Y+16)
			if rowCount != 1 || kind != "text-input" && childCount != 1 {
				t.Fatalf("子控件重复激活行：row=%d child=%d", rowCount, childCount)
			}
			// 子控件拥有焦点时，Space 不能冒泡成父行激活。
			h.focus = r
			_ = d.DispatchEvent(h, events.FocusEvent{Focused: true})
			h.focus = child
			_ = d.DispatchEvent(h, events.FocusEvent{Focused: true})
			if r.Focused() || !child.Focused() {
				t.Fatal("测试焦点 crossing 未进入子控件")
			}
			_ = d.DispatchEvent(h, events.KeyEvent{EventType: events.KeyDown, Key: events.KeySpace})
			if rowCount != 1 {
				t.Fatal("子控件键盘输入激活了父行")
			}
			r.SetActivatable(false)
			click(20, 20)
			if rowCount != 1 || r.Focusable() || r.State().Has(style.Pressed) {
				t.Fatal("取消激活后仍响应主体点击")
			}
		})
	}
}

func TestListRowKeyboardCancellationAndLifetime(t *testing.T) {
	listRowWithoutTypography(t)
	r := NewListRow("Action")
	r.SetActivatable(true)
	owner := gui.NewPopover(nil, nil)
	owner.SetWidget(r)
	h := &tabInputHost{root: r}
	d := new(gui.EventDispatcher)
	dispatchTabPointer(t, d, h, events.PointerDown, 0, 0) // 未布局的空区域不激活。
	r.Arrange(geometry.Rect(0, 0, 320, 60))
	count := 0
	r.ConnectActivated(func() { count++ })
	key := func(key gui.Key, repeat bool) {
		t.Helper()
		if err := d.DispatchEvent(h, events.KeyEvent{EventType: events.KeyDown, Key: events.Key(key), Repeat: repeat}); err != nil {
			t.Fatal(err)
		}
	}
	// 正常 PointerDown 建立 Widget 的焦点 crossing 状态。
	dispatchTabPointer(t, d, h, events.PointerDown, 20, 20)
	dispatchTabPointer(t, d, h, events.PointerUp, 20, 20)
	h.SetFocusedWidget(r)
	_ = d.DispatchEvent(h, events.FocusEvent{Focused: true})
	key(gui.KeySpace, false)
	key(gui.KeySpace, true)
	key(gui.KeyEnter, false)
	if count != 3 {
		t.Fatalf("键盘激活/重复过滤错误：%d", count)
	}
	dispatchTabPointer(t, d, h, events.PointerDown, 20, 20)
	dispatchTabPointer(t, d, h, events.PointerMove, 400, 20)
	dispatchTabPointer(t, d, h, events.PointerUp, 400, 20)
	if count != 3 || r.State().Has(style.Pressed) {
		t.Fatal("移出取消没有清理按压")
	}
	r.SetEnabled(false)
	key(gui.KeyEnter, false)
	if count != 3 {
		t.Fatal("禁用后仍激活")
	}
	r.SetEnabled(true)
	h.SetFocusedWidget(r)
	_ = d.DispatchEvent(h, events.FocusEvent{Focused: true})
	r.ConnectActivated(owner.Destroy)
	late := 0
	r.ConnectActivated(func() { late++ })
	key(gui.KeyEnter, false)
	if r.Root() != nil || late != 0 {
		t.Fatal("销毁回调后仍执行后续激活监听")
	}
}

func TestListRowSlotReentry(t *testing.T) {
	listRowWithoutTypography(t)
	r := NewListRow("Row")
	owner := gui.NewPopover(nil, nil)
	owner.SetWidget(r)
	defer owner.Destroy()
	old, reentrant, outer := listRowFixed(10, 10), listRowFixed(20, 10), listRowFixed(30, 10)
	r.SetPrefix(old)
	once := false
	old.ConnectUnmount(func() {
		if !once {
			once = true
			r.SetPrefix(reentrant)
		}
	})
	r.SetPrefix(outer)
	if r.Prefix() != reentrant || outer.Parent() != nil || old.Parent() != nil {
		t.Fatal("旧内容的卸载回调被过期 setter 覆盖")
	}
	r.SetSuffix(listRowFixed(10, 10))
	r.Suffix().ConnectUnmount(owner.Destroy)
	r.SetSuffix(outer)
	if r.Root() != nil || outer.Parent() != nil {
		t.Fatalf("卸载中销毁后仍挂载新内容：destroyed=%v root=%v outerParent=%T", r.Destroyed(), r.Root(), outer.Parent())
	}
}
