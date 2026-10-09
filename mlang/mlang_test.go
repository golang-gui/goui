package mlang_test

import (
	"fmt"
	"sync"
	"testing"

	"github.com/golang-gui/goui/mlang"
)

func preserveLanguage(t *testing.T) {
	t.Helper()
	previous := mlang.Language()
	t.Cleanup(func() {
		if err := mlang.SetLanguage(previous); err != nil {
			t.Error(err)
		}
	})
}

func setLanguage(t *testing.T, tag string) {
	t.Helper()
	if err := mlang.SetLanguage(tag); err != nil {
		t.Fatal(err)
	}
}

func TestLanguageChanges(t *testing.T) {
	preserveLanguage(t)
	setLanguage(t, "en")
	calls := 0
	h := mlang.ConnectChanged(func() {
		calls++
		if mlang.Language() != "zh-CN" {
			t.Fatal("notification preceded the committed language")
		}
	})
	t.Cleanup(h.Disconnect)
	if calls != 0 {
		t.Fatal("connecting emitted an initial notification")
	}
	setLanguage(t, "ZH-cn")
	setLanguage(t, "zh-CN")
	if calls != 1 || mlang.Language() != "zh-CN" {
		t.Fatalf("same canonical language notified again: %d, %q", calls, mlang.Language())
	}
	for _, invalid := range []string{"", "%%%", "en-@@"} {
		if err := mlang.SetLanguage(invalid); err == nil {
			t.Fatalf("accepted invalid tag %q", invalid)
		}
	}
	if calls != 1 || mlang.Language() != "zh-CN" {
		t.Fatal("invalid preference mutated the current language")
	}
	h.Disconnect()
	setLanguage(t, "fr")
	if calls != 1 {
		t.Fatal("disconnected listener was called")
	}
}

func TestLanguageHelpers(t *testing.T) {
	preserveLanguage(t)
	for _, tc := range []struct {
		tag    string
		helper func(string) mlang.Translation
	}{
		{"en", mlang.En}, {"zh-CN", mlang.ZhCN}, {"ja", mlang.Ja}, {"ko", mlang.Ko},
		{"de", mlang.De}, {"fr", mlang.Fr}, {"ru", mlang.Ru}, {"ar", mlang.Ar},
	} {
		setLanguage(t, tc.tag)
		if got := mlang.String(mlang.Translation{}, tc.helper("translated"))(); got != "translated" {
			t.Fatalf("%s: helper returned %q", tc.tag, got)
		}
		// A helper and Text define the same canonical language, including ZhCN's
		// region. Duplicate validation must not permit two spellings of one tag.
		func() {
			defer func() {
				if recover() == nil {
					t.Errorf("%s: helper disagrees with Text", tc.tag)
				}
			}()
			mlang.String(tc.helper("one"), mlang.Text(tc.tag, "two"))
		}()
	}
}

func TestStringMatchingFallbackAndEmpty(t *testing.T) {
	preserveLanguage(t)
	text := mlang.String(mlang.En("Save"), mlang.ZhCN("保存"),
		mlang.Text("zh-Hant", "儲存"), mlang.Text("ja", "保存"),
		mlang.Text("ko", "저장"), mlang.Text("de", "Speichern"),
		mlang.Text("fr", "Enregistrer"), mlang.Text("ru", "Сохранить"),
		mlang.Text("ar", "حفظ"), mlang.Text("pt", ""))
	for _, tc := range []struct{ tag, want string }{
		{"en", "Save"}, {"en-US", "Save"}, {"zh-CN", "保存"},
		{"zh-Hans", "保存"}, {"zh-TW", "儲存"}, {"zh-Hant-HK", "儲存"},
		{"ja-JP", "保存"}, {"ko-KR", "저장"}, {"de-DE", "Speichern"},
		{"fr-FR", "Enregistrer"}, {"ru-RU", "Сохранить"}, {"ar-SA", "حفظ"},
		{"pt-BR", ""}, {"es", "Save"}, {"und", "Save"}, {"en", "Save"},
	} {
		setLanguage(t, tc.tag)
		for range 2 { // cold match and the last-language cache
			if got := text(); got != tc.want {
				t.Fatalf("%s: got %q, want %q", tc.tag, got, tc.want)
			}
		}
		if mlang.Language() != tc.tag {
			t.Fatalf("matching replaced requested language %s with %s", tc.tag, mlang.Language())
		}
	}
	setLanguage(t, "en")
	fallback := mlang.String(mlang.Text("ja", "既定"))
	if fallback() != "既定" || mlang.String(mlang.Translation{})() != "" {
		t.Fatal("fallback depends on a global default language or empty sentinel")
	}
}

func TestStringDefinitionValidation(t *testing.T) {
	for _, define := range []func(){
		func() { mlang.Text("%%%", "bad") },
		func() { mlang.String(mlang.En("one"), mlang.Text("EN", "two")) },
		func() { mlang.String(mlang.Text("en-US-u-ca-gregory", "one"), mlang.Text("EN-us-u-ca-gregory", "two")) },
	} {
		func() {
			defer func() {
				if recover() == nil {
					t.Error("invalid resource definition did not panic")
				}
			}()
			define()
		}()
	}
}

func TestLanguageCanonicalVariantsAndExtensions(t *testing.T) {
	preserveLanguage(t)
	setLanguage(t, "en")
	text := mlang.String(mlang.En("English"), mlang.Text("de", "Deutsch"))
	calls := 0
	h := mlang.ConnectChanged(func() { calls++ })
	t.Cleanup(h.Disconnect)
	setLanguage(t, "DE-de-u-co-phonebk")
	setLanguage(t, "de-DE-u-co-phonebk")
	if calls != 1 || mlang.Language() != "de-DE-u-co-phonebk" || text() != "Deutsch" {
		t.Fatal("extended tags lost canonical identity or base-language matching")
	}
}

func TestChangedReentrancyAndBlocking(t *testing.T) {
	preserveLanguage(t)
	setLanguage(t, "en")
	calls := 0
	h := mlang.ConnectChanged(func() {
		calls++
		if mlang.Language() == "ja" {
			setLanguage(t, "ko") // no state lock is held while emitting
		}
	})
	t.Cleanup(h.Disconnect)
	h.Block()
	setLanguage(t, "de")
	h.Unblock()
	setLanguage(t, "ja")
	if calls != 2 || mlang.Language() != "ko" {
		t.Fatalf("reentrant change/blocking failed: calls=%d language=%s", calls, mlang.Language())
	}
}

func TestConcurrentResourcesAndLanguageReads(t *testing.T) {
	preserveLanguage(t)
	setLanguage(t, "en")
	text := mlang.String(mlang.En("Save"), mlang.ZhCN("保存"))
	var wg sync.WaitGroup
	for range 8 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for range 500 {
				if got := text(); got != "Save" && got != "保存" {
					t.Errorf("invalid concurrent result %q", got)
				}
				_ = mlang.Language()
				if mlang.String(mlang.En("temporary"))() != "temporary" {
					t.Error("independent resource construction corrupted another resource")
				}
			}
		}()
	}
	for range 100 {
		setLanguage(t, "zh-CN")
		setLanguage(t, "en")
	}
	wg.Wait()
}

func TestParameterizedResource(t *testing.T) {
	preserveLanguage(t)
	format := mlang.String(mlang.En("%[2]s: %[1]d"), mlang.ZhCN("%[1]d 个%[2]s"))
	message := func(count int, name string) string { return fmt.Sprintf(format(), count, name) }
	setLanguage(t, "en")
	if got := message(3, "files"); got != "files: 3" {
		t.Fatal(got)
	}
	setLanguage(t, "zh-CN")
	if got := message(3, "文件"); got != "3 个文件" {
		t.Fatal(got)
	}
}
