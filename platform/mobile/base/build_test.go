package base

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"
)

func buildTestOptions(t *testing.T, goos string) BuildOptions {
	t.Helper()
	root := t.TempDir()
	suffix := ""
	if runtime.GOOS == "windows" {
		suffix = ".exe"
	}
	driver := filepath.Join(root, "Go 构建 O'Brien 工具"+suffix)
	source := filepath.Join(root, "tools")
	os.Mkdir(source, 0755)
	linker := filepath.Join(source, "lld"+suffix)
	for _, pair := range [][2]string{{driver, "./testdata/builddriver"}, {linker, "./testdata/lld"}} {
		cmd := exec.Command("go", "build", "-o", pair[0], pair[1])
		cmd.Env = baseDriverEnv()
		if data, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("测试工具: %v\n%s", err, data)
		}
	}
	tools := NativeTools{Format: 1, GOOS: runtime.GOOS, GOARCH: runtime.GOARCH, Provenance: NativeProvenance{"fixture", "0.1.0", "testdata/lld", "test only"}, LLD: NativeTool{filepath.Base(linker), "0.1.0"}, Files: map[string]string{filepath.Base(linker): ""}}
	spec := filepath.Join(root, "tools-source.json")
	nativeWriteSpec(t, spec, tools)
	toolOutput := filepath.Join(root, "sealed-tools")
	if err := ExportNativeTools(context.Background(), spec, source, toolOutput); err != nil {
		t.Fatal(err)
	}
	variant := "device"
	if goos == "ios" {
		variant = "simulator"
	}
	inputs := nativeTestInputs(t, goos, variant)
	inputPath := filepath.Join(inputs.root, "inputs.json")
	nativeWriteSpec(t, inputPath, inputs)
	baseDir := filepath.Join(root, "base")
	os.Mkdir(baseDir, 0755)
	archive := filepath.Join(baseDir, "support.a")
	os.WriteFile(archive, []byte("support"), 0600)
	hash, _ := baseHash(archive)
	mode := "c-shared"
	if goos == "ios" {
		mode = "c-archive"
	}
	base := BaseManifest{Format: 1, GoVersion: baseGoVersion, GOOS: goos, GOARCH: "arm64", Architecture: "v8.0", BuildMode: mode, Variant: variant, MinimumVersion: inputs.MinimumVersion, Packages: map[string]BasePackage{"runtime/cgo": {Archive: "support.a", SHA256: hash, SourceSHA256: strings.Repeat("0", 64), BuildID: "action/content"}}}
	basePath := filepath.Join(baseDir, "base.json")
	nativeWriteSpec(t, basePath, base)
	payload := filepath.Join(root, "payload")
	data := nativeTestELF()
	if goos == "ios" {
		data = nativeTestMachO()
	}
	os.WriteFile(payload, data, 0600)
	t.Setenv("GOUI_TEST_LLD_COPY", payload)
	t.Setenv("GOUI_TEST_BUILD_LOG", filepath.Join(root, "commands.jsonl"))
	t.Setenv("GOFLAGS", "")
	options := BuildOptions{GoCommand: driver, GoDriver: driver, Linker: driver, Base: basePath, Inputs: inputPath, Tools: filepath.Join(toolOutput, "tools.json"), Directory: root, Package: "./main", Output: filepath.Join(root, "发布 output", "Probe"), Verbose: true}
	if goos == "ios" {
		host := filepath.Join(root, "原生 host.o")
		os.WriteFile(host, []byte("host"), 0600)
		options.Objects = []string{host}
		options.Libraries = []string{"objc"}
		options.Frameworks = []string{"UIKit"}
	}
	return options
}

func TestBuildPublication(t *testing.T) {
	for _, goos := range []string{"android", "ios"} {
		t.Run(goos, func(t *testing.T) {
			options := buildTestOptions(t, goos)
			t.Setenv("GOWORK", "fixture-workspace")
			t.Setenv("GOCACHE", "fixture-cache")
			t.Setenv("GOPROXY", "https://proxy.example.invalid")
			t.Setenv("GOOS", "wrong-target")
			t.Setenv("CGO_ENABLED", "0")
			t.Setenv("CC", "unavailable-ambient-compiler")
			t.Setenv("CXX", "unavailable-ambient-compiler")
			t.Setenv("GOUI_TEST_LLD_NAME", filepath.Base(options.Output))
			var log bytes.Buffer
			if err := Build(context.Background(), options, &log, &log); err != nil {
				t.Fatalf("build: %v\n%s", err, &log)
			}
			want, _ := os.ReadFile(os.Getenv("GOUI_TEST_LLD_COPY"))
			got, _ := os.ReadFile(options.Output)
			if !bytes.Equal(got, want) {
				t.Fatal("最终输出错误")
			}
			entries, _ := os.ReadDir(filepath.Dir(options.Output))
			if len(entries) != 1 {
				t.Fatalf("临时目录未清理: %v", entries)
			}
			raw, err := os.Open(os.Getenv("GOUI_TEST_BUILD_LOG"))
			if err != nil {
				t.Fatal(err)
			}
			defer raw.Close()
			decoder := json.NewDecoder(raw)
			builds := 0
			for {
				var row map[string]any
				err := decoder.Decode(&row)
				if err == io.EOF {
					break
				}
				if err != nil {
					t.Fatal(err)
				}
				if row["CC"] != row["CXX"] || !strings.Contains(row["CC"].(string), options.Linker) {
					t.Error("包含 go env 在内的所有消费步骤都不能回退到系统 C 编译器")
				}
				args := row["args"].([]any)
				if args[0] != "build" {
					continue
				}
				builds++
				for key, value := range map[string]string{"GOOS": goos, "GOARCH": "arm64", "GOARM64": "v8.0", "CGO_ENABLED": "1", "GOTOOLCHAIN": "local", "GOWORK": "fixture-workspace", "GOCACHE": "fixture-cache", "GOPROXY": "https://proxy.example.invalid", "GOUI_BASE": options.Base, "GOUI_LINK_INPUTS": options.Inputs, "GOUI_LINK_TOOLS": options.Tools, "GOUI_GO": options.GoCommand} {
					if row[key] != value {
						t.Errorf("%s=%v, want %q", key, row[key], value)
					}
				}
			}
			if builds != 1 {
				t.Fatalf("Go 构建次数=%d", builds)
			}
			if os.Getenv("GOOS") != "wrong-target" || os.Getenv("CGO_ENABLED") != "0" || os.Getenv("CC") != "unavailable-ambient-compiler" {
				t.Fatal("污染了调用方环境")
			}
			// 生成了临时文件之后失败，仍须保留已发布结果。
			t.Setenv("GOUI_TEST_BUILD_FAIL", "1")
			if err := Build(context.Background(), options, io.Discard, io.Discard); err == nil {
				t.Fatal("忽略了 Go 构建失败")
			}
			got, _ = os.ReadFile(options.Output)
			if !bytes.Equal(got, want) {
				t.Fatal("失败覆盖了旧输出")
			}
			t.Setenv("GOUI_TEST_BUILD_FAIL", "")
			payload, _ := os.ReadFile(os.Getenv("GOUI_TEST_LLD_COPY"))
			os.WriteFile(os.Getenv("GOUI_TEST_LLD_COPY"), []byte("invalid binary"), 0600)
			if err := Build(context.Background(), options, io.Discard, io.Discard); err == nil {
				t.Fatal("发布了无效原生输出")
			}
			got, _ = os.ReadFile(options.Output)
			if !bytes.Equal(got, want) {
				t.Fatal("无效输出覆盖了旧结果")
			}
			os.WriteFile(os.Getenv("GOUI_TEST_LLD_COPY"), payload, 0600)
			if goos == "ios" {
				t.Setenv("GOUI_TEST_LLD_FAIL", "1")
				if err := Build(context.Background(), options, io.Discard, io.Discard); err == nil {
					t.Fatal("忽略了最终链接失败")
				}
				got, _ = os.ReadFile(options.Output)
				if !bytes.Equal(got, want) {
					t.Fatal("链接失败覆盖了旧输出")
				}
			}
		})
	}
}

func TestBuildEntry(t *testing.T) {
	options := buildTestOptions(t, "android")
	options.Entry = true
	if err := os.Mkdir(filepath.Join(options.Directory, "main"), 0755); err != nil {
		t.Fatal(err)
	}
	if err := Build(context.Background(), options, io.Discard, io.Discard); err != nil {
		t.Fatal(err)
	}
	f, err := os.Open(os.Getenv("GOUI_TEST_BUILD_LOG"))
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	decoder := json.NewDecoder(f)
	found := false
	for {
		var row map[string]any
		if err = decoder.Decode(&row); err == io.EOF {
			break
		}
		if err != nil {
			t.Fatal(err)
		}
		if row["args"].([]any)[0] != "build" {
			continue
		}
		found = true
		if row["entry"] != entrySource {
			t.Fatalf("没有注入规定入口: %v", row)
		}
		if _, err = os.Stat(row["entryTarget"].(string)); !os.IsNotExist(err) {
			t.Fatal("注册文件写入了用户项目")
		}
	}
	if !found {
		t.Fatal("没有执行消费构建")
	}
}

func TestBuildPreflight(t *testing.T) {
	options := buildTestOptions(t, "ios")
	cases := []struct {
		name string
		edit func(*BuildOptions)
		want string
	}{
		{"无宿主", func(o *BuildOptions) { o.Objects = nil }, "explicit native host"},
		{"目标不一致", func(o *BuildOptions) {
			m := nativeTestInputs(t, "android", "device")
			o.Inputs = filepath.Join(m.root, "inputs.json")
			nativeWriteSpec(t, o.Inputs, m)
		}, "mismatch"},
		{"原生源码", func(o *BuildOptions) { o.Objects = []string{"host.c"} }, "object/archive expected"},
		{"缺少库", func(o *BuildOptions) { o.Libraries = []string{"missing"} }, "not in target component"},
		{"无效包名", func(o *BuildOptions) { o.Package = "-overlay=other" }, "invalid main package"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			o := options
			c.edit(&o)
			if err := Build(context.Background(), o, io.Discard, io.Discard); err == nil || !strings.Contains(err.Error(), c.want) {
				t.Fatalf("error=%v, want %s", err, c.want)
			}
		})
	}
	for _, key := range []string{"GOFLAGS", "GOUI_TEST_BUILD_BADGO"} {
		t.Run(key, func(t *testing.T) {
			t.Setenv(key, "1")
			if err := Build(context.Background(), options, io.Discard, io.Discard); err == nil {
				t.Fatal("不应接受错误工具链或额外 flags")
			}
		})
	}
	if _, err := os.Stat(os.Getenv("GOUI_TEST_BUILD_LOG")); err == nil {
		data, _ := os.ReadFile(os.Getenv("GOUI_TEST_BUILD_LOG"))
		if bytes.Contains(data, []byte(`"build"`)) {
			t.Fatal("前置失败后仍然构建了源码")
		}
	}
}

func TestBuildCancellation(t *testing.T) {
	options := buildTestOptions(t, "android")
	os.MkdirAll(filepath.Dir(options.Output), 0755)
	os.WriteFile(options.Output, []byte("old"), 0600)
	marker := filepath.Join(options.Directory, "started")
	t.Setenv("GOUI_TEST_BUILD_WAIT", marker)
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	done := make(chan error, 1)
	go func() { done <- Build(ctx, options, io.Discard, io.Discard) }()
	for {
		if _, err := os.Stat(marker); err == nil {
			break
		}
		select {
		case err := <-done:
			t.Fatalf("未进入构建: %v", err)
		case <-time.After(10 * time.Millisecond):
		}
	}
	cancel()
	if err := <-done; !errors.Is(err, context.Canceled) {
		t.Fatalf("取消错误=%v, want context.Canceled", err)
	}
	data, _ := os.ReadFile(options.Output)
	if string(data) != "old" {
		t.Fatal("取消后旧输出被覆盖")
	}
	entries, _ := os.ReadDir(filepath.Dir(options.Output))
	if len(entries) != 1 {
		t.Fatalf("取消未清理临时输出: %v", entries)
	}
}
