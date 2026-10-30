package x11

import (
	"reflect"
	"runtime"
	"testing"
	"unsafe"

	"github.com/golang-gui/goui/core/geometry"
	"github.com/golang-gui/goui/platform/common"
	"github.com/golang-gui/goui/platform/linux/libs/xlib"
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

func TestComposedLookup(t *testing.T) {
	for _, tc := range []struct {
		keycode uint32
		status  xlib.Status
		want    bool
	}{
		{38, xlib.XLookupBoth, false}, // 普通原生字符查询
		{0, xlib.XLookupChars, true},  // 没有 keysym 的 XIM_COMMIT，包含 root style
		{0, xlib.XLookupBoth, true},   // 服务器提交也可以携带 keysym
		{38, xlib.XLookupChars, true}, // 没有 keysym 的组合字符
		{0, xlib.XLookupNone, false}, {0, xlib.XLookupKeySym, false},
	} {
		if got := composedLookup(tc.keycode, tc.status); got != tc.want {
			t.Fatalf("keycode=%d status=%d composed=%t want %t", tc.keycode, tc.status, got, tc.want)
		}
	}
}

func TestInputStyle(t *testing.T) {
	root := uintptr(xlib.XIMPreeditNothing | xlib.XIMStatusNothing)
	inline := uintptr(xlib.XIMPreeditCallbacks | xlib.XIMStatusNothing)
	for _, tc := range []struct {
		styles []uintptr
		want   uintptr
	}{
		{nil, 0}, {[]uintptr{root}, root}, {[]uintptr{root, inline}, inline},
		{[]uintptr{0xffff}, 0}, {[]uintptr{xlib.XIMPreeditNone | xlib.XIMStatusNone}, xlib.XIMPreeditNone | xlib.XIMStatusNone},
	} {
		if got := inputStyle(tc.styles); got != tc.want {
			t.Fatalf("styles %v: got %#x want %#x", tc.styles, got, tc.want)
		}
	}
}

func widePreedit(text string) (*xlib.XIMText, []rune) {
	runes := []rune(text)
	return &xlib.XIMText{Length: uint16(len(runes)), EncodingIsWchar: 1, String: (*byte)(unsafe.Pointer(&runes[0]))}, runes
}

func TestPreeditCharacterChangesAndUTF8Caret(t *testing.T) {
	im := &inputMethod{enabled: true}
	text, storage := widePreedit("a中😀z")
	im.updatePreedit(&xlib.XIMPreeditDrawCallbackStruct{Text: text, Caret: 3})
	runtime.KeepAlive(storage)
	text, storage = widePreedit("你")
	im.updatePreedit(&xlib.XIMPreeditDrawCallbackStruct{Text: text, ChgFirst: 1, ChgLength: 2, Caret: 2})
	runtime.KeepAlive(storage)
	// 只有 feedback 的更新不能删除或替换已有文本。
	im.updatePreedit(&xlib.XIMPreeditDrawCallbackStruct{Text: &xlib.XIMText{Length: 3}, ChgLength: 3, Caret: 1})
	// Text 为 nil 时删除指定字符，不结束整个预编辑。
	im.updatePreedit(&xlib.XIMPreeditDrawCallbackStruct{ChgFirst: 1, ChgLength: 1, Caret: 1})
	want := []common.InputMethodResult{
		{Kind: common.InputMethodPreedit, Text: "a中😀z", Caret: 8},
		{Kind: common.InputMethodPreedit, Text: "a你z", Caret: 4},
		{Kind: common.InputMethodPreedit, Text: "a你z", Caret: 1},
		{Kind: common.InputMethodPreedit, Text: "az", Caret: 1},
	}
	if !reflect.DeepEqual(im.preeditPending, want) {
		t.Fatalf("updates = %#v; want %#v", im.preeditPending, want)
	}
}

func TestPreeditMultibyteText(t *testing.T) {
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()
	// ASCII 不依赖 locale；原生 UTF-8 预编辑由 tests/window/gui/textview 验证。
	// 本测试不修改进程 LC_CTYPE。
	buf := []byte{'n', 'i', 'x', 0}
	text, ok := preeditText(&xlib.XIMText{Length: 2, String: &buf[0]})
	if !ok || string(text) != "ni" {
		t.Fatalf("text=%q ok=%t", string(text), ok)
	}
	runtime.KeepAlive(buf)
}

func TestPreeditContextsAreIndependent(t *testing.T) {
	a, b := &inputMethod{enabled: true}, &inputMethod{enabled: true}
	text, storage := widePreedit("中文")
	a.updatePreedit(&xlib.XIMPreeditDrawCallbackStruct{Text: text, Caret: 2})
	runtime.KeepAlive(storage)
	text, storage = widePreedit("😀")
	b.updatePreedit(&xlib.XIMPreeditDrawCallbackStruct{Text: text, Caret: 1})
	runtime.KeepAlive(storage)
	a.updatePreedit(&xlib.XIMPreeditDrawCallbackStruct{ChgFirst: 0, ChgLength: 1, Caret: 1})
	if string(a.preedit) != "文" || string(b.preedit) != "😀" || b.preeditPending[0].Caret != 4 {
		t.Fatal("preedit storage/caret leaked between native contexts")
	}
}

func TestPreeditCallbacksAndLifecycle(t *testing.T) {
	const ic xlib.XIC = 1 // 仅作登记键，不传给原生函数
	im := &inputMethod{ic: ic, enabled: true}
	inputContexts[ic] = im
	t.Cleanup(func() { delete(inputContexts, ic) })
	if preeditStart(ic, 0, 0) != -1 {
		t.Fatal("preedit length must be unlimited")
	}
	im.preedit = []rune("a😀z")
	caret := &xlib.XIMPreeditCaretCallbackStruct{Direction: xlib.XIMAbsolutePosition, Position: 2}
	preeditCaret(ic, 0, caret)
	if caret.Position != 2 || im.preeditPending[0].Caret != 5 {
		t.Fatal("caret used bytes as character indexes")
	}
	preeditDone(ic, 0, 0)
	if len(im.preeditPending) != 2 || im.preeditPending[1].Text != "" {
		t.Fatal("done must end composition")
	}
	var seen []string
	im.handler = func(result common.InputMethodResult) { seen = append(seen, result.Text) }
	flushPreedit()
	if !reflect.DeepEqual(seen, []string{"a😀z", ""}) {
		t.Fatalf("flush: %v", seen)
	}
	// 处理更新时 Reset 必须取消此前排队的全部输出。
	im.preedit = []rune("x")
	im.queuePreedit()
	im.queuePreedit()
	// 在同一 flush 路径验证回调触发 Reset；模拟 ID 仅用于登记，Reset 前清零。
	calls := 0
	im.handler = func(common.InputMethodResult) { calls++; im.ic = 0; im.Reset() }
	flushPreedit()
	if calls != 1 || len(im.preeditPending) != 0 {
		t.Fatal("reset leaked a pending update")
	}
	// 未知或已销毁的上下文不能接收迟到的原生回调。
	delete(inputContexts, ic)
	preeditDone(ic, 0, 0)
	if len(im.preeditPending) != 0 {
		t.Fatal("late callback revived a destroyed context")
	}
}

// 不解引用原生句柄，按分发顺序分别执行回调状态变更及 Xlib 返回后的通知。
// 替换平台线程登记表的测试不能并行执行。
func inputLossFixture(t *testing.T) (*Platform, []*inputMethod, *[]common.InputMethodResult) {
	t.Helper()
	oldPlatform, oldContexts, oldLost := platform, inputContexts, lostInputContexts
	t.Cleanup(func() { platform, inputContexts, lostInputContexts = oldPlatform, oldContexts, oldLost })
	p := &Platform{im: 123}
	platform, inputContexts, lostInputContexts = p, make(map[xlib.XIC]*inputMethod), nil
	var results []common.InputMethodResult
	contexts := make([]*inputMethod, 3)
	for i := range contexts {
		im := &inputMethod{
			window: &Window{}, ic: xlib.XIC(i + 1), enabled: i != 2,
			spotSet: true, preedit: []rune("中"), preeditCaret: 1,
			preeditPending: []common.InputMethodResult{{Kind: common.InputMethodPreedit, Text: "stale"}},
			handler:        func(result common.InputMethodResult) { results = append(results, result) },
		}
		im.window.im = im
		contexts[i], inputContexts[im.ic] = im, im
	}
	return p, contexts, &results
}

func TestInputMethodLossInvalidatesBeforeNotification(t *testing.T) {
	p, contexts, results := inputLossFixture(t)
	inputMethodDestroyed(999, 0, 0)
	if p.im != 123 || len(inputContexts) != 3 {
		t.Fatal("an unrelated XIM invalidated the active input method")
	}
	inputMethodDestroyed(123, 0, 0)
	if p.im != 0 || len(inputContexts) != 0 || len(*results) != 0 {
		t.Fatal("callback must invalidate handles without entering GUI")
	}
	for _, im := range contexts {
		if im.ic != 0 || im.enabled || im.spotSet || len(im.preedit) != 0 || im.preeditCaret != 0 || len(im.preeditPending) != 0 {
			t.Fatal("lost context retained native/compose state")
		}
		if im.window == nil || im.window.im != im {
			t.Fatal("loss stole the owner's Go context")
		}
		// 这些操作必须安全返回，不能将假句柄传给原生接口。
		im.SetEnabled(true)
		im.SetCaretRect(geometry.Rect(1, 2, 1, 20))
	}
	inputMethodDestroyed(123, 0, 0) // 重复通知不能重复排队
	flushPreedit()
	if len(*results) != 2 || len(lostInputContexts) != 0 {
		t.Fatalf("loss results=%+v pending=%d", *results, len(lostInputContexts))
	}
	for _, result := range *results {
		if result != (common.InputMethodResult{Kind: common.InputMethodPreedit}) {
			t.Fatalf("delivered stale composition or committed on loss: %+v", result)
		}
	}
	for _, im := range contexts {
		im.Reset()
		im.Destroy()
	}
}

func TestInputMethodLossResetDestroyBeforeDelivery(t *testing.T) {
	_, contexts, results := inputLossFixture(t)
	inputMethodDestroyed(123, 0, 0)
	contexts[0].Reset()
	contexts[1].Destroy()
	flushPreedit()
	if len(*results) != 0 || len(lostInputContexts) != 0 {
		t.Fatal("reset/destroyed recipient received a queued loss")
	}
}

func TestInputMethodLossHandlerDestroysOtherRecipient(t *testing.T) {
	_, contexts, _ := inputLossFixture(t)
	count := 0
	contexts[0].handler = func(common.InputMethodResult) { count++; contexts[1].Destroy() }
	contexts[1].handler = func(common.InputMethodResult) { count++; contexts[0].Destroy() }
	inputMethodDestroyed(123, 0, 0)
	flushPreedit()
	flushPreedit()
	if count != 1 {
		t.Fatalf("callbacks after recipient destruction: %d", count)
	}
}
