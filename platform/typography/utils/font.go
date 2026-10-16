package utils

import (
	"fmt"
	"math"
	"unicode/utf8"

	"github.com/golang-gui/goui/platform/typography"
)

// ValidateTabWidth keeps two Pango-unit stops representable in int32.
func ValidateTabWidth(width float32) error {
	if width < 0 || width > 1048575 || math.IsNaN(float64(width)) || math.IsInf(float64(width), 0) {
		return fmt.Errorf("typography: invalid tab width %v", width)
	}
	return nil
}

// ResolveFont validates a request and resolves its platform-independent defaults.
// Family is resolved at the native backend boundary, not here.
func ResolveFont(font typography.FontInfo) (typography.FontInfo, error) {
	if font.Size == 0 {
		font.Size = 12
	}
	if font.Weight == 0 {
		font.Weight = typography.FontWeightRegular
	}
	if font.Width == 0 {
		font.Width = typography.FontWidthNormal
	}
	if font.Size <= 0 || math.IsNaN(float64(font.Size)) || math.IsInf(float64(font.Size), 0) ||
		font.Weight < 100 || font.Weight > 900 || font.Width < 1 || font.Width > 9 || font.Style > typography.FontStyleOblique {
		return font, fmt.Errorf("typography: invalid font request %+v", font)
	}
	return font, nil
}

// CheckTextRange enforces the UTF-8 range precondition before native conversion.
func CheckTextRange(text string, start, length int) {
	if start < 0 || length <= 0 || start > len(text) || length > len(text)-start ||
		(start < len(text) && !utf8.RuneStart(text[start])) ||
		(start+length < len(text) && !utf8.RuneStart(text[start+length])) {
		panic("typography: text range must be nonempty and use UTF-8 rune boundaries")
	}
}

// CheckPlaceholder validates the replacement character and its optional metrics.
func CheckPlaceholder(text string, start int, metrics *typography.PlaceholderMetrics) {
	if start < 0 || start > len(text)-3 || text[start:start+3] != "\uFFFC" {
		panic("typography: placeholder must point to a U+FFFC character")
	}
	if metrics == nil {
		return
	}
	for _, v := range []float32{metrics.Width, metrics.Height, metrics.Baseline} {
		if math.IsNaN(float64(v)) || math.IsInf(float64(v), 0) {
			panic("typography: placeholder metrics must be finite")
		}
	}
	if metrics.Width < 0 || metrics.Height <= 0 || metrics.Baseline < 0 || metrics.Baseline > metrics.Height {
		panic("typography: invalid placeholder dimensions or baseline")
	}
}
