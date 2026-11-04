package mobile

import (
	"reflect"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

func TestEntryPoint(t *testing.T) {
	var entry entryPoint
	if entry.start() {
		t.Fatal("未注册入口不应启动")
	}
	var calls atomic.Int32
	entered, release, returned := make(chan struct{}), make(chan struct{}), make(chan struct{})
	entry.register(func() {
		calls.Add(1)
		close(entered)
		<-release
		close(returned)
	})
	defer close(release)
	var callers sync.WaitGroup
	for range 64 {
		callers.Add(1)
		go func() {
			defer callers.Done()
			if !entry.start() {
				t.Error("已注册的入口启动失败")
			}
		}()
	}
	accepted := make(chan struct{})
	go func() { callers.Wait(); close(accepted) }()
	select {
	case <-accepted:
	case <-time.After(5 * time.Second):
		t.Fatal("启动入口等待了阻塞的 main")
	}
	select {
	case <-entered:
	case <-time.After(5 * time.Second):
		t.Fatal("main 没有运行")
	}
	if calls.Load() != 1 {
		t.Fatalf("main 执行 %d 次", calls.Load())
	}
	select {
	case <-returned:
		t.Fatal("main 不应提前返回")
	default:
	}
}

func TestEntryRegistration(t *testing.T) {
	for _, duplicate := range []bool{false, true} {
		t.Run(map[bool]string{false: "空函数", true: "重复注册"}[duplicate], func(t *testing.T) {
			var entry entryPoint
			defer func() {
				if recover() == nil {
					t.Fatal("无效注册应 panic")
				}
			}()
			if duplicate {
				entry.register(func() {})
				entry.register(func() {})
			} else {
				entry.register(nil)
			}
		})
	}
}

func TestEntryReturnedMain(t *testing.T) {
	var entry entryPoint
	done := make(chan struct{})
	entry.register(func() { close(done) })
	if !entry.start() {
		t.Fatal("启动失败")
	}
	select {
	case <-done:
	case <-time.After(5 * time.Second):
		t.Fatal("main 没有返回")
	}
	for range 64 {
		if !entry.start() {
			t.Fatal("返回后重复请求应仍然成功")
		}
	}
}

func TestEventLoopOrderAndQuit(t *testing.T) {
	loop := NewEventLoop()
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
	loop := NewEventLoop()
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
	loop := NewEventLoop()
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
