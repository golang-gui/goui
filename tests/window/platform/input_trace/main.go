// 三平台原生输入与 GOUI 事件对照记录；不使用 DevServer。
//
// 环境：Windows 10/11 amd64、Linux/X11 或 macOS 可交互桌面。
// 启动：go run ./tests/window/platform/input_trace
// 构建：go build -o input-trace ./tests/window/platform/input_trace
// Windows 将输出文件名改为 input-trace.exe。
// 参数：-log 指定新日志文件（不覆盖）；-note 填设备、系统版本、驱动和远程客户端说明。
// 默认写入临时目录中的 goui-input-trace-*.log，路径同时显示在窗口与终端。
// 初始：左侧 Project 已展开且树持有焦点，右侧表格有双轴滚动范围。
// 等待“记录就绪”后操作；左侧树与右侧表格的 ID 分别为 tree 和 table。
//
// 操作与预期：
//  1. 点击“触摸板”标记，再点击左侧 Project 行；把指针移到右侧表格行体。
//     分别双指上下、左右滚动约三秒。预期仅表格滚动，树的选择与展开保持不变。
//     本例用于诊断现有行为，出现不符时按 F7 记录状态，不自动补偿输入。
//  2. 点击“鼠标”标记，重复滚轮、Shift+滚轮和可用时的左右倾斜滚轮。
//     预期按指针命中区域滚动，日志保留增量、单位和实际输入类型。
//  3. 使用 VNC/RDP 重复以上操作，先点击“远程”标记。
//     不预设远程客户端保留真实设备身份、手势阶段或增量精度。
//  4. 点击“键盘”标记，点击 Project 后按左右方向键，作为真实按键对照。
//     预期树展开/折叠，日志中有原生键码及 GOUI KeyEvent。
//  5. F6 分段标记；F7 记录焦点、树状态、双轴偏移及范围；F8 重置。
//     标记按钮会取得焦点，开始每组滚动前须重新点击树。正常关闭后发送日志。
//
// 平台记录边界：
// Windows：仅 GUI 线程的 WH_GETMESSAGE / WH_CALLWNDPROC，不安装全局钩子。
// 同一消息可在 queue 和 wndproc 各出现一次；queue 包含被 GOUI 提前处理的按键。
// Linux：例程要求 OpenGL；首次绘制时从 GLX 取得本进程的 Xlib 连接，临时包装
// XESetWireToEvent 并原样调用、恢复原转换器。记录此后解码的 2..127 类事件，
// 包括未被后端使用的事件；不修改选择掩码。raw 是 32 字节线格式头，扩展载荷未展开。
// 不包含安装前已解码的队列事件，也不观察 Wayland、原始 HID 或其他 X 客户端。
// macOS：进程内 NSEvent local monitor，保留原事件；不含 NSWindow 通知、
// 原始 HID，以及 AppKit 未交给 local monitor 的事件，不申请辅助功能权限。
// NATIVE 保留各平台字段；GOUI 在 Widget capture 阶段记录输入，不消费事件；
// STATE 在派发后的任务中记录结果。lastNative 仅表示最近原生记录，不能作为严格
// 一对一对应关系（Xlib 可提前解码多个事件、Windows 可重入派发）。
// 副作用：创建窗口、日志和进程内观察器；日志含按键及坐标，请勿输入密码。
// 日志可能影响时序，不用于性能测量。退出时卸载观察器并检查写入错误。
// 复位：F8 或重启。退出：关闭窗口。
package main

import (
	"errors"
	"flag"
	"fmt"
	"os"
	"time"

	"github.com/golang-gui/goui/core/geometry"
	"github.com/golang-gui/goui/gui"
	"github.com/golang-gui/goui/layout"
	"github.com/golang-gui/goui/platform/events"
	"github.com/golang-gui/goui/theme/modern"
	"github.com/golang-gui/goui/widgets"
)

func main() {
	path := flag.String("log", "", "新日志路径；默认临时文件，不覆盖旧文件")
	note := flag.String("note", "", "系统版本、设备、驱动及远程客户端说明")
	flag.Parse()
	if err := run(*path, *note); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func run(path, note string) (err error) {
	if err = prepareNativeTrace(); err != nil {
		return err
	}
	out, err := newTraceLog(path, note)
	if err != nil {
		return err
	}
	defer func() { err = errors.Join(err, out.close()) }()
	fmt.Println("日志文件：", out.file.Name())
	app, err := gui.NewApplication("org.golanggui.InputTrace")
	if err != nil {
		return err
	}
	app.SetStyleSheet(modern.FromSettings(app.Settings()))
	w, err := app.NewWindow(&gui.WindowOptions{Size: geometry.Size{Width: 1100, Height: 740}, Chrome: gui.WindowChromeIntegrated})
	if err != nil {
		return err
	}
	defer w.Destroy()
	closed := false
	w.ConnectDestroy(func() { closed = true; app.Quit() })
	if err := w.SetTitle("GOUI input trace"); err != nil {
		return err
	}
	w.SetMinSize(geometry.Size{Width: 800, Height: 520})
	out.record("ENV", "platform=%q nativeWindow=%#x painter.requested=%q", app.Platform().Name(), w.PlatformWindow().NativeHandle(), os.Getenv("GOUI_PLAT_PAINTER"))

	model := widgets.NewTreeStore[string]()
	model.Append("", "project", "Project")
	model.Append("project", "src", "src")
	model.Append("src", "main", "main.go")
	model.Append("src", "view", "view.go")
	model.Append("project", "assets", "assets")
	model.Append("assets", "logo", "logo.svg")
	tree := widgets.NewTreeView()
	tree.SetID("tree")
	tree.SetModel(model)
	tree.SetDelegate(&treeDelegate{tree: tree, model: model})
	treeScroll := gui.NewScrollView()
	treeScroll.SetChild(tree)
	treeScroll.SetMinSize(geometry.Size{Width: 250})
	treeScroll.SetMaxSize(geometry.Size{Width: 250})
	treeScroll.SetMainWeight(0)
	table := widgets.NewTableView()
	table.SetID("table")
	table.SetMainWeight(1)
	rows := make([]string, 150)
	for i := range rows {
		rows[i] = fmt.Sprintf("row-%03d", i+1)
	}
	table.SetModel(&rowModel{SliceListModel: gui.NewSliceListModel(rows)})
	var columns []*widgets.TableColumn
	for i, title := range []string{"Name", "Kind", "Size", "Modified", "Notes"} {
		column := widgets.NewTableColumn(title, title)
		column.SetWidth(220)
		column.SetDelegate(&cellDelegate{column: i})
		columns = append(columns, column)
	}
	table.SetColumns(columns...)
	state := func() {
		out.record("STATE", "focus=%s tree.current=%q tree.selection=%q project.expanded=%t src.expanded=%t", widgetName(w.FocusedWidget()), tree.Current(), tree.Selection(), tree.Expanded("project"), tree.Expanded("src"))
		for _, pair := range []struct {
			name   string
			widget gui.Widget
		}{{"tree", treeScroll}, {"table", table}} {
			if scroll := findScroll(pair.widget); scroll != nil {
				s := scroll.Snapshot()
				out.record("STATE", "%s.scroll offset=%g,%g max=%g,%g bounds=%v", pair.name, s.ScrollX, s.ScrollY, s.MaxScrollX, s.MaxScrollY, s.Bounds)
			}
		}
	}
	reset := func() {
		tree.SetExpandedIDs([]string{"project", "src"})
		tree.SetCurrent("project")
		tree.SetSelection([]string{"project"})
		for _, widget := range []gui.Widget{treeScroll, table} {
			if scroll := findScroll(widget); scroll != nil {
				scroll.SetScrollX(0)
				scroll.SetScrollY(0)
			}
		}
		w.SetFocusedWidget(tree)
		out.record("MARK", "reset")
	}
	root := gui.NewLinearBox(layout.DirectionVertical)
	root.SetCrossAlign(layout.CrossStretch)
	root.SetSpacing(10)
	root.SetPadding(geometry.UniformInsets(12))
	header := gui.NewHeaderBar()
	header.SetChild(gui.NewLabel("原生消息与 GOUI 输入对照"))
	root.AddChild(header)
	pathLabel := gui.NewLabel("日志：" + out.file.Name())
	pathLabel.SetSelectable(true)
	pathLabel.SetWrapMode(gui.WrapWordChar)
	root.AddChild(pathLabel)
	status := gui.NewLabel("正在安装原生观察器，请稍候……")
	root.AddChild(status)
	hint := gui.NewLabel("先点击左侧 Project，再将指针移到右侧表格，分别双指上下／左右滚动。F6 标记 · F7 状态 · F8 重置。")
	hint.SetWrapMode(gui.WrapWordChar)
	root.AddChild(hint)
	bar := gui.NewLinearBox(layout.DirectionHorizontal)
	bar.SetSpacing(8)
	mark := func(label string) { out.record("MARK", "%q", label); state(); out.flush() }
	for _, title := range []string{"触摸板", "鼠标", "远程", "键盘"} {
		button := gui.NewButton()
		button.SetChild(gui.NewLabel(title))
		button.ConnectClicked(func() { mark(title) })
		bar.AddChild(button)
	}
	root.AddChild(bar)
	row := gui.NewLinearBox(layout.DirectionHorizontal)
	row.SetCrossAlign(layout.CrossStretch)
	row.SetSpacing(12)
	row.SetMainWeight(1)
	row.AddChild(treeScroll)
	row.AddChild(table)
	root.AddChild(row)
	// 合并派发后的状态采样；不在 capture 阶段读取尚未应用的滚动结果。
	pending := false
	afterInput := func() {
		if pending {
			return
		}
		pending = true
		app.Post(func() {
			pending = false
			if !closed {
				state()
				out.flush()
			}
		})
	}
	root.AddEventController(&inputObserver{window: w, log: out, afterInput: afterInput})
	tree.ConnectCurrent(func(id string) { out.record("TREE", "current=%q", id) })
	tree.ConnectSelection(func(ids []string) { out.record("TREE", "selection=%q", ids) })
	tree.ConnectExpanded(func(id string, expanded bool) { out.record("TREE", "expanded id=%q value=%t", id, expanded) })
	table.ConnectSelection(func(ids []string) { out.record("TABLE", "selection=%q", ids) })
	w.ConnectFocus(func(focused bool) { out.record("FOCUS", "window=%t widget=%s", focused, widgetName(w.FocusedWidget())) })
	w.SetWidget(root)
	for _, item := range []struct {
		key gui.Key
		fn  func()
	}{
		{gui.KeyF6, func() { mark("F6") }},
		{gui.KeyF7, func() { state(); out.flush() }},
		{gui.KeyF8, reset},
	} {
		shortcut := gui.NewShortcut(gui.KeyGesture{Key: item.key})
		shortcut.ConnectActivate(item.fn)
		w.Shortcuts().AddShortcut(shortcut)
	}
	stop, err := startNativeTrace(w, out, func(readyErr error) {
		app.Post(func() {
			if readyErr != nil {
				err = errors.Join(err, readyErr)
				out.record("ERROR", "install=%q", readyErr)
				app.Quit()
				return
			}
			if closed {
				return
			}
			status.SetText("记录就绪 — 原生观察不消费事件；完成后关闭窗口并发送日志。")
			w.SetFocusedWidget(tree)
			mark("ready")
		})
	})
	if err != nil {
		return err
	}
	defer func() { err = errors.Join(err, stop()) }()
	firstFrame := w.ConnectAfterPaint(func(p gui.Painter, bounds geometry.Rectangle) {
		if bounds.Width <= 0 || bounds.Height <= 0 || out.scale == p.PixelScale() {
			return
		}
		out.scale = p.PixelScale()
		out.record("ENV", "pixelScale=%g bounds=%v", out.scale, bounds)
	})
	defer firstFrame.Disconnect()
	flush := app.NewTimer()
	flush.ConnectTimeout(func() { out.flush() })
	if err := flush.Start(250 * time.Millisecond); err != nil {
		return err
	}
	defer flush.Stop()
	reset()
	if err := w.Show(); err != nil {
		return err
	}
	app.Run()
	out.record("END", "event loop returned")
	return err
}

type rowModel struct{ *gui.SliceListModel[string] }

func (m *rowModel) RowID(index int) string { return m.ItemAt(index) }

type cellDelegate struct{ column int }

func (*cellDelegate) Setup() gui.Widget { return gui.NewLabel("") }
func (d *cellDelegate) Bind(row widgets.TableRow, w gui.Widget) {
	w.(*gui.Label).SetText(fmt.Sprintf("Row %03d · Column %d", row.Index+1, d.column+1))
}
func (*cellDelegate) Unbind(widgets.TableRow, gui.Widget) {}

type treeDelegate struct {
	tree  *widgets.TreeView
	model *widgets.TreeStore[string]
}
type treeContent struct {
	*widgets.TreeExpander
	id    string
	label *gui.Label
}

func (d *treeDelegate) Setup() gui.Widget {
	c := &treeContent{TreeExpander: widgets.NewTreeExpander(), label: gui.NewLabel("")}
	c.label.SetStyleName("tree-item-text")
	c.SetChild(c.label)
	c.ConnectToggle(func() {
		if c.id != "" {
			d.tree.SetExpanded(c.id, !d.tree.Expanded(c.id))
		}
	})
	return c
}
func (d *treeDelegate) Bind(row widgets.TreeRow, w gui.Widget) {
	c := w.(*treeContent)
	c.id = row.ID
	c.label.SetText(d.model.Item(row.ID))
	c.SetDepth(row.Depth)
	c.SetIndentation(d.tree.Indentation())
	c.SetExpandable(row.Expandable)
	c.SetExpanded(row.Expanded)
}
func (*treeDelegate) Unbind(_ widgets.TreeRow, w gui.Widget) { w.(*treeContent).id = "" }

// 仅观察，不消费输入、不改变焦点。
type inputObserver struct {
	gui.EventControllerBase
	window     gui.Window
	log        *traceLog
	afterInput func()
}

func (o *inputObserver) HandleEvent(ctx gui.EventContext) {
	focus := widgetName(o.window.FocusedWidget())
	switch e := ctx.Event().(type) {
	case events.WheelEvent:
		target := widgetName(gui.Pick(o.window.Widget(), e.Position))
		o.log.record("GOUI", "lastNative=%d wheel pos=%v delta=%g,%g mode=%d modifiers=%#x focus=%s target=%s", o.log.lastNative, e.Position, e.DeltaX, e.DeltaY, e.Mode, e.Modifiers, focus, target)
	case events.KeyEvent:
		o.log.record("GOUI", "lastNative=%d key type=%d key=%v code=%d location=%d repeat=%t modifiers=%#x focus=%s", o.log.lastNative, e.EventType, e.Key, e.Code, e.Location, e.Repeat, e.Modifiers, focus)
	case events.PointerEvent:
		o.log.record("GOUI", "lastNative=%d pointer type=%d pos=%v button=%d buttons=%#x modifiers=%#x focus=%s", o.log.lastNative, e.EventType, e.Position, e.Button, e.Buttons, e.Modifiers, focus)
	default:
		return
	}
	o.afterInput()
}
func widgetName(w gui.Widget) string {
	for ; w != nil; w = w.Parent() {
		if w.ID() != "" {
			return w.ID()
		}
	}
	return "none"
}
func findScroll(w gui.Widget) *gui.ScrollView {
	if s, ok := w.(*gui.ScrollView); ok {
		return s
	}
	for _, child := range w.Children() {
		if s := findScroll(child); s != nil {
			return s
		}
	}
	return nil
}
