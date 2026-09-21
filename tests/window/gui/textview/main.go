// TextView 窗口验证（C 切片，逐项记录实际通过与未执行的场景）。
//
// 环境：桌面会话、原生字体库；记录 OS、绘制后端与 1x/2x 缩放。
// 启动：go run ./tests/window/gui/textview -mb 1（或 -mb 10）。
// 锚点验证加 -numbered：段落首尾带相同编号，避免重复文本掩盖视口跳动。
// 可用 GOUI_PLAT_PAINTER=software/opengl、GOUI_PLAT_SCALE=1/2 分别运行。
// Windows PowerShell 示例：$env:GOUI_PLAT_PAINTER='direct2d'; $env:GOUI_PLAT_SCALE='1';
// go run ./tests/window/gui/textview -mb 10 -numbered
// macOS/Linux：GOUI_PLAT_PAINTER=opengl GOUI_PLAT_SCALE=2 go run ./tests/window/gui/textview -mb 10 -numbered
// Windows 重复 direct2d/opengl/software，macOS/Linux 重复 opengl/software。
// direct2d 是 Windows 默认优先路径，初始化失败可能降级，须按首帧日志记录实际后端。
// 环境变量在当前 shell 生效；测试后恢复此前值。强制 2x 不替代真实跨屏 DPI 验证。
// macOS 剪贴板/撤销/全选用 Cmd，按词导航用 Option，文档首尾用 Cmd+Home/End；
// 没有 Home/End 的键盘可用 Fn+Left/Right 组合。其他平台对应 Ctrl/Ctrl/Home/End。
// 首次绘制日志报告实际图像资源类型及 PixelScale；环境变量只是偏好，不能代替
// 实际后端确认。OpenGL 可能由 Mesa 软件驱动实现，需同时记录 glxinfo 等原生信息。
// Linux 隔离桌面无外部输入法时，用 XMODIFIERS=@im=none 选择 libX11 本地输入法。
// 不要继承指向不存在服务的 @im=fcitx/ibus 后，把没有文字提交当作普通键入验收。
// 验证中文组合/候选窗须启动同一桌面的真实输入法，并使用其 XMODIFIERS。
// 初始：两个独立 ScrollView/TextView 共享一个模型，左侧光标在文档开头。
//
// 操作与预期：
//  1. 在左侧输入 ASCII、粘贴中文/组合字符/emoji，右侧同时显示新内容；
//     两侧光标和滚动互不跟随，状态栏报告字节数和各自的选择范围。
//  2. 单击、Shift+方向键、拖选和双击单词；选择背景随 Cluster，不能切开
//     一个 Cluster。在双向示例行选择部分文字，允许出现不连续高亮。
//  3. 在长、短、长三行间连续上下移动，期望横向位置不因短行丢失。
//     Home/End 定位显示行，Ctrl+Home/End（macOS 为 Cmd）定位文档。
//  4. Enter 分行，行尾 Delete/行首 Backspace 合并两行；Ctrl/Cmd+Z 撤销，
//     Ctrl/Cmd+Shift+Z 重做；连续输入一组，换编辑视图后独立成组。
//  5. 点“左侧只读”后左侧仍能导航/选择/复制，但不能编辑或粘贴；右侧可编辑。
//  6. 滚轮浏览远处内容，光标不能把视口拉回；键盘导航时光标应滚入可见区域。
//     全选并滚到末页，检查视口外的状态栏和工具栏：不得出现选区背景或文字溢出；
//     Software 的 1x/2x 均需检查，不能只看光标所在的可见行。
//     点“换行”并缩放窗口，检查文字和选区裁剪、水平/垂直滚动条；重排后顶部
//     仍显示原来的文字位置（新显示行可包含其前面的文字），末页不持续跳动。
//     -numbered 下记录顶部可见段落编号/词，再拖窄、拖宽窗口；对应文字应仍在
//     顶部显示行内，而不是跳到另一个编号。左右分别滚到不同编号再测试。
//     最后恢复原宽度，不进行滚动或编辑，顶部文字及被裁去的高度应恢复原状。
//     新宽度可以把锚定词与其前面的词合并到一行，但不得出现度量与绘制的换行差异。
//     再 Ctrl/Cmd+Home、Ctrl/Cmd+Shift+End 选择到文档末尾：必须实际显示最后
//     编号及末尾空行光标，不能仅凭状态栏选区字节数正确认定导航成功。
//  7. 激活窗口并点左侧文字，静置后光标约每 500 ms 切换；切到其他应用后不闪烁，
//     返回后恢复。拖出左侧文本视口并停留，选区应自动延伸，右侧不滚动；释放停止。
//  8. 用中文输入法替换左侧选区，组合时右侧和字节数不变；提交后两侧同步，撤销
//     一次恢复原文。Esc/切换焦点取消组合，不删除原选区。Windows/macOS 检查
//     内联预编辑及候选位置；Linux 查询服务端能力，支持 XIM callbacks 时内联，
//     否则由输入法显示组合。Fcitx 4 测试需记录 UseOnTheSpotStyle 的实际值。
//     另测连续输入 a 后拼音提交“你”：首次撤销应仅移除“你”，再次撤销移除 a；
//     不能以替换非空选区的撤销通过代替本项。候选窗应在光标附近，而非窗口底部。
//     Linux callback 样式需记录 libX11 版本：1.8.2 前的上游版本拒绝该样式的 spot，
//     候选定位属于原生扩展限制；不要把 root-style 的通过记为 callback 定位通过。
//  9. 点“复位”恢复初始模型，撤销历史清空。关闭仍在闪烁/组合的窗口应正常退出。
//  10. Linux XIM 服务退出回归（只能在独立桌面/输入法会话执行，勿停止用户的输入法）：
//     选中 Edi，开始内联组合但不提交，然后从该隔离会话停止输入法。预编辑应消失，
//     原选区 Edi 和共享模型不变；复制选区/全文核对，方向键和关闭窗口仍能使用。
//     此时文字输入不可用，当前没有自动重连。确认窗口进程退出后才清理会话。
//     不将此项等同于“服务恰在同步 Reset 等调用内部退出”：后者仍受 libX11
//     原生同步等待限制，详见 DesignIME。故障注入可能令测试进程阻塞，需单独记录。
//  11. 默认启动时 Tab 不插入文字（本用例不承诺焦点遍历）。重启加 -accept-tab，
//     点击左侧正文并定位行中后按 Tab，应插入一个 U+0009、字节数加一；右侧同步。
//     Ctrl/Cmd+Z 撤销该插入；右侧仍使用默认设置，按 Tab 不插入。左侧只读时
//     Tab 也不能修改文档。两个模式须分开记录，不能用默认不消费代替可选插入。
//
// modern 主题及 TextInput/UI 集成见 tests/window/ui/textediting；各平台分别验收。
// 副作用：仅主动复制/剪切时改写系统剪贴板，无文件写入或系统设置修改。
// 复位：点“复位”或重新启动。退出：关闭窗口。无 DevServer 依赖。
package main

import (
	"flag"
	"fmt"
	"image"
	"log"
	"runtime"
	"strings"

	"github.com/golang-gui/goui/core/geometry"
	"github.com/golang-gui/goui/core/signal"
	"github.com/golang-gui/goui/gui"
	"github.com/golang-gui/goui/layout"
)

func main() {
	runtime.LockOSThread()
	megabytes := flag.Int("mb", 1, "approximate document size (1–10 MiB)")
	numbered := flag.Bool("numbered", false, "label paragraphs for viewport anchoring checks")
	acceptTab := flag.Bool("accept-tab", false, "allow Tab insertion in the left editor only")
	flag.Parse()
	if err := run(min(10, max(1, *megabytes)), *numbered, *acceptTab); err != nil {
		log.Fatal(err)
	}
}

func run(megabytes int, numbered, acceptTab bool) error {
	app, err := gui.NewApplication("org.golang-gui.TextViewTest")
	if err != nil {
		return err
	}
	win, err := app.NewWindow(&gui.WindowOptions{Size: geometry.Size{Width: 1050, Height: 700}, Chrome: gui.WindowChromeNative})
	if err != nil {
		return err
	}
	defer win.Destroy()
	if err = win.SetTitle("GOUI TextView — shared model, independent views"); err != nil {
		return err
	}
	header := "Edit here: abc 中文 e\u0301 👩‍💻 ffi\nBidi: abc אבג def العربية xyz\nA long navigation line\nx\nA long navigation line\n\n"
	line := "Repeated paragraph: text editing, viewport layout, selection and undo. 中文测试。\n"
	var initial string
	if numbered {
		var text strings.Builder
		text.Grow(megabytes * 1024 * 1024)
		text.WriteString(header)
		for row := 0; text.Len() < megabytes*1024*1024; row++ {
			fmt.Fprintf(&text, "[%06d:start] text editing, viewport layout; [%06d:middle] selection and undo; [%06d:end] 中文测试。\n", row, row, row)
		}
		initial = text.String()
	} else {
		initial = header + strings.Repeat(line, megabytes*1024*1024/len(line))
	}
	model := gui.NewTextModel(initial)
	left, right := gui.NewTextView(), gui.NewTextView()
	left.SetModel(model)
	right.SetModel(model)
	left.SetAcceptsTab(acceptTab)
	left.SetID("editor-left")
	right.SetID("editor-right")
	scrolls := gui.NewLinearBox(layout.DirectionHorizontal)
	scrolls.SetMainWeight(1)
	scrolls.SetCrossAlign(layout.CrossStretch)
	scrolls.SetSpacing(12)
	for _, editor := range []*gui.TextView{left, right} {
		scroll := gui.NewScrollView()
		scroll.SetChild(editor)
		scroll.SetMainWeight(1)
		scrolls.AddChild(scroll)
	}
	toolbar := gui.NewLinearBox(layout.DirectionHorizontal)
	toolbar.SetSpacing(8)
	var update func()
	button := func(text string, fn func()) {
		b := gui.NewButton()
		b.SetChild(gui.NewLabel(text))
		b.ConnectClicked(fn)
		toolbar.AddChild(b)
	}
	button("左侧只读", func() { left.SetReadOnly(!left.ReadOnly()); update() })
	button("换行", func() {
		mode := gui.WrapNone
		if left.WrapMode() == gui.WrapNone {
			mode = gui.WrapWordChar
		}
		left.SetWrapMode(mode)
		right.SetWrapMode(mode)
	})
	button("复位", func() { model.SetText(initial) })
	status := &diagnosticLabel{Label: gui.NewLabel("")}
	update = func() {
		status.SetText(fmt.Sprintf("%d bytes / %d lines | L %+v | R %+v | left readonly %t", model.Len(), model.LineCount(), left.Selection(), right.Selection(), left.ReadOnly()))
	}
	connections := signal.Handles{
		model.ConnectChange(func(gui.TextChange) { update() }),
		left.ConnectSelection(func(gui.TextSelection) { update() }),
		right.ConnectSelection(func(gui.TextSelection) { update() }),
	}
	defer connections.Disconnect()
	root := gui.NewLinearBox(layout.DirectionVertical)
	root.SetPadding(12)
	root.SetSpacing(8)
	root.SetCrossAlign(layout.CrossStretch)
	root.AddChild(toolbar)
	root.AddChild(scrolls)
	root.AddChild(status)
	win.SetWidget(root)
	update()
	win.SetFocusedWidget(left)
	if err = win.Show(); err != nil {
		return err
	}
	app.Run()
	return nil
}

// Diagnostic only: observe the current painter through its public image API,
// without exporting GUI internals or changing platform fallback policy.
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
