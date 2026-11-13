package gtk

import (
	"runtime"

	"github.com/goexlib/cgo"
)

var (
	libgtk = cgo.NewLazyLibrary("libgtk-3.so.0")

	gtkInitCheck               = libgtk.NewSymbol("gtk_init_check")
	gtkSettingsGetDefault      = libgtk.NewSymbol("gtk_settings_get_default")
	gtkCssProviderGetNamed     = libgtk.NewSymbol("gtk_css_provider_get_named")
	gtkStyleContextAddProvider = libgtk.NewSymbol("gtk_style_context_add_provider")
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
	cName := cgo.CStringTemp(name)
	var color RGBA
	ret, _, err := gtkStyleContextLookupColor.CallRaw(c.GObject, uintptr(cName), uintptr(cgo.Pointer(&color)))
	runtime.KeepAlive(cName)
	if err != nil {
		return RGBA{}, false, err
	}
	return color, ret != 0, nil
}

// CssProviderGetNamed borrows GTK's named theme provider. Do not unref it.
// An empty variant selects the theme's default variant.
func CssProviderGetNamed(name, variant string) (provider CssProvider, err error) {
	cName := cgo.CStringTemp(name)
	var cVariant cgo.Pointer
	if variant != "" {
		cVariant = cgo.CStringTemp(variant)
	}
	ret, _, err := gtkCssProviderGetNamed.CallRaw(uintptr(cName), uintptr(cVariant))
	runtime.KeepAlive(cName)
	runtime.KeepAlive(cVariant)
	provider.GObject = ret
	return provider, err
}

// AddProvider installs a provider on this context only. GTK retains its own
// reference; it is released with the context or when the provider is removed.
func (c StyleContext) AddProvider(provider CssProvider, priority uint32) error {
	_, _, err := gtkStyleContextAddProvider.CallRaw(c.GObject, provider.GObject, uintptr(priority))
	return err
}
