package base

import (
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// BaseFile identifies generated Go source, a native object or an auxiliary
// header. Name preserves the original package-local build filename.
type BaseFile struct {
	Name   string `json:"name"`
	Path   string `json:"path"`
	SHA256 string `json:"sha256"`
}

func recipeDigest(data []byte) string {
	h := sha256.Sum256(data)
	return hex.EncodeToString(h[:])
}

func nativeObject(data []byte) bool {
	return len(data) >= 4 && (string(data[:4]) == "\x7fELF" || string(data[:4]) == "\xcf\xfa\xed\xfe")
}

func verifyRecipe(m *BaseManifest, name string, p BasePackage) error {
	if name == "" || len(p.SourceSHA256) != 64 || p.Archive != "" || p.BuildID != "" || len(p.Imports) != 0 || len(p.GeneratedGo) == 0 || len(p.Objects) == 0 {
		return fmt.Errorf("base %s: incomplete object recipe", name)
	}
	seen := map[string]bool{}
	for group, files := range [][]BaseFile{p.GeneratedGo, p.Objects, p.Auxiliary} {
		for _, f := range files {
			if f.Name == "" || f.Name == "." || f.Name == ".." || !filepath.IsLocal(f.Name) || filepath.Base(f.Name) != f.Name || strings.ContainsAny(f.Name, "/\\:\x00\r\n") || seen[f.Name] || len(f.SHA256) != 64 {
				return fmt.Errorf("base %s: invalid recipe file %q", name, f.Name)
			}
			seen[f.Name] = true
			path, err := baseFile(m.root, f.Path)
			if err != nil {
				return err
			}
			data, err := os.ReadFile(path)
			if err != nil {
				return err
			}
			if recipeDigest(data) != f.SHA256 {
				return fmt.Errorf("base %s: recipe checksum mismatch: %s", name, f.Name)
			}
			if group == 0 && !strings.HasSuffix(f.Name, ".go") || group == 1 && (!strings.HasSuffix(f.Name, ".o") || !nativeObject(data)) || group == 2 && f.Name != "_cgo_export.h" {
				return fmt.Errorf("base %s: invalid recipe file kind: %s", name, f.Name)
			}
			if group == 1 && !recipeObjectTarget(data, m.GOOS, m.GOARCH) {
				return fmt.Errorf("base %s: native object target mismatch: %s", name, f.Name)
			}
		}
	}
	return nil
}

func recipeObjectTarget(data []byte, goos, arch string) bool {
	if len(data) < 32 {
		return false
	}
	le := binary.LittleEndian
	if string(data[:4]) == "\x7fELF" {
		machine := uint16(183)
		if arch == "amd64" {
			machine = 62
		}
		return goos != "ios" && data[4] == 2 && data[5] == 1 && le.Uint16(data[16:]) == 1 && le.Uint16(data[18:]) == machine
	}
	return goos == "ios" && arch == "arm64" && le.Uint32(data) == 0xfeedfacf && le.Uint32(data[4:]) == 0x100000c && le.Uint32(data[12:]) == 1
}

type cgoRecord struct {
	Package BasePackage `json:"package"`
	Path    string      `json:"import_path"`
}

func recordCgoFiles(directory string, paths []string) ([]BaseFile, error) {
	var files []BaseFile
	for _, path := range paths {
		data, err := os.ReadFile(path)
		if err != nil {
			return nil, err
		}
		hash := recipeDigest(data)
		rel := "objects/" + hash
		if err = os.MkdirAll(filepath.Join(directory, "objects"), 0700); err != nil {
			return nil, err
		}
		if err = os.WriteFile(filepath.Join(directory, rel), data, 0600); err != nil {
			return nil, err
		}
		files = append(files, BaseFile{Name: filepath.Base(path), Path: rel, SHA256: hash})
	}
	return files, nil
}

func writeCgoRecord(directory, name string, p BasePackage, generated, objects, auxiliary []string) error {
	var err error
	if p.GeneratedGo, err = recordCgoFiles(directory, generated); err != nil {
		return err
	}
	if p.Objects, err = recordCgoFiles(directory, objects); err != nil {
		return err
	}
	if p.Auxiliary, err = recordCgoFiles(directory, auxiliary); err != nil {
		return err
	}
	data, err := json.Marshal(cgoRecord{Package: p, Path: name})
	if err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(directory, recipeDigest([]byte(name))+".json"), data, 0600)
}
