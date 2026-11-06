package base

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

var hookSource = []byte(`package work

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"sync"

	"cmd/go/internal/cfg"
	"cmd/go/internal/fsys"
	"cmd/internal/buildid"
)

var gouiBaseOnce sync.Once
var gouiBaseManifest *BaseManifest
var gouiBaseError error
var gouiEntryOnce sync.Once
var gouiEntryDir string
var gouiEntryError error

// Source exports use workspace-style file paths for their whole module. This
// changes debug/export paths only; actual module identity and build info remain
// intact, and source/dependency ABI checks below are still mandatory.
func gouiSourceModule(path string) bool {
	manifest := os.Getenv("GOUI_BASE")
	if manifest == "" { return false }
	gouiBaseOnce.Do(func() { gouiBaseManifest, gouiBaseError = BaseOpen(manifest) })
	if gouiBaseError != nil || gouiBaseManifest.Format != 1 { return false }
	for _, record := range gouiBaseManifest.Packages {
		if record.ModulePath == path && record.ModuleVersion == "" { return true }
	}
	return false
}

func (b *Builder) gouiBaseAction(a *Action) (bool, error) {
	path := os.Getenv("GOUI_BASE")
	if path == "" {
		return false, nil
	}
	gouiBaseOnce.Do(func() { gouiBaseManifest, gouiBaseError = BaseOpen(path) })
	if gouiBaseError != nil {
		return false, gouiBaseError
	}
	m := gouiBaseManifest
	arch := cfg.GOARM64
	if cfg.Goarch == "amd64" {
		arch = cfg.GOAMD64
	}
	if runtime.Version() != m.GoVersion || cfg.Goos != m.GOOS || cfg.Goarch != m.GOARCH || cfg.BuildBuildmode != m.BuildMode || arch != m.Architecture || cfg.RawGOEXPERIMENT != m.Experiment {
		return false, fmt.Errorf("base toolchain/target mismatch: need %s %s/%s %s (%s, experiments=%q)", m.GoVersion, m.GOOS, m.GOARCH, m.BuildMode, m.Architecture, m.Experiment)
	}
	if !cfg.BuildTrimpath || cfg.BuildRace || cfg.BuildMSan || cfg.BuildASan || cfg.BuildCover {
		return false, fmt.Errorf("base requires -trimpath and no race/msan/asan/coverage instrumentation")
	}
	if fsys.OverlayFile != "" {
		gouiEntryOnce.Do(func() { gouiEntryDir, gouiEntryError = ValidateEntryOverlay(fsys.OverlayFile) })
		if gouiEntryError != nil {
			return false, gouiEntryError
		}
		if a.Package.Dir == gouiEntryDir && a.Package.Name != "main" {
			return false, fmt.Errorf("entry overlay may only extend a main package")
		}
	}
	p := a.Package
	if m.Format == 2 && !p.UsesCgo() && !p.UsesSwig() { return false, nil }
	if m.Format == 2 {
		for key, expected := range m.CgoFlags {
			actual := cfg.Getenv(key)
			if actual == "" && key != "CGO_CPPFLAGS" { actual = "-O2 -g" }
			if actual != expected { return false, fmt.Errorf("object recipe does not match %s", key) }
		}
	}
	record, found := m.Packages[p.ImportPath]
	if !found {
		if p.UsesCgo() || p.UsesSwig() {
			return false, fmt.Errorf("native support missing for %s; this build never invokes a C compiler", p.ImportPath)
		}
		return false, nil
	}
	if p.Error != nil {
		return true, p.Error
	}
	if !p.UsesCgo() && !p.UsesSwig() {
		return true, fmt.Errorf("base %s: only native-required packages may be precompiled", p.ImportPath)
	}
	if p.Name == "main" || len(p.Internal.Gcflags) != 0 || len(p.Internal.Asmflags) != 0 || p.Internal.PGOProfile != "" || a.needVet || b.NeedCompiledGoFiles || b.needCgoHdr(a) {
		return true, fmt.Errorf("base package %s: unsupported build flags or auxiliary output", p.ImportPath)
	}
	modulePath, moduleVersion, moduleGoVersion := "", "", ""
	if p.Module != nil {
		modulePath = p.Module.Path
		moduleVersion = p.Module.Version
		moduleGoVersion = p.Module.GoVersion
	}
	moduleRecord := record
	// Object recipes compile all Go code now. Their producer release tag is
	// provenance; source/language/native inputs define compatibility.
	if m.Format == 2 { moduleRecord.ModuleVersion = "" }
	if !baseModuleMatches(moduleRecord, modulePath, moduleVersion, moduleGoVersion) {
		return true, fmt.Errorf("base %s: module identity mismatch", p.ImportPath)
	}
	source, err := BaseSourceHash(p.Dir, p.GoFiles, p.CgoFiles, p.CFiles, p.CXXFiles, p.MFiles, p.HFiles, p.FFiles, p.SFiles, p.SysoFiles, p.SwigFiles, p.SwigCXXFiles, p.EmbedFiles)
	if err != nil {
		return true, err
	}
	if source != record.SourceSHA256 {
		return true, fmt.Errorf("base %s: selected source mismatch", p.ImportPath)
	}
	if m.Format == 2 { return false, nil }
	for name, expected := range record.Imports {
		var target string
		for _, dep := range a.Deps {
			if dep.Package != nil && dep.Package.ImportPath == name {
				target = dep.built
				break
			}
		}
		if target == "" {
			return true, fmt.Errorf("base %s: missing dependency %s", p.ImportPath, name)
		}
		actual, err := BaseExportHash(target)
		if err != nil {
			return true, err
		}
		if actual != expected {
			return true, fmt.Errorf("base %s: dependency ABI mismatch: %s", p.ImportPath, name)
		}
	}
	id, err := buildid.ReadFile(record.Archive)
	if err != nil {
		return true, err
	}
	if id != record.BuildID {
		return true, fmt.Errorf("base %s: build ID mismatch", p.ImportPath)
	}
	a.built = record.Archive
	a.buildID = id
	if b.NeedExport {
		p.Export = a.built
		p.BuildID = id
	}
	if cfg.BuildX {
		b.Shell(a).ShowCmd("", "# goui base: %s => %s", p.ImportPath, a.built)
	}
	return true, nil
}
`)

//go:embed entry.go
var entrySourceFile []byte

const baseGoVersion = "go1.25.8"
const execSourceSHA256 = "0bb5e63cec9ee583f15d5c20bdaf356df9059a1ca15b4d4177a46549594d86f1"
const gcSourceSHA256 = "e2b4e705e110f7c78f9d64dbb85898a64dcc34e98a93e655726d1d39e33e3fd1"

type driverProfile struct{ execHash, gcHash, actionHash string }

var driverProfiles = map[string]driverProfile{
	"go1.24.13": {"93cf6c9ea6312ad93df5114c4a180d58238734fef4406b7c34a796c6a638e4ba", "4ece29590b91fe800c31d67ed20701ce9b6df5e287f93fcc22d59f891710e18b", ""},
	"go1.25.8":  {execSourceSHA256, gcSourceSHA256, ""},
	"go1.26.10": {"7f3f0ddd8b29ca2cb5dcc7cca5396bc66bfb9f9e2d743b549eaf5555c943eae2", "7b91523777a11587412b7d4e35f965d771ca7bde60ca6d65d54206171544819f", "7a5819d218710e08a059946c9b0888d15d38724188b705d56ab8118254bab60b"},
	"go1.27.3":  {"2a0a4595371bec5f40eea4c4712a4bd61cb5713556ad5a819c5b321ef445f526", "13359b8707e78300e8e0ec880b9a8d1af4bafdf32140b56df5f041a52fbc1325", "7e264948cdf181f50560cbb21c95b2c526611923ea2e11d859a60f0962e786ec"},
}

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
	profile, ok := driverProfiles[info.GOVERSION]
	if !ok {
		return fmt.Errorf("unsupported mobile Go driver version %s", info.GOVERSION)
	}
	split := profile.actionHash != ""
	src := filepath.Join(info.GOROOT, "src/cmd/go/internal/work/exec.go")
	data, err = os.ReadFile(src)
	if err != nil {
		return err
	}
	sum := sha256.Sum256(data)
	if fmt.Sprintf("%x", sum) != profile.execHash {
		return fmt.Errorf("unsupported cmd/go source: %s", src)
	}
	marker := []byte("\tp := a.Package\n\tsh := b.Shell(a)\n\n\tbit := ")
	replacement := []byte("\tp := a.Package\n\tif handled, err := b.gouiBaseAction(a); handled || err != nil { return err }\n\tsh := b.Shell(a)\n\n\tbit := ")
	if split {
		marker = []byte("\tp := buildAction.Package\n\tsh := b.Shell(a)\n")
		replacement = []byte("\tp := buildAction.Package\n\tif _, err := b.gouiBaseAction(buildAction); err != nil { return nil, err }\n\tsh := b.Shell(a)\n")
	}
	if bytes.Count(data, marker) != 1 {
		return fmt.Errorf("cmd/go build hook is ambiguous")
	}
	cacheMarker := []byte(`fmt.Fprintf(h, "module %s@%s\n", p.Module.Path, p.Module.Version)`)
	if bytes.Count(data, cacheMarker) != 1 {
		return fmt.Errorf("cmd/go source-path cache hook is ambiguous")
	}
	data = bytes.Replace(data, cacheMarker, append(append([]byte(nil), cacheMarker...), []byte(`
			if gouiSourceModule(p.Module.Path) { fmt.Fprintln(h, "goui source-module trimpath v1") }`)...), 1)
	data, err = recipeDriverSource(data, split)
	if err != nil {
		return err
	}
	gcSource, err := os.ReadFile(filepath.Join(info.GOROOT, "src/cmd/go/internal/work/gc.go"))
	if err != nil {
		return err
	}
	gcHash := sha256.Sum256(gcSource)
	if fmt.Sprintf("%x", gcHash) != profile.gcHash {
		return fmt.Errorf("unsupported cmd/go compiler source")
	}
	gcMarker := []byte(`if m := a.Package.Module; m != nil && m.Version != "" {`)
	if bytes.Count(gcSource, gcMarker) != 1 {
		return fmt.Errorf("cmd/go trimpath hook is ambiguous")
	}
	gcSource = bytes.Replace(gcSource, gcMarker, []byte(`if m := a.Package.Module; m != nil && m.Version != "" && !gouiSourceModule(m.Path) {`), 1)
	temp, err := os.MkdirTemp("", "goui-base-driver-*")
	if err != nil {
		return err
	}
	defer os.RemoveAll(temp)
	files := map[string][]byte{
		"goui_recipe.go":    bytes.Replace(recipeSource, []byte("package base"), []byte("package work"), 1),
		"gc.go":             gcSource,
		"exec.go":           bytes.Replace(data, marker, replacement, 1),
		"goui_base.go":      bytes.Replace(baseSource, []byte("package base"), []byte("package work"), 1),
		"goui_base_hook.go": append(append([]byte(nil), hookSource...), recipeHook...),
		"goui_entry.go":     bytes.Replace(entrySourceFile, []byte("package base"), []byte("package work"), 1),
	}
	if split {
		action, err := os.ReadFile(filepath.Join(info.GOROOT, "src/cmd/go/internal/work/action.go"))
		if err != nil {
			return err
		}
		h := sha256.Sum256(action)
		if fmt.Sprintf("%x", h) != profile.actionHash {
			return fmt.Errorf("unsupported cmd/go action source")
		}
		if files["action.go"], err = recipeActionSource(action); err != nil {
			return err
		}
		files["goui_base_hook.go"] = append(files["goui_base_hook.go"], splitRecipeHook...)
		// Path normalization exists only for the legacy archive consumer.
		delete(files, "gc.go")
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
		case "GOOS", "GOARCH", "GOARM64", "GOAMD64", "GOEXPERIMENT", "GOFLAGS", "GOWORK", "CGO_ENABLED", "GOTOOLCHAIN", "GOUI_BASE", "GOUI_RECORD_BASE", "GOENV":
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
