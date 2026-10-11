package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

const validConfig = `# comments are allowed
id = "org.golang-gui.PackagingTest"
name = "打包 & 图标验证"
executable = "packaging-test"
version = "0.1.0"
icon = "icon.svg"
[windows]
console = false
[linux]
categories = ["Utility"]
`

func writeTestFile(t *testing.T, path string, data []byte) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, data, 0644); err != nil {
		t.Fatal(err)
	}
}

func testConfig(t *testing.T) *config {
	t.Helper()
	dir := t.TempDir()
	data, err := os.ReadFile("testdata/icon.svg")
	if err != nil {
		t.Fatal(err)
	}
	writeTestFile(t, filepath.Join(dir, "icon.svg"), data)
	writeTestFile(t, filepath.Join(dir, "goui.toml"), []byte(validConfig))
	writeTestFile(t, filepath.Join(dir, "go.mod"), []byte("module example.com/packaging-test\n\ngo 1.24\n"))
	writeTestFile(t, filepath.Join(dir, "main.go"), []byte("package main\nfunc main() {}\n"))
	c, err := loadConfig(filepath.Join(dir, "goui.toml"))
	if err != nil {
		t.Fatal(err)
	}
	return c
}

func TestConfig(t *testing.T) {
	c := testConfig(t)
	if c.Main != "." || c.Build != 1 || c.Windows.Console || c.Name != "打包 & 图标验证" {
		t.Fatalf("wrong values/defaults: %+v", c)
	}
	if got := c.path("icon.svg"); got != filepath.Join(c.dir, "icon.svg") {
		t.Fatal(got)
	}
	for _, value := range []string{"windows/amd64", "darwin/arm64", "linux/amd64", "linux/arm64"} {
		target, err := parseTarget(value)
		if err != nil || target.String() != value {
			t.Fatalf("%s: %v", value, err)
		}
	}
	for _, value := range []string{"windows/386", "darwin/amd64", "linux", "linux/arm", "linux/amd64/extra"} {
		if _, err := parseTarget(value); err == nil {
			t.Fatal("accepted target", value)
		}
	}
}

func TestConfigRejectsInvalidValues(t *testing.T) {
	for name, edit := range map[string]func(*config){
		"empty id":                     func(c *config) { c.ID = "" },
		"id inconsistent with runtime": func(c *config) { c.ID = "com.example_a.App" },
		"id too long":                  func(c *config) { c.ID = "com." + strings.Repeat("a", 125) },
		"path name":                    func(c *config) { c.Name = "../Notes" },
		"control":                      func(c *config) { c.Name = "Notes\nType=Link" },
		"empty executable":             func(c *config) { c.Executable = "" },
		"path executable":              func(c *config) { c.Executable = "../../notes" },
		"Exec field code":              func(c *config) { c.Executable = "notes%U" },
		"device name":                  func(c *config) { c.Executable = "cOn" },
		"version":                      func(c *config) { c.Version = "v1.0.0" },
		"prerelease":                   func(c *config) { c.Version = "1.0.0-rc1" },
		"overflow":                     func(c *config) { c.Version = "65536.0.0" },
		"negative build":               func(c *config) { c.Build = -1 },
		"large build":                  func(c *config) { c.Build = 10000 },
		"zero build":                   func(c *config) { c.Build = 0 },
		"source list":                  func(c *config) { c.Main = "main.go" },
		"package pattern":              func(c *config) { c.Main = "./..." },
		"category injection":           func(c *config) { c.Linux.Categories = []string{"Utility;Game"} },
		"metadata parent":              func(c *config) { c.Metadata = "../appinfo" },
		"metadata root":                func(c *config) { c.Metadata = "." },
		"icon extension":               func(c *config) { c.Icon = "app.jpg" },
		"native icon":                  func(c *config) { c.Windows.Icon = "app.svg" },
	} {
		t.Run(name, func(t *testing.T) {
			c := testConfig(t)
			edit(c)
			if err := c.validate(); err == nil {
				t.Fatalf("accepted invalid config: %+v", c)
			}
		})
	}
	for _, extra := range []string{"\nunknown = 1", "\npermission = true"} {
		c := testConfig(t)
		path := filepath.Join(c.dir, "goui.toml")
		writeTestFile(t, path, []byte(validConfig+extra))
		if _, err := loadConfig(path); err == nil {
			t.Fatal("accepted unknown field")
		}
	}
}

func TestGeneratedFilesProtected(t *testing.T) {
	path := filepath.Join(t.TempDir(), "goui_windows_amd64.syso")
	writeTestFile(t, path, []byte("user file"))
	if err := writeGenerated(path, []byte("generated")); err == nil {
		t.Fatal("overwrote user file")
	}
	data, _ := os.ReadFile(path)
	if string(data) != "user file" {
		t.Fatal("user file changed")
	}
	path = filepath.Join(t.TempDir(), "generated.go")
	if err := writeGenerated(path, []byte("one")); err != nil {
		t.Fatal(err)
	}
	if err := writeGenerated(path, []byte("two")); err != nil {
		t.Fatal(err)
	}
	writeTestFile(t, path, []byte("edited"))
	if err := writeGenerated(path, []byte("three")); err == nil {
		t.Fatal("overwrote edited generated file")
	}
	data, _ = os.ReadFile(path)
	if string(data) != "edited" {
		t.Fatal("edited file changed")
	}
}

func TestMetadata(t *testing.T) {
	c := testConfig(t)
	c.Metadata = "internal/appinfo"
	path, err := generateMetadata(c)
	if err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(path)
	if err != nil || !strings.Contains(string(data), `ID      = "org.golang-gui.PackagingTest"`) || !strings.Contains(string(data), "package appinfo") {
		t.Fatalf("metadata: %s, %v", data, err)
	}
	c.Build = 3
	if _, err := generateMetadata(c); err != nil {
		t.Fatal(err)
	}
	data, _ = os.ReadFile(path)
	if !strings.Contains(string(data), "Build   = 3") {
		t.Fatalf("metadata was not regenerated: %s", data)
	}
}

func TestGeneratedFileSymlink(t *testing.T) {
	dir := t.TempDir()
	user := filepath.Join(dir, "user-file")
	link := filepath.Join(dir, "generated")
	writeTestFile(t, user, []byte("original"))
	if err := os.Symlink(user, link); err != nil {
		t.Skipf("symlink creation not available: %v", err)
	}
	if err := writeGenerated(link, []byte("generated")); err == nil {
		t.Fatal("replaced a symlink")
	}
	data, _ := os.ReadFile(user)
	if string(data) != "original" {
		t.Fatal("symlink target changed")
	}
	info, err := os.Lstat(link)
	if err != nil || info.Mode()&os.ModeSymlink == 0 {
		t.Fatal("symlink changed", err)
	}
}
