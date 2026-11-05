package mobileentry

import (
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
