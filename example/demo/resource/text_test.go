package resource

import (
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"reflect"
	"regexp"
	"strconv"
	"strings"
	"testing"

	"github.com/golang-gui/goui/mlang"
)

// Validate every named resource against its declared language, including format
// verbs. Reading definitions here avoids adding a production resource registry.
func TestEightLanguageResources(t *testing.T) {
	previous := mlang.Language()
	t.Cleanup(func() { _ = mlang.SetLanguage(previous) })
	files, err := os.ReadDir(".")
	if err != nil {
		t.Fatal(err)
	}
	format := regexp.MustCompile(`%[-+#0-9. ]*[a-zA-Z]`)
	count := 0
	for _, file := range files {
		if !strings.HasSuffix(file.Name(), ".go") || strings.HasSuffix(file.Name(), "_test.go") {
			continue
		}
		source, err := parser.ParseFile(token.NewFileSet(), file.Name(), nil, 0)
		if err != nil {
			t.Fatal(err)
		}
		ast.Inspect(source, func(node ast.Node) bool {
			value, ok := node.(*ast.ValueSpec)
			if !ok || len(value.Values) != 1 {
				return true
			}
			call, ok := value.Values[0].(*ast.CallExpr)
			if !ok {
				return true
			}
			callee, ok := call.Fun.(*ast.Ident)
			if !ok || callee.Name != "text" {
				return true
			}
			if len(call.Args) != 8 {
				t.Fatalf("%s must define eight languages", value.Names[0])
			}
			var translations [8]string
			for i, arg := range call.Args {
				literal, ok := arg.(*ast.BasicLit)
				if !ok || literal.Kind != token.STRING {
					t.Fatal("resource must be literal text")
				}
				translations[i], err = strconv.Unquote(literal.Value)
				if err != nil || translations[i] == "" {
					t.Fatalf("%s language %d is empty/invalid", value.Names[0], i)
				}
				if !reflect.DeepEqual(format.FindAllString(translations[i], -1), format.FindAllString(translations[0], -1)) {
					t.Fatalf("%s language %d changed format verbs", value.Names[0], i)
				}
			}
			getter := text(translations[0], translations[1], translations[2], translations[3], translations[4], translations[5], translations[6], translations[7])
			for i, tag := range []string{"en", "zh-CN", "ja", "ko", "de", "fr", "ru", "ar"} {
				if err := mlang.SetLanguage(tag); err != nil {
					t.Fatal(err)
				}
				if got := getter(); got != translations[i] {
					t.Fatalf("%s/%s: %q, want %q", value.Names[0], tag, got, translations[i])
				}
			}
			count++
			return true
		})
	}
	if count == 0 {
		t.Fatal("no resources validated")
	}
	t.Logf("validated %d resources in eight languages", count)
}
