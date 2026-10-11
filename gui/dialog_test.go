package gui

import (
	"errors"
	"testing"

	"github.com/golang-gui/goui/platform"
)

type dialogTestNative struct {
	platform.FileDialog
	result []platform.FileAccess
	err    error
}

func (d *dialogTestNative) OpenFile(_ platform.Window, _ platform.DialogOptions, cb func([]platform.FileAccess, error)) {
	cb(d.result, d.err)
}
func (d *dialogTestNative) OpenDirectory(w platform.Window, o platform.DialogOptions, cb func([]platform.FileAccess, error)) {
	d.OpenFile(w, o, cb)
}
func (d *dialogTestNative) SaveFile(w platform.Window, o platform.DialogOptions, cb func([]platform.FileAccess, error)) {
	d.OpenFile(w, o, cb)
}
func (d *dialogTestNative) RestoreAccess([]byte) (platform.FileAccess, error) { return nil, d.err }

func TestFileDialogResultsAndErrors(t *testing.T) {
	failure := errors.New("native failure")
	for _, native := range []*dialogTestNative{{}, {err: failure}} {
		loop := &settingsTestLoop{posts: make(chan func(), 1)}
		dialog := newFileDialog(native, loop, nil)
		for _, call := range []func(Window, DialogOptions, func([]FileAccess, error)){dialog.OpenFile, dialog.OpenDirectory, dialog.SaveFile} {
			count := 0
			call(nil, DialogOptions{}, func(result []FileAccess, err error) {
				count++
				if len(result) != 0 || err != native.err {
					t.Fatalf("wrong result: %v %v", result, err)
				}
			})
			if count != 0 {
				t.Fatal("callback bypassed event-loop queue")
			}
			(<-loop.posts)()
			if count != 1 {
				t.Fatal("callback not delivered exactly once")
			}
		}
		if _, err := dialog.RestoreAccess(nil); err != native.err {
			t.Fatal(err)
		}
	}
	for _, creationErr := range []error{failure, nil} {
		loop := &settingsTestLoop{posts: make(chan func(), 1)}
		dialog := newFileDialog(nil, loop, creationErr)
		want := creationErr
		if want == nil {
			want = platform.ErrUnsupported
		}
		for _, call := range []func(Window, DialogOptions, func([]FileAccess, error)){dialog.OpenFile, dialog.OpenDirectory, dialog.SaveFile} {
			call(nil, DialogOptions{}, func(result []FileAccess, err error) {
				if len(result) != 0 || err != want {
					t.Fatalf("creation failure swallowed: %v %v", result, err)
				}
			})
			(<-loop.posts)()
		}
		if _, err := dialog.RestoreAccess(nil); err != want {
			t.Fatal(err)
		}
	}
}
