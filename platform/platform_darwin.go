package platform

import "github.com/golang-gui/goui/platform/cocoa"

func newPlatform(name string) (Platform, error) {
	if name != "cocoa" {
		return nil, ErrUnsupported
	}
	return cocoa.NewPlatform()
}
