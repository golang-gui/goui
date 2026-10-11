package x11

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// No desktop: an isolated helper emulates stdout, successful stderr warnings,
// user cancellation and process errors through the actual exec boundary.
func TestDialogHelperOutputAndErrors(t *testing.T) {
	dir := t.TempDir()
	script := "#!/bin/sh\nprintf 'helper warning\\n' >&2\nprintf '%s\\n' \"$GOUI_DIALOG_TEST_PATH\"\nexit \"$GOUI_DIALOG_TEST_EXIT\"\n"
	if err := os.WriteFile(filepath.Join(dir, "zenity"), []byte(script), 0700); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(dir, "filename with trailing spaces  ")
	t.Setenv("PATH", dir)
	t.Setenv("GOUI_DIALOG_TEST_PATH", path)
	t.Setenv("GOUI_DIALOG_TEST_EXIT", "0")
	output, err := runDialog(dialogZenity, nil)
	if err != nil || output != path {
		t.Fatalf("diagnostic/whitespace corrupted filename: %q %v", output, err)
	}
	t.Setenv("GOUI_DIALOG_TEST_EXIT", "1")
	if _, err := runDialog(dialogZenity, nil); !errors.Is(err, errDialogCancelled) {
		t.Fatal(err)
	}
	t.Setenv("GOUI_DIALOG_TEST_EXIT", "2")
	if _, err := runDialog(dialogZenity, nil); err == nil || errors.Is(err, errDialogCancelled) || !strings.Contains(err.Error(), "helper warning") {
		t.Fatal(err)
	}
}
