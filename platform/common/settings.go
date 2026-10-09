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
	// Language returns the user's first preferred UI language as a BCP 47 tag,
	// not regional formatting preferences. A neutral locale is "und".
	Language() (string, error)
	ColorScheme() (ColorScheme, error)
	AccentColor() (color.Color, error)
	FontFamily() (string, error)
	FontSize() (float32, error)
}

var ErrSettingUnsupported = errors.New("platform setting unsupported")
