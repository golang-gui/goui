package x11

import (
	"image/color"
	"os"
	"os/exec"
	"runtime"
	"testing"

	"github.com/golang-gui/goui/platform/common"
	"github.com/golang-gui/goui/platform/linux/libs/gtk"
	"github.com/golang-gui/goui/platform/linux/libs/pango"
)

// 无显示连接时如实报告不可用，不访问全局平台或尝试连接用户桌面。
func TestSettingsCompositedWithoutDisplay(t *testing.T) {
	for _, s := range []Settings{{}, {platform: new(Platform)}} {
		if value, err := s.Composited(); value || err != common.ErrUnavailable {
			t.Fatalf("无连接的合成查询结果：%t, %v", value, err)
		}
	}
}

func TestLocaleLanguage(t *testing.T) {
	for _, tc := range []struct{ locale, want string }{
		{"en_US.UTF-8", "en-US"}, {"zh_CN.UTF-8", "zh-CN"}, {"ja_JP", "ja-JP"},
		{"ko_KR.utf8", "ko-KR"}, {"de_DE@euro", "de-DE"}, {"fr_FR", "fr-FR"},
		{"ru_RU", "ru-RU"}, {"ar_SA", "ar-SA"}, {"sr_RS@latin", "sr-Latn-RS"},
		{"sr_RS@cyrillic", "sr-Cyrl-RS"}, {"ca_ES@valencia", "ca-ES-valencia"},
		{"C", "und"}, {"C.UTF-8", "und"}, {"POSIX", "und"},
	} {
		got, err := localeLanguage(tc.locale)
		if err != nil || got != tc.want {
			t.Errorf("%q: %q, %v; want %q", tc.locale, got, err, tc.want)
		}
	}
	for _, locale := range []string{"", "not a locale", "zh_CN@unknown-script"} {
		if _, err := localeLanguage(locale); err == nil {
			t.Errorf("accepted invalid locale %q", locale)
		}
	}
}

// GLib 查询进程语言偏好无需显示连接。子进程隔离线程缓存与环境，避免影响父测试进程。
func TestSettingsLanguageNative(t *testing.T) {
	if want := os.Getenv("GOUI_TEST_LANGUAGE_EXPECT"); want != "" {
		got, err := (Settings{}).Language()
		if err != nil || got != want {
			t.Fatalf("native language=%q, %v; want %q", got, err, want)
		}
		t.Logf("native preferred UI language: %s", got)
		return
	}
	for _, tc := range []struct{ preference, want string }{
		{"ja_JP.UTF-8:en_US", "ja-JP"}, {"zh_CN:en", "zh-CN"}, {"C", "und"},
	} {
		cmd := exec.Command(os.Args[0], "-test.run=^TestSettingsLanguageNative$", "-test.v")
		cmd.Env = append(os.Environ(), "DISPLAY=", "LC_ALL=C", "LANG=C", "LANGUAGE="+tc.preference, "GOUI_TEST_LANGUAGE_EXPECT="+tc.want)
		output, err := cmd.CombinedOutput()
		if err != nil {
			t.Fatalf("%q: %v\n%s", tc.preference, err, output)
		}
	}
}

func TestXSettingsUnavailableWithoutPlatform(t *testing.T) {
	s := Settings{}
	if _, err := s.ColorScheme(); err != common.ErrUnavailable {
		t.Fatal("无连接明暗查询没有返回不可用", err)
	}
	if _, err := s.AccentColor(); err != common.ErrUnavailable {
		t.Fatal("无连接取色没有返回不可用", err)
	}
}

func TestGTKAccentColorIsPremultiplied(t *testing.T) {
	for _, tc := range []struct {
		value gtk.RGBA
		want  color.RGBA64
	}{
		{gtk.RGBA{Red: 1, Green: 0.5, Blue: 0.25, Alpha: 0.5}, color.RGBA64{R: 32768, G: 16384, B: 8192, A: 32768}},
		{gtk.RGBA{Red: 1, Green: 1, Blue: 1, Alpha: 0}, color.RGBA64{}},
	} {
		got := color.RGBA64Model.Convert(gtkAccentColor(tc.value)).(color.RGBA64)
		if got != tc.want {
			t.Fatalf("GdkRGBA直通道转换：%v，预期预乘%v", got, tc.want)
		}
	}
}

func TestXSettingsFontUnits(t *testing.T) {
	// Pango原生调用在实际执行它的goroutine中锁线程；无需桌面连接。
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()
	for _, tc := range []struct {
		name, family string
		size         float32
	}{{"Serif Bold 15", "Serif", 15}, {"Sans 10.5", "Sans", 10.5}, {"Serif 20px", "Serif", 0}} {
		desc := pango.FontDescriptionFromString(tc.name)
		if desc == 0 {
			t.Fatal("字体描述未创建")
		}
		family, size, err := fontFromDescription(desc)
		desc.Free()
		if err != nil || family != tc.family || size != tc.size {
			t.Fatalf("%q解析为%q/%g/%v，预期%q/%g pt", tc.name, family, size, err, tc.family, tc.size)
		}
	}
}
