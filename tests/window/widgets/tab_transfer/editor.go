package main

import (
	"fmt"
	"strings"

	"github.com/golang-gui/goui/core/geometry"
	"github.com/golang-gui/goui/gui"
	"github.com/golang-gui/goui/layout"
	"github.com/golang-gui/goui/theme/modern"
	"github.com/golang-gui/goui/widgets"
)

// 编辑页状态保留验收：go run ./tests/window/widgets/tab_transfer -editor。
// 环境同 main.go，可追加 -chrome integrated 或 -expect canceled。
// 初始：左窗仅有 Editor（100 行文本），右窗是空 TabView。
//  1. 点击左窗 Prepare；通过公开 API 准备独立撤销记录、反向选区和 240 DIP 滚动，
//     等待 PREPARED 日志。此后不要点击编辑区或调整窗口大小，以免主动改变待测状态。
//  2. 从左窗抓住 Editor 向下拖出，再移入右窗顶栏松开。预期原页面出现在右窗，
//     源栏为空但源窗口仍在。点击右窗 Check：断言文字、选区、滚动、对象和 Snapshot 归属；
//     通过后关闭源窗，按钮改为 Finish，输出 RETAINED。
//  3. 点击目标编辑区，用 Ctrl+Z（macOS Cmd+Z）撤销；再 Ctrl+Shift+Z / Cmd+Shift+Z
//     重做；最后输入 !。点击 Finish，必须分别观察到原文、准备后的文本和后续编辑，
//     且通过原 TextView 的 Change 信号收到通知。PASS 后自动退出。
//  4. canceled 模式在步骤 2 保持左键按下，按 Esc 后松开；点击左窗或右窗 Check。
//     预期没有转移/卸载/挂载，所有准备状态不变，PASS 后退出；不执行步骤 3。
//
// 预期：页面离开不会自行关闭源窗口，只有 Check 明确关闭；不创建替代编辑器。
// 仅准备测试文档和窗口，不改剪贴板、系统设置或磁盘文件。复位/退出规则同 main.go。
func runEditorTransfer(chrome gui.WindowChromeMode, canceled bool) error {
	app, err := gui.NewApplication("org.golang-gui.TabEditorTransfer")
	if err != nil {
		return err
	}
	app.SetStyleSheet(modern.Sheet(modern.Options{}))
	options := &gui.WindowOptions{Size: geometry.Size{Width: 480, Height: 420}, Chrome: chrome}
	source, err := app.NewWindow(options)
	if err != nil {
		return err
	}
	defer source.Destroy()
	target, err := app.NewWindow(options)
	if err != nil {
		return err
	}
	defer target.Destroy()
	line := "line content for retained editor\n"
	original := strings.Repeat(line, 100)
	edited := "EDIT" + original[4:]
	model := gui.NewTextModel(original)
	editor := gui.NewTextView()
	editor.SetID("retained-editor")
	editor.SetModel(model)
	scroll := gui.NewScrollView()
	scroll.SetChild(editor)
	page := widgets.NewTabPage("Editor", scroll)
	sv, tv := widgets.NewTabView(), widgets.NewTabView()
	sv.AppendPage(page)
	// Keep the selection inside the intended viewport. A pending caret reveal
	// must not compete with the fixture's scroll request before dragging.
	selection := gui.TextSelection{Anchor: 14*len(line) + 18, Caret: 14*len(line) + 6}
	mounts, unmounts, changes := 0, 0, 0
	editor.ConnectMount(func() { mounts++ })
	editor.ConnectUnmount(func() { unmounts++ })
	prepared, retained, undone, redone, continued, verified := false, false, false, false, false, false
	var failure error
	fail := func(err error) {
		if failure == nil {
			failure = err
		}
		app.Post(app.Quit)
	}
	editor.ConnectChange(func(gui.TextChange) {
		changes++
		if !retained {
			return
		}
		switch {
		case !undone && model.Text() == original:
			undone = true
		case undone && !redone && model.Text() == edited:
			redone = true
		case redone && strings.Contains(model.Text(), "!"):
			continued = true
		}
	})
	prepare := gui.NewButton()
	prepare.SetChild(gui.NewLabel("Prepare"))
	prepare.ConnectClicked(func() {
		if prepared || retained || page.Parent() != sv {
			fail(fmt.Errorf("prepare must run once before dragging"))
			return
		}
		if err := model.ReplaceAtomic(gui.TextRange{Start: 0, End: 4}, "EDIT"); err != nil {
			fail(err)
			return
		}
		editor.SetSelection(selection)
		scroll.SetScrollY(240)
		if scroll.ScrollY() != 240 {
			fail(fmt.Errorf("fixture caret reveal changed intended scroll: %g", scroll.ScrollY()))
			return
		}
		prepared = true
		fmt.Printf("PREPARED selection=%+v scroll=%g\n", editor.Selection(), scroll.ScrollY())
	})
	var checkLabels []*gui.Label
	check := func() {
		if retained {
			if !undone || !redone || !continued || changes < 4 || editor.Root() != target || editor.Destroyed() {
				fail(fmt.Errorf("keyboard continuation: undo=%v redo=%v edit=%v changes=%d", undone, redone, continued, changes))
				return
			}
			verified = true
			app.Post(app.Quit)
			return
		}
		owner, host, wantMounts, wantUnmounts := tv, target, 2, 1
		if canceled {
			owner, host, wantMounts, wantUnmounts = sv, source, 1, 0
		}
		if !prepared || page.Parent() != owner || page.Child() != scroll || scroll.Child() != editor ||
			editor.Model() != model || model.Text() != edited || editor.Selection() != selection ||
			scroll.ScrollY() != 240 || !model.CanUndo() || model.CanRedo() || changes != 1 ||
			mounts != wantMounts || unmounts != wantUnmounts || editor.Root() != host || owner.Current() != page {
			fail(fmt.Errorf("retention: prepared=%v owner=%v selection=%+v scroll=%g lifecycle=%d/%d changes=%d", prepared, page.Parent() == owner, editor.Selection(), scroll.ScrollY(), mounts, unmounts, changes))
			return
		}
		other := sv
		if canceled {
			other = tv
		}
		if len(other.Pages()) != 0 || other.Current() != nil || len(other.Snapshot().Children) != 0 {
			fail(fmt.Errorf("empty view retained page or snapshot"))
			return
		}
		info := owner.Snapshot()
		if len(info.Children) != 1 || info.Children[0].Role != widgets.RoleTabPanel || !info.Children[0].Selected ||
			len(info.Children[0].Children) != 1 || len(info.Children[0].Children[0].Children) != 1 ||
			info.Children[0].Children[0].Children[0].ID != editor.ID() {
			fail(fmt.Errorf("target snapshot lost original editor subtree"))
			return
		}
		if canceled {
			verified = true
			app.Post(app.Quit)
			return
		}
		source.Destroy()
		if editor.Destroyed() || editor.Root() != target {
			fail(fmt.Errorf("source destruction destroyed transferred editor"))
			return
		}
		retained = true
		for _, label := range checkLabels {
			if !label.Destroyed() {
				label.SetText("Finish")
			}
		}
		fmt.Println("RETAINED; click editor, Undo, Redo, type !, then Finish")
	}
	for i, w := range []gui.Window{source, target} {
		view := sv
		if i == 1 {
			view = tv
		}
		bar := widgets.NewTabBar()
		bar.SetView(view)
		bar.SetReorderable(true)
		bar.SetTransferable(true)
		bar.ConnectTransferError(fail)
		bar.ConnectDetachRequest(func(r *widgets.TabDetachRequest, handled *bool) { r.Cancel(); fail(fmt.Errorf("unexpected detach")) })
		column := gui.NewLinearBox(layout.DirectionVertical)
		column.SetCrossAlign(layout.CrossStretch)
		column.SetPadding(12)
		column.SetSpacing(8)
		if chrome == gui.WindowChromeIntegrated {
			header := gui.NewHeaderBar()
			header.SetChild(bar)
			column.AddChild(header)
		} else {
			column.AddChild(bar)
		}
		buttons := gui.NewLinearBox(layout.DirectionHorizontal)
		if i == 0 {
			buttons.AddChild(prepare)
		}
		button := gui.NewButton()
		label := gui.NewLabel("Check")
		checkLabels = append(checkLabels, label)
		button.SetChild(label)
		button.ConnectClicked(check)
		buttons.AddChild(button)
		column.AddChild(buttons)
		column.AddChild(view)
		w.SetWidget(column)
		w.SetMinSize(geometry.Size{Width: 400, Height: 350})
		w.ConnectCloseRequest(func(*bool) { app.Quit() })
		if err := w.Show(); err != nil {
			return err
		}
		if err := w.SetPosition(nil, geometry.Point{X: 40 + float32(i)*530, Y: 80}); err != nil {
			return err
		}
	}
	app.Run()
	if failure != nil {
		return failure
	}
	if !verified {
		return fmt.Errorf("editor test closed without verification")
	}
	fmt.Printf("PASS editor canceled=%v lifecycle=%d/%d undo=%v redo=%v continued=%v\n", canceled, mounts, unmounts, undone, redone, continued)
	return nil
}
