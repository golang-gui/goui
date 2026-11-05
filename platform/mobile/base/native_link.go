package base

import (
	"bufio"
	"context"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
)

// ErrNativeCompilation indicates a compiler/probe request, which this linker
// cannot satisfy. No C compiler is invoked, including for Go's flag probes.
var ErrNativeCompilation = errors.New("native compilation is unsupported by the link-only driver")

func nativeResponses(args []string, depth int) ([]string, error) {
	if depth > 4 {
		return nil, fmt.Errorf("native response nesting limit exceeded")
	}
	var expanded []string
	for _, arg := range args {
		if !strings.HasPrefix(arg, "@") {
			expanded = append(expanded, arg)
			continue
		}
		file, err := os.Open(strings.TrimPrefix(arg, "@"))
		if err != nil {
			return nil, err
		}
		scanner := bufio.NewScanner(io.LimitReader(file, (8<<20)+1))
		scanner.Buffer(make([]byte, 4096), 8<<20)
		var inner []string
		size := 0
		for scanner.Scan() {
			size += len(scanner.Bytes()) + 1
			if size > 8<<20 {
				file.Close()
				return nil, fmt.Errorf("native response file is too large")
			}
			line := strings.TrimSpace(scanner.Text())
			if line == "" {
				continue
			}
			if strings.HasPrefix(line, "\"") {
				line, err = strconv.Unquote(line)
				if err != nil {
					file.Close()
					return nil, fmt.Errorf("native response argument: %w", err)
				}
			} else if strings.ContainsAny(line, " \t\r\n") {
				file.Close()
				return nil, fmt.Errorf("native response requires one quoted argument per line")
			}
			inner = append(inner, line)
		}
		err = scanner.Err()
		file.Close()
		if err != nil {
			return nil, err
		}
		inner, err = nativeResponses(inner, depth+1)
		if err != nil {
			return nil, err
		}
		expanded = append(expanded, inner...)
		if len(expanded) > 65536 {
			return nil, fmt.Errorf("too many native response arguments")
		}
	}
	return expanded, nil
}

func nativeLinkArguments(m *NativeInputs, args []string) ([]string, string, error) {
	args, err := nativeResponses(args, 0)
	if err != nil {
		return nil, "", err
	}
	var flat []string
	for i := 0; i < len(args); i++ {
		arg := args[i]
		if strings.ContainsAny(arg, "\x00\r\n") {
			return nil, "", fmt.Errorf("invalid control character in native argument")
		}
		if strings.HasPrefix(arg, "-Wl,") {
			flat = append(flat, strings.Split(arg[4:], ",")...)
		} else if arg == "-Xlinker" {
			if i+1 >= len(args) {
				return nil, "", fmt.Errorf("missing -Xlinker argument")
			}
			i++
			flat = append(flat, args[i])
		} else {
			flat = append(flat, arg)
		}
	}
	var prefix, translated []string
	android := m.GOOS == "android"
	if android {
		dir, err := m.directory(m.Android.LibraryDir)
		if err != nil {
			return nil, "", err
		}
		begin, err := m.file(m.Android.CRTBegin)
		if err != nil {
			return nil, "", err
		}
		prefix = []string{"-flavor", "gnu", "-m", "aarch64linux", "-z", "max-page-size=16384", "-z", "common-page-size=16384", "-z", "now", "-z", "relro", "--no-undefined", "--eh-frame-hdr", "--hash-style=gnu", begin, "-L" + dir}
	} else {
		root, err := m.directory(m.IOS.Sysroot)
		if err != nil {
			return nil, "", err
		}
		platform := "ios"
		if m.Variant == "simulator" {
			platform = "ios-simulator"
		}
		prefix = []string{"-flavor", "darwin", "-arch", "arm64", "-platform_version", platform, m.MinimumVersion, m.SDKVersion, "-syslibroot", root, "-Z", "-L" + filepath.Join(root, "usr/lib"), "-F" + filepath.Join(root, "System/Library/Frameworks"), "-export_dynamic", "-all_load"}
	}
	output := ""
	shared := false
	objects := 0
	for i := 0; i < len(flat); i++ {
		arg := flat[i]
		switch {
		case arg == "-o":
			if i+1 >= len(flat) || output != "" || flat[i+1] == "" {
				return nil, "", fmt.Errorf("native link requires exactly one -o path")
			}
			i++
			output = flat[i]
		case arg == "-O2" || arg == "-g" || arg == "-Qunused-arguments" || arg == "-pthread":
			// These compiler-driver defaults do not change precompiled objects. The
			// platform's libc/libSystem already provides pthread entry points.
		case arg == "-c" || arg == "-E" || arg == "-S" && android || arg == "-x" && android:
			return nil, "", ErrNativeCompilation
		case arg == "-shared" && android:
			if shared {
				return nil, "", fmt.Errorf("duplicate native link mode")
			}
			shared = true
			translated = append(translated, arg)
		case arg == "-rdynamic":
			if android {
				translated = append(translated, "--export-dynamic")
			} else {
				translated = append(translated, "-export_dynamic")
			}
		case strings.HasPrefix(arg, "-l"):
			if _, err := m.library(arg[2:]); err != nil {
				return nil, "", err
			}
			translated = append(translated, arg)
		case arg == "-framework" && !android:
			if i+1 >= len(flat) {
				return nil, "", fmt.Errorf("missing framework name")
			}
			i++
			name := flat[i]
			if name == "" || strings.ContainsAny(name, "/\\.,\x00\r\n") {
				return nil, "", fmt.Errorf("invalid framework name %q", name)
			}
			file := m.IOS.Sysroot + "/System/Library/Frameworks/" + name + ".framework/" + name + ".tbd"
			if _, err := m.file(file); err != nil {
				return nil, "", err
			}
			translated = append(translated, arg, name)
		case arg == "-z" && android:
			if i+1 >= len(flat) {
				return nil, "", fmt.Errorf("missing -z value")
			}
			i++
			value := flat[i]
			switch value {
			case "now", "relro", "nodelete", "defs", "max-page-size=16384", "common-page-size=16384":
				translated = append(translated, arg, value)
			default:
				return nil, "", fmt.Errorf("unsupported native -z value %q", value)
			}
		case android && (arg == "-Bsymbolic" || arg == "--export-dynamic" || arg == "--no-undefined" || arg == "--compress-debug-sections=zlib" || arg == "-s"):
			translated = append(translated, arg)
		case android && strings.HasPrefix(arg, "--build-id="):
			value := strings.TrimPrefix(arg, "--build-id=")
			if value != "none" && value != "sha1" && value != "uuid" {
				raw := strings.TrimPrefix(value, "0x")
				if raw == value || raw == "" {
					return nil, "", fmt.Errorf("unsupported native build-id")
				}
				if _, err := hex.DecodeString(raw); err != nil {
					return nil, "", err
				}
			}
			translated = append(translated, arg)
		case !android && (arg == "-all_load" || arg == "-export_dynamic" || arg == "-no_uuid" || arg == "-S" || arg == "-x"):
			translated = append(translated, arg)
		case strings.HasPrefix(arg, "-"):
			return nil, "", fmt.Errorf("unsupported native link option %q", arg)
		default:
			ext := strings.ToLower(filepath.Ext(arg))
			switch ext {
			case ".c", ".cc", ".cpp", ".cxx", ".m", ".mm", ".s":
				return nil, "", ErrNativeCompilation
			case ".o", ".a":
			default:
				return nil, "", fmt.Errorf("native object/archive expected: %s", arg)
			}
			info, err := os.Stat(arg)
			if err != nil {
				return nil, "", err
			}
			if !info.Mode().IsRegular() {
				return nil, "", fmt.Errorf("native object is not a regular file: %s", arg)
			}
			translated = append(translated, arg)
			objects++
		}
	}
	if output == "" || objects == 0 || android && !shared {
		return nil, "", fmt.Errorf("native link needs -o, object inputs, and Android -shared mode")
	}
	if strings.ContainsAny(output, "\x00\r\n") {
		return nil, "", fmt.Errorf("invalid native output path")
	}
	if android {
		builtins, err := m.file(m.Android.Builtins)
		if err != nil {
			return nil, "", err
		}
		end, err := m.file(m.Android.CRTEnd)
		if err != nil {
			return nil, "", err
		}
		translated = append(translated, builtins, "-lc", "-ldl", "-lm", end)
	} else {
		translated = append(translated, "-lSystem")
	}
	return append(prefix, translated...), output, nil
}

// RunNativeLink translates Go's external-link arguments and invokes verified
// LLD with target inputs. Output is replaced only after format checks succeed.
func RunNativeLink(ctx context.Context, inputPath, toolPath string, args []string, out, errOut io.Writer) error {
	inputs, err := OpenNativeInputs(inputPath)
	if err != nil {
		return err
	}
	translated, output, err := nativeLinkArguments(inputs, args)
	if err != nil {
		return err
	}
	tools, err := OpenNativeTools(toolPath)
	if err != nil {
		return err
	}
	output, err = filepath.Abs(output)
	if err != nil {
		return err
	}
	tempDir, err := os.MkdirTemp(filepath.Dir(output), ".goui-link-*")
	if err != nil {
		return err
	}
	defer os.RemoveAll(tempDir)
	// Mach-O LLD derives its ad-hoc signature identifier from the output basename.
	// Preserve that name while staging on the destination's filesystem.
	temporary := filepath.Join(tempDir, filepath.Base(output))
	translated = append(translated, "-o", temporary)
	// GNU-flavor LLD explicitly accepts POSIX quoting on every host. Mach-O LLD
	// uses its GNU response tokenizer, independent of the build host.
	if inputs.GOOS == "android" {
		translated = append(translated, "--rsp-quoting=posix")
	}
	response, err := os.CreateTemp("", "goui-lld-*.rsp")
	if err != nil {
		return err
	}
	defer os.Remove(response.Name())
	// Always use a response file between this driver and LLD so Windows process
	// command-line limits do not constrain the number of translated arguments.
	for _, arg := range translated[2:] {
		if _, err = fmt.Fprintln(response, nativeLLDQuote(arg)); err != nil {
			response.Close()
			return err
		}
	}
	if err = response.Close(); err != nil {
		return err
	}
	command := exec.CommandContext(ctx, filepath.Join(tools.root, filepath.FromSlash(tools.LLD.File)), translated[0], translated[1], "@"+response.Name())
	command.Stdout = out
	command.Stderr = errOut
	if err = command.Run(); err != nil {
		return fmt.Errorf("native link %s/%s: %w", inputs.GOOS, inputs.GOARCH, err)
	}
	if err = CheckNativeBinary(temporary, inputs); err != nil {
		return fmt.Errorf("verify native link output: %w", err)
	}
	return os.Rename(temporary, output)
}

// LLVM uses GNU response quoting, not Go string escape sequences. Preserve
// literal tabs/Unicode while escaping only backslashes and double quotes.
func nativeLLDQuote(arg string) string {
	return `"` + strings.NewReplacer(`\`, `\\`, `"`, `\"`).Replace(arg) + `"`
}
