package test

import (
	"errors"
	"image/color"
	"math"
	"testing"

	"github.com/golang-gui/goui/platform"
	"github.com/golang-gui/goui/platform/common"
)

func TestSettings(t *testing.T) {
	skipWithoutDisplay(t)

	plat, err := getPlatform()
	if err != nil {
		t.Fatal(err)
	}

	var settings platform.Settings
	runOnMainThread(func() {
		settings, err = plat.NewSettings()
	})
	if err != nil {
		t.Fatal(err)
	}
	if settings == nil {
		t.Fatal("NewSettings returned nil")
	}

	supported := 0

	t.Run("ColorScheme", func(t *testing.T) {
		var scheme platform.ColorScheme
		runOnMainThread(func() {
			scheme, err = settings.ColorScheme()
		})
		if !settingSupported(t, "ColorScheme", err) {
			return
		}
		supported++
		if scheme != platform.ColorSchemeLight && scheme != platform.ColorSchemeDark {
			t.Fatalf("unexpected color scheme: %d", scheme)
		}
		t.Logf("ColorScheme: %d", scheme)
	})

	t.Run("AccentColor", func(t *testing.T) {
		var accent color.Color
		runOnMainThread(func() {
			accent, err = settings.AccentColor()
		})
		if !settingSupported(t, "AccentColor", err) {
			return
		}
		supported++
		if accent == nil {
			t.Fatal("AccentColor returned nil")
		}
		r, g, b, a := accent.RGBA()
		if a == 0 {
			t.Fatalf("AccentColor returned transparent color: rgba16(%d, %d, %d, %d)", r, g, b, a)
		}
		t.Logf("AccentColor: rgba16(%d, %d, %d, %d)", r, g, b, a)
	})

	t.Run("FontFamily", func(t *testing.T) {
		var family string
		runOnMainThread(func() {
			family, err = settings.FontFamily()
		})
		if !settingSupported(t, "FontFamily", err) {
			return
		}
		supported++
		if family == "" {
			t.Fatal("FontFamily returned empty string")
		}
		t.Logf("FontFamily: %q", family)
	})

	t.Run("FontSize", func(t *testing.T) {
		var size float32
		runOnMainThread(func() {
			size, err = settings.FontSize()
		})
		if !settingSupported(t, "FontSize", err) {
			return
		}
		supported++
		if size <= 0 || math.IsNaN(float64(size)) || math.IsInf(float64(size), 0) {
			t.Fatalf("unexpected font size: %v", size)
		}
		t.Logf("FontSize: %v", size)
	})

	if supported == 0 {
		t.Fatal("all platform settings are unsupported")
	}
}

func settingSupported(t *testing.T, name string, err error) bool {
	t.Helper()
	if err == nil {
		return true
	}
	if errors.Is(err, common.ErrSettingUnsupported) {
		t.Logf("%s unsupported: %v", name, err)
		return false
	}
	t.Fatalf("%s failed: %v", name, err)
	return false
}
