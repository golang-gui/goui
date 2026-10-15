package utils

import (
	"github.com/golang-gui/goui/platform/typography"
	"testing"
)

func TestResolveFont(t *testing.T) {
	font, err := ResolveFont(typography.FontInfo{})
	if err != nil || font.Size != 12 || font.Weight != 400 || font.Width != 5 || font.Style != 0 {
		t.Fatalf("defaults: %+v, %v", font, err)
	}
}

func TestCheckTextRange(t *testing.T) {
	CheckTextRange("a中😀", 1, 3)
	for _, pair := range [][2]int{{-1, 1}, {0, 0}, {1, 2}, {2, 2}, {4, 3}, {0, 100}} {
		func() {
			defer func() {
				if recover() == nil {
					t.Errorf("accepted invalid range %v", pair)
				}
			}()
			CheckTextRange("a中😀", pair[0], pair[1])
		}()
	}
}

func TestCheckPlaceholder(t *testing.T) {
	CheckPlaceholder("a\uFFFC", 1, &typography.PlaceholderMetrics{Height: 1})
	CheckPlaceholder("\uFFFC", 0, nil)
	for _, tc := range []struct {
		text    string
		start   int
		metrics typography.PlaceholderMetrics
	}{
		{"abc", 0, typography.PlaceholderMetrics{Height: 1}},
		{"\uFFFC", 1, typography.PlaceholderMetrics{Height: 1}},
		{"\uFFFC", 0, typography.PlaceholderMetrics{Height: 0}},
		{"\uFFFC", 0, typography.PlaceholderMetrics{Width: -1, Height: 1}},
		{"\uFFFC", 0, typography.PlaceholderMetrics{Height: 1, Baseline: 2}},
	} {
		func() {
			defer func() {
				if recover() == nil {
					t.Errorf("invalid placeholder accepted: %+v", tc)
				}
			}()
			CheckPlaceholder(tc.text, tc.start, &tc.metrics)
		}()
	}
}
