package win32

import (
	"testing"

	"github.com/golang-gui/goui/platform/common"
)

// 无效输入在调用原生接口前拒绝，无需桌面。
func TestInputMethodFactoryErrorHasNoInstance(t *testing.T) {
	p := &Platform{}
	im, err := p.NewInputMethod(nil, nil)
	if err == nil {
		t.Fatal("unavailable input method succeeded")
	}
	if im != nil {
		t.Fatalf("failure returned a non-nil interface holding %T", im)
	}
}

// 不创建 HWND、消息循环或 IMM 上下文，只验证系统翻译按键后 WM_CHAR 的 UTF-16 拼接。
func TestInputMethodNativeCharacters(t *testing.T) {
	for _, tc := range []struct {
		name  string
		units []uint16
		want  string
	}{
		{"ascii", []uint16{'a', 'A', ' ', '!'}, "aA !"},
		{"layout", []uint16{'é', '@', '中'}, "é@中"},
		{"surrogate", []uint16{0xd83d, 0xde00}, "😀"},
		{"controls", []uint16{0, 1, 8, 9, 13, 27, 127}, ""},
		{"lone low", []uint16{0xde00}, "�"},
		{"unpaired high", []uint16{0xd83d, 'a'}, "�a"},
		{"replaced high", []uint16{0xd800, 0xd83d, 0xde00}, "�😀"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var got string
			im := &inputMethod{window: &Window{}, enabled: true, handler: func(r common.InputMethodResult) {
				if r.Kind != common.InputMethodCommit {
					t.Fatal("expected commit")
				}
				got += r.Text
			}}
			for _, unit := range tc.units {
				im.handleChar(unit, 1, false)
			}
			if got != tc.want {
				t.Fatalf("got %q, want %q", got, tc.want)
			}
		})
	}
}

func TestInputMethodCharacterLifetime(t *testing.T) {
	var got string
	im := &inputMethod{window: &Window{}, enabled: true, handler: func(r common.InputMethodResult) { got += r.Text }}
	im.handleChar('x', 3, false)
	if got != "xxx" {
		t.Fatalf("repeat=%q", got)
	}
	im.handleChar(0xd83d, 1, false)
	im.Reset() // 没有 HWND，也必须清除未完成的代理对。
	im.handleChar('a', 1, false)
	if got != "xxxa" {
		t.Fatalf("reset leaked surrogate: %q", got)
	}
	im.enabled = false
	im.handleChar('b', 1, false)
	im.Destroy()
	im.handleChar('c', 1, false)
	if got != "xxxa" {
		t.Fatalf("inactive input committed: %q", got)
	}
}

func TestInputMethodCharacterCompositionSource(t *testing.T) {
	var got []common.InputMethodResult
	im := &inputMethod{window: &Window{}, enabled: true, handler: func(r common.InputMethodResult) { got = append(got, r) }}
	im.handleChar('中', 1, false) // WM_CHAR：不能用文本内容推断是否来自 IME。
	im.handleChar('a', 1, true)  // WM_IME_CHAR：ASCII 也属于 IME 结果。
	im.handleChar(0xd83d, 1, true)
	im.handleChar(0xde00, 1, true)
	if len(got) != 3 || got[0].Composed || !got[1].Composed || !got[2].Composed || got[2].Text != "😀" {
		t.Fatalf("native character source lost: %+v", got)
	}
}
