package dev

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/golang-gui/goui/gui"
	"github.com/golang-gui/goui/gui/diagnostics"
)

type testDiagnosticsCapture struct {
	active bool
	stops  int
}

func (c *testDiagnosticsCapture) Snapshot() diagnostics.Snapshot {
	return diagnostics.Snapshot{Active: c.active}
}
func (c *testDiagnosticsCapture) Stop() { c.active = false; c.stops++ }

func TestDiagnosticsProtocolRunsOnlyThroughPost(t *testing.T) {
	app := newTestApplication()
	inside, posts, starts := false, 0, 0
	app.post = func(f func()) { posts++; inside = true; defer func() { inside = false }(); f() }
	var capture *testDiagnosticsCapture
	h := diagnosticsHandler{host: &handler{appFn: func() gui.Application { return app }, callTimeout: defaultCallTimeout}, start: func(a gui.Application, o diagnostics.Options) (diagnosticsCapture, error) {
		if !inside || a != app || !o.Widgets {
			t.Fatal("未通过 GUI 线程或选项丢失")
		}
		starts++
		capture = &testDiagnosticsCapture{active: true}
		return capture, nil
	}}
	call := func(method, body string) diagnostics.Snapshot {
		t.Helper()
		rec := httptest.NewRecorder()
		h.serveHTTP(rec, httptest.NewRequest(method, DiagnosticsPath, bytes.NewBufferString(body)))
		if rec.Code != http.StatusOK {
			t.Fatal(rec.Code, rec.Body.String())
		}
		var s diagnostics.Snapshot
		if err := json.Unmarshal(rec.Body.Bytes(), &s); err != nil {
			t.Fatal(err)
		}
		return s
	}
	if call(http.MethodGet, "").Active || starts != 0 {
		t.Fatal("查询隐式开启诊断")
	}
	if !call(http.MethodPost, `{"enabled":true,"widgets":true}`).Active || starts != 1 {
		t.Fatal("未显式开启")
	}
	call(http.MethodPost, `{"enabled":true,"widgets":true}`)
	if starts != 1 {
		t.Fatal("重复开启重置了采集")
	}
	if call(http.MethodPost, `{"enabled":false}`).Active || capture.stops != 1 {
		t.Fatal("未停止")
	}
	call(http.MethodGet, "")
	if starts != 1 || posts != 5 {
		t.Fatal("轮询改变了采集状态或未经过 Post", starts, posts)
	}
	if !call(http.MethodPost, `{"enabled":true,"widgets":true}`).Active || starts != 2 {
		t.Fatal("停止后无法新建采集")
	}
}

func TestDiagnosticsProtocolValidationAndNoImplicitCapture(t *testing.T) {
	app := newTestApplication()
	h := newHandler(func() gui.Application { return app })
	for _, tc := range []struct {
		method, body string
		status       int
	}{
		{http.MethodGet, "", http.StatusOK},
		{http.MethodPut, "", http.StatusMethodNotAllowed},
		{http.MethodPost, `{}`, http.StatusBadRequest},
		{http.MethodPost, `{"enabled":"yes"}`, http.StatusBadRequest},
		{http.MethodPost, `{"enabled":true,"unknown":1}`, http.StatusBadRequest},
		// 测试替身不支持 GOUI 内核诊断；必须报告错误，不修改输入或窗口。
		{http.MethodPost, `{"enabled":true}`, http.StatusBadRequest},
	} {
		r := httptest.NewRecorder()
		h.ServeHTTP(r, httptest.NewRequest(tc.method, DiagnosticsPath, bytes.NewBufferString(tc.body)))
		if r.Code != tc.status {
			t.Fatal(tc, r.Code, r.Body.String())
		}
	}
	if len(app.dispatches) != 0 {
		t.Fatal("诊断访问派发了输入")
	}
	r := httptest.NewRecorder()
	newHandler(func() gui.Application { return nil }).ServeHTTP(r, httptest.NewRequest(http.MethodGet, DiagnosticsPath, nil))
	if r.Code != http.StatusServiceUnavailable {
		t.Fatal("空应用未报告不可用")
	}
}
