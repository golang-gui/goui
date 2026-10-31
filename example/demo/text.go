package main

import (
	"fmt"
	"strings"

	"github.com/golang-gui/goui/core/signal"
	"github.com/golang-gui/goui/example/demo/resource"
	"github.com/golang-gui/goui/gui"
	"github.com/golang-gui/goui/mlang"
	"github.com/golang-gui/goui/theme/modern"
	"github.com/golang-gui/goui/ui"
	wui "github.com/golang-gui/goui/widgets/ui"
)

// textDemo owns this feature for the lifetime of the gallery.
// host is borrowed for shared assets and application operations.
type textDemo struct {
	events       map[*ui.TextModel]*textEvents
	host         *demo
	text         *ui.TextModel
	textSample   ui.State[int]
	largeText    [2]*ui.TextModel
	textRevision uint64
	selection    ui.State[ui.TextSelection]
	readonly     ui.State[bool]
	wrap         ui.State[bool]
}

func (d *textDemo) View() ui.View { return ui.Build(d.view) }

func (d *textDemo) view() ui.View {
	model := d.editorModel()
	labels := []mlang.Getter{resource.RegularText, resource.LargeDocument, resource.LongJSON}
	items := make([]*ui.MenuItemView, len(labels))
	for i, label := range labels {
		items[i] = ui.MenuItem(label(), func() { d.selectTextSample(i) })
	}
	wrap := ui.WrapNone
	if d.wrap.Get() {
		wrap = ui.WrapWordChar
	}
	return fullPage("text", resource.Text(), resource.TextHint()+" "+resource.PlaceholderHint(),
		ui.TextInput().ID("single-input").BindText(&d.host.input).Placeholder(resource.InputPlaceholder()).OnSubmit(func() { d.host.report(fmt.Sprintf(resource.Submitted(), d.host.input.Get())) }),
		ui.HWrap(
			ui.MenuButton(fmt.Sprintf(resource.TextSampleFormat(), labels[d.textSample.Get()]())).ID("text-samples").Menu(items...),
			wui.Switch(resource.Wrap()).ID("text-wrap").Checked(d.wrap.Get()).OnChange(func(v bool) { d.wrap.Set(v) }),
			wui.Switch(resource.ReadOnly()).ID("text-readonly").Checked(d.readonly.Get()).OnChange(func(v bool) { d.readonly.Set(v) }),
			ui.Build(func() ui.View {
				return ui.HBox(
					ui.Button(resource.Undo()).ID("text-undo").Enabled(!d.readonly.Get() && model.CanUndo()).OnClick(func() { d.editorHistory(false) }),
					ui.Button(resource.Redo()).ID("text-redo").Enabled(!d.readonly.Get() && model.CanRedo()).OnClick(func() { d.editorHistory(true) }),
				).Spacing(12)
			}).Observe(d.events[model]),
		).Spacing(12).LineSpacing(8),
		ui.Label(resource.TextSamplesHint()).WrapMode(gui.WrapWordChar).Style(modern.MutedText),
		ui.ScrollView(ui.TextView().ID("editor").Model(model).Placeholder(resource.EditorPlaceholder()).WrapMode(wrap).ReadOnly(d.readonly.Get()).Padding(12).
			BindSelection(&d.selection)).Style("text-view").MainWeight(1),
		ui.Build(func() ui.View {
			return ui.Label(fmt.Sprintf(resource.TextSampleStats(), model.Len(), model.LineCount(), d.selection.Get())).ID("text-stats").WrapMode(gui.WrapWordChar)
		}).Observe(d.events[model]),
	)
}

func (d *textDemo) editorModel() *ui.TextModel {
	if d.textSample.Get() == 0 {
		return d.text
	}
	return d.largeText[d.textSample.Get()-1]
}

func (d *textDemo) selectTextSample(index int) {
	if index < 0 || index > len(d.largeText) || index == d.textSample.Get() {
		return
	}
	// Build only declares the current model. Generate each larger sample once,
	// on explicit selection, and retain its independent editing history.
	if index > 0 && d.largeText[index-1] == nil {
		d.largeText[index-1] = ui.NewTextModel(largeTextSample(index))
		d.events[d.largeText[index-1]] = &textEvents{model: d.largeText[index-1]}
	}
	{
		d.textSample.Set(index)
		d.selection.Set(ui.TextSelection{})
	}
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

func (d *textDemo) editorHistory(redo bool) {
	w := d.host.app.FindWidget("main", "editor")
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
	}
}

func newTextDemo(host *demo) *textDemo {
	model := ui.NewTextModel(resource.EditorSample())
	return &textDemo{host: host, text: model, wrap: ui.MakeState(true), events: map[*ui.TextModel]*textEvents{model: {model: model}}}
}

// textEvents borrows a document; View reconciliation owns the subscriptions.
type textEvents struct{ model *ui.TextModel }

func (s *textEvents) Connect(fn func()) signal.Handle {
	return s.model.ConnectChange(func(ui.TextChange) { fn() })
}
