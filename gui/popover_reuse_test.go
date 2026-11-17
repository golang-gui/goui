package gui

import (
	"errors"
	"testing"

	"github.com/golang-gui/goui/core/geometry"
	"github.com/golang-gui/goui/platform"
)

func ordinaryReuseFixture(t *testing.T) (*popover, *window, *transientTestPlatform, *testWidget, *testWidget) {
	t.Helper()
	w, plat, makeClient := transientFixture(t)
	a, b := newTestWidget(), newTestWidget()
	tree := newTestWidget()
	tree.AddChild(a)
	tree.AddChild(b)
	w.SetWidget(tree)
	tree.Arrange(geometry.Rect(0, 0, 400, 300))
	a.Arrange(geometry.Rect(10, 20, 80, 30))
	b.Arrange(geometry.Rect(210, 40, 90, 30))
	p := NewPopover(a, nil).(*popover)
	p.SetWidget(makeClient(nil).Widget())
	t.Cleanup(p.Destroy)
	return p, w, plat, a, b
}

func TestPopoverSetAnchorReusesNativeWithinSurface(t *testing.T) {
	p, w, plat, a, b := ordinaryReuseFixture(t)
	mounts, unmounts, closed := 0, 0, 0
	content := newTestWidget()
	content.SetMinSize(geometry.Size{Width: 60, Height: 30})
	content.ConnectMount(func() { mounts++ })
	content.ConnectUnmount(func() { unmounts++ })
	p.SetWidget(content)
	p.ConnectClosed(func() { closed++ })
	p.SetPlacement(PopoverPlacementBottom)
	if err := p.Show(); err != nil {
		t.Fatal(err)
	}
	native, painter := p.platformPopup, p.painter
	if len(w.ownedPopovers) != 1 {
		t.Fatal("可见实例没有登记资源归属")
	}
	p.SetAnchor(a)
	if !p.Visible() || closed != 0 {
		t.Fatal("相同锚点改变了显示")
	}
	p.SetAnchor(b)
	if p.Visible() || p.Anchor() != b || closed != 1 || len(plat.natives) != 1 {
		t.Fatal("SetAnchor 没有只结束旧显示并保留资源")
	}
	if err := p.Show(); err != nil {
		t.Fatal(err)
	}
	if p.platformPopup != native || p.painter != painter || content.Root() != p || mounts != 1 || unmounts != 0 {
		t.Fatal("同 Surface 换锚点重建资源或重复挂载")
	}
	if p.requestedPosition != (geometry.Point{X: 210, Y: 70}) {
		t.Fatal("没有使用新锚点定位", p.requestedPosition)
	}
	w.Widget().(*testWidget).RemoveChild(a)
	if !p.Visible() {
		t.Fatal("旧锚点卸载关闭了新显示")
	}
	p.Hide()
	if len(w.Popovers()) != 0 || len(w.ownedPopovers) != 1 {
		t.Fatal("可见枚举与隐藏资源归属混淆")
	}
	w.Destroy()
	if !plat.natives[0].destroyed || p.painter != nil || content.Root() != nil || len(w.ownedPopovers) != 0 {
		t.Fatal("Surface 未释放隐藏普通 Popover")
	}
}

func TestPopoverSetAnchorRebuildsAcrossSurfaces(t *testing.T) {
	p, old, plat, _, _ := ordinaryReuseFixture(t)
	if err := p.Show(); err != nil {
		t.Fatal(err)
	}
	native, painter := p.platformPopup, p.painter
	other := testWindowWithSurface(&window{surfaceBase: surfaceBase{platformWindow: &placementWindow{area: geometry.Rect(0, 0, 1000, 800)}}})
	t.Cleanup(other.Destroy)
	anchor := newTestWidget()
	other.SetWidget(anchor)
	anchor.Arrange(geometry.Rect(50, 60, 40, 30))
	p.SetAnchor(anchor)
	if p.platformPopup != native || len(plat.natives) != 1 {
		t.Fatal("SetAnchor 提前重建了原生窗口")
	}
	if err := p.Show(); err != nil {
		t.Fatal(err)
	}
	if p.platformPopup == native || p.painter == painter || !plat.natives[0].destroyed || len(plat.natives) != 2 || p.Widget().Root() != p {
		t.Fatal("跨 Surface 未释放重建资源并保留 Go Root")
	}
	if len(old.ownedPopovers) != 0 || len(other.ownedPopovers) != 1 {
		t.Fatal("资源仍登记在旧 Surface")
	}
	current := p.platformPopup
	old.Destroy()
	if p.platformPopup != current || !p.Visible() || plat.natives[1].destroyed {
		t.Fatal("旧 Surface 销毁影响了新归属")
	}
}

func TestPopoverAnchorUnmountRetainsUntilOwnerEnds(t *testing.T) {
	p, w, plat, a, _ := ordinaryReuseFixture(t)
	if err := p.Show(); err != nil {
		t.Fatal(err)
	}
	native, painter := p.platformPopup, p.painter
	tree := w.Widget().(*testWidget)
	tree.RemoveChild(a)
	if p.Visible() || p.platformPopup != native || p.painter != painter || plat.natives[0].destroyed {
		t.Fatal("锚点卸载销毁了仍有效的资源")
	}
	tree.AddChild(a)
	a.Arrange(geometry.Rect(10, 20, 80, 30))
	if err := p.Show(); err != nil {
		t.Fatal(err)
	}
	if p.platformPopup != native || p.painter != painter {
		t.Fatal("重新挂载后没有复用")
	}
	w.Destroy()
	other := testWindowWithSurface(&window{surfaceBase: surfaceBase{platformWindow: &placementWindow{area: geometry.Rect(0, 0, 1000, 800)}}})
	t.Cleanup(other.Destroy)
	anchor := newTestWidget()
	other.SetWidget(anchor)
	anchor.Arrange(geometry.Rect(10, 10, 50, 30))
	p.SetAnchor(anchor)
	if err := p.Show(); err != nil || len(plat.natives) != 2 {
		t.Fatal("原 Surface 结束后 Popover 对象不能迁移", err)
	}
}

func TestPopoverSetAnchorClosedReentryKeepsLaterRequest(t *testing.T) {
	p, _, _, _, b := ordinaryReuseFixture(t)
	if err := p.Show(); err != nil {
		t.Fatal(err)
	}
	handle := p.ConnectClosed(func() {
		if p.Anchor() != b {
			t.Fatal("Closed 看不到新锚点请求")
		}
		if err := p.Show(); err != nil {
			t.Fatal(err)
		}
	})
	t.Cleanup(handle.Disconnect)
	p.SetAnchor(b)
	if !p.Visible() || p.Anchor() != b {
		t.Fatal("SetAnchor 的旧清理覆盖了回调重开")
	}
}

func TestPopoverPreparationFailureRetainsValidNative(t *testing.T) {
	p, w, plat, _, _ := ordinaryReuseFixture(t)
	if err := p.Show(); err != nil {
		t.Fatal(err)
	}
	native, painter := p.platformPopup, p.painter
	p.Hide()
	w.platformWindow.(*placementWindow).err = platform.ErrUnavailable
	if err := p.Show(); !errors.Is(err, platform.ErrUnavailable) || p.Visible() {
		t.Fatal("查询失败仍显示或错误丢失", err)
	}
	if p.platformPopup != native || p.painter != painter || plat.natives[0].destroyed {
		t.Fatal("普通准备失败销毁了有效资源")
	}
}

func TestPopoverContentUnmountCanDestroyOwner(t *testing.T) {
	p, w, plat, _, _ := ordinaryReuseFixture(t)
	if err := p.Show(); err != nil {
		t.Fatal(err)
	}
	p.Widget().ConnectUnmount(func() {
		w.Destroy()
		if p.painter == nil || plat.natives[0].destroyed {
			t.Fatal("内容卸载回调尚未退出就释放了原生资源")
		}
	})
	p.SetWidget(newTestWidget())
	if w.platformWindow != nil || !plat.natives[0].destroyed || p.platformPopup != nil {
		t.Fatal("普通弹层内容卸载后未安全完成资源清理")
	}
}

func TestPopoverRejectsAnchorCycle(t *testing.T) {
	p, _, _, _, _ := ordinaryReuseFixture(t)
	p.SetAnchor(p.Widget())
	if err := p.Show(); err == nil {
		t.Fatal("接受了以自身内容为锚点的循环")
	}
}

func TestSharedSetAnchorSelectsExistingDestinationCache(t *testing.T) {
	w, plat, makeClient := transientFixture(t)
	a := makeClient(nil)
	if err := a.Show(); err != nil {
		t.Fatal(err)
	}
	source := w.shared.popup
	nativeSource := source.platformPopup
	other := testWindowWithSurface(&window{surfaceBase: surfaceBase{platformWindow: &placementWindow{area: geometry.Rect(0, 0, 1000, 800)}}})
	t.Cleanup(other.Destroy)
	anchor := newTestWidget()
	other.SetWidget(anchor)
	anchor.Arrange(geometry.Rect(10, 10, 50, 30))
	b := makeClient(anchor)
	if err := b.Show(); err != nil {
		t.Fatal(err)
	}
	destination := other.shared.popup
	nativeDestination := destination.platformPopup
	a.SetAnchor(anchor)
	if a.Visible() || source.platformPopup != nativeSource || plat.natives[0].destroyed {
		t.Fatal("共享客户端换锚点销毁或迁移了源缓存")
	}
	if err := a.Show(); err != nil {
		t.Fatal(err)
	}
	if b.Visible() || a.Widget().Root() != destination || destination.platformPopup != nativeDestination || len(plat.natives) != 2 {
		t.Fatal("未选择目标 Surface 已存在的缓存")
	}
	c := makeClient(nil)
	if err := c.Show(); err != nil {
		t.Fatal(err)
	}
	if c.Widget().Root() != source || source.platformPopup != nativeSource || len(plat.natives) != 2 {
		t.Fatal("源 Surface 不能继续复用缓存")
	}
}
