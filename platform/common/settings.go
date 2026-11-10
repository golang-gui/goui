package common

import (
	"errors"
	"image/color"
)

type ColorScheme int

const (
	ColorSchemeLight ColorScheme = iota
	ColorSchemeDark
)

func (c ColorScheme) String() string {
	switch c {
	case ColorSchemeLight:
		return "light"
	case ColorSchemeDark:
		return "dark"
	default:
		return "unknown"
	}
}

type Settings interface {
	// Composited queries the current environment's ability to composite surface
	// alpha on the UI thread. False with no error means composition is absent;
	// failures are returned separately. True does not guarantee that a particular
	// window or painter supports transparency.
	Composited() (bool, error)
	// Language returns the user's first preferred UI language as a BCP 47 tag,
	// not regional formatting preferences. A neutral locale is "und".
	Language() (string, error)
	ColorScheme() (ColorScheme, error)
	AccentColor() (color.Color, error)
	FontFamily() (string, error)
	FontSize() (float32, error)
}

var ErrSettingUnsupported = errors.New("platform setting unsupported")
