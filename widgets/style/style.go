// Package style preserves the legacy widget fallback rule entry point.
// Complete fallback styles now live in theme/basic and are applied by GUI
// automatically when the application has no custom stylesheet.
package style

import (
	basestyle "github.com/golang-gui/goui/style"
	"github.com/golang-gui/goui/theme/basic"
)

// Rules returns fresh rules for controls in package widgets.
// Deprecated: use basic.Rules for a complete fallback sheet, or basic.Sheet.
// GUI already provides the complete fallback without explicit composition.
func Rules() []basestyle.Rule { return basic.WidgetRules() }
