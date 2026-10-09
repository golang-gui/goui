package main

import (
	"fmt"
	"strings"
	"time"

	"github.com/golang-gui/goui/example/demo/resource"
	"github.com/golang-gui/goui/gui"
	"github.com/golang-gui/goui/theme/modern"
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
				d.result(resource.DragPreview(), err)
				return
			}
			r.Preview = gui.DragPreview{Image: img, Scale: float32(img.Bounds().Dx()) / w.Rect().Width, Hotspot: r.Position}
		}).OnEnd(func(r ui.DragResult) {
			if r.Err != nil {
				d.result(resource.Drag(), r.Err)
			} else if r.Canceled {
				d.report(resource.DragCanceled())
			} else {
				d.report(fmt.Sprintf(resource.DragEnded(), r.Action))
			}
		})
	}
	target := ui.DropTarget(ui.LocalFormat("demo-card"), ui.DragFormatFiles, ui.DragFormatText).OnDrop(func(r *ui.DropRequest) {
		if paths, ok := r.Data.Files(); ok {
			d.dropped = fmt.Sprintf(resource.FilesReceived(), strings.Join(paths, "\n"))
		} else if text, ok := r.Data.Text(); ok {
			d.dropped = fmt.Sprintf(resource.TextReceived(), text)
		} else if value, ok := r.Data.Local("demo-card"); ok {
			d.dropped = fmt.Sprint(value)
		} else {
			return
		}
		r.Accepted = true
		d.report(resource.DropAccepted())
	}).OnError(func(e error) { d.result(resource.ReceiveDrop(), e) })
	return page("system", resource.System(), resource.SystemHint(),
		section(resource.Clipboard(), ui.TextInput().BindText(&d.input), ui.HWrap(ui.Button(resource.CopyText()).ID("copy-text").OnClick(func() {
			d.app.Clipboard().SetText(d.input.Get())
			d.report(resource.CopyRequested())
		}),
			ui.Button(resource.ReadClipboard()).ID("read-clipboard").OnClick(func() {
				d.app.Clipboard().RequestText(func(text string, ok bool) {
					if ok {
						d.input.Set(text)
						d.report(resource.ClipboardRead())
					} else {
						d.report(resource.ClipboardEmpty())
					}
				})
			})).Spacing(12)),
		section(resource.DialogsTitle(), ui.HWrap(ui.Button(resource.ChooseFile()).ID("choose-files").OnClick(func() {
			d.app.FileDialog().OpenFile(ui.DialogOptions{Title: resource.ChooseDragFiles(), AllowMultiple: true}, d.selectedPaths)
		}),
			ui.Button(resource.ChooseFolder()).OnClick(func() {
				d.app.FileDialog().OpenDirectory(ui.DialogOptions{Title: resource.ChooseFolderTitle()}, d.selectedPaths)
			}),
			ui.Button(resource.SaveDialog()).OnClick(func() {
				d.app.FileDialog().SaveFile(ui.DialogOptions{Title: resource.SaveDialogTitle(), DefaultName: "demo.txt"}, func(paths []string) {
					if len(paths) == 0 {
						d.report(resource.NoSavePath())
					} else {
						d.report(fmt.Sprintf(resource.SavePath(), paths[0]))
					}
				})
			}),
			ui.Button(resource.OpenWebsite()).OnClick(func() { d.result(resource.OpenBrowser(), d.app.OpenURL("https://github.com/golang-gui/goui")) }),
			ui.Button(resource.OpenSelected()).Enabled(len(d.files) > 0).OnClick(func() {
				if len(d.files) > 0 {
					d.result(resource.OpenPath(), d.app.OpenPath(d.files[0]))
				}
			})).Spacing(10).LineSpacing(10)),
		section(resource.TimerTitle(), ui.HBox(ui.Button(resource.TimerToggle()).ID("timer-toggle").OnClick(func() {
			if d.tickTimer != nil {
				d.tickTimer.Stop()
				d.tickTimer = nil
				d.report(resource.TimerStopped())
			} else {
				d.tickTimer = d.app.TickFunc(time.Second, func() { d.ticks++; d.app.RequestUpdate() })
				d.report(resource.TimerStarted())
			}
		}), ui.Label(fmt.Sprintf("Tick：%d", d.ticks))).Spacing(12)),
		section(resource.DragDropTitle(),
			ui.HWrap(ui.Button(resource.DragText()).ID("drag-text").DragSource(source("drag-text", textData)),
				ui.Button(resource.DragLocal()).ID("drag-local").DragSource(source("drag-local", localData)),
				ui.Button(resource.DragFiles()).ID("drag-files").Enabled(len(d.files) > 0).DragSource(source("drag-files", filesData))).Spacing(12).LineSpacing(8),
			surface(ui.HBox(ui.Label(d.dropped).WrapMode(ui.WrapWordChar).MainWeight(1)).Padding(16)).Style(modern.Card).ID("drop-zone").MinHeight(110).DropTarget(target),
			ui.Label(resource.DragHint())),
	)
}
func (d *demo) selectedPaths(paths []string) {
	if len(paths) == 0 {
		d.report(resource.NoDialogPath())
		return
	}
	d.files = append([]string(nil), paths...)
	d.report(fmt.Sprintf(resource.ChosenPath(), strings.Join(paths, ", ")))
}
