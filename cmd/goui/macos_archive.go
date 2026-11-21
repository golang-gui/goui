package main

import (
	"archive/zip"
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
)

// archiveMacOS preserves the supported bundle as a ZIP with explicit Unix modes,
// including when the source directory lives on a Windows filesystem.
func archiveMacOS(ctx context.Context, app, output string) (err error) {
	appInfo, err := os.Stat(app)
	if err != nil {
		return err
	}
	parent, err := filepath.Abs(filepath.Dir(output))
	if err != nil {
		return err
	}
	// Resolve the nearest existing parent before creating anything. An alias to
	// the input bundle must not let the ZIP consume its own growing output.
	for {
		resolved, resolveErr := filepath.EvalSymlinks(parent)
		if resolveErr == nil {
			parent = resolved
			break
		}
		if !os.IsNotExist(resolveErr) || filepath.Dir(parent) == parent {
			return resolveErr
		}
		parent = filepath.Dir(parent)
	}
	for {
		info, err := os.Stat(parent)
		if err != nil {
			return err
		}
		// File identity also handles case-insensitive filesystems and volumes.
		if os.SameFile(appInfo, info) {
			return fmt.Errorf("archive output must be outside the input app")
		}
		if filepath.Dir(parent) == parent {
			break
		}
		parent = filepath.Dir(parent)
	}

	executable, err := macOSBundleExecutable(app)
	if err != nil {
		return err
	}
	if _, err = macOSResources(app, executable); err != nil {
		return err
	}
	if err = checkBinary(filepath.Join(app, "Contents", "MacOS", executable), target{"darwin", "arm64"}); err != nil {
		return err
	}
	if filepath.Ext(output) != ".zip" {
		return fmt.Errorf("macOS archive output must end in .zip")
	}
	if _, e := os.Lstat(output); !os.IsNotExist(e) {
		if e != nil {
			return e
		}
		return fmt.Errorf("archive output already exists: %s", output)
	}
	if err = os.MkdirAll(filepath.Dir(output), 0755); err != nil {
		return err
	}
	file, err := os.CreateTemp(filepath.Dir(output), ".goui-archive-*")
	if err != nil {
		return err
	}
	defer os.Remove(file.Name())
	defer file.Close()
	z := zip.NewWriter(file)
	err = filepath.WalkDir(app, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if err = ctx.Err(); err != nil {
			return err
		}
		if d.Type()&os.ModeSymlink != 0 {
			return fmt.Errorf("unsupported bundle symbolic link: %s", path)
		}
		rel, err := filepath.Rel(app, path)
		if err != nil {
			return err
		}
		rel = filepath.ToSlash(rel)
		name := filepath.Base(app)
		if rel != "." {
			name += "/" + rel
		}
		info, err := d.Info()
		if err != nil {
			return err
		}
		mode := os.FileMode(0644)
		if rel == "Contents/MacOS/"+executable {
			mode = 0755
		}
		header := &zip.FileHeader{Name: name, Method: zip.Deflate}
		header.SetModTime(info.ModTime())
		if d.IsDir() {
			header.Name += "/"
			mode = os.ModeDir | 0755
			header.Method = zip.Store
		} else if !info.Mode().IsRegular() {
			return fmt.Errorf("unsupported bundle file: %s", rel)
		}
		header.SetMode(mode)
		writer, err := z.CreateHeader(header)
		if err != nil {
			return err
		}
		if d.IsDir() {
			return nil
		}
		source, err := os.Open(path)
		if err != nil {
			return err
		}
		_, err = io.Copy(writer, source)
		closeErr := source.Close()
		if err != nil {
			return err
		}
		return closeErr
	})
	closeErr := z.Close()
	if err != nil {
		return err
	}
	if closeErr != nil {
		return closeErr
	}
	if err = ctx.Err(); err != nil {
		return err
	}
	if err = file.Chmod(0644); err != nil {
		return err
	}
	if err = file.Close(); err != nil {
		return err
	}
	// Same-directory hard linking publishes only if output is still absent.
	if err = os.Link(file.Name(), output); err != nil {
		return fmt.Errorf("publish new archive: %w", err)
	}
	return nil
}
