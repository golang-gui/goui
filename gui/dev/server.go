// Package dev exposes GOUI's local development protocol.
package dev

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"net/http"
	"strings"
	"time"

	"github.com/golang-gui/goui/gui"
)

const (
	SnapshotPath = "/goui/dev/snapshot"
	EventPath    = "/goui/dev/event"

	defaultCallTimeout = 5 * time.Second
)

var ErrAddrEmpty = errors.New("goui dev addr is empty")

type Server struct {
	server   *http.Server
	listener net.Listener
}

type handler struct {
	app         gui.Application
	callTimeout time.Duration
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

// ListenAndServe starts a local HTTP development protocol server for app.
func ListenAndServe(app gui.Application, addr string) (*Server, error) {
	if app == nil {
		return nil, gui.ErrAppNil
	}

	addr, err := normalizeAddr(addr)
	if err != nil {
		return nil, err
	}

	listener, err := net.Listen("tcp", addr)
	if err != nil {
		return nil, fmt.Errorf("start goui dev server on %s: %w", addr, err)
	}

	server := &http.Server{
		Handler:           NewHandler(app),
		ReadHeaderTimeout: 5 * time.Second,
	}
	dev := &Server{
		server:   server,
		listener: listener,
	}

	go func() {
		_ = server.Serve(listener)
	}()

	return dev, nil
}

// NewHandler returns an HTTP handler for GOUI's development protocol.
func NewHandler(app gui.Application) http.Handler {
	h := &handler{
		app:         app,
		callTimeout: defaultCallTimeout,
	}
	mux := http.NewServeMux()
	mux.HandleFunc(SnapshotPath, h.handleSnapshot)
	mux.HandleFunc(EventPath, h.handleEvent)
	return mux
}

// Close stops the dev server.
func (s *Server) Close() error {
	if s == nil || s.server == nil {
		return nil
	}
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	return s.server.Shutdown(ctx)
}

func (h *handler) handleSnapshot(w http.ResponseWriter, req *http.Request) {
	if req.Method != http.MethodGet {
		writeMethodNotAllowed(w, http.MethodGet)
		return
	}

	var snapshot gui.ApplicationInfo
	if err := h.call(req, func() error {
		snapshot = h.app.Snapshot()
		return nil
	}); err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}

	writeJSON(w, http.StatusOK, snapshotResponse{
		OK:       true,
		Snapshot: snapshot,
	})
}

func (h *handler) handleEvent(w http.ResponseWriter, req *http.Request) {
	if req.Method != http.MethodPost {
		writeMethodNotAllowed(w, http.MethodPost)
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

	if err := h.call(req, func() error {
		return h.app.DispatchWindowEvent(eventReq.Window, event)
	}); err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}

	writeJSON(w, http.StatusOK, okResponse{OK: true})
}

func (h *handler) call(req *http.Request, f func() error) error {
	if h.app == nil {
		return gui.ErrAppNil
	}
	ctx, cancel := context.WithTimeout(req.Context(), h.callTimeout)
	defer cancel()

	done := make(chan error, 1)
	h.app.Post(func() {
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
	writeJSON(w, status, errorResponse{
		OK:    false,
		Error: message,
	})
}

func writeJSON(w http.ResponseWriter, status int, value any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(value)
}

// normalizeAddr returns a loopback TCP address suitable for the dev server.
//
// A bare port such as "8080" binds to 127.0.0.1:8080. Values that already
// contain a host, such as "localhost:8080", are used as-is.
func normalizeAddr(addr string) (string, error) {
	addr = strings.TrimSpace(addr)
	if addr == "" {
		return "", ErrAddrEmpty
	}
	if strings.HasPrefix(addr, ":") {
		return "127.0.0.1" + addr, nil
	}
	if strings.Count(addr, ":") == 0 {
		return net.JoinHostPort("127.0.0.1", addr), nil
	}
	return addr, nil
}
