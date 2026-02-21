//go:build !windows && !linux

package platform

func newPlatform(name string) (Platform, error) {
	return nil, ErrUnsupported
}
