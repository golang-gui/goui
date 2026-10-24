package gui

import (
	"testing"

	"github.com/golang-gui/goui/platform/typography/directwrite"
)

func TestNativeTextRasterViewport(t *testing.T) { testTextRasterViewport(t, directwrite.NewContext) }

func TestParagraphLayoutDirectWrite(t *testing.T) {
	testParagraphLayoutNative(t, directwrite.NewContext)
}

func TestTextEditorDirectWriteNavigation(t *testing.T) {
	testTextEditorNativeNavigation(t, directwrite.NewContext)
}

func TestTextViewDirectWriteRevealEnd(t *testing.T) {
	testTextViewNativeRevealEnd(t, directwrite.NewContext)
}

func TestTextViewDirectWriteResizeAnchor(t *testing.T) {
	testTextViewNativeResizeAnchor(t, directwrite.NewContext)
}
