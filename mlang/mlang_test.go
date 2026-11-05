package mlang_test

import (
	"fmt"
	"runtime"
	"sync"
	"sync/atomic"
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

// 通知中的 Getter 必须看到已提交的语言；重入切换后不能继续返回旧缓存。
func TestChangedReadsStringCache(t *testing.T) {
	preserveLanguage(t)
	setLanguage(t, "en")
	text := mlang.String(mlang.En("English"), mlang.ZhCN("中文"), mlang.Ko(""))
	if got := text(); got != "English" {
		t.Fatal(got)
	}
	var observed []string
	h := mlang.ConnectChanged(func() {
		tag := mlang.Language()
		observed = append(observed, tag)
		want, ok := map[string]string{"zh-CN": "中文", "ko": ""}[tag]
		if !ok || text() != want {
			t.Fatalf("通知中读到旧缓存：language=%q text=%q", tag, text())
		}
		if tag == "zh-CN" {
			setLanguage(t, "ko")
			if got := text(); got != "" {
				t.Fatalf("重入切换后缓存未更新：%q", got)
			}
		}
	})
	t.Cleanup(h.Disconnect)
	setLanguage(t, "zh-CN")
	if len(observed) != 2 || observed[0] != "zh-CN" || observed[1] != "ko" || text() != "" {
		t.Fatalf("重入通知或缓存错误：notifications=%v text=%q", observed, text())
	}
}

// 每轮等待全部读取完成再切换语言，明确断言切换后的结果，避免仅检查允许值集合。
func TestConcurrentStringCacheAfterLanguageChange(t *testing.T) {
	preserveLanguage(t)
	first := mlang.String(mlang.En("Save"), mlang.ZhCN("保存"))
	second := mlang.String(mlang.Ja("既定"), mlang.ZhCN(""), mlang.Ar("حفظ"))
	for range 3 {
		for _, tc := range []struct{ tag, first, second string }{
			{"en", "Save", "既定"}, {"zh-CN", "保存", ""},
			{"ar", "Save", "حفظ"}, {"und", "Save", "既定"},
			{"en-US", "Save", "既定"},
		} {
			setLanguage(t, tc.tag)
			var wg sync.WaitGroup
			start := make(chan struct{})
			for range 8 {
				wg.Add(1)
				go func() {
					defer wg.Done()
					<-start
					for range 100 {
						if a, b := first(), second(); a != tc.first || b != tc.second {
							t.Errorf("%s：读到 %q/%q，预期 %q/%q", tc.tag, a, b, tc.first, tc.second)
							return
						}
					}
					if got := mlang.Language(); got != tc.tag {
						t.Errorf("当前语言 %q，预期 %q", got, tc.tag)
					}
				}()
			}
			close(start)
			wg.Wait()
			if allocs := testing.AllocsPerRun(100, func() { _, _ = first(), second() }); allocs != 0 {
				t.Fatalf("%s：缓存命中产生了分配：%g", tc.tag, allocs)
			}
		}
	}
}

// 并发提交同一规范化语言仍只通知一次；写入串行化不应随免锁读取优化而丢失。
func TestConcurrentEquivalentLanguageChanges(t *testing.T) {
	preserveLanguage(t)
	setLanguage(t, "en")
	text := mlang.String(mlang.En("English"), mlang.De("Deutsch"))
	_ = text()
	var calls atomic.Int32
	h := mlang.ConnectChanged(func() {
		calls.Add(1)
		if mlang.Language() != "de-DE-u-co-phonebk" || text() != "Deutsch" {
			t.Error("并发提交的通知先于语言或匹配结果生效")
		}
	})
	t.Cleanup(h.Disconnect)
	var wg sync.WaitGroup
	start := make(chan struct{})
	for _, tag := range []string{"de-DE-u-co-phonebk", "DE-de-u-co-phonebk"} {
		for range 8 {
			wg.Add(1)
			go func() {
				defer wg.Done()
				<-start
				if err := mlang.SetLanguage(tag); err != nil {
					t.Error(err)
				}
			}()
		}
	}
	close(start)
	wg.Wait()
	if got := calls.Load(); got != 1 {
		t.Fatalf("同一规范化语言通知了 %d 次", got)
	}
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

// 分别测量稳定语言下的单线程读取与并发读取；资源创建和语言切换不计入耗时。
func BenchmarkString(b *testing.B) {
	previous := mlang.Language()
	b.Cleanup(func() { _ = mlang.SetLanguage(previous) })
	if err := mlang.SetLanguage("zh-CN"); err != nil {
		b.Fatal(err)
	}
	for _, tc := range []struct {
		name         string
		translations []mlang.Translation
	}{
		{"SingleTranslation", []mlang.Translation{mlang.En("Save")}},
		{"EightTranslations", []mlang.Translation{
			mlang.En("Save"), mlang.ZhCN("保存"), mlang.Ja("保存"), mlang.Ko("저장"),
			mlang.De("Speichern"), mlang.Fr("Enregistrer"), mlang.Ru("Сохранить"), mlang.Ar("حفظ"),
		}},
	} {
		b.Run(tc.name, func(b *testing.B) {
			text := mlang.String(tc.translations[0], tc.translations[1:]...)
			_ = text()
			b.Run("Cached", func(b *testing.B) {
				b.ReportAllocs()
				var got string
				for range b.N {
					got = text()
				}
				runtime.KeepAlive(got)
			})
			b.Run("ParallelCached", func(b *testing.B) {
				b.ReportAllocs()
				b.RunParallel(func(pb *testing.PB) {
					var got string
					for pb.Next() {
						got = text()
					}
					runtime.KeepAlive(got)
				})
			})
		})
	}
}

// Language 的规范化字符串读取单独测量，避免与词条匹配或信号开销混合。
func BenchmarkLanguage(b *testing.B) {
	previous := mlang.Language()
	b.Cleanup(func() { _ = mlang.SetLanguage(previous) })
	if err := mlang.SetLanguage("de-DE-u-co-phonebk"); err != nil {
		b.Fatal(err)
	}
	b.ReportAllocs()
	b.ResetTimer()
	var got string
	for range b.N {
		got = mlang.Language()
	}
	runtime.KeepAlive(got)
}
