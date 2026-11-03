package textedit

import (
	"strings"
	"testing"
	"unicode/utf8"

	"github.com/rivo/uniseg"
)

// 长度按完整候选文本的扩展字素计算；不截断 UTF-8 或组合序列。
func TestInputLengthAndPresets(t *testing.T) {
	for _, tc := range []struct {
		name, current, inserted, want string
		rng                           Range
		filter                        InputFilter
		accept                        bool
	}{
		{"unlimited", "", "abcd", "abcd", Range{}, LimitLength(0), true},
		{"fit", "ab", "c", "c", Range{2, 2}, LimitLength(3), true},
		{"prefix", "ab", "cd", "c", Range{2, 2}, LimitLength(3), true},
		{"reject", "abc", "x", "", Range{3, 3}, LimitLength(3), false},
		{"replace", "abc", "xy", "x", Range{1, 2}, LimitLength(3), true},
		{"selected_no_room", "abc", "👨‍👩‍👧‍👦x", "👨‍👩‍👧‍👦", Range{1, 2}, LimitLength(3), true},
		{"combining_join", "e", "́", "́", Range{1, 1}, LimitLength(1), true},
		{"combining_prefix", "e", "́x", "́", Range{1, 1}, LimitLength(1), true},
		{"emoji", "", "👨‍👩‍👧‍👦é🇨🇳x", "👨‍👩‍👧‍👦é", Range{}, LimitLength(2), true},
		{"over_limit", "abcd", "x", "x", Range{1, 2}, LimitLength(2), true},
		{"over_limit_growth", "abcd", "x", "", Range{4, 4}, LimitLength(2), false},
		{"delete", "abc", "", "", Range{0, 2}, LimitLength(1), true},
		{"digits", "", "0123", "0123", Range{}, DigitsOnly(), true},
		{"unicode_digit", "", "１２", "", Range{}, DigitsOnly(), false},
		{"username", "", "A-z_9.foo", "A-z_9", Range{}, ASCIIUsername(5), true},
		{"username_reject", "", "bad@name", "", Range{}, ASCIIUsername(32), false},
		{"password", "", "中 👨‍👩‍👧‍👦", "中 👨‍👩‍👧‍👦", Range{}, Password(3), true},
		{"password_control", "", "a\tb", "", Range{}, Password(8), false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			m := NewModel(tc.current)
			in, err := NewInput(m, Edit{Range: tc.rng, Text: tc.inserted})
			if err != nil {
				t.Fatal(err)
			}
			got, accept := tc.inserted, true
			if tc.filter != nil {
				got, accept = tc.filter(in)
			}
			if got != tc.want || accept != tc.accept {
				t.Fatalf("got %q/%t want %q/%t", got, accept, tc.want, tc.accept)
			}
			if m.Text() != tc.current || m.CanUndo() {
				t.Fatal("策略改变了模型或历史")
			}
		})
	}
}

func TestInputContextChainAndLifetime(t *testing.T) {
	m := NewModel("old")
	in, err := NewInput(m, Edit{Range: Range{0, 3}, Text: "ab"})
	if err != nil {
		t.Fatal(err)
	}
	calls := 0
	upper := func(in Input) (string, bool) { calls++; return strings.ToUpper(in.Text), true }
	check := func(in Input) (string, bool) {
		calls++
		current, _ := in.CurrentText()
		result, _ := in.ResultText()
		if current != "old" || result != "AB" {
			t.Fatal("组合没有使用前项输出及原始范围", current, result)
		}
		return in.Text, true
	}
	if text, ok := Chain(nil, upper, check)(in); text != "AB" || !ok || calls != 2 {
		t.Fatal(text, ok, calls)
	}
	calls = 0
	if _, ok := Chain(func(Input) (string, bool) { return "", false }, upper)(in); ok || calls != 0 {
		t.Fatal("拒绝后继续调用策略")
	}
	if Chain(nil) != nil || AllowRunes(nil) != nil {
		t.Fatal("空策略没有保持 nil")
	}
	m.SetText("changed")
	if _, err := in.ResultText(); err == nil {
		t.Fatal("过期上下文仍能读取")
	}
	if _, err := NewInput(nil, Edit{}); err == nil {
		t.Fatal("接受了空模型")
	}
	if _, err := NewInput(m, Edit{Range: Range{0, 100}}); err == nil {
		t.Fatal("接受了无效范围")
	}
}

// 大段粘贴应只做固定次数的全文扫描，不逐字符反复重建候选字符串。
func TestInputLimitLargePaste(t *testing.T) {
	in, _ := NewInput(NewModel("ab"), Edit{Range: Range{2, 2}, Text: strings.Repeat("中", 100000)})
	text, ok := LimitLength(5)(in)
	if !ok || text != "中中中" {
		t.Fatal("大段粘贴裁剪错误", len(text), ok)
	}
}

// 对随机连接边界检查输出始终是完整插入字素前缀，候选结果绝不超过当前允许上限。
func FuzzInputLimit(f *testing.F) {
	for _, s := range []string{"abc", "é", "👨‍👩‍👧‍👦", "🇦🇧🇨🇩", "क्‍ष"} {
		f.Add(s, "́x👩‍👧", uint8(2))
	}
	f.Fuzz(func(t *testing.T, current, added string, n uint8) {
		if len(current)+len(added) > 4096 {
			return
		}
		current, added = Normalize(current), Normalize(added)
		m := NewModel(current)
		// 在任意有效 rune 边界替换，包含上下文可改变字素分段的情况。
		start := ClampOffset(current, len(current)/2)
		in, _ := NewInput(m, Edit{Range: Range{start, start}, Text: added})
		text, ok := LimitLength(int(n) + 1)(in)
		if !ok {
			return
		}
		if !utf8.ValidString(text) || !strings.HasPrefix(added, text) {
			t.Fatal("输出不是有效前缀")
		}
		if added == "" {
			return
		}
		boundary := false
		g := uniseg.NewGraphemes(added)
		for g.Next() {
			_, end := g.Positions()
			boundary = boundary || end == len(text)
		}
		if !boundary {
			t.Fatal("输出截断了插入字素")
		}
		if got := uniseg.GraphemeClusterCount(current[:start] + text + current[start:]); got > max(int(n)+1, uniseg.GraphemeClusterCount(current)) {
			t.Fatal("输出仍超限", current, added, text)
		}
	})
}
