// 上下文菜单、公开树行组合、拖放观察与自动滚动的原生窗口验收。
//
// 环境：Linux/X11、Windows、macOS 可交互桌面；记录 OS/WM、后端和缩放。
// 启动：go run ./tests/window/gui/interactions
// GOUI_PLAT_PAINTER=software/opengl；Windows 另支持 direct2d。
// GOUI_PLAT_SCALE=2 可验证 2x。初始：640×540 DIP、Modern 亮色；第一分支
// 已展开，120 个编号节点；状态 menus/clicks/drops 均为 0，scroll=0。
// clicks 统计选择变化次数，不是原始鼠标事件数。
//
// 操作与预期：
//  1. 点第一行箭头：仅展开/折叠，不执行行点击；若折叠隐藏当前项，选择按树
//     的既有规则回退到父级。点标签选择。箭头、缩进和行右侧
//     留白都能命中相同行。展开后按方向键导航仍可用。
//  2. 在编号行右侧留白右击，菜单显示该节点 ID，menus 恰增加 1。
//     Windows 按下不显示、抬起显示；Linux/macOS 按下显示。macOS Control+
//     左击也显示菜单，不增加普通点击或拖放 Begin。Esc 关闭菜单。
//  3. 选中一行，按 Shift+F10；菜单针对 Current，而不是最后鼠标位置。
//     长按不得重复弹出。Windows/Linux 可再用键盘菜单键验证。Esc 后再次
//     右击应正常。标题栏及按钮原有点击不受影响。
//  4. 从 Drag source 拖至列表中央：有蓝色图片预览，整行出现边框；observer
//     为 true，显示 Target。松开后 drops+1，Drop ID 等于松开时边框所在行，
//     observer 为 false，状态不再变化。拖到列表外不得继续保留行边框。
//  5. 从 Drag source 拖至列表底部边缘，保持鼠标静止至少 1 秒：scroll 持续
//     增加，Target 随滚动更新。移回视口中间应停止滚动；移到顶部可反向滚动。
//     松开后 Drop ID 必须等于松开前 Target。结束或 Esc 后 scroll 保持不变。
//  6. 点 Auto scroll: OFF，重做边缘拖动：仍可放下，但 scroll 不变化。
//     再启用、点 Reset，scroll 回到 0，后续拖动仍正常。
//  7. 原生文件管理器拖文件到列表边缘：自动滚动，松开 drops+1；不修改文件。
//     1x/2x、亮暗切换与窗口大小变化后重复步骤 1–6，命中、菜单和拖放仍正确。
//
// 平台差异：菜单触发时机如步骤 2；原生拖放取消反馈由系统决定。
// 副作用：仅显示菜单和拖放预览，不写文件、剪贴板或系统设置。
// 复位：Reset 或重启；退出：关闭窗口。错误在状态和终端显示 FAIL 并非零退出。
package main

import (
	"fmt"
	"image"
	"image/color"
	"log"
	"os"
	"runtime"
	"time"

	"github.com/golang-gui/goui/core/geometry"
	"github.com/golang-gui/goui/gui"
	"github.com/golang-gui/goui/layout"
	"github.com/golang-gui/goui/platform/graphics"
	"github.com/golang-gui/goui/theme/modern"
	"github.com/golang-gui/goui/widgets"
)

type probe struct {
	tree                           *widgets.TreeView
	model                          *widgets.TreeStore[string]
	scroll                         *gui.ScrollView
	status                         *gui.Label
	target, drop                   string
	menus, clicks, drops, failures int
	observed                       bool
}

type row struct {
	*widgets.TreeExpander
	label *gui.Label
	id    string
	probe *probe
}

func (r *row) Paint(p gui.Painter) {
	if r.id != "" && r.id == r.probe.target {
		p.DrawRoundRect(geometry.Rect(.5, .5, max(0, r.Rect().Width-1), max(0, r.Rect().Height-1)), 4, 1, graphics.RGB(30, 110, 220))
	}
}

func (s *probe) Setup() gui.Widget {
	r := &row{TreeExpander: widgets.NewTreeExpander(), label: gui.NewLabel(""), probe: s}
	r.label.SetStyleName("tree-item-text")
	r.SetChild(r.label)
	r.ConnectToggle(func() {
		if r.id != "" {
			s.tree.SetExpanded(r.id, !s.tree.Expanded(r.id))
		}
	})
	return r
}
func (s *probe) Bind(state widgets.TreeRow, w gui.Widget) {
	r := w.(*row)
	r.id = state.ID
	r.label.SetText(s.model.Item(state.ID))
	r.SetDepth(state.Depth)
	r.SetIndentation(s.tree.Indentation())
	r.SetExpandable(state.Expandable)
	r.SetExpanded(state.Expanded)
}
func (*probe) Unbind(_ widgets.TreeRow, w gui.Widget) {
	r := w.(*row)
	r.id = ""
	r.SetExpandable(false)
}
func (s *probe) feedback(id string) {
	if s.target != id {
		s.target = id
		s.tree.RequestPaint()
		fmt.Printf("target=%s scroll=%.0f\n", id, s.scroll.ScrollY())
	}
}
func (s *probe) fail(err error) { s.failures++; fmt.Printf("FAIL: %v\n", err) }

func main() {
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()
	app, err := gui.NewApplication("org.golang-gui.InteractionProbe")
	if err != nil {
		log.Fatal(err)
	}
	app.SetStyleSheet(modern.Sheet(modern.Options{}))
	window, err := app.NewWindow(&gui.WindowOptions{Size: geometry.Size{Width: 640, Height: 540}, Chrome: gui.WindowChromeIntegrated})
	if err != nil {
		log.Fatal(err)
	}
	defer window.Destroy()
	_ = window.SetTitle("GOUI Interaction Probe")
	s := &probe{tree: widgets.NewTreeView(), model: widgets.NewTreeStore[string](), scroll: gui.NewScrollView(), status: gui.NewLabel("")}
	s.model.Modify(func(m *widgets.TreeStore[string]) {
		m.Append("", "folder", "Folder: click the disclosure arrow")
		for i := 0; i < 120; i++ {
			m.Append("folder", fmt.Sprintf("row-%03d", i), fmt.Sprintf("Item %03d — full row drag/drop", i))
		}
	})
	s.tree.SetModel(s.model)
	s.tree.SetDelegate(s)
	s.tree.SetExpanded("folder", true)
	s.tree.ConnectSelection(func([]string) { s.clicks++ })
	s.tree.ConnectContextMenu(func(id string, result *gui.MenuModel) {
		s.menus++
		fmt.Printf("menu=%s count=%d\n", id, s.menus)
		menu := gui.NewMenu()
		menu.Append("Context: "+id, nil).SetEnabled(false)
		menu.Append("Close menu", func() {})
		*result = menu
	})
	s.tree.ConnectContextMenuError(s.fail)
	s.scroll.SetChild(s.tree)
	target := gui.NewDropTarget(gui.DragFormatText, gui.DragFormatFiles)
	motion := func(e *gui.DragMotion) {
		if r, _, ok := s.tree.RowAt(e.Position); ok {
			s.feedback(r.ID)
		} else {
			s.feedback("")
			e.Action = 0
		}
	}
	target.ConnectEnter(motion)
	target.ConnectMotion(motion)
	target.ConnectLeave(func() { s.feedback("") })
	target.ConnectDrop(func(e *gui.DropRequest) {
		r, _, ok := s.tree.RowAt(e.Position)
		if !ok {
			return
		}
		if r.ID != s.target {
			s.fail(fmt.Errorf("drop=%s but feedback=%s", r.ID, s.target))
			return
		}
		s.drops++
		s.drop = r.ID
		e.Accepted = true
		fmt.Printf("drop=%s count=%d\n", r.ID, s.drops)
	})
	target.ConnectError(s.fail)
	s.tree.AddEventController(target)
	observer := gui.NewDragMotionEventController()
	observer.ConnectEnter(func(geometry.Point) { s.observed = true })
	observer.ConnectLeave(func() { s.observed = false })
	s.scroll.AddEventController(observer)
	root := gui.NewLinearBox(layout.DirectionVertical)
	root.SetCrossAlign(layout.CrossStretch)
	root.SetSpacing(8)
	root.SetPadding(12)
	root.AddChild(gui.NewLabel("Menu / disclosure / drag at viewport edges"))
	toolbar := gui.NewLinearBox(layout.DirectionHorizontal)
	toolbar.SetSpacing(8)
	button := func(text string, fn func()) *gui.Button {
		b := gui.NewButton()
		b.SetChild(gui.NewLabel(text))
		if fn != nil {
			b.ConnectClicked(fn)
		}
		toolbar.AddChild(b)
		return b
	}
	sourceButton := button("Drag source", nil)
	source := gui.NewDragSource()
	preview := image.NewRGBA(image.Rect(0, 0, 48, 32))
	for y := 2; y < 30; y++ {
		for x := 2; x < 46; x++ {
			preview.SetRGBA(x, y, color.RGBA{R: 30, G: 110, B: 220, A: 255})
		}
	}
	source.ConnectPrepare(func(e *gui.DragPrepare) {
		e.Data = new(gui.DragData)
		e.Data.SetText("interaction probe")
		e.Preview = gui.DragPreview{Image: preview, Scale: 1, Hotspot: geometry.Point{X: 12, Y: 12}}
	})
	source.ConnectEnd(func(r gui.DragResult) {
		if r.Err != nil {
			s.fail(r.Err)
		}
		fmt.Printf("end action=%d canceled=%v\n", r.Action, r.Canceled)
	})
	sourceButton.AddEventController(source)
	auto := button("Auto scroll: ON", nil)
	auto.ConnectClicked(func() {
		enabled := !s.scroll.DragAutoScroll()
		s.scroll.SetDragAutoScroll(enabled)
		text := "Auto scroll: OFF"
		if enabled {
			text = "Auto scroll: ON"
		}
		auto.Child().(*gui.Label).SetText(text)
	})
	button("Reset", func() { s.scroll.SetScrollY(0); s.feedback("") })
	dark := false
	button("Theme", func() { dark = !dark; app.SetStyleSheet(modern.Sheet(modern.Options{Dark: dark})) })
	root.AddChild(toolbar)
	root.AddChild(s.scroll)
	root.AddChild(s.status)
	window.SetWidget(root)
	last := ""
	timer := app.NewTimer()
	timer.ConnectTimeout(func() {
		text := fmt.Sprintf("menus=%d clicks=%d drops=%d FAIL=%d\nobserver=%v Target=%s Drop=%s scroll=%.0f", s.menus, s.clicks, s.drops, s.failures, s.observed, s.target, s.drop, s.scroll.ScrollY())
		if text != last {
			last = text
			s.status.SetText(text)
		}
	})
	if err := timer.Start(100 * time.Millisecond); err != nil {
		log.Fatal(err)
	}
	defer timer.Stop()
	if err := window.Show(); err != nil {
		log.Fatal(err)
	}
	app.Run()
	if s.failures > 0 {
		os.Exit(1)
	}
}
