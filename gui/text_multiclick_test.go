package gui

import (
	"testing"
	"time"

	"github.com/golang-gui/goui/core/geometry"
	"github.com/golang-gui/goui/platform/events"
)

func textSelectionClick(t *testing.T, win *window, point geometry.Point) {
	t.Helper()
	editorPointer(t, win, events.PointerDown, point, 0)
	editorPointer(t, win, events.PointerUp, point, 0)
}

// 固定时钟只控制连续点击判定；所有输入仍经过Window.DispatchEvent。
func multiclickFixture(t *testing.T, single bool) (*textEditor, *window, *time.Time) {
	t.Helper()
	var editor *textEditor
	var win *window
	if single {
		v, w, _ := newInputFixture(t, "one two")
		v.Arrange(geometry.Rect(0, 0, 240, 40))
		v.SetSelection(TextSelection{})
		editor, win = v.textEditor, w
	} else {
		v, w, _ := newEditorFixture(t, "one two\nend")
		editor, win = v.textEditor, w
	}
	now := time.Unix(100, 0)
	editor.input.now = func() time.Time { return now }
	return editor, win, &now
}

func TestTextEditorMultiClickSequenceAndSignals(t *testing.T) {
	for _, single := range []bool{false, true} {
		t.Run(map[bool]string{false: "TextView", true: "TextInput"}[single], func(t *testing.T) {
			e, win, now := multiclickFixture(t, single)
			point := geometry.Point{X: 24, Y: 20}
			end := 8
			if single {
				end = 7
			}
			changes := 0
			e.connectSelection(func(TextSelection) { changes++ })
			for i, want := range []TextSelection{{Anchor: 2, Caret: 2}, {Anchor: 0, Caret: 3}, {Anchor: 0, Caret: end}, {Anchor: 2, Caret: 2}} {
				textSelectionClick(t, win, point)
				if e.selection != want || changes != i+1 || e.owner.Snapshot().TextEditing.Selection != want {
					t.Fatalf("第%d击选区/信号=%+v/%d，预期%+v/%d", i+1, e.selection, changes, want, i+1)
				}
				*now = now.Add(100 * time.Millisecond)
			}
		})
	}
}

func TestTextEditorMultiClickResetsAfterInterruption(t *testing.T) {
	for _, name := range []string{"超时", "时钟回退", "远处", "拖动返回", "键盘", "滚轮", "失焦", "禁用", "卸载", "模型替换", "正文修改", "次键"} {
		t.Run(name, func(t *testing.T) {
			e, win, now := multiclickFixture(t, false)
			point := geometry.Point{X: 24, Y: 8}
			textSelectionClick(t, win, point)
			*now = now.Add(100 * time.Millisecond)
			editorPointer(t, win, events.PointerDown, point, 0)
			if e.selection != (TextSelection{Caret: 3}) {
				t.Fatal("前置双击没有选词")
			}
			if name == "拖动返回" {
				editorPointer(t, win, events.PointerMove, geometry.Point{X: 54, Y: 8}, 0)
				editorPointer(t, win, events.PointerMove, point, 0)
			}
			editorPointer(t, win, events.PointerUp, point, 0)
			*now = now.Add(100 * time.Millisecond)
			switch name {
			case "超时":
				*now = now.Add(501 * time.Millisecond)
			case "时钟回退":
				*now = now.Add(-time.Second)
			case "远处":
				point.X = 44
			case "键盘":
				editorKey(t, win, events.KeyHome, 0)
			case "滚轮":
				_ = win.DispatchEvent(events.WheelEvent{})
			case "失焦":
				win.SetFocusedWidget(nil)
			case "禁用":
				e.owner.SetEnabled(false)
				e.owner.SetEnabled(true)
			case "卸载":
				win.SetWidget(nil)
				win.SetWidget(e.owner)
				e.owner.Arrange(geometry.Rect(0, 0, 209, 108))
			case "模型替换":
				e.setModel(NewTextModel("one two\nend"))
			case "正文修改":
				if err := e.model.Replace(TextRange{Start: 10, End: 11}, "!"); err != nil {
					t.Fatal(err)
				}
			case "次键":
				// 只派发次键按下；本fixture没有原生菜单宿主。
				_ = win.DispatchEvent(events.PointerEvent{EventType: events.PointerDown, Button: events.PointerButtonMiddle, Position: point})
				_ = win.DispatchEvent(events.PointerEvent{EventType: events.PointerUp, Button: events.PointerButtonMiddle, Position: point})
			}
			win.SetFocusedWidget(e.owner)
			textSelectionClick(t, win, point)
			want := TextSelection{Anchor: 2, Caret: 2}
			if name == "远处" {
				want = TextSelection{Anchor: 4, Caret: 4}
			}
			if e.selection != want {
				t.Fatalf("%s后沿用了多击历史：%+v，预期%+v", name, e.selection, want)
			}
		})
	}
}

func TestTextEditorTripleClickLineDragPreservesAnchor(t *testing.T) {
	v, win, _ := newEditorFixture(t, "aa\nbb\ncc\ndd")
	point := geometry.Point{X: 14, Y: 28}
	textSelectionClick(t, win, point)
	textSelectionClick(t, win, point)
	editorPointer(t, win, events.PointerDown, point, 0)
	for _, step := range []struct {
		point geometry.Point
		want  TextSelection
	}{
		{geometry.Point{X: 15, Y: 28}, TextSelection{Anchor: 3, Caret: 6}},
		{geometry.Point{X: 15, Y: 48}, TextSelection{Anchor: 3, Caret: 9}},
		{geometry.Point{X: 15, Y: 68}, TextSelection{Anchor: 3, Caret: 11}},
		{geometry.Point{X: 15, Y: 8}, TextSelection{Anchor: 6, Caret: 0}},
		{geometry.Point{X: 15, Y: 28}, TextSelection{Anchor: 3, Caret: 6}},
	} {
		editorPointer(t, win, events.PointerMove, step.point, 0)
		if v.Selection() != step.want {
			t.Fatalf("整行拖动到%v选区=%+v，预期%+v", step.point, v.Selection(), step.want)
		}
	}
	editorPointer(t, win, events.PointerUp, point, 0)
	if v.input.CapturingPointer() || win.dispatcher.captureTarget != nil {
		t.Fatal("整行拖动结束后仍捕获输入")
	}
}

func TestTextEditorTripleClickThresholds(t *testing.T) {
	for _, tc := range []struct {
		name     string
		interval time.Duration
		distance float32
		triple   bool
	}{
		{"时间边界", 500 * time.Millisecond, 0, true},
		{"超时一纳秒", 500*time.Millisecond + time.Nanosecond, 0, false},
		{"距离边界", 100 * time.Millisecond, 4, true},
		{"距离超限", 100 * time.Millisecond, 5, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			e, win, now := multiclickFixture(t, false)
			point := geometry.Point{X: 24, Y: 8}
			textSelectionClick(t, win, point)
			*now = now.Add(100 * time.Millisecond)
			textSelectionClick(t, win, point)
			*now = now.Add(tc.interval)
			point.X += tc.distance
			textSelectionClick(t, win, point)
			if tc.triple && e.selection != (TextSelection{Caret: 8}) || !tc.triple && e.selection.Anchor != e.selection.Caret {
				t.Fatalf("多击阈值结果错误：%+v", e.selection)
			}
		})
	}
}

func TestTextEditorTripleClickAutoScrollKeepsWholeLines(t *testing.T) {
	v, scroll, win, q, _ := timedEditorFixture(t)
	v.input.now = q.s.now
	point := geometry.Point{X: 14, Y: 28}
	textSelectionClick(t, win, point)
	textSelectionClick(t, win, point)
	editorPointer(t, win, events.PointerDown, point, 0)
	if v.Selection() != (TextSelection{Anchor: 4, Caret: 8}) {
		t.Fatal("三击没有选中第二行row及LF", v.Selection())
	}
	editorPointer(t, win, events.PointerMove, geometry.Point{X: 14, Y: 160}, 0)
	before := v.Selection().Caret
	for range 4 {
		q.advance(textScrollInterval)
		q.dispatch(t)
	}
	if scroll.ScrollY() <= 0 || v.Selection().Anchor != 4 || v.Selection().Caret <= before || v.Selection().Caret%4 != 0 {
		t.Fatal("自动滚动没有继续整行扩选", scroll.ScrollY(), v.Selection())
	}
	editorPointer(t, win, events.PointerUp, point, 0)
	if v.input.CapturingPointer() || v.scrollTimer.Active() {
		t.Fatal("释放后整行自动滚动仍运行")
	}
}

func TestTextEditorTripleClickSelectionCallbackInvalidation(t *testing.T) {
	for _, action := range []string{"卸载", "禁用后恢复", "模型替换"} {
		t.Run(action, func(t *testing.T) {
			e, win, now := multiclickFixture(t, false)
			point := geometry.Point{X: 24, Y: 8}
			textSelectionClick(t, win, point)
			*now = now.Add(100 * time.Millisecond)
			textSelectionClick(t, win, point)
			called := false
			e.connectSelection(func(s TextSelection) {
				if called || s != (TextSelection{Caret: 8}) {
					return
				}
				called = true
				switch action {
				case "卸载":
					win.SetWidget(nil)
				case "禁用后恢复":
					e.owner.SetEnabled(false)
					e.owner.SetEnabled(true)
				case "模型替换":
					e.setModel(NewTextModel("new"))
				}
			})
			*now = now.Add(100 * time.Millisecond)
			editorPointer(t, win, events.PointerDown, point, 0)
			selection := e.selection
			editorPointer(t, win, events.PointerMove, geometry.Point{X: 44, Y: 48}, 0)
			editorPointer(t, win, events.PointerUp, point, 0)
			if !called || e.selection != selection || e.input.CapturingPointer() || win.dispatcher.captureTarget != nil {
				t.Fatal("选择回调失效后仍沿用旧整行拖动", e.selection, selection)
			}
		})
	}
}

func TestTextEditorMultiClickCanceledPressStartsNewSequence(t *testing.T) {
	e, win, now := multiclickFixture(t, false)
	point := geometry.Point{X: 24, Y: 8}
	textSelectionClick(t, win, point)
	*now = now.Add(100 * time.Millisecond)
	textSelectionClick(t, win, point)
	*now = now.Add(100 * time.Millisecond)
	for _, event := range []events.PointerEvent{
		{EventType: events.PointerDown, Button: events.PointerButtonLeft, Buttons: events.PointerButtonLeftDown, Position: point},
		{EventType: events.PointerMove, Position: point}, // 观察到主键已释放，分发器取消当前手势。
		{EventType: events.PointerUp, Button: events.PointerButtonLeft, Position: point},
	} {
		if err := win.DispatchEvent(event); err != nil {
			t.Fatal(err)
		}
	}
	if e.input.CapturingPointer() || win.dispatcher.captureTarget != nil {
		t.Fatal("取消三击后没有释放捕获")
	}
	*now = now.Add(100 * time.Millisecond)
	textSelectionClick(t, win, point)
	if e.selection != (TextSelection{Anchor: 2, Caret: 2}) {
		t.Fatal("取消后下一击没有重新定位", e.selection)
	}
}

func TestTextEditorMultiClickStopsAfterIMEResetChangesModel(t *testing.T) {
	e, win, _ := multiclickFixture(t, false)
	e.onPreedit("候选", 1)
	e.im = &editorResetIM{IMContext: e.im, onReset: func() {
		e.model.SetText("replacement")
	}}
	editorPointer(t, win, events.PointerDown, geometry.Point{X: 24, Y: 8}, 0)
	if e.model.Text() != "replacement" || e.selection != (TextSelection{}) || e.input.CapturingPointer() || win.dispatcher.captureTarget != nil {
		t.Fatal("IME取消回调改文档后仍按旧命中位置建立选择", e.selection)
	}
}

// 三击必须在第三次按下就选中整条逻辑行；软换行不成为文档边界。
func TestTextEditorTripleClickSelectsLogicalLine(t *testing.T) {
	for _, tc := range []struct {
		name, text string
		point      geometry.Point
		width      float32
		want       TextSelection
	}{
		{"中间行及LF", "aa\n中e\u0301 👩‍💻\nlast", geometry.Point{X: 6, Y: 28}, 209, TextSelection{Anchor: 3, Caret: 22}},
		{"空行及LF", "a\n\nz", geometry.Point{X: 6, Y: 28}, 209, TextSelection{Anchor: 2, Caret: 3}},
		{"末行无LF", "a\nlast", geometry.Point{X: 6, Y: 28}, 209, TextSelection{Anchor: 2, Caret: 6}},
		{"末尾空行", "a\n", geometry.Point{X: 6, Y: 28}, 209, TextSelection{Anchor: 2, Caret: 2}},
		{"软换行", "abcdefghij\nlast", geometry.Point{X: 6, Y: 28}, 58, TextSelection{Anchor: 0, Caret: 11}},
		{"空文档", "", geometry.Point{X: 6, Y: 8}, 209, TextSelection{}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			v, win, _ := newEditorFixture(t, tc.text)
			v.Arrange(geometry.Rect(0, 0, tc.width, 108))
			revision := v.Model().Revision()
			textSelectionClick(t, win, tc.point)
			textSelectionClick(t, win, tc.point)
			editorPointer(t, win, events.PointerDown, tc.point, 0)
			if v.Selection() != tc.want {
				t.Fatalf("第三次按下选区=%+v，预期%+v", v.Selection(), tc.want)
			}
			editorPointer(t, win, events.PointerUp, tc.point, 0)
			if v.Snapshot().TextEditing.Selection != tc.want || v.Model().Revision() != revision || v.Model().Text() != tc.text || v.input.CapturingPointer() {
				t.Fatal("三击改变正文、遗漏语义选区或未释放捕获")
			}
		})
	}
}

func TestTextInputTripleClickSelectsAll(t *testing.T) {
	for _, mode := range []string{"普通", "只读", "密码"} {
		t.Run(mode, func(t *testing.T) {
			v, win, _ := newInputFixture(t, "one two 中")
			v.Arrange(geometry.Rect(0, 0, 240, 40))
			v.SetSelection(TextSelection{})
			v.SetReadOnly(mode == "只读")
			v.SetObscured(mode == "密码")
			changes := 0
			v.ConnectText(func(string) { changes++ })
			point := geometry.Point{X: 24, Y: 20}
			textSelectionClick(t, win, point)
			textSelectionClick(t, win, point)
			editorPointer(t, win, events.PointerDown, point, 0)
			want := TextSelection{Caret: len("one two 中")}
			if v.Selection() != want {
				t.Fatalf("三击选区=%+v，预期%+v", v.Selection(), want)
			}
			editorPointer(t, win, events.PointerUp, point, 0)
			if changes != 0 || v.Snapshot().TextEditing.Selection != want {
				t.Fatal("三击发出正文变更或遗漏Snapshot选区")
			}
		})
	}
}
