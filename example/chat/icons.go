package main

import (
	"strings"

	"github.com/golang-gui/goui/icon/svg"
)

// icons are monochrome outlines on a 24 DIP grid. Their color comes from the
// Icon's style, so one parsed source serves both palettes.
type icons struct {
	logo, chats, contacts, starred *svg.Source
	sun, moon                      *svg.Source
	phone, video, attach, emoji    *svg.Source
	send                           *svg.Source
}

const iconStroke = `fill="none" stroke="#000" stroke-width="1.8" stroke-linecap="round" stroke-linejoin="round"`

var iconPaths = map[string]string{
	"logo":     `<path d="M6 4h12a3 3 0 0 1 3 3v7a3 3 0 0 1-3 3h-7l-5 4v-4a3 3 0 0 1-3-3V7a3 3 0 0 1 3-3z" fill="#000"/>`,
	"chats":    `<path d="M6 4h12a3 3 0 0 1 3 3v7a3 3 0 0 1-3 3h-7l-5 4v-4a3 3 0 0 1-3-3V7a3 3 0 0 1 3-3z" ` + iconStroke + `/>`,
	"contacts": `<circle cx="12" cy="8" r="4" ` + iconStroke + `/><path d="M4 20c0-4 3.6-6 8-6s8 2 8 6" ` + iconStroke + `/>`,
	"starred":  `<path d="M12 3.5l2.6 5.3 5.9.9-4.3 4.1 1 5.8L12 16.9l-5.2 2.7 1-5.8-4.3-4.1 5.9-.9z" ` + iconStroke + `/>`,
	"sun": `<circle cx="12" cy="12" r="4" ` + iconStroke + `/><path d="M12 2.5v2M12 19.5v2M2.5 12h2M19.5 12h2` +
		`M5.3 5.3l1.4 1.4M17.3 17.3l1.4 1.4M5.3 18.7l1.4-1.4M17.3 6.7l1.4-1.4" ` + iconStroke + `/>`,
	"moon":   `<path d="M20 14.5A8 8 0 1 1 9.5 4a6.5 6.5 0 0 0 10.5 10.5z" ` + iconStroke + `/>`,
	"phone":  `<path d="M5 4h3l2 5-2.5 1.5a11 11 0 0 0 6 6L15 14l5 2v3a2 2 0 0 1-2 2A16 16 0 0 1 3 6a2 2 0 0 1 2-2z" ` + iconStroke + `/>`,
	"video":  `<rect x="3" y="6" width="12" height="12" rx="2.5" ` + iconStroke + `/><path d="M15 10.5l6-3.5v10l-6-3.5z" ` + iconStroke + `/>`,
	"attach": `<path d="M20.5 11.5l-8.2 8.2a5 5 0 0 1-7-7l8.5-8.5a3.4 3.4 0 0 1 4.8 4.8l-8.4 8.4a1.7 1.7 0 0 1-2.4-2.4l7.7-7.7" ` + iconStroke + `/>`,
	"emoji": `<circle cx="12" cy="12" r="9" ` + iconStroke + `/><path d="M8 14.5a5 5 0 0 0 8 0" ` + iconStroke + `/>` +
		`<circle cx="9" cy="9.5" r="1.2" fill="#000"/><circle cx="15" cy="9.5" r="1.2" fill="#000"/>`,
	"send": `<path d="M12 19V5M5.5 11.5L12 5l6.5 6.5" fill="none" stroke="#000" stroke-width="2.2" stroke-linecap="round" stroke-linejoin="round"/>`,
}

func loadIcons() (*icons, error) {
	parse := func(name string) (*svg.Source, error) {
		return svg.Parse(strings.NewReader(`<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 24 24">` + iconPaths[name] + `</svg>`))
	}
	var i icons
	for _, entry := range []struct {
		name   string
		target **svg.Source
	}{
		{"logo", &i.logo}, {"chats", &i.chats}, {"contacts", &i.contacts}, {"starred", &i.starred},
		{"sun", &i.sun}, {"moon", &i.moon}, {"phone", &i.phone}, {"video", &i.video},
		{"attach", &i.attach}, {"emoji", &i.emoji}, {"send", &i.send},
	} {
		source, err := parse(entry.name)
		if err != nil {
			return nil, err
		}
		*entry.target = source
	}
	return &i, nil
}
