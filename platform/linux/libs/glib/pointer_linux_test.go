package glib

import (
	"strings"
	"testing"

	"github.com/goexlib/cgo"
)

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
