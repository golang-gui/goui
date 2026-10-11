package main

import (
	"bytes"
	"crypto/sha256"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
)

// Generated inputs live beside source code. A sidecar digest protects both
// unrelated files and edits to our generated files; ownership is not inferred
// merely from a reserved filename. An interrupted pair update fails safely.
func writeGenerated(path string, data []byte) error {
	marker := path + ".goui"
	info, err := os.Lstat(path)
	if err == nil {
		if !info.Mode().IsRegular() {
			return fmt.Errorf("refusing to replace non-regular generated file %s", path)
		}
		info, statErr := os.Lstat(marker)
		if statErr != nil || !info.Mode().IsRegular() {
			return fmt.Errorf("invalid ownership marker %s", marker)
		}
		old, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		digest, err := os.ReadFile(marker)
		if err != nil || strings.TrimSpace(string(digest)) != fmt.Sprintf("%x", sha256.Sum256(old)) {
			return fmt.Errorf("refusing to overwrite unowned or edited file %s", path)
		}
		if bytes.Equal(old, data) {
			return nil
		}
	} else if !errors.Is(err, os.ErrNotExist) {
		return err
	} else if _, err := os.Lstat(marker); !errors.Is(err, os.ErrNotExist) {
		return fmt.Errorf("orphan ownership marker %s; inspect it before removing", marker)
	}
	if err := writeFileAtomic(path, data, 0644); err != nil {
		return err
	}
	return writeFileAtomic(marker, []byte(fmt.Sprintf("%x\n", sha256.Sum256(data))), 0644)
}

func writeFileAtomic(path string, data []byte, mode os.FileMode) error {
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return err
	}
	f, err := os.CreateTemp(filepath.Dir(path), ".goui-file-*")
	if err != nil {
		return err
	}
	defer os.Remove(f.Name())
	if _, err := f.Write(data); err != nil {
		f.Close()
		return err
	}
	if err := f.Chmod(mode); err != nil {
		f.Close()
		return err
	}
	if err := f.Close(); err != nil {
		return err
	}
	return os.Rename(f.Name(), path)
}

func copyFile(from, to string, mode os.FileMode) error {
	src, err := os.Open(from)
	if err != nil {
		return err
	}
	defer src.Close()
	if err := os.MkdirAll(filepath.Dir(to), 0755); err != nil {
		return err
	}
	dst, err := os.OpenFile(to, os.O_WRONLY|os.O_CREATE|os.O_EXCL, mode)
	if err != nil {
		return err
	}
	if err := dst.Chmod(mode); err != nil {
		dst.Close()
		return err
	}
	_, err = io.Copy(dst, src)
	closeErr := dst.Close()
	if err != nil {
		return err
	}
	return closeErr
}
