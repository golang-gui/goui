// 树、标签和表格的临时右键菜单动作回归，需要真实桌面、字体及绘图后端。
// 启动：go run ./tests/window/widgets/context_menu [-auto] [-dark] [-nested]
// 使用 GOUI_PLAT_PAINTER=software/opengl（Windows 另有 direct2d）、GOUI_PLAT_SCALE=1/2。
// 初始：三个控件各有一个 original 项，复制计数为零，Modern 亮色（-dark 为暗色）。
// 操作与预期：
//  1. 在 Tree original 上右键，点击 Clone；菜单关闭，立即新增 Tree copy 1，计数只加一。
//     再次右键原项，Home/Enter 执行；出现 copy 2。Shift+F10 也可请求当前项菜单。
//  2. 在 Tab original 标签及 Table original 单元格重复鼠标、键盘选中后 Enter；
//     每次都新增对应副本且只执行一次，标签副本成为当前页，表格副本成为实际数据行。
//  3. Esc 或点菜单外部只关闭，不新增副本；重开没有上次的选中反馈。
//     -nested 把 Clone 放在 More 子菜单中，Right 进入后同样验证整链关闭及一次执行。
//  4. F6 输出 ASSERT PASS 和三个实际模型数量；复制后等待副本实际出现在布局中再检查。
//
// 自动模式经 Window/Popover.DispatchEvent 完成六种组合，并断言动作在关闭后同步执行，
// 副本已进入控件语义树。按实际可观察状态推进，整体超时 20 秒，提前关闭属于失败。
// 自动模式先将真实指针停在窗口外，不同时手动操作；原生输入另按上述步骤复核。
// 平台差异：右键触发时机沿用平台事件规则；macOS 另可 Control+单击，功能键可能需要 Fn。
// 副作用：只有本窗口、内存模型和日志，不访问业务文件、网络、剪贴板或系统设置。
// 复位：重启。退出：关闭窗口；-auto 完成后自动关闭并返回零。
package main

import (
	"flag"
	"fmt"
	"image"
	"log"
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

type record struct{ id, title string }
type records struct{ *gui.SliceListModel[record] }

func (m *records) RowID(i int) string { return m.ItemAt(i).id }

func require(ok bool, message string) {
	if !ok {
		log.Fatal(message)
	}
}
func must(err error) {
	if err != nil {
		log.Fatal(err)
	}
}
func find(info gui.WidgetInfo, text string) gui.WidgetInfo {
	if info.Text == text {
		return info
	}
	for _, child := range info.Children {
		if result := find(child, text); result.Bounds.Width > 0 {
			return result
		}
	}
	return gui.WidgetInfo{}
}

func main() {
	auto := flag.Bool("auto", false, "自动断言后退出")
	dark := flag.Bool("dark", false, "使用 Modern 暗色")
	nested := flag.Bool("nested", false, "使用级联 Clone 菜单")
	flag.Parse()
	tree := widgets.NewTreeStore[string]()
	tree.Append("", "original", "Tree original")
	table := &records{gui.NewSliceListModel([]record{{"original", "Table original"}})}
	tabKeys, tabCurrent := []string{"original"}, "original"
	tabTitles := map[string]string{"original": "Tab original"}
	counts := [3]int{}
	names, ids := []string{"Tree", "Tab", "Table"}, []string{"tree", "bar", "table"}
	started, ready, completed, observed := false, false, false, false
	painter, scale := "", float32(0)
	stage, phase := 0, 0
	deadline := time.Now().Add(20 * time.Second)
	must(ui.Run("org.golang-gui.ContextMenuActions", func(app ui.App) ui.RootView {
		verify := func() {
			require(len(tree.Children("")) == counts[0]+1 && len(tabKeys) == counts[1]+1 && table.ItemsCount() == counts[2]+1, "动作次数与模型数量不一致")
			fmt.Printf("ASSERT PASS tree=%d tabs=%d table=%d painter=%s scale=%g\n", counts[0], counts[1], counts[2], painter, scale)
		}
		if !started {
			started = true
			app.TickFunc(50*time.Millisecond, func() {
				if *auto {
					require(time.Now().Before(deadline), "自动场景超时")
				}
				win := app.FindWindow("main")
				if win == nil {
					return
				}
				if !observed {
					observed = true
					win.ConnectAfterPaint(func(p gui.Painter, _ geometry.Rectangle) {
						if scale == 0 {
							img, err := p.NewImage(image.NewRGBA(image.Rect(0, 0, 1, 1)))
							must(err)
							painter, scale = fmt.Sprintf("%T", img), p.PixelScale()
							img.Destroy()
						}
					})
					must(win.RequestPaint())
				}
				if !ready {
					if scale == 0 {
						return
					}
					for i, id := range ids {
						widget := app.FindWidget("main", id)
						if widget == nil || find(widget.Snapshot(), names[i]+" original").Bounds.Width == 0 {
							return
						}
					}
					ready = true
					fmt.Printf("READY painter=%s scale=%g nested=%v\n", painter, scale, *nested)
				}
				if !*auto {
					return
				}
				kind, keyboard := stage/2, stage%2 == 1
				key := func(k events.Key) {
					for _, kind := range []events.EventType{events.KeyDown, events.KeyUp} {
						must(win.DispatchEvent(events.KeyEvent{EventType: kind, Key: k}))
					}
				}
				switch phase {
				case 0:
					bounds := find(app.FindWidget("main", ids[kind]).Snapshot(), names[kind]+" original").Bounds
					require(bounds.Width > 0, "原始项目不在可见布局中")
					for _, event := range []events.EventType{events.PointerDown, events.PointerUp} {
						must(win.DispatchEvent(events.PointerEvent{EventType: event, Position: bounds.Center(), Button: events.PointerButtonRight}))
					}
					phase = 1
				case 1:
					popups := win.Popovers()
					if len(popups) == 0 {
						return
					}
					if *nested && len(popups) == 1 {
						key(events.KeyHome)
						key(events.KeyArrowRight)
						return
					}
					popup := popups[len(popups)-1]
					item := find(popup.Widget().Snapshot(), "Clone")
					if item.Bounds.Width == 0 {
						return
					}
					before := counts[kind]
					if keyboard {
						key(events.KeyHome)
						key(events.KeyEnter)
					} else {
						for _, event := range []events.EventType{events.PointerDown, events.PointerUp} {
							must(popup.DispatchEvent(events.PointerEvent{EventType: event, Position: item.Bounds.Center(), Button: events.PointerButtonLeft}))
						}
					}
					require(counts[kind] == before+1 && len(win.Popovers()) == 0, "菜单关闭后动作没有同步执行一次")
					phase = 2
				case 2:
					text := fmt.Sprintf("%s copy %d", names[kind], counts[kind])
					if find(app.FindWidget("main", ids[kind]).Snapshot(), text).Bounds.Width == 0 {
						return
					}
					if kind == 1 {
						current := app.FindWidget("main", "tabs").(*widgets.TabView).Current()
						require(current != nil && current.Title() == text, "标签副本没有成为当前页")
					}
					fmt.Printf("CASE PASS %s keyboard=%v count=%d\n", names[kind], keyboard, counts[kind])
					stage++
					phase = 0
					if stage == 6 {
						verify()
						completed = true
						fmt.Println("AUTO PASS")
						win.Destroy()
					}
				}
			})
		}
		menu := func(kind int, source string) []*ui.MenuItemView {
			if *auto {
				require(source == "original", "右键菜单没有定位到原始项目")
			}
			if source == "" {
				return nil
			}
			clone := ui.MenuItem("Clone", func() {
				require(len(app.FindWindow("main").Popovers()) == 0, "动作执行前菜单仍可见")
				counts[kind]++
				id := fmt.Sprintf("copy-%d", counts[kind])
				title := fmt.Sprintf("%s copy %d", names[kind], counts[kind])
				switch kind {
				case 0:
					tree.Append("", id, title)
				case 1:
					tabKeys = append(tabKeys, id)
					tabTitles[id], tabCurrent = title, id
				case 2:
					table.Append(record{id, title})
				}
				fmt.Printf("ACTION %s source=%s count=%d\n", names[kind], source, counts[kind])
				app.RequestUpdate()
			})
			if *nested {
				return []*ui.MenuItemView{ui.MenuItem("More").Submenu(clone)}
			}
			return []*ui.MenuItemView{clone}
		}
		pages := make([]*wui.TabPageView, 0, len(tabKeys))
		for _, id := range tabKeys {
			pages = append(pages, wui.TabPage(id, ui.Label("Page: "+tabTitles[id])).Title(tabTitles[id]))
		}
		return ui.Root().StyleSheet(modern.Sheet(modern.Options{Dark: *dark})).Windows(
			ui.Window("main").Title("GOUI context menu actions").Size(820, 650).Content(ui.VBox(
				ui.Label("Right-click originals; Clone by mouse or Home / Enter"),
				ui.Label(fmt.Sprintf("Copies: Tree %d / Tab %d / Table %d", counts[0], counts[1], counts[2])),
				ui.ScrollView(wui.TreeView(tree, func(_ widgets.TreeRow, text string) ui.View { return ui.Label(text).Style("tree-item-text") }).ID("tree").ContextMenu(func(id string) []*ui.MenuItemView { return menu(0, id) }).OnContextMenuError(must)).MinHeight(105).MaxHeight(105),
				wui.TabBar("tabs").ID("bar").ContextMenu(func(id string) []*ui.MenuItemView { return menu(1, id) }).OnContextMenuError(must),
				wui.TabView(pages...).ID("tabs").Current(tabCurrent).OnCurrent(func(id string) { tabCurrent = id; app.RequestUpdate() }).MinHeight(70).MaxHeight(70),
				wui.TableView(table, wui.TableColumn("name", "Table", func(_ widgets.TableRow, value record) ui.View { return ui.Label(value.title) }).Expand(true)).ID("table").MinHeight(170).MaxHeight(170).ContextMenu(func(id, _ string) []*ui.MenuItemView { return menu(2, id) }).OnContextMenuError(must),
				ui.Button("Check (F6)").OnClick(verify),
			).Padding(16).Spacing(10).CrossAlign(layout.CrossStretch)).Shortcuts(ui.Shortcut(ui.KeyF6).OnActivate(verify)),
		)
	}))
	require(!*auto || completed, "自动场景提前结束")
}
