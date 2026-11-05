package frameworks

import (
	"github.com/golang-gui/goui/platform/darwin/frameworks/foundation"
	"github.com/golang-gui/goui/platform/darwin/frameworks/opengles"
	"github.com/golang-gui/goui/platform/darwin/frameworks/quartzcore"
	"github.com/golang-gui/goui/platform/darwin/frameworks/uikit"
)

var iosInited bool

// InitIOS loads UIKit and the mobile graphics frameworks without AppKit.
func InitIOS() error {
	initMtx.Lock()
	defer initMtx.Unlock()
	if iosInited {
		return nil
	}
	if err := initTypography(); err != nil {
		return err
	}
	for _, init := range []func() error{foundation.InitFoundation, uikit.InitUIKit, opengles.InitOpenGLES, quartzcore.InitQuartzCore} {
		if err := init(); err != nil {
			return err
		}
	}
	iosInited = true
	return nil
}
