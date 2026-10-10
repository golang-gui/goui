package dev

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/golang-gui/goui/core/geometry"
	"github.com/golang-gui/goui/core/signal"
	"github.com/golang-gui/goui/gui"
	"github.com/golang-gui/goui/platform/events"
)

func TestExplicitModifierAndKeyNames(t *testing.T) {
	for _, tt := range []struct {
		name     string
		modifier events.Modifiers
		key      events.Key
	}{
		{"ctrl", events.ModifierControl, events.KeyControl},
		{"option", events.ModifierOption, events.KeyOption},
		{"altgraph", events.ModifierAltGraph, events.KeyAltGraph},
		{"altgr", events.ModifierAltGraph, events.KeyAltGraph},
		{"alt", events.ModifierAlt, events.KeyAlt},
		{"command", events.ModifierCommand, events.KeyCommand},
		{"cmd", events.ModifierCommand, events.KeyCommand},
		{"super", events.ModifierSuper, events.KeySuper},
		{"win", events.ModifierWin, events.KeyWin},
		{"meta", events.ModifierSuper, events.KeySuper},
	} {
		modifier, err := modifierByName(tt.name)
		if err != nil || modifier != tt.modifier {
			t.Fatalf("modifier %s: %v, %v", tt.name, modifier, err)
		}
		key, err := keyByName(tt.name)
		if err != nil || key != tt.key {
			t.Fatalf("key %s: %v, %v", tt.name, key, err)
		}
	}
	if _, err := modifierByName("primary"); err == nil {
		t.Fatal("platform input protocol must not infer Primary")
	}
}

func TestNormalizeAddr(t *testing.T) {
	addr, err := normalizeAddr("8080")
	if err != nil {
		t.Fatal(err)
	}
	if addr != "127.0.0.1:8080" {
		t.Fatalf("unexpected addr %q", addr)
	}

	addr, err = normalizeAddr(":9090")
	if err != nil {
		t.Fatal(err)
	}
	if addr != "127.0.0.1:9090" {
		t.Fatalf("unexpected localhost addr %q", addr)
	}

	addr, err = normalizeAddr("localhost:7070")
	if err != nil {
		t.Fatal(err)
	}
	if addr != "localhost:7070" {
		t.Fatalf("unexpected explicit addr %q", addr)
	}

	_, err = normalizeAddr("")
	if !errors.Is(err, errAddrEmpty) {
		t.Fatalf("expected errAddrEmpty, got %v", err)
	}
}

func TestHandlerSnapshotPreservesGUISemantics(t *testing.T) {
	app := newTestApplication()
	app.windows[0].(*testWindow).info = gui.WindowInfo{
		ID:     "main",
		Title:  "Main",
		Bounds: geometry.Rect(0, 0, 320, 240),
		Widget: gui.WidgetInfo{
			ID:      "save",
			Role:    gui.RoleButton,
			Text:    "Save",
			Bounds:  geometry.Rect(12, 20, 80, 32),
			Visible: true,
			Enabled: true,
			Actions: []gui.Action{gui.ActionClick},
		},
	}
	app.windows[0].(*testWindow).info.Widget.SetAttribute("acme.record", struct {
		Count int `json:"count"`
	}{Count: 3})

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, SnapshotPath, nil)
	newHandler(func() gui.Application { return app }).ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("unexpected status %d: %s", rec.Code, rec.Body.String())
	}
	body := rec.Body.Bytes()
	if !bytes.Contains(body, []byte(`"snapshot"`)) || !bytes.Contains(body, []byte(`"windows"`)) || bytes.Contains(body, []byte(`"Windows"`)) {
		t.Fatalf("snapshot JSON should use protocol field names: %s", string(body))
	}

	var response snapshotResponse
	if err := json.NewDecoder(bytes.NewReader(body)).Decode(&response); err != nil {
		t.Fatal(err)
	}
	if !response.OK {
		t.Fatal("snapshot response was not ok")
	}
	if got := response.Snapshot.Windows[0].Widget.Bounds.X; got != 12 {
		t.Fatalf("unexpected widget bounds x %v", got)
	}
	attribute, ok := response.Snapshot.Windows[0].Widget.Attributes["acme.record"].(map[string]any)
	if !ok || attribute["count"] != float64(3) {
		t.Fatalf("unknown attribute was discarded or incorrectly decoded: %+v", attribute)
	}
}

func TestHandlerSnapshotEncodingFailure(t *testing.T) {
	app := newTestApplication()
	widget := gui.WidgetInfo{ID: "invalid"}
	widget.SetAttribute("acme.failure", failingResponseValue{})
	app.windows[0].(*testWindow).info = gui.WindowInfo{ID: "main", Widget: widget}
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, SnapshotPath, nil)
	newHandler(func() gui.Application { return app }).ServeHTTP(rec, req)
	var response errorResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &response); err != nil {
		t.Fatal(err)
	}
	if rec.Code != http.StatusInternalServerError || response.OK || response.Error == "" || bytes.Contains(rec.Body.Bytes(), []byte(`"snapshot"`)) {
		t.Fatalf("encoding failure was reported as a successful snapshot: %d %s", rec.Code, rec.Body.String())
	}
}

type failingResponseValue struct{}

func (failingResponseValue) MarshalJSON() ([]byte, error) {
	return nil, errors.New("test encoding failure")
}

func TestWriteJSONEncodingFailure(t *testing.T) {
	cycle := make(map[string]any)
	cycle["self"] = cycle
	for _, tt := range []struct {
		name  string
		value any
	}{
		{"function", func() {}},
		{"cycle", cycle},
		{"nan", math.NaN()},
		{"infinity", math.Inf(1)},
		{"custom marshaler", failingResponseValue{}},
	} {
		t.Run(tt.name, func(t *testing.T) {
			rec := httptest.NewRecorder()
			writeJSON(rec, http.StatusOK, struct {
				OK    bool `json:"ok"`
				Value any  `json:"value"`
			}{OK: true, Value: tt.value})
			if rec.Code != http.StatusInternalServerError {
				t.Fatalf("status=%d body=%q; want 500 with a complete error response", rec.Code, rec.Body.String())
			}
			if rec.Header().Get("Content-Type") != "application/json" {
				t.Fatal("encoding failure lost JSON content type")
			}
			var body map[string]json.RawMessage
			if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
				t.Fatalf("incomplete error JSON: %v; body=%q", err, rec.Body.String())
			}
			var message string
			if string(body["ok"]) != "false" || json.Unmarshal(body["error"], &message) != nil || message == "" || len(body) != 2 {
				t.Fatalf("unexpected error response: %s", rec.Body.String())
			}
		})
	}
}

func TestHandlerDispatchesPointerEvent(t *testing.T) {
	app := newTestApplication()
	handler := newHandler(func() gui.Application { return app })
	snapshot := readSnapshot(t, handler)

	body := bytes.NewBufferString(fmt.Sprintf(`{
		"target": %q,
		"type": "pointer_down",
		"x": 12.5,
		"y": 34.5,
		"button": "left",
		"buttons": ["left"],
		"modifiers": ["shift", "ctrl"]
	}`, snapshot.Windows[0].Ref))
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, EventPath, body)
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("unexpected status %d: %s", rec.Code, rec.Body.String())
	}
	if len(app.dispatches) != 1 {
		t.Fatalf("expected one dispatched event, got %d", len(app.dispatches))
	}
	if app.dispatches[0].window != "main" {
		t.Fatalf("unexpected dispatch window %q", app.dispatches[0].window)
	}
	event, ok := app.dispatches[0].event.(events.PointerEvent)
	if !ok {
		t.Fatalf("dispatched %T, want events.PointerEvent", app.dispatches[0].event)
	}
	if event.EventType != events.PointerDown || event.Position.X != 12.5 || event.Position.Y != 34.5 {
		t.Fatalf("unexpected pointer event: %+v", event)
	}
	if event.Button != events.PointerButtonLeft || event.Buttons != events.PointerButtonLeftDown {
		t.Fatalf("unexpected pointer buttons: button=%v buttons=%v", event.Button, event.Buttons)
	}
	if event.Modifiers != events.ModifierShift|events.ModifierControl {
		t.Fatalf("unexpected modifiers %v", event.Modifiers)
	}
}

func TestEventRequestParsesKeyEvent(t *testing.T) {
	event, err := eventRequest{
		Type:      "key_down",
		Key:       json.RawMessage(`"A"`),
		Location:  "left",
		Modifiers: json.RawMessage(`["shift"]`),
		Repeat:    true,
	}.event()
	if err != nil {
		t.Fatal(err)
	}

	keyEvent, ok := event.(events.KeyEvent)
	if !ok {
		t.Fatalf("parsed %T, want events.KeyEvent", event)
	}
	if keyEvent.EventType != events.KeyDown || keyEvent.Key != events.KeyA {
		t.Fatalf("unexpected key event: %+v", keyEvent)
	}
	if keyEvent.Location != events.KeyLocationLeft || keyEvent.Modifiers != events.ModifierShift || !keyEvent.Repeat {
		t.Fatalf("unexpected key event details: %+v", keyEvent)
	}
}

type testApplication struct {
	gui.Application
	windows     []gui.Window
	dispatches  []testDispatch
	dispatchErr error
	post        func(func())
}

type testDispatch struct {
	window string
	event  events.Event
}

func newTestApplication() *testApplication {
	a := &testApplication{}
	a.windows = []gui.Window{&testWindow{app: a, info: gui.WindowInfo{ID: "main"}, widget: gui.NewButton()}}
	return a
}

func (a *testApplication) Post(task func()) {
	if a.post != nil {
		a.post(task)
		return
	}
	task()
}

func (a *testApplication) Windows() []gui.Window {
	return append([]gui.Window(nil), a.windows...)
}

func (a *testApplication) Snapshot() gui.ApplicationInfo {
	panic("dev must enumerate public windows, not call Application.Snapshot")
}

type testWindow struct {
	gui.Window
	app       *testApplication
	info      gui.WindowInfo
	widget    gui.Widget
	popovers  []gui.Popover
	destroy   signal.Signal0
	commits   []string
	commitErr error
}

func (w *testWindow) Snapshot() gui.WindowInfo               { return w.info }
func (w *testWindow) Widget() gui.Widget                     { return w.widget }
func (w *testWindow) Popovers() []gui.Popover                { return append([]gui.Popover(nil), w.popovers...) }
func (w *testWindow) ConnectDestroy(fn func()) signal.Handle { return w.destroy.Connect(fn) }
func (w *testWindow) CommitText(text string) error {
	if w.commitErr != nil {
		return w.commitErr
	}
	w.commits = append(w.commits, text)
	return nil
}
func (w *testWindow) DispatchEvent(event events.Event) error {
	w.app.dispatches = append(w.app.dispatches, testDispatch{
		window: w.info.ID,
		event:  event,
	})
	return w.app.dispatchErr
}

func readSnapshot(t *testing.T, handler http.Handler) applicationSnapshot {
	t.Helper()
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, SnapshotPath, nil))
	if rec.Code != http.StatusOK {
		t.Fatalf("snapshot: %d %s", rec.Code, rec.Body.String())
	}
	var response snapshotResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &response); err != nil {
		t.Fatal(err)
	}
	return response.Snapshot
}

func postInput(t *testing.T, handler http.Handler, path string, body any, want int) {
	t.Helper()
	data, err := json.Marshal(body)
	if err != nil {
		t.Fatal(err)
	}
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, httptest.NewRequest(http.MethodPost, path, bytes.NewReader(data)))
	if rec.Code != want {
		t.Fatalf("%s: status=%d, want=%d; %s", path, rec.Code, want, rec.Body.String())
	}
	var response map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &response); err != nil {
		t.Fatal(err)
	}
	if response["ok"] != (want == http.StatusOK) {
		t.Fatalf("wrong success flag: %s", rec.Body.String())
	}
}

func TestReferencesIndependentOfGUIIDs(t *testing.T) {
	app := newTestApplication()
	first := app.windows[0].(*testWindow)
	first.info.ID = ""
	second := &testWindow{app: app, info: gui.WindowInfo{ID: ""}, widget: gui.NewButton()}
	app.windows = append(app.windows, second)
	h := newHandler(func() gui.Application { return app })
	snapshot := readSnapshot(t, h)
	firstRef, secondRef := snapshot.Windows[0].Ref, snapshot.Windows[1].Ref
	if firstRef == "" || firstRef == secondRef || firstRef == snapshot.Windows[0].Widget.Ref {
		t.Fatal("missing or conflicting runtime refs")
	}
	for _, id := range []string{"duplicate", "renamed"} {
		first.info.ID, second.info.ID = id, id
		postInput(t, h, TextPath, map[string]any{"target": secondRef, "text": id}, http.StatusOK)
		next := readSnapshot(t, h)
		if next.Windows[0].Ref != firstRef || next.Windows[1].Ref != secondRef || next.Windows[0].ID != id {
			t.Fatal("GUI ID rename changed protocol identity")
		}
		if next.Windows[0].Widget.Ref == snapshot.Windows[0].Widget.Ref {
			t.Fatal("semantic nodes must not pretend to have permanent object identity")
		}
	}
	if len(first.commits) != 0 || len(second.commits) != 2 || first.widget.ID() != "" {
		t.Fatal("ref lookup used GUI IDs or wrote an automatic ID back to a widget")
	}
	app.windows = []gui.Window{second}
	postInput(t, h, TextPath, map[string]any{"target": firstRef, "text": "stale"}, http.StatusBadRequest)
	third := &testWindow{app: app, widget: gui.NewButton()}
	app.windows = append(app.windows, third)
	if readSnapshot(t, h).Windows[1].Ref == firstRef {
		t.Fatal("new window reused a dead window reference")
	}
}

type testPopover struct {
	gui.Popover
	visible bool
	modal   bool
	widget  gui.Widget
	closed  signal.Signal0
	events  []events.Event
	commits []string
}

func (p *testPopover) Visible() bool                         { return p.visible }
func (p *testPopover) Modal() bool                           { return p.modal }
func (p *testPopover) Widget() gui.Widget                    { return p.widget }
func (p *testPopover) ConnectClosed(fn func()) signal.Handle { return p.closed.Connect(fn) }
func (p *testPopover) DispatchEvent(event events.Event) error {
	p.events = append(p.events, event)
	return nil
}
func (p *testPopover) CommitText(text string) error { p.commits = append(p.commits, text); return nil }

func TestPopoverReferencesAndInputRouting(t *testing.T) {
	app := newTestApplication()
	w := app.windows[0].(*testWindow)
	p := &testPopover{visible: true, modal: true, widget: gui.NewLabel("popup")}
	w.popovers = []gui.Popover{p}
	h := newHandler(func() gui.Application { return app })
	snapshot := readSnapshot(t, h)
	popup := snapshot.Windows[0].Popovers[0]
	postInput(t, h, EventPath, map[string]any{"target": popup.Ref, "type": "pointer_down", "x": 12, "y": 20}, http.StatusOK)
	postInput(t, h, EventPath, map[string]any{"target": popup.Ref, "type": "wheel", "deltaY": 3}, http.StatusOK)
	postInput(t, h, EventPath, map[string]any{"target": popup.Ref, "type": "key_down", "key": "down"}, http.StatusOK)
	postInput(t, h, TextPath, map[string]any{"target": popup.Ref, "text": "你👋"}, http.StatusOK)
	if len(p.events) != 3 || len(p.commits) != 1 || p.commits[0] != "你👋" || len(app.dispatches) != 0 {
		t.Fatal("popover requests did not use the selected public host input path")
	}
	if e := p.events[0].(events.PointerEvent); e.Position != (geometry.Point{X: 12, Y: 20}) {
		t.Fatal("popover pointer coordinates were changed")
	}
	if e := p.events[2].(events.KeyEvent); e.Key != events.KeyArrowDown {
		t.Fatal("popover keyboard did not enter the selected popup dispatch")
	}
	p.modal = false
	postInput(t, h, EventPath, map[string]any{"target": popup.Ref, "type": "key_down", "key": "a"}, http.StatusBadRequest)
	// Hide and reopen the same object between requests: the old ref must not
	// silently acquire a new presentation, even though enumeration sees it again.
	p.visible = false
	p.closed.Emit()
	p.visible = true
	postInput(t, h, TextPath, map[string]any{"target": popup.Ref, "text": "stale"}, http.StatusBadRequest)
	if next := readSnapshot(t, h).Windows[0].Popovers[0]; next.Ref == popup.Ref {
		t.Fatal("reopened popup reused a closed presentation ref")
	}
	p.visible = false // removal is checked even without a notification
	if len(readSnapshot(t, h).Windows[0].Popovers) != 0 {
		t.Fatal("hidden popup remained in snapshot")
	}
}

func TestSnapshotReferencesPreserveFilteredSemantics(t *testing.T) {
	app := newTestApplication()
	w := app.windows[0].(*testWindow)
	// Actual Children differs deliberately from this semantic tree. No guessing
	// correspondence, no reconstruction that drops attributes or special roles.
	w.info.Widget = gui.WidgetInfo{Role: gui.RoleScrollView, Children: []gui.WidgetInfo{
		{Role: gui.RoleMenuItem, Text: "choice", ID: "repeated"},
		{Role: gui.RoleMenuItem, Text: "choice", ID: "repeated"},
	}}
	w.info.Controls = &gui.WidgetInfo{Role: gui.RoleButton, Name: "window-close"}
	h := newHandler(func() gui.Application { return app })
	snapshot := readSnapshot(t, h)
	root := snapshot.Windows[0]
	seen := make(map[string]bool)
	for _, ref := range []string{root.Ref, root.Widget.Ref, root.Controls.Ref, root.Widget.Children[0].Ref, root.Widget.Children[1].Ref} {
		if ref == "" || seen[ref] {
			t.Fatal("every semantic node must have a distinct ref")
		}
		seen[ref] = true
	}
	if len(root.Widget.Children) != 2 || root.Widget.Children[0].Role != gui.RoleMenuItem || root.Controls.Name != "window-close" || len(w.info.Widget.Children) != 2 {
		t.Fatal("snapshot wrapping changed semantic filtering or original data")
	}
	postInput(t, h, EventPath, map[string]any{"target": root.Widget.Ref, "type": "pointer_down"}, http.StatusBadRequest)
}

func TestTextRequestValidationAndErrors(t *testing.T) {
	app := newTestApplication()
	w := app.windows[0].(*testWindow)
	h := newHandler(func() gui.Application { return app })
	ref := readSnapshot(t, h).Windows[0].Ref
	for _, body := range []any{
		map[string]any{"text": "missing target"},
		map[string]any{"target": ref},
		map[string]any{"target": ref, "text": nil},
		map[string]any{"target": "unknown", "text": "wrong ref"},
		map[string]any{"window": "main", "text": "old addressing"},
	} {
		postInput(t, h, TextPath, body, http.StatusBadRequest)
	}
	postInput(t, h, TextPath, map[string]any{"target": ref, "text": ""}, http.StatusOK)
	w.commitErr = errors.New("no editable focus")
	postInput(t, h, TextPath, map[string]any{"target": ref, "text": "late"}, http.StatusBadRequest)
	if len(w.commits) != 1 || w.commits[0] != "" {
		t.Fatal("invalid text request was dispatched")
	}
	w.destroy.Emit()
	app.windows = nil
	postInput(t, h, TextPath, map[string]any{"target": ref, "text": "destroyed"}, http.StatusBadRequest)
}

func TestCanceledQueuedCallDoesNotExecute(t *testing.T) {
	for _, mode := range []string{"cancel", "timeout"} {
		t.Run(mode, func(t *testing.T) {
			app := newTestApplication()
			queued := make(chan func(), 1)
			app.post = func(fn func()) { queued <- fn }
			h := &handler{callTimeout: time.Second}
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			if mode == "timeout" {
				h.callTimeout = 10 * time.Millisecond
			}
			request := httptest.NewRequest(http.MethodPost, TextPath, nil).WithContext(ctx)
			done := make(chan error, 1)
			calls := 0
			go func() { done <- h.call(request, app, func() error { calls++; return nil }) }()
			task := <-queued
			if mode == "cancel" {
				cancel()
			}
			if err := <-done; err == nil {
				t.Fatal("queued request did not fail")
			}
			task() // Event loop finally catches up, after the HTTP request is gone.
			if calls != 0 {
				t.Fatal("canceled request performed late GUI input")
			}
		})
	}
}
