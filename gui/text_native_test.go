package gui

import (
	"fmt"
	"image"
	"image/color"
	"runtime"
	"strings"
	"testing"

	"github.com/golang-gui/goui/core/geometry"
	"github.com/golang-gui/goui/layout"
	"github.com/golang-gui/goui/platform/events"
	"github.com/golang-gui/goui/platform/graphics/software"
	"github.com/golang-gui/goui/platform/typography"
	"github.com/golang-gui/goui/style"
)

// Native bidi and cluster navigation.

// 原生排版共享验证（入口保留平台后缀）。

// Native shaping without a display: verify editing through the normal dispatcher
// using real native clusters. Assertions are byte boundaries and visual ordering,
// not font-dependent pixel widths. This is not a native window/IME acceptance test.
func testTextEditorNativeNavigation(t *testing.T, newContext func() (typography.Context, error)) {
	t.Helper()
	for _, single := range []bool{false, true} {
		name := "multiline"
		if single {
			name = "singleline"
		}
		t.Run(name, func(t *testing.T) {
			runtime.LockOSThread()
			defer runtime.UnlockOSThread()
			typo, err := newContext()
			if err != nil {
				t.Fatal(err)
			}
			defer typo.Destroy()
			setTestApplication(t, typo)
			var widget Widget
			var editor *textEditor
			if single {
				input := NewTextInput()
				input.SetText("אבג")
				widget, editor = input, input.textEditor
			} else {
				view := NewTextView()
				view.SetModel(NewTextModel("אבג"))
				widget, editor = view, view.textEditor
			}
			win := &window{}
			win.SetWidget(widget)
			defer win.SetWidget(nil) // all native layout cleanup precedes UnlockOSThread
			widget.Arrange(geometry.Rect(0, 0, 200, 80))
			win.SetFocusedWidget(widget)
			editor.setSelectionValue(TextSelection{0, 6})
			editorKey(t, win, events.KeyArrowLeft, 0)
			if editor.selection != (TextSelection{6, 6}) {
				t.Fatalf("RTL selection collapsed left to %+v, want byte 6", editor.selection)
			}
			before, _ := editor.caretRect()
			editorKey(t, win, events.KeyArrowRight, events.ModifierShift)
			after, _ := editor.caretRect()
			if editor.selection != (TextSelection{6, 4}) || after.X <= before.X {
				t.Fatalf("RTL Shift+Right: selection=%+v x=%g -> %g", editor.selection, before.X, after.X)
			}
			editorKey(t, win, events.KeyBackspace, 0)
			if editor.model.Text() != "אב" {
				t.Fatalf("deleted wrong RTL cluster: %q", editor.model.Text())
			}
			editorKey(t, win, events.KeyZ, textCommandModifier())
			if editor.model.Text() != "אבג" || editor.selection != (TextSelection{6, 4}) {
				t.Fatal("undo did not restore RTL text and directional selection")
			}
			editor.model.SetText("e\u0301x")
			editor.setSelectionValue(TextSelection{0, 0})
			editorKey(t, win, events.KeyArrowRight, 0)
			if editor.selection.Caret != 3 {
				t.Fatalf("navigation split native combining cluster: %+v", editor.selection)
			}
			editorKey(t, win, events.KeyBackspace, 0)
			if editor.model.Text() != "x" {
				t.Fatalf("deletion split native combining cluster: %q", editor.model.Text())
			}
		})
	}
}

// Native reveal and resize anchor geometry.

// 原生排版的锚定与定位验证。

func testTextViewNativeRevealEnd(t *testing.T, newContext func() (typography.Context, error)) {
	t.Helper()
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()
	typo, err := newContext()
	if err != nil {
		t.Fatal(err)
	}
	defer typo.Destroy()
	setTestApplication(t, typo)
	var text strings.Builder
	for row := 0; row < 95000; row++ {
		fmt.Fprintf(&text, "[%06d:start] text editing, viewport layout; [%06d:middle] selection and undo; [%06d:end] 中文测试。\n", row, row, row)
	}
	editor := NewTextView()
	editor.SetModel(NewTextModel(text.String()))
	scroll := NewScrollView()
	scroll.SetChild(editor)
	win := &window{}
	win.width, win.height = 507, 590
	win.SetWidget(scroll)
	defer win.SetWidget(nil)
	settle := func() {
		for frame := 0; win.layoutDirty && frame < 100; frame++ {
			win.layoutContent()
		}
		if win.layoutDirty {
			t.Fatal("native layout requests did not settle")
		}
	}
	settle()
	win.SetFocusedWidget(editor)
	scroll.SetScrollY(900)
	settle()
	for _, width := range []float32{441, 591, 507} {
		win.width = width
		win.RequestLayout()
		settle()
	}
	editorKey(t, win, events.KeyHome, textCommandModifier())
	editorKey(t, win, events.KeyEnd, textCommandModifier()|events.ModifierShift)
	settle()
	caret, ok := editor.caretRect()
	if !ok || editor.offset.Y != scroll.ScrollY() || caret.Y < editor.offset.Y || caret.Y+caret.Height > editor.offset.Y+editor.viewport.Height || editor.revealCaret || editor.anchorPending || scroll.revealPending {
		t.Fatalf("native scheduled layouts stopped before revealing caret: caret=%v scroll=%g viewport=%v pending=%t/%t/%t", caret, scroll.ScrollY(), editor.viewport, editor.revealCaret, editor.anchorPending, scroll.revealPending)
	}
}

func testTextViewNativeResizeAnchor(t *testing.T, newContext func() (typography.Context, error)) {
	t.Helper()
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()
	typo, err := newContext()
	if err != nil {
		t.Fatal(err)
	}
	defer typo.Destroy()
	setTestApplication(t, typo)
	var text strings.Builder
	for row := 0; row < 1000; row++ {
		fmt.Fprintf(&text, "[%06d:start] text editing, viewport layout; [%06d:middle] selection and undo; [%06d:end] 中文测试。\n", row, row, row)
	}
	editor := NewTextView()
	editor.SetModel(NewTextModel(text.String()))
	scroll := NewScrollView()
	scroll.SetChild(editor)
	win := &window{}
	win.SetWidget(scroll)
	defer win.SetWidget(nil)
	arrange := func(width float32) {
		r := geometry.Rect(0, 0, width, 590)
		scroll.Measure(layout.Tight(r.Size))
		for range 8 {
			scroll.Arrange(r)
		}
	}
	arrange(507)
	for offset := float32(890); offset < 918; offset++ {
		arrange(507)
		scroll.SetScrollY(0)
		scroll.SetScrollY(offset)
		// Paint clips at padding, and translates text by padding-offset:
		// the first visible content coordinate is therefore offset, not
		// offset-padding. Native metrics are local to the paragraph layout,
		// whose painted origin also includes the baseline's textOffset.
		// Derive the anchor from those metrics, not editor.anchor.
		oldIndex := editor.heights.At(scroll.ScrollY())
		oldRange, _ := editor.model.LineRange(oldIndex)
		oldParagraph := editor.paragraph(oldIndex)
		oldLines, _ := oldParagraph.layout.MeasureMetrics()
		oldY := scroll.ScrollY() - editor.heights.Top(oldIndex) - oldParagraph.textOffset
		oldLine := 0
		for oldLine+1 < len(oldLines) && oldLines[oldLine+1].Y <= oldY {
			oldLine++
		}
		old := textViewportAnchor{offset: oldRange.Start + oldLines[oldLine].Start, within: oldY - oldLines[oldLine].Y}
		// Retain the original model character for the whole resize sequence.
		// Re-sampling each new display line would reproduce the drift bug in
		// the expected value and miss loss of position on a width round trip.
		for _, width := range []float32{457, 441, 482, 532, 591, 532, 482, 441, 507} {
			arrange(width)
			index := editor.model.LineAt(old.offset)
			rng, _ := editor.model.LineRange(index)
			p := editor.paragraph(index)
			lines, _ := p.layout.MeasureMetrics()
			line := 0
			for line+1 < len(lines) && lines[line+1].Start <= old.offset-rng.Start {
				line++
			}
			want := editor.heights.Top(index) + p.textOffset + lines[line].Y + old.within
			// ScrollView rounds to integral DIP. Native line metrics need not be
			// integral, but a whole line/paragraph jump is never rounding error.
			if abs32(scroll.ScrollY()-want) > 1 {
				t.Fatalf("width=%g old=%+v new=%+v scroll=%g want=%g", width, old, editor.anchor, scroll.ScrollY(), want)
			}
			if editor.anchorPending || scroll.revealPending {
				t.Fatalf("resize did not settle: width=%g old=%+v scroll=%g contentOffset=%g anchor=%+v pending=%t/%t", width, old, scroll.ScrollY(), editor.offset.Y, editor.anchor, editor.anchorPending, scroll.revealPending)
			}
		}
	}
}

// Native viewport rasterization through the software painter.

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
