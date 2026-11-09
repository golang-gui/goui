package main

import (
	"archive/zip"
	"os"
	"path/filepath"
	"testing"
)

func TestZipPreservesNativeResources(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "app.zip")
	file, err := os.Create(path)
	if err != nil {
		t.Fatal(err)
	}
	writer := zip.NewWriter(file)
	entry, err := writer.Create("AndroidManifest.xml")
	if err != nil {
		t.Fatal(err)
	}
	entry.Write([]byte("manifest"))
	writer.Close()
	file.Close()
	library := filepath.Join(dir, "lib.so")
	os.WriteFile(library, []byte("native"), 0644)
	if err = addZipFiles(path, map[string]string{"lib/arm64-v8a/libgoui.so": library}); err != nil {
		t.Fatal(err)
	}
	reader, err := zip.OpenReader(path)
	if err != nil {
		t.Fatal(err)
	}
	defer reader.Close()
	if len(reader.File) != 2 || reader.File[0].Name != "AndroidManifest.xml" || reader.File[1].Name != "lib/arm64-v8a/libgoui.so" {
		t.Fatal("组包丢失资源或顺序异常")
	}
	if err = addZipFiles(path, map[string]string{"AndroidManifest.xml": library}); err == nil {
		t.Fatal("不应接受重复资源")
	}
}
func TestEnvironmentOverride(t *testing.T) {
	t.Setenv("GOUI_MOBILE_TOOL_TEST", "original")
	values := environment(map[string]string{"GOUI_MOBILE_TOOL_TEST": "replaced"})
	count := 0
	for _, value := range values {
		if value == "GOUI_MOBILE_TOOL_TEST=original" {
			t.Fatal("旧环境值残留")
		}
		if value == "GOUI_MOBILE_TOOL_TEST=replaced" {
			count++
		}
	}
	if count != 1 {
		t.Fatalf("覆盖后项数=%d", count)
	}
}
func TestShapingReference(t *testing.T) {
	const font = "/usr/share/fonts/truetype/dejavu/DejaVuSans.ttf"
	if _, err := os.Stat(font); err != nil {
		t.Skip("当前系统没有测试参考字体")
	}
	b := builder{config: configuration{Font: font}, root: t.TempDir()}
	if err := b.shapingReference(); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(b.root, "assets/shape.json")); err != nil {
		t.Fatal(err)
	}
}

// 固定归档与对象 recipe 有不同证据；不能因入口 Go 源码恢复普通编译而跳过消费检查。
func TestVerifyConsumerLog(t *testing.T) {
	for _, test := range []struct {
		name   string
		format int
		log    string
		valid  bool
	}{
		{"archive", 1, "# goui base: github.com/golang-gui/goui/platform/mobile/internal/entry", true},
		{"recipe", 2, "# goui recipe: runtime/cgo\n# goui recipe: github.com/golang-gui/goui/platform/mobile/internal/entry", true},
		{"wrong-format", 2, "# goui base: github.com/golang-gui/goui/platform/mobile/internal/entry", false},
		{"missing-runtime", 2, "# goui recipe: github.com/golang-gui/goui/platform/mobile/internal/entry", false},
		{"native-compiler", 2, "/cgo -objdir /tmp\n# goui recipe: runtime/cgo\n# goui recipe: github.com/golang-gui/goui/platform/mobile/internal/entry", false},
		{"unknown", 3, "", false},
	} {
		t.Run(test.name, func(t *testing.T) {
			if err := verifyConsumerLog([]byte(test.log), test.format); (err == nil) != test.valid {
				t.Fatalf("消费证据错误：%v", err)
			}
		})
	}
}
