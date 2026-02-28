//go:build !windows

package platform

func newPlatform(name string) (Platform, error) {
	return nil, ErrUnsupported
}
