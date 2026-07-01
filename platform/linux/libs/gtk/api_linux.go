package gtk

import (
	"runtime"

	"github.com/goexlib/cgo"
)

var (
	libgtk = cgo.NewLazyLibrary("libgtk-3.so.0")

	gtkInitCheck               = libgtk.NewSymbol("gtk_init_check")
	gtkSettingsGetDefault      = libgtk.NewSymbol("gtk_settings_get_default")
	gtkStyleContextNew         = libgtk.NewSymbol("gtk_style_context_new")
	gtkStyleContextLookupColor = libgtk.NewSymbol("gtk_style_context_lookup_color")
)

func InitCheck() (bool, error) {
	ret, _, err := gtkInitCheck.CallRaw(0, 0)
	if err != nil {
		return false, err
	}
	return ret != 0, nil
}

func SettingsGetDefault() (settings Settings, err error) {
	ret, _, err := gtkSettingsGetDefault.CallRaw()
	if err != nil {
		return Settings{}, err
	}
	settings.GObject = ret
	return settings, nil
}

func StyleContextNew() (context StyleContext, err error) {
	ret, _, err := gtkStyleContextNew.CallRaw()
	if err != nil {
		return StyleContext{}, err
	}
	context.GObject = ret
	return context, nil
}

func (c StyleContext) LookupColor(name string) (RGBA, bool, error) {
	cName := cgo.CString(name)
	var color RGBA
	ret, _, err := gtkStyleContextLookupColor.CallRaw(c.GObject, uintptr(cName), uintptr(cgo.Pointer(&color)))
	runtime.KeepAlive(cName)
	if err != nil {
		return RGBA{}, false, err
	}
	return color, ret != 0, nil
}
