package gui

import (
	"errors"
	"testing"

	"github.com/golang-gui/goui/core/geometry"
	"github.com/golang-gui/goui/platform/events"
)

type popoverPressController struct {
	EventControllerBase
	pressed bool
}

func (c *popoverPressController) HandleEvent(ctx EventContext) {
	if e, ok := ctx.Event().(events.PointerEvent); ok && e.EventType == events.PointerDown {
		c.pressed = true
	}
}

func (c *popoverPressController) Reset() { c.pressed = false }

func TestPopoverHideResetsNestedControllers(t *testing.T) {
	for _, kind := range []string{"ordinary", "shared"} {
		t.Run(kind, func(t *testing.T) {
			_, _, newPopup := transientFixture(t)
			client := newPopup(nil)
			var p interface {
				SetWidget(Widget)
				Show() error
				Hide()
				Destroy()
				DispatchEvent(events.Event) error
			} = client
			if kind == "ordinary" {
				p = NewPopover(client.Anchor(), nil)
				t.Cleanup(p.Destroy)
			}
			root, child := newTestWidget(), newTestWidget()
			root.SetMinSize(geometry.Size{Width: 100, Height: 60})
			root.AddChild(child)
			capture := &popoverPressController{EventControllerBase: NewEventControllerBase(PhaseCapture)}
			target := &popoverPressController{EventControllerBase: NewEventControllerBase(PhaseTarget)}
			root.AddEventController(capture)
			child.AddEventController(target)
			p.SetWidget(root)
			if err := p.Show(); err != nil {
				t.Fatal(err)
			}
			root.Arrange(geometry.Rect(0, 0, 100, 60))
			child.Arrange(geometry.Rect(5, 5, 90, 50))
			if err := p.DispatchEvent(events.PointerEvent{EventType: events.PointerDown, Position: child.windowRect().Center(), Button: events.PointerButtonLeft}); err != nil {
				t.Fatal(err)
			}
			if !capture.pressed || !target.pressed {
				t.Fatal("正常输入未到达父控件和子控件的控制器")
			}
			p.Hide()
			if capture.pressed || target.pressed {
				t.Fatal("关闭后父控件或子控件的控制器仍保留按压状态")
			}
		})
	}
}

func TestSharedPopoverOwnsOnlyClientState(t *testing.T) {
	w, plat, newPopup := transientFixture(t)
	p, q := newPopup(nil), newPopup(nil)
	a, b := p.Widget(), q.Widget()
	if _, ok := any(p).(Root); ok {
		t.Fatal("SharedPopover 意外实现 Root")
	}
	if a.Root() != nil || b.Root() != nil || w.shared != nil || len(plat.natives) != 0 {
		t.Fatal("构造或 SetWidget 提前挂载/分配宿主")
	}
	mounts, unmounts := 0, 0
	a.ConnectMount(func() { mounts++ })
	a.ConnectUnmount(func() { unmounts++ })
	if err := p.Show(); err != nil {
		t.Fatal(err)
	}
	root := a.Root()
	if actual, ok := root.(*popover); !ok || actual != w.shared.popup {
		t.Fatal("共享内容没有使用 Surface 缓存的普通 Popover")
	}
	native, painter := testSharedCore(p).platformPopup, testSharedCore(p).painter
	p.Hide()
	if a.Root() != root || mounts != 1 || unmounts != 0 {
		t.Fatal("Hide 没有保留唯一缓存树")
	}
	before := plat.natives[0].paints
	a.RequestLayout()
	a.RequestPaint()
	if plat.natives[0].paints != before {
		t.Fatal("隐藏内容仍调度原生绘制")
	}
	if err := p.Show(); err != nil {
		t.Fatal(err)
	}
	if a.Root() != root || mounts != 1 || unmounts != 0 {
		t.Fatal("同一内容重开重新挂载")
	}
	if err := q.Show(); err != nil {
		t.Fatal(err)
	}
	if a.Root() != nil || b.Root() != root || unmounts != 1 || testSharedCore(q).platformPopup != native || testSharedCore(q).painter != painter {
		t.Fatal("换内容没有复用唯一 GUI/原生宿主")
	}
	p.SetStyleName("old")
	p.SetModal(true)
	p.SetAutoDismiss(true)
	p.SetPosition(geometry.Point{X: 90, Y: 90})
	p.SetPlacement(PopoverPlacementBottom)
	if !q.Visible() || testSharedCore(q).styleName != "" || q.slot.popup.modal || q.slot.popup.position != (geometry.Point{}) {
		t.Fatal("失活客户端的配置污染当前显示")
	}
	p.Hide()
	p.Destroy()
	if !q.Visible() || len(plat.natives) != 1 || len(plat.painters) != 1 {
		t.Fatal("旧客户端销毁干扰当前客户端或重建资源")
	}
}

func TestPopoverTargetsExpirePerShow(t *testing.T) {
	w, _, newPopup := transientFixture(t)
	p := newPopup(nil)
	if err := p.Show(); err != nil {
		t.Fatal(err)
	}
	first := w.Popovers()[0]
	root := p.Widget().Root()
	closed := 0
	first.ConnectClosed(func() { closed++ })
	p.Hide()
	if first.Visible() || first.Widget() != nil || closed != 1 {
		t.Fatal("关闭没有作废当次目标")
	}
	if err := p.Show(); err != nil {
		t.Fatal(err)
	}
	second := w.Popovers()[0]
	if second == first || p.Widget().Root() != root {
		t.Fatal("重开未保留 Root 或复用了过期输入身份")
	}
	for _, e := range []events.Event{events.KeyEvent{EventType: events.KeyDown, Key: events.KeyEscape}, events.PointerEvent{EventType: events.PointerDown}} {
		if err := first.DispatchEvent(e); err == nil {
			t.Fatal("旧目标向重新显示的内容投递了输入")
		}
	}
	if err := first.CommitText("old"); err == nil || !second.Visible() || closed != 1 {
		t.Fatal("旧目标提交或关闭影响重新显示")
	}
}

func TestSharedPopoverInactiveFocusAndInput(t *testing.T) {
	_, _, newPopup := transientFixture(t)
	p, q := newPopup(nil), newPopup(nil)
	a := newTestWidget()
	a.SetFocusable(true)
	p.SetWidget(a)
	b := newTestWidget()
	b.SetFocusable(true)
	q.SetWidget(b)
	if err := p.Show(); err != nil {
		t.Fatal(err)
	}
	if !p.SetFocusedWidget(a) || p.SetFocusedWidget(b) {
		t.Fatal("焦点未限制到当前子树")
	}
	if err := q.Show(); err != nil {
		t.Fatal(err)
	}
	if p.FocusedWidget() != nil || p.SetFocusedWidget(a) || p.SetFocusedWidget(b) {
		t.Fatal("旧客户端仍拥有焦点操作权限")
	}
	if err := p.DispatchEvent(events.PointerEvent{EventType: events.PointerDown}); err == nil {
		t.Fatal("旧客户端仍可投递指针输入")
	}
	if !q.SetFocusedWidget(b) || q.FocusedWidget() != b {
		t.Fatal("当前客户端无法设置自己的焦点")
	}
	root := b.Root()
	q.Hide()
	if root.(EventTarget).SetFocusedWidget(b) || b.Focused() {
		t.Fatal("隐藏的实际宿主仍可获取逻辑焦点")
	}
	if err := q.DispatchEvent(events.PointerEvent{EventType: events.PointerDown}); err == nil {
		t.Fatal("隐藏缓存仍接收指针输入")
	}
}

func TestSharedPopoverTargetClosedReentry(t *testing.T) {
	w, _, newPopup := transientFixture(t)
	p, next := newPopup(nil), newPopup(nil)
	if err := p.Show(); err != nil {
		t.Fatal(err)
	}
	ref := w.Popovers()[0]
	ref.ConnectClosed(func() {
		if err := next.Show(); err != nil {
			t.Fatal(err)
		}
	})
	p.Hide()
	if !next.Visible() || ref.Visible() {
		t.Fatal("目标关闭通知未在完成清理后允许重入")
	}
}

func TestSharedPopoverRejectsAnchorInReplacedTree(t *testing.T) {
	_, _, newPopup := transientFixture(t)
	p := newPopup(nil)
	if err := p.Show(); err != nil {
		t.Fatal(err)
	}
	nested := NewSharedPopover(p.Widget())
	if err := nested.Show(); err == nil || !p.Visible() {
		t.Fatal("替换卸载了新弹层自己的 anchor")
	}
}

func TestSharedPopoverSupersededIsRecognizable(t *testing.T) {
	_, _, newPopup := transientFixture(t)
	a, b, c := newPopup(nil), newPopup(nil), newPopup(nil)
	if err := a.Show(); err != nil {
		t.Fatal(err)
	}
	a.ConnectClosed(func() {
		if err := c.Show(); err != nil {
			t.Fatal(err)
		}
	})
	if err := b.Show(); !errors.Is(err, ErrPopoverSuperseded) || b.Visible() || !c.Visible() {
		t.Fatalf("显示取消未保留较晚操作或不能识别：%v", err)
	}
}

func TestSharedPopoverUnmountCallbackDestroysOwner(t *testing.T) {
	for _, replaceVisible := range []bool{false, true} {
		t.Run(map[bool]string{false: "switch-hidden", true: "replace-visible"}[replaceVisible], func(t *testing.T) {
			w, plat, newPopup := transientFixture(t)
			p := newPopup(nil)
			if err := p.Show(); err != nil {
				t.Fatal(err)
			}
			content := p.Widget()
			content.ConnectUnmount(w.Destroy)
			if replaceVisible {
				p.SetWidget(newTestWidget())
			} else {
				p.Hide()
				q := newPopup(nil)
				if err := q.Show(); err == nil {
					t.Fatal("卸载回调结束 owner 后仍显示成功")
				}
			}
			if w.platformWindow != nil || !plat.natives[0].destroyed || w.shared.retained != nil || w.shared.current != nil {
				t.Fatal("卸载回调结束后遗留窗口或共享宿主")
			}
		})
	}
}
