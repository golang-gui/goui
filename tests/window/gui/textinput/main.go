// 单行遮蔽与两类编辑控件的输入策略验证，不使用 DevServer。
//
// 环境：原生桌面、字体和事件循环。Linux 使用独立 X11 会话／窗口管理器。
// 独立会话无 Fcitx/IBus 时使用 XMODIFIERS=@im=none 启用本地 XIM，避免继承
// 指向其它 DISPLAY 的输入法环境。真实中文组合仍需要会话内的中文输入法服务。
// 启动：GOUI_PLAT_PAINTER=opengl GOUI_PLAT_SCALE=1 go run ./tests/window/gui/textinput
// 另验 software、倍率 2；Windows 可另验 direct2d。默认自动验证后退出。
// -auto=false 保留窗口人工操作；-clipboard 显式启用会覆盖剪贴板的自动检查。
// -native-check 配合 -auto=false：输出各编辑器几何，使用真实桌面输入填写用户名
// User_12@、验证码 12x34567、密码 ab#12、多行 line1 换行 line2；在右侧多行输入 x
// 应被拒绝。密码全选后复制／剪切应无效。按 F6 检查上述结果；本模式也适用于
// xdotool / Computer Use，不会从日志调用任何控件动作。
// 初始：用户名（ASCII 字母数字 _-.，12 字素）、验证码（6 位数字）、遮蔽密码、
// 明文参考框与共享模型的两个多行框（分别限 12 字素／仅数字）。
// 自动步骤与逐项预期：
//  1. 提交非法用户名不改变正文；超长合法用户名只接收前缀；撤销／重做准确恢复。
//  2. 数字策略对替换同样生效；程序赋值不受策略限制。
//  3. 组合字符、中文、Emoji 各显示一个掩码；与独立明文掩码参考的预乘 RGBA 精确
//     相同且非空。切换星号／圆点／普通文本均跟随变化，不改正文与逻辑选区。
//  4. 右键／左右键／删除／撤销使用源 UTF-8 位置，Emoji 不拆开；快照无密码原文。
//  5. -clipboard：复制／剪切后仍为标记文本，粘贴可替换选区并经过输入策略。
//  6. 多行限制保留换行，另一个视图同步模型但不应用自身策略；外部模型赋值保持原样。
//
// 键盘自动操作经 Window.DispatchEvent；文字经 Window.CommitText 进入正常 IMContext。
// 自动提交不代替真实中文 IME：-auto=false 下用系统输入法在密码框组合／取消／提交，
// 应始终遮蔽预编辑，候选窗跟随掩码光标；组合阶段不提前限长，最终提交才执行策略。
// 人工：右键密码框，复制／剪切应禁用；长密码应水平滚动，拖选／Shift+箭头不拆字素。
// 点 Toggle mask / Mask character / Reset 检查显示和复位，Tab 可切换字段。
// 像素只比较同后端／倍率的离屏图像，不跨平台字体金图，不代表桌面合成验收。
// 副作用：创建窗口；仅 -clipboard 自动覆盖桌面剪贴板，不恢复旧值；不修改系统设置。
// 复位：Reset 或重启。退出：自动 PASS 后或关闭窗口；超时／提前退出返回非零。
package main

import (
	"bytes"
	"encoding/json"
	"flag"
	"fmt"
	"image"
	"log"
	"os"
	"strings"
	"time"

	"github.com/golang-gui/goui/core/geometry"
	"github.com/golang-gui/goui/gui"
	"github.com/golang-gui/goui/gui/textedit"
	"github.com/golang-gui/goui/layout"
	"github.com/golang-gui/goui/platform/events"
	"github.com/golang-gui/goui/platform/graphics"
	"github.com/golang-gui/goui/theme/modern"
)

func must(err error) {
	if err != nil {
		panic(err)
	}
}
func check(ok bool, message string) {
	if !ok {
		panic(message)
	}
}
func button(text string, fn func()) *gui.Button {
	b := gui.NewButton()
	b.SetChild(gui.NewLabel(text))
	b.ConnectClicked(fn)
	return b
}

type frameProbe struct {
	gui.WidgetBase
	frames int
	scale  float32
	image  graphics.Image
}

func (w *frameProbe) Paint(p gui.Painter) {
	w.frames++
	w.scale = p.PixelScale()
	if w.image == nil {
		var err error
		w.image, err = p.NewImage(image.NewRGBA(image.Rect(0, 0, 1, 1)))
		must(err)
		actual := strings.ToLower(fmt.Sprintf("%T", w.image))
		requested := os.Getenv("GOUI_PLAT_PAINTER")
		check(requested == "" || strings.Contains(actual, requested+"."), "unexpected backend "+actual)
		fmt.Printf("BACKEND %s scale=%g\n", actual, w.scale)
	}
}
func pixels(w gui.Widget) []byte {
	img, err := gui.RenderWidget(w, 0)
	must(err)
	return graphics.CopyToBitmap(img, graphics.PixelFormatRGBA, nil).Pixels
}

func main() {
	auto := flag.Bool("auto", true, "自动验证后退出")
	clipboard := flag.Bool("clipboard", false, "允许自动检查覆盖剪贴板")
	nativeCheck := flag.Bool("native-check", false, "F6 检查约定的真实桌面输入结果")
	flag.Parse()
	app, err := gui.NewApplication("org.golanggui.TextInputPolicies")
	must(err)
	app.SetStyleSheet(modern.Sheet(modern.Options{}))
	win, err := app.NewWindow(&gui.WindowOptions{Size: geometry.Size{Width: 560, Height: 430}, Center: true})
	must(err)
	defer win.Destroy()
	must(win.SetTitle("GOUI TextInput policies"))
	root := gui.NewLinearBox(layout.DirectionVertical)
	root.SetPadding(geometry.UniformInsets(12))
	root.SetSpacing(6)
	root.SetCrossAlign(layout.CrossStretch)
	add := func(title, id string, filter gui.InputFilter) *gui.TextInput {
		root.AddChild(gui.NewLabel(title))
		w := gui.NewTextInput()
		w.SetID(id)
		w.SetInputFilter(filter)
		root.AddChild(w)
		return w
	}
	username := add("Username: ASCII letters / digits / _-. / max 12", "username", textedit.ASCIIUsername(12))
	pin := add("Code: ASCII digits / max 6", "code", textedit.Chain(textedit.DigitsOnly(), textedit.LimitLength(6)))
	secret := add("Obscured: Unicode / no controls / max 128", "secret", textedit.Password(128))
	secret.SetObscured(true)
	reference := add("Independent plaintext-mask reference", "reference", nil)
	model := gui.NewTextModel("")
	row := gui.NewLinearBox(layout.DirectionHorizontal)
	row.SetSpacing(8)
	var notes [2]*gui.TextView
	for i := range notes {
		v := gui.NewTextView()
		v.SetModel(model)
		v.SetID(fmt.Sprintf("notes-%d", i))
		v.SetMinSize(geometry.Size{Height: 70})
		if i == 0 {
			v.SetInputFilter(textedit.LimitLength(12))
		} else {
			v.SetInputFilter(textedit.DigitsOnly())
		}
		s := gui.NewScrollView()
		s.SetStyleName("text-view")
		s.SetMainWeight(1)
		s.SetChild(v)
		row.AddChild(s)
		notes[i] = v
	}
	root.AddChild(gui.NewLabel("Shared TextView: left max 12 / right digits only"))
	root.AddChild(row)
	controls := gui.NewLinearBox(layout.DirectionHorizontal)
	controls.SetSpacing(8)
	controls.AddChild(button("Toggle mask", func() { secret.SetObscured(!secret.Obscured()) }))
	controls.AddChild(button("Mask character", func() {
		if secret.ObscureChar() == '*' {
			secret.SetObscureChar('•')
		} else {
			secret.SetObscureChar('*')
		}
	}))
	controls.AddChild(button("Reset", func() { username.SetText(""); pin.SetText(""); secret.SetText(""); model.SetText("") }))
	root.AddChild(controls)
	probe := &frameProbe{}
	probe.SetMinSize(geometry.Size{Width: 1, Height: 1})
	probe.ConnectUnmount(func() {
		if probe.image != nil {
			probe.image.Destroy()
			probe.image = nil
		}
	})
	root.AddChild(probe)
	win.SetWidget(root)
	must(win.Show())
	if !*auto {
		if *clipboard {
			check(app.Clipboard() != nil, "clipboard unavailable")
			must(app.Clipboard().SetText("goui clipboard sentinel"))
		}
		if *nativeCheck {
			s := gui.NewShortcut(gui.KeyGesture{Key: gui.KeyF6})
			s.ConnectActivate(func() {
				fmt.Printf("NATIVE username=%q code=%q secretBytes=%d notes=%q\n", username.Text(), pin.Text(), len(secret.Text()), model.Text())
				check(username.Text() == "User_12", "native username input mismatch")
				check(pin.Text() == "123456", "native digits input mismatch")
				check(secret.Text() == "ab#12" && secret.Obscured() && secret.Snapshot().Text == "", "native obscured input/copy/cut mismatch")
				check(model.Text() == "line1\nline2", "native multiline/shared filter mismatch")
				fmt.Println("PASS native mouse/key input / filters / obscured copy-cut")
				if *clipboard {
					app.Clipboard().RequestText(func(text string, err error) {
						must(err)
						check(text == "goui clipboard sentinel", "native copy/cut changed clipboard")
						fmt.Println("PASS native clipboard unchanged")
					})
				}
			})
			win.Shortcuts().AddShortcut(s)
			ready := app.NewTimer()
			ready.ConnectTimeout(func() {
				if probe.frames == 0 {
					return
				}
				bounds := map[string]geometry.Rectangle{}
				for _, w := range []gui.Widget{username, pin, secret, notes[0], notes[1]} {
					bounds[w.ID()] = w.Snapshot().Bounds
				}
				data, err := json.Marshal(bounds)
				must(err)
				fmt.Printf("READY %s\n", data)
				ready.Stop()
			})
			must(ready.Start(100 * time.Millisecond))
		}
		app.Run()
		return
	}
	primary, _ := gui.ModPrimary.Resolve()
	key := func(k events.Key, mods events.Modifiers) {
		for _, kind := range []events.EventType{events.KeyDown, events.KeyUp} {
			must(win.DispatchEvent(events.KeyEvent{EventType: kind, Key: k, Modifiers: mods}))
		}
	}
	focus := func(w gui.Widget) { check(win.SetFocusedWidget(w), "cannot focus editor") }
	compare := func(expected string) {
		win.SetFocusedWidget(nil)
		secret.SetSelection(gui.TextSelection{})
		reference.SetSelection(gui.TextSelection{})
		reference.SetText(expected)
		a, b := pixels(secret), pixels(reference)
		check(bytes.Equal(a, b), "masked pixels differ from independent reference")
		reference.SetText("")
		check(!bytes.Equal(a, pixels(reference)), "text did not produce pixels beyond the field chrome")
		reference.SetText(expected)
	}
	source := "é中👨‍👩‍👧‍👦"
	phase, done, pending := 0, false, 0
	deadline := time.Now().Add(20 * time.Second)
	clipReady := false
	clipText := ""
	timer := app.NewTimer()
	defer timer.Stop()
	timer.ConnectTimeout(func() {
		check(time.Now().Before(deadline), "verification timed out")
		if probe.frames == 0 || probe.frames < pending {
			return
		}
		switch phase {
		case 0:
			focus(username)
			must(win.CommitText("bad@name"))
			check(username.Text() == "", "invalid username was accepted")
			must(win.CommitText("user.name_12345"))
			check(username.Text() == "user.name_12", "username prefix limit failed")
			key(events.KeyZ, primary)
			check(username.Text() == "", "undo filtered edit failed")
			key(events.KeyZ, primary|events.ModifierShift)
			check(username.Text() == "user.name_12", "redo failed")
		case 1:
			focus(pin)
			pin.SetText("12")
			must(win.CommitText("abc"))
			check(pin.Text() == "12", "non-digit input accepted")
			must(win.CommitText("3456789"))
			check(pin.Text() == "123456", "digit limit failed")
			pin.SetSelection(gui.TextSelection{Anchor: 1, Caret: 3})
			must(win.CommitText("9"))
			check(pin.Text() == "19456", "selected replacement failed")
			key(events.KeyZ, primary)
			check(pin.Text() == "123456", "replacement undo failed")
		case 2:
			secret.SetText(source)
			secret.SetObscureChar('*')
			compare("***")
			secret.SetObscureChar('•')
			compare("•••")
			secret.SetObscured(false)
			compare(source)
			secret.SetObscured(true)
			reference.SetText("•••")
		case 3:
			focus(secret)
			secret.SetSelection(gui.TextSelection{})
			for _, end := range []int{3, 6, len(source)} {
				key(events.KeyArrowRight, 0)
				check(secret.Selection().Caret == end, "navigation split source grapheme")
			}
			key(events.KeyBackspace, 0)
			check(secret.Text() == "é中", "backspace split emoji")
			key(events.KeyZ, primary)
			check(secret.Text() == source, "obscured undo failed")
			info := secret.Snapshot()
			data, err := json.Marshal(info)
			must(err)
			check(info.Text == "" && info.TextEditing.Obscured && info.TextEditing.VisibleText == "" && !bytes.Contains(data, []byte(source)), "snapshot exposed plaintext")
		case 4:
			if *clipboard {
				check(app.Clipboard() != nil, "clipboard unavailable")
				must(app.Clipboard().SetText("goui clipboard sentinel"))
				secret.SetSelection(gui.TextSelection{Anchor: 0, Caret: len(source)})
				key(events.KeyC, primary)
				key(events.KeyX, primary)
				check(secret.Text() == source, "cut changed obscured text")
				app.Clipboard().RequestText(func(text string, err error) { must(err); clipText = text; clipReady = true })
			} else {
				fmt.Println("SKIP clipboard (enable with -clipboard)")
			}
		case 5:
			if *clipboard {
				if !clipReady {
					return
				}
				check(clipText == "goui clipboard sentinel", "copy/cut changed clipboard")
				must(app.Clipboard().SetText("77"))
				key(events.KeyV, primary)
			}
		case 6:
			if *clipboard && secret.Text() != "77" {
				return
			}
			must(win.CommitText("\t"))
			check(!strings.Contains(secret.Text(), "\t"), "password control rune accepted")
			focus(notes[0])
			must(win.CommitText("第一行\nsecond line"))
			check(model.Text() == "第一行\nsecond l" && notes[1].Model() == model, "multiline limit or shared model failed")
			focus(notes[1])
			notes[1].SetSelection(gui.TextSelection{Anchor: model.Len(), Caret: model.Len()})
			must(win.CommitText("x"))
			check(model.Text() == "第一行\nsecond l", "second view filter failed")
			model.SetText("programmatic value beyond user limit")
			check(model.Text() == "programmatic value beyond user limit", "model mutation filtered")
		case 7:
			fmt.Println("PASS filters / Unicode masks / source geometry / history / snapshot / shared model")
			done = true
			timer.Stop()
			win.Destroy()
			return
		}
		fmt.Printf("CHECK %d PASS\n", phase)
		phase++
		pending = probe.frames + 1
		must(win.RequestPaint())
	})
	must(timer.Start(100 * time.Millisecond))
	app.Run()
	if !done {
		log.Fatal("window closed before verification completed")
	}
}
