// Go 官方标准库移动验收工程生成器，不依赖桌面。
// 在仓库根执行 go run ./tests/window/platform/mobile_std -o /absolute/output。
// 读取受支持 Go 版本 GOROOT 中明确列出的官方外部测试文件，保留原始断言与版权，
// 生成仓库外纯 Go 应用；不运行原生编译器，不修改 GOROOT。操作说明见 README.md。
package main

import (
	"crypto/sha256"
	"encoding/json"
	"flag"
	"fmt"
	"go/ast"
	"go/format"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"strconv"
	"strings"

	"github.com/golang-gui/goui/internal/nativegen"
)

// 每个文件整体保留；不删除 Test、断言或内部 skip。仅选择可独立编译且使用
// 公开标准库 API 的外部测试，不复制标准库实现来测试另一份副本。
var suites = []struct {
	Package string
	Files   []string
}{
	{"unicode/utf8", []string{"utf8_test.go"}},
	{"unicode", []string{"digit_test.go", "graphic_test.go", "letter_test.go", "script_test.go"}},
	{"math", []string{"const_test.go"}},
	{"cmp", []string{"cmp_test.go"}},
	{"io", []string{"multi_test.go", "pipe_test.go"}},
	{"path", []string{"path_test.go", "match_test.go"}},
	{"strings", []string{"reader_test.go"}},
	{"bytes", []string{"compare_test.go"}},
	{"sync", []string{"waitgroup_test.go", "once_test.go", "cond_test.go", "rwmutex_test.go"}},
	{"sync/atomic", []string{"value_test.go"}},
	{"errors", []string{"errors_test.go", "wrap_test.go", "join_test.go"}},
	{"reflect", []string{"visiblefields_test.go"}},
}

type sourceRecord struct {
	Package, File, SHA256 string
	Tests                 []string
	Excluded              string `json:",omitempty"`
}

func main() {
	output := flag.String("o", "", "new generated application directory outside the repository")
	painter := flag.String("painter", "software", "software or opengl")
	native := flag.Bool("cgo", false, "add a real user CGO test requiring -DGOUI_NATIVE_SENTINEL=73")
	flag.Parse()
	if err := generateWithCgo(*output, *painter, *native); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func generate(output, painter string) error {
	return generateWithCgo(output, painter, false)
}

func generateWithCgo(output, painter string, native bool) error {
	version := runtime.Version()
	if version != "go1.24.13" && version != "go1.25.8" && version != "go1.26.10" && version != "go1.27.3" {
		return fmt.Errorf("unsupported official test source version %s", version)
	}
	if output == "" || !filepath.IsAbs(output) {
		return fmt.Errorf("-o must be an absolute directory")
	}
	if painter != "software" && painter != "opengl" {
		return fmt.Errorf("invalid painter")
	}
	repo, err := os.Getwd()
	if err != nil {
		return err
	}
	for {
		if _, err = os.Stat(filepath.Join(repo, "AGENTS.md")); err == nil {
			break
		}
		parent := filepath.Dir(repo)
		if parent == repo {
			return fmt.Errorf("run within the GOUI checkout")
		}
		repo = parent
	}
	if err = nativegen.CheckOutput(output, repo); err != nil {
		return err
	}
	if _, err = os.Lstat(output); !os.IsNotExist(err) {
		return fmt.Errorf("output already exists or cannot be inspected: %s", output)
	}
	if err = os.MkdirAll(output, 0755); err != nil {
		return err
	}
	write := func(name, source string) error {
		path := filepath.Join(output, name)
		if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
			return err
		}
		if strings.HasSuffix(name, ".go") {
			formatted, err := format.Source([]byte(source))
			if err != nil {
				return fmt.Errorf("%s: %w", name, err)
			}
			source = string(formatted)
		}
		return os.WriteFile(path, []byte(source), 0644)
	}
	if err = write("go.mod", "module example.org/gouistd\ngo "+strings.TrimPrefix(version, "go")+"\n"); err != nil {
		return err
	}
	var imports, appendTests strings.Builder
	var records []sourceRecord
	var excluded []sourceRecord
	for i, suite := range suites {
		var names []string
		packageName := ""
		for _, file := range suite.Files {
			data, err := os.ReadFile(filepath.Join(runtime.GOROOT(), "src", suite.Package, file))
			if err != nil {
				return err
			}
			// 新版本的整文件依赖 internal/testenv，不能在外部模块中逐字执行。
			// 仅改变新版本选择集，原 Go1.25.8 的完整文件和断言保持。
			if (version == "go1.26.10" || version == "go1.27.3") && suite.Package == "sync" && file == "waitgroup_test.go" {
				excluded = append(excluded, sourceRecord{Package: suite.Package, File: file, SHA256: fmt.Sprintf("%x", sha256.Sum256(data)), Excluded: "complete file imports internal/testenv; not portable outside GOROOT"})
				continue
			}
			f, err := parser.ParseFile(token.NewFileSet(), file, data, 0)
			if err != nil {
				return err
			}
			if !strings.HasSuffix(f.Name.Name, "_test") {
				return fmt.Errorf("%s is not an external test package", file)
			}
			packageName = f.Name.Name
			for _, imp := range f.Imports {
				p, _ := strconv.Unquote(imp.Path.Value)
				if strings.HasPrefix(p, "internal/") || strings.Contains(p, "/internal/") {
					return fmt.Errorf("%s imports unavailable internal helper %s", file, p)
				}
			}
			var tests []string
			for _, decl := range f.Decls {
				fn, ok := decl.(*ast.FuncDecl)
				if ok && fn.Recv == nil && strings.HasPrefix(fn.Name.Name, "Test") && fn.Name.Name != "TestMain" {
					tests = append(tests, fn.Name.Name)
				}
			}
			names = append(names, tests...)
			records = append(records, sourceRecord{Package: suite.Package, File: file, SHA256: fmt.Sprintf("%x", sha256.Sum256(data)), Tests: tests})
			// 仅改文件名以参与普通构建，测试文件内容逐字保留。
			path := filepath.Join(output, "official", fmt.Sprintf("pkg%02d", i), strings.TrimSuffix(file, "_test.go")+"_official.go")
			if err = os.MkdirAll(filepath.Dir(path), 0755); err != nil {
				return err
			}
			if err = os.WriteFile(path, data, 0644); err != nil {
				return err
			}
		}
		sort.Strings(names)
		if packageName == "" {
			continue
		}
		var bridge strings.Builder
		fmt.Fprintf(&bridge, "package %s\nimport \"testing\"\nfunc Tests() []testing.InternalTest {return []testing.InternalTest{\n", packageName)
		for _, name := range names {
			fmt.Fprintf(&bridge, "{%q,%s},\n", suite.Package+"/"+name, name)
		}
		bridge.WriteString("}}\n")
		if err = write(fmt.Sprintf("official/pkg%02d/cases.go", i), bridge.String()); err != nil {
			return err
		}
		fmt.Fprintf(&imports, "p%d \"example.org/gouistd/official/pkg%02d\"\n", i, i)
		fmt.Fprintf(&appendTests, "tests=append(tests,p%d.Tests()...)\n", i)
	}
	registry := "package suite\nimport(\"testing\"\n" + imports.String() + ")\nfunc officialTests() []testing.InternalTest {var tests []testing.InternalTest\n" + appendTests.String() + "return tests}\n"
	if err = write("suite/cases.go", registry); err != nil {
		return err
	}
	runner := runnerSource
	if native {
		if strings.Count(runner, "report.Official=len(tests)") != 1 {
			return fmt.Errorf("native test insertion point changed")
		}
		runner = strings.Replace(runner, "report.Official=len(tests)", "report.Official=len(tests)\ntests=append(tests,testing.InternalTest{\"goui-platform/TestUserNativeCGO\",testUserNativeCGO})", 1)
		if err = write("suite/native.go", nativeCgoSource); err != nil {
			return err
		}
	}
	if err = write("suite/runner.go", runner); err != nil {
		return err
	}
	if err = write("main.go", strings.ReplaceAll(applicationSource, "GOUI_SELECTED_PAINTER", painter)); err != nil {
		return err
	}
	config := `id = "org.golanggui.StandardTests"
name = "GOUI Standard Tests"
executable = "goui-std"
main = "."
version = "0.1.0"
build = 1
icon = "icon.svg"
[android]
min_sdk = 29
target_sdk = 35
internet = true
debuggable = true
[ios]
minimum_version = "15.0"
`
	if err = write("goui.toml", config); err != nil {
		return err
	}
	if err = write("icon.svg", `<svg xmlns="http://www.w3.org/2000/svg" width="128" height="128"><rect width="128" height="128" rx="24" fill="#0672c9"/><path d="M28 66 52 90 100 38" fill="none" stroke="white" stroke-width="12"/></svg>`); err != nil {
		return err
	}
	data, err := json.MarshalIndent(struct {
		GoVersion string
		Sources   []sourceRecord
		Excluded  []sourceRecord
		Scope     string
	}{runtime.Version(), records, excluded, "Selected complete external test files; not go test std. Benchmarks, fuzzing and examples are not executed."}, "", "  ")
	if err != nil {
		return err
	}
	if err = os.WriteFile(filepath.Join(output, "official-sources.json"), data, 0644); err != nil {
		return err
	}
	license, err := os.ReadFile(filepath.Join(runtime.GOROOT(), "LICENSE"))
	if err != nil {
		return err
	}
	if err = os.WriteFile(filepath.Join(output, "GO-LICENSE"), license, 0644); err != nil {
		return err
	}
	fmt.Println(output)
	return nil
}
