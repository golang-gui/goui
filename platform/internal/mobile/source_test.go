package mobile

import (
	"github.com/golang-gui/goui/core/geometry"
	"github.com/golang-gui/goui/platform/common"
	"github.com/golang-gui/goui/platform/events"
	"image"
	"testing"
	"time"
)

func sourceWindow(t *testing.T) (*Window, *EventLoop) {
	t.Helper()
	loop := newTestEventLoop(t)
	t.Cleanup(loop.Destroy)
	w, err := NewWindow(loop, geometry.Size{Width: 10, Height: 10}, nil, common.WindowOptions{}, &testWindowHost{})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(w.Destroy)
	return w, loop
}
func TestSourceDelayedTargets(t *testing.T) {
	w, loop := sourceWindow(t)
	var generations []uint64
	w.handler = func(event events.Event) {
		if e, ok := event.(events.SurfaceEvent); ok && e.State.Ready {
			generations = append(generations, e.State.Generation)
		}
	}
	present := func(image.Image) error { return nil }
	// 原生线程先撤销，再换新目标；Go 队列尚未消费旧 attach。
	w.Source().Attach(11, 1, 100, 80, 2, present)
	w.Source().Revoke()
	w.Source().Attach(12, 2, 120, 80, 2, present)
	w.Source().Attach(13, 1, 130, 80, 2, present) // 延迟到达的旧代不得覆盖新目标。
	loop.Post(loop.Quit)
	loop.Run()
	if len(generations) != 1 || generations[0] != 2 || w.NativeHandle() != 12 {
		t.Fatalf("延迟目标复活或覆盖: %v handle=%d", generations, w.NativeHandle())
	}
	w.Destroy()
	w.Source().Attach(14, 3, 140, 80, 2, present)
	if w.SurfaceState().Ready || w.Source().current != nil {
		t.Fatal("关闭后的原生观察重新激活窗口")
	}
}
func TestSourceRevokeWaitsForLease(t *testing.T) {
	w, loop := sourceWindow(t)
	w.Source().Attach(11, 1, 100, 80, 2, func(image.Image) error { return nil })
	loop.Post(loop.Quit)
	loop.Run()
	lease, err := w.AcquireTarget()
	if err != nil {
		t.Fatal(err)
	}
	done := make(chan struct{})
	go func() { w.Source().Revoke(); close(done) }()
	select {
	case <-done:
		t.Fatal("租约仍在使用时允许原生目标释放")
	case <-time.After(20 * time.Millisecond):
	}
	lease.Release()
	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("释放租约后撤销未完成")
	}
	if w.SurfaceState().Ready || w.NativeHandle() != 0 {
		t.Fatal("不等待 Go 队列就应拒绝访问已撤销目标")
	}
}

func TestSourceRedrawCompletesOnNativeThread(t *testing.T) {
	w, loop := sourceWindow(t)
	frames := 0
	w.handler = func(e events.Event) {
		if e.Type() == events.Paint {
			frames++
		}
	}
	loop.Post(func() {
		w.Source().Attach(17, 1, 100, 80, 2, func(image.Image) error { return nil })
		w.Source().Visible(true)
		if !w.SurfaceState().Ready || !w.visible {
			t.Error("原生观察应在当前 UI 回调立即生效")
		}
		w.Source().Redraw()
		if frames != 1 {
			t.Error("原生 redraw 返回前未完成绘制", frames)
		}
		loop.Quit()
	})
	loop.Run()
	if frames != 1 {
		t.Fatal("Quit 未丢弃已经排队的多余绘制", frames)
	}
}
