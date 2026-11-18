package main

import (
	"fmt"
	"time"

	"github.com/golang-gui/goui/platform"
)

func checkEventLoop(plat platform.Platform) {
	loop, err := plat.NewEventLoop()
	if err != nil {
		panic(err)
	}
	defer loop.Destroy()
	started, tasksRan, posted, stopped := make(chan struct{}), make(chan struct{}), make(chan struct{}), make(chan struct{})
	order := make([]int, 0, 2)
	loop.Post(func() { order = append(order, 1); close(started) })
	go func() {
		defer close(posted)
		select {
		case <-started:
		case <-stopped:
			return
		}
		loop.Post(func() { order = append(order, 2); close(tasksRan) })
		select {
		case <-tasksRan:
			loop.Quit()
		case <-stopped:
			return
		}
	}()
	watchdog := time.AfterFunc(5*time.Second, loop.Quit)
	loop.Run()
	watchdog.Stop()
	close(stopped)
	<-posted
	if len(order) != 2 || order[0] != 1 || order[1] != 2 {
		panic(fmt.Errorf("unexpected task order: %v", order))
	}
	loop.Destroy()
	ranAfterDestroy := false
	loop.Post(func() { ranAfterDestroy = true })
	loop.Run()
	if ranAfterDestroy {
		panic("task ran after destroy")
	}
}
