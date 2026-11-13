// TreeView声明式行外观与content Part的原生窗口回归，不使用DevServer。
//
// 环境：Linux/X11（记录WM）、Windows或macOS桌面，记录实际后端与1x/2x。
// 启动：go run ./tests/window/widgets/tree_row_style [-basic] [-dark]
// 可设置GOUI_PLAT_PAINTER=software/opengl/direct2d、GOUI_PLAT_SCALE=1/2。
// 初始：两层展开目录、Target API及100个根节点；Target已选中且为Current。
//
// 操作与逐项预期（各步F6做公开几何／像素／语义检查，不替代原生外观观察）：
//  1. Before、After分别画上／下横线，默认起点在Target的业务图标处，展开槽不画线。
//     Before下分别点Root depth、Deep depth：起点为根／二层，实际图标和文字不移动。
//     Native depth恢复本行深度。Into画整行轮廓，选中背景及current仍保留。
//  2. Omit省略两个声明回调，恢复默认；Clear恢复回调并关闭反馈。各按F6。
//  3. 点击Target的行体；连续两次Tab后焦点回到树且有键盘提示，F6。
//     再Tab到Branch内Action按钮，F6；真实焦点在按钮，行壳没有伪造Focused。
//  4. Before下Far滚动到末项，F6确认离屏复用无残留；Back恢复顶部再F6。
//     Rebuild、Dark各F6，模型、内容位置和交互状态保留。Basic保持亮色。
//  5. Clear后，将Target行体按住拖到Next root上部并释放：真实原生拖放只更新
//     外观并接受，不移动数据。拖动中观察缩进线；结束后清理反馈，F7验证。
//  6. F8要求所有阶段完成，PASS、退出0；提前关闭或检查失败退出1。
//
// 排序合法性与视口外边缘提示不属于本例；这里只验证应用提供状态后的声明式绘制。
// 按钮不进入Tab序列；macOS功能键可能需Fn。F6使用RenderWidget重绘当前子树。
// 颜色采样在2 DIP实线内部，预乘sRGB只允许1级8位舍入；不比较跨平台字体金图。
// 副作用：窗口、内存模型和日志，不改剪贴板、文件或系统设置。复位：重启。
package main

import (
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"image"
	"image/color"
	"os"

	"github.com/golang-gui/goui/core/geometry"
	"github.com/golang-gui/goui/gui"
	"github.com/golang-gui/goui/layout"
	"github.com/golang-gui/goui/style"
	"github.com/golang-gui/goui/theme/basic"
	"github.com/golang-gui/goui/theme/modern"
	"github.com/golang-gui/goui/ui"
	"github.com/golang-gui/goui/widgets"
	wui "github.com/golang-gui/goui/widgets/ui"
)

var ink = color.RGBA{R: 90, G: 100, B: 110, A: 255}

func sheet(fallback, dark bool) style.StyleSheet {
	rules := modern.Rules(modern.Options{Dark: dark})
	if fallback {
		rules = basic.Rules()
	}
	for _, mode := range []string{"before", "after", "into"} {
		name := "test-row-" + mode
		for _, rule := range append([]style.Rule(nil), rules...) {
			if rule.Sel.Name == "tree-item" {
				rule.Sel.Name = name
				rules = append(rules, rule)
			}
		}
		if mode == "into" {
			for _, part := range []string{"", "selected"} {
				rules = append(rules, style.Name(name).Part(part).BorderColor(ink).BorderWidth(2).BorderSides(style.BorderAll))
			}
		} else {
			side := style.BorderTop
			if mode == "after" {
				side = style.BorderBottom
			}
			rules = append(rules, style.Name(name).Part("content").BackgroundColor(color.Transparent).
				BorderColor(ink).BorderWidth(2).BorderSides(side).Radius(0))
		}
	}
	return style.Sheet(rules...)
}

func main() {
	fallback, dark := flag.Bool("basic", false, "Basic theme"), flag.Bool("dark", false, "dark Modern palette")
	flag.Parse()
	if err := run(*fallback, *dark); err != nil {
		fmt.Fprintln(os.Stderr, "FAIL:", err)
		os.Exit(1)
	}
}

func run(fallback, dark bool) error {
	model := widgets.NewTreeStore[string]()
	model.Modify(func(m *widgets.TreeStore[string]) {
		m.Append("", "root", "Root folder")
		m.Append("root", "branch", "Branch")
		m.Append("branch", "leaf", "Nested API")
		m.Append("root", "target", "Target API")
		m.Append("", "next", "Next root")
		for i := range 100 {
			m.Append("", fmt.Sprintf("r%03d", i), fmt.Sprintf("Record %03d", i))
		}
	})
	mode, target, depth := "clear", "target", -1
	omit, ready, finished, far := false, false, false, false
	generation, dragMotions, dragEnds, drops := 0, 0, 0, 0
	seen := map[string]bool{}
	var original gui.Widget
	var size geometry.Size
	var failure error
	err := ui.Run("org.golanggui.TreeRowStyle", func(app ui.App) ui.RootView {
		tree := func() *widgets.TreeView { return app.FindWidget("main", "tree").(*widgets.TreeView) }
		fail := func(err error) { failure = err; app.Quit() }
		bounds := func() map[string]geometry.Rectangle {
			result := map[string]geometry.Rectangle{}
			for _, info := range tree().Snapshot().Children {
				if h, ok := info.Attributes[widgets.HierarchyInfoKey].(widgets.HierarchyInfo); ok {
					result["row-"+h.NodeID] = info.Bounds
				}
			}
			for _, id := range []string{"tree", "scroll", "before", "after", "into", "native", "root-depth", "deep", "omit", "clear", "far", "back", "rebuild", "dark", "action"} {
				if w := app.FindWidget("main", id); w != nil {
					result[id] = w.Snapshot().Bounds
				}
			}
			return result
		}
		lookup := func(info gui.WidgetInfo) gui.Widget {
			viewport := tree().Snapshot().Bounds
			left, top := max(info.Bounds.X, viewport.X), max(info.Bounds.Y, viewport.Y)
			right := min(info.Bounds.X+info.Bounds.Width, viewport.X+viewport.Width)
			bottom := min(info.Bounds.Y+info.Bounds.Height, viewport.Y+viewport.Height)
			point := geometry.Point{X: (left+right)/2 - viewport.X, Y: (top+bottom)/2 - viewport.Y}
			w := gui.Pick(tree(), point)
			for w != nil && w.Snapshot().Role != widgets.RoleTreeItem {
				w = w.Parent()
			}
			return w
		}
		check := func() error {
			if !ready || tree().Model() != model || tree().Snapshot().ItemCount != 105 {
				return errors.New("not ready or model changed")
			}
			found := false
			for _, info := range tree().Snapshot().Children {
				h := info.Attributes[widgets.HierarchyInfoKey].(widgets.HierarchyInfo)
				row := lookup(info)
				if row == nil || info.Focused || len(info.Children) != 1 {
					return fmt.Errorf("row %s hit=%t focus=%t children=%d", h.NodeID, row != nil, info.Focused, len(info.Children))
				}
				want := ""
				if !omit && h.NodeID == target && mode != "clear" {
					want = "test-row-" + mode
				}
				if row.StyleName() != want {
					return fmt.Errorf("row %s style=%q want=%q", h.NodeID, row.StyleName(), want)
				}
				if h.NodeID != "target" {
					continue
				}
				found = true
				if !seen["recycle"] && (row != original || row.Rect().Size != size) {
					return errors.New("style update replaced or resized the row")
				}
				icon := app.FindWidget("main", "icon-target")
				if icon.StyleName() != "tree-item-icon" || icon.Snapshot().Bounds.X-info.Bounds.X != 40 {
					return errors.New("paint depth moved or restyled actual content")
				}
				pixels, err := gui.RenderWidget(row, 1)
				if err != nil {
					return err
				}
				line := func(x, y int) bool {
					got := color.RGBAModel.Convert(pixels.At(x, y)).(color.RGBA)
					for i, value := range []uint8{got.R, got.G, got.B, got.A} {
						want := []uint8{ink.R, ink.G, ink.B, ink.A}[i]
						if int(value) < int(want)-1 || int(value) > int(want)+1 {
							return false
						}
					}
					return true
				}
				y, start := 1, 40
				if mode == "after" {
					// DirectWrite行高可能含小数；采样点中心必须位于底部2 DIP实线内。
					y = int(info.Bounds.Height - 1)
				}
				if depth >= 0 {
					start = 24 + depth*16
				}
				active := !omit && target == "target" && (mode == "before" || mode == "after")
				if active && (!line(start+1, y) || line(start-2, y)) {
					return fmt.Errorf("content line mode=%s depth=%d start=%d y=%d", mode, depth, start, y)
				}
				if !active && mode != "into" && line(60, 1) {
					return errors.New("cleared line still visible")
				}
				if mode == "into" && !omit && !line(12, 1) {
					return errors.New("into border did not cover the full row")
				}
			}
			if far {
				if found || app.FindWidget("main", "scroll").Snapshot().ScrollY <= 0 {
					return errors.New("far did not recycle the target")
				}
				seen["recycle"] = true
			} else if seen["recycle"] && found {
				seen["back"] = true
			}
			if tree().Focused() && tree().FocusVisible() {
				seen["tree-focus"] = true
			}
			if action := app.FindWidget("main", "action"); action != nil && action.Focused() && !tree().Focused() {
				seen["child-focus"] = true
			}
			key := fmt.Sprintf("%s:%d", mode, depth)
			if omit {
				key = "omit"
			}
			seen[key] = true
			if generation > 0 {
				seen["rebuild"] = true
			}
			data, _ := json.Marshal(bounds())
			fmt.Printf("CHECK mode=%s depth=%d omit=%t far=%t focus=%t bounds=%s\n", mode, depth, omit, far, tree().Focused(), data)
			return nil
		}
		set := func(next string) { mode, omit, target = next, false, "target"; app.RequestUpdate() }
		button := func(id, text string, fn func()) ui.View {
			return ui.Button(text).ID(id).Focusable(false).FocusOnPointer(false).OnClick(fn)
		}
		view := wui.TreeView(model, func(row widgets.TreeRow, title string) ui.View {
			children := []ui.View{ui.Label("■").ID("icon-" + row.ID).Style("tree-item-icon"), ui.Label(title).Style("tree-item-text")}
			if row.ID == "branch" {
				children = append(children, ui.Button("Action").ID("action"))
			}
			return ui.HBox(children...).Spacing(8)
		}).ID("tree")
		if !ready {
			view.Expanded([]string{"root", "branch"}).Selection([]string{"target"}).Current("target")
		}
		if !omit {
			view.RowStyle(func(row widgets.TreeRow, _ string) string {
				if row.ID == target && mode != "clear" {
					return "test-row-" + mode
				}
				return ""
			}).RowContentDepth(func(widgets.TreeRow, string) (int, bool) { return depth, depth >= 0 })
		}
		motion := func(m *ui.DragMotion) {
			row, rect, ok := tree().RowAt(m.Position)
			if !ok {
				m.Action = 0
				return
			}
			dragMotions++
			mode, target, omit = "before", row.ID, false
			if m.Position.Y > rect.Y+rect.Height/2 {
				mode = "after"
			}
			m.Action = ui.DragMove
			fmt.Printf("DRAG target=%s mode=%s depth=%d\n", target, mode, depth)
			app.RequestUpdate()
		}
		view.DragSource(ui.DragSource().Actions(ui.DragMove).OnPrepare(func(r *ui.DragPrepare) {
			row, _, ok := tree().RowAt(r.Position)
			if ok {
				r.Data = new(ui.DragData)
				r.Data.SetLocal("tree-row-style", row.ID)
			}
		}).OnEnd(func(r ui.DragResult) {
			dragEnds++
			if r.Err != nil {
				failure = r.Err
			}
			set("clear")
		})).DropTarget(ui.DropTarget(ui.LocalFormat("tree-row-style")).Actions(ui.DragMove).
			OnEnter(motion).OnMotion(motion).OnLeave(func() { set("clear") }).OnError(fail).
			OnDrop(func(r *ui.DropRequest) { r.Accepted = true; drops++; set("clear") }))
		content := ui.VBox(
			ui.HBox(button("before", "Before", func() { set("before") }), button("after", "After", func() { set("after") }), button("into", "Into", func() { set("into") }),
				button("native", "Native depth", func() { depth = -1; app.RequestUpdate() }), button("root-depth", "Root depth", func() { depth = 0; app.RequestUpdate() }), button("deep", "Deep depth", func() { depth = 2; app.RequestUpdate() })).Spacing(6),
			ui.HBox(button("omit", "Omit", func() { omit = true; app.RequestUpdate() }), button("clear", "Clear", func() { set("clear") }),
				button("far", "Far", func() { far = true; tree().Reveal("r099") }), button("back", "Back", func() { far = false; tree().Reveal("root") }),
				button("rebuild", "Rebuild", func() { generation++; app.RequestUpdate() }), button("dark", "Dark", func() { dark = !dark; seen["palette"] = true; app.RequestUpdate() })).Spacing(6),
			ui.ScrollView(view).ID("scroll").Style("text-view").MinSize(540, 240).MaxSize(540, 240),
			ui.Label("F6 check; two Tabs for tree hint; Tab for Action; drag Target to Next; F7 drag check; F8 finish"),
		).Spacing(10).Padding(16).CrossAlign(layout.CrossStart)
		return ui.Root().StyleSheet(sheet(fallback, dark)).Windows(ui.Window("main").Title("GOUI Tree row style").Size(920, 440).Content(content).Shortcuts(
			ui.Shortcut(ui.KeyF6).OnActivate(func() {
				if err := check(); err != nil {
					fail(err)
				}
			}),
			ui.Shortcut(ui.KeyF7).OnActivate(func() {
				if dragMotions == 0 || dragEnds == 0 || drops == 0 || mode != "clear" {
					fail(fmt.Errorf("drag motion/end/drop=%d/%d/%d mode=%s", dragMotions, dragEnds, drops, mode))
					return
				}
				seen["drag"] = true
				fmt.Println("CHECK drag")
			}),
			ui.Shortcut(ui.KeyF8).OnActivate(func() {
				for _, key := range []string{"clear:-1", "before:-1", "after:-1", "before:0", "before:2", "into:-1", "omit", "recycle", "back", "tree-focus", "child-focus", "rebuild", "palette", "drag"} {
					if !seen[key] {
						fail(fmt.Errorf("missing stage %s", key))
						return
					}
				}
				finished = true
				fmt.Println("PASS tree row style")
				app.Quit()
			}),
		).OnAfterPaint(func(p gui.Painter, _ geometry.Rectangle) {
			if ready {
				return
			}
			for _, info := range tree().Snapshot().Children {
				if h, ok := info.Attributes[widgets.HierarchyInfoKey].(widgets.HierarchyInfo); !ok || h.NodeID != "target" {
					continue
				}
				original = lookup(info)
				if original == nil || info.Bounds.Width <= 0 || info.Bounds.X <= 0 {
					return
				}
				size = original.Rect().Size
				resource, err := p.NewImage(image.NewRGBA(image.Rect(0, 0, 1, 1)))
				if err != nil {
					failure = err
					app.Post(app.Quit)
					return
				}
				defer resource.Destroy()
				ready = true
				data, _ := json.Marshal(bounds())
				fmt.Printf("READY backend=%T scale=%g bounds=%s\n", resource, p.PixelScale(), data)
				break
			}
		}))
	})
	if err != nil {
		return err
	}
	if failure != nil {
		return failure
	}
	if !finished {
		return errors.New("closed before all checks")
	}
	return nil
}
