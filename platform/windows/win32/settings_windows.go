package win32

import (
	"errors"
	"fmt"
	"image/color"
	"math"
	"syscall"
	"unsafe"

	"github.com/golang-gui/goui/platform/common"
	"github.com/golang-gui/goui/platform/graphics"
	"github.com/golang-gui/goui/platform/windows/sdk/winapi"
)

type Settings struct{}

func newSettings() (common.Settings, error) {
	return &Settings{}, nil
}

func (Settings) ColorScheme() (common.ColorScheme, error) {
	value, err := readCurrentUserDWORD(`Software\Microsoft\Windows\CurrentVersion\Themes\Personalize`, "AppsUseLightTheme")
	if err != nil {
		if errors.Is(err, syscall.ERROR_FILE_NOT_FOUND) || errors.Is(err, syscall.ERROR_PATH_NOT_FOUND) {
			return 0, common.ErrSettingUnsupported
		}
		return 0, err
	}
	if value == 0 {
		return common.ColorSchemeDark, nil
	}
	return common.ColorSchemeLight, nil
}

func (Settings) AccentColor() (color.Color, error) {
	var color winapi.DWORD
	var opaque winapi.BOOL
	if err := winapi.DwmGetColorizationColor(&color, &opaque); err != nil {
		if errors.Is(err, syscall.ERROR_MOD_NOT_FOUND) || errors.Is(err, syscall.ERROR_PROC_NOT_FOUND) {
			return graphics.Color{}, common.ErrSettingUnsupported
		}
		return graphics.Color{}, err
	}

	return graphics.RGBA(
		byte(color>>16),
		byte(color>>8),
		byte(color),
		byte(color>>24),
	), nil
}

func (Settings) FontFamily() (string, error) {
	font, err := queryMessageFont()
	if err != nil {
		return "", err
	}
	family := syscall.UTF16ToString(font.FaceName[:])
	if family == "" {
		return "", common.ErrSettingUnsupported
	}
	return family, nil
}

func (Settings) FontSize() (float32, error) {
	font, err := queryMessageFont()
	if err != nil {
		return 0, err
	}
	if font.Height == 0 {
		return 0, common.ErrSettingUnsupported
	}

	dpi, err := systemDPI()
	if err != nil {
		return 0, err
	}
	return float32(math.Abs(float64(font.Height)) * 72 / float64(dpi)), nil
}

func queryMessageFont() (winapi.LOGFONT, error) {
	metrics := winapi.NONCLIENTMETRICS{
		Size: winapi.Sizeof_NONCLIENTMETRICS,
	}
	err := winapi.SystemParametersInfo(
		winapi.SPI_GETNONCLIENTMETRICS,
		metrics.Size,
		unsafe.Pointer(&metrics),
		0,
	)
	if err != nil {
		return winapi.LOGFONT{}, err
	}
	return metrics.MessageFont, nil
}

func systemDPI() (int32, error) {
	hdc, err := winapi.GetDC(0)
	if err != nil {
		return 0, err
	}
	if hdc == 0 {
		return 0, fmt.Errorf("GetDC failed")
	}
	defer winapi.ReleaseDC(0, hdc)

	ret := winapi.GetDeviceCaps(hdc, winapi.LOGPIXELSY)
	if ret == 0 {
		return 0, fmt.Errorf("GetDeviceCaps(LOGPIXELSY) failed")
	}
	return int32(ret), nil
}

func readCurrentUserDWORD(path, name string) (uint32, error) {
	pathPtr, err := syscall.UTF16PtrFromString(path)
	if err != nil {
		return 0, err
	}
	namePtr, err := syscall.UTF16PtrFromString(name)
	if err != nil {
		return 0, err
	}

	var key winapi.HKEY
	if err := winapi.RegOpenKeyEx(winapi.HKEY_CURRENT_USER, pathPtr, 0, winapi.KEY_READ, &key); err != nil {
		return 0, err
	}
	defer winapi.RegCloseKey(key)

	var valueType winapi.DWORD
	var value winapi.DWORD
	size := winapi.DWORD(unsafe.Sizeof(value))
	err = winapi.RegQueryValueEx(
		key,
		namePtr,
		nil,
		&valueType,
		(*winapi.BYTE)(unsafe.Pointer(&value)),
		&size,
	)
	if err != nil {
		return 0, err
	}
	if valueType != winapi.REG_DWORD || size != winapi.DWORD(unsafe.Sizeof(value)) {
		return 0, fmt.Errorf("registry value %q is not a DWORD", name)
	}
	return uint32(value), nil
}
