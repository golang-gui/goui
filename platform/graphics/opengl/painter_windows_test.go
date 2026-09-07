package opengl

import (
	"fmt"
	"image"
	"image/color"
	"math"
	"runtime"
	"syscall"
	"testing"

	"github.com/goexlib/cgo"
	"github.com/golang-gui/goui/core/geometry"
	"github.com/golang-gui/goui/platform/graphics"
	"github.com/golang-gui/goui/platform/windows/sdk/winapi"
	"github.com/golang-gui/nanovgo/gl"
)

// Read the native rasterizer's pixels, including NanoVG's shader scissor and
// AA fringe. Coordinate-only tests cannot detect a border weakened by clipping.
func TestRectanglePixelsWithWidgetTranslationAndClip(t *testing.T) {
	for _, scale := range []float32{1, 1.25, 1.5, 1.75, 2} {
		t.Run(fmt.Sprint(scale), func(t *testing.T) {
			f := newPixelFixture(t)
			outer := graphics.Rect(0, 0, 23.4, 17.7)
			offset := graphics.Point{X: 6.2, Y: 5.3}
			clip := outer.Translate(offset)
			for _, pixels := range []int{0, 1, 2, 3} {
				for _, radius := range []float32{0, 3} {
					t.Logf("pixels=%d radius=%g", pixels, radius)
					width := float32(pixels) / scale
					draw := func(p *Painter) {
						black := graphics.RGB(0, 0, 0)
						if pixels == 0 {
							p.FillRoundRect(outer, radius, black)
						} else {
							p.DrawRoundRect(outer.Inset(width/2), radius, width, black)
						}
					}
					transform := geometry.Translate(offset.X, offset.Y)
					img := f.render(scale, transform, clip, draw)
					unclipped := f.render(scale, transform, graphics.Rectangle{}, draw)
					assertSamePixels(t, img, unclipped)
					left := int(math.Round(float64(clip.X * scale)))
					top := int(math.Round(float64(clip.Y * scale)))
					right := int(math.Round(float64((clip.X + clip.Width) * scale)))
					bottom := int(math.Round(float64((clip.Y + clip.Height) * scale)))
					for x := left - 1; x <= right; x++ {
						ink := x >= left && x < right && (pixels == 0 || x < left+pixels || x >= right-pixels)
						assertPixel(t, img, x, (top+bottom)/2, ink)
					}
					for y := top - 1; y <= bottom; y++ {
						ink := y >= top && y < bottom && (pixels == 0 || y < top+pixels || y >= bottom-pixels)
						assertPixel(t, img, (left+right)/2, y, ink)
					}
				}
			}
		})
	}
}

func TestStraightLinePixelsWithTranslation(t *testing.T) {
	f := newPixelFixture(t)
	for _, scale := range []float32{1, 1.25, 1.5, 1.75, 2} {
		for _, pixels := range []int{1, 2, 3} {
			for _, vertical := range []bool{false, true} {
				width := float32(pixels) / scale
				p0, p1 := graphics.Point{X: 0, Y: width / 2}, graphics.Point{X: 20, Y: width / 2}
				if vertical {
					p0, p1 = graphics.Point{X: width / 2, Y: 0}, graphics.Point{X: width / 2, Y: 20}
				}
				img := f.render(scale, geometry.Translate(6.2, 5.3), graphics.Rectangle{}, func(p *Painter) {
					p.DrawLine(p0, p1, width, graphics.RGB(0, 0, 0))
				})
				for delta := -1; delta <= pixels; delta++ {
					x, y := 15, int(math.Round(float64(float32(5.3)*scale)))+delta
					if vertical {
						x, y = int(math.Round(float64(float32(6.2)*scale)))+delta, 15
					}
					assertPixel(t, img, x, y, delta >= 0 && delta < pixels)
				}
			}
		}
	}
}

func TestSubpixelGeometryUsesExactDeviceTransform(t *testing.T) {
	f := newPixelFixture(t)
	src := image.NewRGBA(image.Rect(0, 0, 3, 2))
	src.SetRGBA(0, 0, color.RGBA{R: 255, A: 255})
	src.SetRGBA(2, 1, color.RGBA{B: 255, A: 255})
	img, err := f.p.NewImage(src)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(img.Destroy)
	for _, scale := range []float32{1.25, 1.5, 1.75, 2} {
		transform := geometry.Translate(3.2, 4.7)
		draw := func(p *Painter) {
			black := graphics.RGB(0, 0, 0)
			path := graphics.MoveTo(5.2, 5.3).LineTo(17.8, 5.3).
				ArcTo(3.2, 4.7, 0, 0, 1, 21, 10).
				BezierTo(20.6, 16.1, 9.7, 17.3, 5.2, 5.3).Close()
			p.FillPath(path, black)
			p.DrawLine(graphics.Point{X: 0.2, Y: 19.7}, graphics.Point{X: 23.8, Y: 22.6}, 1, black)
			p.FillEllipse(graphics.Point{X: 29.4, Y: 6.3}, 4.7, 3.2, black)
			p.DrawImage(graphics.Rect(23.2, 14.7, 11.3, 8.1), img)
		}
		// DPI scaling must be exactly the same device transform as explicit
		// user scaling for primitives whose subpixel geometry we must preserve.
		got := f.render(scale, transform, graphics.Rectangle{}, draw)
		want := f.render(1, geometry.Scale(scale, scale).Multiply(transform), graphics.Rectangle{}, draw)
		assertSamePixels(t, got, want)
	}
}

func TestBeginResetsTransformAndClip(t *testing.T) {
	f := newPixelFixture(t)
	f.render(1.5, geometry.Translate(6.2, 5.3), graphics.Rect(0, 0, 1, 1), func(p *Painter) {})
	p := f.p
	p.Begin(pixelWidth, pixelHeight, 2)
	p.Clear(graphics.RGB(255, 255, 255))
	p.FillRect(graphics.Rect(2, 2, 2, 2), graphics.RGB(0, 0, 0))
	p.End()
	img := f.readback()
	for y := 0; y < img.Rect.Dy(); y++ {
		for x := 0; x < img.Rect.Dx(); x++ {
			assertPixel(t, img, x, y, x >= 4 && x < 8 && y >= 4 && y < 8)
		}
	}
}

func assertSamePixels(t *testing.T, got, want *image.RGBA) {
	t.Helper()
	for i, value := range got.Pix {
		// Shader interpolation at AA corners can differ by one 8-bit level.
		// Straight edges are checked separately with exact opaque/empty values.
		if delta := int(value) - int(want.Pix[i]); delta < -1 || delta > 1 {
			x, y := (i%got.Stride)/4, i/got.Stride
			t.Fatalf("pixel (%d,%d) = %v, want %v", x, y, got.RGBAAt(x, y), want.RGBAAt(x, y))
		}
	}
}

func TestFractionalDIPBorderKeepsStrokeCoverage(t *testing.T) {
	f := newPixelFixture(t)
	for _, scale := range []float32{1.25, 1.5, 1.75} {
		outer := graphics.Rect(0, 0, 23.4, 17.7)
		offset := graphics.Point{X: 6.2, Y: 5.3}
		img := f.render(scale, geometry.Translate(offset.X, offset.Y), outer.Translate(offset), func(p *Painter) {
			p.DrawRect(outer.Inset(0.5), 1, graphics.RGB(0, 0, 0))
		})
		left := int(math.Round(float64(offset.X * scale)))
		y := int((offset.Y + outer.Height/2) * scale)
		assertPixel(t, img, left-1, y, false)
		assertPixel(t, img, left, y, true)
		var coverage float64
		for x := left; x < left+3; x++ {
			coverage += float64(255-img.RGBAAt(x, y).R) / 255
		}
		if math.Abs(coverage-float64(scale)) > 0.02 {
			t.Fatalf("1 DIP stroke at scale %g covers %g pixels", scale, coverage)
		}
	}
}

func TestClipIsWindowLocalAndCanBeReplaced(t *testing.T) {
	f := newPixelFixture(t)
	img := f.render(1.5, geometry.Translate(4.2, 3.7).Rotate(20), graphics.Rect(0, 0, 0.2, 0.2), func(p *Painter) {
		p.FillRect(graphics.Rect(-100, -100, 200, 200), graphics.RGB(0, 0, 0))
		p.SetClipRect(graphics.Rect(4, 4, 2, 2))
		p.FillRect(graphics.Rect(-100, -100, 200, 200), graphics.RGB(0, 0, 0))
		p.SetClipRect(graphics.Rectangle{})
		p.SetTransform(geometry.Identity())
		p.FillRect(graphics.Rect(10, 10, 2, 2), graphics.RGB(0, 0, 0))
	})
	for y := 0; y < img.Rect.Dy(); y++ {
		for x := 0; x < img.Rect.Dx(); x++ {
			ink := (x >= 6 && x < 9 && y >= 6 && y < 9) || (x >= 15 && x < 18 && y >= 15 && y < 18)
			assertPixel(t, img, x, y, ink)
		}
	}
}

func assertPixel(t *testing.T, img *image.RGBA, x, y int, ink bool) {
	t.Helper()
	want := color.RGBA{255, 255, 255, 255}
	if ink {
		want = color.RGBA{0, 0, 0, 255}
	}
	if got := img.RGBAAt(x, y); got != want {
		t.Fatalf("pixel (%d,%d) = %v, want %v", x, y, got, want)
	}
}

// Deliberately not divisible by the tested DPI scales: truncating the logical
// BeginFrame dimensions stretches the viewport and defeats pixel alignment.
const pixelWidth, pixelHeight = 83, 67

type pixelWindow winapi.HWND

func (w pixelWindow) NativeHandle() uintptr { return uintptr(w) }
func (w pixelWindow) RequestPaint() error   { return nil }

type pixelFixture struct {
	t          *testing.T
	p          *Painter
	readPixels uintptr
}

func newPixelFixture(t *testing.T) *pixelFixture {
	t.Helper()
	runtime.LockOSThread()
	t.Cleanup(runtime.UnlockOSThread)
	instance, err := winapi.GetModuleHandle(nil)
	if err != nil {
		t.Fatal(err)
	}
	class, _ := syscall.UTF16PtrFromString("STATIC")
	wnd, err := winapi.CreateWindowEx(0, class, nil, winapi.WS_POPUP, 0, 0, pixelWidth, pixelHeight, 0, 0, instance, nil)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { winapi.DestroyWindow(wnd) })
	native, err := NewPainter(pixelWindow(wnd))
	if err != nil {
		t.Fatal(err)
	}
	p := native.(*Painter)
	t.Cleanup(p.Destroy)
	proc := func(name string) uintptr {
		t.Helper()
		fn, err := p.ctx.GetProcAddress(name)
		if err != nil || fn == 0 {
			t.Fatalf("load %s: %v", name, err)
		}
		return fn
	}
	// Use an FBO so readback does not depend on window visibility or occlusion.
	var framebuffer uint32
	var buffers [2]uint32
	cgo.Call(proc("glGenFramebuffers"), int32(1), &framebuffer)
	cgo.Call(proc("glBindFramebuffer"), uint32(gl.FRAMEBUFFER), framebuffer)
	cgo.Call(proc("glGenRenderbuffers"), int32(2), &buffers[0])
	deleteFramebuffers, deleteRenderbuffers := proc("glDeleteFramebuffers"), proc("glDeleteRenderbuffers")
	t.Cleanup(func() {
		_ = p.ctx.MakeCurrent()
		cgo.Call(deleteFramebuffers, int32(1), &framebuffer)
		cgo.Call(deleteRenderbuffers, int32(2), &buffers[0])
	})
	for i, format := range []uint32{gl.RGBA8, gl.DEPTH24_STENCIL8} {
		cgo.Call(proc("glBindRenderbuffer"), uint32(gl.RENDERBUFFER), buffers[i])
		cgo.Call(proc("glRenderbufferStorage"), uint32(gl.RENDERBUFFER), format, int32(pixelWidth), int32(pixelHeight))
		attachment := uint32(gl.COLOR_ATTACHMENT0)
		if i == 1 {
			attachment = gl.DEPTH_STENCIL_ATTACHMENT
		}
		cgo.Call(proc("glFramebufferRenderbuffer"), uint32(gl.FRAMEBUFFER), attachment, uint32(gl.RENDERBUFFER), buffers[i])
	}
	if status := cgo.CallRet[uint32](proc("glCheckFramebufferStatus"), uint32(gl.FRAMEBUFFER)); status != gl.FRAMEBUFFER_COMPLETE {
		t.Fatalf("incomplete framebuffer: %#x", status)
	}
	f := &pixelFixture{t: t, p: p, readPixels: proc("glReadPixels")}
	if err := p.ctx.ClearCurrent(); err != nil {
		t.Fatal(err)
	}
	return f
}

func (f *pixelFixture) render(scale float32, transform geometry.Transform, clip graphics.Rectangle, draw func(*Painter)) *image.RGBA {
	f.t.Helper()
	p := f.p
	p.Begin(pixelWidth, pixelHeight, scale)
	p.Clear(graphics.RGB(255, 255, 255))
	p.SetTransform(transform)
	p.SetClipRect(clip)
	draw(p)
	p.End()
	return f.readback()
}

func (f *pixelFixture) readback() *image.RGBA {
	f.t.Helper()
	p := f.p
	if err := p.ctx.MakeCurrent(); err != nil {
		f.t.Fatal(err)
	}
	defer p.ctx.ClearCurrent()
	pixels := make([]byte, pixelWidth*pixelHeight*4)
	cgo.Call(f.readPixels, int32(0), int32(0), int32(pixelWidth), int32(pixelHeight), uint32(gl.RGBA), uint32(gl.UNSIGNED_BYTE), &pixels[0])
	if err := gl.GetError(); err != gl.NO_ERROR {
		f.t.Fatalf("OpenGL error: %#x", err)
	}
	img := image.NewRGBA(image.Rect(0, 0, pixelWidth, pixelHeight))
	for y := 0; y < pixelHeight; y++ {
		copy(img.Pix[y*img.Stride:(y+1)*img.Stride], pixels[(pixelHeight-1-y)*img.Stride:(pixelHeight-y)*img.Stride])
	}
	return img
}
