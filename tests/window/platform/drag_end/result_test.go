package main

import (
	"errors"
	"testing"

	"github.com/golang-gui/goui/platform/dragdrop"
)

func TestVerifyResult(t *testing.T) {
	for _, tt := range []struct {
		name, expect        string
		begins, ends, drops int
		accepted, destroyed bool
		result              dragdrop.Result
		ok                  bool
	}{
		{"accepted at origin", "accepted", 1, 1, 1, true, false, dragdrop.Result{Action: dragdrop.Copy, PositionValid: true}, true},
		{"unaccepted", "unaccepted", 1, 1, 0, false, false, dragdrop.Result{PositionValid: true}, true},
		{"refused Drop", "unaccepted", 1, 1, 1, false, false, dragdrop.Result{PositionValid: true}, true},
		{"Escape", "canceled", 1, 1, 0, false, false, dragdrop.Result{Canceled: true}, true},
		{"destroyed", "canceled", 1, 1, 0, false, true, dragdrop.Result{Canceled: true}, true},
		{"error", "error", 1, 1, 0, false, false, dragdrop.Result{Err: errors.New("timeout")}, true},
		{"no Begin", "canceled", 0, 1, 0, false, false, dragdrop.Result{Canceled: true}, false},
		{"duplicate End", "canceled", 1, 2, 0, false, false, dragdrop.Result{Canceled: true}, false},
		{"missing payload", "accepted", 1, 1, 1, false, false, dragdrop.Result{Action: dragdrop.Copy, PositionValid: true}, false},
		{"false cancellation after commit", "canceled", 1, 1, 1, true, false, dragdrop.Result{Canceled: true}, false},
		{"Drop after Escape", "canceled", 1, 1, 1, false, false, dragdrop.Result{Canceled: true}, false},
		{"missing position", "unaccepted", 1, 1, 0, false, false, dragdrop.Result{}, false},
		{"destroyed position", "unaccepted", 1, 1, 0, false, true, dragdrop.Result{PositionValid: true}, false},
		{"wrong expected reason", "canceled", 1, 1, 0, false, false, dragdrop.Result{PositionValid: true}, false},
	} {
		t.Run(tt.name, func(t *testing.T) {
			err := verifyResult(tt.expect, true, tt.destroyed, tt.begins, tt.ends, tt.drops, tt.accepted, tt.result)
			if (err == nil) != tt.ok {
				t.Fatalf("verifyResult=%v, want success=%t", err, tt.ok)
			}
		})
	}
}
