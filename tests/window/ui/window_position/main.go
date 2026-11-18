// 声明式窗口查找、定位和身份生命周期验证。
// 环境：Windows/macOS 或 Linux/X11 + EWMH WM；Software/OpenGL，1x/2x，原生装饰。
// 启动：go run ./tests/window/ui/window_position
// 初始：main、target 两个独立窗口，提供相同按钮/快捷键。
// 操作与预期（每次定位后等待原生移动完成，再单独执行查询）：
//  1. F1 将 target 客户区移到 main 客户区 (200,100) DIP；F2 查询相对 main/工作区/自身
//     位置，误差最多 1 DIP（覆盖原生整数坐标取整）；自身位置必须为零，工作区原点可为负。
//  2. 拖动 main 后重复 F1/F2，应使用新原点；只移动 main 不带动 target；F1 不激活/缩放 target。
//  3. F6 后 F2，Position(nil) 应为 (120,120)，重复无累积偏移；WM 调整/拒绝不能计作精确定位通过。
//  4. 手动移动 target，F3 保存位置后触发声明重建，核对位置和对象身份均不变；检查期间不要再拖动。
//  5. F4 移除 target 后 FindWindow 返回 nil；F1/F2 只报告缺失，不能把缺失引用当工作区原点。
//     F5 用同 ID 重建，新对象必须不同；重复 F1/F2 验证新对象。
//  6. 原生关闭 target 时 OnDestroy 应查不到该窗口，并移除声明；F5 可再次创建，main 仍可用。
//  7. F8/关闭 main 退出；错误日志为 FAIL 且非零退出，不能把启动当验收。
//
// 检查由真实按钮/快捷键触发，使用既有 Post 队列；不验显示前定位、多显示器持久化或混合 DPI。
// 副作用：只创建/移动/关闭本例窗口，不改系统设置和剪贴板。复位：重启。退出：F8/Quit/关闭 main。
package main

import (
	"fmt"
	"log"
	"math"
	"os"

	"github.com/golang-gui/goui/core/geometry"
	"github.com/golang-gui/goui/gui"
	"github.com/golang-gui/goui/ui"
)

func main() {
	visible, failures, rebuilds := true, 0, 0
	var previous gui.Window
	var expected *geometry.Point
	referenceID := "main"
	check := func(name string, err error) {
		if err != nil {
			failures++
			log.Printf("FAIL %s: %v", name, err)
		} else {
			log.Printf("PASS %s", name)
		}
	}
	near := func(got, want geometry.Point) error {
		// 1x/2x 下最多 1 DIP 容差只覆盖原生整数坐标取整。
		if math.Abs(float64(got.X-want.X)) > 1 || math.Abs(float64(got.Y-want.Y)) > 1 {
			return fmt.Errorf("position=%v, want=%v (tolerance 1 DIP)", got, want)
		}
		return nil
	}
	err := ui.Run("org.golang-gui.UIWindowPosition", func(app ui.App) ui.RootView {
		rebuilds++
		withTarget := func(fn func(gui.Window, gui.Window)) func() {
			return func() {
				main, target := app.FindWindow("main"), app.FindWindow("target")
				if main == nil || target == nil {
					log.Print("target absent (or main unavailable); no positioning request")
					return
				}
				fn(main, target)
			}
		}
		actions := []struct {
			key   ui.Key
			label string
			run   func()
		}{
			{ui.KeyF1, "F1 Target relative to main", withTarget(func(main, target gui.Window) {
				p := geometry.Point{X: 200, Y: 100}
				err := target.SetPosition(main, p)
				check("relative request", err)
				expected, referenceID = nil, "main"
				if err == nil {
					expected = &p
				}
			})},
			{ui.KeyF2, "F2 Query / check position", withTarget(func(main, target gui.Window) {
				for _, ref := range []struct {
					name   string
					window gui.Window
				}{{"main", main}, {"work-area", nil}, {"self", target}} {
					p, err := target.Position(ref.window)
					log.Printf("Position(%s)=%v err=%v", ref.name, p, err)
					if err == nil && expected != nil && ref.name == referenceID {
						err = near(p, *expected)
					}
					check("query "+ref.name, err)
				}
				area, err := target.WorkAreaAt(geometry.Point{})
				log.Printf("WorkAreaAt(0)=%v err=%v", area, err)
				check("work area", err)
			})},
			{ui.KeyF3, "F3 Rebuild / preserve position", withTarget(func(_ gui.Window, target gui.Window) {
				expected = nil // 手动移动后原有定位预期失效。
				before, err := target.Position(nil)
				if err != nil {
					check("before rebuild", err)
					return
				}
				app.RequestUpdate()
				app.Post(func() {
					current := app.FindWindow("target")
					if current != target {
						check("rebuild identity", fmt.Errorf("window replaced or missing"))
						return
					}
					after, err := current.Position(nil)
					if err == nil {
						err = near(after, before)
					}
					check("rebuild identity and position", err)
				})
			})},
			{ui.KeyF4, "F4 Remove target", func() {
				previous = app.FindWindow("target")
				visible, expected = false, nil
				app.RequestUpdate()
				app.Post(func() {
					if app.FindWindow("target") != nil {
						check("removed lookup", fmt.Errorf("still present"))
						return
					}
					check("removed lookup", nil)
				})
			}},
			{ui.KeyF5, "F5 Recreate target", func() {
				if app.FindWindow("target") != nil {
					log.Print("target already present")
					return
				}
				visible = true
				app.RequestUpdate()
				app.Post(func() {
					current := app.FindWindow("target")
					if current == nil || current == previous {
						check("recreated identity", fmt.Errorf("missing or reused object"))
						return
					}
					check("recreated identity", nil)
				})
			}},
			{ui.KeyF6, "F6 Target work-area (120,120)", withTarget(func(_ gui.Window, target gui.Window) {
				p := geometry.Point{X: 120, Y: 120}
				err := target.SetPosition(nil, p)
				check("work-area request", err)
				expected, referenceID = nil, "work-area"
				if err == nil {
					expected = &p
				}
			})},
			{ui.KeyF8, "F8 Quit", app.Quit},
		}
		window := func(id string) ui.WindowView {
			children := []ui.View{ui.Label(fmt.Sprintf("%s / rebuild %d", id, rebuilds))}
			var shortcuts []*ui.ShortcutView
			for _, action := range actions {
				children = append(children, ui.Button(action.label).OnClick(action.run))
				shortcuts = append(shortcuts, ui.Shortcut(action.key).OnActivate(action.run))
			}
			return ui.Window(id).Title("GOUI UI position "+id).Size(400, 400).
				Chrome(ui.WindowChromeNative).Shortcuts(shortcuts...).Content(ui.VBox(children...).Spacing(6))
		}
		windows := []ui.WindowView{window("main").OnDestroy(app.Quit)}
		if visible {
			windows = append(windows, window("target").OnDestroy(func() {
				visible, expected = false, nil
				if app.FindWindow("target") != nil {
					check("destroy lookup", fmt.Errorf("still present"))
				} else {
					check("destroy lookup", nil)
				}
			}))
		}
		return ui.Root().Windows(windows...)
	})
	if err != nil {
		log.Print(err)
		failures++
	}
	if failures != 0 {
		os.Exit(1)
	}
}
