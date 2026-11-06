package base

import (
	"encoding/binary"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestObjectRecipeIntegrity(t *testing.T) {
	root := t.TempDir()
	write := func(name string, data []byte) BaseFile {
		t.Helper()
		hash := recipeDigest(data)
		path := filepath.Join(root, hash)
		if err := os.WriteFile(path, data, 0600); err != nil {
			t.Fatal(err)
		}
		return BaseFile{Name: name, Path: hash, SHA256: hash}
	}
	object := make([]byte, 64)
	copy(object, "\x7fELF")
	object[4], object[5] = 2, 1
	binary.LittleEndian.PutUint16(object[16:], 1)
	binary.LittleEndian.PutUint16(object[18:], 183)
	p := BasePackage{SourceSHA256: strings.Repeat("a", 64), GeneratedGo: []BaseFile{write("_cgo_gotypes.go", []byte("package cgo\n"))}, Objects: []BaseFile{write("_x001.o", object)}}
	m := BaseManifest{Format: 2, GoVersion: baseGoVersion, GOOS: "android", GOARCH: "arm64", Architecture: "v8.0", BuildMode: "c-shared", Variant: "device", MinimumVersion: "29", Packages: map[string]BasePackage{"runtime/cgo": p}}
	m.CgoFlags = map[string]string{"CGO_CFLAGS": "-O2 -g", "CGO_CPPFLAGS": "", "CGO_CXXFLAGS": "-O2 -g", "CGO_FFLAGS": "-O2 -g", "CGO_LDFLAGS": "-O2 -g"}
	m.ProducerTools = map[string]string{"cgo": strings.Repeat("a", 64), "CC": strings.Repeat("b", 64), "CXX": strings.Repeat("c", 64)}
	manifest := filepath.Join(root, "base.json")
	check := func(want string) {
		t.Helper()
		b, err := json.Marshal(m)
		if err != nil {
			t.Fatal(err)
		}
		if err = os.WriteFile(manifest, b, 0600); err != nil {
			t.Fatal(err)
		}
		_, err = BaseOpen(manifest)
		if want == "" && err != nil || want != "" && (err == nil || !strings.Contains(err.Error(), want)) {
			t.Fatalf("want %q, got %v", want, err)
		}
	}
	check("")
	delete(m.CgoFlags, "CGO_CPPFLAGS")
	check("missing producer flag")
	m.CgoFlags["CGO_CPPFLAGS"] = ""
	m.ProducerTools["CC"] = "invalid"
	check("invalid producer tool")
	m.ProducerTools["CC"] = strings.Repeat("b", 64)
	// 摘要必须检测到缓存源码被修改。
	os.WriteFile(filepath.Join(root, p.GeneratedGo[0].Path), []byte("package changed\n"), 0600)
	check("checksum mismatch")
	p.GeneratedGo[0] = write("_cgo_gotypes.go", []byte("package cgo\n"))
	// 即使同步更新摘要，也不能把其它架构或 Go 对象冒充原生支持对象。
	binary.LittleEndian.PutUint16(object[18:], 62)
	p.Objects[0] = write("_x001.o", object)
	m.Packages["runtime/cgo"] = p
	check("target mismatch")
	p.Objects[0] = write("_x001.o", []byte("go object android arm64 go1.25.8\n"))
	m.Packages["runtime/cgo"] = p
	check("file kind")
	binary.LittleEndian.PutUint16(object[18:], 183)
	p.Objects[0] = write("..", object)
	m.Packages["runtime/cgo"] = p
	check("invalid recipe file")
}
