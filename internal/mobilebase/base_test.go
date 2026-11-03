package mobilebase

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestBaseIntegrity(t *testing.T) {
	root := t.TempDir()
	archive := filepath.Join(root, "support.a")
	if err := os.WriteFile(archive, []byte("native support"), 0600); err != nil {
		t.Fatal(err)
	}
	hash, err := baseHash(archive)
	if err != nil {
		t.Fatal(err)
	}
	valid := BaseManifest{Format: 1, GoVersion: baseGoVersion, GOOS: "android", GOARCH: "arm64", Architecture: "v8.0", BuildMode: "c-shared", Variant: "device", MinimumVersion: "29", Packages: map[string]BasePackage{"runtime/cgo": {Archive: "support.a", SHA256: hash, SourceSHA256: strings.Repeat("0", 64), BuildID: "action/content"}}}
	for _, test := range []struct {
		name string
		edit func(*BaseManifest)
		want string
	}{
		{"正确清单", func(*BaseManifest) {}, ""},
		{"版本不匹配", func(m *BaseManifest) { m.GoVersion = "go1.25.9" }, "unsupported"},
		{"目标不匹配", func(m *BaseManifest) { m.GOOS = "darwin" }, "unsupported"},
		{"缺少运行时", func(m *BaseManifest) { delete(m.Packages, "runtime/cgo") }, "incomplete"},
		{"越界路径", func(m *BaseManifest) {
			p := m.Packages["runtime/cgo"]
			p.Archive = "../support.a"
			m.Packages["runtime/cgo"] = p
		}, "invalid component path"},
		{"绝对路径", func(m *BaseManifest) {
			p := m.Packages["runtime/cgo"]
			p.Archive = archive
			m.Packages["runtime/cgo"] = p
		}, "invalid component path"},
		{"归档损坏", func(m *BaseManifest) {
			p := m.Packages["runtime/cgo"]
			p.SHA256 = strings.Repeat("0", 64)
			m.Packages["runtime/cgo"] = p
		}, "SHA-256 mismatch"},
	} {
		t.Run(test.name, func(t *testing.T) {
			data, _ := json.Marshal(valid)
			var m BaseManifest
			json.Unmarshal(data, &m)
			test.edit(&m)
			data, _ = json.Marshal(m)
			path := filepath.Join(root, "base.json")
			os.WriteFile(path, data, 0600)
			_, err := BaseOpen(path)
			if test.want == "" {
				if err != nil {
					t.Fatal(err)
				}
			} else if err == nil || !strings.Contains(err.Error(), test.want) {
				t.Fatalf("error=%v, want %s", err, test.want)
			}
		})
	}
	outside := filepath.Join(t.TempDir(), "outside.a")
	os.WriteFile(outside, []byte("native support"), 0600)
	if err = os.Symlink(outside, filepath.Join(root, "escape.a")); err == nil {
		if _, err = baseFile(root, "escape.a"); err == nil {
			t.Fatal("不应接受越界符号链接")
		}
	}
}

func TestSourceIdentity(t *testing.T) {
	roots := []string{t.TempDir(), t.TempDir()}
	for _, root := range roots {
		os.WriteFile(filepath.Join(root, "one.go"), []byte("package one"), 0600)
		os.WriteFile(filepath.Join(root, "two.h"), []byte("#define V 7"), 0600)
	}
	first, err := BaseSourceHash(roots[0], []string{"one.go", "two.h"})
	if err != nil {
		t.Fatal(err)
	}
	second, err := BaseSourceHash(roots[1], []string{"two.h", "one.go", "one.go"})
	if err != nil || first != second {
		t.Fatalf("源码身份不应依赖目录和枚举顺序: %s %s %v", first, second, err)
	}
	os.WriteFile(filepath.Join(roots[1], "two.h"), []byte("#define V 19"), 0600)
	second, _ = BaseSourceHash(roots[1], []string{"one.go", "two.h"})
	if first == second {
		t.Fatal("头文件变化必须使基座失效")
	}
	if _, err = BaseSourceHash(roots[0], []string{"../escape"}); err == nil {
		t.Fatal("不应接受包外输入")
	}
}

func TestExportIdentity(t *testing.T) {
	makeArchive := func(id string, payload []byte) string {
		data := append([]byte("go object android arm64 go1.25.8\nbuild id \""+id+"\"\n\n$$B\nu"), payload...)
		data = append(data, []byte("\n$$\n")...)
		header := fmt.Sprintf("%-16s%-12s%-6s%-6s%-8s%-10d`\n", "__.PKGDEF", "0", "0", "0", "644", len(data))
		if len(header) != 60 {
			t.Fatal(len(header))
		}
		path := filepath.Join(t.TempDir(), "pkg.a")
		os.WriteFile(path, append([]byte("!<arch>\n"+header), data...), 0600)
		return path
	}
	first, err := BaseExportHash(makeArchive("host-a", []byte("ABI")))
	if err != nil {
		t.Fatal(err)
	}
	second, err := BaseExportHash(makeArchive("host-b", []byte("ABI")))
	if err != nil || first != second {
		t.Fatalf("构建身份不是导出 ABI: %v", err)
	}
	changed, _ := BaseExportHash(makeArchive("host-a", []byte("changed ABI")))
	if changed == first {
		t.Fatal("导出数据改变必须使 ABI 身份变化")
	}
	broken := filepath.Join(t.TempDir(), "broken.a")
	os.WriteFile(broken, bytes.Repeat([]byte("x"), 100), 0600)
	if _, err = BaseExportHash(broken); err == nil {
		t.Fatal("不应接受非 Go 归档")
	}
}
