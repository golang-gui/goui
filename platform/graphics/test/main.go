package main

import (
	"bytes"
	"image"
	"image/color"
	"image/jpeg"
	"log"
	"os"
	"runtime"
	"strings"
	"time"

	"github.com/golang-gui/goui/core/geometry"
	"github.com/golang-gui/goui/platform"
	"github.com/golang-gui/goui/platform/events"
	"github.com/golang-gui/goui/platform/graphics"
	"github.com/golang-gui/goui/platform/typography"
)

var (
	typo    typography.Context
	painter graphics.Painter
	img     image.Image
)

func getTextRange(text, subText string) (start, length int) {
	start = strings.Index(text, subText)
	return start, len(subText)
}

func render(width, height float32) {
	runtime.LockOSThread()

	beg := time.Now()
	defer func() {
		log.Printf("render cost: %v", time.Since(beg))
	}()

	painter.Begin(width, height, 1.0)
	{
		painter.Clear(graphics.RGBA(90, 160, 200, 255))
		painter.FillRoundRect(graphics.Rect(50, 50, 100, 60), 12, graphics.RGBA(90, 50, 50, 255))

		if typo != nil {
			text := "✨这是一段比较长的文本；不用担心，它会自动换行。🧧我会改变部分文本的背景色！"
			textRect := graphics.Rect(50, 120, 300, 100)

			layout, err := typo.NewTextLayout(text, typography.TextFormat{
				Font: typography.FontInfo{
					Family: "Microsoft YaHei",
					Size:   16,
				},
				WrapMode:  typography.WrapWordChar,
				TextAlign: typography.TextAlignCenter,
			}, textRect.Width, textRect.Height)
			if err != nil {
				panic(err)
			}

			kaiStart, kaiLength := getTextRange(text, "这是一段比较长的文本")
			layout.SetTextFont(kaiStart, kaiLength, typography.FontInfo{
				Family: "Kaiti",
				Size:   12,
			})
			layout.SetTextColor(kaiStart, kaiLength, color.RGBA{R: 160, A: 255})

			underlineStart, underlineLength := getTextRange(text, "这是")
			layout.SetUnderline(underlineStart, underlineLength, true)

			strikeStart, strikeLength := getTextRange(text, "不用担心")
			layout.SetStrikethrough(strikeStart, strikeLength, true)

			_, clusters := layout.MeasureMetrics()
			bgClusters := clusters[len(clusters)-4 : len(clusters)-1]
			first := bgClusters[0]
			last := bgClusters[len(bgClusters)-1]
			bgRect := graphics.Rect(textRect.X+first.X, textRect.Y+first.Y, last.X+last.Width-first.X, last.Y+last.Height-first.Y)
			// 绘制文本背景色
			painter.FillRect(bgRect, graphics.RGBA(30, 60, 130, 255))

			painter.DrawTextLayout(textRect.Pos, layout)
		}

		painter.DrawImage(graphics.Rect(50, 260, 300, 200), img)
		painter.DrawImage(graphics.Rect(50, 480, 100, 100), img)

		painter.DrawPath(graphics.MoveTo(200, 50).QuadBezierTo(250, 100, 300, 50), 2, graphics.RGBA(100, 0, 0, 255))
		painter.DrawPath(graphics.MoveTo(310, 50).LineTo(360, 50).ArcTo(20, 20, 0, 0, 0, 380, 70), 2, graphics.RGBA(0, 100, 0, 255))

		painter.DrawEllipse(graphics.Point{480, 100}, 50, 50, 2, graphics.RGBA(50, 130, 60, 255))
		painter.FillEllipse(graphics.Point{480, 100}, 30, 30, graphics.RGBA(50, 50, 130, 255))
		painter.DrawLine(graphics.Point{480 - 50, 100}, graphics.Point{480 + 50, 100}, 2, graphics.RGB(130, 0, 0))
		painter.DrawLine(graphics.Point{480, 100 - 50}, graphics.Point{480, 100 + 50}, 2, graphics.RGB(130, 0, 0))

		painter.DrawRoundRect(graphics.Rect(430, 200, 260, 180), 12, 4, graphics.RGB(30, 100, 30))
		painter.DrawRect(graphics.Rect(450, 220, 220, 140), 4, graphics.RGB(30, 100, 30))

		// --- Transform tests ---
		// Each group draws a white outline reference without transform, then
		// the same shape with a transform, so you can visually verify correctness.

		// 1. Translate: rect at (0,0) → should appear at (520,80)
		painter.SetTransform(geometry.Identity())
		painter.DrawRect(graphics.Rect(520, 80, 60, 40), 1, graphics.RGBA(255, 255, 255, 100))
		painter.SetTransform(geometry.Translate(520, 80))
		painter.FillRect(graphics.Rect(0, 0, 60, 40), graphics.RGBA(255, 0, 0, 200))

		// 2. Translate + Rotate 30° (local-space: rotate shape, then place)
		painter.SetTransform(geometry.Identity())
		painter.DrawRect(graphics.Rect(620, 80, 60, 40), 1, graphics.RGBA(255, 255, 255, 100))
		painter.SetTransform(geometry.Translate(620, 80).Rotate(30))
		painter.FillRect(graphics.Rect(0, 0, 60, 40), graphics.RGBA(0, 255, 0, 200))

		// 3. Translate + Scale 1.5x
		painter.SetTransform(geometry.Identity())
		painter.DrawRect(graphics.Rect(520, 160, 60, 40), 1, graphics.RGBA(255, 255, 255, 100))
		painter.SetTransform(geometry.Translate(520, 160).Scale(1.5, 1.5))
		painter.FillRect(graphics.Rect(0, 0, 60, 40), graphics.RGBA(0, 0, 255, 200))

		// 4. Rotated rect stroke (DrawRect with transform)
		painter.SetTransform(geometry.Identity())
		painter.SetTransform(geometry.Translate(700, 120).Rotate(45))
		painter.DrawRect(graphics.Rect(-30, -30, 60, 60), 3, graphics.RGBA(255, 200, 0, 255))

		// 5. Translated + rotated line (DrawLine with transform)
		painter.SetTransform(geometry.Identity())
		painter.SetTransform(geometry.Translate(520, 260).Rotate(45))
		painter.DrawLine(graphics.Point{0, 0}, graphics.Point{60, 0}, 3, graphics.RGBA(255, 100, 255, 255))

		// 6. Translated ellipse (FillEllipse + DrawEllipse with transform)
		painter.SetTransform(geometry.Identity())
		painter.SetTransform(geometry.Translate(620, 260))
		painter.FillEllipse(graphics.Point{0, 0}, 25, 25, graphics.RGBA(100, 200, 255, 200))
		painter.DrawEllipse(graphics.Point{0, 0}, 30, 30, 2, graphics.RGBA(0, 100, 200, 255))

		// 7. Translated + rotated path (DrawPath with transform)
		painter.SetTransform(geometry.Identity())
		painter.SetTransform(geometry.Translate(520, 360).Rotate(-15))
		path := graphics.MoveTo(0, 0).LineTo(80, 0).ArcTo(15, 15, 0, 0, 1, 95, 15).LineTo(0, 15).Close()
		painter.DrawPath(path, 2, graphics.RGBA(200, 100, 50, 255))

		// 8. Translated + rotated round rect (FillRoundRect / DrawRoundRect with transform)
		painter.SetTransform(geometry.Identity())
		painter.SetTransform(geometry.Translate(700, 360).Rotate(20))
		painter.FillRoundRect(graphics.Rect(0, 0, 80, 50), 10, graphics.RGBA(200, 50, 200, 180))
		painter.DrawRoundRect(graphics.Rect(0, 0, 80, 50), 10, 2, graphics.RGBA(255, 255, 255, 255))

		// 9. Translated text (DrawTextLayout with transform)
		if typo != nil {
			smallLayout, err := typo.NewTextLayout("Transform!", typography.TextFormat{
				Font: typography.FontInfo{Family: "Microsoft YaHei", Size: 18},
			}, 200, 40)
			if err == nil {
				// 9a. Reference: no transform
				painter.SetTransform(geometry.Identity())
				painter.DrawTextLayout(graphics.Point{520, 460}, smallLayout)

				// 9b. Translate only
				painter.SetTransform(geometry.Translate(520, 500))
				painter.DrawTextLayout(graphics.Point{0, 0}, smallLayout)

				// 9c. Translate + Rotate 30°
				painter.SetTransform(geometry.Translate(520, 540).Rotate(30))
				painter.DrawTextLayout(graphics.Point{0, 0}, smallLayout)

				// 9d. Translate + Scale 1.5x
				painter.SetTransform(geometry.Translate(680, 460).Scale(1.5, 1.5))
				painter.DrawTextLayout(graphics.Point{0, 0}, smallLayout)

				// 9e. Translate + Rotate -90° (vertical text, bottom-up)
				painter.SetTransform(geometry.Translate(780, 580).Rotate(-90))
				painter.DrawTextLayout(graphics.Point{0, 0}, smallLayout)
			}
		}

		// 10. Transformed image (DrawImage with transform)
		// 10a. Reference: no transform
		painter.SetTransform(geometry.Identity())
		painter.DrawImage(graphics.Rect(700, 460, 80, 60), img)

		// 10b. Translate + Rotate 45°
		painter.SetTransform(geometry.Translate(700, 540).Rotate(45))
		painter.DrawImage(graphics.Rect(0, 0, 60, 40), img)

		// 10c. Translate + Scale 0.5x
		painter.SetTransform(geometry.Translate(620, 540).Scale(0.5, 0.5))
		painter.DrawImage(graphics.Rect(0, 0, 80, 60), img)

		// 11. Identity reset — shape should be at exact window coords
		painter.SetTransform(geometry.Identity())
		painter.DrawRect(graphics.Rect(520, 550, 100, 30), 1, graphics.RGBA(255, 255, 255, 150))
	}
	painter.End()
}

func panicIf(err error) {
	if err != nil {
		panic(err)
	}
}

func main() {
	runtime.LockOSThread()

	data, err := os.ReadFile("testdata/flower.jpg")
	panicIf(err)

	img, err = jpeg.Decode(bytes.NewReader(data))
	panicIf(err)

	plat, err := platform.NewPlatform(platform.DefaultName())
	panicIf(err)

	eventLoop, err := plat.NewEventLoop()
	panicIf(err)
	defer eventLoop.Destroy()

	var win platform.Window
	var width, height float32

	win, err = plat.NewWindow(800, 600, func(event events.Event) {
		switch ev := event.(type) {
		case events.CloseEvent:
			win.Destroy()
			eventLoop.Quit()
		case events.SizeEvent:
			width, height = ev.PixelWidth, ev.PixelHeight
		case events.PaintEvent:
			render(width, height)
		}
	})
	panicIf(err)

	typo, err = plat.NewTypography()
	panicIf(err)

	painter, err = plat.NewPainter(win, typo)
	panicIf(err)

	win.SetTitle("Painter test")
	win.Show()

	eventLoop.Run()
}
