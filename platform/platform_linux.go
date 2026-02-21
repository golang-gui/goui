package platform

import "github.com/golang-gui/goui/platform/x11"

func newPlatform(name string) (Platform, error) {
	if name != "x11" {
		return nil, ErrUnsupported
	}
	return x11.NewPlatform()
}
