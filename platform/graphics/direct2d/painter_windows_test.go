package direct2d

import (
	"runtime"
	"testing"

	"github.com/golang-gui/goui/platform/events"
	"github.com/golang-gui/goui/platform/graphics"
	"github.com/golang-gui/goui/platform/typography"
	"github.com/golang-gui/goui/platform/typography/directwrite"
	"github.com/golang-gui/goui/platform/windows/win32"
)

var (
	typo    typography.Context
	painter graphics.Painter
)

func render(width, height uint) {
	painter.Begin(width, height)
	{
		painter.Clear(graphics.RGBA(180, 180, 180, 255))
		painter.FillRoundRect(graphics.Rect(50, 50, 100, 60), 12, graphics.RGBA(100, 100, 100, 255))

		if typo != nil {
			textLayout, err := typo.NewTextLayout("🎁按钮", typography.TextFormat{
				Font: typography.FontInfo{
					Family: "Microsoft YaHei",
					Size:   18,
				},
				TextColor: graphics.RGBA(200, 200, 200, 255),
			}, 100, 60)
			if err != nil {
				panic(err)
			}
			painter.DrawTextLayout(graphics.Point{X: 50, Y: 50}, textLayout)
		}

		painter.DrawPath(graphics.MoveTo(200, 50).QuadBezierTo(250, 150, 300, 50), 2, graphics.RGBA(100, 0, 0, 255))
	}
	painter.End()
}

func Test_Direct2DPainter(t *testing.T) {
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

	win.SetTitle("Direct2D Painter")
	win.Show()

	for !quit {
		queue.Wait()
	}
}
