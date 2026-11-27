package main

import (
	"strings"

	"github.com/golang-gui/goui/icon/svg"
)

// icons are monochrome shapes on a 24 DIP grid; their color comes from the
// Icon's style.
type icons struct {
	play, pause, previous, next *svg.Source
	heart, heartFilled          *svg.Source
}

const heartPath = `M12 20s-7.5-4.6-7.5-10.1A4.4 4.4 0 0 1 12 7.3a4.4 4.4 0 0 1 7.5 2.6C19.5 15.4 12 20 12 20z`

var iconPaths = map[string]string{
	"play":  `<path d="M8 5.5v13a1 1 0 0 0 1.5.86l10.5-6.5a1 1 0 0 0 0-1.72L9.5 4.64A1 1 0 0 0 8 5.5z" fill="#000"/>`,
	"pause": `<rect x="6.5" y="5" width="4" height="14" rx="1.2" fill="#000"/><rect x="13.5" y="5" width="4" height="14" rx="1.2" fill="#000"/>`,
	"previous": `<path d="M19 6.2v11.6a.9.9 0 0 1-1.4.75L9 12.75a.9.9 0 0 1 0-1.5l8.6-5.8A.9.9 0 0 1 19 6.2z" fill="#000"/>` +
		`<rect x="5" y="5.5" width="2.5" height="13" rx="1.2" fill="#000"/>`,
	"next": `<path d="M5 6.2v11.6a.9.9 0 0 0 1.4.75l8.6-5.8a.9.9 0 0 0 0-1.5L6.4 5.45A.9.9 0 0 0 5 6.2z" fill="#000"/>` +
		`<rect x="16.5" y="5.5" width="2.5" height="13" rx="1.2" fill="#000"/>`,
	"heart":       `<path d="` + heartPath + `" fill="none" stroke="#000" stroke-width="1.8" stroke-linejoin="round"/>`,
	"heartFilled": `<path d="` + heartPath + `" fill="#000"/>`,
}

func loadIcons() (*icons, error) {
	var i icons
	for _, entry := range []struct {
		name   string
		target **svg.Source
	}{
		{"play", &i.play}, {"pause", &i.pause}, {"previous", &i.previous}, {"next", &i.next},
		{"heart", &i.heart}, {"heartFilled", &i.heartFilled},
	} {
		source, err := svg.Parse(strings.NewReader(`<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 24 24">` +
			iconPaths[entry.name] + `</svg>`))
		if err != nil {
			return nil, err
		}
		*entry.target = source
	}
	return &i, nil
}
