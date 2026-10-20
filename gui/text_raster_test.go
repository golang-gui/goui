package gui

import (
	"image"
	"image/color"
	"runtime"
	"strings"
	"testing"

	"github.com/golang-gui/goui/platform/graphics/software"
	"github.com/golang-gui/goui/platform/typography"
	"github.com/golang-gui/goui/style"
)

type rasterContext struct {
	typography.Context
	requests []image.Rectangle
}

func (c *rasterContext) NewTextLayout(s string, f typography.TextFormat, w, h float32) (typography.TextLayout, error) {
	l, err := c.Context.NewTextLayout(s, f, w, h)
	if err != nil {
		return nil, err
	}
	if len(s) > 1000 {
		return &rasterLayout{TextLayout: l, context: c}, nil
	}
	return l, nil
}

type rasterLayout struct {
	typography.TextLayout
	context *rasterContext
}

func (l *rasterLayout) Rasterize(scale float32, region image.Rectangle, buf []byte) (typography.TextBitmap, error) {
	l.context.requests = append(l.context.requests, region)
	return l.TextLayout.Rasterize(scale, region, buf)
}

// A narrow wrapped view used to request a surface taller than Cairo supports;
// a long input/no-wrap view requested an unsupported full-width surface. Paint
// repeatedly through the real GUI painter, including the far end of each view.
func testTextRasterViewport(t *testing.T, newContext func() (typography.Context, error)) {
	t.Helper()
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()
	ctx, err := newContext()
	if err != nil {
		t.Fatal(err)
	}
	defer ctx.Destroy()
	traced := &rasterContext{Context: ctx}
	setTestApplication(t, traced)
	App.SetStyleSheet(style.Sheet(append(DefaultStyleRules(), style.Name("text-input").TextColor(color.NRGBA{R: 255, A: 255}), style.Name("text-view").TextColor(color.NRGBA{R: 255, A: 255}))...))
	backend, err := software.NewPainter(&widgetRenderSurface{})
	if err != nil {
		t.Fatal(err)
	}
	defer backend.Destroy()
	for _, mode := range []string{"input", "nowrap", "wrapped"} {
		func() {
			text := strings.Repeat("abcdefghij", 8000)
			win := &window{}
			win.width, win.height, win.painter = 600, 260, backend
			var root Widget
			var selectEnd func()
			var editor *textEditor
			if mode == "input" {
				v := NewTextInput()
				v.SetText(text)
				v.SetSelection(TextSelection{})
				editor = v.textEditor
				root = v
				selectEnd = func() { v.SetSelection(TextSelection{Anchor: len(text), Caret: len(text)}) }
			} else {
				v := NewTextView()
				if mode == "nowrap" {
					v.SetWrapMode(WrapNone)
				} else {
					win.width = 48
					text = strings.Repeat("abcdefghij", 1000)
				}
				v.SetModel(NewTextModel(text))
				editor = v.textEditor
				scroll := NewScrollView()
				scroll.SetChild(v)
				root = scroll
				selectEnd = func() { v.SetSelection(TextSelection{Anchor: len(text), Caret: len(text)}) }
			}
			win.SetWidget(root)
			defer win.SetWidget(nil)
			settle := func() {
				for frame := 0; win.layoutDirty && frame < 100; frame++ {
					win.layoutContent()
				}
				if win.layoutDirty {
					t.Fatal("layout did not settle", mode)
				}
			}
			settle()
			for pass := 0; pass < 2; pass++ {
				if pass == 1 {
					selectEnd()
					settle()
					if editor.selection.Caret != len(text) || mode == "wrapped" && editor.offset.Y < 10000 || mode != "wrapped" && editor.offset.X < 40000 {
						t.Fatalf("%s: tail was not revealed: caret=%d offset=%v", mode, editor.selection.Caret, editor.offset)
					}
				}
				for _, scale := range []float32{1, 1.25, 2} {
					for paint := 0; paint < 2; paint++ {
						img, err := RenderWidget(root, scale)
						if err != nil {
							t.Fatal(mode, err)
						}
						red := false
						for y := img.Bounds().Min.Y; y < img.Bounds().Max.Y && !red; y++ {
							for x := img.Bounds().Min.X; x < img.Bounds().Max.X; x++ {
								r, g, b, a := img.At(x, y).RGBA()
								if r > g+2000 && r > b+2000 && a > 0 {
									red = true
									break
								}
							}
						}
						if !red {
							t.Fatalf("%s pass=%d scale=%g: visible text is blank", mode, pass, scale)
						}
					}
				}
			}
		}()
	}
	if len(traced.requests) == 0 {
		t.Fatal("long text was never rasterized")
	}
	for _, r := range traced.requests {
		if r.Dx() > 1024 || r.Dy() > 1024 {
			t.Fatal("GUI requested an unbounded native bitmap", r)
		}
	}
}
