// Multilingual resources, imperative translation and declarative reconciliation
// window checks, without DevServer.
// Environment: Linux X11/Windows/macOS desktop. Record OS/WM, actual backend,
// fonts and 1x/2x. Run: go run ./tests/window/ui/mlang
// -auto checks all eight languages sequentially and closes. Use
// GOUI_PLAT_PAINTER=software/opengl (also direct2d on Windows), GOUI_PLAT_SCALE=2.
// Initially an English Integrated window has eight language buttons, SVG/text
// from one resource package, declarative/imperative greetings, editable input,
// mixed-script samples and zero saves.
// Steps and expected results:
//  1. Choose all eight languages: title, both greetings and Save update without
//     manual Rebuild. Choosing the active language adds no Build. Chinese,
//     Japanese, Korean, Russian and Arabic have no missing-glyph boxes.
//  2. Edit/select text, then change language: content, selection and identity
//     remain. Button clicks take normal focus; translation updates do not
//     additionally return focus to the editor.
//  3. Right-click or Shift+F10 in the input: seven commands use the current
//     language, and Copy/Select All work. Cycle updates an open menu once per
//     second, resizing it and preserving selection; click again to stop. Cycling
//     itself accesses no clipboard. Package tests cover Copy after translation.
//  4. Save increments its count, which survives language/Dark changes. SVG
//     foreground follows the theme. Long Russian/French and narrow sizes reflow.
//     Check prints LANGUAGE PASS and current labels.
//  5. -auto checks original widget identity, text/selection, both translation
//     paths, SVG Source identity and nonempty 1x/2x offscreen text per language.
//     Its setters prepare data; they do not verify keyboard input.
//
// Platform differences: native controls use the OS language; custom control
// semantics follow mlang. Arabic tests shaping, not RTL widget order/window
// mirroring. Fonts come from the system. Side effects: windows/logs; explicit
// Copy changes the clipboard, with no automatic file/clipboard access.
// Reset: restart. Exit: Close/native close; automatic completion prints
// LANGUAGE EXIT PASS.
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
