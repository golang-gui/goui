package main

import (
	"archive/zip"
	"bytes"
	"context"
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"encoding/xml"
	"errors"
	"fmt"
	"io"
	"math/big"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/golang-gui/goui/cmd/goui/internal/androidres"
)

func mobileXML(value string) string {
	var b bytes.Buffer
	xml.EscapeText(&b, []byte(value))
	return b.String()
}

func mobileManifest(c *config) []byte {
	permission := ""
	if c.Android.Internet {
		permission = `<uses-permission android:name="android.permission.INTERNET"/>`
	}
	return []byte(fmt.Sprintf(`<manifest xmlns:android="http://schemas.android.com/apk/res/android" package="%s" android:versionCode="%d" android:versionName="%s">
<uses-sdk android:minSdkVersion="%d" android:targetSdkVersion="%d"/>%s
<application android:label="%s" android:icon="@mipmap/app_icon" android:hasCode="true" android:debuggable="%t" android:allowBackup="false" android:extractNativeLibs="true" android:theme="@android:style/Theme.Material.Light.NoActionBar">
<activity android:name="android.app.NativeActivity" android:exported="true"><meta-data android:name="android.app.lib_name" android:value="goui"/>
<intent-filter><action android:name="android.intent.action.MAIN"/><category android:name="android.intent.category.LAUNCHER"/></intent-filter></activity></application></manifest>
`, mobileXML(c.ID), c.Build, mobileXML(c.Version), c.Android.MinSDK, c.Android.TargetSDK, permission, mobileXML(c.Name), c.Android.Debuggable))
}

func mobilePlist(c *config, variant string) []byte {
	var b strings.Builder
	b.WriteString(`<?xml version="1.0" encoding="UTF-8"?><!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd"><plist version="1.0"><dict>`)
	platform := "iPhoneOS"
	if variant == "simulator" {
		platform = "iPhoneSimulator"
	}
	for _, kv := range [][2]string{{"CFBundleIdentifier", c.ID}, {"CFBundleDisplayName", c.Name}, {"CFBundleName", c.Name}, {"CFBundleExecutable", c.Executable}, {"CFBundlePackageType", "APPL"}, {"CFBundleInfoDictionaryVersion", "6.0"}, {"CFBundleShortVersionString", c.Version}, {"CFBundleVersion", strconv.Itoa(c.Build)}, {"MinimumOSVersion", c.IOS.MinimumVersion}, {"DTPlatformName", strings.ToLower(platform)}} {
		fmt.Fprintf(&b, "<key>%s</key><string>%s</string>", kv[0], mobileXML(kv[1]))
	}
	fmt.Fprintf(&b, "<key>CFBundleSupportedPlatforms</key><array><string>%s</string></array>", platform)
	b.WriteString(`<key>UIDeviceFamily</key><array><integer>1</integer></array><key>LSRequiresIPhoneOS</key><true/>
<key>CFBundleIcons</key><dict><key>CFBundlePrimaryIcon</key><dict><key>CFBundleIconFiles</key><array><string>AppIcon60x60</string></array><key>UIPrerenderedIcon</key><true/></dict></dict>
<key>UILaunchScreen</key><dict/>
<key>UIApplicationSceneManifest</key><dict><key>UIApplicationSupportsMultipleScenes</key><false/><key>UISceneConfigurations</key><dict><key>UIWindowSceneSessionRoleApplication</key><array><dict><key>UISceneConfigurationName</key><string>GOUI</string><key>UISceneDelegateClassName</key><string>GoUIScene</string></dict></array></dict></dict>
<key>UISupportedInterfaceOrientations</key><array><string>UIInterfaceOrientationPortrait</string><string>UIInterfaceOrientationPortraitUpsideDown</string><string>UIInterfaceOrientationLandscapeLeft</string><string>UIInterfaceOrientationLandscapeRight</string></array></dict></plist>
`)
	return []byte(b.String())
}

func generateMobile(c *config, t target, m *mobileComponent, o mobileOptions, directory string) ([]string, error) {
	icon, err := loadIcon(c)
	if err != nil {
		return nil, err
	}
	files := map[string][]byte{}
	if t.OS == "android" {
		if c.Android.TargetSDK > m.TargetSDK {
			return nil, fmt.Errorf("android target_sdk %d exceeds component support %d", c.Android.TargetSDK, m.TargetSDK)
		}
		files["AndroidManifest.xml"] = mobileManifest(c)
		for _, s := range []struct {
			name string
			size int
		}{{"mdpi", 48}, {"hdpi", 72}, {"xhdpi", 96}, {"xxhdpi", 144}, {"xxxhdpi", 192}} {
			data, err := pngBytes(icon.image(s.size))
			if err != nil {
				return nil, err
			}
			files["res/mipmap-"+s.name+"/app_icon.png"] = data
		}
	} else {
		files["Info.plist"] = mobilePlist(c, o.Variant)
		for _, s := range []struct {
			name string
			size int
		}{{"AppIcon60x60@2x.png", 120}, {"AppIcon60x60@3x.png", 180}, {"AppIcon1024.png", 1024}} {
			data, err := pngBytes(icon.image(s.size))
			if err != nil {
				return nil, err
			}
			files[s.name] = data
		}
	}
	var result []string
	for _, name := range sortedNames(files) {
		path := filepath.Join(directory, filepath.FromSlash(name))
		if err = mobileWriteFile(path, files[name]); err != nil {
			return nil, err
		}
		result = append(result, path)
	}
	return result, nil
}

func mobilePackageFormat(c *config, t target, o mobileOptions, format string) (string, error) {
	if format == "" {
		format = "apk"
		if t.OS == "ios" {
			format = "app"
		}
	}
	if t.OS == "android" && format != "apk" || t.OS == "ios" && format != "app" && format != "ipa" {
		return "", fmt.Errorf("unsupported mobile package format %q", format)
	}
	if format == "ipa" && o.Variant != "device" {
		return "", fmt.Errorf("IPA requires iOS device variant")
	}
	if t.OS == "ios" && !o.Unsigned && (c.IOS.Signing.PKCS12 == "" || c.IOS.Signing.Profile == "") {
		return "", fmt.Errorf("iOS signing requires ios.signing.pkcs12 and profile; use -unsigned for subsequent signing")
	}
	return format, nil
}

func packageMobile(ctx context.Context, c *config, t target, m *mobileComponent, o mobileOptions, binary, format, output string, errOut io.Writer) (string, error) {
	format, err := mobilePackageFormat(c, t, o, format)
	if err != nil {
		return "", err
	}
	if output == "" {
		output = filepath.Join(c.dir, "dist", mobileFolder(t, o.Variant), c.Name+"."+format)
	} else {
		output = c.path(output)
	}
	if filepath.Ext(output) != "."+format {
		return "", fmt.Errorf("mobile package output must end in .%s", format)
	}
	if _, err := os.Lstat(output); !errors.Is(err, os.ErrNotExist) {
		if err != nil {
			return "", err
		}
		return "", fmt.Errorf("package output already exists: %s", output)
	}
	if err := os.MkdirAll(filepath.Dir(output), 0755); err != nil {
		return "", err
	}
	work, err := os.MkdirTemp(filepath.Dir(output), ".goui-mobile-package-*")
	if err != nil {
		return "", err
	}
	defer os.RemoveAll(work)
	resources := filepath.Join(work, "resources")
	if _, err = generateMobile(c, t, m, o, resources); err != nil {
		return "", err
	}
	if t.OS == "android" {
		apk, err := packageAndroid(ctx, c, m, binary, resources, work, errOut)
		if err != nil {
			return "", err
		}
		if err = ctx.Err(); err != nil {
			return "", err
		}
		return output, os.Rename(apk, output)
	}
	app := filepath.Join(work, "Payload", c.Name+".app")
	if err = os.MkdirAll(app, 0755); err != nil {
		return "", err
	}
	entries, err := os.ReadDir(resources)
	if err != nil {
		return "", err
	}
	for _, e := range entries {
		if err = copyFile(filepath.Join(resources, e.Name()), filepath.Join(app, e.Name()), 0644); err != nil {
			return "", err
		}
	}
	if err = copyFile(binary, filepath.Join(app, c.Executable), 0755); err != nil {
		return "", err
	}
	if !o.Unsigned {
		if err = signIOS(ctx, c, app); err != nil {
			return "", err
		}
	} else if errOut != nil {
		fmt.Fprintln(errOut, "iOS output is unsigned and requires your own signing or cloud-device re-signing before device installation.")
	}
	if err = ctx.Err(); err != nil {
		return "", err
	}
	if format == "app" {
		return output, os.Rename(app, output)
	}
	archive := filepath.Join(work, "application.ipa")
	if err = mobileZipDirectory(filepath.Join(work, "Payload"), archive, "Payload", filepath.Join(app, c.Executable)); err != nil {
		return "", err
	}
	return output, os.Rename(archive, output)
}

func packageAndroid(ctx context.Context, c *config, m *mobileComponent, binary, resources, work string, errOut io.Writer) (string, error) {
	manifest, err := androidres.Manifest(mobileManifest(c))
	if err != nil {
		return "", err
	}
	table, err := androidres.Resources(c.ID)
	if err != nil {
		return "", err
	}
	for name, content := range map[string][]byte{"AndroidManifest.xml": manifest, "resources.arsc": table} {
		if err = mobileWriteFile(filepath.Join(work, name), content); err != nil {
			return "", err
		}
	}
	dex, err := m.file(m.Dex)
	if err != nil {
		return "", err
	}
	files := map[string]string{"lib/arm64-v8a/libgoui.so": binary, "classes.dex": dex, "AndroidManifest.xml": filepath.Join(work, "AndroidManifest.xml"), "resources.arsc": filepath.Join(work, "resources.arsc")}
	for _, icon := range androidres.Icons {
		files[icon.Path()] = filepath.Join(resources, filepath.FromSlash(icon.Path()))
	}
	unsigned := filepath.Join(work, "unsigned.apk")
	archive, err := os.Create(unsigned)
	if err != nil {
		return "", err
	}
	w := zip.NewWriter(archive)
	names := make([]string, 0, len(files))
	for name := range files {
		names = append(names, name)
	}
	sort.Strings(names)
	for _, name := range names {
		method := uint16(zip.Deflate)
		if name == "resources.arsc" {
			method = zip.Store
		}
		var entry io.Writer
		entry, err = w.CreateHeader(&zip.FileHeader{Name: name, Method: method})
		if err == nil {
			var f *os.File
			f, err = os.Open(files[name])
			if err == nil {
				_, err = io.Copy(entry, f)
				if closeErr := f.Close(); err == nil {
					err = closeErr
				}
			}
		}
		if err != nil {
			break
		}
	}
	if closeErr := w.Close(); err == nil {
		err = closeErr
	}
	if closeErr := archive.Close(); err == nil {
		err = closeErr
	}
	if err != nil {
		return "", err
	}
	apk := filepath.Join(work, "application.apk")
	if err = signAndroid(ctx, c, unsigned, apk, errOut); err != nil {
		return "", err
	}
	return apk, nil
}

func androidDevelopmentKey() (string, string, error) {
	dir, err := os.UserConfigDir()
	if err != nil {
		return "", "", err
	}
	parent := filepath.Join(dir, "goui", "keys")
	if err = os.MkdirAll(parent, 0700); err != nil {
		return "", "", err
	}
	dir = filepath.Join(parent, "android-development")
	key, cert := filepath.Join(dir, "private.pem"), filepath.Join(dir, "certificate.pem")
	if _, err = os.Lstat(dir); err == nil {
		return key, cert, checkAndroidKey(key, cert)
	} else if !os.IsNotExist(err) {
		return "", "", err
	}
	tmp, err := os.MkdirTemp(parent, ".android-key-*")
	if err != nil {
		return "", "", err
	}
	defer os.RemoveAll(tmp)
	private, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		return "", "", err
	}
	serial, err := rand.Int(rand.Reader, new(big.Int).Lsh(big.NewInt(1), 128))
	if err != nil {
		return "", "", err
	}
	now := time.Now()
	template := &x509.Certificate{SerialNumber: serial, Subject: pkix.Name{CommonName: "GOUI Android Development"}, NotBefore: now.Add(-time.Hour), NotAfter: now.AddDate(20, 0, 0), KeyUsage: x509.KeyUsageDigitalSignature}
	der, err := x509.CreateCertificate(rand.Reader, template, template, &private.PublicKey, private)
	if err != nil {
		return "", "", err
	}
	pkcs8, err := x509.MarshalPKCS8PrivateKey(private)
	if err != nil {
		return "", "", err
	}
	if err = os.WriteFile(filepath.Join(tmp, "private.pem"), pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: pkcs8}), 0600); err != nil {
		return "", "", err
	}
	if err = os.WriteFile(filepath.Join(tmp, "certificate.pem"), pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: der}), 0600); err != nil {
		return "", "", err
	}
	if err = os.Rename(tmp, dir); err != nil {
		if _, statErr := os.Stat(dir); statErr != nil {
			return "", "", err
		}
	}
	return key, cert, checkAndroidKey(key, cert)
}

func checkAndroidKey(key, cert string) error {
	k, err := os.ReadFile(key)
	if err != nil {
		return err
	}
	block, _ := pem.Decode(k)
	if block == nil {
		return fmt.Errorf("invalid development private key")
	}
	private, err := x509.ParsePKCS8PrivateKey(block.Bytes)
	if err != nil {
		return err
	}
	rsaKey, ok := private.(*rsa.PrivateKey)
	if !ok {
		return fmt.Errorf("invalid development key type")
	}
	c, err := os.ReadFile(cert)
	if err != nil {
		return err
	}
	block, _ = pem.Decode(c)
	if block == nil {
		return fmt.Errorf("invalid development certificate")
	}
	certificate, err := x509.ParseCertificate(block.Bytes)
	if err != nil {
		return err
	}
	public, ok := certificate.PublicKey.(*rsa.PublicKey)
	if !ok || !rsaKey.PublicKey.Equal(public) {
		return fmt.Errorf("development certificate/private key mismatch")
	}
	return nil
}

func mobileZipFile(w *zip.Writer, name, path string) error {
	f, err := os.Open(path)
	if err != nil {
		return err
	}
	defer f.Close()
	info, err := f.Stat()
	if err != nil {
		return err
	}
	header, err := zip.FileInfoHeader(info)
	if err != nil {
		return err
	}
	header.Name = name
	header.Method = zip.Deflate
	header.Modified = time.Date(2000, 1, 1, 0, 0, 0, 0, time.UTC)
	out, err := w.CreateHeader(header)
	if err != nil {
		return err
	}
	_, err = io.Copy(out, f)
	return err
}

func mobileZipDirectory(root, output, prefix, executable string) error {
	f, err := os.Create(output)
	if err != nil {
		return err
	}
	w := zip.NewWriter(f)
	err = filepath.WalkDir(root, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			return nil
		}
		if !d.Type().IsRegular() {
			return fmt.Errorf("non-regular bundle resource: %s", path)
		}
		name, err := filepath.Rel(root, path)
		if err != nil {
			return err
		}
		if path == executable {
			header := &zip.FileHeader{Name: prefix + "/" + filepath.ToSlash(name), Method: zip.Deflate}
			header.SetMode(0755)
			out, err := w.CreateHeader(header)
			if err != nil {
				return err
			}
			f, err := os.Open(path)
			if err != nil {
				return err
			}
			_, err = io.Copy(out, f)
			closeErr := f.Close()
			if err != nil {
				return err
			}
			return closeErr
		}
		return mobileZipFile(w, prefix+"/"+filepath.ToSlash(name), path)
	})
	closeErr := w.Close()
	fileErr := f.Close()
	if err != nil {
		return err
	}
	if closeErr != nil {
		return closeErr
	}
	return fileErr
}
