package main

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/golang-gui/golink"
	"github.com/golang-gui/goui/cmd/goui/internal/mobileassets"
	mobilebase "github.com/golang-gui/goui/platform/mobile/base"
)

func mobileDigest(data []byte) string {
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:])
}

func resolveMobileComponent(ctx context.Context, c *config, directory string, t target, variant string) (*mobileComponent, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if directory != "" {
		return openMobileComponent(directory, t, variant)
	}
	command := exec.CommandContext(ctx, "go", "env", "GOVERSION")
	command.Dir = c.path(c.Main)
	command.Env = append(os.Environ(), "GOTOOLCHAIN=local")
	version, err := command.Output()
	if err != nil {
		return nil, fmt.Errorf("inspect Go toolchain for embedded mobile base: %w", err)
	}
	return embeddedMobileComponent(ctx, strings.TrimSpace(string(version)), t, variant)
}

func embeddedMobileComponent(ctx context.Context, version string, t target, variant string) (*mobileComponent, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	blobs, ok := mobileassets.Select(version, t.String(), variant)
	if version == "" {
		var b mobileassets.Blob
		b, ok = mobileassets.Target(t.String(), variant)
		blobs = []mobileassets.Blob{b}
	}
	if !ok {
		return nil, fmt.Errorf("no embedded mobile base for %s %s %s; install a supported Go toolchain or supply -components", version, t, variant)
	}
	m := &mobileComponent{Format: 2, Target: t.String(), Variant: variant, Base: "base/base.json", Inputs: "inputs/inputs.json", GoLink: golink.Version, Files: map[string]string{}}
	if version == "" {
		m.Base = ""
	}
	if t.OS == "android" {
		m.Dex, m.TargetSDK, m.Libraries = "platform/classes.dex", 35, []string{"android"}
	} else {
		m.Objects = []string{"platform/main.o", "platform/uikit.o"}
		m.Libraries = []string{"objc", "resolv"}
		m.Frameworks = []string{"UIKit", "Foundation", "CoreGraphics", "QuartzCore", "OpenGLES", "Security"}
	}
	for _, blob := range blobs {
		for name, hash := range blob.Files {
			if _, exists := m.Files[name]; exists {
				return nil, fmt.Errorf("duplicate embedded mobile input %s", name)
			}
			m.Files[name] = hash
		}
	}
	manifest, err := json.MarshalIndent(m, "", "  ")
	if err != nil {
		return nil, err
	}
	manifest = append(manifest, '\n')
	m.digest = mobileDigest(manifest)
	cache, err := os.UserCacheDir()
	if err != nil {
		return nil, err
	}
	parent := filepath.Join(cache, "goui", "mobile")
	if err = os.MkdirAll(parent, 0700); err != nil {
		return nil, err
	}
	m.root = filepath.Join(parent, m.digest)
	validate := func() error {
		actual, err := mobileHash(filepath.Join(m.root, "mobile.json"))
		if err != nil {
			return err
		}
		if actual != m.digest {
			return fmt.Errorf("embedded component manifest was modified: %s", m.root)
		}
		for name := range m.Files {
			if _, err := m.file(name); err != nil {
				return err
			}
		}
		return nil
	}
	if _, err = os.Lstat(m.root); err == nil {
		return m, validate()
	} else if !os.IsNotExist(err) {
		return nil, err
	}
	stage, err := os.MkdirTemp(parent, ".mobile-")
	if err != nil {
		return nil, err
	}
	defer os.RemoveAll(stage)
	for _, blob := range blobs {
		if err = unpackMobile(ctx, stage, blob); err != nil {
			return nil, err
		}
	}
	if err = os.WriteFile(filepath.Join(stage, "mobile.json"), manifest, 0600); err != nil {
		return nil, err
	}
	if err = ctx.Err(); err != nil {
		return nil, err
	}
	if err = os.Rename(stage, m.root); err != nil {
		// Another build may have completed the same immutable cache entry.
		if validErr := validate(); validErr != nil {
			return nil, fmt.Errorf("publish embedded component: %w", err)
		}
	}
	return m, validate()
}

func resolveMobileTarget(ctx context.Context, directory string, t target, variant string) (*mobileComponent, error) {
	if directory != "" {
		return openMobileProfile(directory, t, variant, false)
	}
	return embeddedMobileComponent(ctx, "", t, variant)
}

func mobileTargetDigest(m *mobileComponent) string {
	files := map[string]string{}
	for path, hash := range m.Files {
		if strings.HasPrefix(path, "inputs/") || strings.HasPrefix(path, "platform/") {
			files[path] = hash
		}
	}
	data, _ := json.Marshal(struct {
		Target, Variant string
		SDK             int
		Files           map[string]string
	}{m.Target, m.Variant, m.TargetSDK, files})
	return mobileDigest(data)
}

func validateMobileCgoEnvironment(ctx context.Context, c *config, mode string, flags map[string]string) error {
	settings := map[string]string{}
	for _, key := range []string{"CGO_ENABLED", "CC", "CXX", "CGO_CFLAGS", "CGO_CPPFLAGS", "CGO_CXXFLAGS", "CGO_FFLAGS", "CGO_LDFLAGS"} {
		if value, ok := os.LookupEnv(key); ok {
			settings[key] = value
		}
	}
	if err := checkMobileCgoSettings(mode, settings, flags); err != nil {
		return err
	}
	command := exec.CommandContext(ctx, "go", "env", "GOENV")
	command.Dir = c.path(c.Main)
	data, err := command.Output()
	if err != nil {
		return fmt.Errorf("inspect Go environment: %w", err)
	}
	path := strings.TrimSpace(string(data))
	if path == "off" || path == "" {
		return nil
	}
	data, err = os.ReadFile(path)
	if os.IsNotExist(err) {
		return nil
	}
	if err != nil {
		return err
	}
	for _, line := range strings.Split(string(data), "\n") {
		key, value, ok := strings.Cut(line, "=")
		if _, present := settings[key]; ok && !present {
			settings[key] = strings.TrimSuffix(value, "\r")
		}
	}
	return checkMobileCgoSettings(mode, settings, flags)
}

func checkMobileCgoSettings(mode string, settings, flags map[string]string) error {
	if mode != "prebuilt" && mode != "native" {
		return fmt.Errorf("unsupported cgo strategy %q", mode)
	}
	if v := settings["CGO_ENABLED"]; v != "" && v != "0" && v != "1" {
		return fmt.Errorf("invalid CGO_ENABLED value")
	}
	if settings["CGO_ENABLED"] == "0" {
		return fmt.Errorf("mobile linking requires CGO_ENABLED=1; a prebuilt build avoids native compilers but does not disable Go's cgo semantics")
	}
	if mode == "native" {
		return nil
	}
	for _, key := range []string{"CC", "CXX"} {
		if settings[key] != "" {
			return fmt.Errorf("explicit %s conflicts with prebuilt mobile support; use -cgo=native for a target toolchain", key)
		}
	}
	for key, want := range flags {
		value := settings[key]
		if value == "" && key != "CGO_CPPFLAGS" {
			value = "-O2 -g"
		}
		if value != want {
			return fmt.Errorf("%s does not match prebuilt support; use -cgo=native", key)
		}
	}
	return nil
}

func unpackMobile(ctx context.Context, directory string, blob mobileassets.Blob) error {
	compressed := []byte(blob.Data)
	if mobileDigest(compressed) != blob.SHA256 {
		return fmt.Errorf("embedded mobile archive checksum mismatch")
	}
	gz, err := gzip.NewReader(bytes.NewReader(compressed))
	if err != nil {
		return err
	}
	defer gz.Close()
	r := tar.NewReader(gz)
	seen := map[string]bool{}
	for {
		if err = ctx.Err(); err != nil {
			return err
		}
		h, err := r.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return err
		}
		want, declared := blob.Files[h.Name]
		if h.Typeflag != tar.TypeReg || !declared || seen[h.Name] || !filepath.IsLocal(h.Name) || strings.Contains(h.Name, "\\") || h.Size < 0 || h.Size > 32<<20 {
			return fmt.Errorf("invalid embedded mobile entry %q", h.Name)
		}
		seen[h.Name] = true
		data, err := io.ReadAll(r)
		if err != nil {
			return err
		}
		if mobileDigest(data) != want {
			return fmt.Errorf("embedded mobile input checksum mismatch: %s", h.Name)
		}
		path := filepath.Join(directory, filepath.FromSlash(h.Name))
		if err = os.MkdirAll(filepath.Dir(path), 0700); err != nil {
			return err
		}
		f, err := os.OpenFile(path, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0600)
		if err != nil {
			return err
		}
		_, writeErr := f.Write(data)
		closeErr := f.Close()
		if writeErr != nil {
			return writeErr
		}
		if closeErr != nil {
			return closeErr
		}
	}
	if len(seen) != len(blob.Files) {
		return fmt.Errorf("incomplete embedded mobile archive")
	}
	return nil
}

func prepareMobileTools(ctx context.Context, m *mobileComponent, log io.Writer) error {
	if m.GoLink != golink.Version {
		return fmt.Errorf("component requires golink %s, tool contains %s", m.GoLink, golink.Version)
	}
	self, err := os.Executable()
	if err != nil {
		return err
	}
	identity, err := mobileHash(self)
	if err != nil {
		return err
	}
	basePath, err := m.file(m.Base)
	if err != nil {
		return err
	}
	base, err := mobilebase.BaseOpen(basePath)
	if err != nil {
		return err
	}
	cache, err := os.UserCacheDir()
	if err != nil {
		return err
	}
	parent := filepath.Join(cache, "goui", "mobile-host", runtime.GOOS+"-"+runtime.GOARCH, base.GoVersion)
	if err = os.MkdirAll(parent, 0700); err != nil {
		return err
	}
	root := filepath.Join(parent, identity)
	name := "go-mobile"
	if runtime.GOOS == "windows" {
		name += ".exe"
	}
	m.driver, m.linker, m.tools = filepath.Join(root, name), self, filepath.Join(root, "tools", "tools.json")
	if info, err := os.Lstat(m.driver); err == nil && info.Mode().IsRegular() {
		_, err = mobilebase.OpenNativeTools(m.tools)
		return err
	}
	stage, err := os.MkdirTemp(parent, ".host-")
	if err != nil {
		return err
	}
	defer os.RemoveAll(stage)
	if log != nil {
		fmt.Fprintf(log, "Preparing %s mobile Go driver for %s/%s.\n", base.GoVersion, runtime.GOOS, runtime.GOARCH)
	}
	if err = mobilebase.BuildDriver(ctx, "go", filepath.Join(stage, name), log); err != nil {
		return err
	}
	tools := mobilebase.NativeTools{Format: 2, GOOS: runtime.GOOS, GOARCH: runtime.GOARCH, GoLink: golink.Version, Files: map[string]string{}, Provenance: mobilebase.NativeProvenance{Name: "golink", Version: golink.Version, Source: "https://github.com/golang-gui/golink", License: "MIT; Go-derived codesign: BSD-3-Clause (THIRD_PARTY.md)"}}
	data, err := json.MarshalIndent(tools, "", "  ")
	if err != nil {
		return err
	}
	if err = os.Mkdir(filepath.Join(stage, "tools"), 0700); err != nil {
		return err
	}
	if err = os.WriteFile(filepath.Join(stage, "tools", "tools.json"), data, 0600); err != nil {
		return err
	}
	if err = os.Rename(stage, root); err != nil {
		if info, statErr := os.Lstat(m.driver); statErr != nil || !info.Mode().IsRegular() {
			return err
		}
	}
	_, err = mobilebase.OpenNativeTools(m.tools)
	return err
}
