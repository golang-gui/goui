package dev

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/golang-gui/goui/gui"
)

// Protocol endpoints served over loopback HTTP.
const (
	SnapshotPath = "/goui/dev/snapshot"
	EventPath    = "/goui/dev/event"
)

const defaultCallTimeout = 5 * time.Second

var (
	errAddrEmpty   = errors.New("goui dev addr is empty")
	errAppNotReady = errors.New("goui app is not ready")
)

// init starts the dev server when GOUI_DEV_ADDR is set. Importing this package for
// its side effect (import _ ".../gui/dev") is the opt-in; without the import,
// net/http is never linked into the binary. A bad address or taken port must not
// crash the app, so the error is swallowed.
func init() {
	if addr := os.Getenv("GOUI_DEV_ADDR"); addr != "" {
		// TODO: log the error once the framework has logging.
		_ = serve(addr)
	}
}

// serve starts the dev HTTP server on a loopback address. The running application
// is resolved from gui.App on demand, so the server may start before the app
// exists (it answers 503 until then).
func serve(addr string) error {
	addr, err := normalizeAddr(addr)
	if err != nil {
		return err
	}
	listener, err := net.Listen("tcp", addr)
	if err != nil {
		return fmt.Errorf("start goui dev server on %s: %w", addr, err)
	}
	server := &http.Server{
		Handler:           newHandler(func() gui.Application { return gui.App }),
		ReadHeaderTimeout: 5 * time.Second,
	}
	go func() { _ = server.Serve(listener) }()
	return nil
}

type handler struct {
	appFn       func() gui.Application
	callTimeout time.Duration
}

// newHandler builds the protocol handler. appFn resolves the running application on
// demand (nil until it exists); tests inject a stub through it.
func newHandler(appFn func() gui.Application) http.Handler {
	h := &handler{appFn: appFn, callTimeout: defaultCallTimeout}
	mux := http.NewServeMux()
	mux.HandleFunc(SnapshotPath, h.handleSnapshot)
	mux.HandleFunc(EventPath, h.handleEvent)
	return mux
}

type snapshotResponse struct {
	OK       bool                `json:"ok"`
	Snapshot gui.ApplicationInfo `json:"snapshot"`
}

type okResponse struct {
	OK bool `json:"ok"`
}

type errorResponse struct {
	OK    bool   `json:"ok"`
	Error string `json:"error"`
}

func (h *handler) handleSnapshot(w http.ResponseWriter, req *http.Request) {
	if req.Method != http.MethodGet {
		writeMethodNotAllowed(w, http.MethodGet)
		return
	}

	app := h.appFn()
	if app == nil {
		writeError(w, http.StatusServiceUnavailable, errAppNotReady)
		return
	}

	var snapshot gui.ApplicationInfo
	if err := h.call(req, app, func() error {
		snapshot = app.Snapshot()
		return nil
	}); err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}

	writeJSON(w, http.StatusOK, snapshotResponse{OK: true, Snapshot: snapshot})
}

func (h *handler) handleEvent(w http.ResponseWriter, req *http.Request) {
	if req.Method != http.MethodPost {
		writeMethodNotAllowed(w, http.MethodPost)
		return
	}

	app := h.appFn()
	if app == nil {
		writeError(w, http.StatusServiceUnavailable, errAppNotReady)
		return
	}
	defer req.Body.Close()

	var eventReq eventRequest
	if err := json.NewDecoder(req.Body).Decode(&eventReq); err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	if eventReq.Window == "" {
		writeError(w, http.StatusBadRequest, errWindowRequired)
		return
	}

	event, err := eventReq.event()
	if err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}

	if err := h.call(req, app, func() error {
		return app.DispatchWindowEvent(eventReq.Window, event)
	}); err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}

	writeJSON(w, http.StatusOK, okResponse{OK: true})
}

// call runs f on the UI thread (via app.Post) and waits, bounded by callTimeout.
func (h *handler) call(req *http.Request, app gui.Application, f func() error) error {
	ctx, cancel := context.WithTimeout(req.Context(), h.callTimeout)
	defer cancel()

	done := make(chan error, 1)
	app.Post(func() {
		done <- f()
	})

	select {
	case err := <-done:
		return err
	case <-ctx.Done():
		return ctx.Err()
	}
}

func writeMethodNotAllowed(w http.ResponseWriter, method string) {
	w.Header().Set("Allow", method)
	writeError(w, http.StatusMethodNotAllowed, fmt.Errorf("method not allowed"))
}

func writeError(w http.ResponseWriter, status int, err error) {
	message := ""
	if err != nil {
		message = err.Error()
	}
	writeJSON(w, status, errorResponse{OK: false, Error: message})
}

func writeJSON(w http.ResponseWriter, status int, value any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(value)
}

// normalizeAddr returns a loopback TCP address for the dev server. A bare port such
// as "8080" binds 127.0.0.1:8080; values that already carry a host, such as
// "localhost:8080", are used as-is.
func normalizeAddr(addr string) (string, error) {
	addr = strings.TrimSpace(addr)
	if addr == "" {
		return "", errAddrEmpty
	}
	if strings.HasPrefix(addr, ":") {
		return "127.0.0.1" + addr, nil
	}
	if strings.Count(addr, ":") == 0 {
		return net.JoinHostPort("127.0.0.1", addr), nil
	}
	return addr, nil
}
