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

func render(width, height uint) {
	runtime.LockOSThread()

	beg := time.Now()
	defer func() {
		log.Printf("render cost: %v", time.Since(beg))
	}()

	painter.Begin(float32(width), float32(height), 2.0)
	{
		painter.Clear(graphics.RGBA(90, 160, 200, 255))
		painter.FillRoundRect(graphics.Rect(50, 50, 100, 60), 12, graphics.RGBA(90, 50, 50, 255))

		if typo != nil {
			text := "✨这是一段比较长的文本；不用担心，它会自动换行。🧧我会改变部分文本的背景色！"
			textRect := graphics.Rect(50, 120, 300, 100)

			layout, err := typo.NewTextLayout(text, typography.TextFormat{
				Font: typography.FontInfo{
					Family: "Microsoft YaHei",
					Size:   18,
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
				Size:   16,
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
	var width, height uint

	win, err = plat.NewWindow(func(event events.Event) {
		switch ev := event.(type) {
		case events.CloseEvent:
			win.Destroy()
			eventLoop.Quit()
		case events.SizeEvent:
			width, height = ev.Width, ev.Height
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
