// 声明式 Drag and Drop 窗口验证；不使用 DevServer。
//
// 环境：Linux/X11、Windows 或 macOS 桌面；记录 OS、Linux WM、实际后端
// 和缩放。GOUI_PLAT_PAINTER=software/opengl，Windows 另可用 direct2d；
// GOUI_PLAT_SCALE=1/2。ui.Run 在 GUI 主线程执行。
// 启动：go run ./tests/window/ui/dragdrop
// Windows PowerShell：$env:GOUI_PLAT_SCALE='2'; go run ./tests/window/ui/dragdrop
// 默认使用 Data+OnPrepare 提供自定义预览；-preview=false 使用纯 Data
// 声明，不注册 OnPrepare，并验证无图时的原生回退。
// -widget-preview 使用 ui.App.FindWidget 按 ID 找到源控件，再由
// gui.RenderWidget 重绘生成预览；仅在 -preview=true 时生效。
// 初始：760×480 窗口，上方 Drag source、下方 Drop target；Rebuild、
// Source Enabled、Bindings 按钮均可用。Revision=0，各事件计数和 FAIL=0，
// Last drop 为 none；Source Enabled 与 Bindings 均为 true。
//
// 操作与预期：
//  1. 单击源：仅 Click+1。默认模式拖到目标时，Data 预置文本在
//     Prepare 中可见，Prepare/Begin/Drop/End 各+1，
//     Click 不变、FAIL=0；Last drop 显示当前 revision 的文本，End
//     不会覆盖它。重复一次，各信号仍只增加一次。
//  2. 点 Rebuild 后拖两次：Revision+1，目标每次收到新 revision 的文本；
//     源/目标旧回调不得触发，计数不得重复。
//  3. 点 Source Enabled 使其为 false：源仍可单击，但拖动不增加
//     Prepare/Begin/Drop/End；恢复 true 后又能拖入目标。
//  4. 点 Bindings 使其为 false：源和目标的拖放控制器都移除，拖动
//     不增加计数；恢复 true 后每次拖动仍只触发一次。
//  5. 拖到窗口外无接收者区域，或拖动中按 Esc：没有 Drop 或误 Click，
//     End action=0；平台能区分取消时 canceled=true。再次拖入目标成功。
//  6. 默认预览应为 48 DIP 蓝黄图，外边缘透明、黑色十字热点位于
//     (12,12) DIP 的指针位置；Rebuild 前后均应显示。缩放窗口并在
//     1x/2x 下重做步骤 1–5：逻辑尺寸、热点、透明边缘、命中和计数
//     保持正确。以 -preview=false 重启，应改用原生回退，Prepare
//     计数保持 0，Begin/Drop/End 正常增加且目标收到相同数据。
//  7. 以 -widget-preview 重启，拖动时预览应是当前按钮的重绘图像；
//     1x/2x 的尺寸与 12 DIP 热点一致。Rebuild 后重复拖动，仍使用
//     当前控件；找不到 ID、重绘失败或缩放无效会增加 FAIL。
//
// 平台差异：原生取消标志可观察性不同；窗口布局与后端分别记录。
// 本例验收 UI 协调；原生文件和跨进程格式见 gui/dragdrop 与
// platform/dragdrop 的独立窗口用例。
// 副作用：无文件、剪贴板或系统设置修改。
// 复位：重新启动程序。退出：关闭窗口；FAIL 非零时返回非零退出码。
package main

import (
	"flag"
	"fmt"
	"image"
	"image/color"
	"log"
	"os"

	"github.com/golang-gui/goui/core/geometry"
	"github.com/golang-gui/goui/gui"
	"github.com/golang-gui/goui/layout"
	"github.com/golang-gui/goui/theme/modern"
	"github.com/golang-gui/goui/ui"
)

func main() {
	preview := flag.Bool("preview", true, "show a 48 DIP image with a marked 12 DIP hotspot")
	widgetPreview := flag.Bool("widget-preview", false, "render the current source widget by ID for its preview")
	flag.Parse()
	clicks, drops, builds, prepares, begins, ends, failures := 0, 0, 0, 0, 0, 0, 0
	enabled, bindings, active := true, true, false
	clickStart, dropStart := 0, 0
	status, lastDrop := "Drag the upper button to the lower one", "Last drop: none"
	check := func(ok bool, reason string) {
		if !ok {
			failures++
			status = "FAIL: " + reason
			fmt.Println(status)
		}
	}
	err := ui.Run("org.golang-gui.UIDragDropProbe", func(app ui.App) ui.RootView {
		generation := builds
		var source *ui.DragSourceView
		var target *ui.DropTargetView
		if bindings {
			data := new(ui.DragData)
			data.SetText(fmt.Sprintf("ui-drag-value/%d", generation))
			source = ui.DragSource().Data(data).Enabled(enabled)
			if *preview {
				source.OnPrepare(func(request *ui.DragPrepare) {
					check(generation == builds, "stale source Prepare callback")
					check(!active, "Prepare while another drag is active")
					prepares++
					value, ok := request.Data.Text()
					check(ok && value == fmt.Sprintf("ui-drag-value/%d", generation), "declared data missing during Prepare")
					if *widgetPreview {
						widget := app.FindWidget("dragdrop", "source")
						check(widget != nil, "source ID lookup: widget not found")
						if widget != nil {
							img, renderErr := gui.RenderWidget(widget, 0)
							check(renderErr == nil, fmt.Sprintf("source preview render: %v", renderErr))
							if renderErr == nil {
								request.Preview.Image = img
								if width := widget.Rect().Width; width > 0 {
									request.Preview.Scale = float32(img.Bounds().Dx()) / width
								}
								check(request.Preview.Scale > 0, "source preview scale")
							}
						}
					} else {
						request.Preview.Image = previewImage()
						request.Preview.Scale = 1
					}
					request.Preview.Hotspot = geometry.Point{X: 12, Y: 12}
					status = fmt.Sprintf("Prepare revision=%d", generation)
					fmt.Println(status)
					app.RequestUpdate()
				})
			}
			source.OnBegin(func() {
				check(!active, "overlapping source Begin")
				active = true
				clickStart, dropStart = clicks, drops
				begins++
				status = "Begin"
				fmt.Println(status)
				app.RequestUpdate()
			}).OnEnd(func(result ui.DragResult) {
				started := active
				check(started || result.Err != nil, "End without Begin or startup error")
				if started {
					check(clicks == clickStart, "drag also triggered Click")
					check(drops-dropStart <= 1, "multiple Drops from one source")
				}
				if started && drops > dropStart {
					check(result.Action == ui.DragCopy, "accepted Drop did not finish Copy")
				}
				check(result.Err == nil, fmt.Sprintf("source error: %v", result.Err))
				active = false
				ends++
				status = fmt.Sprintf("End action=%d canceled=%t error=%v", result.Action, result.Canceled, result.Err)
				fmt.Println(status)
				app.RequestUpdate()
			})
			target = ui.DropTarget(ui.DragFormatText).OnDrop(func(e *ui.DropRequest) {
				check(generation == builds, "stale target Drop callback")
				value, ok := e.Data.Text()
				expected := fmt.Sprintf("ui-drag-value/%d", builds)
				e.Accepted = ok && value == expected
				check(e.Accepted, fmt.Sprintf("text=%q expected=%q", value, expected))
				if e.Accepted {
					drops++
					lastDrop = fmt.Sprintf("Last drop: %q, total=%d", value, drops)
					status = "VERIFIED: current source and target callbacks"
					fmt.Println(lastDrop)
				}
				app.RequestUpdate()
			}).OnError(func(err error) { check(false, "target error: "+err.Error()); app.RequestUpdate() })
		}
		return ui.Root().StyleSheet(modern.Sheet(modern.Options{})).Windows(
			ui.Window("dragdrop").Title("GOUI UI DragDrop acceptance").Size(760, 480).Content(
				ui.VBox(
					ui.Label(fmt.Sprintf("Revision=%d | Prepare=%d Begin=%d Drop=%d End=%d Click=%d FAIL=%d",
						builds, prepares, begins, drops, ends, clicks, failures)),
					ui.Button("Drag source").ID("source").MinSize(400, 100).MainWeight(1).
						DragSource(source).OnClick(func() {
						clicks++
						fmt.Printf("Click count=%d\n", clicks)
						app.RequestUpdate()
					}),
					ui.Button("Drop target").ID("target").MinSize(400, 100).MainWeight(1).DropTarget(target),
					ui.HBox(
						ui.Button("Rebuild").OnClick(func() { builds++; fmt.Printf("Rebuild revision=%d\n", builds); app.RequestUpdate() }),
						ui.Button(fmt.Sprintf("Source Enabled: %t", enabled)).OnClick(func() { enabled = !enabled; app.RequestUpdate() }),
						ui.Button(fmt.Sprintf("Bindings: %t", bindings)).OnClick(func() { bindings = !bindings; app.RequestUpdate() }),
					).Spacing(8),
					ui.Label(status), ui.Label(lastDrop),
				).Spacing(12).Padding(16).CrossAlign(layout.CrossStretch),
			),
		)
	})
	if err != nil {
		log.Fatal(err)
	}
	if failures != 0 {
		fmt.Fprintf(os.Stderr, "%d UI dragdrop failures\n", failures)
		os.Exit(1)
	}
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
