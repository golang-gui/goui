// 原生窗口菜单栏验证，需要支持 WindowMenuBar 的桌面；当前为 macOS。
// 启动：go run ./tests/window/platform/menu_bar；裸程序和 .app 分别记录。
// 初始：A/B 两个窗口，A 在前台；Native 菜单有 Clone、禁用项、Next window、Finish。
// 操作：在 A 点击 Clone 并按 Command+1，各执行一次；标题和勾选变化后 ID 不变。
// 等价键报告普通 KeyEvent，由本程序处理；不应同时报告 MenuEvent。
// 选择 Next window 转到 B，用 Command+1 执行一次，再 Next window 回到 A。
// 两个窗口故意使用相同的条目 ID，事件必须回到各自窗口；A=2/B=1 后 Finish 输出 PASS。
// 也可用 Quit／Command+Q 完成；原生循环返回后检查计数并继续执行清理。
// -no-menu 不创建窗口和业务菜单：一次 Quit 后原生循环须正常返回并输出 PASS。
// 展开菜单时任务仍应执行；窗口关闭、提前退出、重复动作、超时均失败。
// 副作用：仅本进程的原生窗口、菜单和 stdout；三分钟超时。重启复位。
// 不支持的宿主明确失败，不算原生验收通过。不改业务文件、剪贴板或系统配置。
package main

import (
	"flag"
	"fmt"
	"time"

	"github.com/golang-gui/goui/core/geometry"
	"github.com/golang-gui/goui/platform"
	"github.com/golang-gui/goui/platform/events"
)

func must(err error) {
	if err != nil {
		panic(err)
	}
}
func main() {
	noMenu := flag.Bool("no-menu", false, "不创建窗口，验证原生退出后正常返回")
	flag.Parse()
	p, err := platform.NewPlatform(platform.DefaultName(), "org.golang-gui.NativeMenuVerification")
	must(err)
	defer p.Destroy()
	counts := [2]int{}
	loop, err := p.NewEventLoop()
	must(err)
	defer loop.Destroy()
	var windows [2]platform.Window
	var menus [2]platform.WindowMenuBar
	items := func(i int) []platform.MenuItem {
		title := "Clone"
		if counts[i] > 0 {
			title = "Clone completed"
		}
		return []platform.MenuItem{{ID: 1, Title: "Native", Enabled: true, Submenu: true, Children: []platform.MenuItem{
			{ID: 2, Title: title, Enabled: true, Checked: counts[i] > 0, Shortcut: platform.MenuShortcut{Key: events.Key1, Modifiers: events.ModifierCommand}},
			{ID: 3, Title: "Disabled", Enabled: false},
			{ID: 5, Title: "Next window", Enabled: true},
			{ID: 4, Title: "Finish", Enabled: true},
		}}}
	}
	if !*noMenu {
		for i := range windows {
			windows[i], err = p.NewWindow(geometry.Size{Width: 500, Height: 260}, func(event events.Event) {
				switch e := event.(type) {
				case events.KeyEvent:
					if e.EventType == events.KeyDown && e.Key == events.Key1 && e.Modifiers == events.ModifierCommand {
						e.PreventDefault()
						if !e.Repeat {
							counts[i]++
							fmt.Printf("KEY window=%d count=%d\n", i, counts[i])
							must(menus[i].SetMenu(items(i)))
						}
					}
				case events.MenuEvent:
					fmt.Printf("SELECT window=%d item=%d\n", i, e.Item)
					switch e.Item {
					case 2:
						counts[i]++
						if counts[i] > 2 {
							panic("duplicate activation")
						}
						must(menus[i].SetMenu(items(i)))
					case 5:
						must(windows[1-i].(platform.WindowManagement).Show())
					case 4:
						loop.Quit()
					default:
						panic("disabled or unknown action")
					}
				case events.CloseEvent:
					panic("verification interrupted")
				}
			}, platform.WindowOptions{Chrome: platform.WindowChromeNative})
			must(err)
			defer windows[i].Destroy()
			var ok bool
			menus[i], ok = windows[i].(platform.WindowMenuBar)
			if !ok {
				panic("native window menu bar unsupported")
			}
			must(menus[i].SetMenu(items(i)))
			// 错误描述不能替换已登记菜单；临时切片的修改不得污染平台副本。
			invalid := items(i)
			invalid[0].Children[0].ID = 1
			if menus[i].SetMenu(invalid) == nil {
				panic("duplicate ID accepted")
			}
			must(windows[i].(platform.WindowManagement).SetTitle(fmt.Sprintf("Native menu %c", 'A'+i)))
		}
		must(windows[1].(platform.WindowManagement).Show())
		must(windows[0].(platform.WindowManagement).Show())
	}
	timer := time.AfterFunc(3*time.Minute, func() { loop.Post(func() { panic("native verification timed out") }) })
	defer timer.Stop()
	fmt.Println("READY native menu")
	loop.Run()
	if !*noMenu && counts != [2]int{2, 1} {
		panic(fmt.Sprintf("wrong counts: %v", counts))
	}
	fmt.Println("PASS native menu", counts)
}
