package main

import (
	"errors"
	"fmt"
	"image/color"
	"math"

	"github.com/golang-gui/goui/platform"
	"github.com/golang-gui/goui/platform/common"
)

func checkSettings(plat platform.Platform) {
	var err error

	var settings platform.Settings
	func() {
		settings, err = plat.NewSettings()
	}()
	if err != nil {
		panic(err)
	}
	if settings == nil {
		panic("NewSettings returned nil")
	}

	supported := 0

	func() {
		fmt.Println("CHECK", "ColorScheme")
		var scheme platform.ColorScheme
		func() {
			scheme, err = settings.ColorScheme()
		}()
		if !settingSupported("ColorScheme", err) {
			return
		}
		supported++
		if scheme != platform.ColorSchemeLight && scheme != platform.ColorSchemeDark {
			panic(fmt.Errorf("unexpected color scheme: %d", scheme))
		}
		fmt.Printf("ColorScheme: %d\n", scheme)
	}()

	func() {
		fmt.Println("CHECK", "AccentColor")
		var accent color.Color
		func() {
			accent, err = settings.AccentColor()
		}()
		if !settingSupported("AccentColor", err) {
			return
		}
		supported++
		if accent == nil {
			panic("AccentColor returned nil")
		}
		r, g, b, a := accent.RGBA()
		if a == 0 {
			panic(fmt.Errorf("AccentColor returned transparent color: rgba16(%d, %d, %d, %d)", r, g, b, a))
		}
		fmt.Printf("AccentColor: rgba16(%d, %d, %d, %d)\n", r, g, b, a)
	}()

	func() {
		fmt.Println("CHECK", "FontFamily")
		var family string
		func() {
			family, err = settings.FontFamily()
		}()
		if !settingSupported("FontFamily", err) {
			return
		}
		supported++
		if family == "" {
			panic("FontFamily returned empty string")
		}
		fmt.Printf("FontFamily: %q\n", family)
	}()

	func() {
		fmt.Println("CHECK", "FontSize")
		var size float32
		func() {
			size, err = settings.FontSize()
		}()
		if !settingSupported("FontSize", err) {
			return
		}
		supported++
		if size <= 0 || math.IsNaN(float64(size)) || math.IsInf(float64(size), 0) {
			panic(fmt.Errorf("unexpected font size: %v", size))
		}
		fmt.Printf("FontSize: %v\n", size)
	}()

	if supported == 0 {
		panic("all platform settings are unsupported")
	}
}

func settingSupported(name string, err error) bool {
	func() {
	}()
	if err == nil {
		return true
	}
	if errors.Is(err, common.ErrSettingUnsupported) {
		fmt.Printf("%s unsupported: %v\n", name, err)
		return false
	}
	panic(fmt.Errorf("%s failed: %v", name, err))
	return false
}
