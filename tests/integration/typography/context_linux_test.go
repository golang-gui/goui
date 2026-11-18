package typography_test

import (
	"github.com/golang-gui/goui/platform/typography"
	"github.com/golang-gui/goui/platform/typography/pango"
)

func newContext() (typography.Context, error) { return pango.NewContext() }

var nativeFamilies = []string{"DejaVu Sans", "monospace", "sans"}
