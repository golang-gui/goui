package main

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestCommands(t *testing.T) {
	c := testConfig(t)
	path := filepath.Join(c.dir, "goui.toml")
	ctx := context.Background()
	for _, args := range [][]string{nil, {"help"}, {"build", "-h"}} {
		var output bytes.Buffer
		if err := run(ctx, args, &output, &output); err != nil || output.Len() == 0 {
			t.Fatalf("help: %v, %s", err, &output)
		}
	}
	for _, args := range [][]string{
		{"unknown"}, {"build", "-unknown"}, {"build", "-target", "windows/386"},
		{"generate", "-config", path, "extra"}, {"package", "-config", path, "-target", "windows/amd64", "-format", "exe"},
	} {
		var output bytes.Buffer
		if err := run(ctx, args, &output, &output); err == nil {
			t.Fatal("invalid command succeeded", args)
		}
	}
	var output bytes.Buffer
	if err := run(ctx, []string{"doctor", "-config", path, "-target", "linux/amd64"}, &output, &output); err != nil || !strings.Contains(output.String(), "OK:") {
		t.Fatalf("doctor: %v %s", err, &output)
	}
	if _, err := os.Stat(filepath.Join(c.dir, "dist")); !os.IsNotExist(err) {
		t.Fatal("doctor wrote output files")
	}
	for i := 0; i < 2; i++ {
		if err := run(ctx, []string{"generate", "-config", path, "-target", "linux/amd64", "-o", "generated"}, &output, &output); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := os.Stat(filepath.Join(c.dir, "generated", "usr", "share", "applications", c.ID+".desktop")); err != nil {
		t.Fatal(err)
	}
	writeTestFile(t, filepath.Join(c.dir, "main.go"), []byte("package library\n"))
	if _, err := mainPackage(ctx, c, target{"linux", "amd64"}, ""); err == nil {
		t.Fatal("accepted a non-main package")
	}
}

func TestConfigOutsideModule(t *testing.T) {
	c := testConfig(t)
	moduleDir := c.dir
	configDir := t.TempDir()
	c.dir = configDir
	var err error
	c.Main, err = filepath.Rel(configDir, moduleDir)
	if err != nil {
		t.Fatal(err)
	}
	pkg, err := mainPackage(context.Background(), c, target{"linux", "amd64"}, "")
	if err != nil {
		t.Fatalf("config-relative main package did not use its own module: %+v %v", pkg, err)
	}
	got, err := os.Stat(pkg.Dir)
	if err != nil {
		t.Fatal(err)
	}
	want, err := os.Stat(moduleDir)
	if err != nil {
		t.Fatal(err)
	}
	// Go may canonicalize macOS's /var alias to /private/var. Compare the
	// actual directory, not its spelling, without accepting a different module.
	if !os.SameFile(got, want) {
		t.Fatalf("resolved module directory %q, want %q", pkg.Dir, moduleDir)
	}
}

func TestDesktopEntry(t *testing.T) {
	c := testConfig(t)
	want := "[Desktop Entry]\nType=Application\nName=打包 & 图标验证\nExec=/usr/bin/packaging-test\n" +
		"Icon=org.golang-gui.PackagingTest\nStartupWMClass=org.golang-gui.PackagingTest\nTerminal=false\nCategories=Utility;\n"
	if got := desktopEntry(c); got != want {
		t.Fatalf("desktop entry: %s", got)
	}
	if got := desktopString("a\\b\nc\td\r"); got != `a\\b\nc\td\r` {
		t.Fatal("incorrect Desktop Entry escaping", got)
	}
	verifyPlist(t, plist(c), c)
}
