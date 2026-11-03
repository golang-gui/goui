package dev

import (
	"encoding/json"
	"errors"
	"net/http"

	"github.com/golang-gui/goui/gui"
	"github.com/golang-gui/goui/gui/diagnostics"
)

// DiagnosticsPath controls an optional GUI-thread capture. Merely importing
// dev, fetching a snapshot or polling this endpoint never enables collection.
const DiagnosticsPath = "/goui/dev/diagnostics"

type diagnosticsCapture interface {
	Snapshot() diagnostics.Snapshot
	Stop()
}

type diagnosticsHandler struct {
	host    *handler
	capture diagnosticsCapture // accessed only inside host.call
	start   func(gui.Application, diagnostics.Options) (diagnosticsCapture, error)
}

func (h *diagnosticsHandler) serveHTTP(w http.ResponseWriter, req *http.Request) {
	if req.Method != http.MethodGet && req.Method != http.MethodPost {
		w.Header().Set("Allow", "GET, POST")
		writeError(w, http.StatusMethodNotAllowed, errors.New("expected GET or POST"))
		return
	}
	app := h.host.appFn()
	if app == nil {
		writeError(w, http.StatusServiceUnavailable, errAppNotReady)
		return
	}
	var input struct {
		Enabled *bool `json:"enabled"`
		Widgets bool  `json:"widgets"`
	}
	if req.Method == http.MethodPost {
		defer req.Body.Close()
		decoder := json.NewDecoder(req.Body)
		decoder.DisallowUnknownFields()
		if err := decoder.Decode(&input); err != nil {
			writeError(w, http.StatusBadRequest, err)
			return
		}
		if input.Enabled == nil {
			writeError(w, http.StatusBadRequest, errors.New("enabled is required"))
			return
		}
	}
	var snapshot diagnostics.Snapshot
	err := h.host.call(req, app, func() error {
		if input.Enabled != nil {
			if *input.Enabled {
				if h.capture == nil || !h.capture.Snapshot().Active {
					capture, err := h.start(app, diagnostics.Options{Widgets: input.Widgets})
					if err != nil {
						return err
					}
					h.capture = capture
				}
			} else if h.capture != nil {
				h.capture.Stop()
			}
		}
		if h.capture != nil {
			snapshot = h.capture.Snapshot()
		}
		return nil
	})
	if err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	writeJSON(w, http.StatusOK, snapshot)
}
