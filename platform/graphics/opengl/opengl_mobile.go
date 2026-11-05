//go:build android || ios

package opengl

import "github.com/golang-gui/goui/platform/common"

// mobileNativeWindow borrows drawing targets without depending on a concrete
// platform window. Context owns its EGL/EAGL resources; Window owns the target.
type mobileNativeWindow interface {
	NativeWindow
	AcquireTarget() (common.NativeTarget, error)
}

func mobileContextSupported(share Context, config Config) bool {
	return share == nil && config.PixelFormat.Samples <= 0 && !config.PixelFormat.Stereo && !config.PixelFormat.SRGB
}
