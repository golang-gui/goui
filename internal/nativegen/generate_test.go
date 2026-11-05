package nativegen

import (
	"os"
	"path/filepath"
	"testing"
)

func TestWrite(t *testing.T) {
	root := t.TempDir()
	if err := Write(root, "fixture", map[string]string{"native/probe.c": "int probe(void){return 42;}\n"}); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(root, "fixture/native/probe.c")
	data, err := os.ReadFile(path)
	if err != nil || string(data) != "int probe(void){return 42;}\n" {
		t.Fatalf("生成内容错误: %q, %v", data, err)
	}
	if err := Write(root, "fixture", map[string]string{"../escape.c": "bad", "other.c": "bad"}); err == nil {
		t.Fatal("不得接受逃逸路径")
	}
	if _, err := os.Stat(filepath.Join(root, "fixture/other.c")); !os.IsNotExist(err) {
		t.Fatal("无效批次不应生成部分文件")
	}
}

func TestOutputOutsideRepository(t *testing.T) {
	root := t.TempDir()
	repo := filepath.Join(root, "repo")
	if err := os.Mkdir(repo, 0755); err != nil {
		t.Fatal(err)
	}
	for _, path := range []string{repo, filepath.Join(repo, "new/native")} {
		if err := CheckOutput(path, repo); err == nil {
			t.Fatalf("接受仓库内输出: %s", path)
		}
	}
	link := filepath.Join(root, "link")
	if err := os.Symlink(repo, link); err == nil {
		if err := CheckOutput(filepath.Join(link, "new"), repo); err == nil {
			t.Fatal("符号链接绕过输出检查")
		}
	}
	if err := CheckOutput(filepath.Join(root, "artifacts/generated"), repo); err != nil {
		t.Fatal(err)
	}
}
