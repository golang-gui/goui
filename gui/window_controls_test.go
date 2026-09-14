package gui

import (
	"image"
	"image/color"
	"testing"

	"github.com/golang-gui/goui/core/colors"
	"github.com/golang-gui/goui/core/geometry"
	"github.com/golang-gui/goui/layout"
	"github.com/golang-gui/goui/platform"
	"github.com/golang-gui/goui/platform/events"
	"github.com/golang-gui/goui/platform/graphics"
	"github.com/golang-gui/goui/platform/graphics/software"
	"github.com/golang-gui/goui/style"
)

func TestRectangularCaptionAppearance(t *testing.T) {
	for _, dark := range []bool{false, true} {
		t.Run(map[bool]string{false: "light", true: "dark"}[dark], func(t *testing.T) {
			app := &application{}
			useTestApplication(t, app)
			background := color.Gray{245}
			if dark {
				background.Y = 32
			}
			app.SetStyleSheet(style.Sheet(
				style.Name(styleNameWindow).BackgroundColor(background),
				// Application accents and ordinary button shapes must not leak in.
				style.Name(styleNameButton).BackgroundColor(color.White).Radius(20),
			))
			win, _ := chromeFixture(t, false, true)
			win.setFocused(true)
			for _, button := range win.controls.buttons {
				if button.Rect().Width != 46 {
					t.Fatalf("caption width: %g", button.Rect().Width)
				}
				check := func(hover, pressed bool) (color.Color, color.Color) {
					button.hovered, button.pressed = hover, pressed
					s := button.decorationStyle()
					if radius, _ := s.Radius(); radius != 0 {
						t.Fatal("rectangular caption inherited rounded corners")
					}
					bg, _ := s.BackgroundColor()
					fg, _ := s.ForegroundColor()
					return bg, fg
				}
				bg, active := check(false, false)
				if !colors.Equal(bg, color.Transparent) {
					t.Fatal("idle caption must reveal titlebar background")
				}
				win.setFocused(false)
				_, inactive := check(false, false)
				_, _, _, alpha := inactive.RGBA()
				if colors.Equal(active, inactive) || alpha >= 0xffff {
					t.Fatal("inactive caption was not dimmed")
				}
				win.setFocused(true)
				hover, hoverInk := check(true, false)
				pressed, pressedInk := check(true, true)
				if colors.Equal(hover, pressed) || colors.Equal(hover, bg) {
					t.Fatal("caption states must remain distinguishable")
				}
				if button.close {
					if !colors.Equal(hover, color.NRGBA{R: 196, G: 43, B: 28, A: 255}) ||
						!colors.Equal(hoverInk, color.White) || !colors.Equal(pressedInk, color.White) {
						t.Fatal("close warning lost red background / white glyph")
					}
				} else {
					r, g, b, a := hover.RGBA()
					if r != g || g != b || a == 0 || a == 0xffff {
						t.Fatal("ordinary caption hover must use a neutral translucent fill")
					}
				}
				// Linux's circular presentation retains its independent palette.
				button.circular = true
				palette := win.decorationPalette(true)
				for _, focused := range []bool{false, true} {
					win.setFocused(focused)
					for state, expected := range []color.Color{palette.button, palette.hovered, palette.pressed} {
						button.hovered, button.pressed = state == 1, state == 2
						s := button.decorationStyle()
						circle, _ := s.BackgroundColor()
						ink, _ := s.ForegroundColor()
						radius, _ := s.Radius()
						if !colors.Equal(circle, expected) || !colors.Equal(ink, palette.foreground) ||
							radius != circularControlDiameter/2 || button.decorationBrush(ink) != graphics.ColorOf(ink) {
							t.Fatal("circular decoration appearance changed")
						}
					}
				}
			}
			win.paintDirty = false
			win.setFocused(false)
			if !win.paintDirty {
				t.Fatal("activation change did not repaint controls")
			}
		})
	}
}

func TestCaptionTranslucentColorsAreNotPremultipliedTwice(t *testing.T) {
	useTestApplication(t, &application{
		style: style.Sheet(style.Name(styleNameWindow).BackgroundColor(color.Gray{32})),
	})
	win, _ := chromeFixture(t, false, true)
	win.setFocused(true)
	button := win.controls.buttons[0]
	button.hovered = true
	p := new(testButtonBackgroundPainter)
	button.Paint(p)
	if p.brush != graphics.RGBA(255, 255, 255, 26) {
		t.Fatalf("dark hover RGB was attenuated before blending: %+v", p.brush)
	}
	button.hovered = false
	win.setFocused(false)
	foreground, _ := button.decorationStyle().ForegroundColor()
	if brush := button.decorationBrush(foreground); brush != graphics.RGBA(255, 255, 255, 102) {
		t.Fatalf("inactive white RGB was attenuated before blending: %+v", brush)
	}

	// Verify the actual composite, not only the style or brush values.
	frame := &controlsFrame{}
	painter, err := software.NewPainter(frame)
	if err != nil {
		t.Fatal(err)
	}
	win.painter = painter
	win.pixelWidth, win.pixelHeight = win.width*2, win.height*2
	win.SetWidget(nil)
	for _, test := range []struct {
		hovered, pressed bool
		want             uint32
	}{{false, false, 32}, {true, false, 55}, {true, true, 77}} {
		button.hovered, button.pressed = test.hovered, test.pressed
		win.paint()
		r, g, b, _ := frame.image.At(int((button.windowRect().X+3)*2), 10*2).RGBA()
		if r>>8 < test.want-1 || r>>8 > test.want+1 || r != g || g != b {
			t.Fatalf("dark caption composite = %d,%d,%d; want ~%d", r>>8, g>>8, b>>8, test.want)
		}
	}
}

func TestWindowControlsCustomHitAndClick(t *testing.T) {
	win, native := chromeFixture(t, false, true)
	controls := win.controls
	for i, expected := range []platform.WindowHit{platform.WindowHitMinimize, platform.WindowHitMaximize, platform.WindowHitClose} {
		if got := native.hitTest(controls.buttons[i].windowRect().Center()); got != expected {
			t.Fatalf("button %d hit=%v, want %v", i, got, expected)
		}
	}
	if len(native.requests) != 0 {
		t.Fatal("query executed button commands")
	}
	clickAt(win, controls.buttons[0].windowRect().Center())
	clickAt(win, controls.buttons[1].windowRect().Center())
	if len(native.requests) != 2 || native.requests[0] != WindowStateMinimized || native.requests[1] != WindowStateMaximized {
		t.Fatalf("commands: %v", native.requests)
	}
	native.state = WindowStateMaximized
	_ = win.DispatchEvent(events.StateEvent{State: native.state})
	win.paint()
	if got := win.Snapshot().Controls.Children[1].Text; got != "Restore" {
		t.Fatalf("state not reflected in snapshot: %s", got)
	}
	clickAt(win, controls.buttons[1].windowRect().Center())
	if native.requests[2] != WindowStateNormal {
		t.Fatal("restore did not request Normal")
	}
	veto := win.ConnectCloseRequest(func(allow *bool) { *allow = false })
	clickAt(win, controls.buttons[2].windowRect().Center())
	if win.destroyed {
		t.Fatal("close bypassed veto")
	}
	veto.Disconnect()
	clickAt(win, controls.buttons[2].windowRect().Center())
	if !win.destroyed || controls.Window() != nil || !controls.base().destroyed {
		t.Fatal("close did not release the window-owned tree")
	}
}

func TestWindowControlsOwnershipWithoutContent(t *testing.T) {
	win, native := chromeFixture(t, false, true)
	controls := win.controls
	original := win.Widget()
	win.SetWidget(nil)
	win.paint()
	if win.layoutDirty || native.hitTest(controls.buttons[1].windowRect().Center()) != platform.WindowHitMaximize {
		t.Fatal("content-free window lost native caption hit testing")
	}
	if win.Widget() != nil || win.controls != controls || controls.Parent() != nil || controls.Window() != win {
		t.Fatal("controls were reparented, replaced or installed as hidden content")
	}
	snapshot := win.Snapshot()
	if snapshot.Controls == nil || len(snapshot.Controls.Children) != 3 {
		t.Fatal("missing decoration snapshot")
	}
	clickAt(win, controls.buttons[0].windowRect().Center())
	if len(native.requests) != 1 {
		t.Fatal("controls require application content to receive input")
	}
	win.SetWidget(original)
	win.paint()
	if win.Widget() != original || win.controls != controls {
		t.Fatal("content replacement changed controls")
	}
}

func TestWindowControlsNativeOwnership(t *testing.T) {
	win, native := chromeFixture(t, true, false)
	controls := win.controls
	if len(controls.Children()) != 0 || controls.buttons[0] != nil || win.Snapshot().Controls != nil {
		t.Fatal("native mode created phantom GUI controls")
	}
	if native.controls.Pos != (geometry.Point{X: 12, Y: 8}) {
		t.Fatalf("without a header, native controls should keep their OS position: %v", native.controls)
	}
	win.SetWidget(nil)
	win.paint()
	win.paint()
	if len(native.positions) != 0 {
		t.Fatal("content removal repositioned controls")
	}
	if win.controls != controls {
		t.Fatal("native ownership followed content")
	}
}

func TestWindowControlsResizeFullscreenAndDisabled(t *testing.T) {
	win, native := chromeFixture(t, false, true)
	for _, width := range []float32{800, 80} {
		_ = win.DispatchEvent(events.SizeEvent{Width: width, Height: 400})
		win.paint()
		bounds := chromeInfo(win.Chrome()).ControlsBounds
		if bounds.X+bounds.Width != width || bounds.Width > width {
			t.Fatalf("resize geometry: %v", bounds)
		}
	}
	native.state = WindowStateFullscreen
	_ = win.DispatchEvent(events.StateEvent{State: native.state})
	win.paint()
	if !emptyRect(chromeInfo(win.Chrome()).ControlsBounds) || win.Snapshot().Controls != nil {
		t.Fatal("fullscreen retained custom controls")
	}
	native.state = WindowStateNormal
	_ = win.DispatchEvent(events.StateEvent{State: native.state})
	win.paint()
	if win.Snapshot().Controls == nil {
		t.Fatal("controls not restored")
	}
	disabled := &window{platformWindow: &chromeTestWindow{}}
	defer disabled.Destroy()
	if disabled.controls != nil || !emptyRect(chromeInfo(disabled.Chrome()).ControlsBounds) {
		t.Fatal("disabled service allocated controls")
	}
}

func TestWindowControlsHoverAndCaptureAcrossTrees(t *testing.T) {
	win, native := chromeFixture(t, false, false)
	button := win.controls.buttons[0]
	point := button.windowRect().Center()
	_ = win.DispatchEvent(events.PointerEvent{EventType: events.PointerMove, Position: point})
	if !button.hovered {
		t.Fatal("decoration did not receive hover")
	}
	_ = win.DispatchEvent(events.PointerEvent{EventType: events.PointerMove, Position: geometry.Point{X: 20, Y: 200}})
	if button.hovered {
		t.Fatal("cross-tree leave was lost")
	}
	clickAt(win, point)
	if len(native.requests) != 1 || native.moveRequests != 0 {
		t.Fatal("custom controls became a drag region")
	}
	// A gesture owned by content must retain its move/up delivery over controls.
	win.dispatcher.captureTarget = win.Widget()
	_ = win.DispatchEvent(events.PointerEvent{EventType: events.PointerUp, Button: events.PointerButtonLeft, Position: point})
	if len(native.requests) != 1 {
		t.Fatal("capture released through caption button")
	}
}

func TestWindowControlsPressMoveAndCancel(t *testing.T) {
	for i, name := range []string{"minimize", "maximize", "close"} {
		t.Run(name, func(t *testing.T) {
			win, _ := chromeFixture(t, false, true)
			win.ConnectCloseRequest(func(allow *bool) { *allow = false })
			button := win.controls.buttons[i]
			clicks := 0
			button.ConnectClicked(func() { clicks++ })
			point := button.windowRect().Center()
			moved := geometry.Point{X: point.X + 3, Y: point.Y + 2}
			send := func(kind events.EventType, p geometry.Point, buttons events.PointerButtons) {
				t.Helper()
				if err := win.DispatchEvent(events.PointerEvent{
					EventType: kind, Position: p, Button: events.PointerButtonLeft, Buttons: buttons,
				}); err != nil {
					t.Fatal(err)
				}
			}
			send(events.PointerMove, point, 0)
			send(events.PointerDown, point, events.PointerButtonLeftDown)
			// Capture switches the native input path, which may emit Enter before Move.
			send(events.PointerEnter, moved, events.PointerButtonLeftDown)
			send(events.PointerMove, moved, events.PointerButtonLeftDown)
			if !button.pressed || !button.click.pressed || clicks != 0 {
				t.Fatal("moving within the button lost the press or clicked early")
			}
			send(events.PointerUp, moved, 0)
			if button.pressed || clicks != 1 {
				t.Fatal("release did not click exactly once")
			}
			send(events.PointerDown, point, events.PointerButtonLeftDown)
			outside := geometry.Point{X: 20, Y: 200}
			send(events.PointerMove, outside, events.PointerButtonLeftDown)
			if button.pressed || button.click.pressed {
				t.Fatal("leaving the button did not cancel the press")
			}
			send(events.PointerUp, outside, 0)
			if clicks != 1 {
				t.Fatal("release outside the button activated it")
			}
		})
	}
}

func TestControlsFollowHeaderAllocation(t *testing.T) {
	for _, nativeButtons := range []bool{false, true} {
		win, native := chromeFixture(t, nativeButtons, !nativeButtons)
		header := NewHeaderBar()
		content := newTestWidget()
		content.SetMinSize(geometry.Size{Width: 20, Height: 16})
		header.SetChild(content)
		root := NewLinearBox(layout.DirectionVertical)
		root.SetCrossAlign(layout.CrossStretch)
		root.AddChild(header)
		win.SetWidget(root)
		for _, height := range []float32{48, 80, 32} {
			header.SetMinSize(geometry.Size{Height: height})
			win.paint()
			bounds := chromeInfo(win.Chrome()).ControlsBounds
			intrinsicHeight := height
			if nativeButtons {
				intrinsicHeight = 14
			}
			if header.Rect().Height != height || bounds.Height != intrinsicHeight ||
				bounds.Y != (height-intrinsicHeight)/2 {
				t.Fatalf("height %v: header=%v controls=%v", height, header.Rect(), bounds)
			}
			if !nativeButtons {
				for _, button := range win.controls.buttons {
					if button.Rect().Height != height {
						t.Fatalf("button allocation did not follow header height: %v", button.Rect())
					}
				}
				for _, y := range []float32{1, height - 1} {
					if native.hitTest(geometry.Point{X: bounds.X + bounds.Width/2, Y: y}) != platform.WindowHitMaximize {
						t.Fatalf("full-height caption hit missing at y=%g", y)
					}
				}
			}
			if !emptyRect(content.windowRect().Intersect(bounds)) {
				t.Fatal("height update left content overlapping controls")
			}
			win.paint()
			if win.layoutDirty {
				t.Fatal("height update caused a layout loop")
			}
		}
		root.RemoveChild(header)
		win.paint()
		bounds := chromeInfo(win.Chrome()).ControlsBounds
		if nativeButtons {
			if native.controls.Y != 8 || native.positions[len(native.positions)-1].HasPosition {
				t.Fatal("header removal did not restore native default")
			}
		} else if bounds.Y != 0 || bounds.Height != captionButtonHeight {
			t.Fatal("header removal did not restore intrinsic custom placement")
		}
	}
}

type controlsFrame struct {
	image  image.Image
	frames int
}

func (f *controlsFrame) Draw(img image.Image) error {
	f.image = img
	f.frames++
	return nil
}

func (*controlsFrame) Transparent() bool { return false }

type controlsBackground struct{ WidgetBase }

func (w *controlsBackground) Paint(p Painter) {
	p.FillRect(geometry.Rect(0, 0, w.rect.Width, w.rect.Height), graphics.RGB(40, 100, 200))
}

func TestWindowControlsSoftwareFrame(t *testing.T) {
	for _, scale := range []float32{1, 1.5, 2} {
		win, _ := chromeFixture(t, false, true)
		frame := &controlsFrame{}
		painter, err := software.NewPainter(frame)
		if err != nil {
			t.Fatal(err)
		}
		win.painter = painter
		win.pixelWidth, win.pixelHeight = win.width*scale, win.height*scale
		content := &controlsBackground{}
		win.SetWidget(content)
		win.paint()
		if frame.frames != 1 || frame.image.Bounds().Dx() != int(win.width*scale) {
			t.Fatal("controls were drawn in a separate frame or wrong scale")
		}
		r, g, b, _ := frame.image.At(int(10*scale), int(200*scale)).RGBA()
		if r>>8 != 40 || g>>8 != 100 || b>>8 != 200 {
			t.Fatal("application content was not painted")
		}
		rect := win.controls.buttons[0].windowRect()
		center := rect.Center()
		dark := 0
		for y := int((center.Y - 7) * scale); y < int((center.Y+7)*scale); y++ {
			for x := int((center.X - 7) * scale); x < int((center.X+7)*scale); x++ {
				r, g, b, _ := frame.image.At(x, y).RGBA()
				// A one-DIP stroke on an integer coordinate has half-pixel
				// coverage at 1x; antialiased gray is still a visible icon.
				if r < 0xc000 && g < 0xc000 && b < 0xc000 {
					dark++
				}
			}
		}
		if dark == 0 {
			t.Fatalf("caption icon was missing or covered by content at %gx", scale)
		}
		win.SetWidget(nil)
		win.paint()
		if frame.frames != 2 {
			t.Fatal("empty-content window did not paint controls")
		}
	}
}
