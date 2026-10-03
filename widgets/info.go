package widgets

import "github.com/golang-gui/goui/gui"

// Semantic roles supplied by this widget package. Values remain stable across
// implementation changes; generic GUI snapshot consumers need no tab knowledge.
const (
	RoleTabView   gui.Role = "tabview"
	RoleTabBar    gui.Role = "tabbar"
	RoleTab       gui.Role = "tab"
	RoleTabPanel  gui.Role = "tabpanel"
	RoleSplitView gui.Role = "splitview"
	RoleSeparator gui.Role = "separator"
)
