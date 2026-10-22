// 多语言资源、命令式翻译与声明式自动协调验收；不依赖 DevServer。
// 环境：Linux X11/Windows/macOS 桌面，记录 OS/WM、实际后端、1x/2x 与字体。
// 启动：go run ./tests/window/ui/mlang；-auto 顺序检查八语言后关闭退出。
// GOUI_PLAT_PAINTER=software/opengl（Windows 另可 direct2d），GOUI_PLAT_SCALE=2。
// 初始：英文 Integrated 窗口，八个语言按钮、同一 resource 包的 SVG/文本、
// 声明式/命令式问候、可编辑输入框和多文字系统样例，保存次数为零。
// 操作与预期：
//  1. 依次点击八种语言：标题、两处问候和保存按钮同步切换，无需手动 Rebuild；
//     再点同语言不增加 Build 计数。中/日/韩/俄/阿拉伯文不应有缺字方框。
//  2. 编辑输入框并选择文本，再切换：内容、选区及 Widget 身份保留；按钮
//     正常取得点击焦点，翻译更新本身不应额外抢回编辑焦点。
//  3. 输入框右键或 Shift+F10：七个命令按当前语言显示，复制、全选仍有效。
//     点击 Cycle 后展开菜单，观察每秒实时翻译、菜单尺寸调整与选区保留；
//     再点 Cycle 停止。循环不读写剪贴板，包内另有翻译后复制命令断言。
//  4. 保存计数递增；切换语言/Dark 后计数不丢失，SVG 前景跟随主题，长
//     俄文/法文和缩窄窗口正常重排。Check 输出 LANGUAGE PASS 与当前文案。
//  5. -auto 逐语言检查原 Widget、编辑内容/选区、两种翻译方式、SVG Source
//     身份及 1x/2x 离屏文字非空。自动编辑内容由 setter 准备，不冒充键盘验收。
//
// 差异：原生 OS 控制按钮遵循 OS 语言；自绘控制语义文本随 mlang 更新。
// 阿拉伯文验证字形排版，不承诺 RTL 控件顺序/整窗镜像。字体由系统提供。
// 副作用：仅窗口/日志；手动复制会写剪贴板，不自动读写文件/剪贴板。
// 复位：重启。退出：Close/原生关闭；自动完成输出 LANGUAGE EXIT PASS。
package main

import (
	"flag"
	"fmt"
	"log"
	"time"

	"github.com/golang-gui/goui/core/signal"
	"github.com/golang-gui/goui/gui"
	"github.com/golang-gui/goui/mlang"
	"github.com/golang-gui/goui/tests/window/ui/mlang/resource"
	"github.com/golang-gui/goui/theme/modern"
	"github.com/golang-gui/goui/ui"
)

var languages = []struct{ tag, name, greeting string }{
	{"en", "English", "Hello — welcome!"}, {"zh-CN", "中文", "你好，欢迎使用！"},
	{"ja", "日本語", "こんにちは、ようこそ！"}, {"ko", "한국어", "안녕하세요, 환영합니다!"},
	{"de", "Deutsch", "Hallo — willkommen!"}, {"fr", "Français", "Bonjour — bienvenue !"},
	{"ru", "Русский", "Здравствуйте — добро пожаловать!"}, {"ar", "العربية", "مرحباً — أهلاً وسهلاً!"},
}

// The application-owned View demonstrates imperative translateUI and explicit
// connection ownership, using the normal public WidgetView extension.
type commandLabelView struct{ ui.ViewBase[commandLabelView] }

func commandLabel() *commandLabelView {
	v := &commandLabelView{}
	v.Self = v
	return v
}
func (v *commandLabelView) Build() ui.View { return v }
func (*commandLabelView) Mount(ctx ui.BuildContext) gui.Widget {
	label := gui.NewLabel("")
	translateUI := func() { label.SetText(resource.Greeting()) }
	translateUI()
	ctx.SetState(mlang.ConnectChanged(translateUI))
	return label
}
func (*commandLabelView) Update(ui.BuildContext, gui.Widget) {}
func (*commandLabelView) Unmount(ctx ui.BuildContext, _ gui.Widget) {
	ctx.State().(signal.Handle).Disconnect()
}

func main() {
	auto := flag.Bool("auto", false, "自动验证八语言后退出")
	flag.Parse()
	if err := resource.Load(); err != nil {
		log.Fatal(err)
	}
	builds, saves := 0, 0
	dark, setup, destroyed := false, false, false
	var first []gui.Widget
	var cycle *ui.Timer
	var verify func()
	err := ui.Run("org.golang-gui.LanguageTest", func(app ui.App) ui.RootView {
		builds++
		change := func(tag string) {
			if err := mlang.SetLanguage(tag); err != nil {
				panic(err)
			}
		}
		verify = func() {
			ids := []string{"declarative", "command", "editor", "icon"}
			for i, id := range ids {
				if len(first) > 0 && app.FindWidget("main", id) != first[i] {
					panic("widget replaced")
				}
			}
			label := app.FindWidget("main", "declarative").(*gui.Label)
			command := app.FindWidget("main", "command").(*gui.Label)
			icon := app.FindWidget("main", "icon").(*gui.Icon)
			if label.Text() != resource.Greeting() || command.Text() != resource.Greeting() || icon.Source() != resource.SaveIcon {
				panic("translation/source mismatch")
			}
			for _, scale := range []float32{1, 2} {
				pixels, err := gui.RenderWidget(label, scale)
				if err != nil || pixels == nil || pixels.Bounds().Empty() {
					panic(fmt.Sprintf("text raster: %v", err))
				}
				ink := false
				for y := pixels.Bounds().Min.Y; y < pixels.Bounds().Max.Y && !ink; y++ {
					for x := pixels.Bounds().Min.X; x < pixels.Bounds().Max.X; x++ {
						_, _, _, a := pixels.At(x, y).RGBA()
						if a != 0 {
							ink = true
							break
						}
					}
				}
				if !ink {
					panic("text raster contains no ink")
				}
			}
			input := app.FindWidget("main", "editor").(*gui.TextInput)
			fmt.Printf("LANGUAGE PASS tag=%s builds=%d text=%q input=%q selection=%+v source/widgets=stable\n",
				mlang.Language(), builds, label.Text(), input.Text(), input.Selection())
		}
		closeWindow := func() {
			if err := app.FindWindow("main").RequestClose(); err != nil {
				panic(err)
			}
		}
		if !setup {
			setup = true
			app.TimeoutFunc(250*time.Millisecond, func() {
				for _, id := range []string{"declarative", "command", "editor", "icon"} {
					first = append(first, app.FindWidget("main", id))
				}
				if !*auto {
					return
				}
				input := first[2].(*gui.TextInput)
				input.SetText("user-owned text")
				input.SetSelection(gui.TextSelection{Anchor: 1, Caret: 5})
				var step func(int)
				step = func(index int) {
					change(languages[index].tag)
					app.TimeoutFunc(160*time.Millisecond, func() {
						verify()
						if first[0].(*gui.Label).Text() != languages[index].greeting || input.Text() != "user-owned text" || input.Selection() != (gui.TextSelection{Anchor: 1, Caret: 5}) {
							panic("language/editor state mismatch")
						}
						if index+1 < len(languages) {
							step(index + 1)
						} else {
							closeWindow()
						}
					})
				}
				step(0)
			})
		}
		buttons := make([]ui.View, 0, len(languages))
		for _, language := range languages {
			buttons = append(buttons, ui.Button(language.name).OnClick(func() { change(language.tag) }))
		}
		return ui.Root().StyleSheet(modern.Sheet(modern.Options{Dark: dark})).Windows(
			ui.Window("main").Title(resource.Title()).Size(940, 590).Chrome(ui.WindowChromeIntegrated).
				OnDestroy(func() {
					destroyed = true
					if cycle != nil {
						cycle.Stop()
					}
				}).Content(ui.VBox(
				ui.HeaderBar(ui.Label(resource.Title())),
				ui.VBox(
					ui.HBox(buttons...).Spacing(6),
					ui.Label(fmt.Sprintf("Current: %s   Build: %d   Saves: %d", mlang.Language(), builds, saves)),
					ui.HBox(ui.Icon(resource.SaveIcon).ID("icon").Size(28).Style(modern.AccentIcon), ui.Label(resource.Greeting()).ID("declarative")).Spacing(12),
					commandLabel().ID("command"), ui.TextInput().ID("editor"),
					ui.Label("English · 中文 · 日本語 · 한국어 · Deutsch · Français · Русский · العربية"),
					ui.HBox(ui.Button(resource.Save()).OnClick(func() { saves++; app.RequestUpdate() }),
						ui.Button("Dark").OnClick(func() { dark = !dark; app.RequestUpdate() }),
						ui.Button("Cycle").OnClick(func() {
							if cycle != nil && cycle.Active() {
								cycle.Stop()
								return
							}
							index := 0
							cycle = app.TickFunc(time.Second, func() { index = (index + 1) % len(languages); change(languages[index].tag) })
						}), ui.Button("Check").OnClick(func() { verify() }), ui.Button("Close").OnClick(closeWindow),
					).Spacing(8),
				).Padding(20).Spacing(18),
			)),
		)
	})
	if err != nil {
		log.Fatal(err)
	}
	if !destroyed {
		log.Fatal("window was not destroyed")
	}
	fmt.Println("LANGUAGE EXIT PASS")
}
