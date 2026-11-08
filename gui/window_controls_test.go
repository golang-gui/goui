package gui

import (
	"image"
	"image/color"
	"math"
	"testing"

	"github.com/golang-gui/goui/core/colors"
	"github.com/golang-gui/goui/core/geometry"
	"github.com/golang-gui/goui/layout"
	"github.com/golang-gui/goui/mlang"
	"github.com/golang-gui/goui/platform"
	"github.com/golang-gui/goui/platform/events"
	"github.com/golang-gui/goui/platform/graphics"
	"github.com/golang-gui/goui/platform/graphics/software"
	"github.com/golang-gui/goui/style"
)

func TestWindowControlsLocalizedSnapshot(t *testing.T) {
	previous := mlang.Language()
	t.Cleanup(func() { _ = mlang.SetLanguage(previous) })
	win, native := chromeFixture(t, false, true)
	_ = mlang.SetLanguage("ja")
	info := win.controls.Snapshot()
	for i, want := range []string{"最小化", "最大化", "閉じる"} {
		if info.Children[i].Text != want || info.Children[i].Role != RoleButton {
			t.Fatalf("button %d lost localized text or role: %+v", i, info.Children[i])
		}
	}
	native.state = WindowStateMaximized
	_ = win.DispatchEvent(events.StateEvent{State: native.state})
	_ = mlang.SetLanguage("ar")
	if got := win.controls.Snapshot().Children[1].Text; got != "استعادة" {
		t.Fatalf("maximized controls did not localize Restore: %q", got)
	}
}

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
					button.updateState(style.Hovered, hover)
					button.SetState(style.Pressed, pressed)
					s := button.decorationStyle()
					if radius, _ := s.CornerRadius(); radius != geometry.UniformCornerRadius(0) {
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
						button.updateState(style.Hovered, state == 1)
						button.SetState(style.Pressed, state == 2)
						s := button.decorationStyle()
						circle, _ := s.BackgroundColor()
						ink, _ := s.ForegroundColor()
						radius, _ := s.CornerRadius()
						if !colors.Equal(circle, expected) || !colors.Equal(ink, palette.foreground) ||
							radius != geometry.UniformCornerRadius(circularControlDiameter/2) || button.decorationBrush(ink) != graphics.ColorOf(ink) {
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
	button.updateState(style.Hovered, true)
	p := new(testButtonBackgroundPainter)
	button.Paint(p)
	if p.brush != graphics.RGBA(255, 255, 255, 26) {
		t.Fatalf("dark hover RGB was attenuated before blending: %+v", p.brush)
	}
	button.updateState(style.Hovered, false)
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
		button.updateState(style.Hovered, test.hovered)
		button.SetState(style.Pressed, test.pressed)
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
	disabled := testWindowWithSurface(&window{surfaceBase: surfaceBase{platformWindow: &chromeTestWindow{}}})
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
	if !button.State().Has(style.Hovered) {
		t.Fatal("decoration did not receive hover")
	}
	_ = win.DispatchEvent(events.PointerEvent{EventType: events.PointerMove, Position: geometry.Point{X: 20, Y: 200}})
	if button.State().Has(style.Hovered) {
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
			if !button.State().Has(style.Pressed) || !button.click.pressed || clicks != 0 {
				t.Fatal("moving within the button lost the press or clicked early")
			}
			send(events.PointerUp, moved, 0)
			if button.State().Has(style.Pressed) || clicks != 1 {
				t.Fatal("release did not click exactly once")
			}
			send(events.PointerDown, point, events.PointerButtonLeftDown)
			outside := geometry.Point{X: 20, Y: 200}
			send(events.PointerMove, outside, events.PointerButtonLeftDown)
			if button.State().Has(style.Pressed) || button.click.pressed {
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

type captionPixelRecorder struct {
	captionIconRecorder
	scale float32
}

func (p *captionPixelRecorder) PixelScale() float32 { return p.scale }

// 标题高度和分数窗口宽度只影响图标的整体定位，不能改变 X 的几何或按钮布局。
func TestCloseCaptionGlyphDevicePlacement(t *testing.T) {
	for _, circular := range []bool{false, true} {
		win, _ := chromeFixture(t, false, true)
		win.clientChrome = circular
		for _, button := range win.controls.buttons {
			button.circular = circular
		}
		header := NewHeaderBar()
		root := NewLinearBox(layout.DirectionVertical)
		root.SetCrossAlign(layout.CrossStretch)
		root.AddChild(header)
		win.SetWidget(root)
		for _, width := range []float32{640, 640.25, 640.5} {
			win.width = width
			for _, height := range []float32{40, 41, 48, 49, 40.5, 73.25} {
				header.SetMinSize(geometry.Size{Height: height})
				win.RequestLayout()
				win.paint()
				button := win.controls.buttons[2]
				icon := button.content
				bounds, hit := icon.base().windowRect(), button.windowRect()
				for _, scale := range []float32{0, 1, 1.25, 1.5, 1.75, 2} {
					p := &captionPixelRecorder{scale: scale}
					icon.Paint(p)
					if len(p.points) != 4 || len(p.widths) != 2 || p.widths[0] != 1 || p.widths[1] != 1 {
						t.Fatal("关闭图标必须仍为两条 1 DIP 斜线")
					}
					span := float32(10)
					if circular {
						span = 8
					}
					a, b, c, d := p.points[0], p.points[1], p.points[2], p.points[3]
					if b.X-a.X != span || b.Y-a.Y != span || c.X != b.X || c.Y != a.Y || d.X != a.X || d.Y != b.Y {
						t.Fatalf("circular=%t height=%g scale=%g: X 的长度、角度或对称性改变：%v", circular, height, scale, p.points)
					}
					for _, axis := range [][2]float32{{bounds.X, (a.X + b.X) / 2}, {bounds.Y, (a.Y + b.Y) / 2}} {
						if scale == 0 {
							if axis[1] != 6 {
								t.Fatal("无有效像素比例时不应调整图标")
							}
							continue
						}
						center := float64(axis[0]+axis[1]) * float64(scale)
						if math.Abs(center-math.Round(center)) > 0.0001 || math.Abs(float64(axis[1]-6)*float64(scale)) > 0.5001 {
							t.Fatalf("circular=%t width=%g height=%g scale=%g: 图标中心未对齐或位移超过半像素：%v", circular, width, height, scale, axis)
						}
					}
					if icon.base().windowRect() != bounds || button.windowRect() != hit || win.chrome.queryRegion(hit.Center()) != ChromeRegionClose {
						t.Fatal("绘制改变了布局或关闭按钮命中区域")
					}
				}
			}
		}
	}
}

type captionPixelTarget struct{ controlsFrame }

func (*captionPixelTarget) Transparent() bool { return true }

// 透明预乘 RGBA 的 alpha 表示几何覆盖率。45° 线经过像素中心时，覆盖面积为
// 1-max(0, 1-width/sqrt(2))²；Software 的 26.6 定点描边与 8 位输出允许 3/255。
// 采样远离端帽和两条斜线的交点，同时比较完整图标，避免仅比较两份错误图像。
func TestCloseCaptionGlyphPixelCoverage(t *testing.T) {
	for _, circular := range []bool{false, true} {
		win, _ := chromeFixture(t, false, true)
		win.focused = true
		win.controls.buttons[2].circular = circular
		backend, err := software.NewPainter(new(captionPixelTarget))
		if err != nil {
			t.Fatal(err)
		}
		t.Cleanup(backend.Destroy)
		for _, scale := range []float32{1, 1.25, 1.5, 1.75, 2} {
			var reference *image.RGBA
			for _, x := range []float32{17, 17.25, 17.5} {
				for _, height := range []float32{40, 41, 48, 49, 40.5, 73.25} {
					glyph := &captionIcon{controls: win.controls, index: 2}
					glyph.Arrange(geometry.Rect(x, (height-12)/2, 12, 12))
					img, err := backend.RenderImage(128, 128, scale, func() {
						paintWidget(glyph, newPainter(backend, geometry.Rect(0, 0, 128/scale, 128/scale), scale))
					})
					if err != nil {
						t.Fatal(err)
					}
					cx := int(math.Floor(float64((x+6)*scale) + 0.5))
					cy := int(math.Floor(float64(height*scale/2) + 0.5))
					_, _, _, alpha := img.At(cx-2, cy-2).RGBA()
					want := 1 - math.Pow(max(0, 1-float64(scale)/math.Sqrt2), 2)
					if math.Abs(float64(alpha)/65535-want) > 3.0/255 {
						t.Fatalf("circular=%t scale=%g x=%g height=%g: 斜线覆盖率=%g，独立面积预期=%g", circular, scale, x, height, float64(alpha)/65535, want)
					}
					centered := image.NewRGBA(image.Rect(0, 0, 32, 32))
					for dy := range 32 {
						for dx := range 32 {
							centered.Set(dx, dy, img.At(cx-16+dx, cy-16+dy))
						}
					}
					if reference == nil {
						reference = centered
						continue
					}
					for n, value := range centered.Pix {
						if value != reference.Pix[n] {
							t.Fatalf("circular=%t scale=%g x=%g height=%g: 仅改变布局位置却改变了图标像素 %d：%d != %d", circular, scale, x, height, n, value, reference.Pix[n])
						}
					}
				}
			}
		}
	}
}
