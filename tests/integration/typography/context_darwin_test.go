package typography_test

import (
	"github.com/golang-gui/goui/platform/typography"
	"github.com/golang-gui/goui/platform/typography/coretext"
)

func newContext() (typography.Context, error) { return coretext.NewContext() }

var nativeFamilies = []string{"Helvetica", "Menlo", "Arial"}
