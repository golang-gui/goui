package frameworks

import (
	"sync"

	. "github.com/golang-gui/goui/platform/darwin/frameworks/appkit"
	. "github.com/golang-gui/goui/platform/darwin/frameworks/core_foundation"
	. "github.com/golang-gui/goui/platform/darwin/frameworks/core_graphics"
	. "github.com/golang-gui/goui/platform/darwin/frameworks/core_text"
	. "github.com/golang-gui/goui/platform/darwin/frameworks/foundation"
	. "github.com/golang-gui/goui/platform/darwin/frameworks/opengl"
)

var (
	inited  bool
	initMtx sync.Mutex
)

func Init() (err error) {
	initMtx.Lock()
	defer initMtx.Unlock()
	if inited {
		return nil
	}

	err = InitCoreFoundation()
	if err != nil {
		return
	}
	err = InitCoreGraphics()
	if err != nil {
		return
	}
	err = InitCoreText()
	if err != nil {
		return
	}
	err = InitFoundation()
	if err != nil {
		return
	}
	err = InitAppKit()
	if err != nil {
		return
	}
	err = InitOpenGL()
	if err != nil {
		return
	}

	inited = true
	return nil
}
