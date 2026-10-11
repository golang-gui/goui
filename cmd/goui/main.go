// Command goui builds binaries and prepares platform application resources.
// See doc/DesignBuildPackaging.md for configuration and deployment boundaries.
package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"os/signal"
	"runtime"
)

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stop()
	if err := run(ctx, os.Args[1:], os.Stdout, os.Stderr); err != nil {
		fmt.Fprintln(os.Stderr, "goui:", err)
		os.Exit(1)
	}
}

func run(ctx context.Context, args []string, out, errOut io.Writer) error {
	if len(args) == 0 || args[0] == "help" || args[0] == "-h" || args[0] == "--help" {
		fmt.Fprintln(out, "usage: goui <doctor|generate|build|package> [-config goui.toml] [-target OS/ARCH]")
		fmt.Fprintln(out, "build outputs a binary; package consumes it (macOS: -format app, Linux: -format dir).")
		return nil
	}
	command := args[0]
	if command != "doctor" && command != "generate" && command != "build" && command != "package" {
		return fmt.Errorf("unknown command %q", command)
	}
	fs := flag.NewFlagSet(command, flag.ContinueOnError)
	fs.SetOutput(errOut)
	configPath := fs.String("config", "goui.toml", "configuration file (all paths relative to this file)")
	targetName := fs.String("target", runtime.GOOS+"/"+runtime.GOARCH, "target OS/ARCH")
	var output, tags, ldflags, binaryPath, format string
	if command != "doctor" {
		fs.StringVar(&output, "o", "", "output path, relative to the configuration directory")
	}
	if command == "build" || command == "generate" || command == "doctor" {
		fs.StringVar(&tags, "tags", "", "Go build tags")
	}
	if command == "build" {
		fs.StringVar(&ldflags, "ldflags", "", "Go linker flags")
	}
	if command == "package" {
		fs.StringVar(&binaryPath, "binary", "", "existing binary (default: build output)")
		fs.StringVar(&format, "format", "", "app for macOS, dir for Linux; defaults to target format")
	}
	if err := fs.Parse(args[1:]); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return nil
		}
		return err
	}
	if fs.NArg() != 0 {
		return fmt.Errorf("unexpected arguments: %v", fs.Args())
	}
	target, err := parseTarget(*targetName)
	if err != nil {
		return err
	}
	c, err := loadConfig(*configPath)
	if err != nil {
		return err
	}
	switch command {
	case "doctor":
		var icon *appIcon
		icon, err = loadIcon(c)
		if err == nil && (target.OS == "windows" && c.Windows.Icon != "" || target.OS == "darwin" && c.MacOS.Icon != "") {
			_, err = nativeIcon(c, icon, target.OS)
		}
		if err == nil {
			_, err = mainPackage(ctx, c, target, tags)
		}
		if err == nil {
			fmt.Fprintf(out, "OK: configuration, icon and Go main package (%s). Target desktop/runtime not checked.\n", target)
		}
	case "generate":
		var files []string
		files, err = generate(ctx, c, target, tags, output)
		if err == nil {
			for _, file := range files {
				fmt.Fprintln(out, file)
			}
		}
	case "build":
		var file string
		file, err = build(ctx, c, target, tags, ldflags, output, out, errOut)
		if err == nil {
			fmt.Fprintln(out, file)
		}
	case "package":
		var file string
		file, err = packageApp(c, target, format, binaryPath, output)
		if err == nil {
			fmt.Fprintln(out, file)
		}
	}
	return err
}
