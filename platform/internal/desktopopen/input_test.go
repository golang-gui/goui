package desktopopen

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func TestValidateURL(t *testing.T) {
	for _, input := range []string{
		"https://example.com/a%20b?q=%23#fragment", "mailto:help@example.com",
		"someapp://resource/123", "custom:opaque%20payload", "custom:",
		"file:///tmp/a%23b", "https://example.com/中文", "https://example.com/?q=a+b",
	} {
		t.Run(input, func(t *testing.T) {
			if err := ValidateURL(input); err != nil {
				t.Fatal(err)
			}
		})
	}
	for _, input := range []string{
		"", "example.com", "/tmp/file", "//example.com", "1bad:value",
		"https://example.com/\x00", "https://example.com/\n", "https://example.com/\x7f",
		"https://[broken", "https://example.com/%zz", "https://example.com/?q=%zz",
		"custom:bad%", "https://example.com/#%zz",
	} {
		t.Run("invalid_"+input, func(t *testing.T) {
			if err := ValidateURL(input); err == nil {
				t.Fatal("accepted invalid URL")
			}
		})
	}
	if err := ValidateURL("https://user:secret@example.com/%zz"); err == nil || strings.Contains(err.Error(), "secret") {
		t.Fatalf("validation error included credentials: %v", err)
	}
}

func TestAbsolutePath(t *testing.T) {
	cwd, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	for _, input := range []string{".", "not-created/中文 #%.txt", "~/$GOUI_OPEN_TEST", "file with spaces"} {
		got, err := AbsolutePath(input)
		if err != nil {
			t.Fatal(err)
		}
		if want := filepath.Join(cwd, input); got != want {
			t.Fatalf("%q: got %q, want %q", input, got, want)
		}
	}
	abs := filepath.Join(t.TempDir(), "中文 #%.txt")
	if got, err := AbsolutePath(abs); err != nil || got != abs {
		t.Fatalf("absolute path: %q, %v", got, err)
	}
	for _, input := range []string{"", "a\x00b"} {
		if _, err := AbsolutePath(input); err == nil {
			t.Fatalf("accepted %q", input)
		}
	}
	if runtime.GOOS == "windows" {
		for _, path := range []string{`C:\Users\中文\a #%.txt`, `\\server\share\a #%.txt`} {
			if got, err := AbsolutePath(path); err != nil || got != path {
				t.Fatalf("Windows path: %q, %v", got, err)
			}
		}
	}
}
