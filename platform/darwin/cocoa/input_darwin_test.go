package cocoa

import (
	"testing"

	. "github.com/golang-gui/goui/platform/darwin/frameworks/appkit"
	"github.com/golang-gui/goui/platform/darwin/frameworks/iokit"
	"github.com/golang-gui/goui/platform/events"
)

func TestLogicalMacLetters(t *testing.T) {
	for _, tt := range []struct {
		code uint16
		text string
		want events.Key
	}{
		{0, "q", events.KeyQ}, // 法语布局中，美式 A 的位置对应 Q。
		{12, "a", events.KeyA},
		{12, "A", events.KeyA}, // Shift 不改变按键身份。
		{6, "y", events.KeyY},  // 德语 QWERTZ 布局。
		{16, "z", events.KeyZ},
		{41, "m", events.KeyM},      // 字母位于美式标点键的位置。
		{0, "ф", events.KeyUnknown}, // 不能推测为美式 A。
		{0, "", events.KeyUnknown},
		{0, "ab", events.KeyUnknown},
		{36, "\r", events.KeyEnter},
		{76, "\r", events.KeyNumpadEnter},
	} {
		physical, location := keyFromMacKeyCode(tt.code)
		if got := logicalMacKey(physical, location, tt.text); got != tt.want {
			t.Fatalf("code %d text %q: %v, want %v", tt.code, tt.text, got, tt.want)
		}
	}
}

func TestMacSidedModifierEvents(t *testing.T) {
	for _, tc := range []struct {
		name                           string
		left, right                    uint16
		leftMask, rightMask, aggregate NSEventModifierFlags
		modifier                       events.Modifiers
	}{
		{"Shift", 56, 60, iokit.NX_DEVICELSHIFTKEYMASK, iokit.NX_DEVICERSHIFTKEYMASK, NSEventModifierFlagShift, events.ModifierShift},
		{"Control", 59, 62, iokit.NX_DEVICELCTLKEYMASK, iokit.NX_DEVICERCTLKEYMASK, NSEventModifierFlagControl, events.ModifierControl},
		{"Option", 58, 61, iokit.NX_DEVICELALTKEYMASK, iokit.NX_DEVICERALTKEYMASK, NSEventModifierFlagOption, events.ModifierOption},
		{"Command", 55, 54, iokit.NX_DEVICELCMDKEYMASK, iokit.NX_DEVICERCMDKEYMASK, NSEventModifierFlagCommand, events.ModifierCommand},
	} {
		t.Run(tc.name, func(t *testing.T) {
			for _, reverse := range []bool{false, true} {
				first, other, firstMask, otherMask := tc.left, tc.right, tc.leftMask, tc.rightMask
				if reverse {
					first, other, firstMask, otherMask = other, first, otherMask, firstMask
				}
				for i, s := range []struct {
					code  uint16
					flags NSEventModifierFlags
					typ   events.EventType
					mods  events.Modifiers
				}{
					{first, tc.aggregate | firstMask, events.KeyDown, tc.modifier},
					{other, tc.aggregate | firstMask | otherMask, events.KeyDown, tc.modifier},
					{first, tc.aggregate | otherMask, events.KeyUp, tc.modifier},
					{other, 0, events.KeyUp, 0},
				} {
					// 映射无状态；此前事件由其他窗口或应用接收，本次首次获得事件时也成立。
					got, ok := modifierEventType(s.code, s.flags)
					if !ok || got != s.typ || modifiersFromFlags(s.flags) != s.mods {
						t.Fatalf("reverse=%v step=%d: type=%v ok=%v mods=%v", reverse, i, got, ok, modifiersFromFlags(s.flags))
					}
				}
			}
		})
	}
	if _, ok := modifierEventType(0, NSEventModifierFlagShift); ok {
		t.Fatal("non-modifier flagsChanged was converted to a key event")
	}
}

// 只验证映射，不调用 AppKit，不需要应用或桌面会话。
func TestNativeModifierMapping(t *testing.T) {
	if modifierForKey(events.KeyAlt) != 0 || modifierForKey(events.KeyWin) != 0 || modifierForKey(events.KeySuper) != 0 {
		t.Fatal("macOS must not interpret Alt, Win or Super as native modifiers")
	}
	for _, tt := range []struct {
		flags NSEventModifierFlags
		want  events.Modifiers
	}{
		{NSEventModifierFlagControl, events.ModifierControl},
		{NSEventModifierFlagShift, events.ModifierShift},
		{NSEventModifierFlagOption, events.ModifierOption},
		{NSEventModifierFlagCommand, events.ModifierCommand},
		{NSEventModifierFlagCommand | NSEventModifierFlagOption, events.ModifierCommand | events.ModifierOption},
	} {
		if got := modifiersFromFlags(tt.flags); got != tt.want {
			t.Fatalf("flags %x: got %x, want %x", tt.flags, got, tt.want)
		}
	}
	for _, tt := range []struct {
		code     uint16
		key      events.Key
		location events.KeyLocation
		modifier events.Modifiers
	}{
		{54, events.KeyCommand, events.KeyLocationRight, events.ModifierCommand},
		{55, events.KeyCommand, events.KeyLocationLeft, events.ModifierCommand},
		{58, events.KeyOption, events.KeyLocationLeft, events.ModifierOption},
		{61, events.KeyOption, events.KeyLocationRight, events.ModifierOption},
		{59, events.KeyControl, events.KeyLocationLeft, events.ModifierControl},
		{62, events.KeyControl, events.KeyLocationRight, events.ModifierControl},
	} {
		key, location := keyFromMacKeyCode(tt.code)
		if key != tt.key || location != tt.location || modifierForKey(key) != tt.modifier {
			t.Fatalf("keycode %d: key=%v location=%v modifier=%v", tt.code, key, location, modifierForKey(key))
		}
	}
}
