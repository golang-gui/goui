package graphics

import (
	"image"
	"image/color"
	"testing"
)

func bitmapEqual(a, b Bitmap) bool {
	if a.X != b.X || a.Y != b.Y || a.Width != b.Width || a.Height != b.Height || a.Stride != b.Stride || a.Format != b.Format {
		return false
	}
	if len(a.Pixels) != len(b.Pixels) {
		return false
	}
	for i := range a.Pixels {
		if a.Pixels[i] != b.Pixels[i] {
			return false
		}
	}
	return true
}

func TestMakeBitmap(t *testing.T) {
	t.Run("RGBA zero origin", func(t *testing.T) {
		bmp := MakeBitmap(0, 0, 100, 50, PixelFormatRGBA, nil)
		if bmp.X != 0 || bmp.Y != 0 || bmp.Width != 100 || bmp.Height != 50 {
			t.Fatalf("unexpected geometry: %+v", bmp)
		}
		if bmp.Stride != 100*4 {
			t.Fatalf("unexpected stride: %d", bmp.Stride)
		}
		if len(bmp.Pixels) != 100*50*4 {
			t.Fatalf("unexpected pixel buffer length: %d", len(bmp.Pixels))
		}
	})

	t.Run("Gray with buffer reuse", func(t *testing.T) {
		buf := make([]byte, 100*100) // 正好 10000 字节
		bmp := MakeBitmap(10, 20, 100, 100, PixelFormatGray, buf)
		if &bmp.Pixels[0] != &buf[0] {
			t.Fatal("expected buffer reuse")
		}
		if len(bmp.Pixels) != 100*100 {
			t.Fatalf("unexpected length: %d", len(bmp.Pixels))
		}
	})

	t.Run("small buffer growth", func(t *testing.T) {
		small := make([]byte, 10)
		bmp := MakeBitmap(0, 0, 100, 100, PixelFormatRGBA, small)
		if len(bmp.Pixels) != 100*100*4 {
			t.Fatalf("expected grown buffer length, got %d", len(bmp.Pixels))
		}
	})
}

func TestBounds(t *testing.T) {
	bmp := MakeBitmap(10, 20, 30, 40, PixelFormatRGBA, nil)
	r := bmp.Bounds()
	if r.Min.X != 10 || r.Min.Y != 20 || r.Max.X != 40 || r.Max.Y != 60 {
		t.Fatalf("unexpected bounds: %v", r)
	}
}

func TestPixOffset(t *testing.T) {
	bmp := MakeBitmap(1, 2, 10, 10, PixelFormatRGBA, nil)
	off := bmp.PixOffset(1, 2)
	if off != 0 {
		t.Fatalf("offset at (1,2) should be 0, got %d", off)
	}
	off = bmp.PixOffset(2, 2)
	if off != 4 {
		t.Fatalf("offset at (2,2) should be 4, got %d", off)
	}
	off = bmp.PixOffset(1, 3)
	if off != 40 {
		t.Fatalf("offset at (1,3) should be 40, got %d", off)
	}
}

func TestGetSetPixelRGBA(t *testing.T) {
	bmp := MakeBitmap(0, 0, 2, 2, PixelFormatRGBA, nil)
	bmp.SetPixel(0, 0, 255, 128, 64, 200)
	r, g, b, a := bmp.GetPixel(0, 0)
	if r != 255 || g != 128 || b != 64 || a != 200 {
		t.Fatalf("unexpected RGBA: %d %d %d %d", r, g, b, a)
	}
	bmp.SetPixel(5, 5, 0, 0, 0, 0)
	r, g, b, a = bmp.GetPixel(5, 5)
	if r != 0 || g != 0 || b != 0 || a != 0 {
		t.Fatalf("out-of-bounds should return zero, got %v", []byte{r, g, b, a})
	}
}

func TestGetSetPixelBGRA(t *testing.T) {
	bmp := MakeBitmap(0, 0, 1, 1, PixelFormatBGRA, nil)
	bmp.SetPixel(0, 0, 10, 20, 30, 40)
	r, g, b, a := bmp.GetPixel(0, 0)
	if r != 10 || g != 20 || b != 30 || a != 40 {
		t.Fatalf("expected (10,20,30,40), got (%d,%d,%d,%d)", r, g, b, a)
	}
	if bmp.Pixels[0] != 30 || bmp.Pixels[1] != 20 || bmp.Pixels[2] != 10 || bmp.Pixels[3] != 40 {
		t.Fatalf("raw BGRA wrong: %v", bmp.Pixels[:4])
	}
}

func TestGetSetPixelGray(t *testing.T) {
	bmp := MakeBitmap(0, 0, 2, 2, PixelFormatGray, nil)
	bmp.SetPixel(0, 0, 128, 128, 128, 255)
	a := bmp.Pixels[0]
	if a != 128 {
		t.Fatalf("unexpected gray value: %d", a)
	}
	r, g, b, a2 := bmp.GetPixel(0, 0)
	if r != 128 || g != 128 || b != 128 || a2 != 255 {
		t.Fatalf("get gray expected all 128, got (%d,%d,%d,%d)", r, g, b, a2)
	}
}

func TestAtSet(t *testing.T) {
	bmp := MakeBitmap(0, 0, 10, 10, PixelFormatRGBA, nil)
	c := color.RGBA{R: 100, G: 150, B: 200, A: 255}
	bmp.Set(5, 5, c)
	got := bmp.At(5, 5).(color.RGBA)
	if got != c {
		t.Fatalf("At/Set mismatch: %+v vs %+v", got, c)
	}
	zero := bmp.At(20, 20).(color.RGBA)
	if zero != (color.RGBA{}) {
		t.Fatal("out-of-bounds At should return zero value")
	}
	bmp.Set(20, 20, color.White)
}

func TestColorModel(t *testing.T) {
	tests := []struct {
		format PixelFormat
		model  color.Model
		col    color.Color
		want   color.Color
	}{
		{PixelFormatRGBA, color.RGBAModel, color.RGBA{1, 2, 3, 4}, color.RGBA{1, 2, 3, 4}},
		{PixelFormatBGRA, BGRAModel, color.RGBA{10, 20, 30, 40}, BGRA{B: 30, G: 20, R: 10, A: 40}},
		{PixelFormatGray, color.GrayModel, color.RGBA{50, 50, 50, 255}, color.Gray{Y: 50}},
	}
	for _, tc := range tests {
		bmp := Bitmap{Format: tc.format}
		model := bmp.ColorModel()
		converted := model.Convert(tc.col)
		if converted != tc.want {
			t.Errorf("format %v: got %v, want %v", tc.format, converted, tc.want)
		}
	}
}

func TestSubImage(t *testing.T) {
	bmp := MakeBitmap(0, 0, 4, 4, PixelFormatRGBA, nil)
	for y := 0; y < 4; y++ {
		for x := 0; x < 4; x++ {
			bmp.SetPixel(x, y, byte(10+x), byte(20+y), 0, 255)
		}
	}
	subImg := bmp.SubImage(image.Rect(1, 1, 3, 3)).(Bitmap)
	if subImg.Width != 2 || subImg.Height != 2 {
		t.Fatalf("wrong subimage size: %dx%d", subImg.Width, subImg.Height)
	}
	r, g, _, _ := subImg.GetPixel(2, 2) // 子图坐标 (2,2) 超出范围，会返回 0
	if r != 0 {
		t.Log("out of bounds returns zero as expected")
	}
	r, g, _, _ = subImg.GetPixel(1, 1)
	if r != 11 || g != 21 {
		t.Fatalf("at (1,1) expected (11,21), got (%d,%d)", r, g)
	}
	r, g, _, _ = subImg.GetPixel(2, 2)
	if r != 12 || g != 22 {
		t.Fatalf("at (2,2) expected (12,22), got (%d,%d)", r, g)
	}
	r, _, _, _ = subImg.GetPixel(0, 0)
	if r != 0 {
		t.Fatal("out-of-bounds (0,0) should return zero")
	}
	subImg.SetPixel(2, 2, 99, 88, 77, 66)
	r, g, _, _ = bmp.GetPixel(2, 2)
	if r != 99 || g != 88 {
		t.Fatalf("share not working: %d,%d", r, g)
	}
}

func TestToBitmap(t *testing.T) {
	t.Run("RGBA same format zero copy", func(t *testing.T) {
		src := image.NewRGBA(image.Rect(1, 2, 5, 6))
		dst, ok := ToBitmap(src, PixelFormatRGBA)
		if !ok {
			t.Fatal("should succeed")
		}
		if dst.Format != PixelFormatRGBA {
			t.Fatal("wrong format")
		}
		if &dst.Pixels[0] != &src.Pix[0] {
			t.Fatal("not zero copy")
		}
	})

	t.Run("RGBA different format fail", func(t *testing.T) {
		src := image.NewRGBA(image.Rect(0, 0, 10, 10))
		_, ok := ToBitmap(src, PixelFormatBGRA)
		if ok {
			t.Fatal("should not succeed for format mismatch")
		}
	})

	t.Run("Gray same format zero copy", func(t *testing.T) {
		src := image.NewGray(image.Rect(0, 0, 10, 10))
		dst, ok := ToBitmap(src, PixelFormatGray)
		if !ok {
			t.Fatal("should succeed for Gray -> Gray")
		}
		if &dst.Pixels[0] != &src.Pix[0] {
			t.Fatal("not zero copy for Gray")
		}
	})

	t.Run("bitmap roundtrip", func(t *testing.T) {
		orig := MakeBitmap(5, 5, 5, 5, PixelFormatRGBA, nil)
		orig.SetPixel(5, 5, 1, 2, 3, 4)
		var iface image.Image = orig
		dst, ok := ToBitmap(iface, PixelFormatRGBA)
		if !ok {
			t.Fatal("bitmap to bitmap should succeed")
		}
		if !bitmapEqual(orig, dst) {
			t.Fatal("bitmap roundtrip not equal")
		}
	})
}

func TestCopyToBitmap(t *testing.T) {
	t.Run("RGBA same stride fast copy", func(t *testing.T) {
		src := image.NewRGBA(image.Rect(1, 2, 11, 12)) // 10x10
		for i := range src.Pix {
			src.Pix[i] = byte(i % 256)
		}
		dst := CopyToBitmap(src, PixelFormatRGBA, nil)
		if !bitmapEqual(dst, Bitmap{
			X: 1, Y: 2, Width: 10, Height: 10,
			Stride: 40, Format: PixelFormatRGBA, Pixels: src.Pix,
		}) {
			t.Fatal("fast copy failed")
		}
		dst.Pixels[0] = 200
		if src.Pix[0] == 200 {
			t.Fatal("not a copy")
		}
	})

	t.Run("Gray same stride fast copy", func(t *testing.T) {
		src := image.NewGray(image.Rect(0, 0, 100, 50))
		for i := range src.Pix {
			src.Pix[i] = byte(i % 255)
		}
		dst := CopyToBitmap(src, PixelFormatGray, nil)
		if dst.Format != PixelFormatGray {
			t.Fatal("expected gray format")
		}
		if len(dst.Pixels) != len(src.Pix) {
			t.Fatal("length mismatch")
		}
		if dst.Pixels[0] != src.Pix[0] {
			t.Fatal("content mismatch")
		}
	})

	t.Run("Bitmap same format copy", func(t *testing.T) {
		orig := MakeBitmap(2, 3, 5, 5, PixelFormatRGBA, nil)
		orig.SetPixel(2, 3, 100, 101, 102, 103)
		cp := CopyToBitmap(orig, PixelFormatRGBA, nil)
		if !bitmapEqual(orig, cp) {
			t.Fatal("Bitmap copy not equal")
		}
		cp.SetPixel(2, 3, 0, 0, 0, 0)
		r, g, _, _ := orig.GetPixel(2, 3)
		if r != 100 || g != 101 {
			t.Fatal("original was mutated")
		}
	})

	t.Run("different format conversion", func(t *testing.T) {
		rgba := MakeBitmap(0, 0, 2, 2, PixelFormatRGBA, nil)
		rgba.SetPixel(0, 0, 10, 20, 30, 40)
		rgba.SetPixel(1, 0, 50, 60, 70, 80)
		gray := CopyToBitmap(rgba, PixelFormatGray, nil)
		if gray.Format != PixelFormatGray {
			t.Fatal("expected gray")
		}
		a := gray.Pixels[0]
		if a != 10 {
			t.Fatalf("first gray pixel expected 10, got %d", a)
		}
		a = gray.Pixels[1]
		if a != 50 {
			t.Fatalf("second gray pixel expected 50, got %d", a)
		}
	})

	t.Run("generic image fallback", func(t *testing.T) {
		alpha := image.NewAlpha(image.Rect(10, 20, 12, 22))
		alpha.SetAlpha(10, 20, color.Alpha{A: 150})
		alpha.SetAlpha(11, 20, color.Alpha{A: 200})
		dst := CopyToBitmap(alpha, PixelFormatGray, nil)
		if dst.Pixels[0] != 150 || dst.Pixels[1] != 200 {
			t.Fatalf("fallback conversion wrong: [%d, %d]", dst.Pixels[0], dst.Pixels[1])
		}
	})

	t.Run("reuse provided buffer", func(t *testing.T) {
		buf := make([]byte, 100*100*4)
		src := image.NewRGBA(image.Rect(0, 0, 100, 100))
		dst := CopyToBitmap(src, PixelFormatRGBA, buf)
		if &dst.Pixels[0] != &buf[0] {
			t.Fatal("did not reuse provided buffer")
		}
	})
}

func TestBitmapImplementsImage(t *testing.T) {
	var _ image.Image = Bitmap{}
	var _ image.Image = &Bitmap{}
}

func TestBGRAConversion(t *testing.T) {
	bmp := MakeBitmap(0, 0, 1, 1, PixelFormatBGRA, nil)
	bmp.SetPixel(0, 0, 255, 128, 0, 200)
	c := bmp.At(0, 0)
	bgra, ok := c.(BGRA)
	if !ok {
		t.Fatalf("At did not return BGRA, got %T", c)
	}
	if bgra.R != 255 || bgra.G != 128 || bgra.B != 0 || bgra.A != 200 {
		t.Fatalf("wrong BGRA value: %+v", bgra)
	}
}

func TestGrayAtReturnsGray(t *testing.T) {
	bmp := MakeBitmap(0, 0, 1, 1, PixelFormatGray, nil)
	bmp.SetPixel(0, 0, 99, 99, 99, 99)
	c := bmp.At(0, 0).(color.Gray)
	if c.Y != 99 {
		t.Fatalf("expected Gray{Y:99}, got %+v", c)
	}
}
