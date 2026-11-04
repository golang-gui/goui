//go:build android || ios

package mobile

import "github.com/golang-gui/goui/platform/mobile/internal/entry"

func init() { entry.Register(process.start) }
