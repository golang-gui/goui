package main

import (
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"image"
	"image/color"
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
	"github.com/golang-gui/goui/theme/modern"
	"github.com/golang-gui/goui/ui"
	"github.com/golang-gui/goui/widgets"
	wui "github.com/golang-gui/goui/widgets/ui"
)

// 校验按钮实际传入的创建选项；不为例程测试创建原生窗口或模拟 Application。
func TestAuxWindowExampleDeclarations(t *testing.T) {
	source, err := parser.ParseFile(token.NewFileSet(), "windows.go", nil, 0)
	if err != nil {
		t.Fatal(err)
	}
	expression := func(expr ast.Expr) string {
		switch value := expr.(type) {
		case *ast.Ident:
			return value.Name
		case *ast.SelectorExpr:
			if pkg, ok := value.X.(*ast.Ident); ok {
				return pkg.Name + "." + value.Sel.Name
			}
		}
		return ""
	}
	for _, test := range []struct {
		id     string
		fields map[string]string
		frame  string
	}{
		{"skip-taskbar-window", map[string]string{"Chrome": "gui.WindowChromeNative", "SkipTaskbar": "true"}, "false"},
		{"border-box-window", map[string]string{"Chrome": "gui.WindowChromeNone", "Transparent": "true"}, "true"},
	} {
		t.Run(test.id, func(t *testing.T) {
			found := false
			ast.Inspect(source, func(node ast.Node) bool {
				call, ok := node.(*ast.CallExpr)
				if !ok {
					return true
				}
				method, ok := call.Fun.(*ast.SelectorExpr)
				if !ok || method.Sel.Name != "OnClick" || !demoHasCall(method.X, "ID", strconv.Quote(test.id)) {
					return true
				}
				ast.Inspect(call, func(node ast.Node) bool {
					invoke, ok := node.(*ast.CallExpr)
					if !ok {
						return true
					}
					method, ok := invoke.Fun.(*ast.SelectorExpr)
					if !ok || method.Sel.Name != "openWindow" || len(invoke.Args) != 2 {
						return true
					}
					options, ok := invoke.Args[0].(*ast.CompositeLit)
					if !ok || expression(options.Type) != "gui.WindowOptions" {
						t.Fatal("example must explicitly pass GUI window options")
					}
					got := make(map[string]string)
					for _, field := range options.Elts {
						if field, ok := field.(*ast.KeyValueExpr); ok {
							got[expression(field.Key)] = expression(field.Value)
						}
					}
					if len(got) != len(test.fields) || expression(invoke.Args[1]) != test.frame {
						t.Fatalf("options=%v BorderBox=%s", got, expression(invoke.Args[1]))
					}
					for field, want := range test.fields {
						if got[field] != want {
							t.Errorf("%s=%s, want %s", field, got[field], want)
						}
					}
					found = true
					return false
				})
				return false
			})
			if !found {
				t.Fatal("missing actionable window example")
			}
		})
	}
}

// 检查例程真实构造的根；BorderBox 不能被透明样例的外层容器包住。
// 文字排版及原生命中另在 README 的桌面操作中验证。
func TestAuxWindowContentComposition(t *testing.T) {
	for _, test := range []struct {
		name                   string
		transparent, borderBox bool
	}{
		{"native", false, false},
		{"transparent", true, false},
		{"border-box", true, true},
	} {
		t.Run(test.name, func(t *testing.T) {
			column := gui.NewLinearBox(layout.DirectionVertical)
			column.SetCrossAlign(layout.CrossStretch)
			header := gui.NewHeaderBar()
			column.AddChild(header)
			scroll := gui.NewScrollView()
			scroll.SetDirection(gui.ScrollVertical)
			scroll.SetChild(new(layoutProbe))
			column.AddChild(scroll)
			root := auxWindowContent(column, test.transparent, test.borderBox)
			if test.borderBox {
				frame, ok := root.(*gui.BorderBox)
				if !ok || frame.Child() != column || !frame.Resizable() || frame.ResizeWidth() != 6 || frame.StyleName() != modern.Card {
					t.Fatal("BorderBox must be root with explicit Card appearance and default resize width")
				}
				frame.Measure(layout.Tight(geometry.Size{Width: 500, Height: 380}))
				frame.Arrange(geometry.Rect(0, 0, 500, 380))
				if column.Rect() != geometry.Rect(6, 6, 488, 368) || header.Rect().Y != 0 || scroll.Rect().Y != header.Rect().Height {
					t.Fatal("root safe inset was duplicated or scrolling displaced HeaderBar")
				}
				if info := root.Snapshot(); info.Role != gui.RoleBox || len(info.Children) != 1 || info.Children[0].Bounds != column.Rect() {
					t.Fatal("window example lost its content subtree")
				}
			} else {
				var panel *surfaceWidget
				if test.transparent {
					outer, ok := root.(*gui.LinearBox)
					if !ok || outer.Padding() != 16 || len(outer.Children()) != 1 {
						t.Fatal("legacy transparent example lost its outer margin")
					}
					panel, _ = outer.Children()[0].(*surfaceWidget)
				} else {
					panel, _ = root.(*surfaceWidget)
				}
				if panel == nil || panel.Child() != column || panel.StyleName() != modern.Card {
					t.Fatal("ordinary example changed its content surface")
				}
			}
		})
	}
	if frame := gui.NewBorderBox(); frame.StyleName() != "" {
		t.Fatal("demo must not change the BorderBox constructor's invisible default")
	}
}

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

// 检查例程真实声明，不为测试导出 UI 私有状态或重建协调器。
// 实际高度、原生按钮避让及字体放大后的裁剪另按 README 做桌面验证。
func TestDemoCompactChromeDeclaration(t *testing.T) {
	source, err := parser.ParseFile(token.NewFileSet(), "main.go", nil, 0)
	if err != nil {
		t.Fatal(err)
	}
	var content *ast.CallExpr
	var mainWindow ast.Node
	var titleStyle ast.Node
	ast.Inspect(source, func(node ast.Node) bool {
		if assignment, ok := node.(*ast.AssignStmt); ok && len(assignment.Lhs) == 1 && len(assignment.Rhs) == 1 {
			if name, ok := assignment.Lhs[0].(*ast.Ident); ok && name.Name == "content" {
				for expr := assignment.Rhs[0]; ; {
					call, ok := expr.(*ast.CallExpr)
					if !ok {
						break
					}
					method, ok := call.Fun.(*ast.SelectorExpr)
					if !ok {
						break
					}
					if method.Sel.Name == "VBox" {
						content = call
						break
					}
					expr = method.X
				}
			}
		}
		if selector, ok := node.(*ast.SelectorExpr); ok && selector.Sel.Name == "KeyboardHint" {
			t.Error("sidebar must not show the verbose keyboard instructions")
		}
		if call, ok := node.(*ast.CallExpr); ok {
			if method, ok := call.Fun.(*ast.SelectorExpr); ok && method.Sel.Name == "FontFamily" && demoHasCall(call, "Name", `"demo-title"`) {
				titleStyle = call
			}
			if method, ok := call.Fun.(*ast.SelectorExpr); ok && method.Sel.Name == "Content" && demoHasCall(call, "Window", `"main"`) {
				mainWindow = call
			}
		}
		return true
	})
	if content == nil || len(content.Args) != 4 {
		t.Fatal("missing demo header/toolbar/body/status declaration")
	}
	for _, check := range []struct {
		name, method, value string
		node                ast.Node
	}{
		{"main window initial centering", "Center", "true", mainWindow},
		{"header padding", "Padding", "6", content.Args[0]},
		{"header logo", "Size", "24", content.Args[0]},
		{"header title", "Style", `"demo-title"`, content.Args[0]},
		{"title font follows settings", "FontSize", "fontSize", titleStyle},
		{"status padding", "Padding", "6", content.Args[3]},
		{"status minimum height", "MinHeight", "28", content.Args[3]},
		{"long status wraps", "WrapMode", "gui.WrapWordChar", content.Args[3]},
	} {
		if check.node == nil || !demoHasCall(check.node, check.method, check.value) {
			t.Errorf("%s: missing %s(%s)", check.name, check.method, check.value)
		}
	}
	for _, node := range []ast.Node{content.Args[0], content.Args[3]} {
		ast.Inspect(node, func(node ast.Node) bool {
			if selector, ok := node.(*ast.SelectorExpr); ok && selector.Sel.Name == "MaxHeight" {
				t.Error("header/status must grow with large fonts and long messages")
			}
			return true
		})
	}
}

// Primary 的内容显式选择对比前景；侧栏选中只用中性色，不借用主要操作样式。
func TestDemoPrimaryContentDeclaration(t *testing.T) {
	primaryCount := 0
	for _, file := range []string{"main.go", "basic.go", "overlays.go"} {
		positions := token.NewFileSet()
		source, err := parser.ParseFile(positions, file, nil, 0)
		if err != nil {
			t.Fatal(err)
		}
		if file == "main.go" && !demoHasCall(source, "Style", `"demo-nav-current"`) {
			t.Error("sidebar current item must have its own neutral style")
		}
		ast.Inspect(source, func(node ast.Node) bool {
			call, ok := node.(*ast.CallExpr)
			if !ok || len(call.Args) != 1 {
				return true
			}
			method, ok := call.Fun.(*ast.SelectorExpr)
			if !ok || method.Sel.Name != "Style" {
				return true
			}
			name, ok := call.Args[0].(*ast.SelectorExpr)
			if !ok || name.Sel.Name != "Primary" {
				return true
			}
			primaryCount++
			if file == "main.go" || !demoHasCall(method.X, "Style", "modern.PrimaryContent") {
				t.Errorf("%s: primary action needs explicit contrasting content, not a navigation item", positions.Position(call.Pos()))
			}
			return true
		})
	}
	if primaryCount != 2 {
		t.Fatalf("expected basic action and dialog confirmation; found %d primary actions", primaryCount)
	}
}

func demoHasCall(node ast.Node, method, value string) bool {
	found := false
	ast.Inspect(node, func(node ast.Node) bool {
		call, ok := node.(*ast.CallExpr)
		if !ok || len(call.Args) != 1 {
			return true
		}
		selector, ok := call.Fun.(*ast.SelectorExpr)
		if !ok || selector.Sel.Name != method {
			return true
		}
		var argument string
		switch arg := call.Args[0].(type) {
		case *ast.BasicLit:
			argument = arg.Value
		case *ast.Ident:
			argument = arg.Name
		case *ast.SelectorExpr:
			if qualifier, ok := arg.X.(*ast.Ident); ok {
				argument = qualifier.Name + "." + arg.Sel.Name
			}
		}
		found = found || argument == value
		return true
	})
	return found
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
	d.username.Set("user name")
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
		if d.input.Get() != "user input" || d.username.Get() != "user name" || model.Text() != "user editor text" || model.CanUndo() != canUndo ||
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

// 默认样例由例程持有；八语言都必须覆盖，而不是只翻译标题和按钮。
func TestEmptyDemoEditorsSurviveLanguageChanges(t *testing.T) {
	previous := mlang.Language()
	t.Cleanup(func() { _ = mlang.SetLanguage(previous) })
	if err := mlang.SetLanguage("en"); err != nil {
		t.Fatal(err)
	}
	d, err := newDemo()
	if err != nil {
		t.Fatal(err)
	}
	d.app = new(demoTestApp)
	defer d.stopActivities()
	model, original := d.text, d.text.Text()
	d.input.Set("")
	if err := model.ReplaceAtomic(ui.TextRange{Start: 0, End: model.Len()}, ""); err != nil {
		t.Fatal(err)
	}
	revision := model.Revision()
	for _, choice := range demoLanguages {
		if err := mlang.SetLanguage(choice.tag); err != nil {
			t.Fatal(err)
		}
		d.translateUI()
		_ = d.textPage()
		if d.text != model || model.Text() != "" || d.input.Get() != "" || model.Revision() != revision || !model.CanUndo() || model.CanRedo() {
			t.Fatalf("%s: rebuilding empty editors overwrote user content or history", choice.tag)
		}
	}
	if !model.Undo() || model.Text() != original || !model.Redo() || model.Text() != "" {
		t.Fatal("translation changed the clear operation's undo/redo transaction")
	}
}

func TestDemoSamplesUseCurrentLanguage(t *testing.T) {
	previous := mlang.Language()
	t.Cleanup(func() { _ = mlang.SetLanguage(previous) })
	for _, choice := range demoLanguages {
		t.Run(choice.tag, func(t *testing.T) {
			if err := mlang.SetLanguage(choice.tag); err != nil {
				t.Fatal(err)
			}
			d, err := newDemo()
			if err != nil {
				t.Fatal(err)
			}
			defer d.stopActivities()
			if d.input.Get() != resource.InputSample() || d.text.Text() != resource.EditorSample() ||
				d.inspection.Text() != resource.InspectionSample() || d.tree.Item("project") != resource.SampleProject() ||
				d.list.ItemAt(999) != fmt.Sprintf(resource.ListRow(), 1000) ||
				d.documents["document-2"].Text() != fmt.Sprintf(resource.DocumentSample(), "2") {
				t.Fatal("initial samples did not use the current language")
			}
			row := d.table.ItemAt(0)
			if row.Name != fmt.Sprintf(resource.RecordName(), 1) || row.Kind != resource.RecordDocument() || row.Note != resource.RecordNote() {
				t.Fatal("table sample data not localized")
			}
			if d.tree.Item("main") != "main.go" || row.ID != "file-000" || !strings.Contains(d.text.Text(), "👨‍👩‍👧‍👦") || !strings.Contains(d.text.Text(), "é") {
				t.Fatal("translation changed IDs, filenames or editing test characters")
			}
		})
	}
}

func TestLanguageChangeUpdatesUntouchedSamples(t *testing.T) {
	previous := mlang.Language()
	t.Cleanup(func() { _ = mlang.SetLanguage(previous) })
	_ = mlang.SetLanguage("en")
	d, err := newDemo()
	if err != nil {
		t.Fatal(err)
	}
	d.app = new(demoTestApp)
	defer d.stopActivities()
	model, document := d.text, d.documents["document-1"]
	for _, choice := range demoLanguages {
		_ = mlang.SetLanguage(choice.tag)
		d.translateUI()
		if d.text != model || d.documents["document-1"] != document || model.Text() != resource.EditorSample() ||
			document.Text() != fmt.Sprintf(resource.DocumentSample(), "1") || d.input.Get() != resource.InputSample() ||
			d.baselineInput.Get() != resource.BaselineSample() || d.splitInput.Get() != resource.SplitInputSample() ||
			d.inspection.Text() != resource.InspectionSample() || d.dropDescription() != resource.DropZoneHint() ||
			d.tree.Item("project") != resource.SampleProject() || d.list.ItemAt(0) != fmt.Sprintf(resource.ListRow(), 1) {
			t.Fatalf("%s: untouched samples not refreshed in place", choice.tag)
		}
		revision := model.Revision()
		d.translateUI()
		if model.Revision() != revision || model.CanUndo() || model.CanRedo() {
			t.Fatal("translation created edit history or was not idempotent")
		}
	}
	d.addTab(d.tabWindows[0])
	id := d.tabWindows[0].current
	if d.documents[id].Text() != fmt.Sprintf(resource.DocumentSample(), "4") {
		t.Fatal("new tab ignored the current language")
	}
	d.requestTabClose(d.tabWindows[0], id)
	d.closeTab(d.tabWindows[0])
	if _, ok := d.documentRevisions[id]; ok {
		t.Fatal("closed document retained localization bookkeeping")
	}
}

// 即便撤销后内容恰好等于默认值，也不能丢失用户的重做历史。
func TestLanguageChangePreservesEditedSampleOwnership(t *testing.T) {
	previous := mlang.Language()
	t.Cleanup(func() { _ = mlang.SetLanguage(previous) })
	_ = mlang.SetLanguage("en")
	d, err := newDemo()
	if err != nil {
		t.Fatal(err)
	}
	defer d.stopActivities()
	original := d.text.Text()
	if err := d.text.ReplaceAtomic(ui.TextRange{Start: 0, End: 0}, "user edit\n"); err != nil {
		t.Fatal(err)
	}
	if !d.text.Undo() || d.text.Text() != original || !d.text.CanRedo() {
		t.Fatal("invalid regression setup")
	}
	document := d.documents["document-1"]
	if err := document.ReplaceAtomic(ui.TextRange{Start: 0, End: 0}, "document edit\n"); err != nil {
		t.Fatal(err)
	}
	d.input.Set(resource.InputSample()) // 显式赋值仍交给用户持有，不靠内容猜测。
	d.baselineInput.Set("user baseline")
	d.splitInput.Set("user split")
	d.inspection.SetText(`{"ID":"main","Text":"原始快照"}`)
	d.table.setNote("file-000", resource.RecordNote())
	d.table.sort("size", widgets.SortDescending)
	first := d.table.RowID(0)
	_ = mlang.SetLanguage("zh-CN")
	d.translateUI()
	if d.text.Text() != original || !d.text.CanRedo() || !document.CanUndo() ||
		!strings.HasPrefix(document.Text(), "document edit\n") || d.input.Get() != "Hello, GOUI" ||
		d.baselineInput.Get() != "user baseline" || d.splitInput.Get() != "user split" ||
		d.inspection.Text() != `{"ID":"main","Text":"原始快照"}` || d.table.RowID(0) != first {
		t.Fatal("language change replaced user-owned state or undo history")
	}
	if !d.text.Redo() || d.text.Text() != "user edit\n"+original || !document.Undo() {
		t.Fatal("history no longer works after translation")
	}
	for i := 0; i < d.table.ItemsCount(); i++ {
		r := d.table.ItemAt(i)
		if r.ID == "file-000" && r.Note != "Editable note" {
			t.Fatal("explicit note edit was mistaken for an untouched default")
		}
		if r.ID == "file-001" && r.Note != resource.RecordNote() {
			t.Fatal("untouched note did not follow the language")
		}
	}
	d.table.resetNote("file-000")
	_ = mlang.SetLanguage("de")
	d.translateUI()
	for i := 0; i < d.table.ItemsCount(); i++ {
		if row := d.table.ItemAt(i); row.Note != resource.RecordNote() {
			t.Fatal("reset note did not return ownership to the demo")
		}
	}
}

func TestDropDescriptionTranslatesWithoutChangingPayload(t *testing.T) {
	previous := mlang.Language()
	t.Cleanup(func() { _ = mlang.SetLanguage(previous) })
	d, err := newDemo()
	if err != nil {
		t.Fatal(err)
	}
	d.app = new(demoTestApp)
	defer d.stopActivities()
	for _, test := range []struct {
		name string
		data func(*gui.DragData)
		raw  string
		kind demoDropKind
	}{
		{"text", func(data *gui.DragData) { data.SetText("用户 text 日本語\nملف") }, "用户 text 日本語\nملف", demoDropText},
		{"empty text", func(data *gui.DragData) { data.SetText("") }, "", demoDropText},
		{"files", func(data *gui.DragData) { data.SetFiles([]string{"/tmp/文件.txt", "/tmp/test.go"}) }, "/tmp/文件.txt\n/tmp/test.go", demoDropFiles},
		{"card", func(data *gui.DragData) { data.SetLocal("demo-card", "goui-demo-card") }, "goui-demo-card", demoDropCard},
	} {
		t.Run(test.name, func(t *testing.T) {
			data := new(gui.DragData)
			test.data(data)
			r := &ui.DropRequest{Data: data}
			d.receiveDrop(r)
			if !r.Accepted {
				t.Fatal("demo rejected a supported drop")
			}
			for _, choice := range demoLanguages {
				_ = mlang.SetLanguage(choice.tag)
				d.translateUI()
				if d.dropped != test.raw || d.droppedKind != test.kind {
					t.Fatal("translation changed the raw payload or format")
				}
				want := resource.LocalCardReceived()
				if test.kind == demoDropFiles {
					want = fmt.Sprintf(resource.FilesReceived(), test.raw)
				} else if test.kind == demoDropText {
					want = fmt.Sprintf(resource.TextReceived(), test.raw)
				}
				if d.dropDescription() != want {
					t.Fatal("drop description did not follow the language")
				}
			}
		})
	}
	unsupported := &ui.DropRequest{Data: new(gui.DragData)}
	d.receiveDrop(unsupported)
	if unsupported.Accepted || d.dropped != "goui-demo-card" {
		t.Fatal("unsupported data overwrote the accepted drop")
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

// 只在内存中栅格化。采样均远离抗锯齿边缘，按预乘 RGBA8 精确比较：
// 512 画布的透明外角、蓝色窗框、白眼和鼻子；横版还检查灰色字母。
func TestDemoLogoPixels(t *testing.T) {
	d, err := newDemo()
	if err != nil {
		t.Fatal(err)
	}
	defer d.stopActivities()
	if d.logo == nil {
		t.Fatal("missing logo source")
	}
	for _, foreground := range []graphics.Color{graphics.RGB(180, 30, 90), graphics.RGB(30, 180, 90)} {
		img := d.logo.Image(512, 512, foreground)
		if img == nil || img.Bounds() != image.Rect(0, 0, 512, 512) {
			t.Fatal("invalid square logo dimensions")
		}
		for _, sample := range []struct {
			x, y int
			want color.RGBA
		}{
			{0, 0, color.RGBA{}},
			{256, 100, color.RGBA{G: 173, B: 216, A: 255}},
			{122, 103, color.RGBA{R: 255, G: 255, B: 255, A: 255}},
			{184, 304, color.RGBA{R: 255, G: 255, B: 255, A: 255}},
			{256, 358, color.RGBA{R: 23, G: 50, B: 74, A: 255}},
		} {
			if got := color.RGBAModel.Convert(img.At(sample.x, sample.y)).(color.RGBA); got != sample.want {
				t.Errorf("logo (%d,%d), foreground %v: %v, want %v", sample.x, sample.y, foreground, got, sample.want)
			}
		}
	}
	if d.wordmark == nil || d.wordmark.Bounds() != image.Rect(0, 0, 305, 128) {
		t.Fatal("wordmark must preserve its 1220:512 aspect ratio")
	}
	for _, sample := range []struct {
		x, y int
		want color.RGBA
	}{
		{0, 0, color.RGBA{}},
		{61, 32, color.RGBA{G: 173, B: 216, A: 255}},
		{285, 64, color.RGBA{R: 91, G: 122, B: 143, A: 255}},
	} {
		if got := color.RGBAModel.Convert(d.wordmark.At(sample.x, sample.y)).(color.RGBA); got != sample.want {
			t.Errorf("wordmark (%d,%d): %v, want %v", sample.x, sample.y, got, sample.want)
		}
	}
}

// 实际展示尺寸包含按钮、标题栏、图形样例、动画及配置 2x 缩放。
// 只比较同一尺寸下的完整预乘 RGBA 像素，变化的前景不能染色 Logo。
func TestDemoLogoSizesPreserveColors(t *testing.T) {
	d, err := newDemo()
	if err != nil {
		t.Fatal(err)
	}
	defer d.stopActivities()
	for _, size := range []int{16, 18, 24, 32, 36, 48, 64, 80, 160} {
		t.Run(fmt.Sprintf("%dpx", size), func(t *testing.T) {
			original := d.logo.Image(size, size, graphics.Color{})
			if original == nil || original.Bounds() != image.Rect(0, 0, size, size) {
				t.Fatal("invalid original logo image")
			}
			for _, foreground := range []graphics.Color{
				graphics.RGB(32, 32, 36), graphics.RGB(236, 236, 238),
				graphics.RGB(180, 30, 90), graphics.RGB(151, 151, 159),
				{R: .25, G: .25, B: .25, A: .5},
			} {
				img := d.logo.Image(size, size, foreground)
				if img == nil || img.Bounds() != image.Rect(0, 0, size, size) {
					t.Fatal("invalid logo image")
				}
				for y := 0; y < size; y++ {
					for x := 0; x < size; x++ {
						got := color.RGBAModel.Convert(img.At(x, y))
						want := color.RGBAModel.Convert(original.At(x, y))
						if got != want {
							t.Fatalf("foreground %v recolored logo (%d,%d): %v, want %v", foreground, x, y, got, want)
						}
					}
				}
			}
		})
	}
}

// 可着色图标集合不能包含 Logo 的单色副本；alpha 掩码比较不依赖前景色。
func TestDemoTintedIconsExcludeLogo(t *testing.T) {
	d, err := newDemo()
	if err != nil {
		t.Fatal(err)
	}
	defer d.stopActivities()
	logo := d.logo.Image(32, 32, graphics.Color{})
	for index, source := range d.icons {
		img := source.Image(32, 32, graphics.RGB(100, 150, 200))
		if img == nil || img.Bounds() != logo.Bounds() {
			t.Fatalf("icon %d: invalid image", index)
		}
		matches := true
		for y := 0; y < 32 && matches; y++ {
			for x := 0; x < 32; x++ {
				_, _, _, a := img.At(x, y).RGBA()
				_, _, _, logoAlpha := logo.At(x, y).RGBA()
				if a != logoAlpha {
					matches = false
					break
				}
			}
		}
		if matches {
			t.Errorf("icon %d reuses the logo mask; use an ordinary icon for tinting", index)
		}
	}
}

// 像素正确不能证明声明使用了正确资源；检查按钮和标签页真实绑定。
func TestDemoBrandIconBindings(t *testing.T) {
	for _, test := range []struct{ name, file, method string }{
		{"button", "basic.go", "Content"},
		{"tab", "tabs.go", "Icon"},
	} {
		t.Run(test.name, func(t *testing.T) {
			source, err := parser.ParseFile(token.NewFileSet(), test.file, nil, 0)
			if err != nil {
				t.Fatal(err)
			}
			found := false
			ast.Inspect(source, func(node ast.Node) bool {
				call, ok := node.(*ast.CallExpr)
				if !ok || len(call.Args) != 1 {
					return true
				}
				method, ok := call.Fun.(*ast.SelectorExpr)
				if !ok || method.Sel.Name != test.method {
					return true
				}
				if test.name == "button" {
					caption := false
					ast.Inspect(call.Args[0], func(node ast.Node) bool {
						if getter, ok := node.(*ast.SelectorExpr); ok && getter.Sel.Name == "IconButton" {
							caption = true
						}
						return true
					})
					if !caption {
						return true
					}
				}
				found = true
				if !demoHasCall(call, "Icon", "d.logo") {
					t.Error("brand icon must use the original-color d.logo, not a tintable sample")
				}
				return false
			})
			if !found {
				t.Fatal("missing brand icon declaration")
			}
		})
	}
}

// 原色资源的像素正确不代表动画声明使用了它；直接检查该声明的绑定，
// 避免为了这一例程测试导出 UI 私有状态或实现第二套协调器。
func TestAnimatedIconUsesRawLogo(t *testing.T) {
	source, err := parser.ParseFile(token.NewFileSet(), "graphics.go", nil, 0)
	if err != nil {
		t.Fatal(err)
	}
	found := false
	ast.Inspect(source, func(node ast.Node) bool {
		call, ok := node.(*ast.CallExpr)
		if !ok || len(call.Args) != 1 {
			return true
		}
		method, ok := call.Fun.(*ast.SelectorExpr)
		if !ok || method.Sel.Name != "ID" {
			return true
		}
		literal, ok := call.Args[0].(*ast.BasicLit)
		if !ok || literal.Kind != token.STRING || literal.Value != `"animated-icon"` {
			return true
		}
		found = true
		icon, ok := method.X.(*ast.CallExpr)
		if !ok || len(icon.Args) != 1 {
			t.Fatal("animated-icon must be declared with ui.Icon(d.logo)")
		}
		constructor, ok := icon.Fun.(*ast.SelectorExpr)
		if !ok || constructor.Sel.Name != "Icon" {
			t.Fatal("animated-icon must be declared with ui.Icon(d.logo)")
		}
		binding, ok := icon.Args[0].(*ast.SelectorExpr)
		if !ok || binding.Sel.Name != "logo" {
			t.Fatal("animated-icon uses a monochrome sample instead of the original-color logo")
		}
		return false
	})
	if !found {
		t.Fatal("missing animated-icon declaration")
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
		} else if r.Note != resource.RecordNote() {
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
