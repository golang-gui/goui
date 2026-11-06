package main

import (
	"bytes"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func TestGeneratedStandardTests(t *testing.T) {
	expected, ok := map[string]int{"go1.25.8": 134, "go1.26.10": 130, "go1.27.3": 130}[runtime.Version()]
	if !ok {
		t.Skip("当前 Go 版本尚未提供官方源码选择集")
	}
	root := filepath.Join(t.TempDir(), "app")
	if err := generate(root, "software"); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(filepath.Join(root, "official-sources.json"))
	if err != nil {
		t.Fatal(err)
	}
	var manifest struct {
		GoVersion string
		Sources   []sourceRecord
		Excluded  []sourceRecord
	}
	if err = json.Unmarshal(data, &manifest); err != nil {
		t.Fatal(err)
	}
	count := 0
	for _, record := range manifest.Sources {
		source, err := os.ReadFile(filepath.Join(runtime.GOROOT(), "src", record.Package, record.File))
		if err != nil {
			t.Fatal(err)
		}
		if record.SHA256 != fmt.Sprintf("%x", sha256.Sum256(source)) {
			t.Fatalf("来源摘要不一致: %s/%s", record.Package, record.File)
		}
		for i, suite := range suites {
			if suite.Package != record.Package {
				continue
			}
			copy, err := os.ReadFile(filepath.Join(root, "official", fmt.Sprintf("pkg%02d", i), strings.TrimSuffix(record.File, "_test.go")+"_official.go"))
			if err != nil {
				t.Fatal(err)
			}
			if !bytes.Equal(source, copy) {
				t.Fatalf("官方测试源文件被改写: %s/%s", record.Package, record.File)
			}
		}
		count += len(record.Tests)
	}
	if manifest.GoVersion != runtime.Version() || count != expected {
		t.Fatalf("标准用例清单变化: version=%s count=%d", manifest.GoVersion, count)
	}
	if runtime.Version() == "go1.25.8" && len(manifest.Excluded) != 0 || runtime.Version() != "go1.25.8" && len(manifest.Excluded) != 1 {
		t.Fatalf("整文件排除清单不符: %+v", manifest.Excluded)
	}
	for _, r := range manifest.Excluded {
		source, err := os.ReadFile(filepath.Join(runtime.GOROOT(), "src", r.Package, r.File))
		if err != nil || r.Package != "sync" || r.File != "waitgroup_test.go" || r.Excluded == "" || r.SHA256 != fmt.Sprintf("%x", sha256.Sum256(source)) {
			t.Fatalf("排除原因或原始摘要丢失: %+v, %v", r, err)
		}
	}
	if err = generate(root, "software"); err == nil {
		t.Fatal("生成器不能覆盖已存在的工程")
	}
	host := filepath.Join(root, "host")
	if err = os.Mkdir(host, 0755); err != nil {
		t.Fatal(err)
	}
	program := `package main
import("os";"fmt";"example.org/gouistd/suite")
func main(){r:=suite.Run(os.Args[1],func(string){});fmt.Printf("code=%d official=%d count=%d",r.Code,r.Official,len(r.Results))}
`
	if err = os.WriteFile(filepath.Join(host, "main.go"), []byte(program), 0644); err != nil {
		t.Fatal(err)
	}
	run := func() string {
		t.Helper()
		command := exec.Command("go", "run", "./host", t.TempDir())
		command.Dir = root
		command.Env = append(os.Environ(), "GOWORK=off", "GOFLAGS=", "GOTOOLCHAIN=local", "GOOS="+runtime.GOOS, "GOARCH="+runtime.GOARCH, "CGO_ENABLED=0")
		data, err := command.CombinedOutput()
		if err != nil {
			t.Fatalf("官方测试 runner: %v\n%s", err, data)
		}
		return string(data)
	}
	if output := run(); !strings.Contains(output, fmt.Sprintf("code=0 official=%d count=%d", expected, expected+4)) {
		t.Fatalf("没有运行完整选择集: %s", output)
	}
	// 临时工程中注入一个明确失败的测试，证明包装未吞掉 testing 的失败结果。
	registry := filepath.Join(root, "suite/cases.go")
	source, err := os.ReadFile(registry)
	if err != nil {
		t.Fatal(err)
	}
	negative := strings.Replace(string(source), "return tests", "tests=append(tests,testing.InternalTest{Name: \"intentional-failure\", F:func(t *testing.T){t.Error(\"expected failure\")}})\nreturn tests", 1)
	if err = os.WriteFile(registry, []byte(negative), 0644); err != nil {
		t.Fatal(err)
	}
	if output := run(); !strings.Contains(output, fmt.Sprintf("code=1 official=%d count=%d", expected+1, expected+5)) {
		t.Fatalf("失败用例被忽略: %s", output)
	}
}

func TestGeneratedNativeCgoTests(t *testing.T) {
	if runtime.Version() != "go1.25.8" && runtime.Version() != "go1.26.10" && runtime.Version() != "go1.27.3" {
		t.Skip("当前Go版本尚未提供官方源码选择集")
	}
	cc, err := exec.LookPath("cc")
	if err != nil {
		t.Skip("真实CGO测试需要宿主C编译器")
	}
	root := filepath.Join(t.TempDir(), "app")
	if err = generateWithCgo(root, "opengl", true); err != nil {
		t.Fatal(err)
	}
	if err = os.Mkdir(filepath.Join(root, "host"), 0755); err != nil {
		t.Fatal(err)
	}
	if err = os.WriteFile(filepath.Join(root, "host/main.go"), []byte(`package main
import("fmt";"os";"example.org/gouistd/suite")
func main(){r:=suite.Run(os.Args[1],func(string){});for _,c:=range r.Results{if c.Name=="goui-platform/TestUserNativeCGO"{fmt.Printf("native=%s code=%d",c.Status,r.Code)}}}
`), 0644); err != nil {
		t.Fatal(err)
	}
	for _, macro := range []string{"73", "74", ""} {
		command := exec.Command("go", "run", "./host", t.TempDir())
		command.Dir = root
		flags := "-O2 -g"
		if macro != "" {
			flags += " -DGOUI_NATIVE_SENTINEL=" + macro
		}
		command.Env = append(os.Environ(), "GOWORK=off", "GOTOOLCHAIN=local", "GOFLAGS=", "GOOS="+runtime.GOOS, "GOARCH="+runtime.GOARCH, "CGO_ENABLED=1", "CC="+cc, "CGO_CFLAGS="+flags)
		out, err := command.CombinedOutput()
		switch macro {
		case "73":
			if err != nil || !bytes.Contains(out, []byte("native=pass code=0")) {
				t.Fatalf("用户宏未生效: %v\n%s", err, out)
			}
		case "74":
			if err != nil || !bytes.Contains(out, []byte("native=fail code=1")) {
				t.Fatalf("错误宏未触发独立预期: %v\n%s", err, out)
			}
		default:
			if err == nil || !bytes.Contains(out, []byte("GOUI_NATIVE_SENTINEL must be supplied")) {
				t.Fatalf("缺少宏仍能通过构建: %v\n%s", err, out)
			}
		}
	}
}
