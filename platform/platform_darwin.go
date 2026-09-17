package platform

import "github.com/golang-gui/goui/platform/darwin/cocoa"

func newPlatform(name, appId string) (Platform, error) {
	if name != "cocoa" {
		return nil, ErrUnsupported
	}
	p, err := cocoa.NewPlatform(appId)
	if err != nil {
		return nil, err
	}
	return p, nil
}
