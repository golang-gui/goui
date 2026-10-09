package main

import (
	"go/ast"
	"go/parser"
	"go/token"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"testing"

	"github.com/golang-gui/goui/core/geometry"
	"github.com/golang-gui/goui/example/demo/resource"
	"github.com/golang-gui/goui/gui"
	"github.com/golang-gui/goui/layout"
	"github.com/golang-gui/goui/mlang"
	"github.com/golang-gui/goui/platform/graphics"
	"github.com/golang-gui/goui/ui"
	"github.com/golang-gui/goui/widgets"
	wui "github.com/golang-gui/goui/widgets/ui"
)

// 私有 demo-card 规则已移除，遗漏的样式引用会让浮动面板和 Dialog 透明。
// 只检查样式调用，不把同名的应用内拖放数据格式当作样式。
func TestDemoCardStyleMigration(t *testing.T) {
	files, err := filepath.Glob("*.go")
	if err != nil {
		t.Fatal(err)
	}
	for _, file := range files {
		if strings.HasSuffix(file, "_test.go") {
			continue
		}
		positions := token.NewFileSet()
		source, err := parser.ParseFile(positions, file, nil, 0)
		if err != nil {
			t.Fatal(err)
		}
		ast.Inspect(source, func(node ast.Node) bool {
			call, ok := node.(*ast.CallExpr)
			if !ok || len(call.Args) != 1 {
				return true
			}
			method, ok := call.Fun.(*ast.SelectorExpr)
			if !ok || (method.Sel.Name != "Style" && method.Sel.Name != "SetStyleName") {
				return true
			}
			if literal, ok := call.Args[0].(*ast.BasicLit); ok && literal.Kind == token.STRING {
				if name, err := strconv.Unquote(literal.Value); err == nil && name == "demo-card" {
					t.Errorf("%s: removed demo-card style; use modern.Card", positions.Position(literal.Pos()))
				}
			}
			return true
		})
	}
}

func TestLanguageChangePreservesDemoModelsAndState(t *testing.T) {
	previous := mlang.Language()
	t.Cleanup(func() { _ = mlang.SetLanguage(previous) })
	d, err := newDemo()
	if err != nil {
		t.Fatal(err)
	}
	d.app = new(demoTestApp)
	defer d.stopActivities()
	choices, tree, list, table, model := d.choices, d.tree, d.list, d.table, d.text
	d.input.Set("user input")
	d.text.SetText("user editor text")
	d.selection = ui.TextSelection{Anchor: 1, Caret: 5}
	d.drop, d.fancyDrop, d.page = 3, 1, 1
	d.tableWidths[0] = 257
	d.table.setNote("file-000", "edited note")
	d.table.sort("size", widgets.SortDescending)
	firstRow := d.table.RowID(0)
	d.documents["document-1"].SetText("edited document")
	d.tabWindows[0].current = "document-2"
	canUndo := model.CanUndo()
	listener := mlang.ConnectChanged(d.translateUI)
	defer listener.Disconnect()
	for _, choice := range demoLanguages {
		if err := mlang.SetLanguage(choice.tag); err != nil {
			t.Fatal(err)
		}
		d.translateUI()
		if choices != d.choices || tree != d.tree || list != d.list || table != d.table || model != d.text {
			t.Fatal("language change replaced a persistent model")
		}
		if d.input.Get() != "user input" || model.Text() != "user editor text" || model.CanUndo() != canUndo ||
			d.selection != (ui.TextSelection{Anchor: 1, Caret: 5}) || d.page != 1 || d.drop != 3 || d.fancyDrop != 1 ||
			d.tableWidths[0] != 257 || d.table.RowID(0) != firstRow || d.documents["document-1"].Text() != "edited document" ||
			d.tabWindows[0].current != "document-2" {
			t.Fatal("translation overwrote user state")
		}
		for i := 0; i < table.ItemsCount(); i++ {
			if row := table.ItemAt(i); row.ID == "file-000" && row.Note != "edited note" {
				t.Fatal("translation overwrote table edit")
			}
		}
		if choices.ItemAt(0).Text != resource.DefaultOption() || !choices.ItemAt(2).Disabled {
			t.Fatal("presentation not updated or disabled state lost")
		}
		for i, getter := range pageNames {
			if getter() == "" {
				t.Fatalf("page %d has no translated name", i)
			}
		}
	}
}

// 这些测试只覆盖例程自己的内存数据和布局适配，不创建窗口/事件循环。
func TestInitialModelsAndAssets(t *testing.T) {
	d, err := newDemo()
	if err != nil {
		t.Fatal(err)
	}
	defer d.motion.Stop()
	if d.list.ItemsCount() != 1000 || d.table.ItemsCount() != 120 || len(d.tabWindows) != 1 {
		t.Fatal("unexpected initial model sizes")
	}
	if !slices.Equal(d.tree.Children("src"), []string{"main", "view"}) {
		t.Fatal("missing tree children")
	}
	first := d.documents["document-1"]
	second := d.documents["document-2"]
	first.SetText("edited")
	if first == second || second.Text() == "edited" {
		t.Fatal("documents share mutable state")
	}
	if d.motion.Running() || d.busy || d.tickTimer != nil {
		t.Fatal("demo must start idle")
	}
	for i, source := range d.icons {
		img := source.Image(32, 32, graphics.RGB(100, 150, 200))
		if img == nil || img.Bounds().Dx() != 32 || img.Bounds().Dy() != 32 {
			t.Fatalf("icon %d: invalid image", i)
		}
		visible := false
		for y := 0; y < 32; y++ {
			for x := 0; x < 32; x++ {
				_, _, _, a := img.At(x, y).RGBA()
				visible = visible || a > 0
			}
		}
		if !visible {
			t.Fatalf("icon %d: blank image", i)
		}
	}
}

func TestTabOrderAndTreeExpansion(t *testing.T) {
	ids := []string{"a", "b", "c"}
	for _, test := range []struct {
		id    string
		index int
		want  []string
	}{{"a", 2, []string{"b", "c", "a"}}, {"c", 0, []string{"c", "a", "b"}}, {"d", 99, []string{"a", "b", "c", "d"}}, {"b", -1, []string{"b", "a", "c"}}} {
		if got := moveID(ids, test.id, test.index); !slices.Equal(got, test.want) {
			t.Fatalf("move %s: %v, want %v", test.id, got, test.want)
		}
	}
	if !slices.Equal(ids, []string{"a", "b", "c"}) {
		t.Fatal("move changed caller-owned slice")
	}
	if got := setMember(ids, "b", false); !slices.Equal(got, []string{"a", "c"}) {
		t.Fatal(got)
	}
	if got := setMember(ids, "a", true); len(got) != 3 {
		t.Fatal("duplicate expansion ID")
	}
	if got := setMember(ids, "d", true); !slices.Equal(got, []string{"a", "b", "c", "d"}) {
		t.Fatal(got)
	}
}

type demoTestApp struct {
	ui.App
	updates int
}

func (a *demoTestApp) RequestUpdate() { a.updates++ }

func TestTabCloseConfirmationBelongsToCurrentWindow(t *testing.T) {
	d, err := newDemo()
	if err != nil {
		t.Fatal(err)
	}
	d.app = new(demoTestApp)
	main := d.tabWindows[0]
	target := &tabWindow{id: "target", pages: []string{"document-2"}, current: "document-2"}
	main.pages = []string{"document-1", "document-3"}
	d.tabWindows = append(d.tabWindows, target)
	d.page = 1
	d.requestTabClose(target, "document-2")
	if target.closing != "document-2" || main.closing != "" || d.page != 1 {
		t.Fatal("close request escaped its workspace")
	}
	d.closeTab(target)
	if target.closing != "" || len(target.pages) != 0 || d.documents["document-2"] != nil || len(main.pages) != 2 {
		t.Fatal("confirmation closed the wrong page")
	}
	d.requestTabClose(target, "document-1")
	if target.closing != "" {
		t.Fatal("accepted a page owned by another window")
	}
}

func TestTabTransferCancelsSourceConfirmation(t *testing.T) {
	d, err := newDemo()
	if err != nil {
		t.Fatal(err)
	}
	d.app = new(demoTestApp)
	source := d.tabWindows[0]
	target := &tabWindow{id: "target"}
	d.tabWindows = append(d.tabWindows, target)
	d.requestTabClose(source, "document-2")
	d.transferTab(wui.TabTransfer{SourceWindow: source.id, TargetWindow: target.id, PageID: "document-2", Index: 0})
	d.closeTab(source) // A stale confirmation must not remove the moved model.
	if source.closing != "" || target.current != "document-2" || d.documents["document-2"] == nil {
		t.Fatal("transfer left a stale confirmation")
	}
}

func TestDetachedTabUsesTargetControlsOccupancy(t *testing.T) {
	for _, test := range []struct {
		info        gui.ChromeInfo
		inset, want float32
	}{
		{gui.ChromeInfo{Enabled: true, Controls: gui.ChromeControlsNative, ControlsBounds: geometry.Rect(12, 8, 68, 14)}, 0, 80},
		{gui.ChromeInfo{Enabled: true, Controls: gui.ChromeControlsNative, ControlsBounds: geometry.Rect(22, 8, 78, 14)}, 5, 95},
		{gui.ChromeInfo{Enabled: true, Controls: gui.ChromeControlsCustom, ControlsBounds: geometry.Rect(500, 8, 80, 14)}, 5, 0},
		{gui.ChromeInfo{}, 0, 0},
	} {
		if got := tabControlsInset(test.info, test.inset); got != test.want {
			t.Fatalf("controls inset=%g, want %g", got, test.want)
		}
	}
}

func TestPageGutterIsOutsideScrollViewport(t *testing.T) {
	for _, origin := range []float32{0, 5} {
		scroll := gui.NewScrollView()
		view := page("gutter", "gutter", "test").(*ui.BoxView)
		box := view.Mount(nil).(*gui.LinearBox)
		view.Update(bodyContext{children: []gui.Widget{scroll}}, box)
		box.Measure(layout.Tight(geometry.Size{Width: 500, Height: 300}))
		box.Arrange(geometry.Rect(origin, origin, 500, 300))
		if scroll.Rect() != geometry.Rect(8, 8, 484, 284) {
			t.Fatalf("viewport gutter=%v", scroll.Rect())
		}
		if scroll.Snapshot().Bounds != geometry.Rect(origin+8, origin+8, 484, 284) {
			t.Fatal("gutter lost window-local semantics")
		}
	}
}

func TestRecordSortAndEditUseStableIDs(t *testing.T) {
	m := newRecordModel()
	m.sort("size", widgets.SortDescending)
	for i := 1; i < m.ItemsCount(); i++ {
		if m.ItemAt(i-1).Size < m.ItemAt(i).Size {
			t.Fatal("sizes not descending")
		}
	}
	id := m.RowID(0)
	m.setNote(id, "kept after reorder")
	m.sort("name", widgets.SortAscending)
	found := false
	for i := 0; i < m.ItemsCount(); i++ {
		r := m.ItemAt(i)
		if r.ID == id {
			found = true
			if r.Note != "kept after reorder" {
				t.Fatal("edit applied to index rather than row ID")
			}
		} else if r.Note != "可编辑备注" {
			t.Fatal("another row was edited")
		}
	}
	if !found {
		t.Fatal("row lost during sorting")
	}
	m.sort("name", widgets.SortNone)
	if m.RowID(0) != "file-000" {
		t.Fatal("SortNone does not restore initial order")
	}
}

type layoutProbe struct {
	gui.WidgetBase
	constraint layout.Constraint
}

func (w *layoutProbe) Measure(c layout.Constraint) layout.Measurement {
	w.constraint = c
	return layout.Measured(geometry.Size{Width: c.Max.Width, Height: 450})
}

func TestFormScrollViewConstrainsWidthAndScrollsHeight(t *testing.T) {
	w := gui.NewScrollView()
	w.SetDirection(gui.ScrollVertical)
	child := new(layoutProbe)
	w.SetChild(child)
	w.Measure(layout.Loose(geometry.Size{Width: 500, Height: 300}))
	if child.constraint.Max.Width != 500 || child.constraint.Max.Height != layout.Inf {
		t.Fatal("form children need finite width and unbounded height")
	}
	w.Arrange(geometry.Rect(0, 0, 480, 300))
	w.SetScrollY(70)
	if got := child.Rect(); got != geometry.Rect(0, -70, 472, 450) {
		t.Fatalf("arranged %v", got)
	}
	if info := w.Snapshot(); info.Role != gui.RoleScrollView || len(info.Children) != 1 || info.MaxScrollY != 150 || info.MaxScrollX != 0 {
		t.Fatal("form omitted semantic subtree or exposed an incorrect range")
	}
}

// 只给 BoxView 提供已构造的测试孩子；布局属性由例程真实声明的 Update 应用。
// 不重建声明式协调器，不启动 Application 或文字排版。
type bodyContext struct {
	ui.BuildContext
	children []gui.Widget
}

func (ctx bodyContext) UpdateChildren(target ui.Container, views []ui.View) []gui.Widget {
	if len(views) != len(ctx.children) {
		panic("body declaration and test children differ")
	}
	for _, child := range ctx.children {
		target.AddChild(child)
	}
	return ctx.children
}

func TestBodyUsesAllocatedContentEdges(t *testing.T) {
	// 由宿主决定可用区域；例程不读取 Chrome 私有的边缘宽度，也不重复留白。
	// 同时覆盖无偏移分配与非零内容起点，后者的 Snapshot 仍须使用窗口坐标。
	for _, test := range []struct {
		name   string
		bounds geometry.Rectangle
	}{
		{"narrow-full", geometry.Rect(0, 0, 820, 600)},
		{"narrow-inset", geometry.Rect(5, 5, 810, 590)},
		{"wide-full", geometry.Rect(0, 0, 1120, 800)},
		{"wide-inset", geometry.Rect(5, 5, 1110, 790)},
	} {
		t.Run(test.name, func(t *testing.T) {
			side := new(surfaceWidget)
			side.SetLayoutManager(layout.NewFillLayout())
			side.SetChild(gui.NewLinearBox(layout.DirectionVertical))
			side.SetMinSize(geometry.Size{Width: 190})
			side.SetMaxSize(geometry.Size{Width: 190})
			content := gui.NewLinearBox(layout.DirectionVertical)
			content.SetMinSize(geometry.Size{Width: 200, Height: 2000})
			scroll := gui.NewScrollView()
			scroll.SetChild(content)
			view := demoBody(ui.VBox(), ui.VBox())
			body := view.Mount(nil).(*gui.LinearBox)
			view.Update(bodyContext{children: []gui.Widget{side, scroll}}, body)
			body.Measure(layout.Tight(test.bounds.Size))
			body.Arrange(test.bounds)
			if got := side.Rect(); got != geometry.Rect(0, 0, 190, test.bounds.Height) {
				t.Fatalf("navigation allocation %v adds unexpected outer padding", got)
			}
			if got := scroll.Rect(); got != geometry.Rect(190, 0, test.bounds.Width-190, test.bounds.Height) {
				t.Fatalf("scroll allocation %v does not fill the remaining content area", got)
			}
			var bar *gui.ScrollBar
			for _, child := range scroll.Children() {
				if b, ok := child.(*gui.ScrollBar); ok && b.Orientation() == layout.DirectionVertical {
					bar = b
				}
			}
			if bar == nil || !bar.Visible() || bar.Rect().Width != 8 {
				t.Fatal("expected a full 8 DIP vertical scrollbar")
			}
			want := geometry.Rect(test.bounds.X+test.bounds.Width-8, test.bounds.Y, 8, test.bounds.Height)
			if got := bar.Snapshot().Bounds; got != want {
				t.Fatalf("scrollbar bounds %v, want %v at the allocated right edge", got, want)
			}
			// 检查左右两个边缘都能命中完整滚动条，而不只是中心可点。
			for _, x := range []float32{test.bounds.Width - 7.5, test.bounds.Width - .5} {
				if got := gui.Pick(body, geometry.Point{X: x, Y: test.bounds.Height / 2}); got != bar {
					t.Fatalf("scrollbar edge x=%v picked %T, want the scrollbar", x, got)
				}
			}
		})
	}
}
