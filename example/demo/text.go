package main

import (
	"fmt"

	"github.com/golang-gui/goui/gui"
	"github.com/golang-gui/goui/ui"
	wui "github.com/golang-gui/goui/widgets/ui"
)

func (d *demo) textPage() ui.View {
	wrap := ui.WrapNone
	if d.wrap {
		wrap = ui.WrapWordChar
	}
	return fullPage("文本编辑", "单行提交、多行编辑与原生 IME。模型在页面之外创建，切换分类保留文本和撤销历史。",
		ui.TextInput().ID("single-input").BindText(&d.input).OnSubmit(func() { d.report("提交内容：" + d.input.Get()) }),
		ui.HWrap(
			wui.Switch("自动换行").Checked(d.wrap).OnChange(func(v bool) { d.wrap = v; d.app.RequestUpdate() }),
			wui.Switch("只读").Checked(d.readonly).OnChange(func(v bool) { d.readonly = v; d.app.RequestUpdate() }),
			ui.Button("撤销").Enabled(!d.readonly && d.text.CanUndo()).OnClick(func() { d.editorHistory(false) }),
			ui.Button("重做").Enabled(!d.readonly && d.text.CanRedo()).OnClick(func() { d.editorHistory(true) }),
		).Spacing(12).LineSpacing(8),
		ui.ScrollView(ui.TextView().ID("editor").Model(d.text).WrapMode(wrap).ReadOnly(d.readonly).Padding(12).
			OnSelection(func(s ui.TextSelection) { d.selection = s; d.app.RequestUpdate() }).OnText(func(string) { d.app.RequestUpdate() })).MainWeight(1),
		ui.Label(fmt.Sprintf("%d 行 · 选区 %+v", d.text.LineCount(), d.selection)),
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
