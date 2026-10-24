package gui

import (
	"testing"

	"github.com/golang-gui/goui/platform/typography/coretext"
)

func TestNativeTextRasterViewport(t *testing.T) { testTextRasterViewport(t, coretext.NewContext) }

func TestParagraphLayoutCoreText(t *testing.T) { testParagraphLayoutNative(t, coretext.NewContext) }

func TestTextEditorCoreTextNavigation(t *testing.T) {
	testTextEditorNativeNavigation(t, coretext.NewContext)
}

func TestTextViewCoreTextRevealEnd(t *testing.T) {
	testTextViewNativeRevealEnd(t, coretext.NewContext)
}

func TestTextViewCoreTextResizeAnchor(t *testing.T) {
	testTextViewNativeResizeAnchor(t, coretext.NewContext)
}

func TestParagraphCoordinatesNative(t *testing.T) {
	testParagraphCoordinatesNative(t, coretext.NewContext)
}
