// Package fileaccess implements desktop path access and private credential
// encoding shared by native dialog backends. It does not grant permissions.
package fileaccess

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/golang-gui/goui/platform/common"
)

const maxCredential = 4 << 20

type credential struct {
	Version int    `json:"version"`
	OS      string `json:"os"`
	Kind    string `json:"kind"`
	Data    []byte `json:"data"`
}

// Encode wraps native data in a versioned, backend-specific envelope.
func Encode(kind string, data []byte) ([]byte, error) {
	if kind == "" || len(data) == 0 || len(data) > maxCredential/2 {
		return nil, fmt.Errorf("file credential payload: %w", os.ErrInvalid)
	}
	return json.Marshal(credential{Version: 1, OS: runtime.GOOS, Kind: kind, Data: data})
}

// Decode rejects unknown versions, foreign backends, trailing data and oversized
// inputs before handing any bytes to the native bookmark parser.
func Decode(data []byte, kind string) ([]byte, error) {
	if len(data) == 0 || len(data) > maxCredential {
		return nil, fmt.Errorf("file credential size: %w", os.ErrInvalid)
	}
	var c credential
	d := json.NewDecoder(bytes.NewReader(data))
	d.DisallowUnknownFields()
	if err := d.Decode(&c); err != nil {
		return nil, fmt.Errorf("file credential: %w", os.ErrInvalid)
	}
	var extra any
	if d.Decode(&extra) != io.EOF || c.Version != 1 || c.OS != runtime.GOOS ||
		c.Kind != kind || len(c.Data) == 0 || len(c.Data) > maxCredential/2 {
		return nil, fmt.Errorf("file credential format/backend: %w", os.ErrInvalid)
	}
	return c.Data, nil
}

type pathAccess struct {
	path   string
	closed bool
}

// NewPath allows a not-yet-created save destination, but never expands shell
// syntax. Native pickers must supply absolute paths.
func NewPath(path string) (common.FileAccess, error) {
	if !filepath.IsAbs(path) || strings.ContainsRune(path, 0) {
		return nil, fmt.Errorf("file selection path: %w", os.ErrInvalid)
	}
	return &pathAccess{path: filepath.Clean(path)}, nil
}

// Paths transfers all converted objects together, cleaning up on any failure.
func Paths(paths []string) ([]common.FileAccess, error) {
	if len(paths) == 0 {
		return nil, fmt.Errorf("successful file selection has no paths: %w", os.ErrInvalid)
	}
	var result []common.FileAccess
	for _, path := range paths {
		a, err := NewPath(path)
		if err != nil {
			for _, previous := range result {
				previous.Close()
			}
			return nil, err
		}
		result = append(result, a)
	}
	return result, nil
}

func (a *pathAccess) Name() string { return filepath.Base(a.path) }

func (a *pathAccess) LocalPath() (string, error) {
	if a.closed {
		return "", os.ErrClosed
	}
	return a.path, nil
}

func (a *pathAccess) Credential() ([]byte, error) {
	if a.closed {
		return nil, os.ErrClosed
	}
	return Encode("path", []byte(a.path))
}

func (a *pathAccess) Close() error {
	a.closed = true
	return nil
}

// RestorePath checks existence, not read/write permission or file identity.
// Plain path credentials do not follow renamed files or grant extra access.
func RestorePath(data []byte) (common.FileAccess, error) {
	path, err := Decode(data, "path")
	if err != nil {
		return nil, err
	}
	a, err := NewPath(string(path))
	if err != nil {
		return nil, err
	}
	if _, err := os.Stat(string(path)); err != nil {
		return nil, err
	}
	return a, nil
}
