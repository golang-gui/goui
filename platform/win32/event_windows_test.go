package win32

import (
	"testing"
	"time"
)

func TestEventQueue(t *testing.T) {
	plat, err := NewPlatform()
	if err != nil {
		t.Fatal(err)
	}
	defer plat.Destroy()

	q, err := plat.NewEventQueue()
	if err != nil {
		t.Fatal(err)
	}
	defer q.Destroy()

	quit := false
	go func() {
		time.Sleep(3 * time.Second)
		q.Post()
		quit = true
	}()

	for !quit {
		q.Wait()
	}
}
