//go:build !windows && !linux && !darwin

package platform

func newPlatform(name string) (Platform, error) {
	return nil, ErrUnsupported
}
