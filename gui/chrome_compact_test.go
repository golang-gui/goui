package gui

import (
	"testing"

	"github.com/golang-gui/goui/core/geometry"

	"github.com/golang-gui/goui/platform/graphics"
)

func TestCircularControlsRightInset(t *testing.T) {
	win, _ := chromeFixture(t, false, true)
	win.clientChrome = true
	for _, button := range win.controls.buttons {
		button.circular = true
	}
	// Window decoration consumes the height query; concrete HeaderBar avoidance
	// is independently covered in widgets.TestHeaderBarCircularControlsAvoidance.
	win.Chrome().ConnectQueryControls(func(c *ChromeControls) { c.Height = 40 })
	for _, scale := range []float32{1, 1.5, 2} {
		for _, width := range []float32{640, 800} {
			win.width = width
			win.pixelWidth, win.pixelHeight = width*scale, win.height*scale
			win.RequestLayout()
			win.paint()
			bounds := win.controls.Rect()
			// Intersect clips against the native surface and may round the height
			// subtraction at fractional scales; use the same geometric contract.
			want := geometry.Rect(width-106, win.frameInset()+6, 100, 28).
				Intersect(geometry.Rect(0, 0, win.width, win.height))
			if bounds != want {
				t.Fatalf("unexpected controls placement at %gx: %v", scale, bounds)
			}
			close := win.controls.buttons[2].windowRect()
			if close.Width != 28 || width-(close.X+close.Width-2) != 8 {
				t.Fatal("visible right inset is not 8 DIP or hit width changed")
			}
			if win.chrome.queryRegion(close.Center()) != ChromeRegionClose {
				t.Fatal("moving controls broke close hit testing")
			}
			if close.X+close.Width > width-win.frameInset() {
				t.Fatal("close button entered the frame's safe clip")
			}
		}
	}
}

type captionIconRecorder struct {
	testButtonBackgroundPainter
	points []geometry.Point
	widths []float32
}

func (p *captionIconRecorder) DrawLine(a, b geometry.Point, width float32, _ graphics.Brush) {
	p.points = append(p.points, a, b)
	p.widths = append(p.widths, width)
}

func (p *captionIconRecorder) DrawRect(rect geometry.Rectangle, width float32, brush graphics.Brush) {
	p.DrawLine(rect.Pos, geometry.Point{X: rect.X + rect.Width, Y: rect.Y + rect.Height}, width, brush)
}

func TestCaptionGlyphSizeDoesNotChangeHitBox(t *testing.T) {
	for _, circular := range []bool{false, true} {
		win, _ := chromeFixture(t, false, true)
		win.clientChrome = circular
		for _, button := range win.controls.buttons {
			button.circular = circular
		}
		win.paint()
		if circular && win.controls.Rect().Y != win.frameInset()+6 {
			t.Fatalf("controls are not centered in the 40 DIP fallback row: %v", win.controls.Rect())
		}
		for _, state := range []WindowState{WindowStateNormal, WindowStateMaximized} {
			win.platformWindow.(*desktopTestWindow).state = state
			for index, button := range win.controls.buttons {
				icon := button.content
				p := &captionIconRecorder{}
				icon.Paint(p)
				low, high := float32(1), float32(11)
				if circular {
					low, high = 2, 10
					if button.Rect().Size != (geometry.Size{Width: 28, Height: 28}) {
						t.Fatal("compact glyph changed the hit box")
					}
					paint := &testButtonBackgroundPainter{}
					button.Paint(paint)
					if paint.rect != geometry.Rect(2, 2, 24, 24) || paint.radius != 12 {
						t.Fatal("compact glyph changed the circular background")
					}
				}
				if icon.Rect().Size != (geometry.Size{Width: 12, Height: 12}) {
					t.Fatal("compact glyph changed icon allocation")
				}
				minX, minY, maxX, maxY := float32(100), float32(100), float32(-100), float32(-100)
				for _, point := range p.points {
					minX, minY = min(minX, point.X), min(minY, point.Y)
					maxX, maxY = max(maxX, point.X), max(maxY, point.Y)
				}
				if minX != low || maxX != high || index != 0 && (minY != low || maxY != high) || index == 0 && (minY != 6 || maxY != 6) {
					t.Fatalf("circular=%v state=%v icon=%d: glyph bounds=(%v,%v)-(%v,%v)", circular, state, index, minX, minY, maxX, maxY)
				}
				for _, width := range p.widths {
					if width != 1 {
						t.Fatal("glyph stroke was scaled")
					}
				}
			}
		}
	}
}
