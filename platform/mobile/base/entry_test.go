package base

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestEntryOverlay(t *testing.T) {
	dir := t.TempDir()
	source := filepath.Join(dir, "generated.go")
	target := filepath.Join(dir, entryFileName)
	path := filepath.Join(dir, "overlay.json")
	write := func(replace map[string]string) {
		t.Helper()
		data, err := json.Marshal(struct{ Replace map[string]string }{replace})
		if err != nil {
			t.Fatal(err)
		}
		if err = os.WriteFile(path, data, 0600); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.WriteFile(source, []byte(entrySource), 0600); err != nil {
		t.Fatal(err)
	}
	write(map[string]string{target: source})
	got, err := ValidateEntryOverlay(path)
	if err != nil || got != dir {
		t.Fatalf("dir=%q error=%v", got, err)
	}
	for _, replace := range []map[string]string{
		{},
		{target: source, filepath.Join(dir, "runtime.go"): source},
		{filepath.Join(dir, "other.go"): source},
		{entryFileName: source},
		{target: "generated.go"},
	} {
		write(replace)
		if _, err = ValidateEntryOverlay(path); err == nil {
			t.Fatal("接受了非注册文件的 overlay")
		}
	}
	write(map[string]string{target: source})
	if err = os.WriteFile(source, []byte(strings.ReplaceAll(entrySource, "RegisterMain(main)", "RegisterMain(other)")), 0600); err != nil {
		t.Fatal(err)
	}
	if _, err = ValidateEntryOverlay(path); err == nil {
		t.Fatal("接受了修改过的注册源码")
	}
	os.WriteFile(source, []byte(entrySource), 0600)
	os.WriteFile(target, []byte("package main"), 0600)
	if _, err = ValidateEntryOverlay(path); err == nil {
		t.Fatal("覆盖了现有文件")
	}
}
