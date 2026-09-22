package gui

import (
	"runtime"
	"testing"

	"github.com/golang-gui/goui/platform/events"
)

func TestKeyResolve(t *testing.T) {
	for _, os := range []string{"windows", "linux", "darwin"} {
		for key := KeyEscape; key <= KeyPrimary; key++ {
			want, valid := events.Key(key), true
			switch key {
			case KeyPrimary:
				want = events.KeyControl
				if os == "darwin" {
					want = events.KeyCommand
				}
			case KeyAlt:
				if os == "darwin" {
					want = events.KeyOption
				}
			case KeyWin:
				valid = os == "windows"
			case KeySuper, KeyAltGraph:
				valid = os == "linux"
			case KeyOption, KeyCommand:
				valid = os == "darwin"
			}
			if !valid {
				want = events.KeyUnknown
			}
			if got, ok := key.resolve(os); got != want || ok != valid {
				t.Fatalf("%s key %d: (%v,%v), want (%v,%v)", os, key, got, ok, want, valid)
			}
			if os == runtime.GOOS {
				if got, ok := key.Resolve(); got != want || ok != valid {
					t.Fatalf("public Resolve(%d): %v %v", key, got, ok)
				}
			}
		}
		for _, key := range []Key{KeyUnknown, KeyPrimary + 1, ^Key(0)} {
			if got, ok := key.resolve(os); got != events.KeyUnknown || ok {
				t.Fatalf("invalid %d accepted", key)
			}
		}
	}
	if _, ok := KeyA.resolve("unsupported"); ok {
		t.Fatal("unknown platform accepted")
	}
}

func TestKeyModifiersResolve(t *testing.T) {
	// Enumerate every declaration, including combinations with unsupported
	// modifiers. Expected masks are explicit per-platform facts, not obtained
	// from the production resolver.
	for _, platform := range []struct {
		name    string
		mapping [9]events.Modifiers
	}{
		{"windows", [9]events.Modifiers{events.ModifierShift, events.ModifierControl, events.ModifierAlt, events.ModifierControl, events.ModifierWin, 0, 0, 0, 0}},
		{"linux", [9]events.Modifiers{events.ModifierShift, events.ModifierControl, events.ModifierAlt, events.ModifierControl, 0, events.ModifierSuper, 0, 0, events.ModifierAltGraph}},
		{"darwin", [9]events.Modifiers{events.ModifierShift, events.ModifierControl, events.ModifierOption, events.ModifierCommand, 0, 0, events.ModifierCommand, events.ModifierOption, 0}},
	} {
		t.Run(platform.name, func(t *testing.T) {
			for mask := KeyModifiers(0); mask < 512; mask++ {
				var want events.Modifiers
				valid := true
				for bit, native := range platform.mapping {
					if mask&(1<<bit) != 0 {
						valid = valid && native != 0
						want |= native
					}
				}
				if !valid {
					want = 0
				}
				got, ok := mask.resolve(platform.name)
				if got != want || ok != valid {
					t.Fatalf("mask %#x: (%#x, %v), want (%#x, %v)", mask, got, ok, want, valid)
				}
				if platform.name == runtime.GOOS {
					actual, supported := mask.Resolve()
					if actual != want || supported != valid {
						t.Fatalf("public Resolve mask %#x: (%#x, %v), want (%#x, %v)", mask, actual, supported, want, valid)
					}
				}
			}
			for bit := 9; bit < 16; bit++ {
				if got, ok := (ModPrimary | 1<<bit).resolve(platform.name); got != 0 || ok {
					t.Fatalf("unknown bit %d accepted: (%#x, %v)", bit, got, ok)
				}
			}
		})
	}
	for _, mask := range []KeyModifiers{0, ModPrimary, ModShift} {
		if got, ok := mask.resolve("unsupported"); got != 0 || ok {
			t.Fatalf("unsupported platform accepted: (%#x, %v)", got, ok)
		}
	}
}
