package main

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"image"
	"image/png"
	"io"
	"math"
	"os"
	"path/filepath"
	"strings"

	"github.com/golang-gui/goui/gui"
	"github.com/golang-gui/goui/icon/svg"
	"github.com/jackmordaunt/icns/v3"
	"golang.org/x/image/draw"
)

// Reuse cmd/svgpng's checked SVG renderer, not a second SVG implementation.
type appIcon struct {
	svg *svg.Source
	png image.Image
	raw []byte
}

func readResource(path string) ([]byte, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	data, err := io.ReadAll(io.LimitReader(f, (16<<20)+1))
	if err == nil && len(data) > 16<<20 {
		err = fmt.Errorf("resource exceeds 16 MiB: %s", path)
	}
	return data, err
}

func loadIcon(c *config) (*appIcon, error) {
	data, err := readResource(c.path(c.Icon))
	if err != nil {
		return nil, err
	}
	icon := &appIcon{raw: data}
	if strings.ToLower(filepath.Ext(c.Icon)) == ".svg" {
		source, err := svg.Parse(bytes.NewReader(data))
		if err != nil {
			return nil, err
		}
		icon.svg = source.WithRawColors()
		return icon, nil
	}
	dimensions, err := png.DecodeConfig(bytes.NewReader(data))
	if err != nil {
		return nil, fmt.Errorf("icon: %w", err)
	}
	if dimensions.Width <= 0 || dimensions.Height <= 0 || dimensions.Width > 4096 || dimensions.Height > 4096 {
		return nil, fmt.Errorf("PNG icon dimensions must be in 1..4096")
	}
	icon.png, err = png.Decode(bytes.NewReader(data))
	return icon, err
}

func (icon *appIcon) image(size int) image.Image {
	if icon.svg != nil {
		return icon.svg.Image(size, size, gui.Color{})
	}
	result := image.NewRGBA(image.Rect(0, 0, size, size))
	b := icon.png.Bounds()
	scale := min(float64(size)/float64(b.Dx()), float64(size)/float64(b.Dy()))
	w, h := max(1, int(math.Round(float64(b.Dx())*scale))), max(1, int(math.Round(float64(b.Dy())*scale)))
	rect := image.Rect((size-w)/2, (size-h)/2, (size-w)/2+w, (size-h)/2+h)
	draw.CatmullRom.Scale(result, rect, icon.png, b, draw.Src, nil)
	return result
}

func pngBytes(img image.Image) ([]byte, error) {
	var b bytes.Buffer
	err := png.Encode(&b, img)
	return b.Bytes(), err
}

// ICO layout follows xuges/goico's multi-PNG approach. ICO encodes 256 as a
// zero width/height byte; rejecting it (as that version does) loses HiDPI icons.
// This private implementation does not add a separate public encoding API.
func (icon *appIcon) ico() ([]byte, error) {
	sizes := []int{16, 24, 32, 48, 64, 128, 256}
	data := make([]byte, 6+16*len(sizes))
	binary.LittleEndian.PutUint16(data[2:], 1)
	binary.LittleEndian.PutUint16(data[4:], uint16(len(sizes)))
	for i, size := range sizes {
		pngData, err := pngBytes(icon.image(size))
		if err != nil {
			return nil, err
		}
		entry := data[6+i*16 : 6+(i+1)*16]
		entry[0], entry[1] = byte(size), byte(size)
		binary.LittleEndian.PutUint16(entry[4:], 1)
		binary.LittleEndian.PutUint16(entry[6:], 32)
		binary.LittleEndian.PutUint32(entry[8:], uint32(len(pngData)))
		binary.LittleEndian.PutUint32(entry[12:], uint32(len(data)))
		data = append(data, pngData...)
	}
	return data, nil
}

func (icon *appIcon) icns() ([]byte, error) {
	set := icns.IconSet{}
	for _, item := range []struct {
		id   string
		size int
	}{{"ic11", 32}, {"ic12", 64}, {"ic07", 128}, {"ic08", 256}, {"ic13", 256}, {"ic09", 512}, {"ic14", 512}, {"ic10", 1024}} {
		set.Icons = append(set.Icons, &icns.Icon{
			Type: icns.OsType{ID: item.id, Size: uint(item.size)}, Image: icon.image(item.size),
		})
	}
	var b bytes.Buffer
	_, err := set.WriteTo(&b)
	return b.Bytes(), err
}

func nativeIcon(c *config, icon *appIcon, osName string) ([]byte, error) {
	path := c.Windows.Icon
	if osName == "darwin" {
		path = c.MacOS.Icon
	}
	if path == "" {
		if osName == "darwin" {
			return icon.icns()
		}
		return icon.ico()
	}
	data, err := readResource(c.path(path))
	if err != nil {
		return nil, err
	}
	if osName == "darwin" {
		if len(data) < 16 || string(data[:4]) != "icns" || uint64(binary.BigEndian.Uint32(data[4:8])) != uint64(len(data)) {
			return nil, fmt.Errorf("invalid ICNS file: %s", path)
		}
		for offset := 8; offset < len(data); {
			if len(data)-offset < 8 {
				return nil, fmt.Errorf("truncated ICNS entry: %s", path)
			}
			length := uint64(binary.BigEndian.Uint32(data[offset+4:]))
			if length < 8 || length > uint64(len(data)-offset) {
				return nil, fmt.Errorf("invalid ICNS entry: %s", path)
			}
			offset += int(length)
		}
	} else {
		if len(data) < 6 || binary.LittleEndian.Uint16(data[:2]) != 0 || binary.LittleEndian.Uint16(data[2:4]) != 1 {
			return nil, fmt.Errorf("invalid ICO file: %s", path)
		}
		n := int(binary.LittleEndian.Uint16(data[4:6]))
		if n == 0 || n > 256 || len(data) < 6+16*n {
			return nil, fmt.Errorf("invalid ICO directory: %s", path)
		}
		for i := 0; i < n; i++ {
			entry := data[6+16*i:]
			size, offset := uint64(binary.LittleEndian.Uint32(entry[8:])), uint64(binary.LittleEndian.Uint32(entry[12:]))
			if size == 0 || offset < uint64(6+16*n) || offset+size > uint64(len(data)) {
				return nil, fmt.Errorf("invalid ICO image offset: %s", path)
			}
		}
	}
	return data, nil
}
