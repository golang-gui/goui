// 树连续选区、虚拟化邻接信息与独立单行边框验收，不使用DevServer。
//
// 环境：Linux/X11（记录WM）、Windows或macOS桌面，记录实际后端及1x/2x；
// Software另测1.5x。启动：go run ./tests/window/widgets/tree_selection [-basic] [-dark]
// 可用GOUI_PLAT_PAINTER和GOUI_PLAT_SCALE选择后端／倍率。初始Root/A/Nested/B
// 连续选中，另有Other和100行；内容最小高度27.5，使常规字体下行高为35.5 DIP。
//
// 操作与逐项预期（每步完成后F6，检查绑定回调、RowAt、Snapshot与公开重绘像素）：
//  1. F6；Single、Group、Split各F6，分别显示单行、跨层级连续区和分组选区。
//     连选仅外缘有圆角，接合处及左右色带没有凹口或半透明重叠。
//  2. All、Far、Back各F6：离屏行参与邻接，视口首行不能冒充选区首行。Fold
//     折叠A、再Fold展开，各F6；重建平铺顺序后邻接正确，不保留隐藏节点选择。
//  3. 点击A行体，再Shift点击B行体，F6：选择A/Nested/B，完成真实范围选择。
//     Rebuild、Dark各F6，树与已实现行身份保持。Basic仍使用自身默认配色。
//  4. Group、Border各F6：A在连续区中间仍显示完整圆角单行红框，背景继续连接。
//     将A行体按住拖到B行体，释放后F7：原生拖放恰好接受一次，源为A；不改模型。
//     拖动过程中边框跟随逻辑目标，结束后清除，已有选区仍在。
//  5. F8要求以上检查全部完成，PASS、退出0；提前关闭或断言失败退出1。
//
// F6只读取公开状态和RenderWidget，实际输入通过正常平台分发；不调用语义动作。
// 接合色带使用主题默认Normal选中色，fixture将Hovered/Pressed也固定为同色，以
// 隔离几何与色差。sRGB不透明颜色只容许1级8位舍入；半透明独立预期由包内覆盖。
// 副作用：本例窗口、内存模型和日志；不写文件、剪贴板或系统设置。复位：重启。
package main

import (
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"image"
	"image/color"
	"math"
	"os"
	"slices"

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

func main() {
	fallback, dark := flag.Bool("basic", false, "Basic theme"), flag.Bool("dark", false, "Dark Modern theme")
	flag.Parse()
	if err := run(*fallback, *dark); err != nil {
		fmt.Fprintln(os.Stderr, "FAIL", err)
		os.Exit(1)
	}
}

func run(fallback, dark bool) error {
	model := widgets.NewTreeStore[string]()
	model.Modify(func(m *widgets.TreeStore[string]) {
		m.Append("", "root", "Root")
		m.Append("root", "a", "A")
		m.Append("a", "nested", "Nested")
		m.Append("root", "b", "B")
		m.Append("", "other", "Other")
		for i := range 100 {
			m.Append("", fmt.Sprintf("r%03d", i), fmt.Sprintf("Record %03d", i))
		}
	})
	ready, finished, folded, far, outlined := false, false, false, false, false
	stage, target, source := "initial", "", ""
	drops, ends := 0, 0
	seen := map[string]bool{}
	latest := map[string]widgets.TreeRow{}
	lastWidgets := map[string]gui.Widget{}
	var original gui.Widget
	var failure error
	logical := func() []string {
		ids := []string{"root", "a"}
		if !folded {
			ids = append(ids, "nested")
		}
		ids = append(ids, "b", "other")
		for i := range 100 {
			ids = append(ids, fmt.Sprintf("r%03d", i))
		}
		return ids
	}
	err := ui.Run("org.golanggui.TreeSelection", func(app ui.App) ui.RootView {
		tree := func() *widgets.TreeView { return app.FindWidget("main", "tree").(*widgets.TreeView) }
		bounds := func() map[string]geometry.Rectangle {
			out := map[string]geometry.Rectangle{}
			for _, id := range []string{"tree", "single", "group", "split", "all", "far", "back", "fold", "rebuild", "dark", "border"} {
				if w := app.FindWidget("main", id); w != nil {
					out[id] = w.Snapshot().Bounds
				}
			}
			for _, info := range tree().Snapshot().Children {
				h := info.Attributes[widgets.HierarchyInfoKey].(widgets.HierarchyInfo)
				out["row-"+h.NodeID] = info.Bounds
			}
			return out
		}
		ink := color.RGBA{R: 209, G: 209, B: 209, A: 255}
		if dark {
			ink = color.RGBA{R: 79, G: 79, B: 83, A: 255}
		}
		if fallback {
			ink = color.RGBA{R: 210, G: 218, B: 230, A: 255}
		}
		fail := func(err error) { failure = err; app.Quit() }
		check := func() error {
			v := tree()
			if !ready || v != original {
				return errors.New("tree replaced or not ready")
			}
			ids, selected := logical(), v.Selection()
			set := map[string]bool{}
			for _, id := range selected {
				set[id] = true
			}
			vp := v.Snapshot().Bounds
			currentWidgets := map[string]gui.Widget{}
			pixels, err := gui.RenderWidget(v, 1)
			if err != nil {
				return err
			}
			pixel := func(x, y int, want color.RGBA) error {
				got := color.RGBAModel.Convert(pixels.At(x, y)).(color.RGBA)
				for i, value := range []uint8{got.R, got.G, got.B, got.A} {
					w := []uint8{want.R, want.G, want.B, want.A}[i]
					if int(value) < int(w)-1 || int(value) > int(w)+1 {
						return fmt.Errorf("pixel(%d,%d)=%v want=%v", x, y, got, want)
					}
				}
				return nil
			}
			for _, info := range v.Snapshot().Children {
				h := info.Attributes[widgets.HierarchyInfoKey].(widgets.HierarchyInfo)
				index := slices.Index(ids, h.NodeID)
				if index < 0 {
					return fmt.Errorf("unexpected visible ID %s", h.NodeID)
				}
				previous, next := index > 0 && set[ids[index-1]], index+1 < len(ids) && set[ids[index+1]]
				x, y := info.Bounds.X-vp.X, info.Bounds.Y-vp.Y
				point := geometry.Point{X: x + info.Bounds.Width/2, Y: (max(0, y) + min(vp.Height, y+info.Bounds.Height)) / 2}
				row, _, hit := v.RowAt(point)
				widget := gui.Pick(v, point)
				for widget != nil && widget.Snapshot().Role != widgets.RoleTreeItem {
					widget = widget.Parent()
				}
				if widget == nil {
					return errors.New("row is missing from normal picking")
				}
				if (stage == "rebuild" || stage == "palette") && lastWidgets[h.NodeID] != widget {
					return fmt.Errorf("rebuild replaced row %s", h.NodeID)
				}
				currentWidgets[h.NodeID] = widget
				bound := latest[h.NodeID]
				if !hit || row.ID != h.NodeID || row.PreviousSelected != previous || row.NextSelected != next || bound != row || row.Selected != info.Selected || info.Selected != set[h.NodeID] || info.Focused {
					return fmt.Errorf("row=%+v bound=%+v expected=%t/%t selected=%t", row, bound, previous, next, set[h.NodeID])
				}
				if !info.Selected {
					continue
				}
				left, right := int(math.Round(float64(x))), int(math.Round(float64(x+info.Bounds.Width)))-1
				for _, edge := range []struct {
					y      float32
					joined bool
				}{{y, previous}, {y + info.Bounds.Height - 1, next}} {
					py := int(math.Round(float64(edge.y)))
					if py < 0 || py >= pixels.Bounds().Dy() {
						continue
					}
					want := color.RGBA{}
					if edge.joined {
						want = ink
					}
					if err := pixel(left, py, want); err != nil {
						return err
					}
					if err := pixel(right, py, want); err != nil {
						return err
					}
				}
				if outlined && h.NodeID == "a" {
					if !previous || !next {
						return errors.New("outline must cover a middle row")
					}
					if err := pixel(int(x+info.Bounds.Width/2), int(math.Round(float64(y))), color.RGBA{R: 220, G: 50, B: 40, A: 255}); err != nil {
						return err
					}
				}
			}
			if far && v.Snapshot().VisibleStart <= 0 {
				return errors.New("Far did not virtualize the first rows")
			}
			if slices.Equal(selected, []string{"a", "nested", "b"}) {
				seen["input-range"] = true
			}
			seen[stage] = true
			lastWidgets = currentWidgets
			data, _ := json.Marshal(bounds())
			fmt.Printf("CHECK stage=%s selected=%v far=%t folded=%t outlined=%t bounds=%s\n", stage, selected, far, folded, outlined, data)
			return nil
		}
		button := func(id, title string, fn func()) ui.View {
			return ui.Button(title).ID(id).Focusable(false).FocusOnPointer(false).OnClick(fn)
		}
		selectIDs := func(name string, ids []string) func() {
			return func() { stage = name; tree().SetSelection(ids); app.RequestUpdate() }
		}
		view := wui.TreeView(model, func(row widgets.TreeRow, title string) ui.View {
			return ui.Label(title).Style("tree-item-text").MinSize(0, 27.5)
		}).
			ID("tree").SelectionMode(widgets.SelectionMultiple).RowInsets(12, 12).
			RowStyle(func(row widgets.TreeRow, _ string) string {
				latest[row.ID] = row
				if row.ID == target {
					return "drop-row"
				}
				return ""
			})
		expanded := []string{"root"}
		if !folded {
			expanded = append(expanded, "a")
		}
		view.Expanded(expanded)
		if !ready {
			view.Selection([]string{"root", "a", "nested", "b"})
		}
		motion := func(m *ui.DragMotion) {
			if row, _, ok := tree().RowAt(m.Position); ok {
				target = row.ID
				m.Action = ui.DragMove
				app.RequestUpdate()
			}
		}
		view.DragSource(ui.DragSource().Actions(ui.DragMove).OnPrepare(func(r *ui.DragPrepare) {
			if row, _, ok := tree().RowAt(r.Position); ok {
				source = row.ID
				r.Data = new(ui.DragData)
				r.Data.SetLocal("tree-selection", source)
			}
		}).OnEnd(func(r ui.DragResult) {
			ends++
			if r.Err != nil {
				failure = r.Err
			}
			outlined = false
			target = ""
			app.RequestUpdate()
		})).
			DropTarget(ui.DropTarget(ui.LocalFormat("tree-selection")).Actions(ui.DragMove).OnEnter(motion).OnMotion(motion).
				OnLeave(func() { target = ""; app.RequestUpdate() }).OnError(fail).OnDrop(func(r *ui.DropRequest) { r.Accepted = true; drops++ }))
		rules := modern.Rules(modern.Options{Dark: dark})
		if fallback {
			rules = basic.Rules()
		}
		for _, rule := range append([]style.Rule(nil), rules...) {
			if rule.Sel.Name == "tree-item" {
				rule.Sel.Name = "drop-row"
				rules = append(rules, rule)
			}
		}
		for _, name := range []string{"tree-item", "drop-row"} {
			for _, state := range []style.State{style.Normal, style.Hovered, style.Pressed} {
				rules = append(rules, style.Name(name).Part("selected").State(state).BackgroundColor(ink))
			}
		}
		rules = append(rules, style.Name("drop-row").BorderColor(color.RGBA{R: 220, G: 50, B: 40, A: 255}).BorderWidth(2).Radius(8))
		content := ui.VBox(
			ui.HBox(button("single", "Single", selectIDs("single", []string{"a"})), button("group", "Group", selectIDs("group", []string{"root", "a", "nested", "b"})),
				button("split", "Split", selectIDs("split", []string{"root", "nested", "b"})), button("all", "All", selectIDs("all", logical())),
				button("far", "Far", func() { stage = "far"; far = true; tree().Reveal("r099") }), button("back", "Back", func() { stage = "back"; far = false; tree().Reveal("root") })).Spacing(8),
			ui.HBox(button("fold", "Fold", func() {
				folded = !folded
				stage = "expanded"
				if folded {
					stage = "collapsed"
				}
				app.RequestUpdate()
			}),
				button("rebuild", "Rebuild", func() { stage = "rebuild"; app.RequestUpdate() }), button("dark", "Dark", func() { stage = "palette"; dark = !dark; app.RequestUpdate() }),
				button("border", "Border", func() { stage = "border"; outlined = true; target = "a"; app.RequestUpdate() })).Spacing(8),
			ui.ScrollView(view).ID("scroll").Style("text-view").MinSize(480, 240).MaxSize(480, 240),
			ui.Label("F6 check; click A, Shift-click B; Group + Border; drag A to B; F7; F8 finish"),
		).Spacing(10).Padding(16).CrossAlign(layout.CrossStart)
		return ui.Root().StyleSheet(style.Sheet(rules...)).Windows(ui.Window("main").Title("GOUI Tree selection").Size(920, 470).Content(content).Shortcuts(
			ui.Shortcut(ui.KeyF6).OnActivate(func() {
				if err := check(); err != nil {
					fail(err)
				}
			}),
			ui.Shortcut(ui.KeyF7).OnActivate(func() {
				if source != "a" || drops != 1 || ends != 1 || failure != nil || !slices.Equal(tree().Selection(), []string{"root", "a", "nested", "b"}) {
					fail(fmt.Errorf("drag source=%s drop/end=%d/%d error=%v", source, drops, ends, failure))
					return
				}
				seen["drag"] = true
				fmt.Println("CHECK drag")
			}),
			ui.Shortcut(ui.KeyF8).OnActivate(func() {
				for _, key := range []string{"initial", "single", "group", "split", "all", "far", "back", "collapsed", "expanded", "input-range", "rebuild", "palette", "border", "drag"} {
					if !seen[key] {
						fail(fmt.Errorf("missing stage %s", key))
						return
					}
				}
				finished = true
				fmt.Println("PASS tree selection")
				app.Quit()
			}),
		).OnAfterPaint(func(p gui.Painter, _ geometry.Rectangle) {
			if ready || tree().Rect().Width <= 0 || tree().Snapshot().Bounds.X <= 0 || len(tree().Snapshot().Children) < 4 {
				return
			}
			original, ready = tree(), true
			resource, err := p.NewImage(image.NewRGBA(image.Rect(0, 0, 1, 1)))
			if err != nil {
				failure = err
				app.Post(app.Quit)
				return
			}
			defer resource.Destroy()
			data, _ := json.Marshal(bounds())
			fmt.Printf("READY backend=%T scale=%g bounds=%s\n", resource, p.PixelScale(), data)
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
