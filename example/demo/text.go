package main

import (
	"fmt"
	"strings"

	"github.com/golang-gui/goui/example/demo/resource"
	"github.com/golang-gui/goui/gui"
	"github.com/golang-gui/goui/mlang"
	"github.com/golang-gui/goui/theme/modern"
	"github.com/golang-gui/goui/ui"
	wui "github.com/golang-gui/goui/widgets/ui"
)

func (d *demo) textPage() ui.View {
	model := d.editorModel()
	labels := []mlang.Getter{resource.RegularText, resource.LargeDocument, resource.LongJSON}
	items := make([]*ui.MenuItemView, len(labels))
	for i, label := range labels {
		items[i] = ui.MenuItem(label(), func() { d.selectTextSample(i) })
	}
	wrap := ui.WrapNone
	if d.wrap {
		wrap = ui.WrapWordChar
	}
	return fullPage("text", resource.Text(), resource.TextHint()+" "+resource.PlaceholderHint(),
		ui.TextInput().ID("single-input").BindText(&d.input).Placeholder(resource.InputPlaceholder()).OnSubmit(func() { d.report(fmt.Sprintf(resource.Submitted(), d.input.Get())) }),
		ui.HWrap(
			ui.MenuButton(fmt.Sprintf(resource.TextSampleFormat(), labels[d.textSample]())).ID("text-samples").Menu(items...),
			wui.Switch(resource.Wrap()).ID("text-wrap").Checked(d.wrap).OnChange(func(v bool) { d.wrap = v; d.app.RequestUpdate() }),
			wui.Switch(resource.ReadOnly()).ID("text-readonly").Checked(d.readonly).OnChange(func(v bool) { d.readonly = v; d.app.RequestUpdate() }),
			ui.Button(resource.Undo()).ID("text-undo").Enabled(!d.readonly && model.CanUndo()).OnClick(func() { d.editorHistory(false) }),
			ui.Button(resource.Redo()).ID("text-redo").Enabled(!d.readonly && model.CanRedo()).OnClick(func() { d.editorHistory(true) }),
		).Spacing(12).LineSpacing(8),
		ui.Label(resource.TextSamplesHint()).WrapMode(gui.WrapWordChar).Style(modern.MutedText),
		ui.ScrollView(ui.TextView().ID("editor").Model(model).Placeholder(resource.EditorPlaceholder()).WrapMode(wrap).ReadOnly(d.readonly).Padding(12).
			OnSelection(func(s ui.TextSelection) { d.selection = s; d.app.RequestUpdate() }).OnChange(func(ui.TextChange) { d.app.RequestUpdate() })).Style("text-view").MainWeight(1),
		ui.Label(fmt.Sprintf(resource.TextSampleStats(), model.Len(), model.LineCount(), d.selection)).ID("text-stats").WrapMode(gui.WrapWordChar),
	)
}

func (d *demo) editorModel() *ui.TextModel {
	if d.textSample == 0 {
		return d.text
	}
	return d.largeText[d.textSample-1]
}

func (d *demo) selectTextSample(index int) {
	if index < 0 || index > len(d.largeText) || index == d.textSample {
		return
	}
	// Build only declares the current model. Generate each larger sample once,
	// on explicit selection, and retain its independent editing history.
	if index > 0 && d.largeText[index-1] == nil {
		d.largeText[index-1] = ui.NewTextModel(largeTextSample(index))
	}
	d.textSample, d.selection = index, ui.TextSelection{}
	d.app.RequestUpdate()
}

func largeTextSample(index int) string {
	if index == 2 {
		return `{"sample":"long-line","payload":"` + strings.Repeat("AbCdEf0123456789", 65536) + `","end":"End of sample"}`
	}
	var text strings.Builder
	text.Grow(2 << 20)
	text.WriteString("{\n  \"entries\": [\n")
	for i := range 10000 {
		if i > 0 {
			text.WriteString(",\n")
		}
		fmt.Fprintf(&text, `    {"id":%d,"name":"Document %05d","message":"Hello, 世界 🌍","text":"Select, edit, undo and resize this document."}`, i+1, i+1)
	}
	text.WriteString("\n  ],\n  \"end\": \"End of sample\"\n}")
	return text.String()
}

func (d *demo) editorHistory(redo bool) {
	w := d.app.FindWidget("main", "editor")
	if w == nil {
		return
	}
	// Invoke the model's public history API and restore the history selection
	// to the active editor view.
	if editor, ok := w.(*gui.TextView); ok {
		r := editor.Model().UndoWithSelection
		if redo {
			r = editor.Model().RedoWithSelection
		}
		result := r()
		if result.Applied && result.Selection != nil {
			editor.SetSelection(*result.Selection)
		}
		d.app.RequestUpdate()
	}
}
