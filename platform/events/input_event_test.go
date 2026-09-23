package events

import "testing"

func TestKeyDefaultResponse(t *testing.T) {
	e := KeyEvent{Key: KeyS, Modifiers: ModifierControl}
	e.PreventDefault() // synthetic/no response is safe.
	handled := false
	e.Handled = &handled
	copy := e
	copy.PreventDefault()
	if !handled || e.Key != KeyS || e.Modifiers != ModifierControl {
		t.Fatal("response lost or input facts changed")
	}
}

func TestModifierNamesAndDistinctCommand(t *testing.T) {
	if ModifierOption == 0 || ModifierOption&(ModifierShift|ModifierControl|ModifierAlt|ModifierSuper|ModifierCommand|ModifierWin) != 0 || KeyOption == KeyAlt {
		t.Fatal("Option must be distinct from Alt and other modifiers")
	}
	if ModifierCommand&(ModifierShift|ModifierControl|ModifierAlt|ModifierSuper) != 0 || ModifierCommand == 0 {
		t.Fatal("Command must have an independent modifier bit")
	}
	if KeyCommand == KeySuper || KeyCommand == KeyAlt || KeyCommand == KeyControl {
		t.Fatal("Command must be a distinct key")
	}
}

func TestWinIsDistinct(t *testing.T) {
	if ModifierWin == 0 || ModifierWin&(ModifierShift|ModifierControl|ModifierAlt|ModifierSuper|ModifierCommand|ModifierOption) != 0 {
		t.Fatal("Win must have an independent modifier bit")
	}
	for _, key := range []Key{KeyUnknown, KeyShift, KeyControl, KeyAlt, KeySuper, KeyCommand, KeyOption} {
		if KeyWin == key {
			t.Fatalf("Win must be distinct from key %v", key)
		}
	}
}
