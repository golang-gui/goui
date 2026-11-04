package mobilebase

import (
	"bytes"
	"context"
	"crypto/sha256"
	_ "embed"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
)

// The complete replacement sources are embedded, so building the driver never
// edits the user's GOROOT or requires a separate Go repository checkout.
//
//go:embed base.go
var baseSource []byte

//go:embed hook.go.txt
var hookSource []byte

//go:embed entry.go
var entrySourceFile []byte

const baseGoVersion = "go1.25.8"
const execSourceSHA256 = "0bb5e63cec9ee583f15d5c20bdaf356df9059a1ca15b4d4177a46549594d86f1"

// BuildDriver builds the narrowly adapted cmd/go for the supplied toolchain.
// Go's compiler, assembler, linker, runtime and ordinary build actions are intact.
func BuildDriver(ctx context.Context, goCommand, output string, log io.Writer) error {
	command := exec.CommandContext(ctx, goCommand, "env", "-json", "GOVERSION", "GOROOT")
	command.Env = baseDriverEnv()
	data, err := command.Output()
	if err != nil {
		return fmt.Errorf("inspect Go toolchain: %w", err)
	}
	var info struct{ GOVERSION, GOROOT string }
	if err = json.Unmarshal(data, &info); err != nil {
		return err
	}
	if info.GOVERSION != baseGoVersion {
		return fmt.Errorf("mobile base needs %s, found %s", baseGoVersion, info.GOVERSION)
	}
	src := filepath.Join(info.GOROOT, "src/cmd/go/internal/work/exec.go")
	data, err = os.ReadFile(src)
	if err != nil {
		return err
	}
	sum := sha256.Sum256(data)
	if fmt.Sprintf("%x", sum) != execSourceSHA256 {
		return fmt.Errorf("unsupported cmd/go source: %s", src)
	}
	marker := []byte("\tp := a.Package\n\tsh := b.Shell(a)\n\n\tbit := ")
	if bytes.Count(data, marker) != 1 {
		return fmt.Errorf("cmd/go build hook is ambiguous")
	}
	replacement := []byte("\tp := a.Package\n\tif handled, err := b.gouiBaseAction(a); handled || err != nil { return err }\n\tsh := b.Shell(a)\n\n\tbit := ")
	temp, err := os.MkdirTemp("", "goui-base-driver-*")
	if err != nil {
		return err
	}
	defer os.RemoveAll(temp)
	files := map[string][]byte{
		"exec.go":           bytes.Replace(data, marker, replacement, 1),
		"goui_base.go":      bytes.Replace(baseSource, []byte("package mobilebase"), []byte("package work"), 1),
		"goui_base_hook.go": hookSource,
		"goui_entry.go":     bytes.Replace(entrySourceFile, []byte("package mobilebase"), []byte("package work"), 1),
	}
	overlay := struct{ Replace map[string]string }{make(map[string]string)}
	for name, content := range files {
		dest := filepath.Join(temp, name)
		if err = os.WriteFile(dest, content, 0600); err != nil {
			return err
		}
		overlay.Replace[filepath.Join(info.GOROOT, "src/cmd/go/internal/work", name)] = dest
	}
	data, err = json.Marshal(overlay)
	if err != nil {
		return err
	}
	overlayFile := filepath.Join(temp, "overlay.json")
	if err = os.WriteFile(overlayFile, data, 0600); err != nil {
		return err
	}
	output, err = filepath.Abs(output)
	if err != nil {
		return err
	}
	command = exec.CommandContext(ctx, goCommand, "build", "-trimpath", "-overlay="+overlayFile, "-o", output, "cmd/go")
	command.Dir = temp
	// Downloaded Go toolchains live under the user's module cache. Tool generation
	// uses only GOROOT packages, so give this invocation its own empty module cache;
	// the overlay then belongs to the toolchain, not to application dependencies.
	command.Env = buildEnv(baseDriverEnv(), map[string]string{"GOMODCACHE": filepath.Join(temp, "modules")})
	command.Stdout = log
	command.Stderr = log
	if err = command.Run(); err != nil {
		return fmt.Errorf("build mobile Go driver: %w", err)
	}
	return nil
}

func baseDriverEnv() []string {
	// Toolchain generation is always a host build and must not recurse into a
	// selected target base, project workspace, or inherited target compiler flags.
	var env []string
	for _, item := range os.Environ() {
		key, _, _ := bytes.Cut([]byte(item), []byte("="))
		switch string(key) {
		case "GOOS", "GOARCH", "GOARM64", "GOAMD64", "GOEXPERIMENT", "GOFLAGS", "GOWORK", "CGO_ENABLED", "GOTOOLCHAIN", "GOUI_BASE", "GOENV":
			continue
		}
		env = append(env, item)
	}
	return append(env, "GOWORK=off", "CGO_ENABLED=0", "GOTOOLCHAIN=local", "GOFLAGS=", "GOENV=off", "GOOS="+runtime.GOOS, "GOARCH="+runtime.GOARCH, "GOARM64=v8.0", "GOAMD64=v1", "GOEXPERIMENT=")
}

func buildEnv(env []string, values map[string]string) []string {
	result := make([]string, 0, len(env)+len(values))
	for _, entry := range env {
		key, _, _ := strings.Cut(entry, "=")
		replaced := false
		for name := range values {
			if strings.EqualFold(key, name) {
				replaced = true
				break
			}
		}
		if !replaced {
			result = append(result, entry)
		}
	}
	for key, value := range values {
		result = append(result, key+"="+value)
	}
	return result
}
