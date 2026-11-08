package ui

import (
	"errors"
	"os"
	"runtime"
	"testing"
	"time"

	"github.com/golang-gui/goui/gui"
)

type dialogTestAccess struct {
	closed bool
	calls  int
}

type dialogTestApplication struct {
	*windowTestApplication
	tasks chan func()
}

func (a *dialogTestApplication) Post(fn func()) { a.tasks <- fn }

func (a *dialogTestAccess) Name() string { return "selected.txt" }
func (a *dialogTestAccess) LocalPath() (string, error) {
	a.calls++
	if a.closed {
		return "", os.ErrClosed
	}
	return "/selected.txt", nil
}
func (a *dialogTestAccess) Credential() ([]byte, error) {
	a.calls++
	if a.closed {
		return nil, os.ErrClosed
	}
	return []byte("credential"), nil
}
func (a *dialogTestAccess) Close() error { a.calls++; a.closed = true; return nil }

type dialogTestService struct {
	gui.FileDialog
	access gui.FileAccess
	owner  gui.Surface
	calls  int
	err    error
}

func (d *dialogTestService) OpenFile(owner gui.Surface, _ gui.DialogOptions, cb func([]gui.FileAccess, error)) {
	d.owner = owner
	d.calls++
	if d.access == nil {
		cb(nil, d.err)
	} else {
		cb([]gui.FileAccess{d.access}, d.err)
	}
}
func (d *dialogTestService) RestoreAccess([]byte) (gui.FileAccess, error) { return d.access, d.err }

func TestFileDialogErrorsAndOwnership(t *testing.T) {
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()
	a := newApp(newWindowTestApplication(), nil)
	defer a.stop()
	failure := errors.New("picker failed")
	d := FileDialog{current: a, dlg: &dialogTestService{err: failure}}
	d.OpenFile("", DialogOptions{}, func(access []FileAccess, err error) {
		if len(access) != 0 || err != failure {
			t.Fatal("error swallowed", err)
		}
	})
	if _, err := d.RestoreAccess(nil); err != failure {
		t.Fatal(err)
	}
	native := &dialogTestAccess{}
	d.dlg = &dialogTestService{access: native}
	var picked FileAccess
	d.OpenFile("", DialogOptions{}, func(access []FileAccess, err error) {
		if err != nil || len(access) != 1 {
			t.Fatal(err)
		}
		picked = access[0]
	})
	if native.closed {
		t.Fatal("picker prematurely released caller-owned access")
	}
	restored, err := d.RestoreAccess(nil)
	if err != nil || restored.Name() != "selected.txt" {
		t.Fatal(err)
	}
	if err := picked.Close(); err != nil {
		t.Fatal(err)
	}
	if _, err := restored.LocalPath(); !errors.Is(err, os.ErrClosed) {
		t.Fatal(err)
	}
	if picked.Name() != "selected.txt" {
		t.Fatal("name lost")
	}
}

func TestFileAccessWorkerAndStoppedApplication(t *testing.T) {
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()
	queued := &dialogTestApplication{newWindowTestApplication(), make(chan func(), 1)}
	a := newApp(queued, nil)
	native := &dialogTestAccess{}
	f := FileAccess{current: a, access: native, name: native.Name()}
	operations := []func() error{
		func() error { _, err := f.LocalPath(); return err },
		func() error { _, err := f.Credential(); return err },
		f.Close,
	}
	for i, operation := range operations {
		done := make(chan error, 1)
		go func() { done <- operation() }()
		select {
		case task := <-queued.tasks:
			if native.calls != i {
				t.Fatal("native operation ran on worker")
			}
			task()
		case <-time.After(time.Second):
			t.Fatal("no UI task")
		}
		select {
		case err := <-done:
			if err != nil {
				t.Fatal(err)
			}
		case <-time.After(time.Second):
			t.Fatal("worker did not complete")
		}
	}
	a.stop()
	for _, operation := range operations {
		if !errors.Is(operation(), ErrAppStopped) {
			t.Fatal("stopped operation reported success")
		}
	}
	if native.calls != len(operations) {
		t.Fatal("native calls after stop")
	}
}

func TestFileDialogResolvesOwnerAndReportsStoppedApp(t *testing.T) {
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()
	a := newApp(newWindowTestApplication(), nil)
	native := &dialogTestService{}
	d := FileDialog{current: a, dlg: native}
	w := newTestWindow()
	a.windows["main"] = &windowMount{window: w, root: newRoot()}
	count := 0
	if err := d.OpenFile("main", DialogOptions{}, func([]FileAccess, error) { count++ }); err != nil {
		t.Fatal(err)
	}
	if count != 1 || native.owner != w {
		t.Fatal("窗口 ID 未解析成相应实例")
	}
	if err := d.OpenFile("missing", DialogOptions{}, func(_ []FileAccess, err error) {
		count++
		if err == nil {
			t.Fatal("缺失窗口被降级为独立对话框")
		}
	}); err != nil {
		t.Fatal(err)
	}
	if native.calls != 1 || count != 2 {
		t.Fatal("缺失窗口仍调用了原生操作")
	}
	if err := d.OpenFile("", DialogOptions{}, func([]FileAccess, error) {}); err != nil || native.owner != nil {
		t.Fatal("空 ID 应明确表示无 owner")
	}
	a.stop()
	if err := d.OpenFile("main", DialogOptions{}, func([]FileAccess, error) { t.Fatal("停止后不得回调") }); !errors.Is(err, ErrAppStopped) {
		t.Fatal(err)
	}
}

func TestFileDialogReleasesAccessWhenUIStops(t *testing.T) {
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()
	a := newApp(newWindowTestApplication(), nil)
	f := FileDialog{current: a}
	result := &dialogTestAccess{}
	callback := f.callback(func([]FileAccess, error) { t.Fatal("停止后调用业务回调") })
	a.stop()
	callback([]gui.FileAccess{result}, nil)
	if !result.closed {
		t.Fatal("UI 停止后无法通过 UI FileAccess.Close 释放资源，应直接释放原生访问")
	}
}
