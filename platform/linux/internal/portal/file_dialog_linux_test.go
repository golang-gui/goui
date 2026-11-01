package portal

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"os/exec"
	"path/filepath"
	"reflect"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/godbus/dbus/v5"
	"github.com/golang-gui/goui/platform/common"
)

func TestFileDialogParameters(t *testing.T) {
	opts := common.DialogOptions{AllowMultiple: true, InitialDir: "/tmp/目录", DefaultName: "a.txt",
		Filters: []common.FileFilter{{Name: "Images", Patterns: []string{"*.png", "*.jpg"}}}}
	for _, test := range []struct {
		action Action
		parent string
		method string
	}{
		{OpenFile, "x11:1234", "OpenFile"}, {OpenDirectory, "x11:1234", "OpenFile"}, {SaveFile, "", "SaveFile"},
	} {
		method, args, err := parameters(test.action, test.parent, opts)
		if err != nil || method != test.method || args["modal"].Value() != (test.parent != "") {
			t.Fatalf("参数映射错误: %s %v %v", method, args, err)
		}
		if !reflect.DeepEqual(args["current_folder"].Value(), append([]byte(opts.InitialDir), 0)) {
			t.Fatal("目录必须是带末尾 NUL 的字节数组")
		}
		if test.action != SaveFile && args["multiple"].Value() != (test.action == OpenFile) {
			t.Fatal("目录不能继承多选选项")
		}
		if test.action == SaveFile && args["current_name"].Value() != "a.txt" {
			t.Fatal("未传保存文件名")
		}
		if test.action != OpenDirectory && args["filters"].Signature().String() != "a(sa(us))" {
			t.Fatal("过滤器 D-Bus 签名错误")
		}
	}
	if _, _, err := parameters(OpenFile, "", common.DialogOptions{InitialDir: "bad\x00path"}); err == nil {
		t.Fatal("接受了包含 NUL 的路径")
	}
}

func TestFileDialogResponse(t *testing.T) {
	for _, test := range []struct {
		name  string
		uris  []string
		paths []string
	}{
		{"转义一次", []string{"file:///tmp/a%20b%2520%E4%B8%AD"}, []string{"/tmp/a b%20中"}},
		{"本地主机", []string{"file://localhost/tmp/a"}, []string{"/tmp/a"}},
		{"多选", []string{"file:///tmp/a", "file:///tmp/b"}, []string{"/tmp/a", "/tmp/b"}},
		{"非文件", []string{"https://example.org/a"}, nil},
		{"远程主机", []string{"file://remote/tmp/a"}, nil},
		{"相对路径", []string{"file:a"}, nil},
		{"空路径", []string{"file://localhost"}, nil},
		{"查询", []string{"file:///tmp/a?x"}, nil},
		{"空查询", []string{"file:///tmp/a?"}, nil},
		{"片段", []string{"file:///tmp/a#"}, nil},
		{"NUL", []string{"file:///tmp/a%00b"}, nil},
		{"部分失败", []string{"file:///tmp/a", "file://remote/b"}, nil},
		{"空结果", []string{}, nil},
	} {
		t.Run(test.name, func(t *testing.T) {
			paths, err := response(OpenFile, true, []any{uint32(0), map[string]dbus.Variant{"uris": dbus.MakeVariant(test.uris)}})
			if (err == nil) != (test.paths != nil) || !reflect.DeepEqual(paths, test.paths) {
				t.Fatalf("结果=%q 错误=%v 预期=%q", paths, err, test.paths)
			}
		})
	}
	for _, code := range []uint32{0, 1, 2, 99} {
		paths, err := response(OpenFile, false, []any{code, map[string]dbus.Variant{}})
		if paths != nil || (err == nil) != (code == 1) {
			t.Fatalf("取消与故障混淆: code=%d paths=%v err=%v", code, paths, err)
		}
	}
	for _, action := range []Action{OpenFile, OpenDirectory, SaveFile} {
		if _, err := response(action, action != OpenFile, []any{uint32(0), map[string]dbus.Variant{
			"uris": dbus.MakeVariant([]string{"file:///a", "file:///b"}),
		}}); err == nil {
			t.Fatal("单选操作接受了多个结果")
		}
	}
}

// 无需桌面：使用独立 session bus 和受控 Portal 服务验证真实协议与竞态。
func testBus(t *testing.T) string {
	t.Helper()
	path, err := exec.LookPath("dbus-daemon")
	if err != nil {
		t.Skip("未执行真实总线测试：缺少 dbus-daemon；纯参数／结果测试仍执行")
	}
	cmd := exec.Command(path, "--session", "--nofork", "--nopidfile", "--print-address=1")
	out, err := cmd.StdoutPipe()
	if err != nil {
		t.Fatal(err)
	}
	if err := cmd.Start(); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = cmd.Process.Kill(); _ = cmd.Wait() })
	address, err := bufio.NewReader(out).ReadString('\n')
	if err != nil {
		t.Fatal(err)
	}
	address = strings.TrimSpace(address)
	t.Setenv("DBUS_SESSION_BUS_ADDRESS", address)
	return address
}

type testSelection struct {
	path            dbus.ObjectPath
	parent          string
	method          string
	options         map[string]dbus.Variant
	filterSignature string
}

type testChooser struct {
	export  sync.Mutex // godbus 的动态对象导出不支持并发注册。
	filters sync.Map
	conn    *dbus.Conn
	calls   chan testSelection
	closed  chan dbus.ObjectPath
	gate    chan struct{}
	body    []any
	failure *dbus.Error
	foreign bool
}

func (s *testChooser) OpenFile(sender dbus.Sender, parent, title string, options map[string]dbus.Variant) (dbus.ObjectPath, *dbus.Error) {
	return s.open("OpenFile", sender, parent, options)
}

func (s *testChooser) SaveFile(sender dbus.Sender, parent, title string, options map[string]dbus.Variant) (dbus.ObjectPath, *dbus.Error) {
	return s.open("SaveFile", sender, parent, options)
}

func (s *testChooser) open(method string, sender dbus.Sender, parent string, options map[string]dbus.Variant) (dbus.ObjectPath, *dbus.Error) {
	token, _ := options["handle_token"].Value().(string)
	filterSignature, _ := s.filters.Load(token)
	signature, _ := filterSignature.(string)
	if s.foreign {
		token += "_actual"
	}
	path := dbus.ObjectPath(requestPrefix + strings.ReplaceAll(strings.TrimPrefix(string(sender), ":"), ".", "_") + "/" + token)
	s.export.Lock()
	err := s.conn.Export(&testRequest{s: s, path: path}, path, request)
	s.export.Unlock()
	if err != nil {
		return "", dbus.MakeFailedError(err)
	}
	s.calls <- testSelection{path: path, parent: parent, method: method, options: options, filterSignature: signature}
	if s.body != nil {
		if err := s.conn.Emit(path, request+".Response", s.body...); err != nil {
			return "", dbus.MakeFailedError(err)
		}
	}
	if s.gate != nil {
		<-s.gate
	}
	return path, s.failure
}

type testRequest struct {
	s    *testChooser
	path dbus.ObjectPath
}

func (r *testRequest) Close() *dbus.Error { r.s.closed <- r.path; return nil }

type testProperties struct{ version uint32 }

func (p *testProperties) Get(iface, name string) (dbus.Variant, *dbus.Error) {
	if iface != chooser || name != "version" {
		return dbus.Variant{}, dbus.NewError("org.freedesktop.DBus.Error.UnknownProperty", nil)
	}
	return dbus.MakeVariant(p.version), nil
}

func testService(t *testing.T, version uint32, setup func(*testChooser)) (*FileChooser, *testChooser) {
	t.Helper()
	testBus(t)
	s := &testChooser{calls: make(chan testSelection, 16), closed: make(chan dbus.ObjectPath, 16)}
	conn, err := dbus.ConnectSessionBus(dbus.WithIncomingInterceptor(func(message *dbus.Message) {
		// godbus 导出方法的反射转换会重新构造 Variant；在该转换前检查线上签名，
		// 避免将解码后的通用结构体切片推断为 aav，从而误判原始 a(sa(us))。
		if message.Type == dbus.TypeMethodCall && len(message.Body) == 3 {
			if options, ok := message.Body[2].(map[string]dbus.Variant); ok {
				if token, ok := options["handle_token"].Value().(string); ok {
					s.filters.Store(token, options["filters"].Signature().String())
				}
			}
		}
	}))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = conn.Close() })
	s.conn = conn
	if setup != nil {
		setup(s)
	}
	if err := conn.Export(s, desktopPath, chooser); err != nil {
		t.Fatal(err)
	}
	if err := conn.Export(&testProperties{version}, desktopPath, "org.freedesktop.DBus.Properties"); err != nil {
		t.Fatal(err)
	}
	if _, err := conn.RequestName(service, dbus.NameFlagDoNotQueue); err != nil {
		t.Fatal(err)
	}
	f := NewFileChooser()
	t.Cleanup(func() {
		f.Close()
		done := make(chan struct{})
		go func() { f.active.Wait(); close(done) }()
		select {
		case <-done:
		case <-time.After(12 * time.Second):
			t.Error("请求未按有界生命周期结束")
		}
	})
	return f, s
}

func testReceive[T any](t *testing.T, ch <-chan T) T {
	t.Helper()
	select {
	case value := <-ch:
		return value
	case <-time.After(5 * time.Second):
		t.Fatal("等待协议事件超时")
		var zero T
		return zero
	}
}

func TestFileDialogProtocol(t *testing.T) {
	for _, action := range []Action{OpenFile, OpenDirectory, SaveFile} {
		t.Run([]string{"文件", "目录", "保存"}[action], func(t *testing.T) {
			f, s := testService(t, 3, func(s *testChooser) {
				s.foreign = true // 实际句柄变化，且响应故意先于方法返回。
				s.body = []any{uint32(0), map[string]dbus.Variant{"uris": dbus.MakeVariant([]string{"file:///tmp/a%20b"})}}
			})
			paths, err := f.Open(context.Background(), action, "x11:abcd", common.DialogOptions{
				DefaultName: "target.txt", InitialDir: "/tmp", AllowMultiple: true,
				Filters: []common.FileFilter{{Name: "Text", Patterns: []string{"*.txt", "*.md"}}},
			})
			if err != nil || !reflect.DeepEqual(paths, []string{"/tmp/a b"}) {
				t.Fatalf("早到响应丢失或转换错误: %v %v", paths, err)
			}
			call := testReceive(t, s.calls)
			if call.parent != "x11:abcd" || call.options["modal"].Value() != true ||
				(call.method == "SaveFile") != (action == SaveFile) {
				t.Fatalf("原生参数错误: %+v", call)
			}
			var directory []byte
			if call.options["current_folder"].Store(&directory) != nil || string(directory) != "/tmp\x00" {
				t.Fatal("实际总线传递的目录字节错误")
			}
			if action != OpenDirectory && call.filterSignature != "a(sa(us))" {
				t.Fatalf("实际总线传递的过滤器签名错误: %s", call.filterSignature)
			}
		})
	}
}

func TestFileDialogUnavailableAndErrors(t *testing.T) {
	f, _ := testService(t, 2, nil)
	if _, err := f.Open(context.Background(), OpenDirectory, "", common.DialogOptions{}); !errors.Is(err, ErrUnsupported) {
		t.Fatalf("目录能力缺失应允许降级: %v", err)
	}
	for _, code := range []uint32{1, 2} {
		t.Run(string(rune('0'+code)), func(t *testing.T) {
			f, s := testService(t, 3, func(s *testChooser) { s.body = []any{code, map[string]dbus.Variant{}} })
			paths, err := f.Open(context.Background(), OpenFile, "", common.DialogOptions{})
			if paths != nil || (err == nil) != (code == 1) || errors.Is(err, ErrUnsupported) {
				t.Fatalf("取消／拒绝不应降级: %v %v", paths, err)
			}
			if testReceive(t, s.calls).options["modal"].Value() != false {
				t.Fatal("无 owner 请求必须显式关闭模态")
			}
		})
	}
}

func TestFileDialogCancelAndClose(t *testing.T) {
	for _, late := range []bool{false, true} {
		t.Run(map[bool]string{false: "等待响应", true: "迟到句柄"}[late], func(t *testing.T) {
			f, s := testService(t, 3, func(s *testChooser) {
				if late {
					s.gate = make(chan struct{})
				}
			})
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			done := make(chan error, 1)
			go func() { _, err := f.Open(ctx, OpenFile, "x11:1", common.DialogOptions{}); done <- err }()
			call := testReceive(t, s.calls)
			cancel()
			if got := testReceive(t, s.closed); got != call.path {
				t.Fatal("取消了其他请求")
			}
			if late {
				close(s.gate)
			}
			if err := testReceive(t, done); !errors.Is(err, context.Canceled) {
				t.Fatalf("取消结果错误: %v", err)
			}
		})
	}
	f, s := testService(t, 3, nil)
	done := make(chan error, 1)
	go func() { _, err := f.Open(context.Background(), OpenFile, "", common.DialogOptions{}); done <- err }()
	call := testReceive(t, s.calls)
	f.Close()
	if testReceive(t, s.closed) != call.path || !errors.Is(testReceive(t, done), common.ErrUnavailable) {
		t.Fatal("关闭服务未结束远端请求")
	}
	f.Close()
	if _, err := f.Open(context.Background(), OpenFile, "", common.DialogOptions{}); !errors.Is(err, common.ErrUnavailable) {
		t.Fatal("关闭后仍受理请求")
	}
}

func TestFileDialogServiceLossAndConcurrentRequests(t *testing.T) {
	f, s := testService(t, 3, nil)
	var wg sync.WaitGroup
	done := make(chan error, 2)
	for i := 0; i < 2; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_, err := f.Open(context.Background(), OpenFile, "", common.DialogOptions{})
			done <- err
		}()
	}
	first, second := testReceive(t, s.calls), testReceive(t, s.calls)
	if first.path == second.path {
		t.Fatal("并发请求复用了标识")
	}
	if _, err := s.conn.ReleaseName(service); err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 2; i++ {
		if err := testReceive(t, done); err == nil || errors.Is(err, ErrUnsupported) {
			t.Fatalf("服务退出不得重放为 helper: %v", err)
		}
	}
	wg.Wait()
}

func TestFileDialogRecoveryAndRejection(t *testing.T) {
	f := NewFileChooser()
	t.Cleanup(f.Close)
	t.Setenv("DBUS_SESSION_BUS_ADDRESS", "unix:path="+filepath.Join(t.TempDir(), "missing"))
	if _, err := f.Open(context.Background(), OpenFile, "", common.DialogOptions{}); !errors.Is(err, ErrUnsupported) {
		t.Fatalf("连接缺失应允许降级: %v", err)
	}
	_, s := testService(t, 3, func(s *testChooser) {
		s.body = []any{uint32(0), map[string]dbus.Variant{"uris": dbus.MakeVariant([]string{"file:///tmp/recovered"})}}
	})
	paths, err := f.Open(context.Background(), OpenFile, "", common.DialogOptions{})
	if err != nil || !reflect.DeepEqual(paths, []string{"/tmp/recovered"}) {
		t.Fatalf("不应永久缓存不可用: %v %v", paths, err)
	}
	_ = testReceive(t, s.calls)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := f.Open(ctx, OpenFile, "", common.DialogOptions{}); !errors.Is(err, context.Canceled) {
		t.Fatal("启动前取消未生效")
	}
	select {
	case <-s.calls:
		t.Fatal("已取消的请求仍调用了 Portal")
	default:
	}
	t.Run("服务拒绝不得降级", func(t *testing.T) {
		f, _ := testService(t, 3, func(s *testChooser) {
			s.failure = dbus.NewError("org.freedesktop.DBus.Error.AccessDenied", []any{"denied"})
		})
		if _, err := f.Open(context.Background(), OpenFile, "", common.DialogOptions{}); err == nil || errors.Is(err, ErrUnsupported) {
			t.Fatalf("拒绝错误不能转为 helper: %v", err)
		}
	})
	t.Run("不确定超时不得降级", func(t *testing.T) {
		f, _ := testService(t, 3, func(s *testChooser) {
			s.failure = dbus.NewError("org.freedesktop.DBus.Error.NoReply", []any{"timeout"})
		})
		if _, err := f.Open(context.Background(), OpenFile, "", common.DialogOptions{}); err == nil || errors.Is(err, ErrUnsupported) {
			t.Fatalf("不确定是否已显示时不能重新弹窗: %v", err)
		}
	})
}

func TestFileDialogResponseRoutingAndDisconnect(t *testing.T) {
	f, s := testService(t, 3, nil)
	type result struct {
		parent string
		paths  []string
		err    error
	}
	done := make(chan result, 2)
	for i := 1; i <= 2; i++ {
		parent := fmt.Sprintf("x11:%d", i)
		go func() {
			paths, err := f.Open(context.Background(), OpenFile, parent, common.DialogOptions{})
			done <- result{parent, paths, err}
		}()
	}
	first, second := testReceive(t, s.calls), testReceive(t, s.calls)
	// 响应顺序反转，并插入同一服务发出的无关响应。
	for _, call := range []testSelection{{path: dbus.ObjectPath(requestPrefix + "unrelated")}, second, first} {
		err := s.conn.Emit(call.path, request+".Response", uint32(0), map[string]dbus.Variant{
			"uris": dbus.MakeVariant([]string{"file:///tmp/" + call.parent}),
		})
		if err != nil {
			t.Fatal(err)
		}
	}
	for i := 0; i < 2; i++ {
		got := testReceive(t, done)
		if got.err != nil || !reflect.DeepEqual(got.paths, []string{"/tmp/" + got.parent}) {
			t.Fatalf("并发响应串线: %+v", got)
		}
	}
	go func() {
		_, err := f.Open(context.Background(), OpenFile, "", common.DialogOptions{})
		done <- result{err: err}
	}()
	_ = testReceive(t, s.calls)
	conn, err := f.connection(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	_ = conn.Close()
	if err := testReceive(t, done).err; err == nil || errors.Is(err, ErrUnsupported) {
		t.Fatalf("已受理请求断线不得降级: %v", err)
	}
}
