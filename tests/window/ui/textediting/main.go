// UI 文本编辑窗口验证：共享多行模型、独立单行编辑和声明式重建。
//
// 环境：桌面/字体库；记录 OS、后端、缩放（1x/2x）及 Linux WM。
// 启动：go run ./tests/window/ui/textediting -mb 1（也可 -mb 10）。
// Windows PowerShell：$env:GOUI_PLAT_PAINTER='direct2d'; $env:GOUI_PLAT_SCALE='1';
// go run ./tests/window/ui/textediting -mb 10
// macOS/Linux：GOUI_PLAT_PAINTER=opengl GOUI_PLAT_SCALE=2 go run ./tests/window/ui/textediting -mb 10
// Windows 重复 direct2d/opengl/software，macOS/Linux 重复 opengl/software，分别测 1x/2x。
// direct2d 仅为 Windows 优先路径，可能降级；以日志实际资源类型为准。测试后恢复
// shell 环境变量的此前值；强制 2x 不等于已验证真实 DPI 变化或跨屏候选窗坐标。
// macOS 下文 Ctrl 编辑快捷键改用 Cmd；没有 Home/End 时用 Fn+Left/Right 组合。
// Linux 隔离桌面若不运行外部输入法，使用 XMODIFIERS=@im=none 启动，选择
// libX11 本地输入法；不能继承指向不存在服务的 @im=fcitx/ibus 后验收普通键入。
// 本地输入法检查：第二个空框逐键输入 draft，再 Enter，应显示 draft 且 Submit
// 仅增加一次；全选复制必须精确为 draft（5 字节），不能预先粘贴代替键入。
// 真实 IME 验证则保持对应 XMODIFIERS，并确认服务运行于同一桌面；@im=none
// 不用于验收中文组合或候选窗。配置缺失服务的测试只能计作输入法不可用场景。
// 初始：上方两个单行输入框、中间一个值绑定多行框，下方两个共享模型的
// 多行编辑器，modern 亮色。第一个单行框绑定 Text/Selection，第二个不声明
// Text；状态和共享模型均在 Build 外创建。
// 首帧日志报告实际图像资源类型/PixelScale，Submit 日志报告累计次数；
// 不以环境变量或窗口成功启动代替实际后端和操作结果。
//
// 操作与预期：
//  1. 在第一个单行框中间定位、Shift 扩选、输入/粘贴多行文字：选区被替换，
//     每个归一化换行变成一个空格；文字不换行，长内容水平滚动，行高不变化。
//     Label 与输入文字的 baseline 对齐。组合字符/emoji 的导航和删除不能拆 Cluster。
//  2. 在单行框按 Enter：Submit 计数只加一、不插入空格或 LF，触发重建后光标
//     仍在原处。Ctrl/Cmd+Z 撤销，Shift+Ctrl/Cmd+Z 重做；一次粘贴/IME 提交一次撤销。
//  3. 在第二个框输入，然后点 Rebuild：内容保留。重复切换 Theme/Accent 后，
//     两个框和多行的内容、选区不变；背景/选区/光标随主题变化，字体不重复缩放。
//  4. 在左下多行中编辑，右下立即同步；两侧独立滚动/选择。Home/End、文档首尾、
//     跨段选区、拖选自动滚动、点击双击和只读行为与 GUI TextView 用例一致。
//     Readonly 按钮只限制声明式单行框与左下编辑器，仍可选择/复制；右下可编辑。
//  5. 用真实 IME 在选区组合、取消/提交；预编辑不进入 Text/共享模型，提交后同步。
//     Windows/macOS 检查内联组合和候选位置；Linux XIM root-style 的组合由 IME 显示。
//  6. 在 1/10 MiB、1x/2x 下滚动和 resize；布局保持可见文字锚点，选择/光标不越过
//     输入区域裁剪。关闭正在闪烁或组合的窗口应正常退出，无遗留窗口。
//  7. 初始第一个输入框 End、Shift+Left 后复制，应为完整 👩‍💻；接着 Left、Left、
//     Shift+Left 后复制，应为 e + combining acute（UTF-8 共 3 字节）。第二个框
//     粘贴 אבג，确认粘贴完成后全选、Left、Shift+Right，复制应为 ג；Backspace
//     后全文为 אב，撤销应恢复 אבג 和选中的 ג。不要在异步粘贴完成前改变选区：
//     过期回复应被丢弃，不能误记成粘贴成功或失败。
//  8. Linux 键盘布局回归仅在独立桌面执行（不要修改用户桌面）：设置 fr 布局、
//     XMODIFIERS=@im=none，第二个空框键入 a/z/é、死键 ^ 后 e、AltGr+E，
//     全选复制应精确为 azéê€（9 UTF-8 字节），不是美式布局对应的 q/w/2 等。
//     在左下选中最初 Edi，键入同样文字后 Enter；从右侧复制全文应精确为
//     "azéê€\n" + 初始全文去掉前三字节，其他内容不得变化。
//     保持程序运行，仅在独立桌面切换为 us intl；第二个框全选后键入 q/w、
//     死键单引号后 e，复制应为 qwé（4 字节）。记录实际键盘布局与输入法；
//     这些本地 Compose 操作不代替真实中文 IME 验收。退出并清理独立会话。
//  9. 在中间 Notes 框输入、移动光标/反向选择，观察下方 Notes 字节数与选区。
//     点击 Rebuild/Theme 后内容和选区保持，Ctrl/Cmd+Z 仍能撤销此前输入。
//     点击 Replace notes：内容变为 "Replacement\n中文"，光标位于 0，旧历史清空；
//     点击 Select notes：选中 "中文"（字节 12–18）。点击 Empty notes 清空。
//     先开启 Auto rebuild，再在 Notes 用中文 IME 组合：每秒重建，但正文计数
//     不包含预编辑、组合不被重建打断；提交后正文才增加，取消则正文不变。
//     再次点击 Auto rebuild 停止计时；退出应用也自动停止。不要在组合过程中
//     点击按钮换焦点，将焦点切换引起的取消误认为重建问题。
//
// 副作用：仅主动复制/剪切时写剪贴板，不改文件/系统设置。无 DevServer。
// 复位：重启程序。退出：关闭窗口。跨平台场景须分别记录，启动不代表验收通过。
package main

import (
	"flag"
	"fmt"
	"image"
	"image/color"
	"log"
	"runtime"
	"strings"
	"time"

	"github.com/golang-gui/goui/gui"
	"github.com/golang-gui/goui/layout"
	"github.com/golang-gui/goui/style"
	"github.com/golang-gui/goui/theme/modern"
	"github.com/golang-gui/goui/ui"
)

func main() {
	runtime.LockOSThread()
	mb := flag.Int("mb", 1, "approximate document size (1–10 MiB)")
	flag.Parse()
	line := "Repeated paragraph: editing, viewport, selection and undo. 中文测试。\n"
	initial := "Edit here: abc 中文 e\u0301 👩‍💻 ffi\nBidi: abc אבג def العربية xyz\n\n" + strings.Repeat(line, min(10, max(1, *mb))*1024*1024/len(line))
	model := ui.NewTextModel(initial)
	text := ui.MakeState("Search 中文 e\u0301 👩‍💻")
	searchSelection := ui.MakeState(ui.TextSelection{})
	notes := ui.MakeState("Notes: edit this small value-bound document.\n中文 e\u0301 👩‍💻")
	notesSelection := ui.MakeState(ui.TextSelection{})
	var rebuildTimer *ui.Timer
	dark, readonly, bare := false, false, false
	submits, rebuilds, accent := 0, 0, 0
	accents := []color.Color{nil, color.RGBA{R: 140, G: 70, B: 220, A: 255}, color.Black}
	var leftSelection, rightSelection ui.TextSelection
	if err := ui.Run("org.golang-gui.TextEditingUITest", func(app ui.App) ui.RootView {
		settings := app.Settings()
		var sheet style.StyleSheet
		if !bare {
			sheet = modern.Sheet(modern.Options{Dark: dark, AccentColor: accents[accent], FontFamily: settings.FontFamily(), FontSize: settings.FontSize()})
		}
		onSubmit := func() {
			submits++
			log.Printf("submit=%d", submits)
			app.RequestUpdate()
		}
		return ui.Root().StyleSheet(sheet).Windows(
			ui.Window("editing").Title("GOUI UI Text Editing").Chrome(ui.WindowChromeNative).Size(1080, 720).Content(
				ui.VBox(
					ui.HBox(
						ui.Button("Theme").OnClick(func() { dark = !dark; app.RequestUpdate() }),
						ui.Button("Accent").OnClick(func() { accent = (accent + 1) % len(accents); app.RequestUpdate() }),
						ui.Button("Bare GUI").OnClick(func() { bare = !bare; app.RequestUpdate() }),
						ui.Button("Rebuild").OnClick(func() { rebuilds++; app.RequestUpdate() }),
						ui.Button("Readonly").OnClick(func() { readonly = !readonly; app.RequestUpdate() }),
					).Spacing(8),
					ui.HBox(
						ui.Label("Search:"),
						ui.TextInput().Name("search").BindText(&text).BindSelection(&searchSelection).OnSubmit(onSubmit).ReadOnly(readonly).MainWeight(1),
						ui.Label("Uncontrolled:"),
						ui.TextInput().Name("uncontrolled").OnSubmit(onSubmit).MainWeight(1),
					).CrossAlign(layout.CrossBaseline).Spacing(8),
					ui.HBox(
						ui.Button("Replace notes").OnClick(func() {
							notes.Set("Replacement\n中文")
							notesSelection.Set(ui.TextSelection{})
						}),
						ui.Button("Select notes").OnClick(func() {
							notesSelection.Set(ui.TextSelection{Anchor: 12, Caret: len(notes.Get())})
						}),
						ui.Button("Empty notes").OnClick(func() {
							notes.Set("")
							notesSelection.Set(ui.TextSelection{})
						}),
						ui.Button("Auto rebuild").OnClick(func() {
							if rebuildTimer.Active() {
								rebuildTimer.Stop()
							} else if rebuildTimer != nil {
								rebuildTimer.Start(time.Second)
							} else {
								rebuildTimer = app.TickFunc(time.Second, func() { rebuilds++; app.RequestUpdate() })
							}
							app.RequestUpdate()
						}),
					).Spacing(8),
					ui.ScrollView(ui.TextView().Name("notes").BindText(&notes).
						BindSelection(&notesSelection).ReadOnly(readonly)).MinHeight(100).MaxHeight(100),
					ui.Label(fmt.Sprintf("Notes: %d bytes | selection %+v | Search %+v | auto rebuild %t", len(notes.Get()), notesSelection.Get(), searchSelection.Get(), rebuildTimer.Active())),
					ui.HBox(
						ui.ScrollView(ui.TextView().Name("left").Model(model).ReadOnly(readonly).
							OnChange(func(ui.TextChange) { app.RequestUpdate() }).
							OnSelection(func(s ui.TextSelection) { leftSelection = s; app.RequestUpdate() })),
						ui.ScrollView(ui.TextView().Name("right").Model(model).
							OnSelection(func(s ui.TextSelection) { rightSelection = s; app.RequestUpdate() })),
					).Spacing(12).CrossAlign(layout.CrossStretch).MainWeight(1),
					statusLabel(fmt.Sprintf("%d bytes | L %+v | R %+v | submits %d | rebuilds %d | readonly %t", model.Len(), leftSelection, rightSelection, submits, rebuilds, readonly)),
				).Padding(12).Spacing(10).CrossAlign(layout.CrossStretch),
			),
		)
	}); err != nil {
		log.Fatal(err)
	}
}

// A normal custom WidgetView: diagnostics use public drawing APIs, not private
// UI reconciliation state or an exported test-only production interface.
type statusView struct {
	ui.ViewBase[statusView]
	text string
}

func statusLabel(text string) *statusView {
	v := &statusView{text: text}
	v.Self = v
	return v
}

func (v *statusView) Build() ui.View { return v }
func (v *statusView) Mount(ui.BuildContext) gui.Widget {
	return &diagnosticLabel{Label: gui.NewLabel(v.text)}
}
func (v *statusView) Update(_ ui.BuildContext, w gui.Widget) {
	w.(*diagnosticLabel).SetText(v.text)
}
func (*statusView) Unmount(ui.BuildContext, gui.Widget) {}

type diagnosticLabel struct {
	*gui.Label
	reported bool
}

func (l *diagnosticLabel) Paint(p gui.Painter) {
	if !l.reported {
		l.reported = true
		img, err := p.NewImage(image.NewRGBA(image.Rect(0, 0, 1, 1)))
		if err != nil {
			log.Printf("painter probe failed: %v", err)
		} else {
			log.Printf("painter image=%T, pixel scale=%g", img, p.PixelScale())
			img.Destroy()
		}
	}
	l.Label.Paint(p)
}
