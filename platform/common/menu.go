package common

// MenuShortcut is a logical key and an exact native modifier combination.
// Equivalents deliver ordinary KeyEvents, not a second menu activation.
// KeyUnknown means no equivalent. Backends translate representable keys without
// merging platform-specific modifiers; unsupported combinations have no equivalent.
type MenuShortcut struct {
	Key       Key
	Modifiers Modifiers
}

// MenuItem is a detached native menu description. IDs are window-local and
// nonzero. Hidden entries are omitted by the caller. Native objects never own
// application model pointers. Children are exclusive with leaf activation.
type MenuItem struct {
	ID                                   uint64
	Title                                string
	Enabled, Separator, Checked, Submenu bool
	Shortcut                             MenuShortcut
	Children                             []MenuItem
}

// WindowMenuBar is an optional capability of the same native Window, with no
// separate lifetime or event stream. SetMenu copies the entire description;
// an empty slice withdraws this window's business menus. Native application
// menus and active-window selection remain backend-owned. Calls are UI-thread
// affine. Errors leave the previous description registered. Structural updates
// during native tracking commit at a safe boundary. Selections arrive through
// the window's existing handler as MenuEvent, after tracking has ended. Keyboard
// equivalents follow the ordinary KeyEvent path so the owner retains input policy.
// IDs identify action bindings, not list indices; callers must allocate a new
// ID when replacing a binding and validate that a selected binding is live.
type WindowMenuBar interface {
	Window
	SetMenu(items []MenuItem) error
}
