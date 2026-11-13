// 文本编辑器连续点击与整行拖选的原生窗口回归，不使用DevServer。
//
// 环境：Linux/X11（记录WM）、Windows或macOS桌面；记录实际后端与1x/2x。
// 启动：go run ./tests/window/gui/text_multiclick [-basic] [-dark]
// 可设置GOUI_PLAT_PAINTER=software/opengl/direct2d、GOUI_PLAT_SCALE=1/2。
// 初始：普通输入框one two、密码框、可折行且可滚动的多行框，以及只读aa/bb/空行/last。
// 等READY后按以下顺序操作，每步F6验证公开Selection、Snapshot及正文不变：
//  1. 双击普通框one，选中one；三击同处，选中one two全文。
//  2. 三击密码框，选中全部掩码，不能显示原文。
//  3. 三击左侧多行框首段的任一显示行，选中整个逻辑行和末尾LF。
//     选区可以跨自动折行，行尾只显示既有换行选中标记，不染满视口空白。
//  4. 三击只读bb，选中bb及LF；再次三击bb且第三次按住，拖到aa后释放，
//     反向选中aa、bb及两行LF；再次从bb三击拖到last，选中bb、空行、last。
//  5. 三击只读空行，仅选择其LF（可见窄高亮）；三击last，选到末尾而不虚构LF。
//  6. 回到左侧首段，三击且第三次按住拖出视口下方，停留至少300ms后释放：
//     视口向下滚，选区按逻辑行扩展且保留原始首段，释放后停止滚动。
//  7. F8：全部步骤验证后PASS、退出0。提前关闭或检查失败退出1。
//
// 三击保持既有500ms/4DIP判定，每次按下/抬起都要实际发送；F6会重置多击序列。
// macOS使用主键，Control+点击仍属于上下文菜单；功能键可能需要Fn。
// 原生高亮、自动折行、空行标记及滚动裁剪须同步观察，不能只凭Snapshot判定。
// 副作用：仅窗口和日志；不读写剪贴板或系统设置。复位：重启。退出：F8或关闭。
package main

import (
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"image"
	"os"
	"strings"

	"github.com/golang-gui/goui/core/geometry"
	"github.com/golang-gui/goui/gui"
	"github.com/golang-gui/goui/layout"
	"github.com/golang-gui/goui/style"
	"github.com/golang-gui/goui/theme/basic"
	"github.com/golang-gui/goui/theme/modern"
	"github.com/golang-gui/goui/ui"
)

func main() {
	fallback := flag.Bool("basic", false, "Basic theme")
	dark := flag.Bool("dark", false, "dark Modern palette")
	flag.Parse()
	if err := run(*fallback, *dark); err != nil {
		fmt.Fprintln(os.Stderr, "FAIL:", err)
		os.Exit(1)
	}
}

func run(fallback, dark bool) error {
	const first = "Alpha beta 中文 e\u0301 emoji 👩‍💻. This logical line wraps across the viewport."
	const password = "one two 中"
	const readOnly = "aa\nbb\n\nlast"
	text := first + "\n" + strings.Repeat("Scroll row: one two three.\n", 30) + "last"
	model, readModel := gui.NewTextModel(text), gui.NewTextModel(readOnly)
	steps := []struct {
		id, name string
		want     gui.TextSelection
	}{
		{"input", "double click one", gui.TextSelection{Caret: 3}},
		{"input", "triple click one", gui.TextSelection{Caret: 7}},
		{"password", "triple click password", gui.TextSelection{Caret: len(password)}},
		{"wrapped", "triple click wrapped first paragraph", gui.TextSelection{Caret: len(first) + 1}},
		{"readonly", "triple click bb", gui.TextSelection{Anchor: 3, Caret: 6}},
		{"readonly", "triple click bb and drag up to aa", gui.TextSelection{Anchor: 6, Caret: 0}},
		{"readonly", "triple click bb and drag down to last", gui.TextSelection{Anchor: 3, Caret: len(readOnly)}},
		{"readonly", "triple click empty line", gui.TextSelection{Anchor: 6, Caret: 7}},
		{"readonly", "triple click last", gui.TextSelection{Anchor: 7, Caret: len(readOnly)}},
		{"wrapped", "triple click first paragraph and drag below viewport", gui.TextSelection{}},
	}
	stage, ready, finished := 0, false, false
	var failure error
	err := ui.Run("org.golanggui.TextMulticlick", func(app ui.App) ui.RootView {
		fail := func(err error) { failure = err; app.Quit() }
		bounds := func() map[string]geometry.Rectangle {
			result := map[string]geometry.Rectangle{}
			for _, id := range []string{"input", "password", "wrapped", "readonly", "scroll"} {
				result[id] = app.FindWidget("main", id).Snapshot().Bounds
			}
			return result
		}
		check := func() {
			if !ready || stage >= len(steps) {
				fail(errors.New("not ready or all steps already checked"))
				return
			}
			step := steps[stage]
			info := app.FindWidget("main", step.id).Snapshot()
			selection := info.TextEditing.Selection
			if stage == len(steps)-1 {
				if selection.Anchor != 0 || selection.Caret <= len(first)+1 ||
					selection.Caret > len(text) || selection.Caret < len(text) && text[selection.Caret-1] != '\n' ||
					app.FindWidget("main", "scroll").Snapshot().ScrollY <= 0 {
					fail(fmt.Errorf("auto-scroll selection=%+v", selection))
					return
				}
			} else if selection != step.want {
				fail(fmt.Errorf("stage %d %s: selection=%+v want=%+v", stage, step.name, selection, step.want))
				return
			}
			input := app.FindWidget("main", "input").(*gui.TextInput)
			secret := app.FindWidget("main", "password").(*gui.TextInput)
			view := app.FindWidget("main", "wrapped").(*gui.TextView)
			read := app.FindWidget("main", "readonly").(*gui.TextView)
			if input.Text() != "one two" || secret.Text() != password || !secret.Snapshot().TextEditing.Obscured ||
				view.Model() != model || model.Text() != text || read.Model() != readModel || readModel.Text() != readOnly ||
				!read.ReadOnly() || info.Role != gui.RoleTextView && info.Role != gui.RoleTextInput {
				fail(errors.New("text, identity, read-only or semantic state changed"))
				return
			}
			data, _ := json.Marshal(bounds())
			fmt.Printf("CHECK stage=%d selection=%+v bounds=%s\n", stage, selection, data)
			stage++
			app.RequestUpdate()
		}
		rules := modern.Rules(modern.Options{Dark: dark})
		if fallback {
			rules = basic.Rules()
		}
		instruction := "F8 finish"
		if stage < len(steps) {
			instruction = fmt.Sprintf("%d: %s; F6 check", stage+1, steps[stage].name)
		}
		content := ui.VBox(
			ui.HBox(
				ui.VBox(ui.Label("TextInput"), ui.TextInput().ID("input").Text("one two").MinSize(360, 40).MaxSize(360, 40)),
				ui.VBox(ui.Label("Password"), ui.TextInput().ID("password").Text(password).Obscured(true).MinSize(360, 40).MaxSize(360, 40)),
			).Spacing(16),
			ui.HBox(
				ui.VBox(ui.Label("Wrapped TextView"), ui.ScrollView(ui.TextView().ID("wrapped").Model(model)).ID("scroll").Style("text-view").MinSize(360, 180).MaxSize(360, 180)),
				ui.VBox(ui.Label("Read-only: aa / bb / empty / last"), ui.ScrollView(ui.TextView().ID("readonly").Model(readModel).ReadOnly(true)).Style("text-view").MinSize(360, 180).MaxSize(360, 180)),
			).Spacing(16),
			ui.Label(instruction),
		).Spacing(16).Padding(16).CrossAlign(layout.CrossStart)
		return ui.Root().StyleSheet(style.Sheet(rules...)).Windows(ui.Window("main").Title("GOUI Text multiclick").Size(800, 430).Content(content).Shortcuts(
			ui.Shortcut(ui.KeyF6).OnActivate(check),
			ui.Shortcut(ui.KeyF8).OnActivate(func() {
				if stage != len(steps) {
					fail(fmt.Errorf("missing checks: %d/%d", stage, len(steps)))
					return
				}
				finished = true
				fmt.Println("PASS text multiclick")
				app.Quit()
			}),
		).OnAfterPaint(func(p gui.Painter, _ geometry.Rectangle) {
			if ready || app.FindWidget("main", "input").Snapshot().Bounds.X <= 0 {
				return
			}
			resource, err := p.NewImage(image.NewRGBA(image.Rect(0, 0, 1, 1)))
			if err != nil {
				failure = err
				app.Post(app.Quit)
				return
			}
			defer resource.Destroy()
			read := app.FindWidget("main", "readonly").(*gui.TextView)
			padding := read.Padding()
			rowHeight := (read.Measure(layout.Unbounded()).Height - padding.Top - padding.Bottom) / 4
			data, _ := json.Marshal(bounds())
			ready = true
			fmt.Printf("READY backend=%T scale=%g rowHeight=%g bounds=%s\n", resource, p.PixelScale(), rowHeight, data)
		}))
	})
	if err != nil {
		return err
	}
	if failure != nil {
		return failure
	}
	if !finished {
		return errors.New("closed before all checks")
	}
	return nil
}
