package cocoa

import (
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/golang-gui/goui/platform/darwin/frameworks"
	. "github.com/golang-gui/goui/platform/darwin/frameworks/appkit"
	. "github.com/golang-gui/goui/platform/darwin/frameworks/foundation"
	"golang.org/x/text/unicode/norm"
)

func TestFileDialogResponse(t *testing.T) {
	if err := fileDialogResponseError(NSModalResponseCancel); err != nil {
		t.Fatal(err)
	}
	// AppKit NSModalResponseAbort is -1001, not user cancellation (0).
	if err := fileDialogResponseError(NSModalResponse(-1001)); err == nil {
		t.Fatal("aborted native panel reported as user cancellation")
	}
}

// Foundation bookmarks do not require a desktop or an NSApplication. Native
// calls execute on this locked thread, with their own autorelease pools.
func TestFileAccessNativeBookmark(t *testing.T) {
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()
	if err := frameworks.Init(); err != nil {
		t.Fatal(err)
	}
	service, err := newFileDialog()
	if err != nil {
		t.Fatal(err)
	}
	fd := service.(*fileDialog)
	path := filepath.Join(t.TempDir(), "文件-é.txt")
	if err := os.WriteFile(path, []byte("bookmark contents"), 0600); err != nil {
		t.Fatal(err)
	}
	var access *fileAccess
	AutoReleasePool(func() { access, err = fd.newAccess(NSURLClassId.FileURLWithPath(ToNSString(path)), false) })
	if err != nil {
		t.Fatal(err)
	}
	credential, err := access.Credential()
	if err != nil {
		t.Fatal(err)
	}
	if err := access.Close(); err != nil {
		t.Fatal(err)
	}
	if err := access.Close(); err != nil {
		t.Fatal(err)
	}
	if _, err := access.LocalPath(); !errors.Is(err, os.ErrClosed) {
		t.Fatal(err)
	}
	if _, err := access.Credential(); !errors.Is(err, os.ErrClosed) {
		t.Fatal(err)
	}
	if norm.NFC.String(access.Name()) != filepath.Base(path) {
		t.Fatal(access.Name())
	}
	for i := range 30 {
		r, err := fd.RestoreAccess(credential)
		if err != nil {
			t.Fatalf("iteration %d: %v", i, err)
		}
		p, err := r.LocalPath()
		original, originalErr := os.Stat(path)
		restored, restoredErr := os.Stat(p)
		if err != nil || originalErr != nil || restoredErr != nil || !os.SameFile(original, restored) {
			t.Fatal(p, err)
		}
		data, err := os.ReadFile(p)
		if err != nil || string(data) != "bookmark contents" {
			t.Fatal(err)
		}
		credential, err = r.Credential()
		if err != nil {
			t.Fatal(err)
		}
		r.Close()
	}
	if _, err := fd.RestoreAccess([]byte("not a credential")); !errors.Is(err, os.ErrInvalid) {
		t.Fatal(err)
	}
	if err := os.Remove(path); err != nil {
		t.Fatal(err)
	}
	if r, err := fd.RestoreAccess(credential); err == nil {
		r.Close()
		t.Fatal("missing resource restored")
	}
}
