package platform

import "github.com/golang-gui/goui/platform/darwin/ios"

func newPlatform(name, appID string) (Platform, error) {
	if name != "ios" {
		return nil, ErrUnsupported
	}
	p, err := ios.NewPlatform(appID)
	if err != nil {
		return nil, err
	}
	return p, nil
}
