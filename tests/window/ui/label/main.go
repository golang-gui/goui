// 声明式 Label 格式化窗口验收，不使用 DevServer。
// 环境：Linux X11/Windows/macOS 桌面，系统字体；与 GUI label 例程相同的后端和倍率。
// 启动：go run ./tests/window/ui/label [-auto] [-capture-dir DIR]；DIR 为仓库外已有目录。
// 两种启动均先自动断言；普通启动保留窗口，待 PASS 后再执行手动操作。
// 初始：Modern 亮色，格式化翻译、局部绑定 Label、保留原始语法的编辑框。
// 操作与预期：
//  1. 点 Cycle：绑定内容依次格式化、错误源串、普通文本；只有属性更新，控件身份不变。
//  2. 点 Language：中英文完整句子切换，强调范围跟随翻译，参数中的格式字符原样显示。
//  3. 点 Theme：切换亮暗，字号/粗斜体保留，语义颜色更新，无多份 Label 拼接。
//  4. 编辑输入框：绑定 Label 同步显示；输入框和只读 TextView 始终保留原始语法。
//
// 自动步骤检查源码/纯文本/Snapshot、BindText 无全局重建、语言/主题重建后身份保留，
// 并可导出同后端原生排版生成的 Software 位图；像素参考断言在 GUI label 例程中。
// 平台差异仅原生字体、装饰和抗锯齿；不验收 Label 选择、复制、系统 IME。
// 副作用：进程内语言设置、窗口及可选 PNG；不改系统语言或剪贴板。
// 复位：重启。退出：关闭窗口；-auto 完成输出 PASS 后退出，提前退出或超时视为失败。
package main

import (
	"flag"
	"fmt"
	"image/png"
	"log"
	"os"
	"path/filepath"
	"time"

	"github.com/golang-gui/goui/gui"
	"github.com/golang-gui/goui/layout"
	"github.com/golang-gui/goui/mlang"
	"github.com/golang-gui/goui/textformat"
	"github.com/golang-gui/goui/theme/modern"
	"github.com/golang-gui/goui/ui"
)

var prompt = mlang.String(
	mlang.En(`\{format\}Save \{bold:%s\}? \{warning:Unsaved changes\}`),
	mlang.ZhCN(`\{format\}是否保存\{bold:%s\}？\{warning:尚有修改\}`),
)

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

func main() {
	auto := flag.Bool("auto", false, "自动验证后退出")
	capture := flag.String("capture-dir", "", "仓库外已有截图目录")
	flag.Parse()
	must(mlang.SetLanguage("en"))
	formatted := `\{format\}Bound \{bold:中文\} / \{italic:é\} / \{warning:warning\}`
	broken := `\{format\}\{bold:unclosed`
	source, dark := ui.MakeState(formatted), false
	file := `C:\data\{bold:literal\}.txt`
	builds, cycle, step, initialBuilds := 0, 0, 0, 0
	started, done := false, false
	var original *gui.Label
	deadline := time.Now().Add(20 * time.Second)
	err := ui.Run("org.golang-gui.UILabelFormat", func(app ui.App) ui.RootView {
		builds++
		cycleText := func() { cycle++; source.Set([]string{formatted, broken, "Plain: < > & C:\\data"}[cycle%3]) }
		language := func() {
			tag := "zh-CN"
			if mlang.Language() == tag {
				tag = "en"
			}
			must(mlang.SetLanguage(tag))
		}
		theme := func() { dark = !dark; app.RequestUpdate() }
		check := func() {
			label := app.FindWidget("main", "bound").(*gui.Label)
			parsed, e := textformat.Parse(source.Get())
			expected := parsed.Plain
			if e != nil {
				expected = source.Get()
			}
			require(label == original && label.Text() == source.Get() && label.PlainText() == expected && label.Snapshot().Text == expected, "binding identity or source/plain mismatch")
			require(!label.Focusable(), "Label became selectable")
			require(app.FindWidget("main", "input").(*gui.TextInput).Text() == source.Get(), "editor interpreted format")
			require(app.FindWidget("main", "readonly").(*gui.TextView).Model().Text() == source.Get(), "readonly editor interpreted format")
			translated := app.FindWidget("main", "translated").(*gui.Label)
			expectedPrompt := "Save " + file + "? Unsaved changes"
			if mlang.Language() == "zh-CN" {
				expectedPrompt = "是否保存" + file + "？尚有修改"
			}
			require(translated.PlainText() == expectedPrompt, "translation/range or literal argument mismatch")
			log.Printf("CHECK PASS step=%d builds=%d language=%s typography=%s allocation=%+v", step, builds, mlang.Language(), gui.App.Typography().Name(), app.FindWindow("main").Widget().Rect())
			if *capture != "" {
				img, e := gui.RenderWidget(app.FindWindow("main").Widget(), 0)
				must(e)
				f, e := os.Create(filepath.Join(*capture, fmt.Sprintf("ui-label-%d.png", step)))
				must(e)
				must(png.Encode(f, img))
				must(f.Close())
			}
		}
		if !started {
			started = true
			var timer *ui.Timer
			timer = app.TickFunc(250*time.Millisecond, func() {
				require(time.Now().Before(deadline), "UI label verification timed out")
				// RenderWidget 不主动布局。定时通知可能先于首次 WM_PAINT；
				// 先满足真实分配的前提，超时与后面的断言仍然保留。
				allocation := app.FindWindow("main").Widget().Rect()
				if allocation.Width <= 0 || allocation.Height <= 0 {
					return
				}
				switch step {
				case 0:
					original = app.FindWidget("main", "bound").(*gui.Label)
					initialBuilds = builds
					check()
					source.Set(formatted + " updated")
				case 1:
					check()
					require(builds == initialBuilds, "BindText rebuilt the application")
					source.Set(broken)
				case 2:
					check()
					require(builds == initialBuilds, "invalid format rebuilt the application")
					source.Set("Plain: < > & C:\\data")
				case 3:
					check()
					require(builds == initialBuilds, "plain text rebuilt the application")
					language()
				case 4:
					check()
					require(builds > initialBuilds, "language did not rebuild translated text")
					theme()
				case 5:
					check()
					source.Set(formatted)
				case 6:
					check()
					timer.Stop()
					done = true
					log.Println("PASS UI Label formatting, binding, language and theme")
					if *auto {
						app.FindWindow("main").Destroy()
					}
				}
				step++
			})
		}
		template := prompt()
		return ui.Root().StyleSheet(modern.Sheet(modern.Options{Dark: dark})).Windows(
			ui.Window("main").Title("GOUI UI Label format").Size(780, 500).Center(true).Chrome(gui.WindowChromeIntegrated).Content(
				ui.VBox(
					ui.HeaderBar(ui.Label("UI Label — Text / BindText / mlang")),
					ui.HBox(ui.Button("Cycle").OnClick(cycleText), ui.Button("Language").OnClick(language), ui.Button("Theme").OnClick(theme), ui.Button("Check").OnClick(check)).Spacing(8),
					ui.Label(fmt.Sprintf(template, textformat.EscapeArg(template, file))).ID("translated").WrapMode(ui.WrapWordChar),
					ui.Label(`\{format\}\{bold:Nested \{italic:styles\}\} / \{underline:underline\} / \{strikethrough:strike\}`),
					ui.Label().ID("bound").BindText(&source).WrapMode(ui.WrapWordChar),
					ui.TextInput().ID("input").BindText(&source),
					ui.ScrollView(ui.TextView().ID("readonly").ReadOnly(true).BindText(&source)).MinHeight(80).MaxHeight(80),
					ui.Label(`Raw: \{bold:literal\}  < > &`),
					ui.Label("Label remains non-selectable; editors display the source."),
				).Padding(16).Spacing(12).CrossAlign(layout.CrossStretch),
			),
		)
	})
	must(err)
	require(done, "window closed before verification completed")
}
