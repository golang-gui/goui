package mobile

import (
	"reflect"
	"sync"
	"testing"
)

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
