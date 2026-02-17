package common

import (
	"image"
	"image/color"
)

type Image interface {
	image.Image
	Set(x, y int, c color.Color)
}
