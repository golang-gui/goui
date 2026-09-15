//go:build !windows && !linux && !darwin

package platform

func newPlatform(name, appId string) (Platform, error) {
	return nil, ErrUnsupported
}
