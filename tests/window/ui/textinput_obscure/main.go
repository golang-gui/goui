// 密码末字素短暂显露与声明式重建验收，不使用 DevServer。
//
// 环境：Linux X11 / Windows / macOS 桌面、原生字体与事件循环。
// 启动：GOUI_PLAT_PAINTER=opengl GOUI_PLAT_SCALE=1 go run ./tests/window/ui/textinput_obscure
// 另验 software、倍率 2；Windows 可另验 direct2d。默认自动断言后退出。
// -auto=false 保留窗口手工操作，-hold 在自动检查后保留窗口。
// 独立 X11 会话没有输入法服务时使用 XMODIFIERS=@im=none，不继承其它桌面的 Fcitx。
// 初始：遮蔽框开启 2 秒反馈；下方独立参考框只用于像素断言。
// 操作与预期：
//  1. 新输入一个中文、组合字素或 Emoji：只有这个完整字素短暂显示，2 秒后遮蔽。
//  2. 点击 Rebuild：控件身份、正文与选区不变，倒计时不延长。
//  3. 移动光标、改变选区、焦点移至参考框：立即遮蔽；重新聚焦不恢复显露。
//  4. 多字符提交、程序赋值不显露；窗口自动步骤检查相同行为及源字素导航。
//  5. 原生输入复验：重启 -auto=false，在密码框键入 ab#12 后立即按 F7，
//     检查画面与 ••••2 一致；等待超过 2 秒按 F6，检查画面与 ••••• 一致。
//     F6/F7 均断言 Snapshot 无正文。日志 READY 给出鼠标命中区域，供真实桌面操作。
//
// 自动步骤使用 Window.CommitText / DispatchEvent，检查 UI 绑定写回与重建不重启计时。
// 比较同倍率、同字体的非空预乘 RGBA，不使用跨平台字体金图；RenderWidget 是离屏
// 像素检查，不等同于桌面合成验收。测试样式统一两框边框并隐藏光标，避免焦点闪烁
// 干扰像素比较；正常主题的光标和 IME 候选交互需人工另验。
// 副作用：仅创建窗口，不操作剪贴板、网络或系统设置，不写入源码目录。
// 复位：Reset 或重启。退出：自动 PASS 后或关闭窗口，提前退出和超时返回非零。
package main

import (
	"bytes"
	"encoding/json"
	"flag"
	"fmt"
	"image"
	"image/color"
	"log"
	"os"
	"strings"
	"time"

	"github.com/golang-gui/goui/gui"
	"github.com/golang-gui/goui/layout"
	"github.com/golang-gui/goui/platform/events"
	"github.com/golang-gui/goui/platform/graphics"
	"github.com/golang-gui/goui/style"
	"github.com/golang-gui/goui/ui"
)

func check(ok bool, message string) {
	if !ok {
		panic(message)
	}
}
func must(err error) {
	if err != nil {
		panic(err)
	}
}

type frameProbe struct {
	gui.WidgetBase
	frames int
	image  graphics.Image
}

func (w *frameProbe) Paint(p gui.Painter) {
	w.frames++
	if w.image == nil {
		var err error
		w.image, err = p.NewImage(image.NewRGBA(image.Rect(0, 0, 1, 1)))
		must(err)
		actual, requested := strings.ToLower(fmt.Sprintf("%T", w.image)), os.Getenv("GOUI_PLAT_PAINTER")
		check(requested == "" || strings.Contains(actual, requested+"."), "unexpected backend "+actual)
		fmt.Printf("BACKEND %s scale=%g\n", actual, p.PixelScale())
	}
}

type probeView struct {
	ui.ViewBase[probeView]
	probe *frameProbe
}

func (v *probeView) Mount(ui.BuildContext) gui.Widget { return v.probe }
func (*probeView) Update(ui.BuildContext, gui.Widget) {}
func (v *probeView) Unmount(ui.BuildContext, gui.Widget) {
	if v.probe.image != nil {
		v.probe.image.Destroy()
		v.probe.image = nil
	}
}

func pixels(w gui.Widget) []byte {
	img, err := gui.RenderWidget(w, 0)
	must(err)
	return graphics.CopyToBitmap(img, graphics.PixelFormatRGBA, nil).Pixels
}

func main() {
	auto := flag.Bool("auto", true, "自动验收后退出")
	hold := flag.Bool("hold", false, "自动验收后保留窗口")
	flag.Parse()
	const delay = 2 * time.Second
	text := ui.MakeState("")
	probe := &probeView{probe: &frameProbe{}}
	probe.Self = probe
	rules := append(gui.DefaultStyleRules(),
		style.Name("obscure-test").FontSize(18).ForegroundColor(color.Black).BackgroundColor(color.White).BorderWidth(1).BorderColor(color.Gray{Y: 150}).Radius(4),
		style.Name("obscure-test").Part("caret").ForegroundColor(color.RGBA{}),
	)
	sheet := style.Sheet(rules...)
	started, done, phase := false, false, 0
	var committed time.Time
	var original *gui.TextInput
	deadline := time.Now().Add(20 * time.Second)
	err := ui.Run("org.golanggui.ObscureDelay", func(app ui.App) ui.RootView {
		if !started {
			started = true
			var timer *ui.Timer
			timer = app.TickFunc(80*time.Millisecond, func() {
				check(time.Now().Before(deadline), "obscure delay verification timed out")
				if probe.probe.frames == 0 {
					return
				}
				win := app.FindWindow("main")
				input := app.FindWidget("main", "secret").(*gui.TextInput)
				reference := app.FindWidget("main", "reference").(*gui.TextInput)
				compare := func(expected string) {
					reference.SetText(expected)
					a, b := pixels(input), pixels(reference)
					check(bytes.Equal(a, b), fmt.Sprintf("phase %d: display pixels differ from independent reference", phase))
					reference.SetText("")
					check(!bytes.Equal(a, pixels(reference)), "no text pixels beyond field chrome")
					reference.SetText(expected)
					info := input.Snapshot()
					check(info.Text == "" && info.TextEditing.Obscured && info.TextEditing.VisibleText == "", "semantic plaintext escaped obscuring")
				}
				if original == nil {
					original = input
					bounds, e := json.Marshal(input.Snapshot().Bounds)
					must(e)
					fmt.Printf("READY %s\n", bounds)
					for _, entry := range []struct {
						key  gui.Key
						name string
						want string
					}{{gui.KeyF7, "F7", "••••2"}, {gui.KeyF6, "F6", "•••••"}} {
						s := gui.NewShortcut(gui.KeyGesture{Key: entry.key})
						s.ConnectActivate(func() {
							check(input.Text() == "ab#12", "native input did not reach the bound editor")
							compare(entry.want)
							fmt.Println("PASS native input", entry.name)
						})
						win.Shortcuts().AddShortcut(s)
					}
				}
				if !*auto || done {
					timer.Stop()
					return
				}
				check(original == input && input.Text() == text.Get(), "declaration changed widget identity or text binding")
				switch phase {
				case 0:
					check(win.SetFocusedWidget(input), "cannot focus secret input")
					input.SetText("é")
					committed = time.Now()
					must(win.CommitText("中"))
				case 1:
					if time.Since(committed) < 500*time.Millisecond {
						return
					}
					compare("•中")
					app.RequestUpdate()
				case 2:
					if time.Since(committed) < 900*time.Millisecond {
						return
					}
					compare("•中")
					app.RequestUpdate()
				case 3:
					if time.Since(committed) < 2300*time.Millisecond {
						return
					}
					compare("••")
					input.SetText("é")
					must(win.CommitText("👨‍👩‍👧‍👦"))
				case 4:
					compare("•👨‍👩‍👧‍👦")
					must(win.DispatchEvent(events.KeyEvent{EventType: events.KeyDown, Key: events.KeyArrowLeft}))
					check(input.Selection().Caret == 3, "navigation entered a revealed source grapheme")
				case 5:
					compare("••")
					input.SetText("é")
					must(win.CommitText("中"))
					check(win.SetFocusedWidget(reference), "cannot move focus")
				case 6:
					compare("••")
					check(win.SetFocusedWidget(input), "cannot restore focus")
					input.SetText("")
					must(win.CommitText("中文"))
				case 7:
					compare("••")
					fmt.Println("PASS delayed reveal / Unicode mapping / declarative rebuild / focus / source navigation / semantic privacy")
					done = true
					timer.Stop()
					if !*hold {
						app.Quit()
					}
					return
				}
				fmt.Printf("CHECK %d PASS\n", phase)
				phase++
			})
		}
		return ui.Root().Windows(ui.Window("main").Title("GOUI ObscureDelay").Size(520, 230).Center(true).Content(
			ui.VBox(
				ui.Label("Password: last grapheme visible for 2 seconds"),
				ui.TextInput().ID("secret").BindText(&text).Obscured(true).ObscureDelay(delay).Style("obscure-test").MinHeight(36).MaxHeight(36),
				ui.Label("Independent display reference"),
				ui.TextInput().ID("reference").Style("obscure-test").MinHeight(36).MaxHeight(36),
				ui.HBox(ui.Button("Rebuild").OnClick(app.RequestUpdate), ui.Button("Reset").OnClick(func() { text.Set("") })).Spacing(8),
				probe.MinHeight(1).MaxHeight(1),
			).Padding(12).Spacing(8).CrossAlign(layout.CrossStretch),
		)).StyleSheet(sheet)
	})
	if err != nil {
		log.Fatal(err)
	}
	if *auto && !done {
		log.Fatal("window closed before verification completed")
	}
}
