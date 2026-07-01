package win32

import (
	"errors"
	"fmt"
	"image/color"
	"math"
	"syscall"
	"time"
	"unsafe"

	"github.com/golang-gui/goui/platform/common"
	"github.com/golang-gui/goui/platform/graphics"
	"github.com/golang-gui/goui/platform/windows/sdk/winapi"
)

type Settings struct {
	onChanged func()
}

func newSettings(onChanged func()) (common.Settings, error) {
	s := &Settings{onChanged: onChanged}
	if onChanged != nil {
		go s.watch()
	}
	return s, nil
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

func (s *Settings) watch() {
	ticker := time.NewTicker(time.Second)
	defer ticker.Stop()

	prev := s.snapshot()
	for range ticker.C {
		next := s.snapshot()
		if next == prev {
			continue
		}
		prev = next
		s.onChanged()
	}
}

type settingsSnapshot struct {
	ColorScheme    common.ColorScheme
	ColorSchemeErr string
	AccentR        uint32
	AccentG        uint32
	AccentB        uint32
	AccentA        uint32
	AccentErr      string
	FontFamily     string
	FontFamilyErr  string
	FontSize       float32
	FontSizeErr    string
}

func (s *Settings) snapshot() settingsSnapshot {
	var snapshot settingsSnapshot

	snapshot.ColorScheme, snapshot.ColorSchemeErr = snapshotColorScheme(s.ColorScheme())
	snapshot.AccentR, snapshot.AccentG, snapshot.AccentB, snapshot.AccentA, snapshot.AccentErr = snapshotColor(s.AccentColor())
	snapshot.FontFamily, snapshot.FontFamilyErr = snapshotString(s.FontFamily())
	snapshot.FontSize, snapshot.FontSizeErr = snapshotFloat32(s.FontSize())

	return snapshot
}

func snapshotColorScheme(value common.ColorScheme, err error) (common.ColorScheme, string) {
	return value, snapshotError(err)
}

func snapshotColor(value color.Color, err error) (uint32, uint32, uint32, uint32, string) {
	if err != nil {
		return 0, 0, 0, 0, snapshotError(err)
	}
	if value == nil {
		return 0, 0, 0, 0, "<nil>"
	}
	r, g, b, a := value.RGBA()
	return r, g, b, a, ""
}

func snapshotString(value string, err error) (string, string) {
	return value, snapshotError(err)
}

func snapshotFloat32(value float32, err error) (float32, string) {
	return value, snapshotError(err)
}

func snapshotError(err error) string {
	if err == nil {
		return ""
	}
	if errors.Is(err, common.ErrSettingUnsupported) {
		return common.ErrSettingUnsupported.Error()
	}
	return err.Error()
}
