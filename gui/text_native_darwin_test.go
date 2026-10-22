package gui

import (
	"testing"

	"github.com/golang-gui/goui/platform/typography/coretext"
)

func TestNativeTextRasterViewport(t *testing.T) { testTextRasterViewport(t, coretext.NewContext) }
