package main

import (
	"testing"

	"github.com/golang-gui/goui/platform/events"
)

func TestPlatformKeyNames(t *testing.T) {
	for _, tt := range []struct {
		key      events.Key
		modifier events.Modifiers
		name     string
	}{
		{events.KeyWin, events.ModifierWin, "Win"},
		{events.KeySuper, events.ModifierSuper, "Super"},
		{events.KeyCommand, events.ModifierCommand, "Command"},
		{events.KeyOption, events.ModifierOption, "Option"},
		{events.KeyAltGraph, events.ModifierAltGraph, "AltGraph"},
		{events.KeyAlt, events.ModifierAlt, "Alt"},
	} {
		if keyName(tt.key) != tt.name || modifiersName(tt.modifier) != tt.name {
			t.Fatalf("%s: key=%s modifier=%s", tt.name, keyName(tt.key), modifiersName(tt.modifier))
		}
	}
}
