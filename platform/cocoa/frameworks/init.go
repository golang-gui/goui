package frameworks

import (
	"github.com/golang-gui/goui/platform/cocoa/frameworks/appkit"
	"github.com/golang-gui/goui/platform/cocoa/frameworks/common"
	"github.com/golang-gui/goui/platform/cocoa/frameworks/foundation"
	"github.com/golang-gui/goui/platform/cocoa/frameworks/objcrt"
)

func Init() (err error) {
	objcrt.Init()
	err = foundation.Init(common.LoadSystemFramework)
	if err != nil {
		return
	}
	err = appkit.Init(common.LoadSystemFramework)
	if err != nil {
		return
	}

	return nil
}
