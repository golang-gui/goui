package main

import (
	"bytes"
	"context"
	"crypto"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/asn1"
	"encoding/binary"
	"io"
	"math/big"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	gomacho "github.com/blacktop/go-macho"
	machotypes "github.com/blacktop/go-macho/pkg/codesign/types"
	"github.com/digitorus/timestamp"
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

// 合成身份仅验证格式、身份绑定和完整性，不代表 Apple 授权或公证通过。
func macOSTestIdentity(t *testing.T) (*rsa.PrivateKey, []*x509.Certificate, *rsa.PrivateKey, *x509.Certificate, *x509.CertPool) {
	t.Helper()
	caKey, ca := iosTestCertificate(t, 101, pkix.Name{CommonName: "GOUI Mac test root"}, nil, nil, true)
	makeLeaf := func(serial int64, tsa bool) (*rsa.PrivateKey, *x509.Certificate) {
		key, err := rsa.GenerateKey(rand.Reader, 2048)
		if err != nil {
			t.Fatal(err)
		}
		template := &x509.Certificate{SerialNumber: big.NewInt(serial), Subject: pkix.Name{CommonName: "GOUI Mac test", OrganizationalUnit: []string{"GOUI_TEST"}}, NotBefore: time.Now().Add(-time.Hour), NotAfter: time.Now().Add(time.Hour), KeyUsage: x509.KeyUsageDigitalSignature, ExtKeyUsage: []x509.ExtKeyUsage{x509.ExtKeyUsageCodeSigning}, ExtraExtensions: []pkix.Extension{{Id: developerIDApplicationOID, Critical: true, Value: []byte{5, 0}}}}
		if tsa {
			template.ExtKeyUsage = []x509.ExtKeyUsage{x509.ExtKeyUsageTimeStamping}
			usage, err := asn1.Marshal([]asn1.ObjectIdentifier{{1, 3, 6, 1, 5, 5, 7, 3, 8}})
			if err != nil {
				t.Fatal(err)
			}
			template.ExtraExtensions = []pkix.Extension{{Id: asn1.ObjectIdentifier{2, 5, 29, 37}, Critical: true, Value: usage}}
		}
		data, err := x509.CreateCertificate(rand.Reader, template, ca, &key.PublicKey, caKey)
		if err != nil {
			t.Fatal(err)
		}
		cert, err := x509.ParseCertificate(data)
		if err != nil {
			t.Fatal(err)
		}
		return key, cert
	}
	key, cert := makeLeaf(102, false)
	tsaKey, tsaCert := makeLeaf(103, true)
	roots := x509.NewCertPool()
	roots.AddCert(ca)
	return key, []*x509.Certificate{cert, ca}, tsaKey, tsaCert, roots
}

func TestMacOSGoSigning(t *testing.T) {
	ctx := context.Background()
	key, certs, tsaKey, tsaCert, roots := macOSTestIdentity(t)
	identity, err := developerIDIdentity(key, certs, roots, time.Now())
	if err != nil {
		t.Fatal(err)
	}
	var wrongNonce, wrongHash atomic.Bool
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		raw, err := io.ReadAll(r.Body)
		if err != nil {
			t.Error(err)
			w.WriteHeader(500)
			return
		}
		request, err := timestamp.ParseRequest(raw)
		if err != nil {
			t.Error(err)
			w.WriteHeader(400)
			return
		}
		stamp := timestamp.Timestamp{HashAlgorithm: crypto.SHA256, HashedMessage: request.HashedMessage, Nonce: request.Nonce, Time: time.Now().UTC(), SerialNumber: big.NewInt(1), Policy: asn1.ObjectIdentifier{1, 2, 3, 4}, Certificates: certs[1:], AddTSACertificate: true}
		if wrongNonce.Load() {
			stamp.Nonce = big.NewInt(0)
		}
		if wrongHash.Load() {
			sum := sha256.Sum256([]byte("other signature"))
			stamp.HashedMessage = sum[:]
		}
		data, err := stamp.CreateResponseWithOpts(tsaCert, tsaKey, crypto.SHA256)
		if err != nil {
			t.Error(err)
			w.WriteHeader(500)
			return
		}
		w.Header().Set("Content-Type", "application/timestamp-reply")
		_, _ = w.Write(data)
	}))
	defer server.Close()
	stamp := func(sig []byte) ([]byte, error) {
		return requestAppleTimestamp(ctx, server.Client(), server.URL, roots, sig)
	}
	c := testConfig(t)
	binaryPath, err := build(ctx, c, target{"darwin", "arm64"}, "", "", "", io.Discard, io.Discard)
	if err != nil {
		t.Fatal(err)
	}
	original, err := os.ReadFile(binaryPath)
	if err != nil {
		t.Fatal(err)
	}
	app, err := packageApp(ctx, c, target{"darwin", "arm64"}, "app", "", "")
	if err != nil {
		t.Fatal(err)
	}
	if err = signMacOSBundle(ctx, c, app, identity, stamp); err != nil {
		t.Fatal(err)
	}
	hash, err := verifyMacOSBundle(app, roots)
	if err != nil || hash == ([20]byte{}) {
		t.Fatalf("签名验签: %x %v", hash, err)
	}
	path := filepath.Join(app, "Contents", "MacOS", c.Executable)
	signed, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	headerEnd := 32 + int(binary.LittleEndian.Uint32(original[20:]))
	signatureOffset := len(original)
	m, err := gomacho.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	if m.CodeSignature().CodeDirectories[0].Header.Flags&machotypes.RUNTIME == 0 {
		t.Fatal("必须开启 Hardened Runtime")
	}
	signatureOffset = int(m.CodeSignature().Offset)
	m.Close()
	if !bytes.Equal(original[headerEnd:signatureOffset], signed[headerEnd:signatureOffset]) {
		t.Fatal("签名不得修改可执行内容")
	}
	for _, name := range []string{"code", "resource", "added", "missing", "plist"} {
		t.Run(name, func(t *testing.T) {
			file := path
			before := signed
			switch name {
			case "code":
				changed := bytes.Clone(signed)
				changed[headerEnd+64] ^= 1
				if err := os.WriteFile(file, changed, 0755); err != nil {
					t.Fatal(err)
				}
			case "resource", "missing":
				file = filepath.Join(app, "Contents", "Resources", "AppIcon.icns")
				before, err = os.ReadFile(file)
				if err != nil {
					t.Fatal(err)
				}
				if name == "missing" {
					err = os.Remove(file)
				} else {
					err = os.WriteFile(file, []byte("changed"), 0644)
				}
			case "added":
				file = filepath.Join(app, "Contents", "Resources", "extra")
				err = os.WriteFile(file, []byte("unsealed"), 0644)
			case "plist":
				file = filepath.Join(app, "Contents", "Info.plist")
				before, err = os.ReadFile(file)
				if err == nil {
					err = os.WriteFile(file, bytes.Replace(before, []byte(c.ID), []byte("org.other.App"), 1), 0644)
				}
			}
			if err != nil {
				t.Fatal(err)
			}
			if _, err := verifyMacOSBundle(app, roots); err == nil {
				t.Fatal("修改、删除或增加内容必须验签失败")
			}
			if name == "added" {
				err = os.Remove(file)
			} else {
				err = os.WriteFile(file, before, 0644)
			}
			if err != nil {
				t.Fatal(err)
			}
		})
	}
	wrongNonce.Store(true)
	if _, err = stamp([]byte("nonce test")); err == nil {
		t.Fatal("必须拒绝错误 nonce")
	}
	wrongNonce.Store(false)
	wrongHash.Store(true)
	if _, err = stamp([]byte("digest test")); err == nil {
		t.Fatal("必须拒绝错误时间戳摘要")
	}
	wrongHash.Store(false)
	if _, err = developerIDIdentity(key, certs, x509.NewCertPool(), time.Now()); err == nil {
		t.Fatal("必须验证 Apple 信任链")
	}
	if _, err = developerIDIdentity(tsaKey, certs, roots, time.Now()); err == nil {
		t.Fatal("必须验证公私钥匹配")
	}
	if _, err = developerIDIdentity(key, certs, roots, time.Now().Add(2*time.Hour)); err == nil {
		t.Fatal("必须拒绝过期证书")
	}
	if _, err = verifyMacOSBundle(app, x509.NewCertPool()); err == nil {
		t.Fatal("验签必须校验时间戳信任链")
	}

	c.MacOS.Signing.PKCS12 = filepath.Join(c.dir, "missing.p12")
	output := filepath.Join(c.dir, "Failure.app")
	if _, err = packageApp(ctx, c, target{"darwin", "arm64"}, "app", "", output); err == nil {
		t.Fatal("不能静默回退为未签名")
	}
	if _, err = os.Stat(output); !os.IsNotExist(err) {
		t.Fatal("签名失败不发布输出", err)
	}
}

func TestAppleRoleCertificate(t *testing.T) {
	key, certs, _, _, roots := macOSTestIdentity(t)
	leaf := certs[0]
	if len(leaf.UnhandledCriticalExtensions) != 1 {
		t.Fatal("合成证书应包含未处理的 critical Developer ID 扩展")
	}
	if _, err := developerIDIdentity(key, certs, roots, time.Now()); err != nil {
		t.Fatal(err)
	}
	if len(leaf.UnhandledCriticalExtensions) != 1 {
		t.Fatal("不能改写原始证书")
	}
	unknown := *leaf
	unknown.UnhandledCriticalExtensions = append([]asn1.ObjectIdentifier{{1, 2, 3, 4}}, leaf.UnhandledCriticalExtensions...)
	if _, err := developerIDIdentity(key, []*x509.Certificate{&unknown, certs[1]}, roots, time.Now()); err == nil {
		t.Fatal("不能忽略其他未知的 critical 扩展")
	}
	malformed := *leaf
	malformed.Extensions = append([]pkix.Extension(nil), leaf.Extensions...)
	for i, e := range malformed.Extensions {
		if e.Id.Equal(developerIDApplicationOID) {
			malformed.Extensions[i].Value = []byte{1, 1, 0}
		}
	}
	if _, err := appleRoleCertificate(&malformed, developerIDApplicationOID); err == nil {
		t.Fatal("角色扩展必须使用已理解的 NULL 值")
	}
	if _, err := appleRoleCertificate(leaf, asn1.ObjectIdentifier{1, 2, 3, 4}); err == nil {
		t.Fatal("不能接受不存在的角色")
	}
}

func TestMacOSDistributionConfig(t *testing.T) {
	c := testConfig(t)
	path := filepath.Join(c.dir, "goui.toml")
	writeTestFile(t, path, []byte(validConfig+`
[macos.signing]
pkcs12 = 'developer.p12'
password_env = 'GOUI_MAC_TEST_PASSWORD'
`))
	loaded, err := loadConfig(path)
	if err != nil {
		t.Fatal(err)
	}
	if loaded.MacOS.Signing.PKCS12 != "developer.p12" || loaded.MacOS.Signing.PasswordEnv != "GOUI_MAC_TEST_PASSWORD" {
		t.Fatal("签名和公证配置未正确解码")
	}
	loaded.MacOS.Sandbox = true
	if err = loaded.validate(); err != nil {
		t.Fatal("沙箱可使用 PKCS#12 身份", err)
	}
	loaded.MacOS.SigningIdentity = "-"
	if err = loaded.validate(); err == nil {
		t.Fatal("不能同时选择两种签名入口")
	}
	loaded.MacOS.SigningIdentity = ""
	loaded.MacOS.Signing.PKCS12 = ""
	loaded.MacOS.Sandbox = false
	if err = loaded.validate(); err == nil {
		t.Fatal("密码环境变量需要 PKCS#12")
	}
}
