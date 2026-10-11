package fileaccess

import (
	"bytes"
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"testing"
)

func TestPathAccess(t *testing.T) {
	for _, directory := range []bool{false, true} {
		t.Run(map[bool]string{false: "file", true: "directory"}[directory], func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "文件-é.txt")
			if directory {
				if err := os.Mkdir(path, 0700); err != nil {
					t.Fatal(err)
				}
			} else if err := os.WriteFile(path, []byte("test"), 0600); err != nil {
				t.Fatal(err)
			}
			a, err := NewPath(path)
			if err != nil {
				t.Fatal(err)
			}
			if got, err := a.LocalPath(); err != nil || got != path {
				t.Fatalf("path %q: %v", got, err)
			}
			if a.Name() != filepath.Base(path) {
				t.Fatal(a.Name())
			}
			data, err := a.Credential()
			if err != nil {
				t.Fatal(err)
			}
			if err := a.Close(); err != nil {
				t.Fatal(err)
			}
			if err := a.Close(); err != nil {
				t.Fatal(err)
			}
			if _, err := a.LocalPath(); !errors.Is(err, os.ErrClosed) {
				t.Fatal(err)
			}
			if _, err := a.Credential(); !errors.Is(err, os.ErrClosed) {
				t.Fatal(err)
			}
			if a.Name() != filepath.Base(path) {
				t.Fatal("name lost after close")
			}
			r, err := RestorePath(data)
			if err != nil {
				t.Fatal(err)
			}
			defer r.Close()
			if got, err := r.LocalPath(); err != nil || got != path {
				t.Fatalf("restored %q: %v", got, err)
			}
			if err := os.Remove(path); err != nil {
				t.Fatal(err)
			}
			if _, err := RestorePath(data); !errors.Is(err, os.ErrNotExist) {
				t.Fatal(err)
			}
		})
	}
}

func TestSaveDestinationAndSelection(t *testing.T) {
	path := filepath.Join(t.TempDir(), "new.txt")
	a, err := NewPath(path)
	if err != nil {
		t.Fatal(err)
	}
	defer a.Close()
	data, err := a.Credential()
	if err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(path); !errors.Is(err, os.ErrNotExist) {
		t.Fatal("selection created file", err)
	}
	if _, err := RestorePath(data); !errors.Is(err, os.ErrNotExist) {
		t.Fatal(err)
	}
	for _, invalid := range []string{"", "relative", path + "\x00bad"} {
		if _, err := NewPath(invalid); !errors.Is(err, os.ErrInvalid) {
			t.Fatalf("%q: %v", invalid, err)
		}
	}
	if got, err := Paths([]string{path, "relative"}); got != nil || !errors.Is(err, os.ErrInvalid) {
		t.Fatal(got, err)
	}
	if got, err := Paths(nil); got != nil || !errors.Is(err, os.ErrInvalid) {
		t.Fatal("empty successful selection reported as cancellation", got, err)
	}
}

func TestCredentialValidation(t *testing.T) {
	valid, err := Encode("native", []byte{0, 1, 2, 255})
	if err != nil {
		t.Fatal(err)
	}
	payload, err := Decode(valid, "native")
	if err != nil || !bytes.Equal(payload, []byte{0, 1, 2, 255}) {
		t.Fatal(payload, err)
	}
	foreign := "darwin"
	if runtime.GOOS == foreign {
		foreign = "windows"
	}
	for _, data := range [][]byte{
		nil, []byte("garbage"), append(append([]byte{}, valid...), []byte(" {}")...),
		bytes.Replace(valid, []byte(`"version":1`), []byte(`"version":2`), 1),
		bytes.Replace(valid, []byte(runtime.GOOS), []byte(foreign), 1),
		bytes.Replace(valid, []byte(`"native"`), []byte(`"other"`), 1),
		[]byte(`{"version":1,"unknown":true}`), []byte(`{"version":1,"data":"%%%"}`),
		bytes.Repeat([]byte("x"), maxCredential+1),
	} {
		if _, err := Decode(data, "native"); !errors.Is(err, os.ErrInvalid) {
			t.Fatalf("invalid credential accepted: %v", err)
		}
	}
	for _, data := range [][]byte{nil, bytes.Repeat([]byte("x"), maxCredential/2+1)} {
		if _, err := Encode("native", data); !errors.Is(err, os.ErrInvalid) {
			t.Fatal(err)
		}
	}
	if _, err := Encode("", []byte{1}); !errors.Is(err, os.ErrInvalid) {
		t.Fatal(err)
	}
}
