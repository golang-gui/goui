package win32

import (
	"runtime"
	"testing"

	"github.com/golang-gui/goui/platform/common"
	"golang.org/x/text/language"
)

// 调用真实 Windows 语言 API，不创建窗口或事件循环。
func TestSettingsLanguageNative(t *testing.T) {
	got, err := (Settings{}).Language()
	if err != nil {
		t.Fatal(err)
	}
	if tag, err := language.Parse(got); err != nil || tag == language.Und {
		t.Fatalf("invalid native language %q: %v", got, err)
	}
	again, err := (Settings{}).Language()
	if err != nil || again != got {
		t.Fatalf("unstable language: %q, %v; first %q", again, err, got)
	}
	t.Logf("native preferred UI language: %s", got)
}

// Windows 8+ 始终启用 DWM，裸测试 EXE 不应因缺少兼容性 manifest 而误报 false。
// 仅查询系统能力，不创建 HWND，也不改变 DWM 或用户设置。
func TestSettingsCompositedWithoutApplicationManifest(t *testing.T) {
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()
	p := new(Platform)
	build, err := p.windowsBuild()
	if err != nil {
		t.Fatal(err)
	}
	settings, err := p.NewSettings()
	if err != nil {
		t.Fatal(err)
	}
	value, err := settings.Composited()
	if err != nil || build >= 9200 && !value {
		t.Fatal("DWM 查询失败或受 manifest 兼容返回影响", build, value, err)
	}
	p.destroyed = true
	if value, err := settings.Composited(); value || err != common.ErrUnavailable {
		t.Fatal("销毁后仍报告有效查询结果", value, err)
	}
}
