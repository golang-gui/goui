// Command goui-base produces and verifies native support components.
// This maintainer tool is separate from the application build/package interface.
package main

import (
	"context"
	"flag"
	"fmt"
	"io"
	"os"
	"os/signal"

	"github.com/golang-gui/goui/internal/mobilebase"
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
		return fmt.Errorf("usage: goui-base <driver|export|verify> [options]")
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
