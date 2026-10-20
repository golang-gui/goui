package main

import (
	"image/png"
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

func TestParseSizes(t *testing.T) {
	got, err := parseSizes(" 16, 32 ,128x64,")
	if err != nil {
		t.Fatal(err)
	}
	want := []sizeSpec{{edge: 16}, {edge: 32}, {width: 128, height: 64}}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("parseSizes = %+v, want %+v", got, want)
	}
	for _, bad := range []string{"", ",", "0", "-1", "a", "16x", "x16", "16x0"} {
		if _, err := parseSizes(bad); err == nil {
			t.Errorf("parseSizes(%q) succeeded", bad)
		}
	}
}

func TestSizeResolve(t *testing.T) {
	cases := []struct {
		spec          sizeSpec
		aspect        float64
		width, height int
	}{
		{sizeSpec{edge: 64}, 1, 64, 64},
		{sizeSpec{edge: 610}, 1220.0 / 512, 610, 256},
		{sizeSpec{edge: 100}, 0.5, 50, 100},
		{sizeSpec{edge: 1}, 10, 1, 1},
		{sizeSpec{width: 30, height: 20}, 0.5, 30, 20},
	}
	for _, c := range cases {
		w, h := c.spec.resolve(c.aspect)
		if w != c.width || h != c.height {
			t.Errorf("%+v.resolve(%v) = %dx%d, want %dx%d", c.spec, c.aspect, w, h, c.width, c.height)
		}
	}
}

func TestAspectRatio(t *testing.T) {
	cases := map[string]float64{
		`<svg viewBox="0 0 200 100"/>`:                         2,
		`<svg viewBox="10,10 50,100" width="1" height="1"/>`:   0.5,
		`<?xml version="1.0"?><svg width="30px" height="10"/>`: 3,
	}
	for body, want := range cases {
		got, err := aspectRatio([]byte(body))
		if err != nil || got != want {
			t.Errorf("aspectRatio(%s) = %v, %v; want %v", body, got, err, want)
		}
	}
	for _, body := range []string{``, `<svg/>`, `<svg viewBox="0 0 0 10"/>`} {
		if _, err := aspectRatio([]byte(body)); err == nil {
			t.Errorf("aspectRatio(%s) succeeded", body)
		}
	}
}

func TestConvertWritesSizedPNGs(t *testing.T) {
	dir := t.TempDir()
	file := filepath.Join(dir, "wide.svg")
	body := `<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 20 10"><rect width="20" height="10" fill="#FF0000"/></svg>`
	if err := os.WriteFile(file, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	written, err := convert(file, dir, []sizeSpec{{edge: 40}, {width: 8, height: 8}})
	if err != nil {
		t.Fatal(err)
	}
	want := []string{filepath.Join(dir, "wide-40x20.png"), filepath.Join(dir, "wide-8x8.png")}
	if !reflect.DeepEqual(written, want) {
		t.Fatalf("written = %v, want %v", written, want)
	}
	f, err := os.Open(want[0])
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	img, err := png.Decode(f)
	if err != nil {
		t.Fatal(err)
	}
	if b := img.Bounds(); b.Dx() != 40 || b.Dy() != 20 {
		t.Fatalf("bounds = %v, want 40x20", b)
	}
	// Center pixel is fully inside the opaque red rect: raw colors, not tinted.
	r, g, b, a := img.At(20, 10).RGBA()
	if r != 0xFFFF || g != 0 || b != 0 || a != 0xFFFF {
		t.Fatalf("center = %04x %04x %04x %04x, want opaque red", r, g, b, a)
	}

	// The square canvas centers the 2:1 drawing; the top row stays transparent.
	f8, err := os.Open(want[1])
	if err != nil {
		t.Fatal(err)
	}
	defer f8.Close()
	square, err := png.Decode(f8)
	if err != nil {
		t.Fatal(err)
	}
	if _, _, _, a := square.At(4, 0).RGBA(); a != 0 {
		t.Fatalf("letterbox alpha = %04x, want 0", a)
	}
}

func TestConvertRejectsUnsupportedSVG(t *testing.T) {
	dir := t.TempDir()
	file := filepath.Join(dir, "text.svg")
	if err := os.WriteFile(file, []byte(`<svg viewBox="0 0 10 10"><text>x</text></svg>`), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := convert(file, dir, []sizeSpec{{edge: 16}}); err == nil {
		t.Fatal("convert accepted unsupported text element")
	}
	valid := filepath.Join(dir, "valid.svg")
	if err := os.WriteFile(valid, []byte(`<svg viewBox="0 0 10 10"><rect width="10" height="10"/></svg>`), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := convert(valid, dir, []sizeSpec{{edge: 5000}}); err == nil {
		t.Fatal("convert accepted oversized output")
	}
}

// The project logos must stay inside the icon/svg profile so goui itself can
// display them.
func TestProjectLogosConvert(t *testing.T) {
	logos, err := filepath.Glob(filepath.Join("..", "..", "doc", "logo", "*.svg"))
	if err != nil {
		t.Fatal(err)
	}
	if len(logos) == 0 {
		t.Fatal("no logos found in doc/logo")
	}
	for _, logo := range logos {
		if _, err := convert(logo, t.TempDir(), []sizeSpec{{edge: 64}}); err != nil {
			t.Errorf("%s: %v", logo, err)
		}
	}
}
