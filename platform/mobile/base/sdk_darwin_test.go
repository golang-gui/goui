package base

import (
	"context"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func TestNativeIOSSDKSelection(t *testing.T) {
	want, err := exec.Command("/usr/bin/xcrun", "--sdk", "iphonesimulator", "--show-sdk-path").Output()
	if err != nil {
		t.Skip("未安装iOS Simulator SDK")
	}
	t.Setenv("SDKROOT", "")
	t.Setenv("CC", "/usr/bin/clang")
	t.Setenv("CXX", "/usr/bin/clang++")
	t.Setenv("CGO_ENABLED", "1")
	goCmd := filepath.Join(runtime.GOROOT(), "bin/go")
	env, err := nativeBuildEnvironment(context.Background(), BuildOptions{GoCommand: goCmd}, &NativeInputs{GOOS: "ios", GOARCH: "arm64", Variant: "simulator", MinimumVersion: "15.0"})
	if err != nil {
		t.Fatal(err)
	}
	values := map[string]string{}
	for _, entry := range env {
		key, value, _ := strings.Cut(entry, "=")
		values[key] = value
	}
	if values["SDKROOT"] != strings.TrimSpace(string(want)) || !strings.Contains(values["CC"], "arm64-apple-ios15.0-simulator") {
		t.Fatalf("SDK或目标错误: sdk=%q cc=%q want=%q", values["SDKROOT"], values["CC"], want)
	}
}
