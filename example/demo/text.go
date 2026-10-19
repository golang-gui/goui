package main

import (
	"fmt"

	"github.com/golang-gui/goui/example/demo/resource"
	"github.com/golang-gui/goui/gui"
	"github.com/golang-gui/goui/ui"
	wui "github.com/golang-gui/goui/widgets/ui"
)

func (d *demo) textPage() ui.View {
	wrap := ui.WrapNone
	if d.wrap {
		wrap = ui.WrapWordChar
	}
	return fullPage("text", resource.Text(), resource.TextHint()+" "+resource.PlaceholderHint(),
		ui.TextInput().ID("single-input").BindText(&d.input).Placeholder(resource.InputPlaceholder()).OnSubmit(func() { d.report(fmt.Sprintf(resource.Submitted(), d.input.Get())) }),
		ui.HWrap(
			wui.Switch(resource.Wrap()).Checked(d.wrap).OnChange(func(v bool) { d.wrap = v; d.app.RequestUpdate() }),
			wui.Switch(resource.ReadOnly()).Checked(d.readonly).OnChange(func(v bool) { d.readonly = v; d.app.RequestUpdate() }),
			ui.Button(resource.Undo()).Enabled(!d.readonly && d.text.CanUndo()).OnClick(func() { d.editorHistory(false) }),
			ui.Button(resource.Redo()).Enabled(!d.readonly && d.text.CanRedo()).OnClick(func() { d.editorHistory(true) }),
		).Spacing(12).LineSpacing(8),
		ui.ScrollView(ui.TextView().ID("editor").Model(d.text).Placeholder(resource.EditorPlaceholder()).WrapMode(wrap).ReadOnly(d.readonly).Padding(12).
			OnSelection(func(s ui.TextSelection) { d.selection = s; d.app.RequestUpdate() }).OnText(func(string) { d.app.RequestUpdate() })).MainWeight(1),
		ui.Label(fmt.Sprintf(resource.SelectionFormat(), d.text.LineCount(), d.selection)),
	)
}
func (d *demo) editorHistory(redo bool) {
	w := d.app.FindWidget("main", "editor")
	if w == nil {
		return
	}
	// 工具栏调用模型的公开历史 API，并把历史选区恢复给当前编辑视图。
	if editor, ok := w.(*gui.TextView); ok {
		r := d.text.UndoWithSelection
		if redo {
			r = d.text.RedoWithSelection
		}
		result := r()
		if result.Applied && result.Selection != nil {
			editor.SetSelection(*result.Selection)
		}
		d.app.RequestUpdate()
	}
}
