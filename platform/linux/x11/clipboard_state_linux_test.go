package x11

import (
	"errors"
	"testing"
	"time"

	"github.com/golang-gui/goui/platform/common"
	"github.com/golang-gui/goui/platform/linux/libs/xlib"
)

func TestClipboardReadDeadlineAndReentry(t *testing.T) {
	now := time.Now()
	c := newClipboard()
	if c.pollTimeout(now) != -1 {
		t.Fatal("idle clipboard has a deadline")
	}
	calls := 0
	next := &clipboardRead{deadline: now.Add(time.Second), callback: func(string, error) {}}
	c.pending = &clipboardRead{deadline: now.Add(time.Microsecond), callback: func(text string, err error) {
		calls++
		if text != "" || !errors.Is(err, common.ErrClipboardTimeout) || c.pending != nil {
			t.Fatal("timeout did not release read state before callback", text, err)
		}
		c.pending = next // completion must not erase a reentrant request.
	}}
	if c.pollTimeout(now) != 1 {
		t.Fatal("sub-millisecond deadline was truncated")
	}
	c.expire(now)
	if calls != 0 {
		t.Fatal("read expired early")
	}
	c.expire(now.Add(time.Millisecond))
	c.expire(now.Add(time.Millisecond))
	if calls != 1 || c.pending != next {
		t.Fatal("duplicate completion or lost reentrant request")
	}
}

func TestClipboardCloseCompletesOnceAndRejectsReentry(t *testing.T) {
	c := newClipboard()
	calls := 0
	c.pending = &clipboardRead{callback: func(_ string, err error) {
		calls++
		if !errors.Is(err, common.ErrUnavailable) {
			t.Fatal(err)
		}
		c.RequestText(func(_ string, err error) {
			calls++
			if !errors.Is(err, common.ErrUnavailable) {
				t.Fatal("closed clipboard accepted reentrant read", err)
			}
		})
	}}
	c.close()
	c.close()
	if calls != 2 || c.pending != nil || c.SetText("closed") == nil {
		t.Fatal("close did not settle/reject operations", calls)
	}
}

func TestClipboardIgnoresUnrelatedAndLateReplies(t *testing.T) {
	previous := platform
	platform = &Platform{}
	defer func() { platform = previous }()
	platform.atoms.CLIPBOARD = 10
	c := newClipboard()
	r := &clipboardRead{requestor: 20, target: 30, callback: func(string, error) { t.Fatal("unrelated reply completed read") }}
	c.pending = r
	for _, ev := range []xlib.SelectionEvent{
		{Requestor: 19, Selection: 10, Target: 30},
		{Requestor: 20, Selection: 11, Target: 30},
		{Requestor: 20, Selection: 10, Target: 31},
	} {
		c.handleSelectionNotify(&ev)
		if c.pending != r {
			t.Fatal("unrelated reply erased read")
		}
	}
	c.pending = nil
	c.handleSelectionNotify(&xlib.SelectionEvent{Requestor: 20, Selection: 10, Target: 30})
}

func TestClipboardLoopExitSettlesRead(t *testing.T) {
	previous := platform
	platform = &Platform{}
	defer func() { platform = previous }()
	l := &EventLoop{pipe: [2]int32{-1, -1}}
	c := newClipboard()
	platform.eventLoop, platform.clipboard = l, c
	calls := 0
	c.pending = &clipboardRead{callback: func(_ string, err error) {
		calls++
		if !errors.Is(err, common.ErrUnavailable) {
			t.Fatal("lost loop-exit error", err)
		}
		c.RequestText(func(_ string, err error) {
			if !errors.Is(err, common.ErrUnavailable) {
				t.Fatal("destroyed loop accepted another read")
			}
		})
	}}
	l.Destroy()
	l.Destroy()
	if calls != 1 {
		t.Fatal("loop exit completion count", calls)
	}
}

func TestClipboardLatin1(t *testing.T) {
	if data, ok := clipboardLatin1("é\n"); !ok || string(data) != "\xe9\n" {
		t.Fatal("STRING must use Latin-1", data, ok)
	}
	if _, ok := clipboardLatin1("中文"); ok {
		t.Fatal("advertised an unrepresentable STRING")
	}
}
