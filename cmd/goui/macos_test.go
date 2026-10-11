package main

import (
	"context"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func TestMacOSSandboxConfig(t *testing.T) {
	for name, edit := range map[string]func(*config){
		"unsigned":                  func(c *config) { c.MacOS.Sandbox = true },
		"files without sandbox":     func(c *config) { c.MacOS.UserFiles = "read-write" },
		"credentials without files": func(c *config) { c.MacOS.Sandbox = true; c.MacOS.SigningIdentity = "-"; c.MacOS.PersistAccess = true },
		"invalid files":             func(c *config) { c.MacOS.UserFiles = "all" },
		"invalid identity":          func(c *config) { c.MacOS.SigningIdentity = "dev\nother" },
	} {
		t.Run(name, func(t *testing.T) {
			c := testConfig(t)
			edit(c)
			if c.validate() == nil {
				t.Fatal("accepted invalid sandbox configuration")
			}
		})
	}
	c := testConfig(t)
	c.MacOS.Sandbox, c.MacOS.PersistAccess = true, true
	c.MacOS.UserFiles, c.MacOS.SigningIdentity = "read-write", "-"
	if err := c.validate(); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(c.dir, "goui.toml")
	writeTestFile(t, path, []byte(validConfig+"\n[macos]\nsandbox = true\nuser_files = 'read-only'\npersist_access = true\nsigning_identity = '-'\n"))
	loaded, err := loadConfig(path)
	if err != nil || !loaded.MacOS.Sandbox || loaded.MacOS.UserFiles != "read-only" || !loaded.MacOS.PersistAccess {
		t.Fatalf("decode: %+v %v", loaded, err)
	}
}

func TestMacOSEntitlements(t *testing.T) {
	c := testConfig(t)
	c.MacOS.Sandbox, c.MacOS.PersistAccess = true, true
	c.MacOS.UserFiles, c.MacOS.SigningIdentity = "read-only", "-"
	files, err := generate(context.Background(), c, target{"darwin", "arm64"}, "", "")
	if err != nil {
		t.Fatal(err)
	}
	var found bool
	for _, file := range files {
		if filepath.Base(file) != "Entitlements.plist" {
			continue
		}
		found = true
		data, err := os.ReadFile(file)
		if err != nil {
			t.Fatal(err)
		}
		text := string(data)
		for _, key := range []string{"com.apple.security.app-sandbox", "com.apple.security.files.user-selected.read-only", "com.apple.security.files.bookmarks.app-scope"} {
			if !strings.Contains(text, "<key>"+key+"</key><true/>") {
				t.Fatalf("missing %s", key)
			}
		}
		if strings.Contains(text, "read-write") || strings.Contains(text, "network") {
			t.Fatal("unrequested rights")
		}
	}
	if !found {
		t.Fatal("no separate entitlement output")
	}
}

// Runs actual codesign on macOS, not a mock or a desktop launch. The signed
// sandbox application's file access is separately tested by the window fixture.
func TestMacOSSigning(t *testing.T) {
	if runtime.GOOS != "darwin" {
		t.Skip("requires native macOS codesign")
	}
	c := testConfig(t)
	c.MacOS.Sandbox, c.MacOS.PersistAccess = true, true
	c.MacOS.UserFiles, c.MacOS.SigningIdentity = "read-write", "-"
	target := target{"darwin", "arm64"}
	if _, err := build(context.Background(), c, target, "", "", "", io.Discard, io.Discard); err != nil {
		t.Fatal(err)
	}
	app, err := packageApp(c, target, "app", "", "")
	if err != nil {
		t.Fatal(err)
	}
	data, err := exec.Command("/usr/bin/codesign", "-d", "--entitlements", ":-", app).CombinedOutput()
	if err != nil || !strings.Contains(string(data), "com.apple.security.app-sandbox") || !strings.Contains(string(data), "com.apple.security.files.bookmarks.app-scope") {
		t.Fatalf("signature entitlements: %s %v", data, err)
	}
	c.MacOS.SigningIdentity = "goui-nonexistent-test-signing-identity"
	output := filepath.Join(c.dir, "Failure.app")
	if _, err := packageApp(c, target, "app", "", output); err == nil {
		t.Fatal("bad identity succeeded")
	}
	if _, err := os.Stat(output); !os.IsNotExist(err) {
		t.Fatalf("published unsuccessful package: %v", err)
	}
}
