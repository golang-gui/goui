package dev

import (
	"bytes"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/golang-gui/goui/core/geometry"
	"github.com/golang-gui/goui/gui"
	"github.com/golang-gui/goui/platform"
	"github.com/golang-gui/goui/platform/events"
	"github.com/golang-gui/goui/platform/typography"
	"github.com/golang-gui/goui/style"
)

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

func TestHandlerSnapshotReturnsApplicationSnapshot(t *testing.T) {
	app := newTestApplication()
	app.snapshot = gui.ApplicationInfo{
		Windows: []gui.WindowInfo{{
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
		}},
	}

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
}

func TestHandlerDispatchesPointerEvent(t *testing.T) {
	app := newTestApplication()

	body := bytes.NewBufferString(`{
		"window": "main",
		"type": "pointer_down",
		"x": 12.5,
		"y": 34.5,
		"button": "left",
		"buttons": ["left"],
		"modifiers": ["shift", "ctrl"]
	}`)
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, EventPath, body)
	newHandler(func() gui.Application { return app }).ServeHTTP(rec, req)

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
	snapshot    gui.ApplicationInfo
	dispatches  []testDispatch
	dispatchErr error
}

func (a *testApplication) Clipboard() gui.Clipboard {
	panic("unimplemented")
}

func (a *testApplication) Settings() gui.Settings {
	panic("unimplemented")
}

type testDispatch struct {
	window string
	event  events.Event
}

func newTestApplication() *testApplication {
	return new(testApplication)
}

func (a *testApplication) Platform() platform.Platform {
	return nil
}

func (a *testApplication) Typography() typography.Context {
	return nil
}

func (a *testApplication) StyleSheet() style.StyleSheet {
	return nil
}

func (a *testApplication) SetStyleSheet(style.StyleSheet) {}

func (a *testApplication) QuitOnLastWindowClosed() bool { return true }

func (a *testApplication) SetQuitOnLastWindowClosed(bool) {}

func (a *testApplication) NewWindow() (gui.Window, error) {
	return nil, nil
}

func (a *testApplication) Run() {}

func (a *testApplication) Quit() {}

func (a *testApplication) Post(task func()) {
	task()
}

func (a *testApplication) Windows() []gui.Window {
	return nil
}

func (a *testApplication) Snapshot() gui.ApplicationInfo {
	return a.snapshot
}

func (a *testApplication) DispatchWindowEvent(window string, event events.Event) error {
	a.dispatches = append(a.dispatches, testDispatch{
		window: window,
		event:  event,
	})
	return a.dispatchErr
}
