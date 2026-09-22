package gui

import (
	"runtime"

	"github.com/golang-gui/goui/platform/events"
)

// KeyModifiers describes GUI shortcut modifiers, not raw platform input.
// Resolve translates a declaration without modifying the original event.
type KeyModifiers uint16

const (
	ModShift    KeyModifiers = 1 << iota
	ModControl               // Control on every supported desktop platform.
	ModAlt                   // Alt on Windows/Linux; Option on macOS.
	ModPrimary               // Control on Windows/Linux; Command on macOS.
	ModWin                   // Windows only.
	ModSuper                 // Linux only.
	ModCommand               // macOS only.
	ModOption                // macOS only.
	ModAltGraph              // Linux/X11 Level3 only.
)

// Resolve returns the actual modifier mask for the current platform.
// Unsupported platform-specific modifiers or unknown bits return (0, false);
// callers must not match an unsuccessful resolution as an unmodified key.
// Equivalent modifiers merge: for example Primary|Control is Control on
// Windows/Linux but Command|Control on macOS. Zero resolves successfully on
// supported platforms. This only translates modifiers; it does not register
// a shortcut or guarantee delivery of system-reserved key combinations.
func (m KeyModifiers) Resolve() (events.Modifiers, bool) {
	return m.resolve(runtime.GOOS)
}

func (m KeyModifiers) resolve(goos string) (events.Modifiers, bool) {
	const common = ModShift | ModControl | ModAlt | ModPrimary
	var supported KeyModifiers
	var alt, primary events.Modifiers
	switch goos {
	case "windows":
		supported, alt, primary = common|ModWin, events.ModifierAlt, events.ModifierControl
	case "linux":
		supported, alt, primary = common|ModSuper|ModAltGraph, events.ModifierAlt, events.ModifierControl
	case "darwin":
		supported, alt, primary = common|ModCommand|ModOption, events.ModifierOption, events.ModifierCommand
	default:
		return 0, false
	}
	if m&^supported != 0 {
		return 0, false
	}
	var result events.Modifiers
	for _, entry := range [...]struct {
		semantic KeyModifiers
		native   events.Modifiers
	}{
		{ModShift, events.ModifierShift},
		{ModControl, events.ModifierControl},
		{ModAlt, alt},
		{ModPrimary, primary},
		{ModWin, events.ModifierWin},
		{ModSuper, events.ModifierSuper},
		{ModCommand, events.ModifierCommand},
		{ModOption, events.ModifierOption},
		{ModAltGraph, events.ModifierAltGraph},
	} {
		if m&entry.semantic != 0 {
			result |= entry.native
		}
	}
	return result, true
}

// Key is a GUI key declaration. It is independent of the raw event type.
// Ordinary keys retain their identity; Alt and Primary resolve by platform.
type Key uint32

const (
	KeyUnknown        Key = Key(events.KeyUnknown)
	KeyEscape         Key = Key(events.KeyEscape)
	KeyEnter          Key = Key(events.KeyEnter)
	KeyTab            Key = Key(events.KeyTab)
	KeyBackspace      Key = Key(events.KeyBackspace)
	KeyDelete         Key = Key(events.KeyDelete)
	KeyInsert         Key = Key(events.KeyInsert)
	KeySpace          Key = Key(events.KeySpace)
	KeyArrowLeft      Key = Key(events.KeyArrowLeft)
	KeyArrowRight     Key = Key(events.KeyArrowRight)
	KeyArrowUp        Key = Key(events.KeyArrowUp)
	KeyArrowDown      Key = Key(events.KeyArrowDown)
	KeyHome           Key = Key(events.KeyHome)
	KeyEnd            Key = Key(events.KeyEnd)
	KeyPageUp         Key = Key(events.KeyPageUp)
	KeyPageDown       Key = Key(events.KeyPageDown)
	KeyShift          Key = Key(events.KeyShift)
	KeyControl        Key = Key(events.KeyControl)
	KeyAlt            Key = Key(events.KeyAlt)
	KeySuper          Key = Key(events.KeySuper)
	KeyCapsLock       Key = Key(events.KeyCapsLock)
	KeyNumLock        Key = Key(events.KeyNumLock)
	KeyPrintScreen    Key = Key(events.KeyPrintScreen)
	KeyScrollLock     Key = Key(events.KeyScrollLock)
	KeyPause          Key = Key(events.KeyPause)
	KeyF1             Key = Key(events.KeyF1)
	KeyF2             Key = Key(events.KeyF2)
	KeyF3             Key = Key(events.KeyF3)
	KeyF4             Key = Key(events.KeyF4)
	KeyF5             Key = Key(events.KeyF5)
	KeyF6             Key = Key(events.KeyF6)
	KeyF7             Key = Key(events.KeyF7)
	KeyF8             Key = Key(events.KeyF8)
	KeyF9             Key = Key(events.KeyF9)
	KeyF10            Key = Key(events.KeyF10)
	KeyF11            Key = Key(events.KeyF11)
	KeyF12            Key = Key(events.KeyF12)
	KeyF13            Key = Key(events.KeyF13)
	KeyF14            Key = Key(events.KeyF14)
	KeyF15            Key = Key(events.KeyF15)
	KeyF16            Key = Key(events.KeyF16)
	KeyF17            Key = Key(events.KeyF17)
	KeyF18            Key = Key(events.KeyF18)
	KeyF19            Key = Key(events.KeyF19)
	KeyF20            Key = Key(events.KeyF20)
	KeyF21            Key = Key(events.KeyF21)
	KeyF22            Key = Key(events.KeyF22)
	KeyF23            Key = Key(events.KeyF23)
	KeyF24            Key = Key(events.KeyF24)
	KeyA              Key = Key(events.KeyA)
	KeyB              Key = Key(events.KeyB)
	KeyC              Key = Key(events.KeyC)
	KeyD              Key = Key(events.KeyD)
	KeyE              Key = Key(events.KeyE)
	KeyF              Key = Key(events.KeyF)
	KeyG              Key = Key(events.KeyG)
	KeyH              Key = Key(events.KeyH)
	KeyI              Key = Key(events.KeyI)
	KeyJ              Key = Key(events.KeyJ)
	KeyK              Key = Key(events.KeyK)
	KeyL              Key = Key(events.KeyL)
	KeyM              Key = Key(events.KeyM)
	KeyN              Key = Key(events.KeyN)
	KeyO              Key = Key(events.KeyO)
	KeyP              Key = Key(events.KeyP)
	KeyQ              Key = Key(events.KeyQ)
	KeyR              Key = Key(events.KeyR)
	KeyS              Key = Key(events.KeyS)
	KeyT              Key = Key(events.KeyT)
	KeyU              Key = Key(events.KeyU)
	KeyV              Key = Key(events.KeyV)
	KeyW              Key = Key(events.KeyW)
	KeyX              Key = Key(events.KeyX)
	KeyY              Key = Key(events.KeyY)
	KeyZ              Key = Key(events.KeyZ)
	Key0              Key = Key(events.Key0)
	Key1              Key = Key(events.Key1)
	Key2              Key = Key(events.Key2)
	Key3              Key = Key(events.Key3)
	Key4              Key = Key(events.Key4)
	Key5              Key = Key(events.Key5)
	Key6              Key = Key(events.Key6)
	Key7              Key = Key(events.Key7)
	Key8              Key = Key(events.Key8)
	Key9              Key = Key(events.Key9)
	KeyMinus          Key = Key(events.KeyMinus)
	KeyEqual          Key = Key(events.KeyEqual)
	KeyBracketLeft    Key = Key(events.KeyBracketLeft)
	KeyBracketRight   Key = Key(events.KeyBracketRight)
	KeyBackslash      Key = Key(events.KeyBackslash)
	KeySemicolon      Key = Key(events.KeySemicolon)
	KeyQuote          Key = Key(events.KeyQuote)
	KeyComma          Key = Key(events.KeyComma)
	KeyPeriod         Key = Key(events.KeyPeriod)
	KeySlash          Key = Key(events.KeySlash)
	KeyBackquote      Key = Key(events.KeyBackquote)
	KeyNumpad0        Key = Key(events.KeyNumpad0)
	KeyNumpad1        Key = Key(events.KeyNumpad1)
	KeyNumpad2        Key = Key(events.KeyNumpad2)
	KeyNumpad3        Key = Key(events.KeyNumpad3)
	KeyNumpad4        Key = Key(events.KeyNumpad4)
	KeyNumpad5        Key = Key(events.KeyNumpad5)
	KeyNumpad6        Key = Key(events.KeyNumpad6)
	KeyNumpad7        Key = Key(events.KeyNumpad7)
	KeyNumpad8        Key = Key(events.KeyNumpad8)
	KeyNumpad9        Key = Key(events.KeyNumpad9)
	KeyNumpadAdd      Key = Key(events.KeyNumpadAdd)
	KeyNumpadSubtract Key = Key(events.KeyNumpadSubtract)
	KeyNumpadMultiply Key = Key(events.KeyNumpadMultiply)
	KeyNumpadDivide   Key = Key(events.KeyNumpadDivide)
	KeyNumpadDecimal  Key = Key(events.KeyNumpadDecimal)
	KeyNumpadEnter    Key = Key(events.KeyNumpadEnter)
	KeyCommand        Key = Key(events.KeyCommand)
	KeyWin            Key = Key(events.KeyWin)
	KeyOption         Key = Key(events.KeyOption)
	KeyAltGraph       Key = Key(events.KeyAltGraph)
	KeyPrimary        Key = KeyAltGraph + 1
)

// Resolve translates a key declaration for the current desktop platform.
// Unknown keys and platform-specific keys on other platforms return false.
func (k Key) Resolve() (events.Key, bool) { return k.resolve(runtime.GOOS) }

func (k Key) resolve(goos string) (events.Key, bool) {
	if goos != "windows" && goos != "linux" && goos != "darwin" {
		return events.KeyUnknown, false
	}
	switch k {
	case KeyUnknown:
		return events.KeyUnknown, false
	case KeyPrimary:
		if goos == "darwin" {
			return events.KeyCommand, true
		}
		return events.KeyControl, true
	case KeyAlt:
		if goos == "darwin" {
			return events.KeyOption, true
		}
		return events.KeyAlt, true
	case KeyWin:
		if goos != "windows" {
			return events.KeyUnknown, false
		}
	case KeySuper, KeyAltGraph:
		if goos != "linux" {
			return events.KeyUnknown, false
		}
	case KeyCommand, KeyOption:
		if goos != "darwin" {
			return events.KeyUnknown, false
		}
	default:
		if k > KeyAltGraph {
			return events.KeyUnknown, false
		}
	}
	return events.Key(k), true
}
