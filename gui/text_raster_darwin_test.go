package gui

import (
	"github.com/golang-gui/goui/platform/typography/coretext"
	"testing"
)

func TestNativeTextRasterViewport(t *testing.T) { testTextRasterViewport(t, coretext.NewContext) }
