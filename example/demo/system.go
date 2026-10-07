package main

import (
	"fmt"
	"strings"
	"time"

	"github.com/golang-gui/goui/gui"
	"github.com/golang-gui/goui/ui"
)

func (d *demo) systemPage() ui.View {
	textData := new(gui.DragData)
	textData.SetText(d.input.Get())
	localData := new(gui.DragData)
	localData.SetLocal("demo-card", "GOUI 应用内卡片")
	filesData := new(gui.DragData)
	filesData.SetFiles(d.files)
	source := func(id string, data *gui.DragData) *ui.DragSourceView {
		return ui.DragSource().Data(data).OnPrepare(func(r *ui.DragPrepare) {
			w := d.app.FindWidget("main", id)
			if w == nil {
				r.Data = nil
				return
			}
			img, err := gui.RenderWidget(w, 0)
			if err != nil {
				r.Data = nil
				d.result("生成拖拽预览", err)
				return
			}
			r.Preview = gui.DragPreview{Image: img, Scale: float32(img.Bounds().Dx()) / w.Rect().Width, Hotspot: r.Position}
		}).OnEnd(func(r ui.DragResult) {
			if r.Err != nil {
				d.result("拖拽", r.Err)
			} else if r.Canceled {
				d.report("拖拽已取消")
			} else {
				d.report(fmt.Sprintf("拖拽结束，操作=%d（0 表示未接受）", r.Action))
			}
		})
	}
	target := ui.DropTarget(ui.LocalFormat("demo-card"), ui.DragFormatFiles, ui.DragFormatText).OnDrop(func(r *ui.DropRequest) {
		if paths, ok := r.Data.Files(); ok {
			d.dropped = "收到文件路径（未读取文件）：\n" + strings.Join(paths, "\n")
		} else if text, ok := r.Data.Text(); ok {
			d.dropped = "收到文本：\n" + text
		} else if value, ok := r.Data.Local("demo-card"); ok {
			d.dropped = fmt.Sprint(value)
		} else {
			return
		}
		r.Accepted = true
		d.report("放下已接受")
	}).OnError(func(e error) { d.result("接收拖放", e) })
	return page("系统与拖放", "以下功能只在点击时执行。文件对话框仅选择路径；拖入不读文件，拖出只提供用户已选择的真实文件。",
		section("剪贴板", ui.TextInput().BindText(&d.input), ui.HWrap(ui.Button("复制文本").ID("copy-text").OnClick(func() {
			d.app.Clipboard().SetText(d.input.Get())
			d.report("已请求复制（剪贴板为尽力而为接口）")
		}),
			ui.Button("读取剪贴板").ID("read-clipboard").OnClick(func() {
				d.app.Clipboard().RequestText(func(text string, ok bool) {
					if ok {
						d.input.Set(text)
						d.report("已读取剪贴板文本")
					} else {
						d.report("剪贴板无文本或不可用")
					}
				})
			})).Spacing(12)),
		section("原生对话框与打开请求", ui.HWrap(ui.Button("选择文件").ID("choose-files").OnClick(func() {
			d.app.FileDialog().OpenFile(ui.DialogOptions{Title: "选择用于拖出的文件", AllowMultiple: true}, d.selectedPaths)
		}),
			ui.Button("选择目录").OnClick(func() {
				d.app.FileDialog().OpenDirectory(ui.DialogOptions{Title: "选择目录（不读取）"}, d.selectedPaths)
			}),
			ui.Button("保存对话框（不写入）").OnClick(func() {
				d.app.FileDialog().SaveFile(ui.DialogOptions{Title: "只展示保存路径，不写文件", DefaultName: "demo.txt"}, func(paths []string) {
					if len(paths) == 0 {
						d.report("保存对话框无返回路径（取消或不可用）")
					} else {
						d.report("选择保存路径，未写入：" + paths[0])
					}
				})
			}),
			ui.Button("打开项目网站").OnClick(func() { d.result("打开浏览器", d.app.OpenURL("https://github.com/golang-gui/goui")) }),
			ui.Button("默认应用打开选中项").Enabled(len(d.files) > 0).OnClick(func() {
				if len(d.files) > 0 {
					d.result("打开路径", d.app.OpenPath(d.files[0]))
				}
			})).Spacing(10).LineSpacing(10)),
		section("计时器：GUI 主线程回调", ui.HBox(ui.Button("开始 / 停止计数").ID("timer-toggle").OnClick(func() {
			if d.tickTimer != nil {
				d.tickTimer.Stop()
				d.tickTimer = nil
				d.report("计时器已停止")
			} else {
				d.tickTimer = d.app.TickFunc(time.Second, func() { d.ticks++; d.app.RequestUpdate() })
				d.report("计时器已启动，离开此页自动停止")
			}
		}), ui.Label(fmt.Sprintf("Tick：%d", d.ticks))).Spacing(12)),
		section("App 内与跨 App 拖放",
			ui.HWrap(ui.Button("拖动文本 →").ID("drag-text").DragSource(source("drag-text", textData)),
				ui.Button("应用内卡片 →").ID("drag-local").DragSource(source("drag-local", localData)),
				ui.Button("选中的文件 →").ID("drag-files").Enabled(len(d.files) > 0).DragSource(source("drag-files", filesData))).Spacing(12).LineSpacing(8),
			surface(ui.HBox(ui.Label(d.dropped).WrapMode(ui.WrapWordChar).MainWeight(1)).Padding(16)).Style("demo-card").ID("drop-zone").MinHeight(110).DropTarget(target),
			ui.Label("文本可拖到其他应用的编辑区；真实文件可拖到文件管理器。应用内卡片只在本应用有效。拖放可按 Esc 取消。")),
	)
}
func (d *demo) selectedPaths(paths []string) {
	if len(paths) == 0 {
		d.report("对话框无返回路径（取消或不可用）")
		return
	}
	d.files = append([]string(nil), paths...)
	d.report("选择路径：" + strings.Join(paths, ", "))
}
