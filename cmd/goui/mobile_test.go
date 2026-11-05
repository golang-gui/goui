package main

import (
	"archive/zip"
	"context"
	"encoding/json"
	"encoding/xml"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	mobilebase "github.com/golang-gui/goui/platform/mobile/base"
)

func mobileTestComponent(t *testing.T) (string, *mobileComponent) {
	t.Helper()
	directory := t.TempDir()
	root := filepath.Join(directory, "ios-arm64")
	if err := os.MkdirAll(root, 0755); err != nil {
		t.Fatal(err)
	}
	write := func(name string, data []byte) {
		t.Helper()
		path := filepath.Join(root, name)
		if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, data, 0600); err != nil {
			t.Fatal(err)
		}
	}
	marshal := func(name string, v any) {
		t.Helper()
		data, err := json.Marshal(v)
		if err != nil {
			t.Fatal(err)
		}
		write(name, data)
	}
	write("base/packages/cgo.a", []byte("test archive"))
	hash, _ := mobileHash(filepath.Join(root, "base/packages/cgo.a"))
	base := mobilebase.BaseManifest{Format: 1, GoVersion: "go1.25.8", GOOS: "ios", GOARCH: "arm64", Architecture: "v8.0", BuildMode: "c-archive", Variant: "device", MinimumVersion: "15.0", Packages: map[string]mobilebase.BasePackage{"runtime/cgo": {Archive: "packages/cgo.a", SHA256: hash, SourceSHA256: strings.Repeat("0", 64), BuildID: "test/archive"}}}
	marshal("base/base.json", base)
	write("inputs/sdk/usr/lib/libSystem.tbd", []byte("test SDK stub"))
	hash, _ = mobileHash(filepath.Join(root, "inputs/sdk/usr/lib/libSystem.tbd"))
	inputs := mobilebase.NativeInputs{Format: 1, GOOS: "ios", GOARCH: "arm64", Variant: "device", MinimumVersion: "15.0", SDKVersion: "15.0", Provenance: mobilebase.NativeProvenance{Name: "test", Version: "1", Source: "unit fixture", License: "test only"}, Files: map[string]string{"sdk/usr/lib/libSystem.tbd": hash}, IOS: &mobilebase.IOSLinkInputs{Sysroot: "sdk"}}
	marshal("inputs/inputs.json", inputs)
	for _, name := range []string{"tools/tools.json", "driver", "linker"} {
		write(name, []byte("unit fixture"))
	}
	m := &mobileComponent{Format: 1, HostOS: runtime.GOOS, HostArch: runtime.GOARCH, Target: "ios/arm64", Variant: "device", Base: "base/base.json", Inputs: "inputs/inputs.json", Tools: "tools/tools.json", Driver: "driver", Linker: "linker", Files: map[string]string{}}
	err := filepath.WalkDir(root, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			return nil
		}
		name, _ := filepath.Rel(root, path)
		hash, err := mobileHash(path)
		if err != nil {
			return err
		}
		m.Files[filepath.ToSlash(name)] = hash
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	marshal("mobile.json", m)
	return directory, m
}

func TestMobileComponent(t *testing.T) {
	dir, _ := mobileTestComponent(t)
	target := target{"ios", "arm64"}
	m, err := openMobileComponent(dir, target, "device")
	if err != nil {
		t.Fatal(err)
	}
	if _, err = m.file("../outside"); err == nil {
		t.Fatal("必须拒绝越界路径")
	}
	if _, err = m.file("unlisted"); err == nil {
		t.Fatal("必须拒绝未声明文件")
	}
	if _, err = openMobileComponent(dir, target, "simulator"); err == nil {
		t.Fatal("不能混用 device/simulator")
	}
	if err = os.WriteFile(filepath.Join(m.root, "driver"), []byte("changed"), 0600); err != nil {
		t.Fatal(err)
	}
	if _, err = openMobileComponent(dir, target, "device"); err == nil || !strings.Contains(err.Error(), "SHA-256") {
		t.Fatalf("未发现工具篡改: %v", err)
	}
}

func TestMobileConfiguration(t *testing.T) {
	for _, name := range []string{"android/arm64", "ios/arm64"} {
		if _, err := parseTarget(name); err != nil {
			t.Fatal(err)
		}
	}
	c := testConfig(t)
	c.ID = "org.example.App"
	o := mobileOptions{}
	if err := validateMobile(c, target{"android", "arm64"}, &o); err != nil {
		t.Fatal(err)
	}
	if c.Android.MinSDK != 29 || c.Android.TargetSDK != 35 || o.Variant != "device" {
		t.Fatal("默认版本/variant 不正确")
	}
	c.ID = "org.invalid-name.App"
	if err := validateMobile(c, target{"android", "arm64"}, &o); err == nil {
		t.Fatal("Android 不能接受带连字符的 package")
	}
	if _, err := mobileVersion("15.0<script>"); err == nil {
		t.Fatal("必须拒绝无效 iOS 最低版本")
	}
	for input, want := range map[string]string{"15.0": "15.0", "26.3.1": "26.3.1"} {
		if _, err := mobileVersion(input); err != nil {
			t.Fatalf("%s: %v", want, err)
		}
	}
	flags, err := mobileLDFlags(`-s -w -X 'example.org/app.Version=hello world'`)
	if err != nil || len(flags) != 4 || flags[3] != "example.org/app.Version=hello world" {
		t.Fatalf("flags=%q error=%v", flags, err)
	}
	if _, err = mobileLDFlags(`-X 'unfinished`); err == nil {
		t.Fatal("必须拒绝未闭合引号")
	}
}

func TestMobileResources(t *testing.T) {
	c := testConfig(t)
	c.Name = "A & B <mobile>"
	c.Android.MinSDK = 29
	c.Android.TargetSDK = 35
	c.Android.Internet = true
	c.IOS.MinimumVersion = "15.0"
	xmlData := mobileManifest(c)
	decoder := xml.NewDecoder(strings.NewReader(string(xmlData)))
	for {
		if _, err := decoder.Token(); err == io.EOF {
			break
		} else if err != nil {
			t.Fatal(err)
		}
	}
	if strings.Contains(string(xmlData), "probe_onCreate") || !strings.Contains(string(xmlData), "android.permission.INTERNET") {
		t.Fatal("必须使用生产入口与明确的权限声明")
	}
	for _, variant := range []string{"device", "simulator"} {
		data := mobilePlist(c, variant)
		if !strings.Contains(string(data), "GoUIScene") || strings.Contains(string(data), "ProbeScene") {
			t.Fatal("Scene 声明不正确")
		}
		decoder = xml.NewDecoder(strings.NewReader(string(data)))
		for {
			if _, err := decoder.Token(); err == io.EOF {
				break
			} else if err != nil {
				t.Fatal(err)
			}
		}
	}
}

func TestMobileUnsignedIPA(t *testing.T) {
	c := testConfig(t)
	c.IOS.MinimumVersion = "15.0"
	binary := filepath.Join(t.TempDir(), "program")
	if err := os.WriteFile(binary, []byte("native executable fixture"), 0755); err != nil {
		t.Fatal(err)
	}
	o := mobileOptions{Variant: "device", Unsigned: true}
	output := filepath.Join(t.TempDir(), "test.ipa")
	if _, err := packageMobile(context.Background(), c, target{"ios", "arm64"}, &mobileComponent{}, o, binary, "ipa", output, io.Discard); err != nil {
		t.Fatal(err)
	}
	r, err := zip.OpenReader(output)
	if err != nil {
		t.Fatal(err)
	}
	defer r.Close()
	seen := map[string]bool{}
	for _, f := range r.File {
		seen[f.Name] = true
		if strings.HasSuffix(f.Name, "/"+c.Executable) && f.Mode().Perm()&0111 == 0 {
			t.Fatal("IPA 丢失可执行权限")
		}
	}
	for _, name := range []string{"Info.plist", c.Executable, "AppIcon60x60@2x.png"} {
		if !seen["Payload/"+c.Name+".app/"+name] {
			t.Fatalf("缺失 IPA 文件: %s", name)
		}
	}
	if _, err = packageMobile(context.Background(), c, target{"ios", "arm64"}, &mobileComponent{}, o, binary, "ipa", output, io.Discard); err == nil {
		t.Fatal("不能覆盖已有 IPA")
	}
	o.Variant = "simulator"
	if _, err = packageMobile(context.Background(), c, target{"ios", "arm64"}, &mobileComponent{}, o, binary, "ipa", filepath.Join(t.TempDir(), "sim.ipa"), io.Discard); err == nil {
		t.Fatal("模拟器不能生成真机 IPA")
	}
	o.Variant = "device"
	o.Unsigned = false
	if _, err = packageMobile(context.Background(), c, target{"ios", "arm64"}, &mobileComponent{}, o, binary, "ipa", filepath.Join(t.TempDir(), "signed.ipa"), io.Discard); err == nil {
		t.Fatal("不能把未签名 IPA 声称为已签名")
	}
}

func TestMobileBuildReceipt(t *testing.T) {
	path := filepath.Join(t.TempDir(), "program")
	os.WriteFile(path, []byte("changed binary"), 0600)
	r := mobileReceipt{Format: 1, Target: "ios/arm64", Variant: "device", SHA256: strings.Repeat("0", 64), ProfileSHA256: "profile"}
	data, _ := json.Marshal(r)
	os.WriteFile(path+".goui.json", data, 0600)
	if err := checkMobileReceipt(path, target{"ios", "arm64"}, &mobileComponent{digest: "profile"}, mobileOptions{Variant: "device"}); err == nil || !strings.Contains(err.Error(), "mismatch") {
		t.Fatalf("未拒绝二进制/收据错配: %v", err)
	}
}

func TestAndroidDevelopmentIdentity(t *testing.T) {
	variable := "XDG_CONFIG_HOME"
	if runtime.GOOS == "windows" {
		variable = "APPDATA"
	}
	if runtime.GOOS == "darwin" {
		variable = "HOME"
	}
	t.Setenv(variable, t.TempDir())
	key, cert, err := androidDevelopmentKey()
	if err != nil {
		t.Fatal(err)
	}
	first, err := os.ReadFile(key)
	if err != nil {
		t.Fatal(err)
	}
	keyAgain, certAgain, err := androidDevelopmentKey()
	if err != nil {
		t.Fatal(err)
	}
	second, _ := os.ReadFile(keyAgain)
	if string(first) != string(second) || cert != certAgain {
		t.Fatal("不能在重建 APK 时改变开发签名")
	}
	if runtime.GOOS != "windows" {
		info, _ := os.Stat(key)
		if info.Mode().Perm() != 0600 {
			t.Fatal("私钥权限必须是 0600")
		}
	}
}
