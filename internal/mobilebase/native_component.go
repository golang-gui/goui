package mobilebase

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
)

// NativeProvenance identifies the maintainer's original SDK or tool release.
// License records its terms or review status; it does not grant redistribution.
type NativeProvenance struct {
	Name    string `json:"name"`
	Version string `json:"version"`
	Source  string `json:"source"`
	License string `json:"license"`
}

// NativeInputs contains target link inputs, independent of host tools and UI.
// Files is an exact relative-path/SHA-256 list. Export fills empty hashes.
type NativeInputs struct {
	Format         int                `json:"format"`
	GOOS           string             `json:"goos"`
	GOARCH         string             `json:"goarch"`
	Variant        string             `json:"variant"`
	MinimumVersion string             `json:"minimum_version"`
	SDKVersion     string             `json:"sdk_version"`
	Provenance     NativeProvenance   `json:"provenance"`
	Files          map[string]string  `json:"files"`
	Android        *AndroidLinkInputs `json:"android,omitempty"`
	IOS            *IOSLinkInputs     `json:"ios,omitempty"`
	root           string
}

type AndroidLinkInputs struct {
	LibraryDir string `json:"library_dir"`
	CRTBegin   string `json:"crt_begin"`
	CRTEnd     string `json:"crt_end"`
	Builtins   string `json:"builtins"`
}

type IOSLinkInputs struct {
	Sysroot string `json:"sysroot"`
}

// NativeTool identifies a tool within a host component.
type NativeTool struct {
	File    string `json:"file"`
	Version string `json:"version"`
}

// NativeTools contains the host executables and their declared runtime files.
// LLD is the multi-flavor LLVM linker; archive construction reuses Go pack.
type NativeTools struct {
	Format     int               `json:"format"`
	GOOS       string            `json:"goos"`
	GOARCH     string            `json:"goarch"`
	Provenance NativeProvenance  `json:"provenance"`
	LLD        NativeTool        `json:"lld"`
	Files      map[string]string `json:"files"`
	root       string
}

func nativeReadJSON(path string, value any) error {
	data, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.DisallowUnknownFields()
	if err = decoder.Decode(value); err != nil {
		return fmt.Errorf("%s: %w", path, err)
	}
	if err = decoder.Decode(new(any)); err != io.EOF {
		return fmt.Errorf("%s: trailing JSON data", path)
	}
	return nil
}

func nativeVerifyFiles(root string, files map[string]string, manifestName string) error {
	if len(files) == 0 {
		return fmt.Errorf("native component has no files")
	}
	for name, expected := range files {
		path, err := baseFile(root, name)
		if err != nil {
			return err
		}
		actual, err := baseHash(path)
		if err != nil {
			return err
		}
		if actual != expected {
			return fmt.Errorf("native component %s: SHA-256 mismatch", name)
		}
	}
	return filepath.WalkDir(root, func(path string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if entry.IsDir() {
			return nil
		}
		name, err := filepath.Rel(root, path)
		if err != nil {
			return err
		}
		name = filepath.ToSlash(name)
		if name == manifestName {
			return nil
		}
		if _, ok := files[name]; !ok {
			return fmt.Errorf("undeclared native component file %s", name)
		}
		return nil
	})
}

func nativeProvenanceValid(p NativeProvenance) bool {
	return p.Name != "" && p.Version != "" && p.Source != "" && p.License != ""
}

func (m *NativeInputs) validate() error {
	if m.Format != 1 || m.GOARCH != "arm64" || !nativeProvenanceValid(m.Provenance) {
		return fmt.Errorf("invalid native input identity")
	}
	sdk, err := nativeVersion(m.SDKVersion)
	if err != nil {
		return err
	}
	minimum, err := nativeVersion(m.MinimumVersion)
	if err != nil {
		return err
	}
	if minimum > sdk {
		return fmt.Errorf("minimum native version exceeds SDK version")
	}

	switch m.GOOS {
	case "android":
		if m.Variant != "device" || m.Android == nil || m.IOS != nil {
			return fmt.Errorf("invalid Android link inputs")
		}
		for _, name := range []string{m.Android.CRTBegin, m.Android.CRTEnd, m.Android.Builtins} {
			if _, ok := m.Files[name]; !ok {
				return fmt.Errorf("missing declared native input %s", name)
			}
		}
		for _, name := range []string{"c", "dl", "m"} {
			if _, err := m.library(name); err != nil {
				return err
			}
		}
	case "ios":
		if (m.Variant != "device" && m.Variant != "simulator") || m.IOS == nil || m.Android != nil {
			return fmt.Errorf("invalid iOS link inputs")
		}
		if _, err := m.directory(m.IOS.Sysroot); err != nil {
			return err
		}
		if _, err := m.library("System"); err != nil {
			return err
		}
	default:
		return fmt.Errorf("unsupported native link target %s/%s", m.GOOS, m.GOARCH)
	}
	return nil
}

// OpenNativeInputs verifies all files before returning a target component.
func OpenNativeInputs(path string) (*NativeInputs, error) {
	var m NativeInputs
	if err := nativeReadJSON(path, &m); err != nil {
		return nil, err
	}
	var err error
	m.root, err = filepath.Abs(filepath.Dir(path))
	if err != nil {
		return nil, err
	}
	if err = nativeVerifyFiles(m.root, m.Files, "inputs.json"); err != nil {
		return nil, err
	}
	if err = m.validate(); err != nil {
		return nil, err
	}
	return &m, nil
}

// OpenNativeTools verifies the current host and all tool/runtime files.
func OpenNativeTools(path string) (*NativeTools, error) {
	var m NativeTools
	if err := nativeReadJSON(path, &m); err != nil {
		return nil, err
	}
	if m.Format != 1 || m.GOOS != runtime.GOOS || m.GOARCH != runtime.GOARCH || !nativeProvenanceValid(m.Provenance) {
		return nil, fmt.Errorf("native tools do not match host %s/%s", runtime.GOOS, runtime.GOARCH)
	}
	var err error
	m.root, err = filepath.Abs(filepath.Dir(path))
	if err != nil {
		return nil, err
	}
	if err = nativeVerifyFiles(m.root, m.Files, "tools.json"); err != nil {
		return nil, err
	}
	for _, tool := range []NativeTool{m.LLD} {
		if _, ok := m.Files[tool.File]; !ok || tool.Version == "" {
			return nil, fmt.Errorf("native tool is not declared: %s", tool.File)
		}
	}
	return &m, nil
}

func (m *NativeInputs) directory(name string) (string, error) {
	path, err := baseFile(m.root, name)
	if err != nil {
		return "", err
	}
	info, err := os.Stat(path)
	if err != nil {
		return "", err
	}
	if !info.IsDir() {
		return "", fmt.Errorf("native input directory expected: %s", name)
	}
	return path, nil
}

func (m *NativeInputs) file(name string) (string, error) {
	if _, ok := m.Files[name]; !ok {
		return "", fmt.Errorf("native input is not declared: %s", name)
	}
	return baseFile(m.root, name)
}

func (m *NativeInputs) library(name string) (string, error) {
	if name == "" || strings.IndexFunc(name, func(r rune) bool {
		return !(r >= 'a' && r <= 'z' || r >= 'A' && r <= 'Z' || r >= '0' && r <= '9' || r == '_' || r == '.' || r == '-')
	}) >= 0 {
		return "", fmt.Errorf("invalid native library %q", name)
	}
	dir := ""
	suffixes := []string{".so", ".a"}
	if m.Android != nil {
		dir = m.Android.LibraryDir
	} else if m.IOS != nil {
		dir = m.IOS.Sysroot + "/usr/lib"
		suffixes = []string{".tbd", ".a"}
	}
	for _, suffix := range suffixes {
		file := dir + "/lib" + name + suffix
		if _, ok := m.Files[file]; ok {
			return m.file(file)
		}
	}
	return "", fmt.Errorf("native library %s is not in target component", name)
}

// ExportNativeInputs seals selected target inputs from a prepared source tree.
func ExportNativeInputs(specPath, source, output string) error {
	var m NativeInputs
	if err := nativeReadJSON(specPath, &m); err != nil {
		return err
	}
	return nativeExport(source, output, "inputs.json", m.Files, &m, func(path string) error { _, err := OpenNativeInputs(path); return err })
}

// ExportNativeTools seals host tools and checks their declared versions.
func ExportNativeTools(ctx context.Context, specPath, source, output string) error {
	var m NativeTools
	if err := nativeReadJSON(specPath, &m); err != nil {
		return err
	}
	return nativeExport(source, output, "tools.json", m.Files, &m, func(path string) error {
		tools, err := OpenNativeTools(path)
		if err != nil {
			return err
		}
		command := exec.CommandContext(ctx, filepath.Join(tools.root, filepath.FromSlash(tools.LLD.File)), "-flavor", "gnu", "--version")
		data, err := command.CombinedOutput()
		if err != nil {
			return fmt.Errorf("check tool %s: %w: %s", tools.LLD.File, err, data)
		}
		if !nativeLLDVersion(string(data), tools.LLD.Version) {
			return fmt.Errorf("tool %s: version mismatch, expected %s", tools.LLD.File, tools.LLD.Version)
		}
		return nil
	})
}

func nativeExport(source, output, name string, files map[string]string, manifest any, verify func(string) error) (err error) {
	source, err = filepath.Abs(source)
	if err != nil {
		return err
	}
	output, err = filepath.Abs(output)
	if err != nil {
		return err
	}
	if _, err = os.Stat(output); err == nil {
		return fmt.Errorf("output already exists: %s", output)
	} else if !os.IsNotExist(err) {
		return err
	}
	if err = os.MkdirAll(filepath.Dir(output), 0755); err != nil {
		return err
	}
	temp, err := os.MkdirTemp(filepath.Dir(output), ".goui-native-*")
	if err != nil {
		return err
	}
	defer os.RemoveAll(temp)
	for file, expected := range files {
		path, err := baseFile(source, file)
		if err != nil {
			return err
		}
		actual, err := baseHash(path)
		if err != nil {
			return err
		}
		if expected != "" && actual != expected {
			return fmt.Errorf("source input %s: SHA-256 mismatch", file)
		}
		target := filepath.Join(temp, filepath.FromSlash(file))
		if err = os.MkdirAll(filepath.Dir(target), 0755); err != nil {
			return err
		}
		if err = baseCopy(path, target); err != nil {
			return err
		}
		info, err := os.Stat(path)
		if err != nil {
			return err
		}
		if err = os.Chmod(target, info.Mode().Perm()); err != nil {
			return err
		}
		files[file] = actual
	}
	data, err := json.MarshalIndent(manifest, "", "  ")
	if err != nil {
		return err
	}
	path := filepath.Join(temp, name)
	if err = os.WriteFile(path, append(data, '\n'), 0644); err != nil {
		return err
	}
	if err = verify(path); err != nil {
		return err
	}
	return os.Rename(temp, output)
}

func nativeLLDVersion(output, expected string) bool {
	fields := strings.Fields(output)
	for i, field := range fields {
		if field == "LLD" && i+1 < len(fields) {
			return fields[i+1] == expected
		}
	}
	return false
}
