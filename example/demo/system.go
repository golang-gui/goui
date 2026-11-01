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

// systemDemo owns this feature for the lifetime of the gallery.
// host is borrowed for shared assets and application operations.
type systemDemo struct {
	host        *demo
	tickTimer   *ui.Timer
	ticks       ui.State[int]
	files       ui.State[[]string]
	fileAccess  []ui.FileAccess
	dropped     ui.State[string]
	droppedKind ui.State[demoDropKind]
}

type demoDropKind uint8

const (
	demoDropNone demoDropKind = iota
	demoDropText
	demoDropFiles
	demoDropCard
)

// Store payload separately from its translated description. Even empty text
// is a successful drop, rather than the initial invitation.
func (d *systemDemo) receiveDrop(r *ui.DropRequest) {
	if paths, ok := r.Data.Files(); ok {
		d.dropped.Set(strings.Join(paths, "\n"))
		d.droppedKind.Set(demoDropFiles)
	} else if text, ok := r.Data.Text(); ok {
		d.dropped.Set(text)
		d.droppedKind.Set(demoDropText)
	} else if value, ok := r.Data.Local("demo-card"); ok {
		d.dropped.Set(fmt.Sprint(value))
		d.droppedKind.Set(demoDropCard)
	} else {
		return
	}
	r.Accepted = true
	d.host.report(resource.DropAccepted())
}

func (d *systemDemo) dropDescription() string {
	switch d.droppedKind.Get() {
	case demoDropFiles:
		return fmt.Sprintf(resource.FilesReceived(), d.dropped.Get())
	case demoDropText:
		return fmt.Sprintf(resource.TextReceived(), d.dropped.Get())
	case demoDropCard:
		return resource.LocalCardReceived()
	default:
		return resource.DropZoneHint()
	}
}

func (d *systemDemo) View() ui.View { return ui.Build(d.view) }

func (d *systemDemo) view() ui.View {
	textData := new(gui.DragData)
	textData.SetText(d.host.input.state.Peek())
	localData := new(gui.DragData)
	localData.SetLocal("demo-card", "goui-demo-card")
	filesData := new(gui.DragData)
	filesData.SetFiles(d.files.Get())
	source := func(id string, data *gui.DragData) *ui.DragSourceView {
		return ui.DragSource().Data(data).OnPrepare(func(r *ui.DragPrepare) {
			if id == "drag-text" {
				r.Data.SetText(d.host.input.state.Peek())
			}
			w := d.host.app.FindWidget("main", id)
			if w == nil {
				r.Data = nil
				return
			}
			img, err := gui.RenderWidget(w, 0)
			if err != nil {
				r.Data = nil
				d.host.result(resource.DragPreview(), err)
				return
			}
			r.Preview = gui.DragPreview{Image: img, Scale: float32(img.Bounds().Dx()) / w.Rect().Width, Hotspot: r.Position}
		}).OnEnd(func(r ui.DragResult) {
			if r.Err != nil {
				d.host.result(resource.Drag(), r.Err)
			} else if r.Canceled {
				d.host.report(resource.DragCanceled())
			} else {
				d.host.report(fmt.Sprintf(resource.DragEnded(), r.Action))
			}
		})
	}
	target := ui.DropTarget(ui.LocalFormat("demo-card"), ui.DragFormatFiles, ui.DragFormatText).OnDrop(d.receiveDrop).OnError(func(e error) { d.host.result(resource.ReceiveDrop(), e) })
	return page("system", resource.System(), resource.SystemHint(),
		section(resource.Clipboard(), ui.TextInput().BindText(&d.host.input), ui.HWrap(ui.Button(resource.CopyText()).ID("copy-text").OnClick(func() {
			if err := d.host.app.Clipboard().SetText(d.host.input.Get()); err != nil {
				d.host.result(resource.CopyText(), err)
				return
			}
			d.host.report(resource.CopyRequested())
		}),
			ui.Button(resource.ReadClipboard()).ID("read-clipboard").OnClick(func() {
				if err := d.host.app.Clipboard().RequestText(func(text string, err error) {
					if err != nil {
						d.host.result(resource.ReadClipboard(), err)
					} else if text == "" {
						d.host.report(resource.ClipboardEmpty())
					} else {
						d.host.input.Set(text)
						d.host.report(resource.ClipboardRead())
					}
				}); err != nil {
					d.host.result(resource.ReadClipboard(), err)
				}
			})).Spacing(12)),
		section(resource.DialogsTitle(), ui.HWrap(ui.Button(resource.ChooseFile()).ID("choose-files").OnClick(func() {
			if err := d.host.app.FileDialog().OpenFile("main", ui.DialogOptions{Title: resource.ChooseDragFiles(), AllowMultiple: true}, d.selectedPaths); err != nil {
				d.host.result(resource.ChooseFile(), err)
			}
		}),
			ui.Button(resource.ChooseFolder()).OnClick(func() {
				if err := d.host.app.FileDialog().OpenDirectory("main", ui.DialogOptions{Title: resource.ChooseFolderTitle()}, d.selectedPaths); err != nil {
					d.host.result(resource.ChooseFolder(), err)
				}
			}),
			ui.Button(resource.SaveDialog()).OnClick(func() {
				if err := d.host.app.FileDialog().SaveFile("main", ui.DialogOptions{Title: resource.SaveDialogTitle(), DefaultName: "demo.txt"}, func(access []ui.FileAccess, err error) {
					if err != nil {
						d.host.result(resource.SaveDialogTitle(), err)
						return
					}
					if len(access) == 0 {
						d.host.report(resource.NoSavePath())
					} else {
						defer access[0].Close()
						path, err := access[0].LocalPath()
						if err != nil {
							d.host.result(resource.SaveDialogTitle(), err)
							return
						}
						d.host.report(fmt.Sprintf(resource.SavePath(), path))
					}
				}); err != nil {
					d.host.result(resource.SaveDialogTitle(), err)
				}
			}),
			ui.Button(resource.OpenWebsite()).OnClick(func() {
				d.host.result(resource.OpenBrowser(), d.host.app.OpenURL("https://github.com/golang-gui/goui"))
			}),
			ui.Button(resource.OpenSelected()).Enabled(len(d.files.Get()) > 0).OnClick(func() {
				if len(d.files.Get()) > 0 {
					d.host.result(resource.OpenPath(), d.host.app.OpenPath(d.files.Get()[0]))
				}
			})).Spacing(10).LineSpacing(10)),
		section(resource.TimerTitle(), ui.HBox(ui.Button(resource.TimerToggle()).ID("timer-toggle").OnClick(func() {
			if d.tickTimer != nil {
				d.tickTimer.Stop()
				d.tickTimer = nil
				d.host.report(resource.TimerStopped())
			} else {
				d.tickTimer = d.host.app.TickFunc(time.Second, func() { d.ticks.Set(d.ticks.Get() + 1) })
				d.host.report(resource.TimerStarted())
			}
		}), ui.Build(func() ui.View { return ui.Label(fmt.Sprintf(resource.TickCount(), d.ticks.Get())).ID("tick-count") })).Spacing(12)),
		section(resource.DragDropTitle(),
			ui.HWrap(ui.Button(resource.DragText()).ID("drag-text").DragSource(source("drag-text", textData)),
				ui.Button(resource.DragLocal()).ID("drag-local").DragSource(source("drag-local", localData)),
				ui.Button(resource.DragFiles()).ID("drag-files").Enabled(len(d.files.Get()) > 0).DragSource(source("drag-files", filesData))).Spacing(12).LineSpacing(8),
			ui.HBox(ui.Label(d.dropDescription()).WrapMode(ui.WrapWordChar).MainWeight(1)).Padding(16).Style(modern.Card).ID("drop-zone").MinHeight(110).DropTarget(target),
			ui.Label(resource.DragHint())),
	)
}
func (d *systemDemo) selectedPaths(access []ui.FileAccess, err error) {
	if err != nil {
		d.host.result(resource.DialogsTitle(), err)
		return
	}
	if len(access) == 0 {
		d.host.report(resource.NoDialogPath())
		return
	}
	var paths []string
	for _, item := range access {
		path, pathErr := item.LocalPath()
		if pathErr != nil {
			for _, acquired := range access {
				acquired.Close()
			}
			d.host.result(resource.DialogsTitle(), pathErr)
			return
		}
		paths = append(paths, path)
	}
	d.closeFiles()
	d.fileAccess = access
	d.files.Set(paths)
	d.host.report(fmt.Sprintf(resource.ChosenPath(), strings.Join(paths, ", ")))
}

func (d *systemDemo) closeFiles() {
	for _, access := range d.fileAccess {
		access.Close()
	}
	d.fileAccess = nil
	d.files.Set(nil)
}

func newSystemDemo(host *demo) *systemDemo {
	return &systemDemo{host: host}
}
