package win32

import (
	"bytes"
	"encoding/binary"
	"image"
	"image/color"
	"image/png"
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/golang-gui/goui/core/geometry"
	"github.com/golang-gui/goui/platform/common"
	"github.com/golang-gui/goui/platform/events"
	"github.com/golang-gui/goui/platform/windows/sdk/winapi"
)

// 生成真实的多尺寸 ICO 验证原生图标提取，不依赖系统 DLL 资源或资源编译器。
func writeTestIcon(t *testing.T) string {
	t.Helper()
	var images [][]byte
	for _, size := range []int{16, 32} {
		img := image.NewNRGBA(image.Rect(0, 0, size, size))
		for y := 0; y < size; y++ {
			for x := 0; x < size; x++ {
				img.SetNRGBA(x, y, color.NRGBA{R: 64, G: 128, B: 255, A: 255})
			}
		}
		var buffer bytes.Buffer
		if err := png.Encode(&buffer, img); err != nil {
			t.Fatal(err)
		}
		images = append(images, buffer.Bytes())
	}
	header := make([]byte, 6+16*len(images))
	binary.LittleEndian.PutUint16(header[2:], 1)
	binary.LittleEndian.PutUint16(header[4:], uint16(len(images)))
	offset := len(header)
	for i, data := range images {
		entry := header[6+i*16:]
		entry[0], entry[1] = byte(16<<i), byte(16<<i)
		binary.LittleEndian.PutUint16(entry[4:], 1)
		binary.LittleEndian.PutUint16(entry[6:], 32)
		binary.LittleEndian.PutUint32(entry[8:], uint32(len(data)))
		binary.LittleEndian.PutUint32(entry[12:], uint32(offset))
		offset += len(data)
	}
	for _, data := range images {
		header = append(header, data...)
	}
	path := filepath.Join(t.TempDir(), "应用图标.ico")
	if err := os.WriteFile(path, header, 0600); err != nil {
		t.Fatal(err)
	}
	return path
}

// 历史窗口系统测试：创建隐藏 HWND，保留原有线程锁定与原生资源释放。
func TestApplicationClassIcons(t *testing.T) {
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()
	icons := loadApplicationIcons(writeTestIcon(t))
	if icons.large == 0 || icons.small == 0 {
		t.Fatal("native icon extraction failed")
	}
	instance, err := winapi.GetModuleHandle(nil)
	if err != nil {
		icons.destroy()
		t.Fatal(err)
	}
	p := &Platform{instance: instance, icons: icons}
	defer p.Destroy()
	if err := p.registerWindow(); err != nil {
		t.Fatal(err)
	}
	var cls winapi.WNDCLASSEX
	cls.Size = winapi.Sizeof_WNDCLASSEX
	if err := winapi.GetClassInfoEx(instance, p.windowClass, &cls); err != nil {
		t.Fatal(err)
	}
	if cls.Icon != icons.large || cls.IconSm != icons.small {
		t.Fatalf("class icons = %v/%v, want %v/%v", cls.Icon, cls.IconSm, icons.large, icons.small)
	}
	savedPlatform := platform
	platform = p
	defer func() { platform = savedPlatform }()
	win, err := newWindow(geometry.Size{Width: 120, Height: 80}, func(events.Event) {}, common.WindowOptions{})
	if err != nil {
		t.Fatal(err)
	}
	defer win.Destroy()
	p.Destroy()
	if !p.windowRegistered || p.icons != icons || p.destroyed {
		t.Fatal("released class icons while a native window still uses them")
	}
	win.Destroy()
	p.Destroy()
	if p.windowRegistered || p.icons != (applicationIcons{}) || !p.destroyed {
		t.Fatal("class icon resources were not released")
	}
	if err := winapi.GetClassInfoEx(instance, p.windowClass, &cls); err == nil {
		t.Fatal("window class is still registered")
	}
	p.Destroy() // 重复清理必须安全。
}

func TestApplicationIconsMissingAndShared(t *testing.T) {
	icons := loadApplicationIcons(filepath.Join(t.TempDir(), "missing.exe"))
	if icons != (applicationIcons{}) {
		icons.destroy()
		t.Fatal("missing file produced icons")
	}
	icons = loadApplicationIcons(writeTestIcon(t))
	if icons.large == 0 {
		t.Fatal("native icon extraction failed")
	}
	if icons.small != 0 && icons.small != icons.large {
		winapi.DestroyIcon(icons.small)
	}
	icons.small = icons.large
	icons.destroy()
	icons.destroy()
	if icons != (applicationIcons{}) {
		t.Fatal("shared handle cleanup did not clear both fields")
	}
}
