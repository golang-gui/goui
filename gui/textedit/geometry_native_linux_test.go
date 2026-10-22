package textedit_test

import (
	"github.com/golang-gui/goui/platform/typography/pango"
	"testing"
)

func TestNativeCharacterNavigation(t *testing.T) {
	verifyNativeCharacterNavigation(t, pango.NewContext, []string{"DejaVu Sans", "monospace", "sans"})
}
