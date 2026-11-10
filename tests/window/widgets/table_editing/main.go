// 声明式表格末行输入后追加空行，保留编辑器身份与输入状态。
//
// 环境：Linux/X11、Windows或macOS可交互桌面，记录OS、后端、缩放和Linux WM。
// 启动：go run ./tests/window/widgets/table_editing [-auto]。
// GOUI_PLAT_PAINTER=software/opengl，Windows另可direct2d；GOUI_PLAT_SCALE=2请求2x。
// 独立X11若没有外部输入法服务，可用XMODIFIERS=@im=none验证普通键盘输入；
// 原生中文IME仍须另外启动并验证相应服务。
// 初始：Modern、Native 640×400窗口，表格只有一条空记录；首次绘制后聚焦首格。
//
// 操作与逐项预期：
//  1. 连续输入abc：首个字符新增一条空记录，之后字符仍进入原格，不必重新点击，
//     总数保持2；F6输出CHECK，原编辑器身份和焦点保持，失焦数为0。
//  2. 移动光标、选区、Primary+Z撤销，再Primary+Shift+Z重做；原编辑历史有效。
//     F4或Rebuild全局重建，原输入框、文字、选区和焦点保持。
//  3. 在中文IME预编辑时点击Append或Insert before（不抢指针焦点），候选和预编辑
//     保持，提交仍在原格；测试中的CommitText自动输入不能替代此原生IME验收。
//  4. Delete first删除首条记录：若编辑的是该行，焦点正常交给表格；其他保留行不能
//     被卸载。反复Append，再滚动到远处允许正常回收；不承诺离屏编辑器持续存在。
//  5. 点新的末行继续输入，再次只追加一条空行；输入和快照跟随稳定业务RowID。
//
// -auto在实际窗口中通过CommitText、Window.DispatchEvent和原有UI协调验证连续输入、
// 全局重建、选区、撤销/重做、插入/删除和再次输入；不使用DevServer，不回调业务捷径。
// 输出PASS后退出；提前关闭、超时或断言失败不算通过。单元格ID只用于观察和查找，
// 框架保留编辑器依靠行变更范围，无需应用为每格设置Widget ID。
// 平台差异：Primary在macOS为Command，其余为Control；字体与原生IME依系统。
// 副作用：仅内存记录、窗口和日志，无文件/网络/剪贴板或系统配置改动。
// 复位：重启。退出：Exit或关闭窗口；应用退出清理窗口和计时器。
package main

import (
	"flag"
	"fmt"
	"image"
	"log"
	"runtime"
	"time"

	"github.com/golang-gui/goui/core/geometry"
	"github.com/golang-gui/goui/gui"
	"github.com/golang-gui/goui/layout"
	"github.com/golang-gui/goui/platform/events"
	"github.com/golang-gui/goui/theme/modern"
	"github.com/golang-gui/goui/ui"
	"github.com/golang-gui/goui/widgets"
	wui "github.com/golang-gui/goui/widgets/ui"
)

type record struct{ id, text string }
type model struct {
	*gui.SliceListModel[record]
	next int
}

func (m *model) RowID(index int) string { return m.ItemAt(index).id }
func (m *model) blank() record {
	m.next++
	return record{id: fmt.Sprintf("row-%d", m.next)}
}
func (m *model) edit(id, text string) {
	m.Modify(func(rows []record) []record {
		for i := range rows {
			if rows[i].id == id {
				rows[i].text = text
				if i == len(rows)-1 && text != "" {
					rows = append(rows, m.blank())
				}
				break
			}
		}
		return rows
	})
}

type scenario struct {
	app                ui.App
	model              *model
	status             ui.State[string]
	input              *gui.TextInput
	timer              *ui.Timer
	deadline           time.Time
	auto, done         bool
	builds, step, lost int
}

func require(ok bool, message string) {
	if !ok {
		panic(message)
	}
}
func must(err error) {
	if err != nil {
		panic(err)
	}
}
func (s *scenario) win() gui.Window { return s.app.FindWindow("main") }
func (s *scenario) check() {
	require(s.app.FindWidget("main", "edit-row-1") == s.input && s.win().FocusedWidget() == s.input && s.lost == 0, "原编辑器身份或焦点丢失")
	log.Printf("CHECK rows=%d text=%q selection=%+v lost=%d builds=%d", s.model.ItemsCount(), s.input.Text(), s.input.Selection(), s.lost, s.builds)
}
func (s *scenario) key(key events.Key, modifiers events.Modifiers) {
	must(s.win().DispatchEvent(events.KeyEvent{EventType: events.KeyDown, Key: key, Modifiers: modifiers}))
}
func (s *scenario) tick() {
	require(time.Now().Before(s.deadline), fmt.Sprintf("表格编辑验收超时：step=%d", s.step))
	if s.step == 0 {
		widget := s.app.FindWidget("main", "edit-row-1")
		if widget == nil {
			return
		}
		s.input = widget.(*gui.TextInput)
		if s.input.Rect().Width <= 0 {
			return
		}
		require(s.win().SetFocusedWidget(s.input), "首格聚焦失败")
		s.input.ConnectFocused(func(focused bool) {
			if !focused {
				s.lost++
			}
		})
		if !s.auto {
			s.timer.Stop()
			log.Printf("READY table editing; F6 check, F4 rebuild; input=%+v", s.input.Snapshot().Bounds)
			return
		}
	}
	switch s.step {
	case 0:
		must(s.win().CommitText("a"))
		s.check()
		require(s.model.ItemsCount() == 2, "输入未追加唯一空行")
	case 1:
		must(s.win().CommitText("bc"))
		s.check()
		require(s.input.Text() == "abc" && s.model.ItemsCount() == 2, "继续输入错误或重复追加")
		s.input.SetSelection(gui.TextSelection{Anchor: 1, Caret: 2})
		s.app.RequestUpdate()
	case 2:
		s.check()
		require(s.builds >= 2 && s.input.Selection() == (gui.TextSelection{Anchor: 1, Caret: 2}), "重建丢失原选区")
		must(s.win().CommitText("X"))
		require(s.input.Text() == "aXc", "原光标/选区没有继续接收输入")
		primary := events.ModifierControl
		if runtime.GOOS == "darwin" {
			primary = events.ModifierCommand
		}
		s.key(events.KeyZ, primary)
		require(s.input.Text() == "abc", "追加或重建丢失撤销历史")
		s.key(events.KeyZ, primary|events.ModifierShift)
		require(s.input.Text() == "aXc", "重做失效")
		s.model.Insert(0, s.model.blank())
	case 3:
		s.check()
		require(s.model.RowID(1) == "row-1", "前置插入的业务索引错误")
		s.model.Remove(0)
	case 4:
		s.check()
		s.input.SetSelection(gui.TextSelection{Anchor: 3, Caret: 3})
		must(s.win().CommitText("中文"))
		require(s.input.Text() == "aXc中文", "多字节输入未进入原格")
		s.model.Remove(0)
		require(s.win().FocusedWidget() == s.app.FindWidget("main", "table") && s.input.Root() == nil, "删除编辑行没有正常移交焦点")
	case 5:
		require(s.app.FindWidget("main", "edit-row-1") == nil, "已删除格仍被声明层引用")
		next := s.app.FindWidget("main", "edit-row-2")
		require(next != nil && s.win().SetFocusedWidget(next), "未能编辑剩余空行")
		must(s.win().CommitText("next"))
		require(s.model.ItemsCount() == 2 && s.win().FocusedWidget() == next, "新的末行不能继续自动追加")
		s.done = true
		s.timer.Stop()
		fmt.Println("PASS table_editing identity/focus/selection/history/insert/remove/rebuild")
		s.app.Post(s.app.Quit)
	}
	s.step++
}
func (s *scenario) view() ui.View {
	button := func(title string, fn func()) ui.View { return ui.Button(title).FocusOnPointer(false).OnClick(fn) }
	return ui.VBox(
		ui.Label("Type in the last row: append a blank row and keep editing."),
		ui.HWrap(button("Append", func() { s.model.Append(s.model.blank()) }), button("Insert before", func() { s.model.Insert(0, s.model.blank()) }),
			button("Delete first", func() {
				if s.model.ItemsCount() > 0 {
					s.model.Remove(0)
				}
			}), button("Rebuild", s.app.RequestUpdate), button("Exit", s.app.Quit)).Spacing(8),
		wui.TableView(s.model,
			wui.TableColumn("id", "ID", func(_ widgets.TableRow, r record) ui.View { return ui.HBox(ui.Label(r.id)).Padding(6) }).Width(100),
			wui.TableColumn("text", "Text", func(_ widgets.TableRow, r record) ui.View {
				return ui.TextInput().ID("edit-" + r.id).Text(r.text).OnText(func(text string) { s.model.edit(r.id, text) })
			}).Expand(true),
		).ID("table").MainWeight(1),
		ui.Label().BindText(&s.status),
	).Padding(16).Spacing(12).CrossAlign(layout.CrossStretch)
}

func main() {
	auto := flag.Bool("auto", false, "自动验收后退出")
	flag.Parse()
	m := &model{SliceListModel: gui.NewSliceListModel([]record(nil))}
	m.Append(m.blank())
	s := &scenario{model: m, auto: *auto, status: ui.MakeState("One blank row"), deadline: time.Now().Add(30 * time.Second)}
	m.ConnectItems(func() { s.status.Set(fmt.Sprintf("Rows: %d", m.ItemsCount())) })
	sheet := modern.Sheet(modern.Options{})
	err := ui.Run("org.golanggui.TableEditing", func(app ui.App) ui.RootView {
		s.app, s.builds = app, s.builds+1
		return ui.Root().StyleSheet(sheet).Windows(ui.Window("main").Title("GOUI table editing").Size(640, 400).Center(true).
			Shortcuts(ui.Shortcut(ui.KeyF4).OnActivate(app.RequestUpdate), ui.Shortcut(ui.KeyF6).OnActivate(s.check)).
			OnAfterPaint(func(p gui.Painter, _ geometry.Rectangle) {
				if s.timer == nil {
					img, err := p.NewImage(image.NewRGBA(image.Rect(0, 0, 1, 1)))
					must(err)
					log.Printf("BACKEND %T scale=%g", img, p.PixelScale())
					img.Destroy()
					s.timer = app.TickFunc(200*time.Millisecond, s.tick)
				}
			}).Content(s.view()))
	})
	if err != nil {
		log.Fatal(err)
	}
	if *auto && !s.done {
		log.Fatal("自动验收未完成")
	}
}
