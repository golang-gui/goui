package textedit_test

import (
	"github.com/golang-gui/goui/platform/typography/directwrite"
	"testing"
)

func TestNativeCharacterNavigation(t *testing.T) {
	verifyNativeCharacterNavigation(t, directwrite.NewContext, []string{"Segoe UI", "Consolas", "Arial"})
}
