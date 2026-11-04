package gui

import (
	"fmt"
	"image"
	"image/color"
	"image/draw"
	"testing"

	"github.com/golang-gui/goui/core/colors"
	"github.com/golang-gui/goui/core/geometry"
	"github.com/golang-gui/goui/platform/graphics"
	"github.com/golang-gui/goui/platform/graphics/software"
	"github.com/golang-gui/goui/style"
)

// 预乘 sRGB RGBA：不透明边框的外缘只应含边框色，不能混入下方背景。
// 独立绘制边框作为颜色参考；两张独立遮罩共同筛出外轮廓的部分覆盖像素，
// 排除内边缘和中心，允许两级8位量化误差，不依赖后端AA曲线的具体形状。
func TestStyledBoxOpaqueBorderOuterPixels(t *testing.T) {
	backend, err := software.NewPainter(&widgetRenderSurface{})
	if err != nil {
		t.Fatal(err)
	}
	defer backend.Destroy()
	for _, scale := range []float32{1, 1.25, 1.5, 2} {
		for _, radius := range []geometry.CornerRadius{{TopLeft: 24, BottomRight: 12, BottomLeft: 4}, geometry.UniformCornerRadius(12)} {
			for _, alpha := range []uint8{128, 255} {
				for _, transform := range []geometry.Transform{geometry.Identity(), {A12: -1, A21: 1, TX: 80}, geometry.Scale(1.1, .8)} {
					t.Run(fmt.Sprintf("%gx/%v/alpha%d/%v", scale, radius, alpha, transform), func(t *testing.T) {
						s := style.Default().CornerRadius(radius).BackgroundColor(color.NRGBA{R: 60, G: 110, B: 160, A: alpha}).BorderWidth(4).BorderColor(color.Gray{Y: 230}).Style
						render := func(mode int) image.Image {
							im, err := backend.RenderImage(int(96*scale), int(96*scale), scale, func() {
								p := newPainter(backend, geometry.Rect(0, 0, 96, 96), scale)
								p.SetTransform(transform)
								p.SetClipRect(geometry.Rect(3, 3, 85, 85))
								r := geometry.Rect(8, 8, 64, 48)
								switch mode {
								case 0:
									PaintStyledBox(p, r, s)
								case 1:
									PaintStyledBorder(p, r, s)
								case 2:
									PaintStyledBackground(p, r, s)
								}
							})
							if err != nil {
								t.Fatal(err)
							}
							return im
						}
						combined, border, background := render(0), render(1), render(2)
						checked := 0
						for y := range combined.Bounds().Dy() {
							for x := range combined.Bounds().Dx() {
								bg := color.RGBAModel.Convert(background.At(x, y)).(color.RGBA)
								want := color.RGBAModel.Convert(border.At(x, y)).(color.RGBA)
								if bg.A < 8 || bg.A > alpha-8 || want.A < 8 || want.A > 247 {
									continue
								}
								got := color.RGBAModel.Convert(combined.At(x, y)).(color.RGBA)
								for i, v := range []uint8{got.R, got.G, got.B, got.A} {
									w := []uint8{want.R, want.G, want.B, want.A}[i]
									if delta := int(v) - int(w); delta < -2 || delta > 2 {
										t.Fatalf("外缘(%d,%d)混入背景：%v，边框参考%v", x, y, got, want)
									}
								}
								checked++
							}
						}
						if checked == 0 {
							t.Fatal("没有验收到外缘部分覆盖像素")
						}
					})
				}
			}
		}
	}
}

func TestStyledBoxAlphaComposition(t *testing.T) {
	backend, err := software.NewPainter(&widgetRenderSurface{})
	if err != nil {
		t.Fatal(err)
	}
	defer backend.Destroy()
	for _, tc := range []struct {
		name               string
		background, border color.Color
	}{
		{"半透明背景不透明边框", color.NRGBA{R: 60, G: 110, B: 160, A: 128}, color.Gray{Y: 230}},
		{"双方半透明", color.NRGBA{R: 60, G: 110, B: 160, A: 128}, color.NRGBA{R: 230, G: 230, B: 230, A: 128}},
		{"不透明背景半透明边框", color.NRGBA{R: 60, G: 110, B: 160, A: 255}, color.NRGBA{R: 230, G: 230, B: 230, A: 128}},
		{"透明背景", color.Transparent, color.Gray{Y: 230}},
		{"透明边框", color.NRGBA{R: 60, G: 110, B: 160, A: 128}, color.Transparent},
		{"无边框颜色", color.NRGBA{R: 60, G: 110, B: 160, A: 128}, nil},
	} {
		t.Run(tc.name, func(t *testing.T) {
			s := style.Default().Radius(12).BorderWidth(4).BackgroundColor(tc.background).BorderColor(tc.border).Style
			im, err := backend.RenderImage(96, 96, 1, func() {
				p := newPainter(backend, geometry.Rect(0, 0, 96, 96), 1)
				PaintStyledBox(p, geometry.Rect(8, 8, 64, 48), s)
			})
			if err != nil {
				t.Fatal(err)
			}
			// 使用标准库 Over 作为预乘参考；采样直边满覆盖处和中心，
			// 不把两个半透明层错误地当成互斥色块或预先混成不透明色。
			ref := image.NewRGBA(image.Rect(0, 0, 1, 1))
			draw.Draw(ref, ref.Bounds(), image.NewUniform(tc.background), image.Point{}, draw.Src)
			for i, point := range []image.Point{{X: 40, Y: 32}, {X: 40, Y: 9}} {
				if i == 1 && tc.border != nil {
					draw.Draw(ref, ref.Bounds(), image.NewUniform(tc.border), image.Point{}, draw.Over)
				}
				want := ref.RGBAAt(0, 0)
				got := color.RGBAModel.Convert(im.At(point.X, point.Y)).(color.RGBA)
				for j, v := range []uint8{got.R, got.G, got.B, got.A} {
					w := []uint8{want.R, want.G, want.B, want.A}[j]
					if delta := int(v) - int(w); delta < -2 || delta > 2 {
						t.Fatalf("预乘Over在%v错误：%v，标准库参考%v", point, got, want)
					}
				}
			}
		})
	}
}

// useTestApplication installs app as the global application for the duration of
// the test, restoring the previous one on cleanup. Unlike setTestApplication it
// lets a test provide a style sheet (and typography) up front.
func useTestApplication(t *testing.T, app *application) {
	t.Helper()
	old := App
	App = app
	t.Cleanup(func() {
		App = old
	})
}

// TestApplicationStyleSheetChangesButtonPaint proves a custom application style
// sheet flows all the way into what a widget actually paints, not just what the
// resolver returns.
func TestApplicationStyleSheetChangesButtonPaint(t *testing.T) {
	background := color.RGBA{R: 12, G: 24, B: 36, A: 255}
	border := color.RGBA{R: 40, G: 50, B: 60, A: 255}
	useTestApplication(t, &application{
		style: style.Sheet(
			style.Name(styleNameButton).
				BackgroundColor(background).
				BorderColor(border).
				BorderWidth(2).
				Radius(6),
		),
	})

	button := NewButton()
	button.Arrange(geometry.Rect(0, 0, 80, 30))

	painter := new(testButtonBackgroundPainter)
	button.Paint(painter)

	// 背景退到不透明边框中线，边框外轮廓及控件布局不变。
	if painter.rect != geometry.Rect(1, 1, 78, 28) || painter.radius != 5 {
		t.Fatalf("unexpected fill geometry: rect=%+v radius=%v", painter.rect, painter.radius)
	}
	if painter.brush != graphics.ColorOf(background) {
		t.Fatalf("unexpected fill brush: %+v", painter.brush)
	}
	if painter.drawRect != geometry.Rect(1, 1, 78, 28) || painter.drawRadius != 5 || painter.drawStrokeWidth != 2 || painter.drawBrush != graphics.ColorOf(border) {
		t.Fatalf("unexpected border: radius=%v width=%v brush=%+v", painter.drawRadius, painter.drawStrokeWidth, painter.drawBrush)
	}
}

func TestStyledBoxClampsBorderRadiusAfterInset(t *testing.T) {
	border := color.RGBA{R: 40, G: 50, B: 60, A: 255}
	useTestApplication(t, &application{
		style: style.Sheet(
			style.Name(styleNameButton).
				BorderColor(border).
				BorderWidth(4).
				Radius(1),
		),
	})

	button := NewButton()
	button.Arrange(geometry.Rect(0, 0, 80, 30))
	painter := new(testButtonBackgroundPainter)
	button.Paint(painter)

	// 小圆角不能因中心线退化被绘成直角；环带外轮廓仍为半径1，
	// 内部矩形从(4,4)开始，保留已有内缩宽度并使用同一边框颜色。
	var starts []geometry.Point
	curves := 0
	painter.fillPath.Range(func(op graphics.PathOperation, args []float32) bool {
		if op == graphics.PathMoveTo {
			starts = append(starts, geometry.Point{X: args[0], Y: args[1]})
		}
		if op == graphics.PathBezierTo {
			curves++
		}
		return false
	})
	if len(starts) != 2 || starts[0] != (geometry.Point{X: 1}) || starts[1] != (geometry.Point{X: 4, Y: 4}) || curves != 4 || painter.brush != graphics.ColorOf(border) {
		t.Fatalf("小圆角环带几何错误：起点%v 圆弧%d 画刷%v", starts, curves, painter.brush)
	}
}

// TestWindowApplicationStyleSheetOverridesGlobal covers the two-tier lookup in
// widgetStyleSheet: a widget mounted in a window whose application has its own
// sheet uses that sheet, not the global one.
// TestButtonHoverBackgroundFromApplicationStyleSheet checks that the widget's
// current state selects the matching rule from the application sheet, and falls
// back to normal when the state clears.
func TestButtonHoverBackgroundFromApplicationStyleSheet(t *testing.T) {
	normalBackground := color.RGBA{R: 20, G: 20, B: 20, A: 255}
	hoverBackground := color.RGBA{R: 60, G: 60, B: 60, A: 255}
	useTestApplication(t, &application{
		style: style.Sheet(
			style.Name(styleNameButton).BackgroundColor(normalBackground).Radius(3),
			style.Name(styleNameButton).State(style.Hovered).BackgroundColor(hoverBackground),
		),
	})

	button := NewButton()
	button.Arrange(geometry.Rect(0, 0, 80, 30))

	painter := new(testButtonBackgroundPainter)
	button.Paint(painter)
	if painter.brush != graphics.ColorOf(normalBackground) {
		t.Fatalf("unexpected normal background: %+v", painter.brush)
	}

	button.updateState(style.Hovered, true)
	painter = new(testButtonBackgroundPainter)
	button.Paint(painter)
	if painter.brush != graphics.ColorOf(hoverBackground) {
		t.Fatalf("unexpected hover background: %+v", painter.brush)
	}

	button.updateState(style.Hovered, false)
	painter = new(testButtonBackgroundPainter)
	button.Paint(painter)
	if painter.brush != graphics.ColorOf(normalBackground) {
		t.Fatalf("hover leave should fall back to normal background: %+v", painter.brush)
	}
}

// TestTextInputFocusedBorderFromApplicationStyleSheet drives focus through the
// real window focus path and checks the focused-state border resolves from the
// application sheet.
func TestTextInputFocusedBorderFromApplicationStyleSheet(t *testing.T) {
	normalBorder := color.RGBA{R: 100, G: 100, B: 100, A: 255}
	focusBorder := color.RGBA{R: 10, G: 200, B: 90, A: 255}
	useTestApplication(t, &application{
		style: style.Sheet(
			style.Name(styleNameTextInput).
				BackgroundColor(color.White).
				BorderColor(normalBorder).
				BorderWidth(1),
			style.Name(styleNameTextInput).
				State(style.Focused).
				BorderColor(focusBorder),
		),
	})

	win := &window{}
	input := NewTextInput()
	win.SetWidget(input)
	input.Arrange(geometry.Rect(0, 0, 100, 24))

	painter := new(testTextInputPainter)
	input.Paint(painter)
	if painter.drawRectBrush != graphics.ColorOf(normalBorder) {
		t.Fatalf("unfocused border should use the normal color: %+v", painter.drawRectBrush)
	}

	if !win.SetFocusedWidget(input) {
		t.Fatal("text input should accept focus")
	}
	painter = new(testTextInputPainter)
	input.Paint(painter)
	if painter.drawRectBrush != graphics.ColorOf(focusBorder) {
		t.Fatalf("focused border should use the focused color: %+v", painter.drawRectBrush)
	}
}

// TestStyleNameSelectsRuleSet exercises StyleName routing: the widget's style
// name is what selects which rules apply, and changing it changes resolution.
// TestResolveFallsBackToDefaultStyleName proves StyleName() is explicit (empty
// when unset) while resolveStyle falls back to the widget's type default name.
func TestResolveFallsBackToDefaultStyleName(t *testing.T) {
	buttonBg := color.RGBA{R: 7, G: 8, B: 9, A: 255}
	useTestApplication(t, &application{
		style: style.Sheet(style.Name(styleNameButton).BackgroundColor(buttonBg)),
	})

	button := NewButton()
	if button.StyleName() != "" {
		t.Fatalf("unset style override should be empty (explicit getter), got %q", button.StyleName())
	}

	bg, ok := ResolveStyle(styleNameButton, style.PartDefault, style.Normal).BackgroundColor()
	if !ok || !colors.Equal(bg, buttonBg) {
		t.Fatalf("button default name should resolve the button style: %v ok=%v", bg, ok)
	}
}

func TestStyleNameSelectsRuleSet(t *testing.T) {
	setTestApplication(t, nil)

	// ResolveStyle routes purely by name: the generic widget name resolves the
	// base (transparent) style, a different name selects a different ruleset.
	background, ok := ResolveStyle(styleNameWidget, style.PartDefault, style.Normal).BackgroundColor()
	if !ok || !colors.Equal(background, color.Transparent) {
		t.Fatalf("widget style name should resolve transparent background: %v ok=%v", background, ok)
	}

	resolved := ResolveStyle(styleNameButton, style.PartDefault, style.Normal)
	background, ok = resolved.BackgroundColor()
	if !ok || !colors.Equal(background, color.RGBA{R: 210, G: 210, B: 210, A: 255}) {
		t.Fatalf("button style name should resolve button background: %v ok=%v", background, ok)
	}
	if radius, ok := resolved.CornerRadius(); !ok || radius != geometry.UniformCornerRadius(4) {
		t.Fatalf("button style name should resolve button radius: %v ok=%v", radius, ok)
	}
}
