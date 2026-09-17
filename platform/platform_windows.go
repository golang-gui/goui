package platform

import "github.com/golang-gui/goui/platform/windows/win32"

func newPlatform(name, appId string) (Platform, error) {
	if name != "win32" {
		return nil, ErrUnsupported
	}
	p, err := win32.NewPlatform(appId)
	if err != nil {
		return nil, err
	}
	return p, nil
}
