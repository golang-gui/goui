package modern

import (
	"testing"

	"github.com/golang-gui/goui/style"
)

func TestPlaceholderColors(t *testing.T) {
	for _, dark := range []bool{false, true} {
		sheet := Sheet(Options{Dark: dark})
		for _, name := range []string{"text-input", "text-view"} {
			for _, state := range []style.State{style.Normal, style.Focused, style.Disabled} {
				got, ok := sheet.Resolve(style.Sel{Name: name, Part: "placeholder", State: state}).ForegroundColor()
				want := rgb(0x61616B)
				if dark {
					want = rgb(0xB8B8C2)
				}
				if state == style.Disabled {
					want = rgb(0x97979F)
					if dark {
						want = rgb(0x797983)
					}
				}
				if !ok || rgba(got) != want {
					t.Fatalf("dark=%t %s/%v: hint color %v, want %v", dark, name, state, got, want)
				}
			}
		}
	}
}
