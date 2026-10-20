package gui

import (
	"github.com/golang-gui/goui/platform/typography/pango"
	"testing"
)

func TestNativeTextRasterViewport(t *testing.T) { testTextRasterViewport(t, pango.NewContext) }
