package textformat

import (
	"errors"
	"fmt"
	"reflect"
	"strings"
	"testing"
	"unicode/utf8"
)

// 预期文字与 UTF-8 范围独立列出，不用另一个解析入口作为预期。
func TestParse(t *testing.T) {
	for _, tc := range []struct {
		name, source, plain string
		formatted           bool
		runs                []Run
	}{
		{"plain", `a < b & C:\Users\Example \{bold:text\}`, `a < b & C:\Users\Example \{bold:text\}`, false, nil},
		{"not-at-start", ` \{format\}\{bold:text\}`, ` \{format\}\{bold:text\}`, false, nil},
		{"empty", prefix, "", true, nil},
		{"plain-format", prefix + "  a:b\n{x}\t", "  a:b\n{x}\t", true, nil},
		{"range", prefix + `a\{bold:中🙂\}b`, "a中🙂b", true, []Run{{1, 8, []string{"bold"}}}},
		{"nested", prefix + `\{warning:A\{bold:B\}C\}`, "ABC", true, []Run{{0, 1, []string{"warning"}}, {1, 2, []string{"warning", "bold"}}, {2, 3, []string{"warning"}}}},
		{"spaces-and-colon", prefix + `\{bold:  a:b  \}`, "  a:b  ", true, []Run{{0, 7, []string{"bold"}}}},
		{"escapes", prefix + `\\{bold:x\\} \n C:\temp`, `\{bold:x\} \n C:\temp`, true, nil},
		{"json", prefix + `\{code:{"a":1}\}`, `{"a":1}`, true, []Run{{0, 7, []string{"code"}}}},
		{"empty-merge", prefix + `\{bold:a\{italic:\}b\}\{bold:c\}`, "abc", true, []Run{{0, 3, []string{"bold"}}}},
		{"unicode-name", prefix + `\{强调:字\}`, "字", true, []Run{{0, 3, []string{"强调"}}}},
		{"combining-rtl", prefix + "\u0627" + `\{bold:e` + "\u0301" + `\}`, "اé", true, []Run{{2, 5, []string{"bold"}}}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got, err := Parse(tc.source)
			if err != nil || got.Plain != tc.plain || got.Formatted != tc.formatted || !reflect.DeepEqual(got.Runs, tc.runs) {
				t.Fatalf("解析=%+v，错误=%v；期望文字=%q、格式=%v、范围=%+v", got, err, tc.plain, tc.formatted, tc.runs)
			}
		})
	}
}

func TestParseErrors(t *testing.T) {
	for _, tc := range []struct {
		body string
		at   int
	}{
		{`\}`, 0}, {`\{:text\}`, 0}, {`\{bold}`, 0}, {`\{bold:text`, 0},
		{"中\\{bad\n:text\\}", 3}, {`a\{bold:b\}\}`, 11}, {"\xff", 0},
	} {
		_, err := Parse(prefix + tc.body)
		var syntax *ParseError
		if !errors.As(err, &syntax) || syntax.Offset != len(prefix)+tc.at {
			t.Errorf("%q：错误=%v，期望源字节位置=%d", tc.body, err, len(prefix)+tc.at)
		}
	}
	valid := prefix + strings.Repeat(`\{bold:`, 32) + "x" + strings.Repeat(`\}`, 32)
	if got, err := Parse(valid); err != nil || got.Plain != "x" || len(got.Runs[0].Styles) != 32 {
		t.Fatalf("32 层应当可解析：%+v / %v", got, err)
	}
	if _, err := Parse(prefix + strings.Repeat(`\{bold:`, 33)); err == nil {
		t.Fatal("超过 32 层未返回错误")
	}
}

func TestLiteralAndArguments(t *testing.T) {
	for _, value := range []string{"", `C:\a\b`, `\{format\}\{bold:数据\}`, `\} <>&`, "中文🙂\nا"} {
		got, err := Parse(Literal(value))
		if err != nil || got.Plain != value || len(got.Runs) != 0 {
			t.Fatalf("字面值 %q 被解释：%+v / %v", value, got, err)
		}
		for _, template := range []string{"File: %[1]s", prefix + `File: \{bold:%[1]s\}`} {
			source := fmt.Sprintf(template, EscapeArg(template, value))
			got, err := Parse(source)
			if err != nil || got.Plain != "File: "+value {
				t.Fatalf("参数被解释或模板切换改变文字：%q => %+v / %v", source, got, err)
			}
		}
	}
}

func FuzzLiteral(f *testing.F) {
	f.Add(`\{format\}\{bold:原样\}`)
	f.Add("字🙂 ا")
	f.Fuzz(func(t *testing.T, value string) {
		if !utf8.ValidString(value) {
			return // 格式模式的输入域是 UTF-8 文本。
		}
		parsed, err := Parse(Literal(value))
		if err != nil || parsed.Plain != value || len(parsed.Runs) != 0 {
			t.Fatalf("字面值不往返：%q => %+v / %v", value, parsed, err)
		}
	})
}
