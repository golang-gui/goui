// Package mobile contains application registration used by mobile build tools.
package mobile

import "github.com/golang-gui/goui/platform/internal/mobileentry"

// RegisterMain registers the ordinary application main through a generated
// init function. Native platform entry starts it once on the GOUI thread.
func RegisterMain(main func()) { mobileentry.RegisterMain(main) }
