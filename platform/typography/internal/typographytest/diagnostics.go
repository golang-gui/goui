package typographytest

import (
	"image"
	"runtime"
	"testing"

	"github.com/golang-gui/goui/internal/diagnostic"
	"github.com/golang-gui/goui/platform/typography"
)

// Diagnostics 用相同契约检查三个原生排版后端；不要求桌面或窗口。
func Diagnostics(t *testing.T, create func() (typography.Context, error)) {
	t.Helper()
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()
	s, err := diagnostic.Start()
	if err != nil {
		t.Fatal(err)
	}
	defer s.Stop()
	ctx, err := create()
	if err != nil {
		t.Fatal(err)
	}
	defer ctx.Destroy()
	l, err := ctx.NewTextLayout("diagnostic", typography.TextFormat{Font: typography.FontInfo{Size: 14}, WrapMode: typography.WrapWordChar}, 120, 80)
	if err != nil {
		t.Fatal(err)
	}
	defer l.Destroy()
	if s.Values()[diagnostic.LayoutCreate] != 1 {
		t.Fatal("成功创建未计数", s.Values())
	}
	before := s.Values()[diagnostic.LayoutResize]
	l.SetSize(120, 80)
	if s.Values()[diagnostic.LayoutResize] != before {
		t.Fatal("相同尺寸被计为重排")
	}
	l.SetSize(140, 80)
	if s.Values()[diagnostic.LayoutResize] != before+1 {
		t.Fatal("尺寸调整未计数")
	}
	_, err = l.Rasterize(1, image.Rect(0, 0, 64, 32), nil)
	if err != nil {
		t.Fatal(err)
	}
	if s.Values()[diagnostic.TextRasterize] != 1 {
		t.Fatal("成功栅格化未计数")
	}
	if _, err = l.Rasterize(0, image.Rect(0, 0, 64, 32), nil); err == nil {
		t.Fatal("接受无效倍率")
	}
	if s.Values()[diagnostic.TextRasterize] != 1 {
		t.Fatal("失败栅格化被计为成功")
	}
	l.Destroy()
	l.Destroy()
	if s.Values()[diagnostic.LayoutDestroy] != 1 {
		t.Fatal("重复销毁重复计数")
	}
}
