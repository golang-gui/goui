package mobilebase

import (
	"bytes"
	"context"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"
)

// TestColdBuild 需要 Linux amd64、Go 1.25.8、cc、ld 和 Python3，无需桌面。
// 维护者阶段允许 C 编译；消费阶段使用独立空缓存、拒绝编译器和仅链接驱动。
// 两版普通 Go 业务包共用相同基座，由独立 C 宿主验证回调、GC 和栈增长。
func TestColdBuild(t *testing.T) {
	if runtime.GOOS != "linux" || runtime.GOARCH != "amd64" || runtime.Version() != baseGoVersion {
		t.Skip("本验收针对 Linux amd64 / Go 1.25.8；其它平台需运行对应基座验收")
	}
	for _, tool := range []string{"cc", "ld", "python3"} {
		if _, err := exec.LookPath(tool); err != nil {
			t.Skipf("维护者工具不可用: %s", tool)
		}
	}
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Minute)
	defer cancel()
	root := t.TempDir()
	source := filepath.Join(root, "src")
	if err := os.CopyFS(source, os.DirFS("testdata/probe")); err != nil {
		t.Fatal(err)
	}
	driver := filepath.Join(root, "go-base")
	var log bytes.Buffer
	if err := BuildDriver(ctx, "go", driver, &log); err != nil {
		t.Fatalf("driver: %v\n%s", err, &log)
	}
	// 以明确配置制作组件，避免继承用户的工作区和交叉编译设置。
	for key, value := range map[string]string{"GOOS": "linux", "GOARCH": "amd64", "GOAMD64": "v1", "GOEXPERIMENT": "", "CGO_ENABLED": "1", "GOFLAGS": "", "GOUI_BASE": "", "GOWORK": "off", "GOTOOLCHAIN": "local"} {
		t.Setenv(key, value)
	}
	component := filepath.Join(root, "component")
	if err := Export(ctx, ExportOptions{GoCommand: "go", Directory: source, Output: component, BuildMode: "c-shared", Variant: "test", MinimumVersion: "glibc"}, &log); err != nil {
		t.Fatalf("export: %v\n%s", err, &log)
	}
	manifest := filepath.Join(component, "base.json")
	m, err := BaseOpen(manifest)
	if err != nil {
		t.Fatal(err)
	}
	if len(m.Packages) != 2 {
		t.Fatalf("基座只能包含运行时和固定入口，实际 %v", m.Packages)
	}
	before, err := os.ReadFile(manifest)
	if err != nil {
		t.Fatal(err)
	}
	host := filepath.Join(root, "host")
	command := exec.CommandContext(ctx, "cc", "testdata/host.c", "-o", host, "-ldl", "-pthread")
	if output, err := command.CombinedOutput(); err != nil {
		t.Fatalf("host: %v\n%s", err, output)
	}
	denyLog := filepath.Join(root, "compiler.log")
	deny := filepath.Join(root, "deny-cc")
	// 固定脚本通过环境记录调用，路径不进入 shell 文本。
	if err = os.WriteFile(deny, []byte("#!/bin/sh\nprintf '%s\\n' called >> \"$GOUI_TEST_CC_LOG\"\nexit 99\n"), 0700); err != nil {
		t.Fatal(err)
	}
	linkDriver := filepath.Join(root, "link-driver")
	data, err := os.ReadFile("testdata/link.py")
	if err != nil {
		t.Fatal(err)
	}
	if err = os.WriteFile(linkDriver, data, 0700); err != nil {
		t.Fatal(err)
	}
	if err = os.WriteFile(filepath.Join(root, "gcc"), []byte("#!/bin/sh\nexit 98\n"), 0700); err != nil {
		t.Fatal(err)
	}
	build := func(label, base string, extra ...string) ([]byte, error) {
		t.Helper()
		cache := filepath.Join(root, "cache-"+label)
		if err := os.Mkdir(cache, 0700); err != nil {
			t.Fatal(err)
		}
		args := []string{"build", "-x", "-trimpath", "-pgo=off", "-buildmode=c-shared", "-ldflags=-linkmode=external -extld=" + linkDriver, "-o", filepath.Join(root, label+".so")}
		args = append(args, extra...)
		args = append(args, ".")
		cmd := exec.CommandContext(ctx, driver, args...)
		cmd.Dir = source
		cmd.Env = append(os.Environ(), "GOCACHE="+cache, "GOUI_BASE="+base, "CC="+deny, "CXX="+deny, "GOUI_TEST_CC_LOG="+denyLog, "PATH="+root)
		return cmd.CombinedOutput()
	}
	for _, version := range []string{"7", "19"} {
		if version == "19" {
			path := filepath.Join(source, "logic/logic.go")
			text, err := os.ReadFile(path)
			if err != nil {
				t.Fatal(err)
			}
			if err = os.WriteFile(path, bytes.Replace(text, []byte("Version = 7"), []byte("Version = 19"), 1), 0600); err != nil {
				t.Fatal(err)
			}
		}
		output, err := build(version, manifest)
		if err != nil {
			t.Fatalf("cold build %s: %v\n%s", version, err, output)
		}
		text := string(output)
		if strings.Contains(text, "/cgo -objdir") || !strings.Contains(text, "/compile ") || !strings.Contains(text, "# goui base: runtime/cgo") || !strings.Contains(text, "logic/logic.go") {
			t.Fatalf("未经过预期源码编译/基座接入路径:\n%s", output)
		}
		output, err = exec.CommandContext(ctx, host, filepath.Join(root, version+".so"), version).CombinedOutput()
		if err != nil {
			t.Fatalf("runtime %s: %v\n%s", version, err, output)
		}
		t.Logf("版本 %s: %s", version, output)
	}
	// 清单始终不变，普通 Go 源码的更新不需要重新制作基座。
	after, _ := os.ReadFile(manifest)
	if !bytes.Equal(before, after) {
		t.Fatal("消费不能修改组件")
	}
	if _, err = BaseOpen(manifest); err != nil {
		t.Fatal(err)
	}
	// 使用全新的缓存检查失败路径，不能因上一轮命中缓存掩盖错误。
	checkFailure := func(label, base, want string, extra ...string) {
		t.Helper()
		output, err := build(label, base, extra...)
		if err == nil || !strings.Contains(string(output), want) {
			t.Fatalf("%s: error=%v, want=%q\n%s", label, err, want, output)
		}
	}
	checkFailure("flags", manifest, "unsupported build flags", "-gcflags=example.com/baseprobe/bridge=-N")
	contract := filepath.Join(source, "contract/contract.go")
	contractSource, err := os.ReadFile(contract)
	if err != nil {
		t.Fatal(err)
	}
	if err = os.WriteFile(contract, bytes.Replace(contractSource, []byte("ABI = 1"), []byte("ABI = 2"), 1), 0600); err != nil {
		t.Fatal(err)
	}
	checkFailure("dependency", manifest, "dependency ABI mismatch: example.com/baseprobe/contract")
	if err = os.WriteFile(contract, contractSource, 0600); err != nil {
		t.Fatal(err)
	}
	moduleFile := filepath.Join(source, "go.mod")
	moduleSource, err := os.ReadFile(moduleFile)
	if err != nil {
		t.Fatal(err)
	}
	if err = os.WriteFile(moduleFile, bytes.Replace(moduleSource, []byte("go 1.25"), []byte("go 1.24"), 1), 0600); err != nil {
		t.Fatal(err)
	}
	checkFailure("module", manifest, "module identity mismatch")
	if err = os.WriteFile(moduleFile, moduleSource, 0600); err != nil {
		t.Fatal(err)
	}
	delete(m.Packages, "example.com/baseprobe/bridge")
	// BaseOpen 返回绝对路径，保存回清单时恢复组件相对路径。
	for name, p := range m.Packages {
		p.Archive, err = filepath.Rel(component, p.Archive)
		if err != nil {
			t.Fatal(err)
		}
		m.Packages[name] = p
	}
	changed, _ := json.Marshal(m)
	missing := filepath.Join(component, "missing.json")
	if err = os.WriteFile(missing, changed, 0600); err != nil {
		t.Fatal(err)
	}
	checkFailure("missing", missing, "native support archive missing for example.com/baseprobe/bridge")
	path := filepath.Join(source, "bridge/bridge.go")
	file, _ := os.OpenFile(path, os.O_APPEND|os.O_WRONLY, 0600)
	file.WriteString("\n// 修改固定入口源码。\n")
	file.Close()
	checkFailure("source", manifest, "selected source mismatch")
	if data, err := os.ReadFile(denyLog); !os.IsNotExist(err) {
		t.Fatalf("消费阶段不能调用 C 编译器: %v %s", err, data)
	}
}
