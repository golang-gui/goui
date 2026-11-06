package base

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
)

// ExportOptions describe a maintainer build. The environment selects the target
// and real C compiler. Production is intentionally separate from consumption.
type ExportOptions struct {
	// Format selects package archives (1) or generated sources/native objects
	// (2). Zero retains the original package archive API for existing callers.
	Format         int
	GoCommand      string
	Directory      string
	Output         string
	BuildMode      string
	Variant        string
	MinimumVersion string
	Package        string
}

type baseListedPackage struct {
	Module                                                                                                                       *struct{ Path, Version, GoVersion string }
	Dir, ImportPath, Name, Export, BuildID                                                                                       string
	GoFiles, CgoFiles, CFiles, CXXFiles, MFiles, HFiles, FFiles, SFiles, SysoFiles, SwigFiles, SwigCXXFiles, EmbedFiles, Imports []string
}

// Export builds and copies only packages that actually require cgo or SWIG.
// Output must not exist; failed production leaves no partially valid component.
func Export(ctx context.Context, options ExportOptions, log io.Writer) (err error) {
	if options.Format == 0 {
		options.Format = 1
	}
	if options.Format != 1 && options.Format != 2 {
		return fmt.Errorf("unsupported base export format %d", options.Format)
	}
	if options.GoCommand == "" {
		options.GoCommand = "go"
	}
	if options.Package == "" {
		options.Package = "."
	}
	if options.Variant == "" || options.MinimumVersion == "" {
		return fmt.Errorf("variant and minimum version are required provenance")
	}
	command := exec.CommandContext(ctx, options.GoCommand, "env", "-json", "GOVERSION", "GOOS", "GOARCH", "GOARM64", "GOAMD64", "GOEXPERIMENT", "CGO_ENABLED", "GOTOOLDIR", "CC", "CXX", "CGO_CFLAGS", "CGO_CPPFLAGS", "CGO_CXXFLAGS", "CGO_FFLAGS", "CGO_LDFLAGS")
	command.Dir = options.Directory
	data, err := command.Output()
	if err != nil {
		return fmt.Errorf("inspect producer toolchain: %w", err)
	}
	var env map[string]string
	if err = json.Unmarshal(data, &env); err != nil {
		return err
	}
	if !supportedBaseVersion(env["GOVERSION"]) || env["CGO_ENABLED"] != "1" || options.Format == 1 && env["GOVERSION"] != baseGoVersion {
		return fmt.Errorf("base production requires a supported Go version and CGO_ENABLED=1 (legacy archives: %s)", baseGoVersion)
	}
	if os.Getenv("GOUI_BASE") != "" || os.Getenv("GOFLAGS") != "" {
		return fmt.Errorf("base production requires empty GOUI_BASE and GOFLAGS")
	}
	output, err := filepath.Abs(options.Output)
	if err != nil {
		return err
	}
	if _, err = os.Stat(output); err == nil {
		return fmt.Errorf("output already exists: %s", output)
	} else if !os.IsNotExist(err) {
		return err
	}
	if err = os.MkdirAll(filepath.Dir(output), 0755); err != nil {
		return err
	}
	temp, err := os.MkdirTemp(filepath.Dir(output), ".goui-base-*")
	if err != nil {
		return err
	}
	defer os.RemoveAll(temp)
	driver, records := options.GoCommand, ""
	if options.Format == 2 {
		records = filepath.Join(temp, "records")
		if err = os.Mkdir(records, 0700); err != nil {
			return err
		}
		driver = filepath.Join(temp, "go-record")
		if runtime.GOOS == "windows" {
			driver += ".exe"
		}
		if err = BuildDriver(ctx, options.GoCommand, driver, log); err != nil {
			return err
		}
	}
	if err = os.Mkdir(filepath.Join(temp, "packages"), 0755); err != nil {
		return err
	}
	command = exec.CommandContext(ctx, driver, "list", "-deps", "-export", "-compiled", "-json", "-trimpath", "-pgo=off", "-buildmode="+options.BuildMode, options.Package)
	if records != "" {
		command.Env = buildEnv(os.Environ(), map[string]string{"GOUI_RECORD_BASE": records})
	}
	command.Dir = options.Directory
	command.Stderr = log
	stream, err := command.StdoutPipe()
	if err != nil {
		return err
	}
	if err = command.Start(); err != nil {
		return err
	}
	packages := make(map[string]baseListedPackage)
	decoder := json.NewDecoder(stream)
	for {
		var pkg baseListedPackage
		if err = decoder.Decode(&pkg); err == io.EOF {
			err = nil
			break
		}
		if err != nil {
			break
		}
		packages[pkg.ImportPath] = pkg
	}
	if err != nil {
		command.Process.Kill()
		command.Wait()
		return fmt.Errorf("read producer package list: %w", err)
	}
	if err = command.Wait(); err != nil {
		return fmt.Errorf("produce native support: %w", err)
	}
	m := BaseManifest{Format: options.Format, GoVersion: env["GOVERSION"], GOOS: env["GOOS"], GOARCH: env["GOARCH"], Architecture: env["GOARM64"], Experiment: env["GOEXPERIMENT"], BuildMode: options.BuildMode, Variant: options.Variant, MinimumVersion: options.MinimumVersion, Packages: make(map[string]BasePackage)}
	if options.Format == 2 {
		m.CgoFlags, m.ProducerTools = map[string]string{}, map[string]string{}
		for _, key := range []string{"CGO_CFLAGS", "CGO_CPPFLAGS", "CGO_CXXFLAGS", "CGO_FFLAGS", "CGO_LDFLAGS"} {
			m.CgoFlags[key] = env[key]
		}
		name := "cgo"
		if runtime.GOOS == "windows" {
			name += ".exe"
		}
		m.ProducerTools["cgo"], err = baseHash(filepath.Join(env["GOTOOLDIR"], name))
		if err != nil {
			return err
		}
		for _, key := range []string{"CC", "CXX"} {
			words, err := splitBuildWords(env[key])
			if err != nil || len(words) == 0 {
				return fmt.Errorf("invalid producer %s", key)
			}
			path, err := exec.LookPath(words[0])
			if err != nil {
				return err
			}
			m.ProducerTools[key], err = baseHash(path)
			if err != nil {
				return err
			}
		}
	}
	if m.GOARCH == "amd64" {
		m.Architecture = env["GOAMD64"]
	}
	for name, pkg := range packages {
		if len(pkg.CgoFiles)+len(pkg.SwigFiles)+len(pkg.SwigCXXFiles) == 0 {
			continue
		}
		if pkg.Name == "main" {
			return fmt.Errorf("cannot precompile application main; put fixed native entry in a support package")
		}
		if options.Format == 2 {
			data, err := os.ReadFile(filepath.Join(records, recipeDigest([]byte(name))+".json"))
			if err != nil {
				return fmt.Errorf("read cgo producer record %s: %w", name, err)
			}
			var record cgoRecord
			if err = json.Unmarshal(data, &record); err != nil {
				return err
			}
			if record.Path != name {
				return fmt.Errorf("cgo producer record identity mismatch: %s", name)
			}
			for _, files := range [][]BaseFile{record.Package.GeneratedGo, record.Package.Objects, record.Package.Auxiliary} {
				for _, f := range files {
					path := filepath.Join(temp, f.Path)
					if err = os.MkdirAll(filepath.Dir(path), 0755); err != nil {
						return err
					}
					if _, err = os.Stat(path); os.IsNotExist(err) {
						if err = baseCopy(filepath.Join(records, f.Path), path); err != nil {
							return err
						}
					} else if err != nil {
						return err
					}
				}
			}
			m.Packages[name] = record.Package
			continue
		}
		record := BasePackage{BuildID: pkg.BuildID, Imports: make(map[string]string)}
		if pkg.Module != nil {
			record.ModulePath = pkg.Module.Path
			record.ModuleVersion = pkg.Module.Version
			record.ModuleGoVersion = pkg.Module.GoVersion
		}
		record.SourceSHA256, err = BaseSourceHash(pkg.Dir, pkg.GoFiles, pkg.CgoFiles, pkg.CFiles, pkg.CXXFiles, pkg.MFiles, pkg.HFiles, pkg.FFiles, pkg.SFiles, pkg.SysoFiles, pkg.SwigFiles, pkg.SwigCXXFiles, pkg.EmbedFiles)
		if err != nil {
			return err
		}
		for _, depName := range pkg.Imports {
			dep := packages[depName]
			if dep.Export == "" {
				continue
			}
			record.Imports[depName], err = BaseExportHash(dep.Export)
			if err != nil {
				return fmt.Errorf("export %s dependency %s: %w", name, depName, err)
			}
		}
		// The content hash is a portable filename, independent of package paths.
		record.SHA256, err = baseHash(pkg.Export)
		if err != nil {
			return err
		}
		record.Archive = "packages/" + record.SHA256 + ".a"
		if err = baseCopy(pkg.Export, filepath.Join(temp, filepath.FromSlash(record.Archive))); err != nil {
			return err
		}
		m.Packages[name] = record
	}
	data, err = json.MarshalIndent(m, "", "  ")
	if err != nil {
		return err
	}
	path := filepath.Join(temp, "base.json")
	if err = os.WriteFile(path, append(data, '\n'), 0644); err != nil {
		return err
	}
	if _, err = BaseOpen(path); err != nil {
		return err
	}
	if records != "" {
		if err = os.RemoveAll(records); err != nil {
			return err
		}
		if err = os.Remove(driver); err != nil {
			return err
		}
		if err = os.Remove(filepath.Join(temp, "packages")); err != nil {
			return err
		}
	}
	return os.Rename(temp, output)
}

func baseCopy(source, destination string) error {
	input, err := os.Open(source)
	if err != nil {
		return err
	}
	defer input.Close()
	output, err := os.OpenFile(destination, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0644)
	if err != nil {
		return err
	}
	_, err = io.Copy(output, input)
	closeErr := output.Close()
	if err != nil {
		return err
	}
	return closeErr
}
