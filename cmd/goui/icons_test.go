package main

import (
	"bytes"
	"encoding/binary"
	"image"
	"image/color"
	"image/png"
	"path/filepath"
	"testing"
)

// Samples are straight NRGBA, well inside the shapes: exact red/green, alpha
// 0/128/255 (one level for 0.5 rounding). The 2:1 SVG/PNG must be contained,
// with transparent top/bottom. No platform-dependent edge samples are used.
func assertIconPixels(t *testing.T, img image.Image, size int) {
	t.Helper()
	if img.Bounds().Dx() != size || img.Bounds().Dy() != size {
		t.Fatal("wrong icon dimensions", img.Bounds())
	}
	for _, sample := range []struct {
		x, y int
		want color.NRGBA
	}{
		{size / 2, 0, color.NRGBA{}},
		{size / 4, size / 2, color.NRGBA{R: 255, A: 255}},
		{size * 3 / 4, size / 2, color.NRGBA{G: 255, A: 128}},
	} {
		got := color.NRGBAModel.Convert(img.At(sample.x, sample.y)).(color.NRGBA)
		if got.R != sample.want.R || got.G != sample.want.G || got.B != sample.want.B || absDiff(got.A, sample.want.A) > 1 {
			t.Fatalf("%dx%d sample (%d,%d): got %v, want %v", size, size, sample.x, sample.y, got, sample.want)
		}
	}
}

func absDiff(a, b uint8) int { return max(int(a)-int(b), int(b)-int(a)) }

func TestIcons(t *testing.T) {
	c := testConfig(t)
	icon, err := loadIcon(c)
	if err != nil {
		t.Fatal(err)
	}
	for _, size := range []int{16, 32, 256, 1024} {
		assertIconPixels(t, icon.image(size), size)
	}
	t.Run("ICO includes 256", func(t *testing.T) {
		data, err := icon.ico()
		if err != nil {
			t.Fatal(err)
		}
		if binary.LittleEndian.Uint16(data[2:]) != 1 || binary.LittleEndian.Uint16(data[4:]) != 7 {
			t.Fatal("bad ICO header")
		}
		previous := uint32(6 + 7*16)
		for i, size := range []int{16, 24, 32, 48, 64, 128, 256} {
			entry := data[6+i*16:]
			if int(entry[0]) != size%256 || int(entry[1]) != size%256 || binary.LittleEndian.Uint16(entry[6:]) != 32 {
				t.Fatalf("bad ICO dimensions/bit depth: %v", entry[:16])
			}
			length, offset := binary.LittleEndian.Uint32(entry[8:]), binary.LittleEndian.Uint32(entry[12:])
			if offset != previous || uint64(offset)+uint64(length) > uint64(len(data)) {
				t.Fatal("bad ICO offset")
			}
			img, err := png.Decode(bytes.NewReader(data[offset : offset+length]))
			if err != nil {
				t.Fatal(err)
			}
			assertIconPixels(t, img, size)
			previous = offset + length
		}
		if int(previous) != len(data) {
			t.Fatal("trailing ICO data")
		}
	})
	t.Run("ICNS independent entries", func(t *testing.T) {
		data, err := icon.icns()
		if err != nil {
			t.Fatal(err)
		}
		if string(data[:4]) != "icns" || binary.BigEndian.Uint32(data[4:]) != uint32(len(data)) {
			t.Fatal("bad ICNS header")
		}
		want := map[string]int{"ic11": 32, "ic12": 64, "ic07": 128, "ic08": 256, "ic13": 256, "ic09": 512, "ic14": 512, "ic10": 1024}
		for offset := 8; offset < len(data); {
			id := string(data[offset : offset+4])
			length := int(binary.BigEndian.Uint32(data[offset+4:]))
			if length < 8 || offset+length > len(data) || want[id] == 0 {
				t.Fatal("bad ICNS entry", id)
			}
			img, err := png.Decode(bytes.NewReader(data[offset+8 : offset+length]))
			if err != nil {
				t.Fatal(err)
			}
			assertIconPixels(t, img, want[id])
			delete(want, id)
			offset += length
		}
		if len(want) != 0 {
			t.Fatal("missing ICNS entries", want)
		}
	})
}

func TestPNGIcon(t *testing.T) {
	c := testConfig(t)
	img := image.NewNRGBA(image.Rect(0, 0, 80, 40))
	for y := 0; y < 40; y++ {
		for x := 0; x < 80; x++ {
			pixel := color.NRGBA{R: 255, A: 255}
			if x >= 40 {
				pixel = color.NRGBA{G: 255, A: 128}
			}
			img.SetNRGBA(x, y, pixel)
		}
	}
	data, err := pngBytes(img)
	if err != nil {
		t.Fatal(err)
	}
	c.Icon = "icon.png"
	writeTestFile(t, c.path(c.Icon), data)
	icon, err := loadIcon(c)
	if err != nil {
		t.Fatal(err)
	}
	assertIconPixels(t, icon.image(64), 64)
	assertIconPixels(t, icon.image(256), 256)
}

func TestInvalidResources(t *testing.T) {
	c := testConfig(t)
	icon, err := loadIcon(c)
	if err != nil {
		t.Fatal(err)
	}
	for _, item := range []struct{ osName, file string }{{"windows", "bad.ico"}, {"darwin", "bad.icns"}} {
		for _, bad := range [][]byte{nil, []byte("icns\x00\x00\x00\x08"), []byte("\x00\x00\x01\x00\x01\x00")} {
			writeTestFile(t, filepath.Join(c.dir, item.file), bad)
			c.Windows.Icon, c.MacOS.Icon = item.file, item.file
			if _, err := nativeIcon(c, icon, item.osName); err == nil {
				t.Fatal("accepted malformed native icon", item.file)
			}
		}
	}
	c.Icon = "bad.svg"
	writeTestFile(t, c.path(c.Icon), []byte("<svg><"))
	if _, err := loadIcon(c); err == nil {
		t.Fatal("accepted malformed SVG")
	}
}
