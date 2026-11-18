package glib

import (
	"runtime"
	"strings"
	"testing"
	"unsafe"

	"github.com/goexlib/cgo"
)

func TestMissingLaunchBinding(t *testing.T) {
	saved := gAppInfoLaunchDefaultForURI
	gAppInfoLaunchDefaultForURI = cgo.NewLazyLibrary("libgoui-missing-launch-test.so").NewSymbol("missing")
	defer func() { gAppInfoLaunchDefaultForURI = saved }()
	if err := AppInfoLaunchDefaultForURI("custom:test", 0); err == nil {
		t.Fatal("launch hid unavailable binding")
	}
}

func TestGErrorLayout(t *testing.T) {
	var err GError
	if unsafe.Offsetof(err.Code) != 4 || unsafe.Offsetof(err.Message) != 8 ||
		unsafe.Sizeof(err) != 8+unsafe.Sizeof(uintptr(0)) {
		t.Fatal("GError ABI mismatch")
	}
}

func TestFilenameToURI(t *testing.T) {
	for _, test := range []struct{ path, uri string }{
		{"/tmp/a b#%.txt", "file:///tmp/a%20b%23%25.txt"},
		{"/tmp/中文", "file:///tmp/%E4%B8%AD%E6%96%87"},
		{"/tmp/raw\xff", "file:///tmp/raw%FF"},
	} {
		got, err := FilenameToURI(test.path)
		if err != nil || got != test.uri {
			t.Fatalf("%q: %q, %v", test.path, got, err)
		}
	}
}

func TestFilenameToURIErrorOwnsDiagnostic(t *testing.T) {
	// 相对路径会产生真实 GError，不会启动外部应用。
	_, err := FilenameToURI("relative-path")
	if err == nil || !strings.Contains(err.Error(), "relative-path") {
		t.Fatalf("expected native error: %v", err)
	}
	message := err.Error()
	for range 100 {
		if _, next := FilenameToURI("another-relative-path"); next == nil {
			t.Fatal("missing native error")
		}
	}
	runtime.GC()
	if err.Error() != message {
		t.Fatal("error retained GLib-owned storage")
	}
}

func TestFilenamePointerResult(t *testing.T) {
	got, err := FilenameToURI("/tmp/a b#%.txt")
	if err != nil || got != "file:///tmp/a%20b%23%25.txt" {
		t.Fatalf("原生字符串结果：%q, %v", got, err)
	}
	_, err = FilenameToURI("relative-path")
	if err == nil || !strings.Contains(err.Error(), "relative-path") {
		t.Fatalf("应保留 GError 诊断：%v", err)
	}
}

func TestMissingFilenameToURIBinding(t *testing.T) {
	saved := gFilenameToURI
	gFilenameToURI = cgo.NewLazyLibrary("libgoui-missing-uri-test.so").NewSymbol("missing")
	defer func() { gFilenameToURI = saved }()
	if _, err := FilenameToURI("/tmp/example.txt"); err == nil {
		t.Fatal("缺失原生符号时应返回错误")
	}
}
