// 虚拟化树的拖拽会话归属回归，不使用DevServer。
//
// 环境：Linux/X11（记录WM）、Windows或macOS交互桌面，记录后端和1x/2x。
// 启动：go run ./tests/window/widgets/tree_drag_lifetime [-basic] [-dark]
// GOUI_PLAT_PAINTER=software/opengl/direct2d，GOUI_PLAT_SCALE=1/2。
// 初始：200个根节点，仅可见数行；树本身声明DragSource，行内容没有源。
//
// 操作与逐项预期：
//  1. 按住首行文字，拖到树底部内侧约8 DIP并停留。只能靠正常原生拖放
//     驱动ScrollView自动滚动，不用滚轮、SetScrollY、Reveal或额外计时器滚动。
//     日志CHECK recycled证明源行已离屏、同一行壳被另一节点复用；随后自动重建，
//     CHECK rebuilt要求TreeView与DragSource身份不变，行不伪造Dragging语义。
//     将指针移回树中部再释放。Drop必须收到原ID，End为Move且只发一次。
//  2. READY phase=1后再拖任意可见行。Motion通过应用更新真正移除树，原生会话
//     应取消；旧声明式OnEnd不得再调用。应用级GUI终态探针确认Canceled且只一次，
//     随后恢复一棵新树，输出CHECK unmounted及READY phase=2。
//  3. 在恢复的树中短拖另一行并释放。必须成功，证明旧会话不再占用Application。
//     F8要求两个成功Drop、一次卸载取消及全部检查完成，PASS、退出0。
//     F6打印当前公开几何及计数，提前关闭或任一检查失败退出1。
//
// 负对照：加-row-source将完全相同的拖拽源声明挂到每行Label；执行第1步应在行
// 回收时报告FAIL source row recycled...并退出1，不会宣称该用法支持虚拟化。
// macOS功能键可能需Fn；Windows原生拖放有嵌套消息循环，以终态而非Begin返回判断结束。
// 拖放只传稳定ID，不移动模型；Local值不深拷贝，不保存行Widget作为业务载荷。
// 探针持有行壳仅为公开身份断言，所属应用负责探针连接，旧UI解绑后也能观察原生收尾。
// 副作用：本例窗口、内存模型和日志，无文件／剪贴板／系统设置修改。复位：重启。
package main

import (
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"image"
	"os"

	"github.com/golang-gui/goui/core/geometry"
	"github.com/golang-gui/goui/core/signal"
	"github.com/golang-gui/goui/gui"
	"github.com/golang-gui/goui/layout"
	"github.com/golang-gui/goui/theme/basic"
	"github.com/golang-gui/goui/theme/modern"
	"github.com/golang-gui/goui/ui"
	"github.com/golang-gui/goui/widgets"
	wui "github.com/golang-gui/goui/widgets/ui"
)

const format = "tree-node"

func main() {
	rowSource := flag.Bool("row-source", false, "negative control: attach sources to recycled rows")
	fallback := flag.Bool("basic", false, "Basic theme")
	dark := flag.Bool("dark", false, "dark Modern palette")
	flag.Parse()
	if err := run(*rowSource, *fallback, *dark); err != nil {
		fmt.Fprintln(os.Stderr, "FAIL", err)
		os.Exit(1)
	}
}

func run(rowSource, fallback, dark bool) error {
	model := widgets.NewTreeStore[string]()
	model.Modify(func(m *widgets.TreeStore[string]) {
		for i := range 200 {
			m.Append("", fmt.Sprintf("n%03d", i), fmt.Sprintf("Record %03d", i))
		}
	})
	phase, builds, begins, uiEnds, drops := 0, 0, 0, 0, 0
	probeEnds := [3]int{}
	mounted, recycled, rebuilt, finished := true, false, false, false
	rebuildRequested, removeRequested := false, false
	ready := map[int]bool{}
	var failure error
	var sourceID string
	var sourceShell gui.Widget
	var sourceTree *widgets.TreeView
	var sourceController *gui.DragSource
	var probe signal.Handle
	var preparedBuild int
	err := ui.Run("org.golanggui.TreeDragLifetime", func(app ui.App) ui.RootView {
		builds++
		version := builds
		findTree := func() *widgets.TreeView {
			t, _ := app.FindWidget("main", "tree").(*widgets.TreeView)
			return t
		}
		fail := func(err error) { failure = err; app.Quit() }
		controller := func(w gui.Widget) *gui.DragSource {
			if w != nil {
				for _, c := range w.EventControllers() {
					if s, ok := c.(*gui.DragSource); ok {
						return s
					}
				}
			}
			return nil
		}
		shell := func(t *widgets.TreeView, info gui.WidgetInfo) gui.Widget {
			v := t.Snapshot().Bounds
			left, top := max(v.X, info.Bounds.X), max(v.Y, info.Bounds.Y)
			right := min(v.X+v.Width, info.Bounds.X+info.Bounds.Width)
			bottom := min(v.Y+v.Height, info.Bounds.Y+info.Bounds.Height)
			w := gui.Pick(t, geometry.Point{X: (left+right)/2 - v.X, Y: (top+bottom)/2 - v.Y})
			for w != nil && w.Snapshot().Role != widgets.RoleTreeItem {
				w = w.Parent()
			}
			return w
		}
		bounds := func() map[string]geometry.Rectangle {
			out := map[string]geometry.Rectangle{}
			if tree := findTree(); tree != nil {
				v := tree.Snapshot()
				if v.Bounds.Width < 100 || v.Bounds.X <= 0 || v.Bounds.Height <= 0 {
					return out
				}
				out["tree"] = v.Bounds
				out["edge"] = geometry.Rect(v.Bounds.X+200, v.Bounds.Y+v.Bounds.Height-8, 1, 1)
				out["drop"] = geometry.Rect(v.Bounds.X+220, v.Bounds.Y+v.Bounds.Height/2, 1, 1)
				for _, info := range v.Children {
					if info.Bounds.Y >= v.Bounds.Y && info.Bounds.Y+info.Bounds.Height <= v.Bounds.Y+v.Bounds.Height {
						out["source"] = geometry.Rect(info.Bounds.X+70, info.Bounds.Y+info.Bounds.Height/2, 1, 1)
						break
					}
				}
			}
			return out
		}
		prepare := func(id string, r *ui.DragPrepare) {
			tree := findTree()
			if tree == nil || phase > 2 {
				return
			}
			if id == "" {
				row, _, ok := tree.RowAt(r.Position)
				if !ok {
					return
				}
				id = row.ID
			}
			for _, info := range tree.Snapshot().Children {
				if info.Attributes[widgets.HierarchyInfoKey].(widgets.HierarchyInfo).NodeID == id {
					sourceShell = shell(tree, info)
					break
				}
			}
			owner := gui.Widget(tree)
			if rowSource {
				owner = app.FindWidget("main", "label-"+id)
			}
			sourceController, sourceTree, sourceID, preparedBuild = controller(owner), tree, id, version
			if sourceController == nil || sourceShell == nil {
				fail(errors.New("source controller or shell unavailable"))
				return
			}
			if probe != nil {
				probe.Disconnect()
			}
			sessionPhase := phase
			probe = sourceController.ConnectEnd(func(result gui.DragResult) {
				probeEnds[sessionPhase]++
				if probeEnds[sessionPhase] != 1 || result.Err != nil {
					fail(fmt.Errorf("phase=%d terminal count=%d result=%+v", sessionPhase, probeEnds[sessionPhase], result))
					return
				}
				if sessionPhase == 0 && result.Canceled {
					scroll := app.FindWidget("main", "scroll").(*gui.ScrollView)
					fail(fmt.Errorf("source row recycled: canceled=%t rowSource=%t scrollY=%g", result.Canceled, rowSource, scroll.ScrollY()))
					return
				}
				if sessionPhase == 1 {
					if !result.Canceled || result.Action != 0 || result.LocalDrop {
						fail(fmt.Errorf("unmounted tree result=%+v", result))
						return
					}
					app.Post(func() {
						if mounted || findTree() != nil || uiEnds != 1 || sourceController.Dragging() {
							fail(errors.New("unmount retained the tree, called old UI End or retained Dragging"))
							return
						}
						fmt.Println("CHECK unmounted canceled once; old UI callback disconnected")
						phase, mounted = 2, true
						app.RequestUpdate()
					})
				}
			})
			data := new(ui.DragData)
			data.SetLocal(format, id)
			r.Data = data
			fmt.Printf("PREPARE phase=%d id=%s build=%d\n", phase, id, version)
		}
		source := func(id string) *ui.DragSourceView {
			return ui.DragSource().Actions(ui.DragMove).OnPrepare(func(r *ui.DragPrepare) { prepare(id, r) }).
				OnBegin(func() { begins++; fmt.Printf("BEGIN phase=%d id=%s\n", phase, sourceID) }).
				OnEnd(func(result ui.DragResult) {
					uiEnds++
					if phase == 1 || result.Err != nil || result.Canceled || result.Action != ui.DragMove || !result.LocalDrop {
						fail(fmt.Errorf("UI End phase=%d result=%+v", phase, result))
						return
					}
					if phase == 0 && (!recycled || !rebuilt || version <= preparedBuild) {
						fail(errors.New("first End did not use latest rebuilt callback after recycling"))
						return
					}
					fmt.Printf("CHECK success phase=%d id=%s latestBuild=%d\n", phase, sourceID, version)
					phase++
					app.RequestUpdate()
				})
		}
		motion := func(r *ui.DragMotion) {
			id, ok := r.Local(format)
			if !ok || id != sourceID {
				fail(errors.New("Motion did not retain the original model ID"))
				return
			}
			r.Action = ui.DragMove
			tree := findTree()
			if tree == nil {
				return
			}
			if phase == 0 {
				visible, reused := false, false
				for _, info := range tree.Snapshot().Children {
					id := info.Attributes[widgets.HierarchyInfoKey].(widgets.HierarchyInfo).NodeID
					visible = visible || id == sourceID
					row := shell(tree, info)
					reused = reused || row == sourceShell && id != sourceID
					if info.DragDrop != nil && info.DragDrop.Dragging {
						fail(errors.New("recycled row impersonates the tree's drag state"))
						return
					}
				}
				if !visible && reused && !recycled {
					recycled = true
					fmt.Printf("CHECK recycled original=%s scrollY=%g\n", sourceID, app.FindWidget("main", "scroll").(*gui.ScrollView).ScrollY())
				}
				if recycled && !rebuildRequested {
					rebuildRequested = true
					app.RequestUpdate()
				}
				if recycled && builds > preparedBuild && !rebuilt {
					if tree != sourceTree || controller(tree) != sourceController || !tree.Snapshot().DragDrop.Dragging {
						fail(errors.New("rebuild replaced the owning tree/source or lost active semantics"))
						return
					}
					rebuilt = true
					fmt.Println("CHECK rebuilt stable tree and source")
				}
			} else if phase == 1 && !removeRequested {
				removeRequested = true
				app.Post(func() { mounted = false; app.RequestUpdate() })
			}
		}
		var body ui.View = ui.Label("Tree unmounted; waiting for native cancellation")
		if mounted {
			tree := wui.TreeView(model, func(row widgets.TreeRow, text string) ui.View {
				label := ui.Label(text).ID("label-" + row.ID).Style("tree-item-text")
				if rowSource {
					label.DragSource(source(row.ID))
				}
				return label
			}).ID("tree")
			if !rowSource {
				tree.DragSource(source(""))
			}
			tree.DropTarget(ui.DropTarget(ui.LocalFormat(format)).Actions(ui.DragMove).OnEnter(motion).OnMotion(motion).
				OnDrop(func(r *ui.DropRequest) {
					id, ok := r.Data.Local(format)
					if !ok || id != sourceID || phase == 0 && (!recycled || !rebuilt) {
						fail(errors.New("Drop changed the payload or skipped recycling/rebuild"))
						return
					}
					r.Accepted = true
					drops++
					fmt.Printf("DROP phase=%d original=%s\n", phase, sourceID)
				}).OnError(fail))
			body = ui.ScrollView(tree).ID("scroll").Style("text-view").Direction(ui.ScrollVertical).
				MinSize(520, 240).MaxSize(520, 240).MainWeight(0)
		}
		content := ui.VBox(ui.Label(fmt.Sprintf("Phase %d: 0 scroll/recycle/drop; 1 unmount; 2 fresh drag; 3 F8 finish", phase)), body,
			ui.Label("Hold inside bottom edge until CHECK recycled + rebuilt; move to middle; release. F6 facts; F8 finish.")).Spacing(12).Padding(16).CrossAlign(layout.CrossStart)
		sheet := modern.Sheet(modern.Options{Dark: dark})
		if fallback {
			sheet = basic.Sheet()
		}
		return ui.Root().StyleSheet(sheet).Windows(ui.Window("main").Title("GOUI Tree drag lifetime").Size(900, 390).Content(content).
			Shortcuts(ui.Shortcut(ui.KeyF6).OnActivate(func() {
				data, _ := json.Marshal(bounds())
				fmt.Printf("CHECK phase=%d begins=%d ends=%v uiEnds=%d drops=%d bounds=%s\n", phase, begins, probeEnds, uiEnds, drops, data)
			}), ui.Shortcut(ui.KeyF8).OnActivate(func() {
				if phase != 3 || begins != 3 || uiEnds != 2 || drops != 2 || probeEnds != [3]int{1, 1, 1} || !recycled || !rebuilt {
					fail(fmt.Errorf("incomplete phase=%d begins=%d ends=%v uiEnds=%d drops=%d", phase, begins, probeEnds, uiEnds, drops))
					return
				}
				finished = true
				fmt.Println("PASS tree drag lifetime")
				app.Quit()
			})).OnAfterPaint(func(p gui.Painter, _ geometry.Rectangle) {
			if ready[phase] || !mounted || findTree() == nil || len(bounds()) < 4 {
				return
			}
			resource, err := p.NewImage(image.NewRGBA(image.Rect(0, 0, 1, 1)))
			if err != nil {
				failure = err
				app.Post(app.Quit)
				return
			}
			defer resource.Destroy()
			ready[phase] = true
			data, _ := json.Marshal(bounds())
			fmt.Printf("READY phase=%d backend=%T scale=%g rowSource=%t bounds=%s\n", phase, resource, p.PixelScale(), rowSource, data)
		}))
	})
	if probe != nil {
		probe.Disconnect()
	}
	if err != nil {
		return err
	}
	if failure != nil {
		return failure
	}
	if !finished {
		return errors.New("closed before completing all stages")
	}
	return nil
}
