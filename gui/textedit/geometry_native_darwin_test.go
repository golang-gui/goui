package textedit_test

import (
	"github.com/golang-gui/goui/platform/typography/coretext"
	"testing"
)

func TestNativeCharacterNavigation(t *testing.T) {
	verifyNativeCharacterNavigation(t, coretext.NewContext, []string{"Helvetica", "Menlo", "Arial"})
}
