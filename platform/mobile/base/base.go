// Package base manages versioned native support inputs for Go builds.
// It contains no windowing, event-loop, or application lifecycle policy.
package base

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
)

// BaseManifest describes one target's native-required package inputs.
// Format 2 records generated Go sources and native objects; format 1 retains
// the original complete package archive protocol for existing components.
type BaseManifest struct {
	Format         int                    `json:"format"`
	GoVersion      string                 `json:"go_version"`
	GOOS           string                 `json:"goos"`
	GOARCH         string                 `json:"goarch"`
	Architecture   string                 `json:"architecture"`
	Experiment     string                 `json:"experiment"`
	BuildMode      string                 `json:"build_mode"`
	Variant        string                 `json:"variant"`
	MinimumVersion string                 `json:"minimum_version"`
	Packages       map[string]BasePackage `json:"packages"`
	CgoFlags       map[string]string      `json:"cgo_flags,omitempty"`
	ProducerTools  map[string]string      `json:"producer_tools,omitempty"`
	root           string
}

// BasePackage records selected source inputs and their prepared native support.
type BasePackage struct {
	ModulePath string `json:"module_path,omitempty"`
	// ModuleVersion pins released archives in format 1. Format 2 uses exact
	// source/language checks and compiles ordinary Go dependencies normally.
	ModuleVersion   string            `json:"module_version,omitempty"`
	ModuleGoVersion string            `json:"module_go_version,omitempty"`
	Archive         string            `json:"archive,omitempty"`
	SHA256          string            `json:"sha256,omitempty"`
	SourceSHA256    string            `json:"source_sha256"`
	BuildID         string            `json:"build_id,omitempty"`
	Imports         map[string]string `json:"imports,omitempty"`
	GeneratedGo     []BaseFile        `json:"generated_go,omitempty"`
	Objects         []BaseFile        `json:"objects,omitempty"`
	Auxiliary       []BaseFile        `json:"auxiliary,omitempty"`
}

func baseModuleMatches(p BasePackage, path, version, language string) bool {
	return p.ModulePath == path && p.ModuleGoVersion == language &&
		(p.ModuleVersion == "" || p.ModuleVersion == version)
}

func supportedBaseVersion(version string) bool {
	switch version {
	case "go1.25.8", "go1.26.10", "go1.27.3":
		return true
	}
	return false
}

// BaseOpen verifies a local component before it is used. This is an integrity
// check; authenticity of a downloaded component belongs to its distributor.
func BaseOpen(path string) (*BaseManifest, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var m BaseManifest
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&m); err != nil {
		return nil, fmt.Errorf("base manifest: %w", err)
	}
	if err := decoder.Decode(new(any)); err != io.EOF {
		return nil, fmt.Errorf("base manifest: trailing data")
	}
	if (m.Format != 1 && m.Format != 2) || !supportedBaseVersion(m.GoVersion) || m.Format == 1 && m.GoVersion != "go1.25.8" {
		return nil, fmt.Errorf("unsupported base format or Go version: %d / %s", m.Format, m.GoVersion)
	}
	if !((m.GOOS == "android" && m.GOARCH == "arm64" && m.BuildMode == "c-shared") || (m.GOOS == "ios" && m.GOARCH == "arm64" && m.BuildMode == "c-archive") || (m.GOOS == "linux" && (m.GOARCH == "arm64" || m.GOARCH == "amd64") && m.BuildMode == "c-shared")) {
		return nil, fmt.Errorf("unsupported base target: %s/%s %s", m.GOOS, m.GOARCH, m.BuildMode)
	}
	if m.Architecture == "" || m.Variant == "" || m.MinimumVersion == "" || len(m.Packages) == 0 {
		return nil, fmt.Errorf("base manifest: incomplete target or package list")
	}
	if _, ok := m.Packages["runtime/cgo"]; !ok {
		return nil, fmt.Errorf("base manifest: missing runtime/cgo")
	}
	if m.Format == 2 {
		for _, key := range []string{"CGO_CFLAGS", "CGO_CPPFLAGS", "CGO_CXXFLAGS", "CGO_FFLAGS", "CGO_LDFLAGS"} {
			if _, ok := m.CgoFlags[key]; !ok {
				return nil, fmt.Errorf("base manifest: missing producer flag %s", key)
			}
		}
		for _, key := range []string{"cgo", "CC", "CXX"} {
			hash, err := hex.DecodeString(m.ProducerTools[key])
			if err != nil || len(hash) != sha256.Size {
				return nil, fmt.Errorf("base manifest: invalid producer tool %s", key)
			}
		}
	}
	root, err := filepath.Abs(filepath.Dir(path))
	if err != nil {
		return nil, err
	}
	m.root = root
	for name, p := range m.Packages {
		if m.Format == 2 {
			if err := verifyRecipe(&m, name, p); err != nil {
				return nil, err
			}
			continue
		}
		if name == "" || len(p.SHA256) != 64 || len(p.SourceSHA256) != 64 || !strings.Contains(p.BuildID, "/") {
			return nil, fmt.Errorf("base manifest: invalid package %q", name)
		}
		full, err := baseFile(root, p.Archive)
		if err != nil {
			return nil, fmt.Errorf("base %s: %w", name, err)
		}
		hash, err := baseHash(full)
		if err != nil {
			return nil, err
		}
		if hash != p.SHA256 {
			return nil, fmt.Errorf("base %s: archive SHA-256 mismatch", name)
		}
		for dep, hash := range p.Imports {
			if dep == "" || len(hash) != 64 {
				return nil, fmt.Errorf("base %s: invalid dependency %q", name, dep)
			}
		}
		p.Archive = full
		m.Packages[name] = p
	}
	return &m, nil
}

func baseFile(root, name string) (string, error) {
	if name == "" || strings.Contains(name, "\\") || filepath.IsAbs(name) || !filepath.IsLocal(filepath.FromSlash(name)) {
		return "", fmt.Errorf("invalid component path %q", name)
	}
	path := filepath.Join(root, filepath.FromSlash(name))
	actual, err := filepath.EvalSymlinks(path)
	if err != nil {
		return "", err
	}
	canonicalRoot, err := filepath.EvalSymlinks(root)
	if err != nil {
		return "", err
	}
	rel, err := filepath.Rel(canonicalRoot, actual)
	if err != nil || !filepath.IsLocal(rel) {
		return "", fmt.Errorf("component path escapes base: %q", name)
	}
	return actual, nil
}

func baseHash(path string) (string, error) {
	file, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer file.Close()
	hash := sha256.New()
	if _, err = io.Copy(hash, file); err != nil {
		return "", err
	}
	return hex.EncodeToString(hash.Sum(nil)), nil
}

// BaseSourceHash hashes the selected files, independent of the source directory.
func BaseSourceHash(dir string, groups ...[]string) (string, error) {
	var names []string
	for _, group := range groups {
		names = append(names, group...)
	}
	slices.Sort(names)
	names = slices.Compact(names)
	hash := sha256.New()
	for _, name := range names {
		if !filepath.IsLocal(name) {
			return "", fmt.Errorf("invalid source file %q", name)
		}
		data, err := os.ReadFile(filepath.Join(dir, name))
		if err != nil {
			return "", err
		}
		fmt.Fprintf(hash, "%s\x00%d\x00", filepath.ToSlash(name), len(data))
		hash.Write(data)
	}
	return hex.EncodeToString(hash.Sum(nil)), nil
}

// BaseExportHash identifies unified Go export data, excluding host-specific
// archive metadata and build IDs. The compiler/linker retain their own checks.
func BaseExportHash(path string) (string, error) {
	file, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer file.Close()
	var header [68]byte
	if _, err = io.ReadFull(file, header[:]); err != nil {
		return "", err
	}
	if string(header[:8]) != "!<arch>\n" || strings.TrimSpace(string(header[8:24])) != "__.PKGDEF" || string(header[66:68]) != "`\n" {
		return "", fmt.Errorf("%s: not a Go package archive", path)
	}
	size, err := strconv.ParseInt(strings.TrimSpace(string(header[56:66])), 10, 64)
	if err != nil || size <= 0 || size > 64<<20 {
		return "", fmt.Errorf("%s: invalid export size", path)
	}
	data := make([]byte, int(size))
	if _, err = io.ReadFull(file, data); err != nil {
		return "", err
	}
	start := bytes.Index(data, []byte("\n$$B\nu"))
	if start < 0 {
		return "", fmt.Errorf("%s: missing unified export data", path)
	}
	data = data[start+6:]
	if !bytes.HasSuffix(data, []byte("\n$$\n")) {
		return "", fmt.Errorf("%s: invalid export trailer", path)
	}
	sum := sha256.Sum256(data[:len(data)-4])
	return hex.EncodeToString(sum[:]), nil
}
