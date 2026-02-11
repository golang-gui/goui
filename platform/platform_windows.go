package platform

import "github.com/golang-gui/goui/platform/win32"

func newPlatform(name string) (Platform, error) {
	if name != "win32" {
		return nil, ErrUnsupported
	}
	return win32.NewPlatform()
}
