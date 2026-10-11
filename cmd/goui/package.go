package main

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
)

// Packaging deliberately has no context/Go command dependency: it consumes a
// complete binary and creates an application layout, never builds implicitly.
func packageApp(c *config, t target, format, binaryPath, output string) (string, error) {
	if t.OS == "windows" {
		return "", fmt.Errorf("Windows build already produces the distributable EXE; no Windows package format is implemented")
	}
	expected := "dir"
	if t.OS == "darwin" {
		expected = "app"
	}
	if format == "" {
		format = expected
	}
	if format != expected {
		return "", fmt.Errorf("%s supports package -format %s in this release", t, expected)
	}
	if binaryPath == "" {
		binaryPath = c.binary(t)
	} else {
		binaryPath = c.path(binaryPath)
	}
	if err := checkBinary(binaryPath, t); err != nil {
		return "", err
	}
	if output == "" {
		name := c.Executable + "-rootfs"
		if t.OS == "darwin" {
			name = c.Name + ".app"
		}
		output = filepath.Join(c.dir, "dist", t.folder(), name)
	} else {
		output = c.path(output)
	}
	if t.OS == "darwin" && filepath.Ext(output) != ".app" {
		return "", fmt.Errorf("macOS package output must end in .app")
	}
	if _, err := os.Lstat(output); !errors.Is(err, os.ErrNotExist) {
		if err != nil {
			return "", err
		}
		return "", fmt.Errorf("package output already exists: %s (choose a new -o directory)", output)
	}
	icon, err := loadIcon(c)
	if err != nil {
		return "", err
	}
	assets, err := resourceFiles(c, t, icon)
	if err != nil {
		return "", err
	}
	if err := os.MkdirAll(filepath.Dir(output), 0755); err != nil {
		return "", err
	}
	pattern := ".goui-package-*"
	if t.OS == "darwin" {
		pattern += ".app"
	}
	work, err := os.MkdirTemp(filepath.Dir(output), pattern)
	if err != nil {
		return "", err
	}
	defer os.RemoveAll(work) // Only the staging directory allocated above.
	if err := os.Chmod(work, 0755); err != nil {
		return "", err
	}
	for _, name := range sortedNames(assets) {
		path := filepath.Join(work, filepath.FromSlash(name))
		if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
			return "", err
		}
		if err := os.WriteFile(path, assets[name], 0644); err != nil {
			return "", err
		}
	}
	executable := filepath.Join("usr", "bin", c.Executable)
	if t.OS == "darwin" {
		executable = filepath.Join("Contents", "MacOS", c.Executable)
	}
	if err := copyFile(binaryPath, filepath.Join(work, executable), 0755); err != nil {
		return "", err
	}
	if t.OS == "darwin" && c.MacOS.SigningIdentity != "" {
		if err := signMacOS(c, work); err != nil {
			return "", err
		}
	}
	if err := os.Rename(work, output); err != nil {
		return "", err
	}
	return output, nil
}
