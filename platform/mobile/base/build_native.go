package base

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
)

// splitBuildWords follows cmd/internal/quoted's non-shell argument grammar.
func splitBuildWords(text string) ([]string, error) {
	var words []string
	for text = strings.TrimSpace(text); text != ""; text = strings.TrimSpace(text) {
		if text[0] == '\'' || text[0] == '"' {
			i := strings.IndexByte(text[1:], text[0])
			if i < 0 {
				return nil, fmt.Errorf("unterminated compiler argument")
			}
			words = append(words, text[1:i+1])
			text = text[i+2:]
			if text != "" && text[0] != ' ' && text[0] != '\t' {
				return nil, fmt.Errorf("invalid compiler argument boundary")
			}
		} else {
			i := strings.IndexAny(text, " \t")
			if i < 0 {
				i = len(text)
			}
			words = append(words, text[:i])
			text = text[i:]
		}
	}
	return words, nil
}

func nativeBuildEnvironment(ctx context.Context, options BuildOptions, inputs *NativeInputs) ([]string, error) {
	if os.Getenv("CGO_ENABLED") == "0" {
		return nil, fmt.Errorf("mobile native builds require CGO_ENABLED=1")
	}
	env := buildEnv(os.Environ(), map[string]string{"GOOS": inputs.GOOS, "GOARCH": inputs.GOARCH, "CGO_ENABLED": "1", "GOUI_BASE": "", "GOUI_RECORD_BASE": "", "GOUI_LINK_COMMAND": "", "GOUI_LINK_INPUTS": "", "GOUI_LINK_TOOLS": ""})
	if inputs.GOOS != "ios" {
		return env, nil
	}
	sdk := os.Getenv("SDKROOT")
	if sdk == "" {
		if runtime.GOOS != "darwin" {
			return nil, fmt.Errorf("native iOS builds require SDKROOT and a target Apple toolchain")
		}
		name := "iphoneos"
		if inputs.Variant == "simulator" {
			name = "iphonesimulator"
		}
		command := exec.CommandContext(ctx, "xcrun", "--sdk", name, "--show-sdk-path")
		data, err := command.Output()
		if err != nil {
			return nil, fmt.Errorf("locate iOS SDK: %w", err)
		}
		sdk = strings.TrimSpace(string(data))
	}
	if s, err := os.Stat(sdk); err != nil || !s.IsDir() {
		return nil, fmt.Errorf("iOS SDKROOT is not a directory: %s", sdk)
	}
	command := exec.CommandContext(ctx, options.GoCommand, "env", "-json", "CC", "CXX")
	command.Dir, command.Env = options.Directory, env
	data, err := command.Output()
	if err != nil {
		return nil, err
	}
	var compilers map[string]string
	if err = json.Unmarshal(data, &compilers); err != nil {
		return nil, err
	}
	target := "arm64-apple-ios" + inputs.MinimumVersion
	if inputs.Variant == "simulator" {
		target += "-simulator"
	}
	for _, key := range []string{"CC", "CXX"} {
		args, err := splitBuildWords(compilers[key])
		if err != nil || len(args) == 0 {
			return nil, fmt.Errorf("invalid %s compiler", key)
		}
		args = append(args, "-target", target, "-isysroot", sdk)
		compilers[key], err = buildQuoted(args)
		if err != nil {
			return nil, err
		}
	}
	compilers["SDKROOT"] = sdk
	return buildEnv(env, compilers), nil
}

func nativeIOSLinkArguments(ctx context.Context, options BuildOptions, env []string) ([]string, string, error) {
	args := []string{"list", "-deps", "-json"}
	if options.Tags != "" {
		args = append(args, "-tags="+options.Tags)
	}
	command := exec.CommandContext(ctx, options.GoCommand, append(args, options.Package)...)
	command.Dir, command.Env = options.Directory, env
	var stderr bytes.Buffer
	command.Stderr = &stderr
	data, err := command.Output()
	if err != nil {
		return nil, "", fmt.Errorf("inspect native link dependencies: %w\n%s", err, &stderr)
	}
	var flags, pkgConfig []string
	compiler := "CC"
	d := json.NewDecoder(bytes.NewReader(data))
	for {
		var p struct{ CgoLDFLAGS, CgoPkgConfig, CXXFiles []string }
		if err = d.Decode(&p); err == io.EOF {
			break
		} else if err != nil {
			return nil, "", err
		}
		flags = append(flags, p.CgoLDFLAGS...)
		pkgConfig = append(pkgConfig, p.CgoPkgConfig...)
		if len(p.CXXFiles) > 0 {
			compiler = "CXX"
		}
	}
	if len(pkgConfig) > 0 {
		name := os.Getenv("PKG_CONFIG")
		if name == "" {
			name = "pkg-config"
		}
		words, err := splitBuildWords(name)
		if err != nil || len(words) == 0 {
			return nil, "", fmt.Errorf("invalid PKG_CONFIG command")
		}
		command = exec.CommandContext(ctx, words[0], append(append(words[1:], "--libs"), pkgConfig...)...)
		command.Dir, command.Env = options.Directory, env
		data, err = command.Output()
		if err != nil {
			return nil, "", fmt.Errorf("native pkg-config: %w", err)
		}
		words, err = splitBuildWords(string(data))
		if err != nil {
			return nil, "", err
		}
		flags = append(flags, words...)
	}
	command = exec.CommandContext(ctx, options.GoCommand, "env", "CGO_LDFLAGS")
	command.Dir, command.Env = options.Directory, env
	data, err = command.Output()
	if err != nil {
		return nil, "", err
	}
	global, err := splitBuildWords(strings.TrimSpace(string(data)))
	if err != nil {
		return nil, "", err
	}
	return append(flags, global...), compiler, nil
}

func buildNative(ctx context.Context, options BuildOptions, out, errOut io.Writer) error {
	if options.GoCommand == "" {
		options.GoCommand = "go"
	}
	if options.Package == "" {
		options.Package = "."
	}
	if strings.HasPrefix(options.Package, "-") {
		return fmt.Errorf("invalid main package %q", options.Package)
	}
	if options.Output == "" || options.Inputs == "" {
		return fmt.Errorf("native build requires output and target inputs")
	}
	inputs, err := OpenNativeInputs(options.Inputs)
	if err != nil {
		return err
	}
	env, err := nativeBuildEnvironment(ctx, options, inputs)
	if err != nil {
		return err
	}
	output, err := filepath.Abs(options.Output)
	if err != nil {
		return err
	}
	if err = os.MkdirAll(filepath.Dir(output), 0755); err != nil {
		return err
	}
	stage, err := os.MkdirTemp(filepath.Dir(output), ".goui-native-*")
	if err != nil {
		return err
	}
	defer os.RemoveAll(stage)
	nativeOutput := filepath.Join(stage, filepath.Base(output))
	goOutput := nativeOutput
	flags := append([]string{"-linkmode=external"}, options.LDFlags...)
	mode := "c-shared"
	if inputs.GOOS == "ios" {
		mode = "c-archive"
		goOutput = filepath.Join(stage, "go.a")
	} else {
		flags = append(flags, "-extldflags=-Wl,-z,max-page-size=16384,-z,common-page-size=16384,-z,now,-z,relro")
	}
	encoded, err := buildQuoted(flags)
	if err != nil {
		return err
	}
	args := []string{"build", "-trimpath", "-pgo=off", "-buildmode=" + mode, "-ldflags=" + encoded, "-o", goOutput}
	if options.Tags != "" {
		args = append(args, "-tags="+options.Tags)
	}
	if options.Entry {
		options.GoDriver = options.GoCommand
		overlay, err := createEntryOverlay(ctx, options, mode, env, stage)
		if err != nil {
			return err
		}
		args = append(args, "-overlay="+overlay)
	}
	if options.Verbose {
		args = append(args, "-x")
	}
	command := exec.CommandContext(ctx, options.GoCommand, append(args, options.Package)...)
	command.Dir, command.Env, command.Stdout, command.Stderr = options.Directory, env, out, errOut
	if err = command.Run(); err != nil {
		return fmt.Errorf("native Go build: %w", err)
	}
	if inputs.GOOS == "ios" {
		linkFlags, compiler, err := nativeIOSLinkArguments(ctx, options, env)
		if err != nil {
			return err
		}
		command = exec.CommandContext(ctx, options.GoCommand, "env", compiler)
		command.Dir, command.Env = options.Directory, env
		data, err := command.Output()
		if err != nil {
			return err
		}
		words, err := splitBuildWords(strings.TrimSpace(string(data)))
		if err != nil || len(words) == 0 {
			return fmt.Errorf("invalid native linker compiler")
		}
		args = append(words[1:], "-o", nativeOutput, goOutput)
		args = append(args, options.Objects...)
		for _, name := range options.Libraries {
			args = append(args, "-l"+name)
		}
		for _, name := range options.Frameworks {
			args = append(args, "-framework", name)
		}
		args = append(args, linkFlags...)
		command = exec.CommandContext(ctx, words[0], args...)
		command.Dir, command.Env, command.Stdout, command.Stderr = options.Directory, env, out, errOut
		if err = command.Run(); err != nil {
			return fmt.Errorf("native iOS link: %w", err)
		}
	}
	if err = CheckNativeTarget(nativeOutput, inputs); err != nil {
		return err
	}
	if err = ctx.Err(); err != nil {
		return err
	}
	return os.Rename(nativeOutput, output)
}
