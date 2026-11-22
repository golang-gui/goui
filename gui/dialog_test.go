package gui

import (
	"errors"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/golang-gui/goui/core/geometry"
	"github.com/golang-gui/goui/platform"
	"github.com/golang-gui/goui/platform/events"
)

type dialogTestNative struct {
	platform.FileDialog
	result  []platform.FileAccess
	options platform.DialogOptions
	err     error
	format  platform.FileFilterFormat
}

func (d *dialogTestNative) FilterFormat() platform.FileFilterFormat {
	if d.format == platform.FileFilterUnknown {
		return platform.FileFilterGlob
	}
	return d.format
}

func (d *dialogTestNative) OpenFile(_ platform.Window, opts platform.DialogOptions, cb func([]platform.FileAccess, error)) {
	d.options = opts
	cb(d.result, d.err)
}
func (d *dialogTestNative) OpenDirectory(w platform.Window, o platform.DialogOptions, cb func([]platform.FileAccess, error)) {
	d.OpenFile(w, o, cb)
}
func (d *dialogTestNative) SaveFile(w platform.Window, o platform.DialogOptions, cb func([]platform.FileAccess, error)) {
	d.OpenFile(w, o, cb)
}
func (d *dialogTestNative) RestoreAccess([]byte) (platform.FileAccess, error) { return nil, d.err }

func TestFileDialogResultsAndErrors(t *testing.T) {
	failure := errors.New("native failure")
	for _, native := range []*dialogTestNative{{}, {err: failure}} {
		loop := &settingsTestLoop{posts: make(chan func(), 1)}
		dialog := newFileDialog(native, loop, nil)
		for _, call := range []func(Surface, DialogOptions, func([]FileAccess, error)){dialog.OpenFile, dialog.OpenDirectory, dialog.SaveFile} {
			count := 0
			call(nil, DialogOptions{}, func(result []FileAccess, err error) {
				count++
				if len(result) != 0 || err != native.err {
					t.Fatalf("wrong result: %v %v", result, err)
				}
			})
			if count != 0 {
				t.Fatal("callback bypassed event-loop queue")
			}
			(<-loop.posts)() // 启动原生操作。
			if count != 0 {
				t.Fatal("native completion bypassed event-loop queue")
			}
			(<-loop.posts)() // 交付完成结果。
			if count != 1 {
				t.Fatal("callback not delivered exactly once")
			}
		}
		if _, err := dialog.RestoreAccess(nil); err != native.err {
			t.Fatal(err)
		}
	}
	for _, creationErr := range []error{failure, nil} {
		loop := &settingsTestLoop{posts: make(chan func(), 1)}
		dialog := newFileDialog(nil, loop, creationErr)
		want := creationErr
		if want == nil {
			want = platform.ErrUnsupported
		}
		for _, call := range []func(Surface, DialogOptions, func([]FileAccess, error)){dialog.OpenFile, dialog.OpenDirectory, dialog.SaveFile} {
			call(nil, DialogOptions{}, func(result []FileAccess, err error) {
				if len(result) != 0 || err != want {
					t.Fatalf("creation failure swallowed: %v %v", result, err)
				}
			})
			(<-loop.posts)()
		}
		if _, err := dialog.RestoreAccess(nil); err != want {
			t.Fatal(err)
		}
	}
}

// 使用真正的 Window/ModalScope 分发逻辑，选择器用可控的异步替身。
type dialogPendingNative struct {
	platform.FileDialog
	owner    platform.Window
	complete func([]platform.FileAccess, error)
	starts   int
}

func (*dialogPendingNative) FilterFormat() platform.FileFilterFormat { return platform.FileFilterGlob }

func (d *dialogPendingNative) OpenFile(owner platform.Window, _ platform.DialogOptions, cb func([]platform.FileAccess, error)) {
	d.starts++
	d.owner = owner
	d.complete = cb
}

type dialogOwnedAccess struct {
	FileAccess
	closed bool
}

func (a *dialogOwnedAccess) Close() error { a.closed = true; return nil }

func TestFileDialogReusesModalScopeBoundary(t *testing.T) {
	w, _, back, layer, inside, _, scope := modalFixture(t)
	w.platformWindow = &showTestWindow{}
	layer.SetVisible(true)
	if !scope.Active() || w.FocusedWidget() != inside {
		t.Fatal("模态子树前置状态错误")
	}
	native := &dialogPendingNative{}
	loop := &settingsTestLoop{posts: make(chan func(), 16)}
	dialog := newFileDialog(native, loop, nil)
	called := 0
	dialog.OpenFile(w, DialogOptions{}, func(access []FileAccess, err error) {
		called++
		if err != nil || modalRootBlocked(w) {
			t.Fatalf("完成前未解除窗口限制：%v", err)
		}
	})
	if native.starts != 0 || !modalRootBlocked(w) || modalWidgetAllowed(inside) || modalInputAllowed(w, inside) {
		t.Fatal("原生启动前未立即建立共同模态边界")
	}
	if !scope.Active() || w.FocusedWidget() != inside {
		t.Fatal("原生对话框破坏了已有 ModalScope 或逻辑焦点")
	}
	(<-loop.posts)()
	if native.starts != 1 || native.owner != w.platformWindow {
		t.Fatal("owner 没有传入原生实现")
	}
	keys := 0
	shortcut := NewShortcut(KeyGesture{Key: KeyF5})
	shortcut.ConnectActivate(func() { keys++ })
	w.Shortcuts().AddShortcut(shortcut)
	if !modalKey(t, w, events.KeyF5) || keys != 0 {
		t.Fatal("窗口模态期间快捷键未被消费")
	}
	closes := 0
	w.ConnectCloseRequest(func(*bool) { closes++ })
	_ = w.DispatchEvent(events.CloseEvent{})
	if closes != 0 || w.destroyed {
		t.Fatal("背景关闭操作越过了原生模态边界")
	}
	// 第二次请求不得重新启动，也不得解除第一次的边界。
	dialog.OpenFile(w, DialogOptions{}, func(_ []FileAccess, err error) {
		if !errors.Is(err, ErrDialogBusy) {
			t.Fatalf("重复请求：%v", err)
		}
	})
	(<-loop.posts)()
	if native.starts != 1 || !modalRootBlocked(w) {
		t.Fatal("重复请求改变了活动操作")
	}
	// 另一个窗口沿正常输入路径工作。
	other, _, _, _, _, _, _ := modalFixture(t)
	sc := NewShortcut(KeyGesture{Key: KeyF5})
	sc.ConnectActivate(func() { keys++ })
	other.Shortcuts().AddShortcut(sc)
	modalKey(t, other, events.KeyF5)
	if keys != 1 {
		t.Fatal("错误地阻止了其他窗口")
	}
	// 原生对话框期间移除内层 Widget 模态，完成后应按旧模态规则恢复后台焦点。
	layer.SetVisible(false)
	native.complete(nil, nil)
	if called != 0 || !modalRootBlocked(w) {
		t.Fatal("原生完成回调绕过了 GUI 队列")
	}
	(<-loop.posts)()
	if called != 1 || w.FocusedWidget() != back || scope.Active() {
		t.Fatal("退出原生模态后未恢复现有 ModalScope 的焦点语义")
	}
	modalKey(t, w, events.KeyF5)
	if keys != 2 {
		t.Fatal("完成后快捷键未恢复")
	}
}

func TestFileDialogBlocksTextAndDropsLateAccess(t *testing.T) {
	input, w, _ := newInputFixture(t, "initial")
	w.platformWindow = &showTestWindow{}
	native := &dialogPendingNative{}
	loop := &settingsTestLoop{posts: make(chan func(), 16)}
	dialog := newFileDialog(native, loop, nil)
	called := 0
	dialog.OpenFile(w, DialogOptions{}, func(access []FileAccess, err error) {
		called++
		if len(access) != 0 || !errors.Is(err, platform.ErrUnavailable) {
			t.Fatalf("失效 owner 收到有效结果：%v %v", access, err)
		}
	})
	(<-loop.posts)()
	if err := w.CommitText("bad"); err == nil {
		t.Fatal("文本提交绕过原生模态")
	}
	w.onInputMethod(platform.InputMethodResult{Kind: platform.InputMethodCommit, Text: "bad"})
	if input.Text() != "initial" || !input.Enabled() {
		t.Fatal("IME 越界或模态改变了 Enabled")
	}
	// 在正常输入路径派发的滚轮/指针同样不得影响编辑器。
	_ = w.DispatchEvent(events.WheelEvent{Position: geometry.Point{X: 10, Y: 10}, DeltaY: 1})
	w.Destroy()
	access := &dialogOwnedAccess{}
	native.complete([]FileAccess{access}, nil)
	(<-loop.posts)()
	if called != 1 || !access.closed {
		t.Fatal("owner 销毁后没有回收迟到的 FileAccess")
	}
}

func TestFileDialogOwnerDestroyedBeforeStart(t *testing.T) {
	w, _, _, _, _, _, _ := modalFixture(t)
	w.platformWindow = &showTestWindow{}
	native := &dialogPendingNative{}
	loop := &settingsTestLoop{posts: make(chan func(), 8)}
	dialog := newFileDialog(native, loop, nil)
	called := false
	dialog.OpenFile(w, DialogOptions{}, func(_ []FileAccess, err error) {
		called = true
		if !errors.Is(err, platform.ErrUnavailable) {
			t.Fatal(err)
		}
	})
	w.Destroy()
	(<-loop.posts)()
	(<-loop.posts)()
	if !called || native.starts != 0 {
		t.Fatal("为已销毁窗口启动了原生选择器")
	}
}

func TestFileDialogSnapshotsOptionsBeforePosting(t *testing.T) {
	native := &dialogTestNative{}
	loop := &settingsTestLoop{posts: make(chan func(), 8)}
	dialog := newFileDialog(native, loop, nil)
	opts := DialogOptions{Filters: []FileFilter{{Name: "Text", Extensions: []string{"txt"}}}}
	dialog.OpenFile(nil, opts, func([]FileAccess, error) {})
	opts.Filters[0].Name = "Changed"
	opts.Filters[0].Extensions[0] = "bad"
	(<-loop.posts)()
	if native.options.Filters[0].Name != "Text" || native.options.Filters[0].Values[0] != "*.[tT][xX][tT]" {
		t.Fatalf("异步启动读取了调用方修改后的选项：%v", native.options)
	}
	(<-loop.posts)()
}

func TestFileDialogExtensionAdaptation(t *testing.T) {
	for _, test := range []struct {
		name   string
		format platform.FileFilterFormat
		values []string
		all    []string
	}{
		{"区分大小写的 glob", platform.FileFilterGlob, []string{"*.[tT][xX][tT]", "*.[mM][dD]", "*.[xX]-1"}, []string{"*"}},
		{"不区分大小写的 wildcard", platform.FileFilterWildcard, []string{"*.txt", "*.md", "*.x-1"}, []string{"*.*"}},
		{"原生扩展名", platform.FileFilterExtension, []string{"txt", "md", "x-1"}, []string{}},
	} {
		t.Run(test.name, func(t *testing.T) {
			native := &dialogTestNative{format: test.format}
			dialog := newFileDialog(native, nil, nil).(*fileDialog)
			opts := DialogOptions{
				Title: "选择", InitialDir: "/files", DefaultName: "report.md", AllowMultiple: true,
				Filters: []FileFilter{{Name: "文本", Extensions: []string{"TXT", "md", "txt", "x-1"}}, {Name: "全部"}},
			}
			got, err := dialog.nativeOptions(opts)
			if err != nil {
				t.Fatal(err)
			}
			want := platform.DialogOptions{
				Title: opts.Title, InitialDir: opts.InitialDir, DefaultName: opts.DefaultName, AllowMultiple: true,
				Filters: []platform.FileFilter{{Name: "文本", Values: test.values}, {Name: "全部", Values: test.all}},
			}
			if !reflect.DeepEqual(got, want) {
				t.Fatalf("原生参数=%#v，预期=%#v", got, want)
			}
			if !reflect.DeepEqual(opts.Filters[0].Extensions, []string{"TXT", "md", "txt", "x-1"}) {
				t.Fatal("规范化改写了调用方的扩展名")
			}
		})
	}
	// 以明确的文件名预期核对 GUI 生成的 Linux 条件，不只比较字符串形状。
	dialog := newFileDialog(&dialogTestNative{}, nil, nil).(*fileDialog)
	opts, err := dialog.nativeOptions(DialogOptions{Filters: []FileFilter{{Extensions: []string{"txt", "md"}}}})
	if err != nil {
		t.Fatal(err)
	}
	for _, test := range []struct {
		name string
		want bool
	}{
		{"report.txt", true}, {"report.TXT", true}, {"report.TxT", true}, {"文档.Md", true},
		{"report.txt.bak", false}, {"report.html", false}, {"README", false}, {"reporttxt", false},
	} {
		matched := false
		for _, pattern := range opts.Filters[0].Values {
			ok, err := filepath.Match(pattern, test.name)
			if err != nil {
				t.Fatal(err)
			}
			matched = matched || ok
		}
		if matched != test.want {
			t.Errorf("文件 %q 匹配=%t，预期=%t", test.name, matched, test.want)
		}
	}
}

func TestFileDialogRejectsInvalidExtensionsBeforeModal(t *testing.T) {
	for _, extension := range []string{"", ".txt", "*.txt", "tar.gz", "a?", "[a]", "a/b", "a\\b", "txt;md", "txt|md", " txt", "t xt", "txt\n", "x\x00", "\xff"} {
		t.Run(extension, func(t *testing.T) {
			w, _, _, _, _, _, _ := modalFixture(t)
			w.platformWindow = &showTestWindow{}
			native := &dialogPendingNative{}
			loop := &settingsTestLoop{posts: make(chan func(), 4)}
			dialog := newFileDialog(native, loop, nil)
			for _, call := range []func(Surface, DialogOptions, func([]FileAccess, error)){dialog.OpenFile, dialog.SaveFile} {
				count := 0
				call(w, DialogOptions{Filters: []FileFilter{{Extensions: []string{extension}}}}, func(access []FileAccess, err error) {
					count++
					if err == nil || len(access) != 0 {
						t.Fatalf("非法扩展名被当作成功或取消：%v %v", access, err)
					}
				})
				if count != 0 || native.starts != 0 || modalRootBlocked(w) {
					t.Fatal("参数错误同步回调、启动面板或阻塞了 owner")
				}
				(<-loop.posts)()
				if count != 1 || len(loop.posts) != 0 {
					t.Fatal("参数错误没有通过正常队列仅交付一次")
				}
			}
		})
	}
}

func TestFileDialogDirectoryIgnoresFileFilters(t *testing.T) {
	native := &dialogTestNative{}
	loop := &settingsTestLoop{posts: make(chan func(), 4)}
	dialog := newFileDialog(native, loop, nil)
	dialog.OpenDirectory(nil, DialogOptions{Filters: []FileFilter{{Extensions: []string{"*.bad"}}}}, func(_ []FileAccess, err error) {
		if err != nil {
			t.Fatal(err)
		}
	})
	(<-loop.posts)()
	(<-loop.posts)()
	if len(native.options.Filters) != 0 {
		t.Fatal("目录选择收到了文件过滤条件")
	}
}

func TestFileDialogUnknownNativeFilterFormat(t *testing.T) {
	dialog := newFileDialog(&dialogTestNative{format: platform.FileFilterFormat(255)}, nil, nil).(*fileDialog)
	if _, err := dialog.nativeOptions(DialogOptions{}); err != nil {
		t.Fatalf("无过滤的操作不应要求过滤能力：%v", err)
	}
	for _, filter := range []FileFilter{{Extensions: []string{"txt"}}, {Name: "All files"}} {
		if _, err := dialog.nativeOptions(DialogOptions{Filters: []FileFilter{filter}}); !errors.Is(err, platform.ErrUnsupported) {
			t.Fatalf("未知格式未报告能力缺失：%v", err)
		}
	}
}

func TestFileDialogClosesUndeliveredAccessAtLoopExit(t *testing.T) {
	loop := &settingsTestLoop{posts: make(chan func(), 8)}
	native := &dialogPendingNative{}
	dialog := newFileDialog(native, loop, nil).(*fileDialog)
	dialog.OpenFile(nil, DialogOptions{}, func([]FileAccess, error) { t.Fatal("退出后不应交付业务回调") })
	(<-loop.posts)()
	access := &dialogOwnedAccess{}
	native.complete([]FileAccess{access}, nil)
	if access.closed {
		t.Fatal("结果入队即被提前关闭")
	}
	dialog.close()
	if !access.closed {
		t.Fatal("事件循环丢弃的结果未释放")
	}
	(<-loop.posts)()
	// 另一次尚未完成的操作，在停止之后才第一次收到原生结果。
	native2 := &dialogPendingNative{}
	dialog2 := newFileDialog(native2, loop, nil).(*fileDialog)
	dialog2.OpenFile(nil, DialogOptions{}, func([]FileAccess, error) { t.Fatal("停止后不应交付业务回调") })
	(<-loop.posts)()
	dialog2.close()
	late := &dialogOwnedAccess{}
	native2.complete([]FileAccess{late}, nil)
	if !late.closed {
		t.Fatal("停止后迟到的访问资源未释放")
	}
}
