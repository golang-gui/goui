package utils

import (
	"bytes"
	"image"
	"image/color"
	"image/png"
	"os"
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

	err := savePng("src.png", &src)
	if err != nil {
		t.Fatal(err)
	}

	dst := CopyBitmap(src, 80, 80, nil)
	err = savePng("dst.png", &dst)
	if err != nil {
		t.Fatal(err)
	}
}

func TestReverseBitmap(t *testing.T) {
	var bitmap typography.TextBitmap
	bitmap.Width = 100
	bitmap.Height = 1
	bitmap.Stride = bitmap.Width * 4
	bitmap.Pixels = makeLine(bitmap.Stride, color.RGBA{R: 255, A: 255})
	err := savePng("red.png", &bitmap)
	if err != nil {
		t.Fatal(err)
	}
	ReverseBitmap(bitmap)
	err = savePng("blue.png", &bitmap)
	if err != nil {
		t.Fatal(err)
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

func savePng(filename string, img image.Image) error {
	var buf bytes.Buffer
	if err := png.Encode(&buf, img); err != nil {
		return err
	}
	return os.WriteFile(filename, buf.Bytes(), 0666)
}
