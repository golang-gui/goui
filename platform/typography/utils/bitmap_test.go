package utils

import (
	"image/color"
	"testing"

	"github.com/golang-gui/goui/platform/typography"
)

func TestCopyBitmap(t *testing.T) {
	var src typography.TextBitmap
	src.Width = 100
	src.Height = 100
	src.Stride = src.Width * 4
	src.Pixels = make([]uint8, src.Stride*src.Height)

	red := makeLine(src.Stride, color.RGBA{R: 255, A: 255})
	green := makeLine(src.Stride, color.RGBA{G: 255, A: 255})
	for y := 0; y < src.Height; y++ {
		index := src.PixOffset(0, y)
		if y%2 == 0 {
			copy(src.Pixels[index:], red)
		} else {
			copy(src.Pixels[index:], green)
		}
	}

	dst := CopyBitmap(src, 80, 80, nil)
	if dst.Width != 80 || dst.Height != 80 || dst.Stride != 320 || len(dst.Pixels) != 25600 {
		t.Fatalf("裁剪结果尺寸错误：%+v", dst)
	}
	// 内存为预乘 RGBA；不采样抗锯齿边缘，逐像素精确检查原有交替色行。
	for y := range dst.Height {
		want := color.RGBA{R: 255, A: 255}
		if y%2 != 0 {
			want = color.RGBA{G: 255, A: 255}
		}
		for x := range dst.Width {
			if got := dst.At(x, y); got != want {
				t.Fatalf("裁剪像素 (%d,%d)=%v，期望 %v", x, y, got, want)
			}
		}
	}
	dst.Pixels[0] = 0
	if src.Pixels[0] != 255 {
		t.Fatal("裁剪结果与源位图共享像素")
	}
}

func TestReverseBitmap(t *testing.T) {
	var bitmap typography.TextBitmap
	bitmap.Width = 100
	bitmap.Height = 1
	bitmap.Stride = bitmap.Width * 4
	bitmap.Pixels = makeLine(bitmap.Stride, color.RGBA{R: 255, A: 255})
	ReverseBitmap(bitmap)
	for x := range bitmap.Width {
		if got := bitmap.At(x, 0); got != (color.RGBA{B: 255, A: 255}) {
			t.Fatalf("交换后的像素 %d=%v，期望不透明蓝色", x, got)
		}
	}
}

func makeLine(stride int, c color.Color) []byte {
	line := make([]byte, stride)
	rgba := color.RGBAModel.Convert(c).(color.RGBA)
	for i := 0; i < len(line); i += 4 {
		line[i] = rgba.R
		line[i+1] = rgba.G
		line[i+2] = rgba.B
		line[i+3] = rgba.A
	}
	return line
}
