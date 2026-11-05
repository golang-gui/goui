package base

import (
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// RunNativeArchive accepts the external-archive invocation emitted by the
// pinned Go linker and reuses Go's archive writer. The archive has no native
// symbol index and must be consumed with the final linker's -all_load policy.
func RunNativeArchive(ctx context.Context, goCommand string, args []string, out, errOut io.Writer) error {
	if len(args) < 5 || args[0] != "-q" || args[1] != "-c" || args[2] != "-s" {
		return fmt.Errorf("unsupported external-archive invocation")
	}
	for _, path := range args[4:] {
		if filepath.Ext(path) != ".o" {
			return fmt.Errorf("native archive input must be an object: %s", path)
		}
		info, err := os.Stat(path)
		if err != nil {
			return err
		}
		if !info.Mode().IsRegular() {
			return fmt.Errorf("native archive input is not a regular file: %s", path)
		}
	}
	if goCommand == "" {
		return fmt.Errorf("GOUI_GO is required for the pinned Go archive tool")
	}
	command := exec.CommandContext(ctx, goCommand, "env", "GOVERSION")
	command.Env = baseDriverEnv()
	version, err := command.Output()
	if err != nil {
		return fmt.Errorf("inspect archive toolchain: %w", err)
	}
	if strings.TrimSpace(string(version)) != baseGoVersion {
		return fmt.Errorf("native archive requires %s", baseGoVersion)
	}
	temp, err := os.CreateTemp(filepath.Dir(args[3]), ".goui-archive-*")
	if err != nil {
		return err
	}
	path := temp.Name()
	if err = temp.Close(); err != nil {
		os.Remove(path)
		return err
	}
	defer os.Remove(path)
	arguments := []string{"tool", "pack", "c", path}
	arguments = append(arguments, args[4:]...)
	command = exec.CommandContext(ctx, goCommand, arguments...)
	command.Env = baseDriverEnv()
	command.Stdout = out
	command.Stderr = errOut
	if err = command.Run(); err != nil {
		return fmt.Errorf("create native archive: %w", err)
	}
	return os.Rename(path, args[3])
}
