package frameworks

import (
	"github.com/golang-gui/goui/platform/cocoa/frameworks/appkit"
	"github.com/golang-gui/goui/platform/cocoa/frameworks/common"
	"github.com/golang-gui/goui/platform/cocoa/frameworks/core_foundation"
	"github.com/golang-gui/goui/platform/cocoa/frameworks/core_graphics"
	"github.com/golang-gui/goui/platform/cocoa/frameworks/foundation"
)

func Init() (err error) {
	err = core_foundation.Init(common.LoadSystemFramework)
	if err != nil {
		return
	}
	err = core_graphics.Init(common.LoadSystemFramework)
	if err != nil {
		return
	}
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
