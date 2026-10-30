package ui

import (
	"testing"

	"github.com/golang-gui/goui/core/geometry"
	"github.com/golang-gui/goui/gui"
	"github.com/golang-gui/goui/layout"
	"github.com/golang-gui/goui/platform/events"
	"github.com/golang-gui/goui/widgets"
)

func TestModifyMergesExplicitValuesAndRestoresContent(t *testing.T) {
	r := newRoot()
	t.Cleanup(r.unmountWindow)
	content := func() View {
		return Label("content").ID("inside").Name("inside name").Tooltip("inside help").
			Style("inside style").MinSize(100, 24).MaxSize(500, 80).
			MainWeight(2).Cursor(CursorText).Focusable(true)
	}
	w := r.update(Modify(Modify(content()).MinHeight(32).MaxWidth(400).MainWeight(3)).
		ID("outside").Name("").Tooltip("").Style("").MinWidth(200).MaxHeight(60).
		MainWeight(0).Cursor(nil).Visible(false).Enabled(false).Focusable(false))
	if w.ID() != "outside" || w.Name() != "" || w.Tooltip() != "" || w.StyleName() != "" ||
		w.MinSize() != (geometry.Size{Width: 200, Height: 32}) ||
		w.MaxSize() != (geometry.Size{Width: 400, Height: 60}) || w.MainWeight() != 0 ||
		w.Cursor() != nil || w.Visible() || w.Enabled() || w.Focusable() {
		t.Fatalf("外层显式值或逐轴合并错误：%+v", w.Snapshot())
	}
	if r.root.body.body.id != "inside" || r.root.baseCtx != nil || r.root.body.baseCtx != nil {
		t.Fatal("包装改写了内容身份或持有重复的 Widget 上下文")
	}
	if got := r.update(Modify(Modify(content())).ID("outside")); got != w ||
		w.MinSize() != (geometry.Size{Width: 100, Height: 24}) ||
		w.MaxSize() != (geometry.Size{Width: 500, Height: 80}) || w.MainWeight() != 2 ||
		!w.Visible() || !w.Enabled() || !w.Focusable() || w.Cursor() != CursorText ||
		w.Name() != "inside name" || w.Tooltip() != "inside help" || w.StyleName() != "inside style" {
		t.Fatal("省略外层配置未恢复当前内容声明，或重新创建了 Widget")
	}
	if got := r.update(Modify(Modify(Label("plain").ID("inside"))).ID("outside")); got != w ||
		w.MinSize() != (geometry.Size{}) || w.MaxSize() != (geometry.Size{}) || w.MainWeight() != 0 || w.Focusable() {
		t.Fatal("内外声明都省略配置时没有恢复构造默认值")
	}
}

func TestModifySharedContentMountsIndependently(t *testing.T) {
	r := newRoot()
	t.Cleanup(r.unmountWindow)
	shared := Label("shared").MainWeight(2).Style("original")
	box := r.update(HBox(
		Modify(shared).ID("a").Style("first").MainWeight(1),
		Modify(shared).ID("b").Style("second").Visible(false),
	)).(*gui.LinearBox)
	a, b := box.Children()[0], box.Children()[1]
	if a == b || a.StyleName() != "first" || b.StyleName() != "second" || a.MainWeight() != 1 ||
		b.MainWeight() != 2 || !a.Visible() || b.Visible() || shared.styleName != "original" || shared.mainWeight != 2 {
		t.Fatal("两个挂载共享了可变资源，或 Modify 修改了原声明")
	}
	info := box.Snapshot()
	if len(info.Children) != 2 || info.Children[0].ID != "a" || info.Children[1].ID != "b" ||
		len(info.Children[0].Children) != 0 || info.Children[1].Visible {
		t.Fatal("Snapshot 增加了包装节点或未报告最终 ID／状态")
	}
	r.update(HBox(Modify(shared).ID("b"), Modify(shared).ID("a")))
	if box.Children()[0] != b || box.Children()[1] != a || !b.Visible() || a.StyleName() != "original" {
		t.Fatal("包装的外层 ID 重排没有保留内容，或省略修饰没有恢复内层")
	}
}

func TestModifyBehaviorOverridesClearAndRestoreWithoutDuplicateControllers(t *testing.T) {
	r := newRoot()
	t.Cleanup(r.unmountWindow)
	host := &overlayInputWindow{testWindow: newTestWindow()}
	inner, outer := 0, 0
	content := func() View {
		return Label("content").Focusable(true).
			OnFocus(func(bool) { inner++ }).
			OnContextMenu(func(ctx gui.EventContext) { inner++; ctx.StopPropagation() }).
			Shortcuts(Shortcut(KeyF5).OnActivate(func() { inner++ })).
			DragSource(DragSource().Actions(DragCopy)).DropTarget(DropTarget(DragFormatText))
	}
	declaration := func() *ModifiedView {
		return Modify(content()).OnFocus(func(bool) { outer++ }).
			OnContextMenu(func(ctx gui.EventContext) { outer++; ctx.StopPropagation() }).
			Shortcuts(Shortcut(KeyF5).OnActivate(func() { outer++ })).
			DragSource(DragSource().Actions(DragMove)).DropTarget(DropTarget(DragFormatFiles))
	}
	w := r.updateWindow(host, declaration())
	ctx := r.root.body.baseCtx
	controllers := w.EventControllers()
	for range 3 {
		r.updateWindow(host, declaration())
		if len(w.EventControllers()) != 4 || r.root.body.baseCtx != ctx {
			t.Fatal("重复协调创建了多份控制器或连接上下文")
		}
		for i, c := range w.EventControllers() {
			if c != controllers[i] {
				t.Fatal("配置更新替换了已有控制器")
			}
		}
	}
	drag := w.Snapshot().DragDrop
	if drag == nil || drag.SourceActions != DragMove || len(drag.TargetFormats) != 1 || drag.TargetFormats[0] != DragFormatFiles {
		t.Fatal("外层拖放配置未覆盖内层")
	}
	host.SetFocusedWidget(w)
	for _, event := range []events.KeyEvent{
		{EventType: events.KeyDown, Key: events.KeyF5},
		{EventType: events.KeyDown, Key: events.KeyF10, Modifiers: events.ModifierShift},
	} {
		if err := host.DispatchEvent(event); err != nil {
			t.Fatal(err)
		}
	}
	// 单独检查连接路由；按键部分仍经过真实 EventDispatcher。
	ctx.onFocus(true)
	if inner != 0 || outer != 3 {
		t.Fatalf("回调未替换：inner=%d outer=%d", inner, outer)
	}
	r.updateWindow(host, Modify(content()).OnFocus(nil).OnContextMenu(nil).Shortcuts().DragSource(nil).DropTarget(nil))
	if len(w.EventControllers()) != 0 || ctx.onFocus != nil || w.Snapshot().DragDrop != nil {
		t.Fatal("显式 nil 或空列表未清除内层行为")
	}
	r.updateWindow(host, Modify(content()))
	if len(w.EventControllers()) != 4 || w.Snapshot().DragDrop.SourceActions != DragCopy {
		t.Fatal("省略外层行为没有恢复当前内层声明")
	}
	if err := host.DispatchEvent(events.KeyEvent{EventType: events.KeyDown, Key: events.KeyF5}); err != nil {
		t.Fatal(err)
	}
	if inner != 1 || outer != 3 {
		t.Fatal("恢复后仍调用旧回调或重复执行")
	}
	r.unmountWindow()
	if len(w.EventControllers()) != 0 || len(ctx.handles) != 0 {
		t.Fatal("释放后仍保留 UI 连接")
	}
}

// 自定义 setter 可执行外部代码，不能在释放 owner 后继续应用后面的修饰。
type modifierReleaseLabel struct {
	*gui.Label
	release      func()
	onAdd        func(gui.EventController)
	enabledCalls int
}

func (w *modifierReleaseLabel) AddEventController(controller gui.EventController) {
	w.Label.AddEventController(controller)
	if w.onAdd != nil {
		w.onAdd(controller)
	}
}

func (w *modifierReleaseLabel) SetFocusable(value bool) {
	w.Label.SetFocusable(value)
	if !value && w.release != nil {
		w.release()
	}
}
func (w *modifierReleaseLabel) SetEnabled(value bool) {
	w.enabledCalls++
	w.Label.SetEnabled(value)
}

func TestModifySetterMayReleaseOwner(t *testing.T) {
	r := newRoot()
	t.Cleanup(r.unmountWindow)
	w := &modifierReleaseLabel{Label: gui.NewLabel("release")}
	makeView := func() *enabledWidgetView {
		v := &enabledWidgetView{make: func() gui.Widget { return w }}
		v.Self = v
		return v
	}
	r.update(Modify(makeView()).Focusable(true))
	w.enabledCalls = 0
	w.release = r.unmountWindow
	if got := r.update(Modify(makeView()).Focusable(false).Enabled(false)); got != nil || w.enabledCalls != 0 {
		t.Fatal("setter 释放节点后仍继续设置属性或复活输出")
	}
}

func TestModifyControllerInstallationMayReleaseOwner(t *testing.T) {
	r := newRoot()
	t.Cleanup(r.unmountWindow)
	w := &modifierReleaseLabel{Label: gui.NewLabel("release")}
	makeView := func() *enabledWidgetView {
		v := &enabledWidgetView{make: func() gui.Widget { return w }}
		v.Self = v
		return v
	}
	r.update(Modify(makeView()))
	ctx := r.root.body.baseCtx
	w.onAdd = func(controller gui.EventController) {
		if _, ok := controller.(*gui.ShortcutController); ok {
			r.unmountWindow()
		}
	}
	got := r.update(Modify(makeView()).Shortcuts(Shortcut(KeyF5).OnActivate(func() { t.Fatal("已释放回调仍执行") })))
	if got != nil || len(w.EventControllers()) != 0 || ctx.shortcuts.controller != nil || len(ctx.shortcuts.entries) != 0 || len(ctx.handles) != 0 {
		t.Fatal("安装控制器期间释放 owner 后泄漏或重新创建了连接")
	}
}

// 共享修饰器遵循显式覆盖、空值清除和省略时恢复构造默认值的协调规则。
func TestViewTooltipRestoresInitialValue(t *testing.T) {
	r := newRoot()
	t.Cleanup(r.unmountWindow)
	declaration := func() *enabledWidgetView {
		v := &enabledWidgetView{make: func() gui.Widget {
			w := gui.NewLabel("content")
			w.SetName("name")
			w.SetTooltip("default help")
			return w
		}}
		v.Self = v
		return v
	}
	w := r.update(declaration().Tooltip("declared help"))
	if w.Tooltip() != "declared help" || w.Name() != "name" || w.Snapshot().Tooltip != w.Tooltip() {
		t.Fatal("Tooltip 修饰器没有应用或改变了名称")
	}
	if next := r.update(declaration().Tooltip("")); next != w || w.Tooltip() != "" {
		t.Fatal("显式空值没有清除提示或替换了控件")
	}
	r.update(declaration())
	if w.Tooltip() != "default help" {
		t.Fatal("省略修饰器没有恢复控件的初始提示")
	}
}

func TestViewEnabledReconcilesOwnSettingsAndSubtrees(t *testing.T) {
	for _, test := range []struct {
		name string
		make func() gui.Widget
	}{
		{"Button", func() gui.Widget { return gui.NewButton() }},
		{"TextInput", func() gui.Widget { return gui.NewTextInput() }},
		{"CheckButton", func() gui.Widget { return widgets.NewCheckButton() }},
		{"Switch", func() gui.Widget { return widgets.NewSwitch() }},
		{"DropDown", func() gui.Widget { return widgets.NewDropDown() }},
	} {
		t.Run(test.name, func(t *testing.T) {
			r := newRoot()
			t.Cleanup(r.unmountWindow)
			declaration := func() *enabledWidgetView {
				v := &enabledWidgetView{make: test.make}
				v.Self = v
				return v
			}
			w := r.update(declaration().Enabled(false))
			if w.Enabled() || gui.IsEnabled(w) || !w.Focusable() {
				t.Fatal("common Enabled modifier did not preserve focus configuration")
			}
			if next := r.update(declaration()); next != w || !w.Enabled() {
				t.Fatal("omission did not restore initial setting on the retained widget")
			}
			w.SetEnabled(false)
			r.update(declaration())
			if !w.Enabled() {
				t.Fatal("common coordination retained external setting")
			}
		})
	}
	r := newRoot()
	t.Cleanup(r.unmountWindow)
	box := r.update(VBox(Label("explicit").Enabled(false), Label("inherited")).Enabled(false)).(*gui.LinearBox)
	children := box.Children()
	if children[0].Enabled() || !children[1].Enabled() || gui.IsEnabled(children[1]) {
		t.Fatal("parent restriction overwrote local child settings")
	}
	r.update(VBox(Label("explicit").Enabled(false), Label("inherited")))
	if gui.IsEnabled(children[0]) || !gui.IsEnabled(children[1]) {
		t.Fatal("removing the container modifier did not restore the correct children")
	}
}

type enabledWidgetView struct {
	ViewBase[enabledWidgetView]
	make func() gui.Widget
}

func (v *enabledWidgetView) Mount(BuildContext) gui.Widget  { return v.make() }
func (*enabledWidgetView) Update(BuildContext, gui.Widget)  {}
func (*enabledWidgetView) Unmount(BuildContext, gui.Widget) {}

// Every widget-view constructor must wire ViewBase.Self so the shared chain
// modifiers (ID/Name/Hidden/Style) return the concrete view instead of panicking on
// a nil Self. This is the guardrail for the one unsafe edge of the self-type.
func TestViewConstructorsWireSelf(t *testing.T) {
	cases := []struct {
		name string
		make func() View // ends in a shared modifier, which panics if Self is nil
	}{
		{"Button", func() View { return Button("x").ID("btn") }},
		{"Label", func() View { return Label("x").ID("lbl") }},
		{"HBox", func() View { return HBox().ID("hbox") }},
		{"HeaderBar", func() View { return HeaderBar(nil).ID("header") }},
		{"VBox", func() View { return VBox().ID("vbox") }},
		{"TextInput", func() View { return TextInput().ID("input") }},
		{"Image", func() View { return Image(nil).ID("img") }},
	}
	for _, c := range cases {
		view := c.make() // would panic in self() if the constructor forgot Self
		if view == nil {
			t.Fatalf("%s: chained modifier returned nil (Self not wired)", c.name)
		}
		if view.base().id == "" {
			t.Fatalf("%s: shared ID modifier did not apply", c.name)
		}
	}
}

func TestViewIDAndNameRemainIndependentAcrossUpdates(t *testing.T) {
	r := newRoot()
	t.Cleanup(r.unmountWindow)
	w := r.update(Label("content").ID("first").Name("natural").Style("label")).(*gui.Label)
	if w.ID() != "first" || w.Name() != "natural" || w.StyleName() != "label" ||
		w.Snapshot().Text != "content" {
		t.Fatalf("initial identity: %+v", w.Snapshot())
	}
	next := r.update(Label("updated").ID("second").Name("natural")).(*gui.Label)
	if next == w {
		t.Fatal("changing declaration ID retained the widget")
	}
	w = next
	if w.ID() != "second" || w.Name() != "natural" || w.StyleName() != "" {
		t.Fatalf("updated identity: %+v", w.Snapshot())
	}
	w = r.update(Label("updated").ID("").Name("")).(*gui.Label)
	if w.ID() != "" || w.Name() != "" {
		t.Fatalf("explicit empty values were ignored: id=%q name=%q", w.ID(), w.Name())
	}
	r.update(Label("plain"))
	if w.ID() != "" || w.Name() != "" {
		t.Fatalf("removed modifiers did not restore defaults: id=%q name=%q", w.ID(), w.Name())
	}
}

type initialIdentityView struct{ ViewBase[initialIdentityView] }

func newInitialIdentityView() *initialIdentityView {
	v := new(initialIdentityView)
	v.Self = v
	return v
}

func (v *initialIdentityView) Mount(BuildContext) gui.Widget {
	w := gui.NewLabel("content")
	w.SetID("original-id")
	w.SetName("original-name")
	return w
}
func (v *initialIdentityView) Update(BuildContext, gui.Widget)  {}
func (v *initialIdentityView) Unmount(BuildContext, gui.Widget) {}

func TestViewIdentityRestoresConstructorDefaults(t *testing.T) {
	r := newRoot()
	t.Cleanup(r.unmountWindow)
	w := r.update(newInitialIdentityView().ID("replacement").Name("renamed"))
	if w.ID() != "replacement" || w.Name() != "renamed" {
		t.Fatalf("declared identity: id=%q name=%q", w.ID(), w.Name())
	}
	w = r.update(newInitialIdentityView().ID("").Name(""))
	if w.ID() != "" || w.Name() != "" {
		t.Fatalf("explicit empty identity: id=%q name=%q", w.ID(), w.Name())
	}
	r.update(newInitialIdentityView())
	if w.ID() != "original-id" || w.Name() != "original-name" {
		t.Fatalf("constructor identity not restored: id=%q name=%q", w.ID(), w.Name())
	}
}

// A view's size modifiers flow through apply() to the gui widget's size
// constraint (an empty VBox has zero intrinsic size, so MinSize alone drives it).
func TestViewSizeModifiersApplyToWidget(t *testing.T) {
	root := newRoot()
	w := root.update(VBox().MinSize(120, 80))
	got := w.Measure(layout.Loose(geometry.Size{Width: 1000, Height: 1000}))
	if got.Size != (geometry.Size{Width: 120, Height: 80}) {
		t.Fatalf("view MinSize not applied to widget: %+v (want 120x80)", got)
	}
}

func TestBoxCrossAlignAppliesToContainer(t *testing.T) {
	root := newRoot()
	box := root.update(VBox().CrossAlign(layout.CrossStretch)).(*gui.LinearBox)

	if box.CrossAlign() != layout.CrossStretch {
		t.Fatalf("view CrossAlign did not apply to the mounted widget: %v", box.CrossAlign())
	}
}

func TestViewPaddingApplies(t *testing.T) {
	root := newRoot()
	w := root.update(VBox().Padding(16))
	got := w.Measure(layout.Loose(geometry.Size{Width: 500, Height: 500}))
	if got.Size != (geometry.Size{Width: 32, Height: 32}) { // empty box, padding on both sides
		t.Fatalf("view Padding not applied: %+v (want 32x32)", got)
	}
}

func TestViewPaddingUnsetKeepsControlDefault(t *testing.T) {
	// A button has a non-zero built-in padding (6); a view that never calls
	// .Padding must not overwrite it to 0 — that is what paddingSet guards.
	root := newRoot()
	if b := root.update(Button("x")).(*gui.Button); b.Padding() != geometry.UniformInsets(6) {
		t.Fatalf("unset padding overwrote control default: %v (want 6)", b.Padding())
	}
	root2 := newRoot()
	if b := root2.update(Button("y").Padding(10)).(*gui.Button); b.Padding() != geometry.UniformInsets(10) {
		t.Fatalf("explicit padding not applied: %v (want 10)", b.Padding())
	}
}

func TestViewFocusableAppliesAndUnsetDontOverride(t *testing.T) {
	// A MenuButton is focusable by default (set in its constructor for
	// menu-bar keyboard navigation). A view that never calls .Focusable must
	// leave that default intact.
	root := newRoot()
	if b := root.update(MenuButton("File")).(*gui.MenuButton); !b.Focusable() {
		t.Fatal("unset focusable overwrote control default (want focusable)")
	}

	// Explicitly disabling/enabling focus is applied through the shared
	// modifier, so a menu-bar-style row can strip focus where needed.
	root2 := newRoot()
	if b := root2.update(MenuButton("File").Focusable(false)).(*gui.MenuButton); b.Focusable() {
		t.Fatal("Focusable(false) not applied to the mounted widget")
	}
	if b := root2.update(MenuButton("File").Focusable(true)).(*gui.MenuButton); !b.Focusable() {
		t.Fatal("Focusable(true) not applied to the mounted widget")
	}
}

// OnFocus is a cross-cutting modifier wired on viewBase: the view base
// lifecycle connects the mounted widget's gui focus signal and routes the
// callback through the persistent viewBaseContext. This test drives the focus
// signal directly (the Reconcile root builds the widget tree; focus dispatch is
// exercised in the gui package).
func TestViewOnFocusConnectsAndDynamicallyReadsCallback(t *testing.T) {
	root := newRoot()
	var saw []bool

	root.update(Button("x").OnFocus(func(focused bool) {
		saw = append(saw, focused)
	}))
	node := root.root
	baseCtx := node.baseCtx
	if baseCtx == nil || len(baseCtx.handles) != 1 {
		t.Fatal("OnFocus should connect a focus signal handle on mount")
	}

	// The callback is read from the live viewBaseContext at fire time, but the
	// context (and its handle) is independent of any single rebuild — a rebuilt
	// view keeps the same context and handle.
	root.update(Button("x").OnFocus(func(focused bool) {
		saw = append(saw, focused)
	}))
	if baseCtx != node.baseCtx || len(baseCtx.handles) != 1 {
		t.Fatal("OnFocus context and handle should persist across rebuilds")
	}
	if node.view.base().onFocus == nil {
		t.Fatal("OnFocus callback should remain set across rebuilds")
	}
	if baseCtx.onFocus == nil {
		t.Fatal("viewBase.update should refresh the context's effective callback")
	}

	// Dropping OnFocus keeps the handle connected (a no-op when the context's
	// callback is cleared), which is the cross-cutting contract: setting it back
	// re-arms the same handle.
	root.update(Button("x"))
	if node.baseCtx == nil || len(node.baseCtx.handles) != 1 {
		t.Fatal("OnFocus handle should remain connected across rebuilds even when callback removed")
	}
	if node.view.base().onFocus != nil {
		t.Fatal("removed OnFocus callback should be cleared on the view base")
	}
	if baseCtx.onFocus != nil {
		t.Fatal("removed OnFocus callback should clear the context's effective callback")
	}
}

func TestViewUnmountDisconnectsOnFocusHandle(t *testing.T) {
	root := newRoot()
	root.update(Button("x").OnFocus(func(focused bool) {}))
	node := root.root
	if node.baseCtx == nil || len(node.baseCtx.handles) != 1 {
		t.Fatal("setup: OnFocus handle not connected")
	}

	root.unmountWindow()

	if node.baseCtx == nil || len(node.baseCtx.handles) != 0 || node.baseCtx.onFocus != nil {
		t.Fatal("OnFocus handle and callback should be cleared on unmount")
	}
}

func TestViewPaddingsAppliesPerEdge(t *testing.T) {
	root := newRoot()
	box := root.update(VBox().Paddings(geometry.Insets{Left: 10, Top: 2, Right: 6, Bottom: 4})).(*gui.LinearBox)
	if box.Padding() != (geometry.Insets{Left: 10, Top: 2, Right: 6, Bottom: 4}) {
		t.Fatalf("view Paddings not applied: %+v", box.Padding())
	}
	if got := box.Measure(layout.Loose(geometry.Size{Width: 500, Height: 500})); got.Size != (geometry.Size{Width: 16, Height: 6}) {
		t.Fatalf("per-edge padding measurement: %+v", got)
	}
}

func TestViewPaddingsRestoreControlDefault(t *testing.T) {
	root := newRoot()
	input := root.update(TextInput().Paddings(geometry.Insets{Left: 1, Top: 2, Right: 3, Bottom: 4})).(*gui.TextInput)
	if input.Padding() != (geometry.Insets{Left: 1, Top: 2, Right: 3, Bottom: 4}) {
		t.Fatalf("text input Paddings not applied: %+v", input.Padding())
	}
	root.update(TextInput())
	if input.Padding() != (geometry.Insets{Left: 8, Top: 4, Right: 8, Bottom: 4}) {
		t.Fatalf("text input default not restored: %+v", input.Padding())
	}

	button := root.update(Button("x").Paddings(geometry.Insets{Left: 1, Top: 2, Right: 3, Bottom: 4})).(*gui.Button)
	if button.Padding() != (geometry.Insets{Left: 1, Top: 2, Right: 3, Bottom: 4}) {
		t.Fatalf("button Paddings not applied: %+v", button.Padding())
	}
	root.update(Button("y"))
	if button.Padding() != geometry.UniformInsets(6) {
		t.Fatalf("button default not restored: %+v", button.Padding())
	}
	view := root.update(TextView().Paddings(geometry.Insets{Left: 5, Top: 6, Right: 7, Bottom: 8})).(*gui.TextView)
	if view.Padding() != (geometry.Insets{Left: 5, Top: 6, Right: 7, Bottom: 8}) {
		t.Fatalf("text view Paddings not applied: %+v", view.Padding())
	}
	root.update(TextView())
	if view.Padding() != geometry.UniformInsets(4) {
		t.Fatalf("text view default not restored: %+v", view.Padding())
	}
}

// 格式控制仅来自文案，普通 Text 与 BindText 不增加新的协调入口。
func TestLabelFormattedTextAndBinding(t *testing.T) {
	state := MakeState(`\{format\}A\{bold:中\}`)
	builds := 0
	rt, fake := bindingApp(t, func() RootView {
		builds++
		return Window("main").Content(Label().BindText(&state))
	})
	label := fake.windows[0].Widget().(*gui.Label)
	if label.Text() != state.Get() || label.PlainText() != "A中" {
		t.Fatal("绑定未解析文案", label.Text())
	}
	state.Set("plain")
	fake.runPosted()
	if builds != 1 || label.Text() != "plain" || label.PlainText() != "plain" || label.Focusable() {
		t.Fatal("局部绑定没有恢复普通文案，或触发全量构建")
	}
	state.Set(`\{format\}B\{italic:字\}`)
	fake.runPosted()
	if fake.windows[0].Widget() != label || label.Snapshot().Text != "B字" {
		t.Fatal("绑定更新替换了 Widget 或泄露标记")
	}
	rt.RequestUpdate()
	fake.runPosted()
	if builds != 2 || fake.windows[0].Widget() != label {
		t.Fatal("普通重建改变控件身份")
	}
}

func TestLabelSelectableReconciliation(t *testing.T) {
	r := newRoot()
	t.Cleanup(r.unmountWindow)
	changes := 0
	build := func() *LabelView {
		return Label(`\{format\}A\{bold:中文\}`).Selectable(true).OnSelection(func(TextSelection) { changes++ })
	}
	label := r.update(build()).(*gui.Label)
	if !label.Selectable() || !label.Focusable() || label.Cursor() != gui.CursorText {
		t.Fatal("声明式选择默认值未生效")
	}
	label.SetSelection(TextSelection{Anchor: 1, Caret: 7})
	if changes != 1 {
		t.Fatal("用户选择通知未到达")
	}
	if next := r.update(build()); next != label || label.Selection() != (TextSelection{Anchor: 1, Caret: 7}) || changes != 1 {
		t.Fatal("重建改变身份或复位选择")
	}
	r.update(build().Selection(TextSelection{Caret: 1}))
	if changes != 1 || label.Selection() != (TextSelection{Caret: 1}) {
		t.Fatal("声明式选择回写了通知")
	}
	r.update(build().Focusable(false).Cursor(gui.CursorCrosshair))
	if label.Focusable() || label.Cursor() != gui.CursorCrosshair {
		t.Fatal("显式共享修饰器被选择默认值覆盖")
	}
	r.update(Label(label.Text()))
	if label.Selectable() || label.Focusable() || label.Cursor() != nil || label.Selection() != (TextSelection{}) {
		t.Fatal("省略 Selectable 未恢复普通 Label")
	}
	r.update(build())
	label.SetSelection(TextSelection{Caret: 1})
	if changes != 2 {
		t.Fatal("启用后信号丢失")
	}
}

func TestLabelSelectionBindingAndTextReset(t *testing.T) {
	text := MakeState(`\{format\}A\{bold:中文\}`)
	selection := MakeState(TextSelection{Anchor: 1, Caret: 7})
	builds := 0
	rt, fake := bindingApp(t, func() RootView {
		builds++
		return Window("main").Content(Label().BindText(&text).Selectable(true).BindSelection(&selection))
	})
	label := fake.windows[0].Widget().(*gui.Label)
	if label.Selection() != selection.Get() || !label.Focusable() {
		t.Fatal("初始绑定未应用")
	}
	label.SetSelection(TextSelection{Caret: 1})
	if selection.Get() != label.Selection() {
		t.Fatal("用户选择未回写")
	}
	fake.runPosted()
	selection.Set(TextSelection{Anchor: 7, Caret: 1})
	fake.runPosted()
	if label.Selection() != selection.Get() || builds != 1 {
		t.Fatal("选区属性绑定引发全局重建或未应用")
	}
	text.Set(`\{format\}A\{italic:中文\}`)
	fake.runPosted()
	if label.Selection() != selection.Get() {
		t.Fatal("格式变化重置绑定选区")
	}
	text.Set("new")
	fake.runPosted()
	if label.Selection() != (TextSelection{Anchor: 3, Caret: 1}) || selection.Get() != (TextSelection{Anchor: 7, Caret: 1}) {
		t.Fatal("正文绑定后应按声明的选区重新钳制而不回写外部状态", label.Selection(), selection.Get(), builds)
	}
	selection.Set(TextSelection{Caret: 2})
	fake.runPosted()
	if label.Selection() != selection.Get() {
		t.Fatal("重置后绑定不再有效")
	}
	rt.RequestUpdate()
	fake.runPosted()
	if builds != 2 || fake.windows[0].Widget() != label || label.Selection() != selection.Get() {
		t.Fatal("全量重建丢失选择或控件身份")
	}
}
