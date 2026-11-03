package mobilebase

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
