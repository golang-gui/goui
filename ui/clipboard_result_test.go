package ui

import (
	"errors"
	"runtime"
	"testing"
	"time"
)

type clipboardTestApplication struct {
	*windowTestApplication
	tasks chan func()
}

func (a *clipboardTestApplication) Post(fn func()) { a.tasks <- fn }

type clipboardTestNative struct {
	err error
}

func (c *clipboardTestNative) SetText(string) error { return c.err }
func (c *clipboardTestNative) RequestText(fn func(string, error)) {
	fn("", c.err)
}

func TestClipboardReportsDispatchAndReadResults(t *testing.T) {
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()
	a := newApp(newWindowTestApplication(), nil)
	defer a.stop()
	native := &clipboardTestNative{}
	c := Clipboard{current: a, clipboard: native}
	calls := 0
	for _, result := range []error{nil, ErrClipboardTimeout, errors.New("native failure")} {
		native.err = result
		if err := c.RequestText(func(text string, err error) {
			calls++
			if text != "" || !errors.Is(err, result) {
				t.Fatal("lost read result", err)
			}
		}); err != nil {
			t.Fatal("accepted read returned dispatch failure", err)
		}
		if err := c.SetText(""); !errors.Is(err, result) {
			t.Fatal("lost write result", err)
		}
	}
	if calls != 3 {
		t.Fatal("callback count", calls)
	}
	// Synchronous completion can quit before RequestText returns; this read was
	// still accepted and must not also report ErrAppStopped.
	if err := c.RequestText(func(string, error) { a.Quit(); calls++ }); err != nil {
		t.Fatal(err)
	}
	if err := c.RequestText(func(string, error) { calls++ }); !errors.Is(err, ErrAppStopped) {
		t.Fatal(err)
	}
	if calls != 4 || !errors.Is(c.SetText(""), ErrAppStopped) {
		t.Fatal("shutdown dispatched clipboard work")
	}
}

func TestClipboardReadRejectedWhenExitDiscardsDispatch(t *testing.T) {
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()
	nativeApp := &clipboardTestApplication{newWindowTestApplication(), make(chan func(), 1)}
	a := newApp(nativeApp, nil)
	defer a.stop()
	c := Clipboard{current: a, clipboard: &clipboardTestNative{}}
	result := make(chan error, 1)
	go func() { result <- c.RequestText(func(string, error) { t.Error("rejected request called back") }) }()
	var task func()
	select {
	case task = <-nativeApp.tasks:
	case <-time.After(time.Second):
		t.Fatal("read was not dispatched")
	}
	a.stop()
	select {
	case err := <-result:
		if !errors.Is(err, ErrAppStopped) {
			t.Fatal("lost dispatch rejection", err)
		}
	case <-time.After(time.Second):
		t.Fatal("read dispatch blocked after exit")
	}
	task() // A queued task surviving exit must not invoke the callback later.
}
