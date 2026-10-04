// TreeView 模型、虚拟化、键盘交互和声明绑定验收，不使用 DevServer。
//
// 环境：Linux/X11、Windows 或 macOS 可交互桌面。记录 OS、Linux WM、实际
// 绘制后端和缩放；仓库根目录启动：
//
//	go run ./tests/window/widgets/tree_view
//
// 默认 UI；-gui 使用纯 GUI。GOUI_PLAT_PAINTER=software/opengl，Windows
// 另可 direct2d；GOUI_PLAT_SCALE=2 请求 2x。-fallback-style 显式组合兜底样式。
// 初始：Native 920×640 DIP、Modern 亮色、多选树，无选中项。项目目录已展开，
// src、延迟加载分支、10000 行分支折叠；src 内有普通、两行和内嵌按钮内容。
//
// 操作与预期：
//  1. 点击行体选择；点击箭头只展开折叠，不选择或激活。展开 src 后双击叶子，
//     终端只打印一次 activate；双击分支只切换展开，不打印 activate。
//  2. Ctrl（macOS Command）单击多选，Shift 单击连续范围；连续 Shift+方向键
//     扩大后反向缩小范围。Primary+方向键只移当前项，Primary+Space 切换选择。
//     Home/End、PageUp/PageDown、左右方向键导航；Primary+A 仅选展开后的节点。
//     折叠包含当前项的分支后，当前项与选择回到该分支，分支外多选保留。
//  3. 展开 Delayed，日志 load 只输出一次，约 500 ms 后出现 loaded child；
//     加载期间折叠，结果不自动展开；再次展开日志可再次 request，但不重复插入。
//  4. 展开 10000 rows，滚轮滚动，行不残留旧文字、选中背景或展开箭头。
//     点击 Reveal last 定位末项，终端输出 reveal；随后向上滚动不会被拉回。
//     缩窄窗口查看水平滚动，纵向定位不重置已有水平偏移。
//  5. 展开 src 后点击 row action，日志只输出 row-action，不改变选择或启动拖动。
//     点击行体后按 Enter 激活；行内按钮持有焦点时树不截获按钮的键盘输入。
//  6. 右击已选行保留多选，右击未选行选择目标；菜单显示目标 ID。点击 Activate
//     输出目标 ID；点击 Delete 移除该节点及后代，当前项合理回退。Shift+F10
//     菜单属于当前项；macOS 另验 Control+单击。空白菜单为 tree background。
//  7. 点击 Rebuild（GUI 为 Refresh），已有选择、当前项、展开和滚动位置保留；
//     行内文字带更新后的版本号。Toggle dark 切换主题，不改变上述状态。
//     选中行悬停／按下时背景有轻微变化；箭头仅在自身被悬停／按下时响应样式，
//     松手和滚动复用后不残留按下背景。当前项细边框不遮住选中背景。
//  8. 点击 Snapshot，检查层级、兄弟序号、当前/选择及虚拟化数量断言，输出
//     ASSERT PASS；这仅代表语义断言，不代替实际输入和外观验收。
//
// 平台差异：Primary 按平台转换；macOS 先激活窗口。没有全局 Tab 焦点导航；
// 长行按自然宽度水平滚动，不自动折行。GUI Refresh 对应 UI 的同模型重建。
// 副作用：本例窗口、内存数据、计时器和终端日志；不访问磁盘、剪贴板或网络。
// 复位：重启。退出：关闭窗口。语义断言失败 panic 并返回非零。
package main

import (
	"flag"
	"fmt"
	"log"
	"runtime"
	"slices"
	"time"

	"github.com/golang-gui/goui/core/geometry"
	"github.com/golang-gui/goui/gui"
	"github.com/golang-gui/goui/layout"
	"github.com/golang-gui/goui/style"
	"github.com/golang-gui/goui/theme/modern"
	"github.com/golang-gui/goui/ui"
	"github.com/golang-gui/goui/widgets"
	widgetstyle "github.com/golang-gui/goui/widgets/style"
	wui "github.com/golang-gui/goui/widgets/ui"
)

type entry struct {
	title  string
	action bool
}
type model struct {
	*widgets.TreeStore[entry]
	pending bool
	delay   func(func())
}

func newModel() *model {
	m := &model{TreeStore: widgets.NewTreeStore[entry]()}
	m.Modify(func(s *widgets.TreeStore[entry]) {
		s.Append("", "project", entry{title: "Project"})
		s.Append("project", "src", entry{title: "src"})
		s.Append("src", "main", entry{title: "main.go"})
		s.Append("src", "multiline", entry{title: "Two-line row\nSecond line remains fully visible"})
		s.Append("src", "action", entry{title: "Interactive row", action: true})
		s.Append("project", "readme", entry{title: "README.md"})
		s.Append("", "delayed", entry{title: "Delayed (500 ms)"})
		s.SetExpandable("delayed", true)
		s.Append("", "large", entry{title: "10000 rows"})
		for i := 0; i < 10000; i++ {
			s.Append("large", fmt.Sprintf("record-%05d", i), entry{title: fmt.Sprintf("Record %05d", i)})
		}
		s.Append("", "long", entry{title: "Long natural-width row: abcdefghijklmnopqrstuvwxyz 0123456789 — horizontal scrolling preserves the complete text"})
	})
	return m
}
func (m *model) RequestChildren(id string) {
	if id != "delayed" {
		return
	}
	fmt.Printf("load-request=%s\n", id)
	if m.pending || len(m.Children(id)) != 0 || m.delay == nil {
		return
	}
	m.pending = true
	m.delay(func() {
		m.pending = false
		if _, exists := m.Parent(id); exists && len(m.Children(id)) == 0 {
			m.Append(id, "loaded", entry{title: "loaded child"})
		}
	})
}
func sheet(dark, fallback bool) style.StyleSheet {
	if fallback {
		return style.Sheet(append(gui.DefaultStyleRules(), widgetstyle.Rules()...)...)
	}
	return modern.Sheet(modern.Options{Dark: dark})
}
func assertTree(tree *widgets.TreeView) {
	info := tree.Snapshot()
	if info.Role != widgets.RoleTree || info.ItemCount < len(info.Children) || len(info.Children) > 80 {
		panic("tree snapshot/virtualization incorrect")
	}
	seen := make(map[string]bool)
	for _, row := range info.Children {
		h := row.Hierarchy
		if row.Role != widgets.RoleTreeItem || h == nil || h.NodeID == "" || seen[h.NodeID] || h.Level < 1 || h.PositionInSet < 1 || h.PositionInSet > h.SetSize || row.Focused {
			panic("row hierarchy/focus incorrect")
		}
		seen[h.NodeID] = true
		if h.Current != (tree.Current() == h.NodeID) {
			panic("current snapshot incorrect")
		}
	}
	fmt.Printf("ASSERT PASS logical=%d realized=%d current=%s selection=%v\n", info.ItemCount, len(info.Children), tree.Current(), tree.Selection())
}
func main() {
	guiMode := flag.Bool("gui", false, "use imperative GUI")
	fallback := flag.Bool("fallback-style", false, "use explicitly composed fallback styles")
	flag.Parse()
	var err error
	if *guiMode {
		runtime.LockOSThread()
		defer runtime.UnlockOSThread()
		err = runGUI(*fallback)
	} else {
		err = runUI(*fallback)
	}
	if err != nil {
		log.Fatal(err)
	}
}
func runUI(fallback bool) error {
	m := newModel()
	version, dark := 0, false
	expanded := []string{"project"}
	return ui.Run("org.golang-gui.TreeViewProbe", func(app ui.App) ui.RootView {
		m.delay = func(fn func()) { app.TimeoutFunc(500*time.Millisecond, fn) }
		tree := wui.TreeView(m, func(row widgets.TreeRow, value entry) ui.View {
			label := ui.Label(fmt.Sprintf("%s [v%d]", value.title, version)).Style("tree-item-text")
			if value.action {
				return ui.HBox(label, ui.Button("row action").OnClick(func() { fmt.Printf("row-action=%s\n", row.ID) })).Spacing(8)
			}
			return label
		}).ID("tree").SelectionMode(widgets.SelectionMultiple).Expanded(expanded).
			OnSelection(func(ids []string) { fmt.Printf("selection=%v\n", ids) }).
			OnCurrent(func(id string) { fmt.Printf("current=%s\n", id) }).
			OnExpanded(func(id string, value bool) {
				if value {
					if !slices.Contains(expanded, id) {
						expanded = append(expanded, id)
					}
				} else {
					expanded = slices.DeleteFunc(expanded, func(next string) bool { return next == id })
				}
				fmt.Printf("expanded=%s:%v\n", id, value)
				app.RequestUpdate()
			}).
			OnActivate(func(id string) { fmt.Printf("activate=%s\n", id) }).
			ContextMenu(func(id string) []*ui.MenuItemView {
				if id == "" {
					return []*ui.MenuItemView{ui.MenuItem("tree background", nil).Enabled(false)}
				}
				return []*ui.MenuItemView{ui.MenuItem(id, nil).Enabled(false), ui.MenuItem("Activate", func() { fmt.Printf("menu-activate=%s\n", id) }), ui.MenuItem("Delete", func() { m.Remove(id) })}
			}).OnContextMenuError(func(err error) { fmt.Printf("menu-error=%v\n", err) })
		return ui.Root().StyleSheet(sheet(dark, fallback)).Windows(
			ui.Window("tree-window").Title("GOUI TreeView UI").Size(920, 640).MinSize(440, 260).Content(
				ui.VBox(
					ui.HBox(
						ui.Button("Rebuild").OnClick(func() { version++; app.RequestUpdate() }),
						ui.Button("Toggle dark").OnClick(func() { dark = !dark; app.RequestUpdate() }),
						ui.Button("Reveal last").OnClick(func() {
							app.FindWidget("tree-window", "tree").(*widgets.TreeView).Reveal("record-09999")
							fmt.Println("reveal=record-09999")
						}),
						ui.Button("Snapshot").OnClick(func() { assertTree(app.FindWidget("tree-window", "tree").(*widgets.TreeView)) }),
					).Spacing(8),
					ui.Label("Click / Primary / Shift; arrows / Enter / Shift+F10; lazy loading and 10000 rows"),
					ui.ScrollView(tree),
				).CrossAlign(layout.CrossStretch).Padding(12).Spacing(8),
			),
		)
	})
}

type delegate struct {
	model   *model
	version *int
	tree    *widgets.TreeView
}
type rowContent struct {
	*widgets.TreeExpander
	label  *gui.Label
	button *gui.Button
	id     string
}

func (d *delegate) Setup() gui.Widget {
	c := &rowContent{TreeExpander: widgets.NewTreeExpander(), label: gui.NewLabel("")}
	box := gui.NewLinearBox(layout.DirectionHorizontal)
	c.SetChild(box)
	c.ConnectToggle(func() {
		if c.id != "" {
			d.tree.SetExpanded(c.id, !d.tree.Expanded(c.id))
		}
	})
	c.label.SetStyleName("tree-item-text")
	box.SetSpacing(8)
	box.AddChild(c.label)
	c.button = gui.NewButton()
	c.button.SetChild(gui.NewLabel("row action"))
	c.button.ConnectClicked(func() { fmt.Printf("row-action=%s\n", c.id) })
	box.AddChild(c.button)
	return c
}
func (d *delegate) Bind(row widgets.TreeRow, widget gui.Widget) {
	c := widget.(*rowContent)
	value := d.model.Item(row.ID)
	c.id = row.ID
	c.SetDepth(row.Depth)
	c.SetIndentation(d.tree.Indentation())
	c.SetExpandable(row.Expandable)
	c.SetExpanded(row.Expanded)
	c.label.SetText(fmt.Sprintf("%s [v%d]", value.title, *d.version))
	c.button.SetVisible(value.action)
}
func (*delegate) Unbind(_ widgets.TreeRow, widget gui.Widget) { widget.(*rowContent).id = "" }
func runGUI(fallback bool) error {
	app, err := gui.NewApplication("org.golang-gui.TreeViewGUIProbe")
	if err != nil {
		return err
	}
	app.SetStyleSheet(sheet(false, fallback))
	window, err := app.NewWindow(&gui.WindowOptions{Size: geometry.Size{Width: 920, Height: 640}})
	if err != nil {
		return err
	}
	defer window.Destroy()
	if err := window.SetTitle("GOUI TreeView GUI"); err != nil {
		return err
	}
	window.SetMinSize(geometry.Size{Width: 440, Height: 260})
	m := newModel()
	m.delay = func(fn func()) {
		timer := app.NewTimer()
		timer.ConnectTimeout(fn)
		if err := timer.StartOnce(500 * time.Millisecond); err != nil {
			panic(err)
		}
	}
	version, dark := 0, false
	tree := widgets.NewTreeView()
	tree.SetID("tree")
	tree.SetModel(m)
	tree.SetDelegate(&delegate{model: m, version: &version, tree: tree})
	tree.SetSelectionMode(widgets.SelectionMultiple)
	tree.SetExpanded("project", true)
	tree.ConnectSelection(func(ids []string) { fmt.Printf("selection=%v\n", ids) })
	tree.ConnectCurrent(func(id string) { fmt.Printf("current=%s\n", id) })
	tree.ConnectExpanded(func(id string, expanded bool) { fmt.Printf("expanded=%s:%v\n", id, expanded) })
	tree.ConnectActivate(func(id string) { fmt.Printf("activate=%s\n", id) })
	tree.ConnectContextMenu(func(id string, result *gui.MenuModel) {
		menu := gui.NewMenu()
		if id == "" {
			menu.Append("tree background", nil).SetEnabled(false)
		} else {
			menu.Append(id, nil).SetEnabled(false)
			menu.Append("Activate", func() { fmt.Printf("menu-activate=%s\n", id) })
			menu.Append("Delete", func() { m.Remove(id) })
		}
		*result = menu
	})
	tree.ConnectContextMenuError(func(err error) { fmt.Printf("menu-error=%v\n", err) })
	root := gui.NewLinearBox(layout.DirectionVertical)
	root.SetCrossAlign(layout.CrossStretch)
	root.SetPadding(12)
	root.SetSpacing(8)
	toolbar := gui.NewLinearBox(layout.DirectionHorizontal)
	toolbar.SetSpacing(8)
	button := func(title string, fn func()) {
		b := gui.NewButton()
		b.SetChild(gui.NewLabel(title))
		b.ConnectClicked(fn)
		toolbar.AddChild(b)
	}
	button("Refresh", func() { version++; tree.Refresh() })
	button("Toggle dark", func() { dark = !dark; app.SetStyleSheet(sheet(dark, fallback)) })
	button("Reveal last", func() { tree.Reveal("record-09999"); fmt.Println("reveal=record-09999") })
	button("Snapshot", func() { assertTree(tree) })
	root.AddChild(toolbar)
	root.AddChild(gui.NewLabel("Click / Primary / Shift; arrows / Enter / Shift+F10; lazy loading and 10000 rows"))
	scroll := gui.NewScrollView()
	scroll.SetChild(tree)
	root.AddChild(scroll)
	window.SetWidget(root)
	if err := window.Show(); err != nil {
		return err
	}
	app.Run()
	return nil
}
