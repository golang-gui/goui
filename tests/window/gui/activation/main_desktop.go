//go:build !android && !ios

// GUI/UI 激活生命周期桌面验收，具体环境、操作、预期及复位见 README.md。
package main

import (
	"flag"
	"fmt"
	"time"

	"github.com/golang-gui/goui/gui"
	"github.com/golang-gui/goui/ui"
	"github.com/xuges/gothread"
)

func main() {
	auto := flag.Bool("auto", false, "自动验证启动、状态更新、退出")
	declarative := flag.Bool("ui", false, "验证 ui.Run 的首次激活构建")
	flag.Parse()
	app, err := gui.NewApplication("org.golanggui.Activation")
	if err != nil {
		panic(err)
	}
	thread := gothread.GetId()
	shutdowns := 0
	app.ConnectShutdown(func() {
		if gothread.GetId() != thread {
			panic("Shutdown 线程错误")
		}
		shutdowns++
	})
	if *declarative {
		value := ui.MakeState("ready")
		builds := 0
		err = ui.Run("org.golanggui.Activation", func(a ui.App) ui.RootView {
			builds++
			if gothread.GetId() != thread || app.Typography() == nil {
				panic("UI 构建早于激活")
			}
			if *auto && builds == 1 {
				a.TimeoutFunc(200*time.Millisecond, func() {
					value.Set("updated")
					a.TimeoutFunc(200*time.Millisecond, func() {
						label := a.FindWidget("main", "status").(*gui.Label)
						if label.Text() != "updated" {
							panic("声明式绑定没有更新")
						}
						a.Quit()
					})
				})
			}
			return ui.Window("main").Title("GOUI activation UI").Size(360, 180).Content(
				ui.VBox(ui.Label().BindText(&value).ID("status"), ui.Button("Update").OnClick(func() { value.Set("updated") })).Spacing(12).Padding(20),
			)
		})
	} else {
		activations := 0
		app.ConnectActivate(func() {
			activations++
			if activations != 1 || gothread.GetId() != thread || app.Typography() == nil {
				panic("Activate 线程、次数或服务错误")
			}
			w, e := app.NewWindow(nil)
			if e != nil {
				panic(e)
			}
			label := gui.NewLabel("ready")
			w.SetWidget(label)
			if e = w.Show(); e != nil {
				panic(e)
			}
			if *auto {
				timer := app.NewTimer()
				timer.ConnectTimeout(func() {
					info := w.Snapshot()
					if info.Bounds.Width <= 0 || info.Widget.Text != "ready" {
						panic("首次布局/快照错误")
					}
					label.SetText("updated")
					w.Destroy()
				})
				if e = timer.StartOnce(200 * time.Millisecond); e != nil {
					panic(e)
				}
			}
		})
		err = app.Run()
	}
	if err != nil {
		panic(err)
	}
	if shutdowns != 1 {
		panic("Shutdown 次数错误")
	}
	fmt.Println("ACTIVATION PASS")
}
