package mobile

import (
	"errors"
	"image"
	"reflect"
	"sync"
	"testing"
	"time"

	"github.com/golang-gui/goui/platform/common"
	"github.com/golang-gui/goui/platform/events"
)

type testWindowHost struct{ closed int }

func (*testWindowHost) setVisible(bool) error { return nil }
func (h *testWindowHost) close()              { h.closed++ }

func TestWindowSurfaceLifecycle(t *testing.T) {
	loop := NewEventLoop()
	defer loop.Destroy()
	var types []events.EventType
	var size events.SizeEvent
	paints := 0
	w := &Window{loop: loop, native: &testWindowHost{}}
	w.handler = func(e events.Event) {
		types = append(types, e.Type())
		if e, ok := e.(events.SizeEvent); ok {
			size = e
		}
		if e.Type() == events.Paint {
			paints++
			loop.Quit()
		}
	}
	w.RequestPaint()
	if w.SurfaceState().Ready || w.NativeHandle() != 0 {
		t.Fatal("未收到表面就报告可绘制")
	}
	old := &surfaceTarget{handle: 7, generation: 1, alive: true, present: func(image.Image) error { return nil }}
	w.attach(old, 800, 600, 2)
	w.setVisible(true)
	// 故意让旧 generation 的 Paint 已经排队，再用新目标替换。
	old.revoke()
	w.detach(old)
	next := &surfaceTarget{handle: 8, generation: 2, alive: true, present: func(image.Image) error { return nil }}
	w.attach(next, 900, 600, 3)
	loop.Run()
	if paints != 1 || size.Width != 300 || size.PixelWidth != 900 || w.NativeHandle() != 8 {
		t.Fatalf("恢复错误: paints=%d size=%+v", paints, size)
	}
	want := []events.EventType{events.Surface, events.Size, events.Visibility, events.Surface, events.Surface, events.Size, events.Paint}
	if !reflect.DeepEqual(types, want) {
		t.Fatalf("事件次序=%v，预期=%v", types, want)
	}
	if w.SurfaceState() != (common.SurfaceState{Ready: true, Generation: 2}) {
		t.Fatal(w.SurfaceState())
	}
}

func TestSurfaceRevocationJoinsPresent(t *testing.T) {
	entered, release, returned := make(chan struct{}), make(chan struct{}), make(chan struct{})
	target := &surfaceTarget{handle: 1, generation: 1, alive: true, present: func(image.Image) error { close(entered); <-release; return nil }}
	go func() { target.draw(image.NewRGBA(image.Rect(0, 0, 1, 1))); close(returned) }()
	<-entered
	revoked := make(chan struct{})
	go func() { target.revoke(); close(revoked) }()
	select {
	case <-revoked:
		t.Fatal("提交未完成就允许原生对象销毁")
	case <-time.After(20 * time.Millisecond):
	}
	close(release)
	select {
	case <-revoked:
	case <-time.After(time.Second):
		t.Fatal("撤销死锁")
	}
	<-returned
	if target.ready() || !errors.Is(target.draw(nil), common.ErrUnavailable) {
		t.Fatal("撤销后仍然允许原生访问")
	}
}

func TestNativeTargetLeaseRevocation(t *testing.T) {
	target := &surfaceTarget{handle: 17, generation: 3, alive: true}
	w := &Window{target: target}
	lease, err := w.AcquireTarget()
	if err != nil || lease.NativeHandle() != 17 || lease.Generation() != 3 {
		t.Fatalf("借用失败：%v", err)
	}
	finished := make(chan struct{})
	go func() { target.revoke(); close(finished) }()
	deadline := time.Now().Add(time.Second)
	for target.ready() && time.Now().Before(deadline) {
		time.Sleep(time.Millisecond)
	}
	if target.ready() {
		t.Fatal("原生撤销没有立即拒绝新借用")
	}
	if _, err = w.AcquireTarget(); !errors.Is(err, common.ErrUnavailable) {
		t.Fatalf("撤销期间仍可借用：%v", err)
	}
	if err := target.draw(image.NewRGBA(image.Rect(0, 0, 1, 1))); !errors.Is(err, common.ErrUnavailable) {
		t.Fatalf("撤销期间 Draw 未拒绝：%v", err)
	}
	select {
	case <-finished:
		t.Fatal("借用未释放就完成撤销")
	default:
	}
	lease.Release()
	lease.Release()
	select {
	case <-finished:
	case <-time.After(time.Second):
		t.Fatal("释放后撤销未结束")
	}
}

func TestWindowDestroyFromCallback(t *testing.T) {
	loop := NewEventLoop()
	defer loop.Destroy()
	host := &testWindowHost{}
	w := &Window{loop: loop, native: host}
	var got []events.EventType
	w.handler = func(e events.Event) {
		got = append(got, e.Type())
		if e.Type() == events.Surface {
			w.Destroy()
		}
	}
	target := &surfaceTarget{handle: 1, generation: 1, alive: true}
	w.attach(target, 320, 240, 1)
	w.Destroy()
	if !reflect.DeepEqual(got, []events.EventType{events.Surface, events.Destroy}) || host.closed != 1 || target.ready() {
		t.Fatalf("销毁重入错误: %v", got)
	}
	if w.NativeHandle() != 0 || !errors.Is(w.RequestPaint(), common.ErrUnavailable) {
		t.Fatal("销毁后仍可用")
	}
}

func TestRevokedQueuedTargetIsIgnored(t *testing.T) {
	loop := NewEventLoop()
	defer loop.Destroy()
	w := &Window{loop: loop, native: &testWindowHost{}, handler: func(events.Event) { t.Error("投递了已经撤销的目标") }}
	target := &surfaceTarget{handle: 9, generation: 1, alive: true}
	var wg sync.WaitGroup
	wg.Add(1)
	go func() { defer wg.Done(); target.revoke() }()
	wg.Wait()
	w.attach(target, 300, 200, 1)
	if w.SurfaceState().Ready {
		t.Fatal("已丢失的目标被复活")
	}
}

func TestWindowDrawRequiresLiveVisibleTarget(t *testing.T) {
	loop := NewEventLoop()
	defer loop.Destroy()
	w := &Window{loop: loop, native: &testWindowHost{}}
	presents := 0
	target := &surfaceTarget{handle: 11, generation: 1, alive: true, present: func(image.Image) error { presents++; return nil }}
	w.attach(target, 8, 6, 2)
	img := image.NewRGBA(image.Rect(0, 0, 8, 6))
	if !w.SurfaceState().Ready || !errors.Is(w.Draw(img), common.ErrUnavailable) {
		t.Fatal("目标就绪不等于内容可见")
	}
	w.setVisible(true)
	if !errors.Is(w.Draw(image.NewRGBA(image.Rect(0, 0, 4, 3))), common.ErrUnavailable) {
		t.Fatal("接受了 DIP 大小而非物理像素大小")
	}
	if err := w.Draw(img); err != nil {
		t.Fatal(err)
	}
	// 原生线程已撤销，但 GUI 队列尚未处理 detach：此时也必须拒绝呈现。
	target.revoke()
	if w.SurfaceState().Ready || w.NativeHandle() != 0 || !errors.Is(w.Draw(img), common.ErrUnavailable) {
		t.Fatal("队列延迟导致销毁后的原生访问")
	}
	if presents != 1 {
		t.Fatalf("原生调用次数=%d", presents)
	}
}
