// Package nativegen writes maintainer-only native sources embedded in Go generators.
package nativegen

import (
	"bytes"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// Main writes sources beneath GOUI_GENERATE_DIR/component. Consumer builds do
// not run generators or require native tools. Source generators use ordinary Go.
func Main(component string, sources map[string]string) {
	root := os.Getenv("GOUI_GENERATE_DIR")
	if root == "" || !filepath.IsAbs(root) {
		log.Fatal("set GOUI_GENERATE_DIR to an absolute maintainer output directory")
	}
	cwd, err := os.Getwd()
	if err != nil {
		log.Fatal(err)
	}
	for dir := cwd; ; dir = filepath.Dir(dir) {
		if data, err := os.ReadFile(filepath.Join(dir, "go.mod")); err == nil && strings.HasPrefix(strings.Join(strings.Fields(string(data)), " "), "module github.com/golang-gui/goui ") {
			if err = CheckOutput(root, dir); err != nil {
				log.Fatal(err)
			}
			break
		}
		if filepath.Dir(dir) == dir {
			log.Fatal("generator must run inside goui repository")
		}
	}
	if err := Write(root, component, sources); err != nil {
		log.Fatal(err)
	}
	fmt.Println(filepath.Join(root, component))
}

// Write materializes only the declared files; unchanged files retain timestamps.
func Write(root, component string, sources map[string]string) error {
	if !filepath.IsLocal(component) {
		return fmt.Errorf("invalid component path %q", component)
	}
	names := make([]string, 0, len(sources))
	for name := range sources {
		if !filepath.IsLocal(name) {
			return fmt.Errorf("invalid source path %q", name)
		}
		names = append(names, name)
	}
	sort.Strings(names)
	for _, name := range names {
		path := filepath.Join(root, component, name)
		data := []byte(sources[name])
		if old, err := os.ReadFile(path); err == nil && bytes.Equal(old, data) {
			continue
		}
		if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
			return err
		}
		if err := os.WriteFile(path, data, 0644); err != nil {
			return err
		}
	}
	return nil
}

// CheckOutput rejects source-tree output, including paths through symlinks.
func CheckOutput(output, repository string) error {
	canonical := func(path string) (string, error) {
		path, err := filepath.Abs(path)
		if err != nil {
			return "", err
		}
		var tail []string
		for {
			p, err := filepath.EvalSymlinks(path)
			if err == nil {
				for i := len(tail) - 1; i >= 0; i-- {
					p = filepath.Join(p, tail[i])
				}
				return p, nil
			}
			if !os.IsNotExist(err) {
				return "", err
			}
			tail = append(tail, filepath.Base(path))
			parent := filepath.Dir(path)
			if parent == path {
				return "", err
			}
			path = parent
		}
	}
	output, err := canonical(output)
	if err != nil {
		return err
	}
	repository, err = canonical(repository)
	if err != nil {
		return err
	}
	rel, err := filepath.Rel(repository, output)
	if err != nil {
		return err
	}
	if rel == "." || filepath.IsLocal(rel) {
		return fmt.Errorf("native generator output must be outside repository")
	}
	return nil
}
