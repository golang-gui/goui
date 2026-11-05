package base

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// BuildOptions select independent local components and a pure Go main package.
// Output is an Android shared library or a final iOS native executable, not an
// installation package. Objects, Libraries and Frameworks belong to the caller's
// platform host; they are never inferred from GUI types. Paths are relative to
// the caller's current directory; Directory selects the Go package directory.
type BuildOptions struct {
	GoCommand  string
	GoDriver   string
	Linker     string
	Base       string
	Inputs     string
	Tools      string
	Directory  string
	Package    string
	Output     string
	Objects    []string
	Libraries  []string
	Frameworks []string
	Verbose    bool
	Tags       string
	LDFlags    []string
	// Entry registers the application's main for the fixed mobile ABI entry.
	Entry bool
}

// Build compiles ordinary Go source with verified native support archives, then
// links with the selected target inputs and host tools. No C compiler is used.
// It never changes the process environment or publishes an incomplete output.
func Build(ctx context.Context, options BuildOptions, out, errOut io.Writer) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	if err := validateBuildFlags(options.Tags, options.LDFlags); err != nil {
		return err
	}
	if options.Output == "" || options.GoDriver == "" || options.Linker == "" {
		return fmt.Errorf("build requires output, Go driver and link driver")
	}
	if options.GoCommand == "" {
		options.GoCommand = "go"
	}
	if options.Package == "" {
		options.Package = "."
	}
	if strings.HasPrefix(options.Package, "-") {
		return fmt.Errorf("invalid main package %q", options.Package)
	}
	var err error
	for _, path := range []*string{&options.Base, &options.Inputs, &options.Tools, &options.Output} {
		if *path == "" {
			return fmt.Errorf("build requires base, input and tool manifests")
		}
		*path, err = filepath.Abs(*path)
		if err != nil {
			return err
		}
	}
	for _, path := range []*string{&options.GoCommand, &options.GoDriver, &options.Linker} {
		*path, err = exec.LookPath(*path)
		if err != nil {
			return err
		}
		*path, err = filepath.Abs(*path)
		if err != nil {
			return err
		}
	}
	base, err := BaseOpen(options.Base)
	if err != nil {
		return err
	}
	inputs, err := OpenNativeInputs(options.Inputs)
	if err != nil {
		return err
	}
	tools, err := OpenNativeTools(options.Tools)
	if err != nil {
		return err
	}
	if base.GOOS != inputs.GOOS || base.GOARCH != inputs.GOARCH || base.Variant != inputs.Variant || base.MinimumVersion != inputs.MinimumVersion {
		return fmt.Errorf("native base and link inputs target/variant/minimum mismatch")
	}
	if base.Architecture != "v8.0" || base.Experiment != "" {
		return fmt.Errorf("build requires baseline ARM64 without experiments")
	}
	if inputs.GOOS == "ios" && len(options.Objects) == 0 {
		return fmt.Errorf("iOS build requires an explicit native host object")
	}
	extras, err := buildNativeExtras(&options, inputs)
	if err != nil {
		return err
	}
	if err = verifyNativeToolVersions(ctx, tools); err != nil {
		if ctx.Err() != nil {
			return ctx.Err()
		}
		return err
	}
	env, err := buildEnvironment(ctx, options, base)
	if err != nil {
		return err
	}

	if err = os.MkdirAll(filepath.Dir(options.Output), 0755); err != nil {
		return err
	}
	temporary, err := os.MkdirTemp(filepath.Dir(options.Output), ".goui-build-*")
	if err != nil {
		return err
	}
	defer os.RemoveAll(temporary)
	// Keep the final basename: LLD uses it for the Mach-O signature identifier.
	output := filepath.Join(temporary, filepath.Base(options.Output))
	goOutput := output
	flags := []string{"-linkmode=external"}
	flags = append(flags, options.LDFlags...)
	if inputs.GOOS == "android" {
		// cmd/go already forwards CC/CXX to cmd/link's -extld. Supplying a
		// compiler argv here avoids a second, lossy -ldflags quoting layer.
		if len(extras) > 0 {
			response := filepath.Join(temporary, "host.rsp")
			var data strings.Builder
			for _, arg := range extras {
				fmt.Fprintf(&data, "%q\n", arg)
			}
			if err = os.WriteFile(response, []byte(data.String()), 0600); err != nil {
				return err
			}
			compiler, err := buildQuoted([]string{options.Linker, "@" + response})
			if err != nil {
				return err
			}
			env = buildEnv(env, map[string]string{"CC": compiler, "CXX": compiler})
		}
	} else {
		goOutput = filepath.Join(temporary, "go.a")
		flags = append(flags, "-extar="+options.Linker)
	}

	encoded, err := buildQuoted(flags)
	if err != nil {
		return err
	}
	args := []string{"build", "-trimpath", "-pgo=off", "-buildmode=" + base.BuildMode, "-ldflags=" + encoded, "-o", goOutput}
	if options.Tags != "" {
		args = append(args, "-tags="+options.Tags)
	}
	if options.Entry {
		overlay, err := createEntryOverlay(ctx, options, base.BuildMode, env, temporary)
		if err != nil {
			return err
		}
		args = append(args, "-overlay="+overlay)
	}
	if options.Verbose {
		args = append(args, "-x")
	}
	args = append(args, options.Package)
	command := exec.CommandContext(ctx, options.GoDriver, args...)
	command.Dir, command.Env, command.Stdout, command.Stderr = options.Directory, env, out, errOut
	if err = command.Run(); err != nil {
		if ctx.Err() != nil {
			return ctx.Err()
		}
		return fmt.Errorf("build Go source: %w", err)
	}
	if inputs.GOOS == "ios" {
		args = []string{"-o", output, goOutput}
		args = append(args, extras...)
		if err = RunNativeLink(ctx, options.Inputs, options.Tools, args, out, errOut); err != nil {
			if ctx.Err() != nil {
				return ctx.Err()
			}
			return err
		}
	}
	if err = CheckNativeBinary(output, inputs); err != nil {
		return fmt.Errorf("verify built output: %w", err)
	}
	if err = ctx.Err(); err != nil {
		return err
	}
	return os.Rename(output, options.Output)
}

func validateBuildFlags(tags string, flags []string) error {
	if strings.ContainsAny(tags, "\x00\r\n") {
		return fmt.Errorf("invalid build tags")
	}
	for i := 0; i < len(flags); i++ {
		switch flags[i] {
		case "-s", "-w":
		case "-X":
			i++
			if i == len(flags) || !strings.Contains(flags[i], "=") || strings.ContainsAny(flags[i], "\x00\r\n") {
				return fmt.Errorf("-X requires importpath.name=value")
			}
		default:
			return fmt.Errorf("unsupported mobile Go linker flag %q (supported: -s, -w, -X)", flags[i])
		}
	}
	return nil
}

func buildNativeExtras(options *BuildOptions, inputs *NativeInputs) ([]string, error) {
	var args []string
	for _, name := range options.Objects {
		full, err := filepath.Abs(name)
		if err != nil {
			return nil, err
		}
		ext := strings.ToLower(filepath.Ext(full))
		if ext != ".o" && ext != ".a" {
			return nil, fmt.Errorf("native host object/archive expected: %s", name)
		}
		info, err := os.Stat(full)
		if err != nil {
			return nil, err
		}
		if !info.Mode().IsRegular() {
			return nil, fmt.Errorf("native host input is not a file: %s", name)
		}
		args = append(args, full)
	}
	for _, name := range options.Libraries {
		if _, err := inputs.library(name); err != nil {
			return nil, err
		}
		args = append(args, "-l"+name)
	}
	for _, name := range options.Frameworks {
		if inputs.IOS == nil || name == "" || strings.ContainsAny(name, "/\\.,\x00\r\n") {
			return nil, fmt.Errorf("invalid target framework %q", name)
		}
		file := inputs.IOS.Sysroot + "/System/Library/Frameworks/" + name + ".framework/" + name + ".tbd"
		if _, err := inputs.file(file); err != nil {
			return nil, err
		}
		args = append(args, "-framework", name)
	}
	return args, nil
}

func buildEnvironment(ctx context.Context, options BuildOptions, base *BaseManifest) ([]string, error) {
	compiler, err := buildQuoted([]string{options.Linker})
	if err != nil {
		return nil, err
	}
	// Even go env may probe CC when querying cgo flags. Select the link-only
	// frontend before inspection, so no ambient native compiler is invoked.
	env := buildEnv(os.Environ(), map[string]string{"GOTOOLCHAIN": "local", "CC": compiler, "CXX": compiler})
	command := exec.CommandContext(ctx, options.GoCommand, "env", "-json", "GOVERSION", "GOROOT", "GOFLAGS", "CGO_CFLAGS", "CGO_CPPFLAGS", "CGO_CXXFLAGS", "CGO_FFLAGS", "CGO_LDFLAGS")
	command.Dir, command.Env = options.Directory, env
	data, err := command.Output()
	if err != nil {
		if ctx.Err() != nil {
			return nil, ctx.Err()
		}
		return nil, fmt.Errorf("inspect build toolchain: %w", err)
	}
	values := make(map[string]string)
	if err = json.Unmarshal(data, &values); err != nil {
		return nil, err
	}
	if values["GOVERSION"] != base.GoVersion || values["GOROOT"] == "" {
		return nil, fmt.Errorf("source build requires %s", base.GoVersion)
	}
	for key, want := range map[string]string{"GOFLAGS": "", "CGO_CFLAGS": "-O2 -g", "CGO_CPPFLAGS": "", "CGO_CXXFLAGS": "-O2 -g", "CGO_FFLAGS": "-O2 -g", "CGO_LDFLAGS": "-O2 -g"} {
		if values[key] != want {
			return nil, fmt.Errorf("native base build does not accept custom %s", key)
		}
	}
	env = buildEnv(env, map[string]string{
		"GOROOT": values["GOROOT"], "GOOS": base.GOOS, "GOARCH": base.GOARCH, "GOARM64": base.Architecture, "GOEXPERIMENT": base.Experiment, "CGO_ENABLED": "1", "GOFLAGS": "",
		"CC": compiler, "CXX": compiler, "GOUI_BASE": options.Base, "GOUI_LINK_INPUTS": options.Inputs, "GOUI_LINK_TOOLS": options.Tools, "GOUI_GO": options.GoCommand,
	})
	command = exec.CommandContext(ctx, options.GoDriver, "env", "GOVERSION")
	command.Dir, command.Env = options.Directory, env
	data, err = command.Output()
	if err != nil {
		if ctx.Err() != nil {
			return nil, ctx.Err()
		}
		return nil, fmt.Errorf("inspect Go driver: %w", err)
	}
	if strings.TrimSpace(string(data)) != base.GoVersion {
		return nil, fmt.Errorf("Go driver requires %s", base.GoVersion)
	}
	return env, nil
}

// Go's quoted flags do not perform shell or backslash escaping. See the pinned
// cmd/internal/quoted.Join; compiler argv and link flags use this same format.
func buildQuoted(args []string) (string, error) {
	result := make([]string, len(args))
	for i, arg := range args {
		if strings.ContainsAny(arg, "\x00\r\n") {
			return "", fmt.Errorf("invalid build argument")
		}
		switch {
		case arg != "" && !strings.ContainsAny(arg, " \t'\""):
			result[i] = arg
		case !strings.ContainsRune(arg, '\''):
			result[i] = "'" + arg + "'"
		case !strings.ContainsRune(arg, '"'):
			result[i] = "\"" + arg + "\""
		default:
			return "", fmt.Errorf("Go quoted flag cannot represent both quote characters in %q", arg)
		}
	}
	return strings.Join(result, " "), nil
}
