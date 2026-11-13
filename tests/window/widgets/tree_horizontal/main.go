// 虚拟化树的水平内容滚动、固定行区域与RowInsets原生回归，不使用DevServer。
//
// 环境：桌面Linux/X11、Windows或macOS，记录WM、实际Painter与1x/2x。
// 启动：go run ./tests/window/widgets/tree_horizontal [-basic] [-dark]
// 使用GOUI_PLAT_PAINTER及GOUI_PLAT_SCALE选择后端／倍率。初始宽行、Short、Branch
// 和300个短根节点，末尾Late wide更长；Branch展开并包含可编辑输入框。
//
// 操作与预期：
//  1. F6检查初态；把水平滚动条拖到最右，再F6。点击Short所在行的可见空白，
//     F6：仍可选中，高亮／current框覆盖行视口，文字可以滚出左侧。
//  2. Insets启用左右12/18 DIP边距，F6；水平滚动往返后边距保持固定，边距不
//     命中行。Omit恢复零边距，F6；再Insets。所有变化由正常声明式协调完成。
//  3. 滚回最左，点击Branch的输入框、Ctrl+A，F6；Rebuild和Dark后分别F6，
//     树身份、输入焦点和选区保留。Basic不随Dark改变自身配色。
//  4. Far、F6：末尾长行首次实现后扩大范围；Back、F6：离屏仍保留该范围。
//     Trim、F6：未实现的Late宽度暂存为估计；再Far、F6：重新测量后收缩；
//     Back回顶部。此流程不保证从未实现的行已有精确宽度。
//  5. 水平条再次拖到最右；从Short可见空白拖到Branch的可见空白，释放后F7。
//     原生拖放应仍解析原始Short节点，目标接受一次；拖动content横线按内容坐标
//     裁剪，结束后清除。不改变模型顺序。
//  6. F8要求全部阶段完成，PASS且退出0；提前关闭或断言失败退出1。
//
// F6仅检查公开几何、语义及当前分配的像素；输入仍需真实键鼠。所选行的色带采样
// 使用不透明sRGB预乘常量、避开圆角及文字，无字体金图；只允许1级8位舍入。
// 窄视口裁空、无界约束、尺寸缓存和重入由包内独立断言覆盖。
// 副作用：测试窗口、内存模型和日志，不写文件、剪贴板或系统设置。复位：重启。
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
	"strings"

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

var selection = color.RGBA{R: 35, G: 130, B: 95, A: 255}

func main() {
	fallback := flag.Bool("basic", false, "Basic theme")
	dark := flag.Bool("dark", false, "Dark Modern theme")
	flag.Parse()
	if err := run(*fallback, *dark); err != nil {
		fmt.Fprintln(os.Stderr, "FAIL", err)
		os.Exit(1)
	}
}

func run(fallback, dark bool) error {
	model := widgets.NewTreeStore[string]()
	model.Modify(func(m *widgets.TreeStore[string]) {
		m.Append("", "wide", strings.Repeat("Wide content / ", 12))
		m.Append("", "short", "Short")
		m.Append("", "branch", "Branch")
		m.Append("branch", "input", "Editor")
		for i := range 300 {
			m.Append("", fmt.Sprintf("r%03d", i), fmt.Sprintf("Record %03d", i))
		}
		m.Append("", "late", strings.Repeat("Late widest content / ", 24))
	})
	ready, finished, inset, omit, trimmed, far := false, false, false, true, false, false
	builds, drops, ends := 0, 0, 0
	target, source := "", ""
	initialWidth, widest := float32(0), float32(0)
	seen := map[string]bool{}
	var failure error
	var original gui.Widget
	err := ui.Run("org.golanggui.TreeHorizontal", func(app ui.App) ui.RootView {
		builds++
		tree := func() *widgets.TreeView { return app.FindWidget("main", "tree").(*widgets.TreeView) }
		scroll := func() *gui.ScrollView { return app.FindWidget("main", "scroll").(*gui.ScrollView) }
		bounds := func() map[string]geometry.Rectangle {
			out := map[string]geometry.Rectangle{}
			for _, id := range []string{"tree", "scroll", "insets", "omit", "rebuild", "dark", "far", "back", "trim", "editor"} {
				if w := app.FindWidget("main", id); w != nil {
					out[id] = w.Snapshot().Bounds
				}
			}
			for _, info := range tree().Snapshot().Children {
				h := info.Attributes[widgets.HierarchyInfoKey].(widgets.HierarchyInfo)
				out["row-"+h.NodeID] = info.Bounds
			}
			for _, child := range scroll().Children() {
				if bar, ok := child.(*gui.ScrollBar); ok && bar.Orientation() == layout.DirectionHorizontal {
					out["hbar"] = bar.Snapshot().Bounds
				}
			}
			return out
		}
		fail := func(err error) { failure = err; app.Quit() }
		check := func() error {
			v, sv := tree(), scroll()
			if !ready || v != original || sv.ScrollY() < 0 {
				return errors.New("not ready or tree replaced")
			}
			vp := v.Snapshot().Bounds
			left, right := v.RowInsets()
			if inset && !omit && (left != 12 || right != 18) || omit && (left != 0 || right != 0) {
				return fmt.Errorf("insets=%g/%g inset=%t omit=%t", left, right, inset, omit)
			}
			for _, info := range v.Snapshot().Children {
				if math.Abs(float64(info.Bounds.X-vp.X-left)) > 0.01 || math.Abs(float64(info.Bounds.Width-(vp.Width-left-right))) > 0.01 {
					return fmt.Errorf("row extent=%v viewport=%v", info.Bounds, vp)
				}
			}
			b := bounds()
			if row, ok := b["row-short"]; ok {
				position := geometry.Point{X: vp.Width / 2, Y: row.Y - vp.Y + row.Height/2}
				r, actual, hit := v.RowAt(position)
				if !hit || r.ID != "short" || actual.Width != row.Width {
					return errors.New("short row cannot be hit in the viewport")
				}
				if sv.ScrollX() > 0 {
					seen["right"] = true
					if len(v.Selection()) == 1 && v.Selection()[0] == "short" {
						pixels, err := gui.RenderWidget(v, 1)
						if err != nil {
							return err
						}
						x, y := int(vp.Width-left-right-12+left), int(position.Y)
						got := color.RGBAModel.Convert(pixels.At(x, y)).(color.RGBA)
						for i, value := range []uint8{got.R, got.G, got.B, got.A} {
							want := []uint8{selection.R, selection.G, selection.B, selection.A}[i]
							if int(value) < int(want)-1 || int(value) > int(want)+1 {
								return fmt.Errorf("selected short row pixel=%v want=%v", got, selection)
							}
						}
						seen["short"] = true
					}
				}
				if sv.ScrollX() == 0 {
					seen["left"] = true
				}
			}
			if inset && !omit {
				seen["insets"] = true
				if _, _, hit := v.RowAt(geometry.Point{X: 1, Y: 14}); hit {
					return errors.New("viewport margin hits a row")
				}
			}
			if omit && seen["insets"] {
				seen["omit"] = true
			}
			if editor, ok := app.FindWidget("main", "editor").(*gui.TextInput); ok && editor.Focused() {
				if editor.Selection().Anchor != 0 || editor.Selection().Caret != len(editor.Text()) {
					return errors.New("editor selection lost")
				}
				seen["editor"] = true
			} else if seen["editor"] && !far && !seen["far"] {
				return errors.New("rebuild or palette change lost editor focus")
			}
			if far {
				if _, ok := b["row-late"]; !ok {
					return errors.New("far did not reach the last row")
				}
				if !trimmed {
					widest = v.ContentSize().Width
					if widest <= initialWidth {
						return errors.New("unvisited wide row did not grow extent")
					}
					seen["far"] = true
				} else {
					if v.ContentSize().Width >= widest || v.ContentSize().Width < initialWidth {
						return fmt.Errorf("remeasured width=%g initial=%g widest=%g", v.ContentSize().Width, initialWidth, widest)
					}
					seen["shrink"] = true
				}
			} else if seen["far"] && !seen["shrink"] {
				if v.ContentSize().Width != widest {
					return fmt.Errorf("lost offscreen estimate %g/%g", v.ContentSize().Width, widest)
				}
				seen["back"] = true
				if trimmed {
					seen["estimate"] = true
				}
			}
			seen["initial"] = true
			data, _ := json.Marshal(b)
			fmt.Printf("CHECK builds=%d scrollX=%g maxX=%g extent=%g inset=%t omit=%t far=%t trim=%t bounds=%s\n", builds, sv.ScrollX(), sv.Snapshot().MaxScrollX, v.ContentSize().Width, inset, omit, far, trimmed, data)
			return nil
		}
		button := func(id, title string, fn func()) ui.View {
			return ui.Button(title).ID(id).Focusable(false).FocusOnPointer(false).OnClick(fn)
		}
		view := wui.TreeView(model, func(row widgets.TreeRow, title string) ui.View {
			if row.ID == "input" {
				return ui.TextInput().ID("editor").Text("keep selection").MinSize(180, 0)
			}
			return ui.HBox(ui.Label("■").Style("tree-item-icon"), ui.Label(title).Style("tree-item-text")).Spacing(8)
		}).ID("tree")
		if !ready {
			view.Expanded([]string{"branch"})
		}
		if !omit {
			view.RowInsets(12, 18)
		}
		view.RowStyle(func(row widgets.TreeRow, _ string) string {
			if row.ID == target {
				return "drop-row"
			}
			return ""
		})
		motion := func(m *ui.DragMotion) {
			if row, _, ok := tree().RowAt(m.Position); ok {
				target, m.Action = row.ID, ui.DragMove
				app.RequestUpdate()
			}
		}
		view.DragSource(ui.DragSource().Actions(ui.DragMove).OnPrepare(func(r *ui.DragPrepare) {
			if row, _, ok := tree().RowAt(r.Position); ok {
				source = row.ID
				r.Data = new(ui.DragData)
				r.Data.SetLocal("tree-horizontal", source)
			}
		}).OnEnd(func(r ui.DragResult) {
			ends++
			if r.Err != nil {
				failure = r.Err
			}
			target = ""
			app.RequestUpdate()
		})).DropTarget(ui.DropTarget(ui.LocalFormat("tree-horizontal")).Actions(ui.DragMove).
			OnEnter(motion).OnMotion(motion).OnError(fail).OnLeave(func() { target = ""; app.RequestUpdate() }).
			OnDrop(func(r *ui.DropRequest) { r.Accepted = true; drops++ }))
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
				rules = append(rules, style.Name(name).Part("selected").State(state).BackgroundColor(selection))
			}
		}
		rules = append(rules, style.Name("drop-row").Part("content").BackgroundColor(color.Transparent).BorderColor(selection).BorderWidth(2).BorderSides(style.BorderTop).Radius(0))
		content := ui.VBox(
			ui.HBox(button("insets", "Insets", func() { inset, omit = true, false; app.RequestUpdate() }), button("omit", "Omit", func() { omit = true; app.RequestUpdate() }),
				button("rebuild", "Rebuild", func() { seen["rebuild"] = true; app.RequestUpdate() }), button("dark", "Dark", func() { dark = !dark; seen["palette"] = true; app.RequestUpdate() }),
				button("far", "Far", func() { far = true; tree().Reveal("late") }), button("back", "Back", func() { far = false; tree().Reveal("wide") }),
				button("trim", "Trim", func() { trimmed = true; model.Set("late", "Small") })).Spacing(8),
			ui.ScrollView(view).ID("scroll").Style("text-view").MinSize(620, 260).MaxSize(620, 260),
			ui.Label("Drag horizontal bar; click Short whitespace; F6 check; native Short to Branch drag; F7; F8 finish"),
		).Spacing(12).Padding(16).CrossAlign(layout.CrossStart)
		return ui.Root().StyleSheet(style.Sheet(rules...)).Windows(ui.Window("main").Title("GOUI Tree horizontal").Size(980, 400).Content(content).Shortcuts(
			ui.Shortcut(ui.KeyF6).OnActivate(func() {
				if err := check(); err != nil {
					fail(err)
				}
			}),
			ui.Shortcut(ui.KeyF7).OnActivate(func() {
				if source != "short" || drops != 1 || ends != 1 || failure != nil {
					fail(fmt.Errorf("native drag source=%s drops=%d ends=%d err=%v", source, drops, ends, failure))
					return
				}
				seen["drag"] = true
				fmt.Println("CHECK drag")
			}),
			ui.Shortcut(ui.KeyF8).OnActivate(func() {
				for _, stage := range []string{"initial", "right", "short", "insets", "omit", "left", "editor", "rebuild", "palette", "far", "back", "estimate", "shrink", "drag"} {
					if !seen[stage] {
						fail(fmt.Errorf("missing stage %s", stage))
						return
					}
				}
				finished = true
				fmt.Println("PASS tree horizontal")
				app.Quit()
			}),
		).OnAfterPaint(func(p gui.Painter, _ geometry.Rectangle) {
			if ready || tree().Rect().Width <= 0 || tree().Snapshot().Bounds.X <= 0 || scroll().Snapshot().MaxScrollX <= 0 {
				return
			}
			original, initialWidth, ready = tree(), tree().ContentSize().Width, true
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
