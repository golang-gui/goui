package platform

import "github.com/golang-gui/goui/platform/android"

func newPlatform(name, appId string) (Platform, error) {
	if name != "android" {
		return nil, ErrUnsupported
	}
	p, err := android.NewPlatform(appId)
	if err != nil {
		return nil, err
	}
	return p, nil
}
