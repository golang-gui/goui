package test

import (
	"bytes"
	"image"
	"image/color"
	"image/png"
	"os"
	"testing"

	"github.com/golang-gui/goui/platform"
	"github.com/golang-gui/goui/platform/graphics"
	"github.com/golang-gui/goui/platform/graphics/software"
	"github.com/golang-gui/goui/platform/typography"
)

func TestTypography(t *testing.T) {
	plat, err := platform.NewPlatform(platform.DefaultName())
	if err != nil {
		t.Fatal(err)
	}

	ctx, err := plat.NewTypography()
	if err != nil {
		t.Fatal(err)
	}
	defer ctx.Destroy()

	err = ctx.AddFont("testdata/MiSansLatin-Normal.ttf")
	if err != nil {
		t.Log(err)
	}

	red := graphics.RGB(180, 0, 0)
	green := graphics.RGB(0, 180, 0)
	blue := graphics.RGB(0, 0, 180)
	gray := graphics.RGB(160, 160, 160)
	white := graphics.RGB(250, 250, 250)

	fontName := "MiSans Latin Normal"
	wrap := typography.WrapWordChar
	align := typography.TextAlignEnd

	t.Run("text", func(t *testing.T) {
		layout, err := newRichTextLayout(ctx, 200, 100, fontName, wrap, align, white)
		if err != nil {
			t.Fatal(err)
		}
		defer layout.Destroy()

		bitmap, err := ctx.DrawTextLayout(layout, 1.0, nil)
		if err != nil {
			t.Fatal(err)
		}

		err = savePng("text-1x.png", bitmap)
		if err != nil {
			t.Fatal(err)
		}

		bitmap, err = ctx.DrawTextLayout(layout, 2.0, nil)
		if err != nil {
			t.Fatal(err)
		}

		err = savePng("text-2x.png", bitmap)
		if err != nil {
			t.Fatal(err)
		}
	})

	t.Run("normal", func(t *testing.T) {
		layout, err := newRichTextLayout(ctx, 200, 100, fontName, wrap, align, white)
		if err != nil {
			t.Fatal(err)
		}

		bitmap, err := ctx.DrawTextLayout(layout, 1.0, nil)
		if err != nil {
			t.Fatal(err)
		}

		err = savePng("normal-text.png", bitmap)
		if err != nil {
			t.Fatal(err)
		}

		painter, err := newTestPainter(ctx)
		if err != nil {
			t.Fatal(err)
		}

		painter.Begin(200, 100)
		drawLayoutMetrics(painter, layout, green, blue, gray, red)
		painter.End()

		err = savePng("normal-output.png", painter.result)
		if err != nil {
			t.Fatal(err)
		}
	})

	t.Run("small", func(t *testing.T) {
		layout, err := newRichTextLayout(ctx, 200, 20, fontName, wrap, align, white)
		if err != nil {
			t.Fatal(err)
		}

		bitmap, err := ctx.DrawTextLayout(layout, 1.0, nil)
		if err != nil {
			t.Fatal(err)
		}

		err = savePng("small-text.png", bitmap)
		if err != nil {
			t.Fatal(err)
		}

		painter, err := newTestPainter(ctx)
		if err != nil {
			t.Fatal(err)
		}

		painter.Begin(200, 100)
		drawLayoutMetrics(painter, layout, green, blue, gray, red)
		painter.End()

		err = savePng("small-output.png", painter.result)
		if err != nil {
			t.Fatal(err)
		}
	})

	t.Run("large", func(t *testing.T) {
		layout, err := newRichTextLayout(ctx, 400, 400, fontName, wrap, align, white)
		if err != nil {
			t.Fatal(err)
		}

		bitmap, err := ctx.DrawTextLayout(layout, 1.0, nil)
		if err != nil {
			t.Fatal(err)
		}

		err = savePng("large-text.png", bitmap)
		if err != nil {
			t.Fatal(err)
		}

		painter, err := newTestPainter(ctx)
		if err != nil {
			t.Fatal(err)
		}

		painter.Begin(400, 400)
		drawLayoutMetrics(painter, layout, green, blue, gray, red)
		painter.End()

		err = savePng("large-output.png", painter.result)
		if err != nil {
			t.Fatal(err)
		}
	})
}

func savePng(filename string, img image.Image) (err error) {
	var buf bytes.Buffer
	buf.Grow(img.Bounds().Dx() * img.Bounds().Dy())
	err = png.Encode(&buf, img)
	if err != nil {
		return
	}
	return os.WriteFile(filename, buf.Bytes(), 0666)
}

func newRichTextLayout(ctx typography.Context, width, height int, font string, wrap typography.WrapMode, align typography.TextAlignment, textColor graphics.Color) (typography.TextLayout, error) {
	text := "abp中国中文👨‍👩‍👧‍👦 مشروع "
	format := typography.TextFormat{
		Font: typography.FontInfo{
			Family: font,
			Size:   32,
		},
		WrapMode:  wrap,
		TextAlign: align,
		TextColor: textColor,
	}

	layout, err := ctx.NewTextLayout(text, format, float32(width), float32(height))
	if err != nil {
		return nil, err
	}

	//layout.SetTextFont(0, 3, typography.FontInfo{
	//	Family: "mono sans",
	//	Size:   24,
	//})
	layout.SetTextColor(0, 3, color.RGBA{R: 160, A: 255})
	layout.SetUnderline(0, 1, true)
	layout.SetStrikethrough(1, 2, true)
	return layout, nil
}

func drawLayoutMetrics(painter graphics.Painter, layout typography.TextLayout, layoutColor, lineColor, baselineColor, clusterColor graphics.Color) {
	width, height := layout.MeasureSize()
	lines, clusters := layout.MeasureMetrics()
	painter.DrawRect(graphics.Rect(0, 0, width, height), 1, layoutColor)
	painter.DrawTextLayout(graphics.Point{}, layout)

	for _, line := range lines {
		p0 := graphics.Point{line.X, line.Baseline}
		p1 := graphics.Point{line.X + line.Width, line.Baseline}
		painter.DrawLine(p0, p1, 1, baselineColor)
		rect := graphics.Rect(line.X, line.Y, line.Width, line.Height)
		painter.DrawRect(rect, 1, lineColor)
	}

	for _, cluster := range clusters {
		rect := graphics.Rect(cluster.X, cluster.Y, cluster.Width, cluster.Height)
		painter.DrawRect(rect, 1, clusterColor)
	}
}

type testPainter struct {
	result image.Image
	graphics.Painter
}

func newTestPainter(typo typography.Context) (_ *testPainter, err error) {
	p := new(testPainter)
	p.Painter, err = software.NewPainter(p, typo)
	if err != nil {
		return nil, err
	}
	return p, nil
}

func (d *testPainter) Draw(img image.Image) error {
	d.result = img
	return nil
}
