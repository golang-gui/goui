package common

import (
	"errors"
	"image/color"
)

var ErrSettingUnsupported = errors.New("platform setting unsupported")

type ColorScheme int

const (
	ColorSchemeLight ColorScheme = iota
	ColorSchemeDark
)

type Settings interface {
	ColorScheme() (ColorScheme, error)
	AccentColor() (color.Color, error)
	FontFamily() (string, error)
	FontSize() (float32, error)
}
