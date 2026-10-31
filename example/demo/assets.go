package main

import (
	"image"
	"image/color"
	"strings"

	"github.com/golang-gui/goui/example/demo/resource"
	"github.com/golang-gui/goui/icon/draw"
	"github.com/golang-gui/goui/icon/svg"
	"github.com/golang-gui/goui/platform/graphics"
)

func (d *demo) loadAssets() error {
	logo, err := svg.Parse(strings.NewReader(resource.LogoMarkSVG()))
	if err != nil {
		return err
	}
	// Branding always keeps its original colors, independently of icon styles.
	// Never add a tintable copy of the logo to the ordinary icon samples.
	d.logo = logo.WithRawColors()
	paths := []string{
		`<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 24 24"><path d="M6 3H14L19 8V21H6Z M14 3V8H19 M9 12H16 M9 16H16" fill="none" stroke="#303840" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"/></svg>`,
		`<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 24 24"><path d="M2 5H10L12 7H22V20H2Z" fill="#e0ac42"/><path d="M3 9H21V19H3Z" fill="#f4cd6b"/></svg>`,
		`<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 24 24"><circle cx="12" cy="12" r="10" fill="none" stroke="#30a878" stroke-width="2"/><path d="M6 12L10 16L18 7" fill="none" stroke="#30a878" stroke-width="2"/></svg>`,
	}
	for _, text := range paths {
		s, err := svg.Parse(strings.NewReader(text))
		if err != nil {
			return err
		}
		d.icons = append(d.icons, s)
	}
	wordmark, err := svg.Parse(strings.NewReader(resource.LogoSVG()))
	if err != nil {
		return err
	}
	d.wordmark = wordmark.WithRawColors()
	d.icons = append(d.icons, &draw.Source{Draw: func(p graphics.Painter, s graphics.Size, c graphics.Color) {
		p.FillEllipse(graphics.Point{X: s.Width / 2, Y: s.Height / 2}, s.Width*.45, s.Height*.45, c)
		p.DrawLine(graphics.Point{X: s.Width * .3, Y: s.Height * .5}, graphics.Point{X: s.Width * .7, Y: s.Height * .5}, max(1, s.Width*.06), graphics.RGBA(255, 255, 255, 230))
	}})
	colored, err := svg.Parse(strings.NewReader(paths[1]))
	if err != nil {
		return err
	}
	d.icons = append(d.icons, colored.WithRawColors())
	// Create this fixed bitmap once and reuse it across Build and Paint calls.
	img := image.NewRGBA(image.Rect(0, 0, 280, 110))
	for y := 0; y < 110; y++ {
		for x := 0; x < 280; x++ {
			img.SetRGBA(x, y, color.RGBA{R: byte(40 + x*150/280), G: byte(70 + y*100/110), B: 180, A: 255})
		}
	}
	d.bitmap = img
	return nil
}
