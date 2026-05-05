package opengl

import (
	"github.com/golang-gui/goui/platform/events"
	"github.com/golang-gui/goui/platform/graphics"
	"github.com/golang-gui/goui/platform/typography"
	"github.com/golang-gui/goui/platform/typography/directwrite"
	"github.com/golang-gui/goui/platform/win32"
	"runtime"
	"testing"
)

var (
	typo    typography.Context
	painter *Painter
)

func render(width, height uint) {
	painter.Begin(float32(width), float32(height), 1.0)
	{
		painter.Clear(graphics.RGBA(180, 180, 180, 255))
		painter.FillRoundRect(graphics.Rect(50, 50, 100, 60), 12, graphics.RGBA(100, 100, 100, 255))
		painter.DrawText(graphics.Rect(50, 50, 100, 60), "🎁按钮", typography.TextFormat{
			Font: typography.FontInfo{
				Family: "Microsoft YaHei",
				Size:   18,
			},
			TextAlign: typography.TextAlignCenter,
			LineAlign: typography.LineAlignCenter,
		}, graphics.RGBA(200, 200, 200, 255))

		layout, err := typo.NewTextLayout("✨这是一段比较长的文本；不用担心，它会自动换行。🧧我会改变部分文本的背景色！", typography.TextFormat{
			Font: typography.FontInfo{
				Family: "Microsoft YaHei",
				Size:   18,
			},
			WordWrap: typography.WrapWordChar,
			//LineAlign: typography.LineAlignCenter,
		}, 300, 200)
		if err != nil {
			panic(err)
		}

		pt := graphics.Point{X: 50, Y: 120}
		_, runs := layout.MeasureLines()
		bgColorRuns := runs[len(runs)-4 : len(runs)-1]
		last := bgColorRuns[len(bgColorRuns)-1]
		start := bgColorRuns[0].Start
		end := last.Start + last.Length
		length := end - start

		layout.SetAttribute(typography.TextAttribute{
			Start:  start,
			Length: length,
			Type:   typography.TextBgColor,
			Value:  graphics.RGBA(30, 60, 120, 255),
		})

		layout.SetAttribute(typography.TextAttribute{
			Start:  start,
			Length: length,
			Type:   typography.TextFgColor,
			Value:  graphics.RGBA(200, 200, 200, 255),
		})

		painter.DrawTextLayout(pt, layout, graphics.RGBA(0, 0, 0, 255))

		painter.DrawPath(graphics.MoveTo(200, 50).QuadBezierTo(250, 100, 300, 50), 2, graphics.RGBA(100, 0, 0, 255))
		painter.DrawPath(graphics.MoveTo(310, 50).LineTo(360, 50).ArcTo(20, 20, 0, 0, 0, 380, 70), 2, graphics.RGBA(0, 100, 0, 255))

		//painter.DrawPath(graphics.MoveTo(100, 50).QuadBezierTo(150, 150, 200, 50), 2, graphics.RGBA(100, 0, 0, 255))
	}
	painter.End()
}

func TestOpenGLPainter(t *testing.T) {
	runtime.LockOSThread()

	plat, err := win32.NewPlatform()
	if err != nil {
		t.Fatal(err)
	}

	queue, err := plat.NewEventQueue()
	if err != nil {
		t.Fatal(err)
	}

	quit := false
	var width, height uint

	win, err := plat.NewWindow(func(event events.Event) {
		switch ev := event.(type) {
		case *events.CloseEvent:
			quit = true
		case *events.SizeEvent:
			width, height = ev.Width, ev.Height
		case *events.PaintEvent:
			render(width, height)
		}
	})
	if err != nil {
		t.Fatal(err)
	}

	typo, err = directwrite.NewContext()
	if err != nil {
		t.Fatal(err)
	}

	painter, err = NewPainter(win, typo)
	if err != nil {
		t.Fatal(err)
	}

	win.SetTitle("OpenGL Painter")
	win.Show()

	for !quit {
		queue.Wait()
	}
}
