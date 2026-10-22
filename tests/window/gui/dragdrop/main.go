// GUI Drag and Drop 窗口验证；不使用 DevServer。
//
// 环境：Linux/X11、Windows 或 macOS 桌面；记录 OS、Linux WM、实际后端
// 和缩放。GOUI_PLAT_PAINTER=software/opengl，Windows 另可用 direct2d；
// GOUI_PLAT_SCALE=1/2。环境变量只是请求值，不代替实际后端确认。
// 启动：go run ./tests/window/gui/dragdrop -second-window -popover
// Windows PowerShell：$env:GOUI_PLAT_SCALE='2'; go run ./tests/window/gui/dragdrop
// 三个平台均默认显示自定义预览；-preview=false 验证无图时的原生回退。
// 可重复传入 -file /absolute/path 提供文件或目录；不指定时在临时目录创建
// 一个含中文和空格的样本。-large-text 提供 256 KiB 文本；-actions 为非文件源
// 开启 Copy/Move/Link，文件源始终只提供 Copy。
// 初始：主窗口上排为 Local、Text、Files、URLs、MIME 五个独立源，下排是
// 各自同名的接收区；另有 Reject 开关、Prepare/Begin/Drop/End/Click/FAIL
// 计数和最近结果。-second-window 增加相同目标，-popover 增加浮窗开关。
// 五种源各只提供一种格式；Files 和 URLs 在不同接收区验收。
//
// 操作与预期：
//  1. 单击任一源：只增加 Click。分别拖到同名接收区：每次 Prepare、
//     Begin、Drop、End 各增加一次，Click 不变，FAIL=0。Local 校验对象
//     身份；Text、URLs、MIME 校验预设内容；Files 校验源提供的路径。
//  2. 点击 Reject 后拖入同名目标：没有 Drop，End action=0；再次点击
//     恢复接收。拖动中按 Esc 或释放到无目标区域：不增加 Drop/Click，
//     End action=0；系统能区分取消时 canceled=true。随后仍可重新拖入。
//  3. 将第二窗口移到旁边，或点 Show/Hide Popover 显示浮窗；拖到其中的
//     同名目标，数据和计数与步骤 1 一致。隐藏浮窗查看完整状态，重复拖动
//     不应出现重复 Drop、未配对 Leave 或 FAIL。
//  4. 启动两个进程，把 Text、URLs、MIME 拖到另一进程同名目标：固定
//     内容显示 VERIFIED；Local 拖到另一进程不得被接收。两个进程都加
//     -large-text 后双向拖 Text：完整 256 KiB 内容应通过校验（X11 INCR）。
//  5. Files 拖到文件管理器的空目录：独立核对复制内容和源仍存在；从文件
//     管理器反向拖入多文件/目录，窗口显示数量，终端逐项打印绝对路径。
//     外部数据若未匹配预设内容只显示 RECEIVED，不能算作内容校验通过。
//     Files/URLs 分区是必要的：X11 两者均可能用 text/uri-list。
//  6. 默认预览应是蓝黄两色、透明外边缘、黑色十字热点：图像 48 DIP，
//     十字 (12,12) DIP 应始终位于指针位置；在 1x/2x 下核对逻辑尺寸、
//     热点和透明边缘，记录各后端采样差异。以 -preview=false 重启，
//     确认改用原生回退且拖放结果不变。-actions 下用系统修饰键请求
//     动作，核对 Drop 与 End 一致；不能把未报告的动作视为已通过。
//
// 平台差异：原生预览样式、修饰键动作及文件管理器行为按平台分别记录。
// 副作用：程序创建自有临时源文件，正常退出时删除；不修改 -file 指定
// 的文件，不自动打开文件/URL。确认文件管理器复制完成后再关闭程序；
// 文件管理器生成的副本由操作者清理。
// 复位：重启程序。退出：关闭任一主窗口；FAIL 非零时返回非零退出码。
package main

import (
	"flag"
	"fmt"
	"image"
	"image/color"
	"os"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/golang-gui/goui/core/geometry"
	"github.com/golang-gui/goui/gui"
	"github.com/golang-gui/goui/layout"
	"github.com/golang-gui/goui/theme/modern"
)

type fileFlags []string

func (f *fileFlags) String() string { return strings.Join(*f, ", ") }
func (f *fileFlags) Set(value string) error {
	path, err := filepath.Abs(value)
	if err != nil {
		return err
	}
	if _, err := os.Stat(path); err != nil {
		return err
	}
	*f = append(*f, path)
	return nil
}

func main() {
	runtime.LockOSThread()
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func run() error {
	var files fileFlags
	flag.Var(&files, "file", "file/directory to offer; repeatable; default creates a temporary sample")
	large := flag.Bool("large-text", false, "offer exactly 256 KiB UTF-8 text")
	preview := flag.Bool("preview", true, "show a 48 DIP image with a marked 12 DIP hotspot")
	second := flag.Bool("second-window", false, "open a second drop target window")
	popup := flag.Bool("popover", false, "enable the modeless Popover target toggle")
	actions := flag.Bool("actions", false, "allow Copy/Move/Link for non-file sources")
	flag.Parse()
	if len(files) == 0 {
		dir, err := os.MkdirTemp("", "goui-dnd-")
		if err != nil {
			return err
		}
		defer os.RemoveAll(dir)
		path := filepath.Join(dir, "GOUI 中文 sample.txt")
		if err := os.WriteFile(path, []byte("GOUI drag-and-drop sample\n中文 / spaces / 100%\n"), 0600); err != nil {
			return err
		}
		files = append(files, path)
	}
	fmt.Printf("source files=%q; OS=%s; requested painter=%q scale=%q; custom preview=%t\n", files, runtime.GOOS, os.Getenv("GOUI_PLAT_PAINTER"), os.Getenv("GOUI_PLAT_SCALE"), *preview)
	app, err := gui.NewApplication("org.golang-gui.DragDropGUIProbe")
	if err != nil {
		return err
	}
	app.SetStyleSheet(modern.Sheet(modern.Options{}))
	win, err := app.NewWindow(&gui.WindowOptions{Size: geometry.Size{Width: 780, Height: 540}})
	if err != nil {
		return err
	}
	defer win.Destroy()
	if err := win.SetTitle("GOUI GUI DragDrop acceptance"); err != nil {
		return err
	}
	p := newProbe(*large)
	p.files = append([]string(nil), files...)
	root := gui.NewLinearBox(layout.DirectionVertical)
	root.SetPadding(16)
	root.SetSpacing(10)
	root.SetCrossAlign(layout.CrossStretch)
	root.AddChild(gui.NewLabel("Five independent formats. Local stays in this application."))
	sources := gui.NewLinearBox(layout.DirectionHorizontal)
	sources.SetSpacing(8)
	for _, name := range []string{"Local", "Text", "Files", "URLs", "MIME"} {
		button := gui.NewButton()
		button.SetID("source-" + strings.ToLower(name))
		button.SetChild(gui.NewLabel(name))
		button.SetMainWeight(1)
		button.SetMinSize(geometry.Size{Width: 90, Height: 90})
		button.ConnectClicked(func() { p.clicks++; p.report("Click %s", name) })
		source := gui.NewDragSource()
		if *actions && name != "Files" {
			source.SetActions(gui.DragCopy | gui.DragMove | gui.DragLink)
		}
		source.ConnectPrepare(func(e *gui.DragPrepare) {
			p.check(!p.active, "overlapping source session")
			p.active, p.begun = true, false
			p.prepares++
			p.clickStart, p.dropStart = p.clicks, p.drops
			p.dropAction = 0
			data := new(gui.DragData)
			switch name {
			case "Local":
				data.SetLocal("probe", p.local)
			case "Text":
				data.SetText(p.text)
			case "Files":
				data.SetFiles(files)
			case "URLs":
				data.SetURLs(probeURLs)
			case "MIME":
				data.SetBytes(probeMIME, probeBytes)
			}
			e.Data = data
			if *preview {
				e.Preview = gui.DragPreview{Image: previewImage(), Scale: 1,
					Hotspot: geometry.Point{X: 12, Y: 12}}
			}
			p.report("Prepare %s", name)
		})
		source.ConnectBegin(func() {
			p.check(p.active && !p.begun, "unexpected Begin")
			p.begun = true
			p.begins++
			p.report("Begin %s", name)
		})
		source.ConnectEnd(func(result gui.DragResult) {
			p.check(p.active, "duplicate End")
			p.check(p.begun || result.Err != nil, "End without Begin or startup error")
			p.check(p.clicks == p.clickStart, "drag also triggered Click")
			p.check(p.drops-p.dropStart <= 1, "one source generated multiple Drops")
			if p.dropAction != 0 {
				p.check(result.Action == p.dropAction, "Drop and End actions differ")
			}
			if result.Err != nil {
				p.check(false, result.Err.Error())
			}
			p.active = false
			p.ends++
			p.report("End %s: action=%d canceled=%t error=%v", name, result.Action, result.Canceled, result.Err)
		})
		button.AddEventController(source)
		sources.AddChild(button)
	}
	root.AddChild(sources)
	root.AddChild(p.targets("main"))
	reject := gui.NewButton()
	rejectLabel := gui.NewLabel("Reject: OFF (click to toggle)")
	reject.SetChild(rejectLabel)
	reject.ConnectClicked(func() {
		p.reject = !p.reject
		rejectLabel.SetText(fmt.Sprintf("Reject: %t (click to toggle)", p.reject))
		p.report("Reject=%t", p.reject)
	})
	toolbar := gui.NewLinearBox(layout.DirectionHorizontal)
	toolbar.SetSpacing(8)
	toolbar.AddChild(reject)
	if *popup {
		floating := gui.NewPopover(sources, nil)
		defer floating.Destroy()
		floating.SetPosition(geometry.Point{Y: 240})
		floating.SetWidget(p.targets("popover"))
		toggle := gui.NewButton()
		toggle.SetChild(gui.NewLabel("Show/Hide Popover"))
		toggle.ConnectClicked(func() {
			if floating.Visible() {
				floating.Hide()
				return
			}
			if err := floating.Show(); err != nil {
				p.check(false, err.Error())
			}
		})
		toolbar.AddChild(toggle)
	}
	root.AddChild(toolbar)
	root.AddChild(p.counts)
	root.AddChild(p.status)
	win.SetWidget(root)
	win.ConnectCloseRequest(func(*bool) { app.Quit() })
	if err := win.Show(); err != nil {
		return err
	}
	if *second {
		other, err := app.NewWindow(&gui.WindowOptions{Size: geometry.Size{Width: 780, Height: 220}})
		if err != nil {
			return err
		}
		defer other.Destroy()
		if err := other.SetTitle("GOUI second DropTarget"); err != nil {
			return err
		}
		other.SetWidget(p.targets("second"))
		other.ConnectCloseRequest(func(*bool) { app.Quit() })
		if err := other.Show(); err != nil {
			return err
		}
	}
	app.Run()
	if p.failures != 0 {
		return fmt.Errorf("DnD probe recorded %d failures", p.failures)
	}
	return nil
}

func previewImage() image.Image {
	img := image.NewNRGBA(image.Rect(0, 0, 48, 48))
	for y := 4; y < 44; y++ {
		for x := 4; x < 44; x++ {
			pixel := color.NRGBA{R: 20, G: 100, B: 230, A: 230}
			if x >= 24 {
				pixel = color.NRGBA{R: 255, G: 205, B: 20, A: 230}
			}
			if (x == 12 && y >= 8 && y <= 16) || (y == 12 && x >= 8 && x <= 16) {
				pixel = color.NRGBA{A: 255}
			}
			img.SetNRGBA(x, y, pixel)
		}
	}
	return img
}
