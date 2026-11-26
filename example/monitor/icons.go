package main

import (
	"strings"

	"github.com/golang-gui/goui/icon/svg"
)

// themeIcons are monochrome outlines on a 24 DIP grid. Their color comes from
// the Icon's style, so one parsed source serves both palettes.
type themeIcons struct{ sun, moon *svg.Source }

const iconStroke = `fill="none" stroke="#000" stroke-width="1.8" stroke-linecap="round" stroke-linejoin="round"`

const (
	sunIcon = `<circle cx="12" cy="12" r="4" ` + iconStroke + `/><path d="M12 2.5v2M12 19.5v2M2.5 12h2M19.5 12h2` +
		`M5.3 5.3l1.4 1.4M17.3 17.3l1.4 1.4M5.3 18.7l1.4-1.4M17.3 6.7l1.4-1.4" ` + iconStroke + `/>`
	moonIcon = `<path d="M20 14.5A8 8 0 1 1 9.5 4a6.5 6.5 0 0 0 10.5 10.5z" ` + iconStroke + `/>`
)

func loadThemeIcons() (themeIcons, error) {
	parse := func(body string) (*svg.Source, error) {
		return svg.Parse(strings.NewReader(`<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 24 24">` + body + `</svg>`))
	}
	sun, err := parse(sunIcon)
	if err != nil {
		return themeIcons{}, err
	}
	moon, err := parse(moonIcon)
	if err != nil {
		return themeIcons{}, err
	}
	return themeIcons{sun: sun, moon: moon}, nil
}
