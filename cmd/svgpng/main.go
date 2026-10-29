// Command svgpng rasterizes SVG files into PNG files of several sizes with
// icon/svg, so the output matches what goui Icon renders. Only the static
// profile described in doc/DesignSVG.md is accepted.
//
// Usage:
//
//	go run ./cmd/svgpng [-o dir] [-sizes 16,32,128x64] file.svg...
//
// A plain size N sets the longer edge to N and keeps the SVG aspect ratio;
// WxH requests an exact canvas and centers the image in it. Each output is
// named <name>-<W>x<H>.png and keeps the SVG's own colors and transparency.
package main

import (
	"bytes"
	"errors"
	"flag"
	"fmt"
	"image"
	"image/png"
	"math"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/golang-gui/goui/gui"
	"github.com/golang-gui/goui/icon/svg"
)

const defaultSizes = "16,24,32,48,64,128,256,512"

func main() {
	out := flag.String("o", ".", "output directory")
	sizes := flag.String("sizes", defaultSizes, "comma separated sizes: N (longer edge) or WxH")
	flag.Usage = func() {
		fmt.Fprintf(flag.CommandLine.Output(), "usage: svgpng [-o dir] [-sizes list] file.svg...\n")
		flag.PrintDefaults()
	}
	flag.Parse()
	if flag.NArg() == 0 {
		flag.Usage()
		os.Exit(2)
	}
	specs, err := parseSizes(*sizes)
	if err != nil {
		fmt.Fprintln(os.Stderr, "svgpng:", err)
		os.Exit(2)
	}
	failed := false
	for _, file := range flag.Args() {
		written, err := convert(file, *out, specs)
		for _, name := range written {
			fmt.Println(name)
		}
		if err != nil {
			fmt.Fprintf(os.Stderr, "svgpng: %s: %v\n", file, err)
			failed = true
		}
	}
	if failed {
		os.Exit(1)
	}
}

// sizeSpec is either an exact canvas (width and height) or a longer-edge
// length (edge) resolved against the SVG aspect ratio.
type sizeSpec struct {
	edge          int
	width, height int
}

func parseSizes(list string) ([]sizeSpec, error) {
	var specs []sizeSpec
	for _, field := range strings.Split(list, ",") {
		field = strings.TrimSpace(field)
		if field == "" {
			continue
		}
		w, h, exact := strings.Cut(field, "x")
		width, err := strconv.Atoi(w)
		if err != nil || width <= 0 {
			return nil, fmt.Errorf("invalid size %q", field)
		}
		if !exact {
			specs = append(specs, sizeSpec{edge: width})
			continue
		}
		height, err := strconv.Atoi(h)
		if err != nil || height <= 0 {
			return nil, fmt.Errorf("invalid size %q", field)
		}
		specs = append(specs, sizeSpec{width: width, height: height})
	}
	if len(specs) == 0 {
		return nil, errors.New("no sizes")
	}
	return specs, nil
}

// resolve returns the canvas size for an SVG with the given aspect ratio
// (width / height).
func (s sizeSpec) resolve(aspect float64) (int, int) {
	if s.edge == 0 {
		return s.width, s.height
	}
	if aspect >= 1 {
		return s.edge, max(1, int(math.Round(float64(s.edge)/aspect)))
	}
	return max(1, int(math.Round(float64(s.edge)*aspect))), s.edge
}

func convert(file, outDir string, specs []sizeSpec) ([]string, error) {
	data, err := os.ReadFile(file)
	if err != nil {
		return nil, err
	}
	source, err := svg.Parse(bytes.NewReader(data))
	if err != nil {
		return nil, err
	}
	source = source.WithRawColors()
	aspect := source.AspectRatio()
	base := strings.TrimSuffix(filepath.Base(file), filepath.Ext(file))
	var written []string
	for _, spec := range specs {
		width, height := spec.resolve(aspect)
		img := source.Image(width, height, gui.Color{})
		if img == nil {
			return written, fmt.Errorf("cannot rasterize %dx%d (each edge must be 1..4096)", width, height)
		}
		name := filepath.Join(outDir, fmt.Sprintf("%s-%dx%d.png", base, width, height))
		if err := writePNG(name, img); err != nil {
			return written, err
		}
		written = append(written, name)
	}
	return written, nil
}

// writePNG encodes img; image.RGBA is premultiplied and png.Encode converts
// it to the straight alpha PNG stores.
func writePNG(name string, img image.Image) error {
	f, err := os.Create(name)
	if err != nil {
		return err
	}
	if err := png.Encode(f, img); err != nil {
		f.Close()
		return err
	}
	return f.Close()
}
