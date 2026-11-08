package mobile

import (
	"github.com/xuges/gothread"
	"reflect"
	"runtime"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

func TestEventLoopOrderAndQuit(t *testing.T) {
	loop := newTestEventLoop(t)
	defer loop.Destroy()
	var got []int
	loop.Post(nil)
	loop.Post(func() {
		got = append(got, 1)
		loop.Post(func() { got = append(got, 3); loop.Quit() })
		loop.Post(func() { t.Error("Quit 后仍执行排队任务") })
	})
	loop.Post(func() { got = append(got, 2) })
	loop.Run()
	loop.Post(func() { t.Error("退出后接受任务") })
	if !reflect.DeepEqual(got, []int{1, 2, 3}) {
		t.Fatalf("顺序=%v", got)
	}
}

func TestEventLoopConcurrentPosts(t *testing.T) {
	loop := newTestEventLoop(t)
	defer loop.Destroy()
	const producers, count = 8, 128
	var wg sync.WaitGroup
	seen := make([]int, producers)
	for producer := range producers {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for value := range count {
				loop.Post(func() {
					if seen[producer] != value {
						t.Errorf("发送者 %d 顺序错误: %d/%d", producer, seen[producer], value)
					}
					seen[producer]++
				})
			}
		}()
	}
	go func() { wg.Wait(); loop.Post(loop.Quit) }()
	loop.Run()
	for _, n := range seen {
		if n != count {
			t.Fatalf("丢失任务: %v", seen)
		}
	}
}

func TestEventLoopStop(t *testing.T) {
	loop := newTestEventLoop(t)
	loop.Post(func() { t.Error("Run 前退出仍然执行任务") })
	loop.Quit()
	loop.Quit()
	loop.Run()
	loop.Destroy()
	loop.Destroy()
	defer func() {
		if recover() == nil {
			t.Fatal("重复 Run 应 panic")
		}
	}()
	loop.Run()
}

// 每个测试使用独立且锁定 OS 线程的执行器，模拟系统 UI 主循环。
// 启动 goroutine 只调用 Run 等待，不消费任务。
func newTestEventLoop(t *testing.T) *EventLoop {
	t.Helper()
	loop, _ := newTestNativeLoop(t)
	return loop
}

func newTestNativeLoop(t *testing.T) (*EventLoop, func(func())) {
	t.Helper()
	wake, stop, ready, done := make(chan struct{}, 1), make(chan struct{}), make(chan struct{}), make(chan struct{})
	native := make(chan func())
	var owner atomic.Uint64
	var loop *EventLoop
	go func() {
		runtime.LockOSThread()
		defer runtime.UnlockOSThread()
		defer close(done)
		owner.Store(uint64(gothread.GetId()))
		close(ready)
		for {
			select {
			case <-wake:
				loop.Dispatch()
			case task := <-native:
				task()
			case <-stop:
				return
			}
		}
	}()
	<-ready
	loop = NewEventLoop(func() {
		select {
		case wake <- struct{}{}:
		default:
		}
	}, func() bool { return owner.Load() == uint64(gothread.GetId()) })
	t.Cleanup(func() { loop.Destroy(); close(stop); <-done })
	return loop, func(task func()) { native <- task }
}

func TestEventLoopUsesNativeThread(t *testing.T) {
	loop := newTestEventLoop(t)
	startup := gothread.GetId()
	var thread uint64
	loop.Post(func() {
		thread = uint64(gothread.GetId())
		if !loop.current() || thread == uint64(startup) {
			t.Error("任务没有切换至原生 UI 执行器")
		}
		got := []int{1}
		loop.Deliver(func() { got = append(got, 2) })
		got = append(got, 3)
		if !reflect.DeepEqual(got, []int{1, 2, 3}) {
			t.Error("原生观察未同步交付", got)
		}
		loop.Quit()
	})
	loop.Run()
	if thread == 0 {
		t.Fatal("没有执行 UI 任务")
	}
}

func TestEventLoopQuitWaitsForNativeCallback(t *testing.T) {
	loop := newTestEventLoop(t)
	entered, release, returned := make(chan struct{}), make(chan struct{}), make(chan struct{})
	loop.Post(func() { loop.Quit(); close(entered); <-release })
	go func() { loop.Run(); close(returned) }()
	<-entered
	select {
	case <-returned:
		close(release)
		t.Fatal("UI 回调尚未返回，Run 就已结束")
	case <-time.After(20 * time.Millisecond):
	}
	close(release)
	select {
	case <-returned:
	case <-time.After(time.Second):
		t.Fatal("原生主循环确认后 Run 未结束")
	}
}

func TestEventLoopRejectsRunOnUIThread(t *testing.T) {
	loop := newTestEventLoop(t)
	panicked := false
	loop.Post(func() {
		func() { defer func() { panicked = recover() != nil }(); loop.Run() }()
		loop.Quit()
	})
	loop.Run()
	if !panicked {
		t.Fatal("允许 Run 阻塞系统 UI 主线程")
	}
}

func TestEventLoopNestedDispatchDefersTasks(t *testing.T) {
	loop := newTestEventLoop(t)
	var got []int
	loop.Post(func() {
		got = append(got, 1)
		loop.Post(func() { got = append(got, 3); loop.Quit() })
		loop.Dispatch()
		got = append(got, 2)
	})
	loop.Run()
	if !reflect.DeepEqual(got, []int{1, 2, 3}) {
		t.Fatal("嵌套原生循环重入 Go 任务", got)
	}
}

func TestEventLoopQuitWaitsForReentrantNativeObservation(t *testing.T) {
	loop, native := newTestNativeLoop(t)
	started, entered, release, returned := make(chan struct{}), make(chan struct{}), make(chan struct{}), make(chan struct{})
	loop.Post(func() { close(started) })
	go func() { loop.Run(); close(returned) }()
	<-started
	native(func() {
		loop.Deliver(func() {
			loop.Quit()
			// 模拟原生观察回调里启动嵌套 RunLoop；不能提前确认退出。
			loop.Dispatch()
			close(entered)
			<-release
		})
	})
	<-entered
	select {
	case <-returned:
		close(release)
		t.Fatal("原生观察尚在执行就确认退出")
	case <-time.After(20 * time.Millisecond):
	}
	close(release)
	select {
	case <-returned:
	case <-time.After(time.Second):
		t.Fatal("观察返回后未确认退出")
	}
}
