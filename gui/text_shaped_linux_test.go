package gui

import (
	"image/color"
	"runtime"
	"strings"
	"testing"

	"github.com/golang-gui/goui/core/geometry"
	"github.com/golang-gui/goui/platform/typography"
	"github.com/golang-gui/goui/platform/typography/pango"
)

type countingGlyphContext struct {
	typography.Context
	created, destroyed int
}

func (c *countingGlyphContext) NewGlyphLayout(text string, f typography.TextFormat) (typography.GlyphLayout, error) {
	l, err := c.Context.(typography.GlyphContext).NewGlyphLayout(text, f)
	if err != nil {
		return nil, err
	}
	c.created++
	return &countingGlyphLayout{GlyphLayout: l, owner: c}, nil
}

type countingGlyphLayout struct {
	typography.GlyphLayout
	owner *countingGlyphContext
}

func (l *countingGlyphLayout) Destroy() { l.owner.destroyed++; l.GlyphLayout.Destroy() }

func TestTextViewShapedWidthsShareNativeSource(t *testing.T) {
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()
	native, err := pango.NewContext()
	if err != nil {
		t.Fatal(err)
	}
	defer native.Destroy()
	ctx := &countingGlyphContext{Context: native}
	setTestApplication(t, ctx)
	v := NewTextView()
	defer v.releaseParagraphs()
	v.SetWrapMode(WrapWordChar)
	v.Model().SetText("start \u202e" + strings.Repeat("abc 123 العربية עברית e\u0301 ffi 👩‍💻\t ", 1000) + "END\u202c")
	v.Arrange(geometry.Rect(0, 0, 240, 200))
	a := v.paragraph(0)
	if a == nil || a.shape == nil || a.fragment.LineCount() < 100 {
		t.Fatal("long Unicode input missed shaped path")
	}
	for width := float32(250); width < 270; width++ {
		v.Arrange(geometry.Rect(0, 0, width, 200))
		b := v.paragraph(0)
		if b.shape != a.shape || b == a {
			t.Fatal("new width did not share immutable source")
		}
	}
	if ctx.created != 1 || ctx.destroyed != 0 {
		t.Fatal("width changes shaped or prematurely released source", ctx.created, ctx.destroyed)
	}
	// Paint color changes cannot mutate the source shared by recent widths.
	v.format.TextColor = color.NRGBA{B: 120, A: 190}
	if a.layout.Format().TextColor == v.format.TextColor {
		t.Fatal("view color unexpectedly mutated native source")
	}
	other := NewTextView()
	other.SetWrapMode(WrapWordChar)
	other.Model().SetText(v.Model().Text())
	other.Arrange(geometry.Rect(0, 0, 240, 200))
	if other.paragraph(0).shape == v.paragraph(0).shape || ctx.created != 2 {
		t.Fatal("different views shared native source")
	}
	other.releaseParagraphs()
	if ctx.destroyed != 1 {
		t.Fatal("second view did not release independent source")
	}
	v.releaseParagraphs()
	if ctx.created != ctx.destroyed {
		t.Fatal("width projections leaked source", ctx.created, ctx.destroyed)
	}
}
