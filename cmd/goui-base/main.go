// Command goui-base produces, verifies and consumes native support components.
// This maintainer tool is separate from the application build/package interface.
package main

import (
	"context"
	"flag"
	"fmt"
	"io"
	"os"
	"os/signal"

	mobilebase "github.com/golang-gui/goui/platform/mobile/base"
)

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stop()
	if err := run(ctx, os.Args[1:], os.Stdout, os.Stderr); err != nil {
		fmt.Fprintln(os.Stderr, "goui-base:", err)
		os.Exit(1)
	}
}
func run(ctx context.Context, args []string, out, errOut io.Writer) error {
	if len(args) == 0 {
		return fmt.Errorf("usage: goui-base <driver|export|verify|link-inputs|link-tools|build> [options]")
	}
	fs := flag.NewFlagSet(args[0], flag.ContinueOnError)
	fs.SetOutput(errOut)
	switch args[0] {
	case "driver":
		goCommand := fs.String("go", "go", "exact Go toolchain command")
		output := fs.String("o", "", "output Go driver")
		if err := fs.Parse(args[1:]); err != nil {
			return err
		}
		if *output == "" || fs.NArg() != 0 {
			return fmt.Errorf("driver requires -o and no positional arguments")
		}
		return mobilebase.BuildDriver(ctx, *goCommand, *output, errOut)
	case "export":
		var options mobilebase.ExportOptions
		fs.IntVar(&options.Format, "format", 2, "base format: generated sources/native objects (2), legacy archives (1)")
		fs.StringVar(&options.GoCommand, "go", "go", "exact Go toolchain command")
		fs.StringVar(&options.Directory, "dir", ".", "producer module directory")
		fs.StringVar(&options.Output, "o", "", "new component directory")
		fs.StringVar(&options.BuildMode, "buildmode", "", "c-shared or c-archive")
		fs.StringVar(&options.Variant, "variant", "", "native target variant, e.g. device or simulator")
		fs.StringVar(&options.MinimumVersion, "minimum", "", "minimum native platform version")
		if err := fs.Parse(args[1:]); err != nil {
			return err
		}
		if options.Output == "" || fs.NArg() != 1 {
			return fmt.Errorf("export requires -o and one main package")
		}
		options.Package = fs.Arg(0)
		return mobilebase.Export(ctx, options, errOut)
	case "link-inputs", "link-tools":
		spec := fs.String("config", "", "component source specification")
		source := fs.String("dir", ".", "prepared component source directory")
		output := fs.String("o", "", "new component output directory")
		if err := fs.Parse(args[1:]); err != nil {
			return err
		}
		if *spec == "" || *output == "" || fs.NArg() != 0 {
			return fmt.Errorf("%s requires -config, -o and no positional arguments", args[0])
		}
		if args[0] == "link-inputs" {
			return mobilebase.ExportNativeInputs(*spec, *source, *output)
		}
		return mobilebase.ExportNativeTools(ctx, *spec, *source, *output)
	case "build":
		var options mobilebase.BuildOptions
		fs.StringVar(&options.GoCommand, "go", "go", "exact Go toolchain command")
		fs.StringVar(&options.GoDriver, "driver", "", "adapted Go driver")
		fs.StringVar(&options.Linker, "linker", "", "goui-link command")
		fs.StringVar(&options.Base, "base", "", "native Go base manifest")
		fs.StringVar(&options.Inputs, "inputs", "", "target link inputs manifest")
		fs.StringVar(&options.Tools, "tools", "", "host link tools manifest")
		fs.StringVar(&options.Directory, "dir", ".", "main module directory")
		fs.StringVar(&options.Output, "o", "", "native output file")
		fs.BoolVar(&options.Verbose, "x", false, "print Go build commands")
		fs.BoolVar(&options.Entry, "entry", false, "register application main for the mobile host")
		fs.Var((*buildList)(&options.Objects), "object", "native host object/archive (repeatable)")
		fs.Var((*buildList)(&options.Libraries), "library", "target library name (repeatable)")
		fs.Var((*buildList)(&options.Frameworks), "framework", "target framework name (repeatable)")
		if err := fs.Parse(args[1:]); err != nil {
			return err
		}
		if fs.NArg() != 1 {
			return fmt.Errorf("build requires one main package")
		}
		options.Package = fs.Arg(0)
		return mobilebase.Build(ctx, options, out, errOut)
	case "verify":
		if err := fs.Parse(args[1:]); err != nil {
			return err
		}
		if fs.NArg() != 1 {
			return fmt.Errorf("verify requires a base.json path")
		}
		m, err := mobilebase.BaseOpen(fs.Arg(0))
		if err != nil {
			return err
		}
		fmt.Fprintf(out, "OK: %s %s/%s %s %s (%d native packages)\n", m.GoVersion, m.GOOS, m.GOARCH, m.BuildMode, m.Variant, len(m.Packages))
		return nil
	default:
		return fmt.Errorf("unknown command %q", args[0])
	}
}

// buildList accepts repeated values without inventing a shell parsing layer.
type buildList []string

func (v *buildList) String() string         { return fmt.Sprint([]string(*v)) }
func (v *buildList) Set(value string) error { *v = append(*v, value); return nil }
