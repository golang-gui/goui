package base

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"testing"

	"github.com/golang-gui/golink"
)

func TestNativeArguments(t *testing.T) {
	android := nativeTestInputs(t, "android", "device")
	ios := nativeTestInputs(t, "ios", "simulator")
	object := filepath.Join(t.TempDir(), "some object.o")
	os.WriteFile(object, []byte("object"), 0600)
	for _, test := range []struct {
		name    string
		m       *NativeInputs
		args    []string
		want    []string
		failure string
	}{
		{"Android", android, []string{"-Wl,-z,relro", "-shared", "-o", "output.so", "-rdynamic", object, "-O2", "-g", "-ldl"}, []string{"-flavor", "gnu", "max-page-size=16384", "--export-dynamic", object}, ""},
		{"iOS", ios, []string{"-o", "output", object, "-framework", "UIKit", "-lobjc", "-Wl,-S", "-Wl,-x"}, []string{"darwin", "ios-simulator", "15.0", "26.2", "-Z", "-all_load", "UIKit"}, ""},
		{"禁止C源文件", android, []string{"-shared", "-o", "output", object, "trivial.c"}, nil, "native compilation"},
		{"禁止嵌套编译", android, []string{"-shared", "-o", "output", object, "-c"}, nil, "native compilation"},
		{"禁止替换架构", android, []string{"-shared", "-o", "output", object, "-m32"}, nil, "unsupported native link option"},
		{"禁止静态模式", android, []string{"-static", "-o", "output", object}, nil, "unsupported native link option"},
		{"禁止降低页对齐", android, []string{"-shared", "-o", "output", object, "-Wl,-z,max-page-size=4096"}, nil, "unsupported native -z"},
		{"禁止关闭RELRO", android, []string{"-shared", "-o", "output", object, "-Wl,-z,norelro"}, nil, "unsupported native -z"},
		{"禁止宿主SDK路径", android, []string{"-shared", "-o", "output", object, "-L/usr/lib"}, nil, "unsupported native link option"},
		{"未知目标库", android, []string{"-shared", "-o", "output", object, "-lmissing"}, nil, "not in target component"},
		{"未知Framework", ios, []string{"-o", "output", object, "-framework", "Unknown"}, nil, "not declared"},
		{"输出不能被覆盖", android, []string{"-shared", "-o", "one", object, "-Wl,-o,two"}, nil, "exactly one"},
		{"缺少对象", android, []string{"-shared", "-o", "output"}, nil, "object inputs"},
		{"缺少模式", android, []string{"-o", "output", object}, nil, "-shared mode"},
	} {
		t.Run(test.name, func(t *testing.T) {
			args, out, err := nativeLinkArguments(test.m, test.args)
			if test.failure != "" {
				if err == nil || !strings.Contains(err.Error(), test.failure) {
					t.Fatalf("error=%v want %s", err, test.failure)
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			if out == "" {
				t.Fatal("丢失输出路径")
			}
			for _, want := range test.want {
				found := false
				for _, arg := range args {
					found = found || arg == want
				}
				if !found {
					t.Fatalf("missing %q: %q", want, args)
				}
			}
		})
	}
}

func TestNativeResponses(t *testing.T) {
	root := t.TempDir()
	path := filepath.Join(root, "args.rsp")
	expected := []string{"-shared", "-o", `C:\folder\some output.so`, "中文.o"}
	var data strings.Builder
	for _, arg := range expected {
		fmt.Fprintf(&data, "%q\n", arg)
	}
	os.WriteFile(path, []byte(data.String()), 0600)
	actual, err := nativeResponses([]string{"@" + path}, 0)
	if err != nil {
		t.Fatal(err)
	}
	if fmt.Sprint(actual) != fmt.Sprint(expected) {
		t.Fatalf("response=%q, want %q", actual, expected)
	}
	os.WriteFile(path, []byte(strconv.Quote("@"+path)), 0600)
	if _, err = nativeResponses([]string{"@" + path}, 0); err == nil {
		t.Fatal("递归响应文件不能无限展开")
	}
	os.WriteFile(path, []byte("unquoted path with spaces"), 0600)
	if _, err = nativeResponses([]string{"@" + path}, 0); err == nil {
		t.Fatal("不应猜测有歧义的响应文件语法")
	}
}

func TestNativeRunAndArchive(t *testing.T) {
	root := t.TempDir()
	source := filepath.Join(root, "tools-source")
	os.Mkdir(source, 0755)
	toolFile := "lld"
	if runtime.GOOS == "windows" {
		toolFile += ".exe"
	}
	command := exec.Command("go", "build", "-o", filepath.Join(source, toolFile), "./testdata/lld")
	command.Env = baseDriverEnv()
	if output, err := command.CombinedOutput(); err != nil {
		t.Fatalf("测试链接器: %v\n%s", err, output)
	}
	tools := NativeTools{Format: 1, GOOS: runtime.GOOS, GOARCH: runtime.GOARCH, Provenance: NativeProvenance{"fake", "0.1.0", "fixture", "test only"}, LLD: NativeTool{toolFile, "0.1.0"}, Files: map[string]string{toolFile: ""}}
	spec := filepath.Join(root, "tools-spec.json")
	nativeWriteSpec(t, spec, tools)
	toolComponent := filepath.Join(root, "tools")
	if err := ExportNativeTools(context.Background(), spec, source, toolComponent); err != nil {
		t.Fatal(err)
	}
	toolsPath := filepath.Join(toolComponent, "tools.json")
	m := nativeTestInputs(t, "ios", "simulator")
	inputsPath := filepath.Join(m.root, "inputs.json")
	nativeWriteSpec(t, inputsPath, m)
	fixture := filepath.Join(root, "native")
	os.WriteFile(fixture, nativeTestMachO(), 0600)
	t.Setenv("GOUI_TEST_LLD_COPY", fixture)
	object := filepath.Join(root, "object.o")
	os.WriteFile(object, []byte("object"), 0600)
	output := filepath.Join(root, "output")
	t.Setenv("GOUI_TEST_LLD_NAME", filepath.Base(output))
	linkObject := object
	if runtime.GOOS != "windows" {
		linkObject = filepath.Join(root, "object with\ttab中文.o")
		if err := os.WriteFile(linkObject, []byte("object"), 0600); err != nil {
			t.Fatal(err)
		}
	}
	args := []string{"-o", output, linkObject}
	var log bytes.Buffer
	if err := RunNativeLink(context.Background(), inputsPath, toolsPath, args, &log, &log); err != nil {
		t.Fatalf("link: %v\n%s", err, &log)
	}
	original, err := os.ReadFile(output)
	if err != nil {
		t.Fatal(err)
	}
	t.Setenv("GOUI_TEST_LLD_FAIL", "1")
	if err = RunNativeLink(context.Background(), inputsPath, toolsPath, args, &log, &log); err == nil {
		t.Fatal("不能吞掉链接器失败")
	}
	actual, _ := os.ReadFile(output)
	if !bytes.Equal(actual, original) {
		t.Fatal("链接失败不能覆盖已有产物")
	}
	t.Setenv("GOUI_TEST_LLD_FAIL", "")
	os.WriteFile(fixture, []byte("invalid native output"), 0600)
	if err = RunNativeLink(context.Background(), inputsPath, toolsPath, args, &log, &log); err == nil {
		t.Fatal("无效输出不能发布")
	}
	actual, _ = os.ReadFile(output)
	if !bytes.Equal(actual, original) {
		t.Fatal("校验失败不能覆盖已有产物")
	}
	_, _, err = nativeLinkArguments(m, []string{"-o", output, object, "compiler.c"})
	if !errors.Is(err, ErrNativeCompilation) {
		t.Fatal(err)
	}
	t.Run("Go原生归档", func(t *testing.T) {
		if runtime.Version() != baseGoVersion {
			t.Skipf("归档委托需 %s，当前 %s", baseGoVersion, runtime.Version())
		}
		archive := filepath.Join(root, "native.a")
		args := []string{"-q", "-c", "-s", archive, object}
		if err = RunNativeArchive(context.Background(), "go", args, &log, &log); err != nil {
			t.Fatalf("archive: %v\n%s", err, &log)
		}
		data, err := os.ReadFile(archive)
		if err != nil || !bytes.HasPrefix(data, []byte("!<arch>\n")) || !bytes.Contains(data, []byte("object.o")) {
			t.Fatalf("原生归档不完整: %v %q", err, data)
		}
	})
}

func TestNativeLLDQuoting(t *testing.T) {
	for _, test := range []struct{ arg, want string }{
		{"space name.o", `"space name.o"`},
		{"actual\ttab.o", "\"actual\ttab.o\""},
		{`literal\t.o`, `"literal\\t.o"`},
		{`quote"name.o`, `"quote\"name.o"`},
		{"中文.o", `"中文.o"`},
	} {
		if got := nativeLLDQuote(test.arg); got != test.want {
			t.Fatalf("%q: %q, want %q", test.arg, got, test.want)
		}
	}
}

// 验证直接调用 Go 链接库，不提供 golink 可执行文件或模拟版本响应。
func TestNativeGoLink(t *testing.T) {
	root := t.TempDir()
	toolRoot := filepath.Join(root, "tools")
	if err := os.Mkdir(toolRoot, 0755); err != nil {
		t.Fatal(err)
	}
	m := NativeTools{Format: 2, GOOS: runtime.GOOS, GOARCH: runtime.GOARCH,
		Provenance: NativeProvenance{"golink", golink.Version, "Go module", "MIT"}, GoLink: golink.Version, Files: map[string]string{}}
	toolsPath := filepath.Join(toolRoot, "tools.json")
	nativeWriteSpec(t, toolsPath, m)
	loaded, err := OpenNativeTools(toolsPath)
	if err != nil {
		t.Fatal(err)
	}
	if err = verifyNativeToolVersions(context.Background(), loaded); err != nil {
		t.Fatal(err)
	}
	inputs := nativeTestInputs(t, "android", "device")
	object, output := filepath.Join(root, "中文 object.o"), filepath.Join(root, "result.so")
	if err = os.WriteFile(object, nativeTestObject("entry"), 0600); err != nil {
		t.Fatal(err)
	}
	for name := range inputs.Files {
		path := filepath.Join(inputs.root, filepath.FromSlash(name))
		switch filepath.Ext(name) {
		case ".o":
			err = os.WriteFile(path, nativeTestObject(""), 0600)
		case ".a":
			err = os.WriteFile(path, []byte("!<arch>\n"), 0600)
		case ".so":
			err = golink.Link(golink.Request{Format: 1, Target: "android/arm64", Objects: []string{object}, Output: path, SONAME: filepath.Base(path)})
		}
		if err != nil {
			t.Fatal(err)
		}
		inputs.Files[name], err = baseHash(path)
		if err != nil {
			t.Fatal(err)
		}
	}
	inputPath := filepath.Join(inputs.root, "inputs.json")
	nativeWriteSpec(t, inputPath, inputs)
	args := []string{"-shared", "-o", output, object, "-Wl,-z,nodelete", "-Wl,-Bsymbolic"}
	translated, _, err := nativeLinkArguments(inputs, args)
	if err != nil {
		t.Fatal(err)
	}
	r, err := nativeGoLinkRequest(inputs, translated, output)
	if err != nil {
		t.Fatal(err)
	}
	if r.Format != 1 || r.Target != "android/arm64" || !r.NoDelete || !r.Symbolic || len(r.Libraries) != 3 || len(r.Archives) != 1 || len(r.Objects) != 3 || r.Objects[1] != object {
		t.Fatalf("丢失输入: %+v", r)
	}
	var log bytes.Buffer
	if err = RunNativeLink(context.Background(), inputPath, toolsPath, args, &log, &log); err != nil {
		t.Fatal(err)
	}
	original, err := os.ReadFile(output)
	if err != nil {
		t.Fatal(err)
	}
	for _, failure := range []string{"错误对象", "未支持选项", "取消"} {
		t.Run(failure, func(t *testing.T) {
			testArgs := append([]string(nil), args...)
			ctx := context.Background()
			switch failure {
			case "错误对象":
				bad := filepath.Join(root, "invalid.o")
				if err := os.WriteFile(bad, []byte("invalid ELF"), 0600); err != nil {
					t.Fatal(err)
				}
				testArgs = append(testArgs, bad)
			case "未支持选项":
				testArgs = append(testArgs, "-Wl,--build-id=sha1")
			case "取消":
				var cancel context.CancelFunc
				ctx, cancel = context.WithCancel(ctx)
				cancel()
			}
			if err := RunNativeLink(ctx, inputPath, toolsPath, testArgs, &log, &log); err == nil {
				t.Fatal("不应成功")
			}
			actual, _ := os.ReadFile(output)
			if !bytes.Equal(actual, original) {
				t.Fatal("失败不能覆盖原产物")
			}
		})
	}
	if err = loaded.supports(&NativeInputs{GOOS: "windows", GOARCH: "amd64"}); err == nil {
		t.Fatal("不能假装支持未实现的原生格式")
	}
	for _, bad := range []NativeTools{
		{Format: 1, GoLink: m.GoLink},
		{Format: 2},
		{Format: 2, GoLink: m.GoLink, LLD: NativeTool{"lld", "22"}},
	} {
		if err := bad.validateLinker(); err == nil {
			t.Fatalf("有歧义的后端: %+v", bad)
		}
	}
	m.GoLink = "0.1.00"
	nativeWriteSpec(t, toolsPath, m)
	loaded, err = OpenNativeTools(toolsPath)
	if err != nil {
		t.Fatal(err)
	}
	if err = verifyNativeToolVersions(context.Background(), loaded); err == nil {
		t.Fatal("版本必须精确匹配")
	}
}

// 对设备和模拟器均通过库调用生成真实 Mach-O，不依赖桌面或 Apple 工具。
func TestNativeGoLinkIOS(t *testing.T) {
	for _, variant := range []string{"device", "simulator"} {
		t.Run(variant, func(t *testing.T) {
			inputs := nativeTestInputs(t, "ios", variant)
			target := "arm64-ios"
			platform := uint32(2)
			if variant == "simulator" {
				target += "-simulator"
				platform = 7
			}
			for name := range inputs.Files {
				path := filepath.Join(inputs.root, filepath.FromSlash(name))
				text := fmt.Sprintf("--- !tapi-tbd\ntbd-version: 4\ntargets: [ %s ]\ninstall-name: '/%s'\nexports:\n  - targets: [ %s ]\n    symbols: []\n...\n", target, filepath.Base(name), target)
				if err := os.WriteFile(path, []byte(text), 0600); err != nil {
					t.Fatal(err)
				}
				var err error
				inputs.Files[name], err = baseHash(path)
				if err != nil {
					t.Fatal(err)
				}
			}
			inputPath := filepath.Join(inputs.root, "inputs.json")
			nativeWriteSpec(t, inputPath, inputs)
			root := t.TempDir()
			toolsPath := filepath.Join(root, "tools.json")
			m := NativeTools{Format: 2, GOOS: runtime.GOOS, GOARCH: runtime.GOARCH, Provenance: NativeProvenance{"golink", golink.Version, "Go module", "MIT and BSD"}, GoLink: golink.Version, Files: map[string]string{}}
			nativeWriteSpec(t, toolsPath, m)
			object := filepath.Join(t.TempDir(), "main.o")
			if err := os.WriteFile(object, nativeTestMachOObject(platform), 0600); err != nil {
				t.Fatal(err)
			}
			output := filepath.Join(t.TempDir(), "Application")
			args := []string{"-o", output, object, "-framework", "UIKit"}
			translated, _, err := nativeLinkArguments(inputs, args)
			if err != nil {
				t.Fatal(err)
			}
			request, err := nativeGoLinkRequest(inputs, translated, output)
			if err != nil {
				t.Fatal(err)
			}
			if request.IOS == nil || request.IOS.Variant != variant || request.IOS.SDKVersion != inputs.SDKVersion || len(request.Reexports) != 1 {
				t.Fatalf("iOS输入丢失: %+v", request)
			}
			var log bytes.Buffer
			if err = RunNativeLink(context.Background(), inputPath, toolsPath, args, &log, &log); err != nil {
				t.Fatal(err)
			}
			if err = CheckNativeBinary(output, inputs); err != nil {
				t.Fatal(err)
			}
		})
	}
}
