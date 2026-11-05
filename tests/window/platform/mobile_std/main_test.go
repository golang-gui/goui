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
	if runtime.Version() != "go1.25.8" {
		t.Skip("官方源码与 testing 适配固定为 Go 1.25.8")
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
	if manifest.GoVersion != "go1.25.8" || count != 134 {
		t.Fatalf("标准用例清单变化: version=%s count=%d", manifest.GoVersion, count)
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
	if output := run(); !strings.Contains(output, "code=0 official=134 count=138") {
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
	if output := run(); !strings.Contains(output, "code=1 official=135 count=139") {
		t.Fatalf("失败用例被忽略: %s", output)
	}
}
