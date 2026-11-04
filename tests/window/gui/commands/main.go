// Command、菜单实时状态与快捷键列的窗口验证。
//
// 环境：原生桌面、字体及事件循环；Linux 使用独立 X11／窗口管理器。
// 启动：go run ./tests/window/gui/commands；-auto 自动派发正常输入并检查后退出。
// 可加 -basic、-dark；GOUI_PLAT_PAINTER、GOUI_PLAT_SCALE 选择后端和倍率。
// 初始：保存操作、菜单入口和 Ctrl/Cmd+S 均启用，保存次数为 0。
// 操作与预期：
//  1. 打开“文件”。标题统一左对齐，快捷键统一右对齐，长短提示不挤压标题。
//  2. 保持菜单打开按 F6，保存立即灰显并失去选中／按下状态；不能点击或按 Enter 执行。
//     再按 F6 恢复，但不自动执行；按 F7 在打开期间修改提示，列边界仍一致。
//  3. 按 Esc 关闭，用提示所示按键保存，次数恰好增加 1；旧按键不再响应。
//  4. 点击“切换快捷键入口”，仅按键禁用，菜单仍可保存且保留提示。
//     点击“切换菜单入口”，只影响菜单，快捷键仍可执行。
//  5. 打开／关闭菜单、切换亮暗和重复上述步骤，观察标题与提示基线、两列间距及灰显。
//
// 自动场景验证打开时禁用／恢复、修改 Gesture、局部禁用、重新打开和语义边界；
// 所有动作经 Window/Popover.DispatchEvent，不使用 DevServer 或直接触发控件信号。
// F8 输出当前状态及真实图片资源类型和倍率；自动派发不替代原生鼠标与键盘验收。
// 副作用：仅创建测试窗口、改变该独立桌面的焦点并输出日志，不读写剪贴板或系统设置。
// 复位：重启；退出：关闭窗口，或自动 PASS 后退出。自动场景提前退出属于失败。
package main

import (
	"flag"
	"fmt"
	"image"
	"os"
	"strings"
	"time"

	"github.com/golang-gui/goui/core/geometry"
	"github.com/golang-gui/goui/gui"
	"github.com/golang-gui/goui/layout"
	"github.com/golang-gui/goui/platform/events"
	"github.com/golang-gui/goui/theme/modern"
)

func must(err error) {
	if err != nil {
		panic(err)
	}
}
func require(ok bool, message string) {
	if !ok {
		panic(message)
	}
}
func button(text string, fn func()) *gui.Button {
	b := gui.NewButton()
	b.SetChild(gui.NewLabel(text))
	b.ConnectClicked(fn)
	return b
}

type inputTarget interface{ DispatchEvent(events.Event) error }

func key(target inputTarget, k events.Key, mods events.Modifiers) {
	must(target.DispatchEvent(events.KeyEvent{EventType: events.KeyDown, Key: k, Modifiers: mods}))
}
func click(target inputTarget, point geometry.Point) {
	for _, kind := range []events.EventType{events.PointerMove, events.PointerDown, events.PointerUp} {
		must(target.DispatchEvent(events.PointerEvent{EventType: kind, Position: point, Button: events.PointerButtonLeft}))
	}
}
func rows(info gui.WidgetInfo) []gui.WidgetInfo {
	result := []gui.WidgetInfo{}
	if info.Role == gui.RoleMenuItem {
		result = append(result, info)
	}
	for _, child := range info.Children {
		result = append(result, rows(child)...)
	}
	return result
}
func hint(info gui.WidgetInfo) string {
	if value, ok := info.Attributes[gui.MenuItemInfoKey].(gui.MenuItemInfo); ok {
		return value.Shortcut
	}
	return ""
}
func right(info gui.WidgetInfo) float32 {
	for _, child := range info.Children {
		if child.Visible && child.Text == hint(info) {
			return child.Bounds.X + child.Bounds.Width
		}
	}
	return 0
}

type probe struct {
	gui.WidgetBase
	backend string
	scale   float32
}

func (p *probe) Measure(c layout.Constraint) layout.Measurement {
	return layout.Measured(c.Clamp(geometry.Size{Width: 1, Height: 1}))
}

func (p *probe) Paint(draw gui.Painter) {
	if p.backend != "" {
		return
	}
	img, err := draw.NewImage(image.NewRGBA(image.Rect(0, 0, 1, 1)))
	must(err)
	p.backend = fmt.Sprintf("%T", img)
	p.scale = draw.PixelScale()
	img.Destroy()
	requested := os.Getenv("GOUI_PLAT_PAINTER")
	if requested != "" {
		require(strings.Contains(strings.ToLower(p.backend), requested), "requested painter fell back")
	}
	fmt.Printf("PAINTER %s scale=%g\n", p.backend, p.scale)
}

func main() {
	auto := flag.Bool("auto", false, "自动检查并退出")
	basic := flag.Bool("basic", false, "Basic 外观")
	dark := flag.Bool("dark", false, "Modern 暗色")
	flag.Parse()
	app, err := gui.NewApplication("org.golanggui.CommandVerification")
	must(err)
	if !*basic {
		app.SetStyleSheet(modern.Sheet(modern.Options{Dark: *dark}))
	}
	win, err := app.NewWindow(&gui.WindowOptions{Size: geometry.Size{Width: 620, Height: 350}, Center: true})
	must(err)
	must(win.SetTitle("GOUI GUI Commands"))
	saves := 0
	status := gui.NewLabel("保存次数：0")
	save := gui.NewCommand()
	save.ConnectActivate(func() { saves++; status.SetText(fmt.Sprintf("保存次数：%d", saves)) })
	saveKey := gui.NewShortcut(gui.KeyGesture{Key: gui.KeyS, Modifiers: gui.ModPrimary})
	saveKey.SetCommand(save)
	win.Shortcuts().AddShortcut(saveKey)
	model := gui.NewMenu()
	for _, entry := range []struct {
		title string
		key   gui.Key
	}{{"新建", gui.KeyN}, {"打开…", gui.KeyO}} {
		c := gui.NewCommand()
		s := gui.NewShortcut(gui.KeyGesture{Key: entry.key, Modifiers: gui.ModPrimary})
		s.SetCommand(c)
		win.Shortcuts().AddShortcut(s)
		model.Append(entry.title, nil).SetCommand(c)
	}
	model.AppendSeparator()
	saveItem := model.Append("保存", nil)
	saveItem.SetCommand(save)
	other := gui.NewCommand()
	otherKey := gui.NewShortcut(gui.KeyGesture{Key: gui.KeyS, Modifiers: gui.ModPrimary | gui.ModShift})
	otherKey.SetCommand(other)
	win.Shortcuts().AddShortcut(otherKey)
	model.Append("另存为…", nil).SetCommand(other)
	model.AppendSeparator()
	model.Append("关闭标签", nil)
	model.Append("关闭全部标签", nil)
	menu := gui.NewMenuButton()
	menu.SetChild(gui.NewLabel("文件"))
	menu.SetMenu(model)
	root := gui.NewLinearBox(layout.DirectionVertical)
	root.SetPadding(geometry.UniformInsets(16))
	root.SetSpacing(12)
	root.AddChild(menu)
	root.AddChild(gui.NewLabel("F6：操作启用　F7：修改按键　F8：检查"))
	root.AddChild(status)
	root.AddChild(button("切换快捷键入口", func() { saveKey.SetEnabled(!saveKey.Enabled()) }))
	root.AddChild(button("切换菜单入口", func() { saveItem.SetEnabled(!saveItem.Enabled()) }))
	p := new(probe)
	root.AddChild(p)
	win.SetWidget(root)
	toggle := func() { save.SetEnabled(!save.Enabled()) }
	change := func() {
		mods := gui.ModPrimary
		if saveKey.Gesture().Modifiers == mods {
			mods |= gui.ModShift
		}
		saveKey.SetGesture(gui.KeyGesture{Key: gui.KeyS, Modifiers: mods})
	}
	for _, entry := range []struct {
		k  gui.Key
		fn func()
	}{{gui.KeyF6, toggle}, {gui.KeyF7, change}, {gui.KeyF8, func() {
		fmt.Printf("CHECK saves=%d command=%t item=%t binding=%t\n", saves, save.Enabled(), saveItem.Enabled(), saveKey.Enabled())
	}}} {
		s := gui.NewShortcut(gui.KeyGesture{Key: entry.k})
		s.ConnectActivate(entry.fn)
		win.Shortcuts().AddShortcut(s)
	}
	must(win.Show())
	complete, step := false, 0
	if *auto {
		timer := app.NewTimer()
		timer.ConnectTimeout(func() {
			pop := func() gui.Popover { require(len(win.Popovers()) == 1, "expected one menu"); return win.Popovers()[0] }
			menuRows := func() []gui.WidgetInfo { return rows(pop().Widget().Snapshot()) }
			savedRow := func() gui.WidgetInfo {
				for _, r := range menuRows() {
					if r.Text == "保存" {
						return r
					}
				}
				panic("missing save row")
			}
			switch step {
			case 0:
				click(win, menu.Snapshot().Bounds.Center())
			case 1:
				r := menuRows()
				require(len(r) == 6, "menu rows missing")
				require(right(r[0]) == right(r[1]) && right(r[1]) == right(r[3]), "shortcut column edges differ")
				key(win, events.KeyF6, 0)
			case 2:
				require(!savedRow().Enabled, "menu did not disable live")
				click(pop(), savedRow().Bounds.Center())
				require(saves == 0 && menu.Opened(), "disabled menu activated")
				key(win, events.KeyF6, 0)
			case 3:
				require(savedRow().Enabled, "menu did not reenable live")
				key(win, events.KeyF7, 0)
			case 4:
				require(hint(savedRow()) == saveKey.Gesture().DisplayText(), "gesture hint stale")
				click(pop(), savedRow().Bounds.Center())
			case 5:
				require(saves == 1 && !menu.Opened(), "menu command did not execute once")
				mods, _ := saveKey.Gesture().Modifiers.Resolve()
				key(win, events.KeyS, mods)
			case 6:
				require(saves == 2, "shortcut command missing")
				saveKey.SetEnabled(false)
				mods, _ := saveKey.Gesture().Modifiers.Resolve()
				key(win, events.KeyS, mods)
				click(win, menu.Snapshot().Bounds.Center())
			case 7:
				require(saves == 2 && savedRow().Enabled && hint(savedRow()) != "", "binding disable leaked into menu")
				click(pop(), savedRow().Bounds.Center())
			case 8:
				require(saves == 3, "menu could not execute with binding disabled")
				saveKey.SetEnabled(true)
				saveItem.SetEnabled(false)
				click(win, menu.Snapshot().Bounds.Center())
			case 9:
				require(!savedRow().Enabled && save.Enabled(), "local menu disable leaked into command")
				key(win, events.KeyEscape, 0)
				mods, _ := saveKey.Gesture().Modifiers.Resolve()
				key(win, events.KeyS, mods)
			case 10:
				require(saves == 4, "menu disable blocked shortcut")
				require(p.backend != "", "no actual paint")
				complete = true
				fmt.Printf("ASSERT PASS GUI command saves=%d backend=%s scale=%g\n", saves, p.backend, p.scale)
				timer.Stop()
				win.Destroy()
			}
			step++
		})
		must(timer.Start(350 * time.Millisecond))
	}
	app.Run()
	if *auto {
		require(complete, "automatic verification exited early")
	}
}
