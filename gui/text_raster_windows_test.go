package gui

import (
	"github.com/golang-gui/goui/platform/typography/directwrite"
	"testing"
)

func TestNativeTextRasterViewport(t *testing.T) { testTextRasterViewport(t, directwrite.NewContext) }
