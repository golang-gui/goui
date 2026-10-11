package main

import (
	"context"
	"debug/elf"
	"debug/macho"
	"debug/pe"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

type goPackage struct {
	Name      string
	Dir       string
	SysoFiles []string
}

func targetEnv(t target) []string {
	var env []string
	for _, item := range os.Environ() {
		key, _, _ := strings.Cut(item, "=")
		if !strings.EqualFold(key, "GOOS") && !strings.EqualFold(key, "GOARCH") {
			env = append(env, item)
		}
	}
	return append(env, "GOOS="+t.OS, "GOARCH="+t.Arch)
}

func mainPackage(ctx context.Context, c *config, t target, tags string) (goPackage, error) {
	args := []string{"list", "-json"}
	if tags != "" {
		args = append(args, "-tags", tags)
	}
	// Resolve a local directory rather than accepting module patterns or .go
	// lists (the latter omit syso inputs). Every execution uses argv, not shell.
	args = append(args, ".")
	command := exec.CommandContext(ctx, "go", args...)
	command.Dir, command.Env = c.path(c.Main), targetEnv(t)
	data, err := command.Output()
	if err != nil {
		if exit, ok := err.(*exec.ExitError); ok {
			return goPackage{}, fmt.Errorf("go list: %s", strings.TrimSpace(string(exit.Stderr)))
		}
		return goPackage{}, fmt.Errorf("go list: %w", err)
	}
	var pkg goPackage
	if err := json.Unmarshal(data, &pkg); err != nil {
		return pkg, fmt.Errorf("go list returned invalid package information: %w", err)
	}
	if pkg.Name != "main" || pkg.Dir == "" {
		return pkg, fmt.Errorf("main must resolve to a single main package")
	}
	return pkg, nil
}

func build(ctx context.Context, c *config, t target, tags, ldflags, output string, out, errOut io.Writer) (string, error) {
	if _, err := generate(ctx, c, t, tags, ""); err != nil {
		return "", err
	}
	pkg, err := mainPackage(ctx, c, t, tags)
	if err != nil {
		return "", err
	}
	if output == "" {
		output = c.binary(t)
	} else {
		output = c.path(output)
	}
	if err := os.MkdirAll(filepath.Dir(output), 0755); err != nil {
		return "", err
	}
	f, err := os.CreateTemp(filepath.Dir(output), ".goui-build-*")
	if err != nil {
		return "", err
	}
	name := f.Name()
	if err := f.Close(); err != nil {
		os.Remove(name)
		return "", err
	}
	defer os.Remove(name)
	args := []string{"build", "-o", name}
	if tags != "" {
		args = append(args, "-tags", tags)
	}
	if t.OS == "windows" && !c.Windows.Console {
		ldflags = strings.TrimSpace(ldflags + " -H=windowsgui")
	}
	if ldflags != "" {
		args = append(args, "-ldflags", ldflags)
	}
	args = append(args, ".")
	command := exec.CommandContext(ctx, "go", args...)
	command.Dir, command.Env = pkg.Dir, targetEnv(t)
	command.Stdout, command.Stderr = out, errOut
	if err := command.Run(); err != nil {
		return "", fmt.Errorf("go build (%s): %w", t, err)
	}
	if err := checkBinary(name, t); err != nil {
		return "", err
	}
	if err := os.Rename(name, output); err != nil {
		return "", err
	}
	return output, nil
}

func checkBinary(path string, t target) error {
	switch t.OS {
	case "windows":
		file, err := pe.Open(path)
		if err != nil {
			return fmt.Errorf("input is not a Windows PE binary: %w", err)
		}
		defer file.Close()
		_, hasHeader := file.OptionalHeader.(*pe.OptionalHeader64)
		if hasHeader && file.Machine == pe.IMAGE_FILE_MACHINE_AMD64 && file.Characteristics&pe.IMAGE_FILE_EXECUTABLE_IMAGE != 0 && file.Characteristics&pe.IMAGE_FILE_DLL == 0 {
			return nil
		}
	case "darwin":
		file, err := macho.Open(path)
		if err != nil {
			return fmt.Errorf("input is not a thin macOS Mach-O binary: %w", err)
		}
		defer file.Close()
		if file.Magic == macho.Magic64 && file.Cpu == macho.CpuArm64 && file.Type == macho.TypeExec {
			return nil
		}
	case "linux":
		file, err := elf.Open(path)
		if err != nil {
			return fmt.Errorf("input is not a Linux ELF binary: %w", err)
		}
		defer file.Close()
		machine := elf.EM_X86_64
		if t.Arch == "arm64" {
			machine = elf.EM_AARCH64
		}
		if file.Class == elf.ELFCLASS64 && file.Data == elf.ELFDATA2LSB && file.Machine == machine && file.Entry != 0 && (file.Type == elf.ET_EXEC || file.Type == elf.ET_DYN) {
			return nil
		}
	}
	return fmt.Errorf("binary %s does not match target %s", path, t)
}
