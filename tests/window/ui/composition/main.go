// 组合节点与普通对象的真实窗口验证，不依赖 DevServer。
// 环境：Linux X11、Windows 或 macOS 桌面；记录后端和缩放。
// 启动：go run ./tests/window/ui/composition；可用 GOUI_PLAT_PAINTER、GOUI_PLAT_SCALE。
// 初始：对象实例 1、成员 1、Count/Pulses 为 0，输入框为空。
// 操作与预期：
//  1. 点击 Increment、Pulse，再按 F6：Count/Pulses 各增加 1；对象不重新创建。
//  2. 在输入框输入文字，反复 Rebuild：文字和对象实例保留，成员仍为 1。
//  3. Hide content 后 F6：无视觉内容但对象未释放、成员 0；Pulse 仍能通知对象。
//     再次 Hide content 恢复输入与 Increment，文字保留，成员恢复为 1。
//  4. Swap output 后 F6：只剩 Alternate，原成员 0，对象不变；再次切换恢复。
//  5. Toggle object 移除声明后 F6：Created/Released 相等、无成员；Pulse 不再
//     通知已释放对象。重新 Toggle object 创建新实例，Count/Pulses 从 0 开始。
//  6. 关闭窗口：后代先解除关联，Release 断开订阅，最终输出 DESTROY PASS。
//
// Check/F6 的 ASSERT PASS 核对生命周期、成员、输入与实际 GUI 树，BOUNDS 是
// 窗口 DIP，只用于定位真实输入。没有对象或组合占位 Widget；1x/1.5x/2x 重复。
// 平台差异：字体和窗口装饰可不同；语义及释放顺序一致。
// 副作用：仅测试窗口和标准输出，不修改文件/设置/剪贴板。
// 复位：重新启动程序；退出：关闭窗口。
package main

import (
	"fmt"

	"github.com/golang-gui/goui/core/signal"
	"github.com/golang-gui/goui/gui"
	"github.com/golang-gui/goui/layout"
	"github.com/golang-gui/goui/theme/modern"
	"github.com/golang-gui/goui/ui"
)

type session struct {
	id, count, pulses, members int
	connection                 signal.Handle
}

type demo struct {
	app                          ui.App
	created, released, delivered int
	visible, alternate, present  bool
	text                         ui.State[string]
	pulse                        signal.Signal0
	active                       *session
}

type sessionView struct {
	ui.ComposeBase[sessionView]
	d     *demo
	child ui.View
}

func sessionScope(d *demo, child ui.View) *sessionView {
	v := &sessionView{d: d, child: child}
	v.Self = v
	return v.ID("session")
}

func (v *sessionView) Build(ctx ui.BuildContext) ui.View {
	if ctx.State() == nil {
		v.d.created++
		s := &session{id: v.d.created}
		s.connection = v.d.pulse.Connect(func() {
			s.pulses++
			v.d.delivered++
			v.d.app.RequestUpdate()
		})
		ctx.SetState(s)
		v.d.active = s
		v.d.app.RequestUpdate()
	}
	return v.child
}

func (*sessionView) Object(ctx ui.BuildContext) any { return ctx.State() }

func (v *sessionView) Release(ctx ui.BuildContext) {
	if s, ok := ctx.State().(*session); ok {
		if s.members != 0 {
			panic("object released before members disconnected")
		}
		s.connection.Disconnect()
		v.d.released++
		v.d.active = nil
		v.d.app.RequestUpdate()
	}
}

type referenceView struct {
	ui.ViewBase[referenceView]
	app ui.App
}

type referenceState struct {
	model  *session
	button *gui.Button
	click  signal.Handle
}

func reference(app ui.App) *referenceView {
	v := &referenceView{app: app}
	v.Self = v
	return v.ID("increment")
}

func (v *referenceView) Mount(ctx ui.BuildContext) gui.Widget {
	s := &referenceState{button: gui.NewButton()}
	s.click = s.button.ConnectClicked(func() {
		s.model.count++
		v.app.RequestUpdate()
	})
	ctx.SetState(s)
	return s.button
}

func (*referenceView) Reparented(ctx ui.BuildContext) {
	s := ctx.State().(*referenceState)
	model, ok := ui.FindAncestor[*session](ctx, "session")
	if !ok {
		panic("missing explicit session")
	}
	if s.model != model {
		if s.model != nil {
			s.model.members--
		}
		s.model = model
		model.members++
	}
}

func (v *referenceView) Update(ctx ui.BuildContext, widget gui.Widget) {
	v.Reparented(ctx)
	s := ctx.State().(*referenceState)
	ctx.UpdateChild(widget.(gui.Bin), ui.Label(fmt.Sprintf("Increment %d", s.model.count)))
}

func (*referenceView) Unmount(ctx ui.BuildContext, _ gui.Widget) {
	s := ctx.State().(*referenceState)
	s.click.Disconnect()
	if s.model != nil {
		s.model.members--
		s.model = nil
	}
}

type contentView struct {
	ui.ComposeBase[contentView]
	d *demo
}

func content(d *demo) *contentView {
	v := &contentView{d: d}
	v.Self = v
	return v.ID("content")
}

func (v *contentView) Build(ui.BuildContext) ui.View {
	if !v.d.visible {
		return nil
	}
	if v.d.alternate {
		return ui.Button("Alternate").ID("alternate")
	}
	return ui.HBox(reference(v.d.app), ui.TextInput().ID("editor").BindText(&v.d.text).MinWidth(220)).Spacing(12)
}

func (d *demo) check() {
	w := d.app.FindWindow("main")
	if d.created-d.released != boolInt(d.present) || (d.active != nil) != d.present {
		panic("object lifetime mismatch")
	}
	members, count, pulses, id := 0, 0, 0, 0
	if d.active != nil {
		members, count, pulses, id = d.active.members, d.active.count, d.active.pulses, d.active.id
	}
	if members != boolInt(d.present && d.visible && !d.alternate) {
		panic("member reference mismatch")
	}
	if gui.FindWidget(w, "session") != nil || gui.FindWidget(w, "content") != nil {
		panic("nonvisual declaration created a wrapper widget")
	}
	if editor := gui.FindWidget(w, "editor"); editor != nil && editor.(*gui.TextInput).Text() != d.text.Get() {
		panic("text state mismatch")
	}
	for _, name := range []string{"hide", "swap", "toggle", "pulse", "rebuild", "check", "increment", "editor", "alternate"} {
		if widget := gui.FindWidget(w, name); widget != nil {
			b := widget.Snapshot().Bounds
			fmt.Printf("BOUNDS %s %.2f %.2f %.2f %.2f\n", name, b.X, b.Y, b.Width, b.Height)
		}
	}
	fmt.Printf("ASSERT PASS created=%d released=%d active=%d members=%d count=%d pulses=%d delivered=%d text=%q\n", d.created, d.released, id, members, count, pulses, d.delivered, d.text.Get())
}

func boolInt(value bool) int {
	if value {
		return 1
	}
	return 0
}

func main() {
	d := &demo{visible: true, present: true}
	if err := ui.Run("org.golanggui.CompositionTest", func(app ui.App) ui.RootView {
		d.app = app
		gui.App.SetQuitOnLastWindowClosed(false)
		var scoped ui.View
		if d.present {
			scoped = sessionScope(d, content(d))
		}
		change := app.RequestUpdate
		return ui.Root().StyleSheet(modern.FromSettings(app.Settings())).Windows(
			ui.Window("main").Title("GOUI composition").Size(900, 340).Content(ui.VBox(
				ui.Label(fmt.Sprintf("Created %d | Released %d | Delivered %d", d.created, d.released, d.delivered)),
				ui.HBox(
					ui.Button("Hide content").ID("hide").OnClick(func() { d.visible = !d.visible; change() }),
					ui.Button("Swap output").ID("swap").OnClick(func() { d.alternate = !d.alternate; change() }),
					ui.Button("Toggle object").ID("toggle").OnClick(func() { d.present = !d.present; change() }),
					ui.Button("Pulse").ID("pulse").OnClick(func() { d.pulse.Emit() }),
					ui.Button("Rebuild").ID("rebuild").OnClick(change),
					ui.Button("Check").ID("check").OnClick(d.check),
				).Spacing(8),
				scoped,
			).Padding(18).Spacing(16).CrossAlign(layout.CrossStart).Shortcuts(ui.Shortcut(ui.KeyF6).OnActivate(d.check))).OnDestroy(func() {
				app.Post(func() {
					if d.active != nil || d.created != d.released {
						panic("final object leak")
					}
					fmt.Println("DESTROY PASS")
					app.Quit()
				})
			}),
		)
	}); err != nil {
		panic(err)
	}
}
