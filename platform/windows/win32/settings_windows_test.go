package win32

import (
	"testing"

	"golang.org/x/text/language"
)

// Calls the real Windows language API, without creating a window or event loop.
func TestSettingsLanguageNative(t *testing.T) {
	got, err := (Settings{}).Language()
	if err != nil {
		t.Fatal(err)
	}
	if tag, err := language.Parse(got); err != nil || tag == language.Und {
		t.Fatalf("invalid native language %q: %v", got, err)
	}
	again, err := (Settings{}).Language()
	if err != nil || again != got {
		t.Fatalf("unstable language: %q, %v; first %q", again, err, got)
	}
	t.Logf("native preferred UI language: %s", got)
}
