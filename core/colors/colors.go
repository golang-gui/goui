// Package colors provides small, dependency-free helpers for working with
// image/color values: equality, and later standard color constants and hex
// parsing. It stays platform-agnostic so any layer can depend on it.
package colors

import "image/color"

// Equal reports whether two colors describe the same color. It compares the
// premultiplied RGBA components so different concrete color types that resolve
// to the same color are equal. Two nil colors are equal; a nil and a non-nil
// color are not.
func Equal(a, b color.Color) bool {
	if a == nil || b == nil {
		return a == b
	}
	ar, ag, ab, aa := a.RGBA()
	br, bg, bb, ba := b.RGBA()
	return ar == br && ag == bg && ab == bb && aa == ba
}
