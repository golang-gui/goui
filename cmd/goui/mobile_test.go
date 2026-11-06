package main

import (
	"archive/zip"
	"bytes"
	"context"
	"crypto/rand"
	"crypto/rsa"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/binary"
	"encoding/json"
	"encoding/xml"
	"io"
	"math/big"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/agusibrahim/apksig-go/pkg/apkverifier"
	"github.com/agusibrahim/apksig-go/pkg/datasource"
	"github.com/aluedeke/go-codesign/pkg/codesign"
	"github.com/golang-gui/golink"
	"go.mozilla.org/pkcs7"
	appleplist "howett.net/plist"
	"software.sslmate.com/src/go-pkcs12"

	mobilebase "github.com/golang-gui/goui/platform/mobile/base"
)

func TestMobileCgoStrategies(t *testing.T) {
	for _, mode := range []string{"prebuilt", "native"} {
		if err := checkMobileCgoSettings(mode, map[string]string{"CGO_ENABLED": "1"}, nil); err != nil {
			t.Fatal(err)
		}
		if err := checkMobileCgoSettings(mode, map[string]string{"CGO_ENABLED": "0"}, nil); err == nil {
			t.Fatal("不得静默覆盖用户明确关闭的 cgo")
		}
	}
	settings := map[string]string{"CC": "target-clang", "CGO_CFLAGS": "-DMOBILE_SENTINEL=73"}
	if err := checkMobileCgoSettings("prebuilt", settings, nil); err == nil {
		t.Fatal("预置模式不能静默忽略用户编译器或参数")
	}
	if err := checkMobileCgoSettings("native", settings, nil); err != nil {
		t.Fatal(err)
	}
	if err := checkMobileCgoSettings("prebuilt", map[string]string{"CGO_CFLAGS": ""}, map[string]string{"CGO_CFLAGS": "-O2 -g"}); err != nil {
		t.Fatal("Go 将空编译参数环境值解释为默认值", err)
	}
	flags := map[string]string{"CGO_CFLAGS": "-O2 -g -DCUSTOM=73"}
	if err := checkMobileCgoSettings("prebuilt", map[string]string{"CGO_CFLAGS": flags["CGO_CFLAGS"]}, flags); err != nil {
		t.Fatal("显式匹配自有基座参数应允许", err)
	}
	if err := checkMobileCgoSettings("prebuilt", nil, flags); err == nil {
		t.Fatal("不得忽略自有基座参数不匹配")
	}
	ctx := context.Background()
	for _, target := range []struct {
		t       target
		variant string
	}{{target{"android", "arm64"}, "device"}, {target{"ios", "arm64"}, "device"}, {target{"ios", "arm64"}, "simulator"}} {
		m, err := embeddedMobileComponent(ctx, "", target.t, target.variant)
		if err != nil {
			t.Fatal(err)
		}
		if m.Base != "" {
			t.Fatal("目标资源不应选择 Go 基座")
		}
		hash := mobileTargetDigest(m)
		m.Files["base/example"] = strings.Repeat("a", 64)
		if mobileTargetDigest(m) != hash {
			t.Fatal("目标身份不应随 Go 支持组件变化")
		}
	}
}

func TestIOSGoSigning(t *testing.T) {
	// 使用合成证书和合成 profile 验证格式、密码、身份绑定及签名完整性；
	// 不将这些材料当成 Apple 授权证书，不据此宣称能安装到真实 iPhone。
	c := testConfig(t)
	c.ID = "org.example.Mobile"
	c.IOS.MinimumVersion = "15.0"
	root := t.TempDir()
	caKey, ca := iosTestCertificate(t, 1, pkix.Name{CommonName: "GOUI Test Root"}, nil, nil, true)
	key, cert := iosTestCertificate(t, 42, pkix.Name{CommonName: "GOUI signing test", OrganizationalUnit: []string{"GOUI_TEST"}}, ca, caKey, false)
	profileCAKey, profileCA := iosTestCertificate(t, 43, pkix.Name{CommonName: "Apple iPhone Certification Authority"}, ca, caKey, true)
	profileKey, profileCert := iosTestCertificate(t, 44, pkix.Name{CommonName: "Apple iPhone OS Provisioning Profile Signing"}, profileCA, profileCAKey, false)
	p12, err := pkcs12.Modern.Encode(key, cert, []*x509.Certificate{ca}, "private-test-password")
	if err != nil {
		t.Fatal(err)
	}
	c.IOS.Signing = mobileSigning{PKCS12: filepath.Join(root, "identity.p12"), Profile: filepath.Join(root, "profile.mobileprovision"), PasswordEnv: "GOUI_TEST_IOS_PASSWORD"}
	if err = os.WriteFile(c.IOS.Signing.PKCS12, p12, 0600); err != nil {
		t.Fatal(err)
	}
	t.Setenv("GOUI_TEST_IOS_PASSWORD", "private-test-password")
	profile := codesign.ProvisioningProfile{Name: "合成测试", TeamIdentifier: []string{"GOUI_TEST"}, ApplicationIdentifierPrefix: []string{"GOUI_TEST"}, DeveloperCertificates: [][]byte{cert.Raw}, Platform: []string{"iOS"}, CreationDate: time.Now().Add(-time.Hour), ExpirationDate: time.Now().Add(time.Hour), Entitlements: map[string]interface{}{"application-identifier": "GOUI_TEST.org.example.*", "get-task-allow": true}}
	forged := false
	writeProfile := func() {
		t.Helper()
		b, err := appleplist.Marshal(profile, appleplist.XMLFormat)
		if err != nil {
			t.Fatal(err)
		}
		cms, err := pkcs7.NewSignedData(b)
		if err != nil {
			t.Fatal(err)
		}
		if forged {
			err = cms.AddSignerChain(cert, key, []*x509.Certificate{ca}, pkcs7.SignerInfoConfig{})
		} else {
			err = cms.AddSignerChain(profileCert, profileKey, []*x509.Certificate{profileCA, ca}, pkcs7.SignerInfoConfig{})
		}
		if err != nil {
			t.Fatal(err)
		}
		b, err = cms.Finish()
		if err != nil {
			t.Fatal(err)
		}
		if err = os.WriteFile(c.IOS.Signing.Profile, b, 0600); err != nil {
			t.Fatal(err)
		}
	}
	writeProfile()
	roots := x509.NewCertPool()
	roots.AddCert(ca)
	identity, rawProfile, entitlements, err := iosSigningIdentity(c, roots)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(entitlements), "GOUI_TEST.org.example.Mobile") || strings.Contains(string(entitlements), "org.example.*") {
		t.Fatal("必须生成应用自己的 entitlement，不得原样复制 wildcard")
	}
	app := filepath.Join(root, "Test.app")
	if err = os.Mkdir(app, 0755); err != nil {
		t.Fatal(err)
	}
	if err = os.WriteFile(filepath.Join(app, "Info.plist"), mobilePlist(c, "device"), 0644); err != nil {
		t.Fatal(err)
	}
	if err = os.WriteFile(filepath.Join(app, "icon.png"), []byte("resource fixture"), 0644); err != nil {
		t.Fatal(err)
	}
	obj := filepath.Join(root, "main.o")
	if err = os.WriteFile(obj, mobileTestMachOObject(), 0600); err != nil {
		t.Fatal(err)
	}
	if err = golink.Link(golink.Request{Format: 1, Target: "ios/arm64", Output: filepath.Join(app, c.Executable), Objects: []string{obj}, IOS: &golink.IOSOptions{Variant: "device", MinimumVersion: "15.0", SDKVersion: "26.2"}}); err != nil {
		t.Fatal(err)
	}
	if err = signIOSBundle(context.Background(), c, app, identity, rawProfile, entitlements); err != nil {
		t.Fatal(err)
	}
	if err = verifyIOSSignature(app, c.Executable, cert); err != nil {
		t.Fatal(err)
	}
	if err = signIOS(context.Background(), c, t.TempDir()); err == nil || !strings.Contains(err.Error(), "certificate chain") {
		t.Fatalf("合成证书不能通过生产入口的 Apple 信任链校验: %v", err)
	}
	// 显式导出合成签名 App，供维护者在 macOS 上运行 codesign --verify。
	// 默认测试不导出文件；导出目录不包含合成证书的私钥。
	if output := os.Getenv("GOUI_TEST_IOS_SIGNED_APP"); output != "" {
		err := filepath.WalkDir(app, func(path string, d os.DirEntry, err error) error {
			if err != nil {
				return err
			}
			rel, err := filepath.Rel(app, path)
			if err != nil {
				return err
			}
			dest := filepath.Join(output, rel)
			if d.IsDir() {
				return os.MkdirAll(dest, 0755)
			}
			info, err := d.Info()
			if err != nil {
				return err
			}
			return copyFile(path, dest, info.Mode().Perm())
		})
		if err != nil {
			t.Fatal(err)
		}
	}
	if err = os.WriteFile(filepath.Join(app, "icon.png"), []byte("changed resource"), 0644); err != nil {
		t.Fatal(err)
	}
	if err = verifyIOSSignature(app, c.Executable, cert); err == nil {
		t.Fatal("修改资源后必须验签失败")
	}
	c.ID = "org.other.App"
	if _, _, _, err = iosSigningIdentity(c, roots); err == nil {
		t.Fatal("不能接受不匹配的 App ID")
	}
	c.ID = "org.example.Mobile"
	forged = true
	writeProfile()
	if _, _, _, err = iosSigningIdentity(c, roots); err == nil || !strings.Contains(err.Error(), "profile signer") {
		t.Fatalf("开发者证书不能自行签发 profile: %v", err)
	}
	forged = false
	profile.ExpirationDate = time.Now().Add(-time.Minute)
	writeProfile()
	if _, _, _, err = iosSigningIdentity(c, roots); err == nil {
		t.Fatal("不能接受过期 profile")
	}
	t.Setenv("GOUI_TEST_IOS_PASSWORD", "wrong")
	if _, _, _, err = iosSigningIdentity(c, roots); err == nil || strings.Contains(err.Error(), "private-test-password") {
		t.Fatal("错误密码应失败且不能泄露密码")
	}
}

func iosTestCertificate(t *testing.T, serial int64, subject pkix.Name, parent *x509.Certificate, parentKey *rsa.PrivateKey, ca bool) (*rsa.PrivateKey, *x509.Certificate) {
	t.Helper()
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatal(err)
	}
	template := &x509.Certificate{SerialNumber: big.NewInt(serial), Subject: subject, NotBefore: time.Now().Add(-time.Hour), NotAfter: time.Now().Add(24 * time.Hour), KeyUsage: x509.KeyUsageDigitalSignature, BasicConstraintsValid: true, IsCA: ca}
	if ca {
		template.KeyUsage |= x509.KeyUsageCertSign
	} else {
		template.ExtKeyUsage = []x509.ExtKeyUsage{x509.ExtKeyUsageCodeSigning}
	}
	if parent == nil {
		parent = template
		parentKey = key
	}
	der, err := x509.CreateCertificate(rand.Reader, template, parent, &key.PublicKey, parentKey)
	if err != nil {
		t.Fatal(err)
	}
	cert, err := x509.ParseCertificate(der)
	if err != nil {
		t.Fatal(err)
	}
	return key, cert
}

func mobileTestMachOObject() []byte {
	// ARM64 的 main 返回零。这里只用作平台无关的链接和签名格式测试。
	const commands = 152 + 24 + 24
	const text = 32 + commands
	const symbols = text + 8
	const stringsOffset = symbols + 16
	b := make([]byte, stringsOffset+7)
	u16, u32, u64 := binary.LittleEndian.PutUint16, binary.LittleEndian.PutUint32, binary.LittleEndian.PutUint64
	u32(b, 0xfeedfacf)
	u32(b[4:], 0x100000c)
	u32(b[12:], 1)
	u32(b[16:], 3)
	u32(b[20:], commands)
	s := b[32:]
	u32(s, 0x19)
	u32(s[4:], 152)
	u64(s[32:], 8)
	u64(s[40:], text)
	u64(s[48:], 8)
	u32(s[64:], 1)
	section := s[72:]
	copy(section, "__text")
	copy(section[16:], "__TEXT")
	u64(section[40:], 8)
	u32(section[48:], text)
	u32(section[52:], 2)
	u32(section[64:], 0x80000400)
	tab := b[32+152:]
	u32(tab, 2)
	u32(tab[4:], 24)
	u32(tab[8:], symbols)
	u32(tab[12:], 1)
	u32(tab[16:], stringsOffset)
	u32(tab[20:], 7)
	version := b[32+152+24:]
	u32(version, 0x32)
	u32(version[4:], 24)
	u32(version[8:], 2)
	u32(version[12:], 15<<16)
	u32(version[16:], 26<<16|2<<8)
	u32(b[text:], 0x52800000)
	u32(b[text+4:], 0xd65f03c0)
	u32(b[symbols:], 1)
	b[symbols+4] = 15
	b[symbols+5] = 1
	u16(b[symbols+6:], 0)
	copy(b[stringsOffset:], "\x00_main\x00")
	return b
}

func TestEmbeddedMobileComponent(t *testing.T) {
	switch runtime.GOOS {
	case "windows":
		t.Setenv("LOCALAPPDATA", t.TempDir())
	case "darwin":
		t.Setenv("HOME", t.TempDir())
	default:
		t.Setenv("XDG_CACHE_HOME", t.TempDir())
	}
	for _, version := range []string{"go1.24.13", "go1.25.8", "go1.26.10", "go1.27.3"} {
		for _, test := range []struct{ os, variant string }{{"android", "device"}, {"ios", "device"}, {"ios", "simulator"}} {
			t.Run(version+"-"+test.os+"-"+test.variant, func(t *testing.T) {
				m, err := embeddedMobileComponent(context.Background(), version, target{test.os, "arm64"}, test.variant)
				if err != nil {
					t.Fatal(err)
				}
				if m.Format != 2 || m.HostOS != "" || m.HostArch != "" || m.Driver != "" || m.Linker != "" || m.Aapt2 != "" || m.AndroidJar != "" {
					t.Fatal("内置组件不应携带宿主工具或完整 SDK")
				}
				basePath, _ := m.file(m.Base)
				base, err := mobilebase.BaseOpen(basePath)
				if err != nil || base.GoVersion != version || base.Format != 2 {
					t.Fatalf("基座校验失败: %v", err)
				}
				for _, p := range base.Packages {
					if p.Archive != "" || len(p.Objects) == 0 || len(p.GeneratedGo) == 0 {
						t.Fatal("不得内置完整Go归档")
					}
				}
				inputPath, _ := m.file(m.Inputs)
				if _, err := mobilebase.OpenNativeInputs(inputPath); err != nil {
					t.Fatal(err)
				}
				next, err := embeddedMobileComponent(context.Background(), version, target{test.os, "arm64"}, test.variant)
				if err != nil || next.root != m.root || next.digest != m.digest {
					t.Fatalf("未复用相同缓存: %v", err)
				}
				if err := os.WriteFile(basePath, []byte("corrupted"), 0600); err != nil {
					t.Fatal(err)
				}
				if _, err := embeddedMobileComponent(context.Background(), version, target{test.os, "arm64"}, test.variant); err == nil {
					t.Fatal("缓存篡改必须失败，不能重新计算摘要后接受")
				}
			})
		}
	}
	if _, err := embeddedMobileComponent(context.Background(), "go1.25.9", target{"android", "arm64"}, "device"); err == nil {
		t.Fatal("不能猜测使用其他 Go 版本的归档")
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := embeddedMobileComponent(ctx, "go1.25.8", target{"android", "arm64"}, "device"); err != context.Canceled {
		t.Fatalf("取消未生效: %v", err)
	}
}

func TestAndroidGoPackaging(t *testing.T) {
	// 只验证组包、资源和签名；占位二进制不用于启动或宣称设备验收通过。
	switch runtime.GOOS {
	case "windows":
		t.Setenv("APPDATA", t.TempDir())
	case "darwin":
		t.Setenv("HOME", t.TempDir())
	default:
		t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	}
	c := testConfig(t)
	c.Name = "测试 🐹 & العربية"
	c.Android.MinSDK, c.Android.TargetSDK = 29, 35
	root := t.TempDir()
	dex := filepath.Join(root, "classes.dex")
	if err := os.WriteFile(dex, []byte("dex fixture"), 0600); err != nil {
		t.Fatal(err)
	}
	hash, _ := mobileHash(dex)
	m := &mobileComponent{TargetSDK: 35, Dex: "classes.dex", Files: map[string]string{"classes.dex": hash}, root: root}
	binary := filepath.Join(root, "libgoui.so")
	if err := os.WriteFile(binary, []byte("native fixture"), 0600); err != nil {
		t.Fatal(err)
	}
	for _, production := range []bool{false, true} {
		if production {
			keyPath, certPath, err := androidDevelopmentKey()
			if err != nil {
				t.Fatal(err)
			}
			identity, err := tls.LoadX509KeyPair(certPath, keyPath)
			if err != nil {
				t.Fatal(err)
			}
			cert, err := x509.ParseCertificate(identity.Certificate[0])
			if err != nil {
				t.Fatal(err)
			}
			p12, err := pkcs12.Modern.Encode(identity.PrivateKey, cert, nil, "test-password")
			if err != nil {
				t.Fatal(err)
			}
			c.Android.Signing = mobileSigning{PKCS12: filepath.Join(root, "identity.p12"), PasswordEnv: "GOUI_TEST_SIGN_PASSWORD"}
			if err := os.WriteFile(c.Android.Signing.PKCS12, p12, 0600); err != nil {
				t.Fatal(err)
			}
			t.Setenv("GOUI_TEST_SIGN_PASSWORD", "test-password")
		}
		output := filepath.Join(t.TempDir(), "app.apk")
		var log bytes.Buffer
		if _, err := packageMobile(context.Background(), c, target{"android", "arm64"}, m, mobileOptions{Variant: "device"}, binary, "apk", output, &log); err != nil {
			t.Fatal(err)
		}
		if strings.Contains(log.String(), "development") == production {
			t.Fatal("正式证书不应报告为开发签名")
		}
		data, err := os.ReadFile(output)
		if err != nil {
			t.Fatal(err)
		}
		result, err := apkverifier.Verify(datasource.NewBytes(data), 29, 35)
		if err != nil || !result.V2Verified || !result.V3Verified {
			t.Fatalf("签名校验失败: %v %#v", err, result)
		}
		r, err := zip.NewReader(bytes.NewReader(data), int64(len(data)))
		if err != nil {
			t.Fatal(err)
		}
		for _, f := range r.File {
			if f.Name == "resources.arsc" {
				offset, err := f.DataOffset()
				if err != nil {
					t.Fatal(err)
				}
				if offset%4 != 0 || f.Method != zip.Store {
					t.Fatal("资源表必须不压缩且四字节对齐")
				}
				data[offset] ^= 1
			}
		}
		result, err = apkverifier.Verify(datasource.NewBytes(data), 29, 35)
		if err == nil && result.Verified {
			t.Fatal("修改资源后签名必须失效")
		}
	}
}

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

func TestMobileNativeSharedDependencies(t *testing.T) {
	// 最小 ELF 只表达独立的 DT_NEEDED；不作为可运行 App，也不绕过最终二进制校验。
	for _, library := range []string{"libc.so", "libuser-private.so"} {
		stringsData := append([]byte{0}, []byte(library+"\x00")...)
		data := make([]byte, 256+len(stringsData)+32)
		copy(data, "\x7fELF")
		data[4], data[5], data[6] = 2, 1, 1
		le := binary.LittleEndian
		le.PutUint16(data[16:], 3)
		le.PutUint16(data[18:], 183)
		le.PutUint32(data[20:], 1)
		le.PutUint64(data[40:], 64)
		le.PutUint16(data[52:], 64)
		le.PutUint16(data[58:], 64)
		le.PutUint16(data[60:], 3)
		str := data[128:192]
		le.PutUint32(str[4:], 3)
		le.PutUint64(str[24:], 256)
		le.PutUint64(str[32:], uint64(len(stringsData)))
		le.PutUint64(str[48:], 1)
		dyn := data[192:256]
		le.PutUint32(dyn[4:], 6)
		le.PutUint64(dyn[24:], uint64(256+len(stringsData)))
		le.PutUint64(dyn[32:], 32)
		le.PutUint32(dyn[40:], 1)
		le.PutUint64(dyn[48:], 8)
		le.PutUint64(dyn[56:], 16)
		copy(data[256:], stringsData)
		le.PutUint64(data[256+len(stringsData):], 1)
		le.PutUint64(data[264+len(stringsData):], 1)
		path := filepath.Join(t.TempDir(), "app.so")
		if err := os.WriteFile(path, data, 0600); err != nil {
			t.Fatal(err)
		}
		err := checkMobileNativeLibraries(path, target{"android", "arm64"}, &mobileComponent{Files: map[string]string{"inputs/lib/libc.so": "fixture"}})
		if library == "libc.so" && err != nil {
			t.Fatal(err)
		}
		if library != "libc.so" && (err == nil || !strings.Contains(err.Error(), "not included by mobile packaging")) {
			t.Fatalf("未打包的动态依赖应明确失败: %v", err)
		}
	}
}
