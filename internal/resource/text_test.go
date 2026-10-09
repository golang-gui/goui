package resource

import (
	"testing"
	"unicode/utf8"

	"github.com/golang-gui/goui/mlang"
)

func TestFrameworkTranslations(t *testing.T) {
	previous := mlang.Language()
	t.Cleanup(func() { _ = mlang.SetLanguage(previous) })
	getters := []mlang.Getter{Undo, Redo, Cut, Copy, Paste, Delete, SelectAll,
		Minimize, Maximize, Close, Restore, CloseTab, ScrollTabsLeft, ScrollTabsRight}
	if err := mlang.SetLanguage("en"); err != nil {
		t.Fatal(err)
	}
	english := make([]string, len(getters))
	for i, getter := range getters {
		english[i] = getter()
	}
	for _, tc := range []struct{ tag, copy string }{
		{"en-US", "Copy"}, {"zh-CN", "复制"}, {"ja-JP", "コピー"}, {"ko-KR", "복사"},
		{"de-DE", "Kopieren"}, {"fr-FR", "Copier"}, {"ru-RU", "Копировать"}, {"ar-SA", "نسخ"},
	} {
		if err := mlang.SetLanguage(tc.tag); err != nil {
			t.Fatal(err)
		}
		if Copy() != tc.copy {
			t.Fatalf("%s: Copy=%q, want %q", tc.tag, Copy(), tc.copy)
		}
		for i, getter := range getters {
			got := getter()
			if got == "" || !utf8.ValidString(got) || tc.tag != "en-US" && got == english[i] {
				t.Fatalf("%s: missing/invalid translation of %q: %q", tc.tag, english[i], got)
			}
		}
	}
	_ = mlang.SetLanguage("es")
	for i, getter := range getters {
		if got := getter(); got != english[i] {
			t.Fatalf("unsupported language fallback: got %q, want %q", got, english[i])
		}
	}
}
