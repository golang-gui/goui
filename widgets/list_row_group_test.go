package widgets

import (
	"image"
	"image/color"
	"math"
	"testing"

	"github.com/golang-gui/goui/core/geometry"
	"github.com/golang-gui/goui/gui"
	"github.com/golang-gui/goui/layout"
	"github.com/golang-gui/goui/platform/events"
	"github.com/golang-gui/goui/platform/graphics"
	"github.com/golang-gui/goui/platform/graphics/software"
	"github.com/golang-gui/goui/style"
)

// 固定高度的空行隔离字体测量，只验证分组自身的布局与连接几何。
func listRowGroupRows(heights ...float32) []*ListRow {
	rows := make([]*ListRow, len(heights))
	for i, height := range heights {
		r := NewListRow("")
		r.SetMinSize(geometry.Size{Height: height})
		rows[i] = r
	}
	return rows
}

func listRowGroupWith(rows ...*ListRow) *ListRowGroup {
	group := NewListRowGroup()
	for _, r := range rows {
		group.AddChild(r)
	}
	return group
}

func TestListRowGroupLayoutAndSnapshot(t *testing.T) {
	listRowWithoutTypography(t)
	rows := listRowGroupRows(48, 60, 40)
	group := listRowGroupWith(rows...)
	if m := group.Measure(layout.Loose(geometry.Size{Width: 80, Height: layout.Inf})); m.Height != 148 || m.Width != 80 {
		t.Fatalf("分组应无间距地堆叠行：%v", m)
	}
	group.Arrange(geometry.Rect(0, 0, 80, 148))
	offset := float32(0)
	for i, r := range rows {
		if b := r.Rect(); b.X != 0 || b.Y != offset || b.Width != 80 || b.Height != r.MinSize().Height {
			t.Fatalf("第 %d 行未拉伸到分组宽度或残留间距：%v", i, b)
		}
		offset += r.Rect().Height
	}
	info := group.Snapshot()
	if info.Role != gui.RoleVBox || len(info.Actions) != 0 || group.Focusable() || len(info.Children) != 3 {
		t.Fatalf("分组角色/动作错误：%+v", info)
	}
	for _, child := range info.Children {
		if child.Role != gui.RoleListItem {
			t.Fatal("分组改变了行的语义角色")
		}
	}
	// 隐藏中间行不留空位，也不留下无人绘制的接缝。
	rows[1].SetVisible(false)
	group.Arrange(geometry.Rect(0, 0, 80, 148))
	if rows[2].Rect().Y != 48 || rows[0].Rect().Height != 48 {
		t.Fatalf("隐藏行仍占位：%v %v", rows[0].Rect(), rows[2].Rect())
	}
	if segment := listRowSegmentOf(rows[0]); !segment.grouped || !segment.shared {
		t.Fatal("隐藏行之后的首行没有与下一行共享接缝")
	}
	if segment := listRowSegmentOf(rows[2]); !segment.grouped || segment.corners&listRowTopLeft != 0 {
		t.Fatal("隐藏行之后的末行没有内收上角")
	}
	if segment := listRowSegmentOf(rows[1]); segment.grouped {
		t.Fatal("隐藏行不应参与连接")
	}
}

func TestListRowGroupSegments(t *testing.T) {
	listRowWithoutTypography(t)
	rows := listRowGroupRows(48, 48, 48)
	group := listRowGroupWith(rows...)
	group.Arrange(geometry.Rect(0, 0, 80, 144))
	standalone := NewListRow("")
	cases := []struct {
		name    string
		row     *ListRow
		grouped bool
		shared  bool
		corners listRowCorners
	}{
		{"分组外独立行", standalone, false, false, listRowAllCorners},
		{"首行", rows[0], true, true, listRowTopLeft | listRowTopRight},
		{"中间行", rows[1], true, true, 0},
		{"末行", rows[2], true, false, listRowBottomLeft | listRowBottomRight},
	}
	for _, tc := range cases {
		segment := listRowSegmentOf(tc.row)
		if segment.grouped != tc.grouped || segment.shared != tc.shared || segment.corners != tc.corners {
			t.Fatalf("%s：%+v", tc.name, segment)
		}
	}
	// 只有相邻的可见行相连：非行内容截断连接，两侧各自保留完整轮廓。
	blocker := gui.NewLabel("section")
	group.AddChild(blocker)
	group.MoveChildBefore(blocker, rows[2])
	group.Arrange(geometry.Rect(0, 0, 80, 144))
	// 截断点之前的末行只保留与上一行共享的上边，下边恢复完整。
	if segment := listRowSegmentOf(rows[1]); !segment.grouped || segment.shared || segment.corners != listRowBottomLeft|listRowBottomRight {
		t.Fatalf("截断点之前的末行几何错误：%+v", segment)
	}
	if segment := listRowSegmentOf(rows[2]); segment.grouped {
		t.Fatal("非行内容之后的末行仍共享接缝")
	}
	if segment := listRowSegmentOf(rows[0]); !segment.grouped || !segment.shared || segment.corners != listRowTopLeft|listRowTopRight {
		t.Fatalf("截断点之前的首行几何错误：%+v", segment)
	}
	// 单行分组不连接，也不删除自己的边框和圆角。
	single := listRowGroupRows(48)
	singleGroup := listRowGroupWith(single[0])
	singleGroup.Arrange(geometry.Rect(0, 0, 80, 48))
	if segment := listRowSegmentOf(single[0]); segment.grouped || segment.corners != listRowAllCorners {
		t.Fatalf("单行分组不应改变外观：%+v", segment)
	}
}

func TestListRowGroupSegmentGeometry(t *testing.T) {
	radius := geometry.UniformCornerRadius(8)
	size := geometry.Size{Width: 80, Height: 48}
	middle := listRowSegment{grouped: true, shared: true, corners: 0}
	if got := middle.radius(radius, size); got != (geometry.CornerRadius{}) {
		t.Fatalf("内行应使用直角：%v", got)
	}
	first := listRowSegment{grouped: true, shared: true, corners: listRowTopLeft | listRowTopRight}
	if got := first.radius(radius, size); got != (geometry.CornerRadius{TopLeft: 8, TopRight: 8}) {
		t.Fatalf("首行只保留上圆角：%v", got)
	}
	if got := middle.radius(radius, geometry.Size{Width: 4, Height: 4}); got != (geometry.CornerRadius{}) {
		t.Fatal("归一化后仍残留圆角")
	}
	// 未声明 BorderSides 与显式 BorderAll 等效；只有非末行交出底边。
	unset := style.Name("list-row").BorderWidth(1).Style
	all := style.Name("list-row").BorderWidth(1).BorderSides(style.BorderAll).Style
	partial := style.Name("list-row").BorderWidth(1).BorderSides(style.BorderTop | style.BorderBottom).Style
	if got := middle.sides(unset); got != style.BorderTop|style.BorderRight|style.BorderLeft {
		t.Fatalf("共享接缝未交出底边：%v", got)
	}
	if got := middle.sides(all); got != style.BorderTop|style.BorderRight|style.BorderLeft {
		t.Fatalf("显式 BorderAll 未交出底边：%v", got)
	}
	if got := middle.sides(partial); got != style.BorderTop {
		t.Fatalf("显式边集未按位移除底边：%v", got)
	}
	last := listRowSegment{grouped: true, corners: listRowBottomLeft | listRowBottomRight}
	if got := last.sides(unset); got != style.BorderAll {
		t.Fatalf("末行应保留完整边框：%v", got)
	}
	// 焦点框是完整轮廓：重复共享边，但不恢复被相邻行占用的圆角。
	if got := middle.complete(); got.shared || got.corners != 0 {
		t.Fatalf("焦点几何错误：%+v", got)
	}
	if got := middle.complete().sides(unset); got != style.BorderAll {
		t.Fatalf("焦点轮廓未闭合：%v", got)
	}
}

// 只转发 ListRow.Paint 实际使用的调用；子树遍历与状态隔离由 GUI 内核用例覆盖。
type listRowGroupPixelPainter struct {
	gui.Painter
	native graphics.Painter
}

func (p listRowGroupPixelPainter) SnapRect(r geometry.Rectangle) geometry.Rectangle {
	return p.native.SnapRect(r)
}
func (p listRowGroupPixelPainter) SetTransform(t geometry.Transform) { p.native.SetTransform(t) }
func (p listRowGroupPixelPainter) FillRect(r geometry.Rectangle, b graphics.Brush) {
	p.native.FillRect(r, b)
}
func (p listRowGroupPixelPainter) FillRoundRect(r geometry.Rectangle, radius float32, b graphics.Brush) {
	p.native.FillRoundRect(r, radius, b)
}
func (p listRowGroupPixelPainter) FillPath(path graphics.Path, b graphics.Brush) {
	p.native.FillPath(path, b)
}
func (p listRowGroupPixelPainter) DrawRect(r geometry.Rectangle, width float32, b graphics.Brush) {
	p.native.DrawRect(r, width, b)
}
func (p listRowGroupPixelPainter) DrawRoundRect(r geometry.Rectangle, radius, width float32, b graphics.Brush) {
	p.native.DrawRoundRect(r, radius, width, b)
}
func (p listRowGroupPixelPainter) DrawPath(path graphics.Path, width float32, b graphics.Brush) {
	p.native.DrawPath(path, width, b)
}

// 逐行按自身窗口位置平移后绘制，等价于内核进入每个子控件时的局部坐标；
// 这里不复制遍历、裁剪或样式解析。
func listRowGroupPixels(t *testing.T, group *ListRowGroup, scale float32) *image.RGBA {
	t.Helper()
	size := group.Rect().Size
	surface := new(progressPixelSurface)
	native, err := software.NewPainter(surface)
	if err != nil {
		t.Fatal(err)
	}
	defer native.Destroy()
	native.Begin(size.Width*scale, size.Height*scale, scale)
	native.Clear(graphics.Color{})
	func() {
		defer native.End()
		for _, child := range group.Children() {
			row, ok := child.(*ListRow)
			if !ok || !row.Visible() {
				continue
			}
			native.SetTransform(geometry.Translate(row.Rect().X, row.Rect().Y))
			row.Paint(listRowGroupPixelPainter{native: native})
		}
	}()
	return surface.pixels
}

func TestListRowGroupSeamPixels(t *testing.T) {
	// 预乘 sRGB RGBA8：不透明白底上的半透明红边一次覆盖约 (255,127,127,255)，
	// 若相邻行重复绘制同一条边会加深到约 (255,63,63,255)。允许 2 字节误差，
	// 覆盖抗锯齿覆盖率与合入取整。
	useProgressApplication(t, &progressTestApplication{sheet: style.Sheet(
		style.Name("list-row").BackgroundColor(color.White).BorderColor(color.NRGBA{R: 255, A: 128}).BorderWidth(2).Radius(8),
	)})
	rows := listRowGroupRows(48, 48, 48)
	group := listRowGroupWith(rows...)
	group.Arrange(geometry.Rect(0, 0, 80, 144))
	for _, scale := range []float32{1, 2} {
		img := listRowGroupPixels(t, group, scale)
		at := func(x, y int) color.RGBA {
			return color.RGBAModel.Convert(img.At(x*int(scale), y*int(scale))).(color.RGBA)
		}
		near := func(got color.RGBA, r, g, b, a uint8) bool {
			for i, v := range []uint8{got.R, got.G, got.B, got.A} {
				want := []uint8{r, g, b, a}[i]
				if delta := int(v) - int(want); delta < -2 || delta > 2 {
					return false
				}
			}
			return true
		}
		for _, sample := range []struct {
			name string
			x, y int
			want [4]uint8
		}{
			{"首行上外边框", 40, 0, [4]uint8{255, 127, 127, 255}},
			{"共享接缝只画一次", 40, 48, [4]uint8{255, 127, 127, 255}},
			{"第二条共享接缝", 40, 96, [4]uint8{255, 127, 127, 255}},
			{"末行下外边框", 40, 143, [4]uint8{255, 127, 127, 255}},
			{"首行左上外圆角透明", 0, 0, [4]uint8{0, 0, 0, 0}},
			{"末行左下外圆角透明", 0, 143, [4]uint8{0, 0, 0, 0}},
			{"内角为直角且有边框", 0, 48, [4]uint8{255, 127, 127, 255}},
			{"内角另一侧为直角", 79, 48, [4]uint8{255, 127, 127, 255}},
			{"行内部为表面色", 40, 24, [4]uint8{255, 255, 255, 255}},
		} {
			if got := at(sample.x, sample.y); !near(got, sample.want[0], sample.want[1], sample.want[2], sample.want[3]) {
				t.Fatalf("%gx %s (%d,%d)=%v，预期 %v", scale, sample.name, sample.x, sample.y, got, sample.want)
			}
		}
	}
	// 独立行仍保留四角圆角与完整边框，不受分组几何影响。
	alone := NewListRow("")
	alone.SetMinSize(geometry.Size{Height: 48})
	alone.Arrange(geometry.Rect(0, 0, 80, 48))
	surface := new(progressPixelSurface)
	native, err := software.NewPainter(surface)
	if err != nil {
		t.Fatal(err)
	}
	defer native.Destroy()
	native.Begin(80, 48, 1)
	native.Clear(graphics.Color{})
	alone.Paint(listRowGroupPixelPainter{native: native})
	native.End()
	for _, sample := range []struct {
		point image.Point
		want  color.RGBA
	}{
		{image.Point{0, 0}, color.RGBA{}},
		{image.Point{79, 47}, color.RGBA{}},
		{image.Point{79, 0}, color.RGBA{}},
		{image.Point{40, 47}, color.RGBA{R: 255, G: 127, B: 127, A: 255}},
		{image.Point{40, 24}, color.RGBA{R: 255, G: 255, B: 255, A: 255}},
	} {
		got := color.RGBAModel.Convert(surface.pixels.At(sample.point.X, sample.point.Y)).(color.RGBA)
		if got != sample.want {
			t.Fatalf("独立行 %v=%v，预期 %v", sample.point, got, sample.want)
		}
	}
}

func TestListRowGroupInputIsolation(t *testing.T) {
	listRowWithoutTypography(t)
	rows := listRowGroupRows(48, 48)
	group := listRowGroupWith(rows...)
	group.Arrange(geometry.Rect(0, 0, 80, 96))
	for _, r := range rows {
		r.SetActivatable(true)
	}
	activated := [2]int{}
	for i, r := range rows {
		r.ConnectActivated(func() { activated[i]++ })
	}
	h := &tabInputHost{root: group}
	d := new(gui.EventDispatcher)
	click := func(y float32) {
		t.Helper()
		dispatchTabPointer(t, d, h, events.PointerDown, 20, y)
		dispatchTabPointer(t, d, h, events.PointerUp, 20, y)
	}
	click(20)
	click(70)
	if activated != [2]int{1, 1} {
		t.Fatalf("分组改变了行的点击归属：%v", activated)
	}
	// 悬停只作用于命中的那一行，不通过分组扩散到邻居。
	if err := d.DispatchEvent(h, events.PointerEvent{EventType: events.PointerMove, Position: geometry.Point{X: 20, Y: 70}}); err != nil {
		t.Fatal(err)
	}
	if rows[0].State().Has(style.Hovered) || !rows[1].State().Has(style.Hovered) {
		t.Fatal("悬停跨越了行边界")
	}
	if group.Focused() || group.ContainsFocus() != rows[1].ContainsFocus() {
		t.Fatal("分组自身不应获得焦点状态")
	}
}

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
