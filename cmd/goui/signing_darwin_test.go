package main

import (
	"context"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// 使用真实 codesign；沙箱应用的文件访问另由窗口程序验证。
func TestMacOSSigning(t *testing.T) {
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
