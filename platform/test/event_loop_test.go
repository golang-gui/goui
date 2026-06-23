package test

import (
	"testing"

	"github.com/golang-gui/goui/platform"
)

func TestEventLoop(t *testing.T) {
	skipWithoutDisplay(t)

	plat, err := getPlatform()
	if err != nil {
		t.Fatal(err)
	}

	var loop platform.EventLoop
	runOnMainThread(func() {
		loop, err = plat.NewEventLoop()
	})
	if loop != nil {
		defer func() {
			runOnMainThread(loop.Destroy)
		}()
	}
	if err != nil {
		t.Fatal(err)
	}

	started := make(chan struct{})
	tasksRan := make(chan struct{})
	posted := make(chan struct{})
	order := make([]int, 0, 2)

	loop.Post(func() {
		order = append(order, 1)
		close(started)
	})
	go func() {
		defer close(posted)
		<-started
		loop.Post(func() {
			order = append(order, 2)
			close(tasksRan)
		})
		<-tasksRan
		loop.Quit()
	}()

	runOnMainThread(loop.Run)
	<-posted

	if len(order) != 2 || order[0] != 1 || order[1] != 2 {
		t.Fatalf("unexpected task order: %v", order)
	}

	runOnMainThread(loop.Destroy)
	ranAfterDestroy := false
	loop.Post(func() {
		ranAfterDestroy = true
	})
	runOnMainThread(loop.Run)
	if ranAfterDestroy {
		t.Fatal("task ran after destroy")
	}
}
