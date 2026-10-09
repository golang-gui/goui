package x11

import (
	"os"
	"os/exec"
	"testing"
)

func TestLocaleLanguage(t *testing.T) {
	for _, tc := range []struct{ locale, want string }{
		{"en_US.UTF-8", "en-US"}, {"zh_CN.UTF-8", "zh-CN"}, {"ja_JP", "ja-JP"},
		{"ko_KR.utf8", "ko-KR"}, {"de_DE@euro", "de-DE"}, {"fr_FR", "fr-FR"},
		{"ru_RU", "ru-RU"}, {"ar_SA", "ar-SA"}, {"sr_RS@latin", "sr-Latn-RS"},
		{"sr_RS@cyrillic", "sr-Cyrl-RS"}, {"ca_ES@valencia", "ca-ES-valencia"},
		{"C", "und"}, {"C.UTF-8", "und"}, {"POSIX", "und"},
	} {
		got, err := localeLanguage(tc.locale)
		if err != nil || got != tc.want {
			t.Errorf("%q: %q, %v; want %q", tc.locale, got, err, tc.want)
		}
	}
	for _, locale := range []string{"", "not a locale", "zh_CN@unknown-script"} {
		if _, err := localeLanguage(locale); err == nil {
			t.Errorf("accepted invalid locale %q", locale)
		}
	}
}

// GLib uses process locale preferences without a display. Isolate its per-thread
// cache and environment in child processes; do not mutate the parent test suite.
func TestSettingsLanguageNative(t *testing.T) {
	if want := os.Getenv("GOUI_TEST_LANGUAGE_EXPECT"); want != "" {
		got, err := (Settings{}).Language()
		if err != nil || got != want {
			t.Fatalf("native language=%q, %v; want %q", got, err, want)
		}
		t.Logf("native preferred UI language: %s", got)
		return
	}
	for _, tc := range []struct{ preference, want string }{
		{"ja_JP.UTF-8:en_US", "ja-JP"}, {"zh_CN:en", "zh-CN"}, {"C", "und"},
	} {
		cmd := exec.Command(os.Args[0], "-test.run=^TestSettingsLanguageNative$", "-test.v")
		cmd.Env = append(os.Environ(), "DISPLAY=", "LC_ALL=C", "LANG=C", "LANGUAGE="+tc.preference, "GOUI_TEST_LANGUAGE_EXPECT="+tc.want)
		output, err := cmd.CombinedOutput()
		if err != nil {
			t.Fatalf("%q: %v\n%s", tc.preference, err, output)
		}
	}
}
