package ui

import (
	"errors"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/golang-gui/goui/gui"
	baseui "github.com/golang-gui/goui/ui"
	"github.com/golang-gui/goui/widgets"
)

// 仅验证适配器状态和异步请求；完整协调由窗口例程验证。
type toastTestContext struct {
	tabContext
	updates atomic.Int32
	apply   func(gui.Widget)
}

func (c *toastTestContext) UpdateChild(target baseui.Bin, child baseui.View, apply ...func(gui.Widget)) gui.Widget {
	if len(apply) != 0 {
		c.apply = apply[0]
	}
	return c.tabContext.UpdateChild(target, child, apply...)
}

func (c *toastTestContext) RequestUpdate() { c.updates.Add(1) }
func (c *toastTestContext) flush() {
	pending := c.after
	c.after = nil
	for _, fn := range pending {
		fn()
	}
}

func TestToastHandleRequestsAndRelease(t *testing.T) {
	var handle ToastHandle
	if !errors.Is(handle.Show(baseui.Label("unbound"), 0), ErrToastUnavailable) || !errors.Is(handle.Close(), ErrToastUnavailable) {
		t.Fatal("未绑定应报告不可用")
	}
	ctx := new(toastTestContext)
	v := Toast(&handle)
	if v.Build(ctx) != nil || v.Object(ctx) != &handle {
		t.Fatal("无请求时不应输出 Widget")
	}
	if handle.Show(nil, 0) == nil {
		t.Fatal("nil 内容应该报错")
	}
	if err := handle.Show(baseui.Label("first"), time.Second); err != nil {
		t.Fatal(err)
	}
	first := ctx.State().(*toastBinding).snapshot()
	if first == nil || first.duration != time.Second || ctx.updates.Load() != 1 {
		t.Fatal("请求未保存并触发局部更新")
	}
	_ = handle.Show(baseui.Label("new"), 0)
	binding := ctx.State().(*toastBinding)
	binding.dismiss(first.serial)
	if binding.snapshot() == nil {
		t.Fatal("旧关闭事件清理了新请求")
	}
	_ = handle.Close()
	if v.Build(ctx) != nil {
		t.Fatal("Close 没有清空输出")
	}
	v.Release(ctx)
	v.Release(ctx)
	if !errors.Is(handle.Show(baseui.Label("late"), 0), ErrToastUnavailable) {
		t.Fatal("释放后仍接受请求")
	}
	next := new(toastTestContext)
	Toast(&handle).Build(next)
	Toast(&handle).Release(next)
}

func TestToastHandleConcurrentRequests(t *testing.T) {
	var handle ToastHandle
	ctx := new(toastTestContext)
	v := Toast(&handle)
	v.Build(ctx)
	var wg sync.WaitGroup
	for range 8 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for i := 0; i < 100; i++ {
				if err := handle.Show(baseui.Label("content"), time.Second); err != nil {
					t.Error(err)
				}
				_ = handle.Close()
			}
		}()
	}
	wg.Wait()
	last := baseui.Label("last")
	_ = handle.Show(last, time.Second)
	if got := ctx.State().(*toastBinding).snapshot(); got == nil || got.view != last {
		t.Fatal("并发请求后最终内容不正确")
	}
	var releasing sync.WaitGroup
	releasing.Add(1)
	go func() {
		defer releasing.Done()
		for range 100 {
			_ = handle.Show(baseui.Label("racing release"), 0)
		}
	}()
	v.Release(ctx)
	releasing.Wait()
	if handle.binding != nil {
		t.Fatal("并发请求在释放后重新绑定")
	}
}

func TestToastBindingDoesNotRestartOrResurrect(t *testing.T) {
	var handle ToastHandle
	owner := new(toastTestContext)
	declaration := Toast(&handle)
	declaration.Build(owner)
	defer declaration.Release(owner)
	_ = handle.Show(baseui.Label("content"), time.Hour)
	v := declaration.Build(owner).(*toastContentView)
	ctx := new(toastTestContext)
	toast := v.Mount(ctx).(*widgets.Toast)
	defer v.Unmount(ctx, toast)
	v.Update(ctx, toast)
	if toast.Visible() {
		t.Fatal("展示应等待 AfterUpdate 子树挂载完成")
	}
	ctx.flush()
	if !toast.Visible() {
		t.Fatal("AfterUpdate 没有开始展示")
	}
	shown := ctx.State().(*toastContentState).shown
	v = declaration.Build(owner).(*toastContentView)
	v.Update(ctx, toast)
	if len(ctx.after) != 0 || ctx.State().(*toastContentState).shown != shown {
		t.Fatal("普通重建重启了展示时长")
	}
	toast.Close()
	toast.SetVisible(true)
	if toast.Visible() || declaration.Build(owner) != nil {
		t.Fatal("自关闭后仍能通过属性重放复活")
	}
	_ = handle.Show(baseui.Label("replacement"), 0)
	v = declaration.Build(owner).(*toastContentView)
	v.Update(ctx, toast)
	_ = handle.Close()
	ctx.flush()
	if toast.Visible() {
		t.Fatal("已取消请求的 AfterUpdate 启动了旧内容")
	}
}

func TestToastDuplicateHandleBindingPanics(t *testing.T) {
	var handle ToastHandle
	first, second := new(toastTestContext), new(toastTestContext)
	view := Toast(&handle)
	view.Build(first)
	defer view.Release(first)
	defer func() {
		if recover() == nil {
			t.Fatal("同一句柄同时绑定两处应拒绝")
		}
	}()
	Toast(&handle).Build(second)
}

func TestToastLocalContentOutputCanDisappearAndResume(t *testing.T) {
	var handle ToastHandle
	owner, ctx := new(toastTestContext), new(toastTestContext)
	declaration := Toast(&handle)
	declaration.Build(owner)
	defer declaration.Release(owner)
	_ = handle.Show(baseui.Label("dynamic"), time.Hour)
	v := declaration.Build(owner).(*toastContentView)
	toast := v.Mount(ctx).(*widgets.Toast)
	defer v.Unmount(ctx, toast)
	v.Update(ctx, toast)
	// 模拟通用协调器 apply 在局部 Build 输出替换时的调用。
	toast.SetChild(nil)
	ctx.apply(nil)
	ctx.flush()
	if toast.Visible() || ctx.State().(*toastContentState).shown != 0 {
		t.Fatal("无内容时不应开始展示")
	}
	child := gui.NewLabel("appeared")
	toast.SetChild(child)
	ctx.apply(child)
	ctx.flush()
	if !toast.Visible() {
		t.Fatal("局部输出出现后没有开始展示")
	}
	shown := ctx.State().(*toastContentState).shown
	toast.SetChild(nil)
	ctx.apply(nil)
	if toast.Visible() || declaration.Build(owner) == nil {
		t.Fatal("内容暂空应隐藏但保留请求")
	}
	toast.SetChild(child)
	ctx.apply(child)
	if !toast.Visible() || len(ctx.after) != 0 || ctx.State().(*toastContentState).shown != shown {
		t.Fatal("恢复已有展示不应重新启动计时")
	}
}
