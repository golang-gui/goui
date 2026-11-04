package frameworks

import (
	"sync"

	. "github.com/golang-gui/goui/platform/darwin/frameworks/appkit"
	. "github.com/golang-gui/goui/platform/darwin/frameworks/core_foundation"
	. "github.com/golang-gui/goui/platform/darwin/frameworks/core_graphics"
	. "github.com/golang-gui/goui/platform/darwin/frameworks/core_text"
	. "github.com/golang-gui/goui/platform/darwin/frameworks/foundation"
	. "github.com/golang-gui/goui/platform/darwin/frameworks/opengl"
	. "github.com/golang-gui/goui/platform/darwin/frameworks/security"
)

var (
	inited           bool
	typographyInited bool
	initMtx          sync.Mutex
)

// InitTypography loads only CoreFoundation, CoreGraphics and CoreText. It is
// shared by macOS and iOS and does not initialize AppKit or desktop OpenGL.
func InitTypography() error {
	initMtx.Lock()
	defer initMtx.Unlock()
	return initTypography()
}

func initTypography() (err error) {
	if typographyInited {
		return nil
	}
	if err = InitCoreFoundation(); err != nil {
		return err
	}
	if err = InitCoreGraphics(); err != nil {
		return err
	}
	if err = InitCoreText(); err != nil {
		return err
	}
	typographyInited = true
	return nil
}

func Init() (err error) {
	initMtx.Lock()
	defer initMtx.Unlock()
	if inited {
		return nil
	}

	err = initTypography()
	if err != nil {
		return
	}
	err = InitFoundation()
	if err != nil {
		return
	}
	err = InitSecurity()
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
