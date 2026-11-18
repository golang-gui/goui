package widgets

import (
	"math"
	"testing"

	"github.com/golang-gui/goui/core/geometry"
	"github.com/golang-gui/goui/gui"
	"github.com/golang-gui/goui/layout"
	"github.com/golang-gui/goui/platform/events"
)

func TestListRowGroupUniformRowsNaturalHeightAndRestore(t *testing.T) {
	listRowWithoutTypography(t)
	rows := listRowGroupRows(48, 80, 60)
	rows[0].SetMainWeight(1)
	rows[2].SetMainWeight(3)
	rows[0].SetMaxSize(geometry.Size{Height: 70})
	group := listRowGroupWith(rows...)
	if group.UniformRows() {
		t.Fatal("统一行高应默认关闭")
	}
	type properties struct {
		min, max geometry.Size
		weight   float32
	}
	var original []properties
	for _, row := range rows {
		original = append(original, properties{row.MinSize(), row.MaxSize(), row.MainWeight()})
	}
	group.SetUniformRows(true)
	c := layout.Loose(geometry.Size{Width: 100, Height: layout.Inf})
	if got := group.Measure(c); got.Width != 100 || got.Height != 240 {
		t.Fatal("未按最高行的自然高度测量", got)
	}
	for _, height := range []float32{500, 100} {
		// 多余空间留空；不足时保留自然行高，不按 MainWeight 压缩某一行。
		group.Arrange(geometry.Rect(0, 0, 100, height))
		for i, row := range rows {
			if got := row.Rect(); got != geometry.Rect(0, float32(i)*80, 100, 80) {
				t.Fatalf("分配高度 %g，第 %d 行未保持统一自然高度：%v", height, i, got)
			}
			if got := (properties{row.MinSize(), row.MaxSize(), row.MainWeight()}); got != original[i] {
				t.Fatal("统一布局改写了行的尺寸或权重", got, original[i])
			}
		}
	}
	if got := group.Measure(layout.Loose(geometry.Size{Width: 100, Height: 100})); got.Height != 100 {
		t.Fatal("分组自身没有遵守父约束", got)
	}
	group.SetUniformRows(false)
	if got := group.Measure(c); got.Height != 188 {
		t.Fatal("关闭后没有恢复自然测量", got)
	}
	group.Arrange(geometry.Rect(0, 0, 100, 188))
	for i, row := range rows {
		if row.Rect().Height != original[i].min.Height {
			t.Fatal("关闭后没有恢复各行自然高度", row.Rect())
		}
	}
	group.Arrange(geometry.Rect(0, 0, 100, 500))
	if rows[0].Rect().Height != 126 || rows[2].Rect().Height != 294 {
		t.Fatal("关闭后没有恢复原 MainWeight 分配", rows[0].Rect(), rows[2].Rect())
	}
}

func TestListRowGroupUniformRowsOnlyDirectRows(t *testing.T) {
	listRowWithoutTypography(t)
	rows := listRowGroupRows(48, 80)
	nestedRows := listRowGroupRows(110, 60)
	nested := listRowGroupWith(nestedRows...)
	other := listRowFixed(10, 120)
	other.SetMainWeight(1)
	group := NewListRowGroup()
	group.SetUniformRows(true)
	for _, child := range []gui.Widget{rows[0], other, nested, rows[1]} {
		group.AddChild(child)
	}
	c := layout.Loose(geometry.Size{Width: 100, Height: layout.Inf})
	if got := group.Measure(c); got.Height != 450 {
		t.Fatal("非行控件或嵌套行影响了直接行的共同高度", got)
	}
	group.Arrange(geometry.Rect(0, 0, 100, 550))
	if rows[0].Rect().Height != 80 || rows[1].Rect().Height != 80 || other.Rect().Height != 220 || nested.Rect().Height != 170 ||
		nestedRows[0].Rect().Height != 110 || nestedRows[1].Rect().Height != 60 || nested.UniformRows() {
		t.Fatal("统一行高改变了非行控件的权重或嵌套布局", group.Snapshot())
	}
	nested.SetUniformRows(true)
	if got := group.Measure(c); got.Height != 500 {
		t.Fatal("嵌套组无法独立开启统一行高", got)
	}
}

// 固定字宽的折行替身提供可独立计算的预期；真实文字另由 Pango 和窗口用例验证。
type listRowUniformWrapping struct{ width, lineHeight float32 }

func (l listRowUniformWrapping) Measure(_ []layout.Child, c layout.Constraint) layout.Measurement {
	width := min(c.Max.Width, l.width)
	lines := float32(math.Ceil(float64(l.width / max(1, width))))
	return layout.Measured(c.Clamp(geometry.Size{Width: width, Height: lines * l.lineHeight}))
}
func (listRowUniformWrapping) Arrange([]layout.Child, geometry.Rectangle) {}

func TestListRowGroupUniformRowsWidthContentVisibilityAndMembership(t *testing.T) {
	listRowWithoutTypography(t)
	rows := listRowGroupRows(20, 20)
	for _, row := range rows {
		row.SetPadding(geometry.Insets{})
	}
	wrap := new(gui.WidgetBase)
	wrap.SetLayoutManager(listRowUniformWrapping{width: 300, lineHeight: 20})
	rows[1].SetSuffix(wrap)
	group := listRowGroupWith(rows...)
	group.SetUniformRows(true)
	parent := gui.NewLinearBox(layout.DirectionVertical)
	parent.SetCrossAlign(layout.CrossStretch)
	parent.AddChild(group)
	measure := func(width, height float32) {
		t.Helper()
		if got := parent.Measure(layout.Loose(geometry.Size{Width: width, Height: layout.Inf})); got.Height != height {
			t.Fatalf("宽度 %g：高度=%g，期望 %g", width, got.Height, height)
		}
		parent.Arrange(geometry.Rect(0, 0, width, height))
	}
	measure(300, 40)
	measure(100, 120)
	if rows[0].Rect().Height != 60 || rows[1].Rect().Height != 60 {
		t.Fatal("Arrange 没有按最终宽度重新计算折行高度")
	}
	// 不先 Measure，直接以另一宽度 Arrange 也必须采用新的折行高度。
	group.Arrange(geometry.Rect(0, 0, 300, 120))
	if rows[0].Rect().Height != 20 || rows[1].Rect().Height != 20 {
		t.Fatal("排布复用了旧宽度的统一高度")
	}
	wrap.SetLayoutManager(listRowUniformWrapping{width: 600, lineHeight: 20})
	measure(100, 240)
	rows[1].SetVisible(false)
	measure(100, 20)
	rows[1].SetVisible(true)
	measure(100, 240)
	group.RemoveChild(rows[1])
	measure(100, 20)
	group.AddChild(rows[1])
	group.MoveChildBefore(rows[1], rows[0])
	measure(100, 240)
	if rows[1].Rect().Y != 0 || rows[0].Rect().Y != 120 {
		t.Fatal("重排后分配位置不正确")
	}
	group.SetUniformRows(false)
	measure(100, 140)
	rows[0].SetVisible(false)
	rows[1].SetVisible(false)
	group.SetUniformRows(true)
	measure(100, 0)
	group.RemoveChild(rows[0])
	group.RemoveChild(rows[1])
	measure(100, 0)
}

func TestListRowGroupUniformRowsUnboundedWidth(t *testing.T) {
	listRowWithoutTypography(t)
	row := listRowGroupRows(20)[0]
	row.SetPadding(geometry.Insets{})
	child := new(gui.WidgetBase)
	child.SetLayoutManager(listRowUniformWrapping{width: 300, lineHeight: 20})
	row.SetSuffix(child)
	group := listRowGroupWith(row)
	group.SetUniformRows(true)
	if got := group.Measure(layout.Unbounded()); got.Width != 300 || got.Height != 20 {
		t.Fatal("无界宽度被错误地收缩", got)
	}
	c := layout.Unbounded()
	c.Min.Width = 100
	if got := group.Measure(c); got.Width != 100 || got.Height != 60 {
		t.Fatal("没有按 CrossStretch 的有限最小宽度测量", got)
	}
}

func TestListRowGroupUniformRowsHitAndSnapshotBounds(t *testing.T) {
	listRowWithoutTypography(t)
	rows := listRowGroupRows(48, 80)
	group := listRowGroupWith(rows...)
	group.SetUniformRows(true)
	group.Arrange(geometry.Rect(0, 0, 100, 200))
	activated := [2]int{}
	for i, row := range rows {
		row.SetActivatable(true)
		row.ConnectActivated(func() { activated[i]++ })
	}
	info := group.Snapshot()
	if info.Role != gui.RoleVBox || len(info.Actions) != 0 || info.Focusable || len(info.Children) != 2 {
		t.Fatal("统一行高改变了分组语义", info)
	}
	for i, child := range info.Children {
		if child.Bounds != geometry.Rect(0, float32(i)*80, 100, 80) || child.Role != gui.RoleListItem || len(child.Actions) != 1 {
			t.Fatal("行的语义边界或动作没有采用实际分配", child)
		}
	}
	host, dispatcher := &tabInputHost{root: group}, new(gui.EventDispatcher)
	for _, y := range []float32{70, 79.5, 80, 150, 180} {
		dispatchTabPointer(t, dispatcher, host, events.PointerDown, 20, y)
		dispatchTabPointer(t, dispatcher, host, events.PointerUp, 20, y)
	}
	if activated != [2]int{2, 2} {
		t.Fatal("加高区域、共享边界或尾部留白的命中错误", activated)
	}
	group.Arrange(geometry.Rect(10, 20, 100, 200))
	if got := rows[1].Snapshot().Bounds; got != geometry.Rect(10, 100, 100, 80) {
		t.Fatal("窗口局部语义边界丢失父级偏移", got)
	}
}
