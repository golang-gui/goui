package mobilebase

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
)

func createEntryOverlay(ctx context.Context, options BuildOptions, mode string, env []string, temporary string) (string, error) {
	// Only locate this package. Resolving cgo imports through list would request
	// auxiliary generated sources, which the archive consumer deliberately lacks.
	command := exec.CommandContext(ctx, options.GoDriver, "list", "-find", "-json", "-trimpath", "-pgo=off", "-buildmode="+mode, options.Package)
	command.Dir, command.Env = options.Directory, env
	var stderr bytes.Buffer
	command.Stderr = &stderr
	data, err := command.Output()
	if err != nil {
		if ctx.Err() != nil {
			return "", ctx.Err()
		}
		return "", fmt.Errorf("locate main package: %w\n%s", err, &stderr)
	}
	var pkg struct{ Name, Dir, ImportPath string }
	if err = json.Unmarshal(data, &pkg); err != nil {
		return "", err
	}
	if pkg.Name != "main" || !filepath.IsAbs(pkg.Dir) || pkg.ImportPath == "command-line-arguments" {
		return "", fmt.Errorf("mobile entry requires one main package, not a Go file list")
	}
	source := filepath.Join(temporary, "entry.go")
	if err = os.WriteFile(source, []byte(entrySource), 0600); err != nil {
		return "", err
	}
	overlay := struct{ Replace map[string]string }{map[string]string{filepath.Join(pkg.Dir, entryFileName): source}}
	data, err = json.Marshal(overlay)
	if err != nil {
		return "", err
	}
	path := filepath.Join(temporary, "entry-overlay.json")
	if err = os.WriteFile(path, data, 0600); err != nil {
		return "", err
	}
	if _, err = ValidateEntryOverlay(path); err != nil {
		return "", err
	}
	return path, nil
}
