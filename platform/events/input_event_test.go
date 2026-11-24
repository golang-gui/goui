package events

import (
	"testing"

	"github.com/golang-gui/goui/core/geometry"
	"github.com/golang-gui/goui/platform/common"
)

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

func TestFocusEventContract(t *testing.T) {
	event := FocusEvent{Focused: true}
	if got := event.Type(); got != Focus {
		t.Fatalf("FocusEvent.Type() = %v, want %v", got, Focus)
	}
	if !event.Focused {
		t.Fatal("FocusEvent.Focused = false")
	}
}

func TestKeyEventContract(t *testing.T) {
	event := KeyEvent{
		EventType: KeyDown,
		Key:       KeyA,
		Code:      KeyCodeUnknown,
		Location:  KeyLocationStandard,
		Modifiers: ModifierShift | ModifierControl,
		Repeat:    true,
	}

	if got := event.Type(); got != KeyDown {
		t.Fatalf("KeyEvent.Type() = %v, want %v", got, KeyDown)
	}
	if event.Key != KeyA {
		t.Fatalf("KeyEvent.Key = %v", event.Key)
	}
	if event.Code != KeyCodeUnknown {
		t.Fatalf("KeyEvent.Code = %v", event.Code)
	}
	if event.Location != KeyLocationStandard {
		t.Fatalf("KeyEvent.Location = %v", event.Location)
	}
	if event.Modifiers != ModifierShift|ModifierControl {
		t.Fatalf("KeyEvent.Modifiers = %v", event.Modifiers)
	}
	if !event.Repeat {
		t.Fatal("KeyEvent.Repeat = false")
	}
}

func TestPointerEventContract(t *testing.T) {
	for _, typ := range []EventType{
		PointerEnter,
		PointerLeave,
		PointerMove,
		PointerDown,
		PointerUp,
	} {
		event := PointerEvent{
			EventType: typ,
			Position:  geometry.Point{X: 12, Y: 34},
			Button:    PointerButtonLeft,
			Buttons:   PointerButtonLeftDown | PointerButtonRightDown,
			Modifiers: ModifierShift | ModifierControl,
		}

		if got := event.Type(); got != typ {
			t.Fatalf("PointerEvent.Type() = %v, want %v", got, typ)
		}
		if event.Position != (geometry.Point{X: 12, Y: 34}) {
			t.Fatalf("PointerEvent.Position = %+v", event.Position)
		}
		if event.Button != PointerButtonLeft {
			t.Fatalf("PointerEvent.Button = %v", event.Button)
		}
		if event.Buttons != PointerButtonLeftDown|PointerButtonRightDown {
			t.Fatalf("PointerEvent.Buttons = %v", event.Buttons)
		}
		if event.Modifiers != ModifierShift|ModifierControl {
			t.Fatalf("PointerEvent.Modifiers = %v", event.Modifiers)
		}
	}
}

func TestWheelEventContract(t *testing.T) {
	event := WheelEvent{
		Position:  geometry.Point{X: 12, Y: 34},
		DeltaX:    1.5,
		DeltaY:    -2.25,
		Mode:      WheelDeltaPixel,
		Buttons:   PointerButtonLeftDown | PointerButtonRightDown,
		Modifiers: ModifierShift | ModifierControl,
	}

	if got := event.Type(); got != Wheel {
		t.Fatalf("WheelEvent.Type() = %v, want %v", got, Wheel)
	}
	if event.Position != (geometry.Point{X: 12, Y: 34}) {
		t.Fatalf("WheelEvent.Position = %+v", event.Position)
	}
	if event.DeltaX != 1.5 || event.DeltaY != -2.25 {
		t.Fatalf("WheelEvent delta = (%v, %v)", event.DeltaX, event.DeltaY)
	}
	if event.Mode != WheelDeltaPixel {
		t.Fatalf("WheelEvent.Mode = %v", event.Mode)
	}
	if event.Buttons != PointerButtonLeftDown|PointerButtonRightDown {
		t.Fatalf("WheelEvent.Buttons = %v", event.Buttons)
	}
	if event.Modifiers != ModifierShift|ModifierControl {
		t.Fatalf("WheelEvent.Modifiers = %v", event.Modifiers)
	}
}

func TestInputCompatibilityAliases(t *testing.T) {
	// 回调签名、map 键类型和指针必须直接互通，不能只是底层数值相同。
	var labels map[common.Key]string = map[Key]string{KeyS: "save"}
	var receive func(common.Key, common.Modifiers) = func(key Key, modifiers Modifiers) {
		if labels[key] != "save" || modifiers != ModifierControl|ModifierShift {
			t.Fatal("旧入口声明不能直接消费 common 输入值")
		}
	}
	event := KeyEvent{Key: KeyS, Modifiers: ModifierControl | ModifierShift}
	receive(event.Key, event.Modifiers)
	var key *common.Key = &event.Key
	var modifiers *common.Modifiers = &event.Modifiers
	*key, *modifiers = KeyF24, ModifierCommand|ModifierOption
	if event.Key != common.KeyF24 || event.Modifiers != common.ModifierCommand|common.ModifierOption {
		t.Fatal("兼容入口与共享类型的值不一致")
	}
}
