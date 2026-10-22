// 声明式动画窗口验收。
// 环境：三平台桌面，记录后端/缩放；启动：go run ./tests/window/ui/animation。
// -auto 自动验证稳定 Widget 身份、最终属性、完成后空闲，输出 UI ANIMATION PASS。
// 初始：Modern Integrated 窗口，一枚 24 DIP 矢量图标和关闭的 Switch。
// 操作：点击 Animate，图标从当前尺寸过渡到 80/24 DIP；反复点击重定向。
//
//	点击 Rebuild，控件保持身份/当前尺寸；切换 Switch，保留原有点击语义。
//
// 预期：UI 状态通过现有异步 RequestUpdate 协调，最终 Icon.Size 与动画终值一致；
//
//	动画结束后不持续重建/绘制。窗口尺寸拖动、亮暗主题不造成动画重启。
//
// 副作用：窗口、终端日志；复位：重启。退出：关闭窗口，释放动画连接。
package main

import (
	"flag"
	"fmt"
	"log"
	"time"

	"github.com/golang-gui/goui/animation"
	"github.com/golang-gui/goui/gui"
	"github.com/golang-gui/goui/icon/draw"
	"github.com/golang-gui/goui/platform/graphics"
	"github.com/golang-gui/goui/theme/modern"
	"github.com/golang-gui/goui/ui"
	wui "github.com/golang-gui/goui/widgets/ui"
)

func main() {
	auto := flag.Bool("auto", false, "自动验证声明式协调后退出")
	flag.Parse()
	size := ui.MakeState(float32(24))
	motion := animation.New(size.Get(), animation.Float32)
	motion.ConnectUpdate(size.Set)
	defer motion.Stop()
	source := &draw.Source{Draw: func(p graphics.Painter, s graphics.Size, c graphics.Color) {
		p.FillEllipse(graphics.Point{X: s.Width / 2, Y: s.Height / 2}, s.Width/2, s.Height/2, c)
	}}
	builds, frames := 0, 0
	setup := false
	var first gui.Widget
	err := ui.Run("org.golang-gui.UIAnimation", func(app ui.App) ui.RootView {
		builds++
		start := func() {
			w := app.FindWindow("main")
			if w == nil {
				return
			}
			target := float32(80)
			if motion.CurrentValue() > 50 {
				target = 24
			}
			if err := motion.AnimateTo(w, target, 400*time.Millisecond, animation.EaseOutCubic); err != nil {
				panic(err)
			}
		}
		if !setup {
			setup = true
			app.TimeoutFunc(150*time.Millisecond, func() {
				w := app.FindWindow("main")
				if w == nil {
					panic("missing window")
				}
				w.ConnectFrame(func(time.Time) { frames++ })
				first = app.FindWidget("main", "animated-icon")
				if *auto {
					start()
				}
			})
			if *auto {
				lastBuilds, lastFrames := 0, 0
				app.TimeoutFunc(900*time.Millisecond, func() {
					icon, ok := app.FindWidget("main", "animated-icon").(*gui.Icon)
					if !ok || icon != first || icon.Size() != 80 || size.Get() != 80 || motion.Running() {
						panic("UI identity/endpoint")
					}
					lastBuilds, lastFrames = builds, frames
				})
				app.TimeoutFunc(1100*time.Millisecond, func() {
					if builds != lastBuilds || frames-lastFrames > 2 {
						panic("idle UI rebuild/paint")
					}
					fmt.Printf("UI ANIMATION PASS builds=%d frames=%d\n", builds, frames)
					app.Quit()
				})
			}
		}
		return ui.Root().Windows(ui.Window("main").Title("GOUI animation UI").Size(620, 340).Chrome(ui.WindowChromeIntegrated).OnDestroy(motion.Stop).Content(
			ui.VBox(ui.HBox(ui.Button("Animate").OnClick(start), ui.Button("Rebuild").OnClick(app.RequestUpdate)).Spacing(10),
				ui.Icon(source).ID("animated-icon").Size(size.Get()), wui.Switch()).Padding(20).Spacing(20),
		)).StyleSheet(modern.Sheet(modern.Options{}))
	})
	if err != nil {
		log.Fatal(err)
	}
}
