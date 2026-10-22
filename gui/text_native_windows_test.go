package gui

import (
	"testing"

	"github.com/golang-gui/goui/platform/typography/directwrite"
)

func TestNativeTextRasterViewport(t *testing.T) { testTextRasterViewport(t, directwrite.NewContext) }
