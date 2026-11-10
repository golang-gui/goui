// 透明合成能力查询与菜单降级验收，不使用 DevServer。
//
// 环境：桌面 Windows/macOS 或独立 Linux/X11 会话；记录 WM、合成器、后端与倍率。
// 启动：go run ./tests/window/ui/composited [-auto -expect=true|false]。
// 例如 Linux：GOUI_PLAT_PAINTER=opengl go run ./tests/window/ui/composited -auto -expect=false。
// 初始：不透明 Native 窗口，显示 platform、GUI、UI 三层查询结果及查询次数。
//
// 操作与预期：
//  1. 点击 Query 或按 F6：环境稳定时三层结果一致，读取本身不请求重建。
//  2. 在独立 X11 会话分别以 xfwm4 --compositor=on/off 启动，预期 true/false。
//     仅在独立会话中运行期切换合成器；平台立即读取事实，GUI/UI 在既有两秒
//     Settings 检查后更新。即使不点击 Query，快照文本也应自动变化并重建视图；
//     Query 行保留上次显式读取值。没有独立的合成计时器或变化信号。
//  3. 点击 Menu：从启动即合成可用时为圆角/阴影菜单；不可用时为不透明直角菜单，
//     不出现透明黑边。Escape 关闭菜单。已有弹层不承诺随运行期合成器切换重建。
//  4. Windows 8+、受支持的 macOS 环境预期 true；true 不保证任意窗口/后端透明创建成功。
//
// -auto 必须指定 -expect，比较独立预期和三层结果，经正常 PointerDown/Up 打开菜单，
// 检查实际弹层 Transparent 配置并退出；提前关闭或断言错误为非零退出。
// 普通启动不注入事件，只在首帧查询一次。移动端使用系统 Surface，不运行本桌面例程。
// 副作用：仅本程序窗口、菜单和日志，不修改合成器、用户设置、剪贴板或文件。
// 复位：关闭后重启。退出：关闭窗口；自动验收完成后自行退出。
package main

import (
	"flag"
	"fmt"
	"image"
	"log"
	"strconv"
	"time"

	"github.com/golang-gui/goui/core/geometry"
	"github.com/golang-gui/goui/gui"
	"github.com/golang-gui/goui/platform/events"
	"github.com/golang-gui/goui/theme/modern"
	"github.com/golang-gui/goui/ui"
)

func main() {
	auto := flag.Bool("auto", false, "自动断言并退出")
	expect := flag.String("expect", "", "独立预期：true 或 false")
	flag.Parse()
	want, parseErr := strconv.ParseBool(*expect)
	if (*auto || *expect != "") && parseErr != nil {
		log.Fatal("-auto requires -expect=true or false")
	}
	status := ui.MakeState("Waiting for first frame")
	builds, checks := 0, 0
	started, inputStarted, done := false, false, false
	err := ui.Run("org.golanggui.Composited", func(app ui.App) ui.RootView {
		builds++
		log.Printf("SNAPSHOT composited=%t builds=%d", app.Settings().Composited(), builds)
		check := func() {
			native, err := gui.App.Platform().NewSettings()
			if err != nil {
				log.Fatal(err)
			}
			raw, err := native.Composited()
			g, u := gui.App.Settings().Composited(), app.Settings().Composited()
			checks++
			if g != u || *expect != "" && (err != nil || raw != want || u != want) {
				log.Fatalf("composition mismatch: platform=%t/%v gui=%t ui=%t expected=%s", raw, err, g, u, *expect)
			}
			text := fmt.Sprintf("Query %d | platform=%t (%v) | gui=%t | ui=%t | builds=%d", checks, raw, err, g, u, builds)
			log.Print(text)
			status.Set(text)
		}
		return ui.Root().StyleSheet(modern.Sheet(modern.Options{})).Windows(
			ui.Window("main").Title("GOUI Composited Settings").Size(720, 230).
				Shortcuts(ui.Shortcut(ui.KeyF6).OnActivate(check)).
				OnAfterPaint(func(p gui.Painter, _ geometry.Rectangle) {
					if !started {
						started = true
						img, err := p.NewImage(image.NewRGBA(image.Rect(0, 0, 1, 1)))
						if err != nil {
							log.Fatal(err)
						}
						log.Printf("BACKEND %T scale=%g", img, p.PixelScale())
						img.Destroy()
						// 首次查询后在下一绘制帧输入；状态文本放在按钮下方，避免改变命中位置。
						app.Post(func() {
							check()
							if *auto {
								app.TimeoutFunc(15*time.Second, func() { log.Fatal("verification timed out") })
							}
						})
						return
					}
					if !*auto || inputStarted {
						return
					}
					inputStarted = true
					app.Post(func() {
						win := app.FindWindow("main")
						point := app.FindWidget("main", "menu").Snapshot().Bounds.Center()
						for _, kind := range []events.EventType{events.PointerDown, events.PointerUp} {
							if err := win.DispatchEvent(events.PointerEvent{EventType: kind, Button: events.PointerButtonLeft, Position: point}); err != nil {
								log.Fatal(err)
							}
						}
						app.TimeoutFunc(250*time.Millisecond, func() {
							check()
							popups := win.Popovers()
							if len(popups) != 1 || !popups[0].Visible() || popups[0].Transparent() != want {
								var actual []string
								for _, popup := range popups {
									actual = append(actual, fmt.Sprintf("visible=%t transparent=%t", popup.Visible(), popup.Transparent()))
								}
								log.Fatalf("menu configuration mismatch: expected transparent=%t popups=%v opened=%t point=%v", want, actual, app.FindWidget("main", "menu").(*gui.MenuButton).Opened(), point)
							}
							done = true
							log.Printf("PASS live composition query and menu configuration: %t", want)
							app.Quit()
						})
					})
				}).Content(ui.VBox(
				ui.HBox(ui.Button("Query (F6)").OnClick(check), ui.MenuButton("Menu").ID("menu").OnOpenError(func(err error) { log.Fatal(err) }).Menu(ui.MenuItem("Item", func() {}))),
				ui.Label(fmt.Sprintf("Settings snapshot: composited=%t | builds=%d", app.Settings().Composited(), builds)).ID("snapshot"),
				ui.Label().BindText(&status),
				ui.Label("Platform queries immediately; Settings refreshes within its polling cycle."),
			).Spacing(12).Padding(16)),
		)
	})
	if err != nil {
		log.Fatal(err)
	}
	if *auto && !done {
		log.Fatal("verification closed before completion")
	}
}
