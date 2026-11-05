// 移动端验收维护工具，不属于面向应用开发者的打包接口。
// 在仓库根运行 go run ./tests/window/platform/mobile_build <base|native|build|package> config.json。
// base/native 需要维护者 SDK；build 仅消费已发布的归档、目标输入和链接工具。
// 配置、SDK、签名、生成源码、构建缓存与产物必须放在仓库外。详见 README.md。
package main

import (
	"archive/zip"
	"bytes"
	"crypto/sha256"
	"encoding/json"
	"encoding/xml"
	"fmt"
	"github.com/golang-gui/goui/internal/nativegen"
	"io"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
)

type configuration struct {
	Output, Target, Variant, Painter, Fixture                               string
	Go, GouiBase, Driver, Linker, Base, Inputs, Tools, ConsumerPath         string
	CC                                                                      json.RawMessage `json:"cc"`
	Java, AndroidJar, D8, Aapt2, Signer, Key, Cert, Apksigner, Font, Assets string
	Modules                                                                 []string
}
type builder struct {
	config              configuration
	repo, root, fixture string
	steps               []string
}

func main() {
	if err := run(os.Args[1:]); err != nil {
		log.Fatal(err)
	}
}
func run(args []string) error {
	if len(args) != 2 {
		return fmt.Errorf("usage: mobile_build <base|native|build|package> config.json")
	}
	data, err := os.ReadFile(args[1])
	if err != nil {
		return err
	}
	var cfg configuration
	if err = json.Unmarshal(data, &cfg); err != nil {
		return err
	}
	if cfg.Fixture == "" {
		cfg.Fixture = "mobile_paint"
	}
	if cfg.Painter == "" {
		cfg.Painter = "software"
	}
	switch cfg.Fixture {
	case "mobile_paint", "mobile_surface", "mobile_start", "android_gui", "android_text":
	default:
		return fmt.Errorf("unknown fixture %q", cfg.Fixture)
	}
	if cfg.Target != "android" && cfg.Target != "ios" {
		return fmt.Errorf("invalid target")
	}
	if cfg.Target == "android" && cfg.Variant != "device" || cfg.Target == "ios" && cfg.Variant != "device" && cfg.Variant != "simulator" {
		return fmt.Errorf("invalid variant")
	}
	if cfg.Painter != "software" && cfg.Painter != "opengl" {
		return fmt.Errorf("invalid painter")
	}
	repo, err := findRepo()
	if err != nil {
		return err
	}
	root, err := filepath.Abs(cfg.Output)
	if err != nil {
		return err
	}
	if err = nativegen.CheckOutput(root, repo); err != nil {
		return err
	}
	if err = os.MkdirAll(root, 0755); err != nil {
		return err
	}
	b := &builder{config: cfg, repo: repo, root: root, fixture: filepath.Join(repo, "tests/window/platform", cfg.Fixture)}
	switch args[0] {
	case "base":
		err = b.base()
	case "native":
		err = b.native()
	case "build":
		err = b.build()
	case "package":
		err = b.pack()
	default:
		return fmt.Errorf("unknown stage %q", args[0])
	}
	if err != nil {
		return err
	}
	receipt := map[string]any{"stage": args[0], "target": cfg.Target, "variant": cfg.Variant, "fixture": cfg.Fixture, "painter": cfg.Painter, "steps": b.steps, "sha256": map[string]string{}}
	hashes := receipt["sha256"].(map[string]string)
	for _, name := range []string{"main.o", "uikit.o", "probe.o", "libgoui.so", "Probe", "probe.apk"} {
		if p, err := os.ReadFile(filepath.Join(root, name)); err == nil {
			hashes[name] = fmt.Sprintf("%x", sha256.Sum256(p))
		}
	}
	return writeJSON(filepath.Join(root, args[0]+"-receipt.json"), receipt)
}
func findRepo() (string, error) {
	p, err := os.Getwd()
	if err != nil {
		return "", err
	}
	p, err = filepath.EvalSymlinks(p)
	if err != nil {
		return "", err
	}
	for {
		if data, err := os.ReadFile(filepath.Join(p, "go.mod")); err == nil && strings.HasPrefix(strings.Join(strings.Fields(string(data)), " "), "module github.com/golang-gui/goui ") {
			return p, nil
		}
		parent := filepath.Dir(p)
		if parent == p {
			return "", fmt.Errorf("run from within goui repository")
		}
		p = parent
	}
}
func writeJSON(path string, value any) error {
	data, err := json.MarshalIndent(value, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, append(data, '\n'), 0644)
}
func (b *builder) command(label string, env []string, dir string, args ...string) error {
	if len(args) == 0 || args[0] == "" {
		return fmt.Errorf("%s: missing tool", label)
	}
	file, err := os.Create(filepath.Join(b.root, label+".log"))
	if err != nil {
		return err
	}
	cmd := exec.Command(args[0], args[1:]...)
	cmd.Env = env
	cmd.Dir = dir
	cmd.Stdout = file
	cmd.Stderr = file
	err = cmd.Run()
	file.Close()
	if err != nil {
		data, _ := os.ReadFile(file.Name())
		if len(data) > 6000 {
			data = data[len(data)-6000:]
		}
		return fmt.Errorf("%s: %w\n%s", label, err, data)
	}
	b.steps = append(b.steps, label)
	fmt.Println(label + " passed")
	return nil
}
func (b *builder) workspace(extra string) (string, error) {
	modules := append([]string{b.repo}, b.config.Modules...)
	if extra != "" {
		modules = append(modules, extra)
	}
	var s strings.Builder
	s.WriteString("go 1.25.8\nuse (\n")
	for _, p := range modules {
		p, err := filepath.Abs(p)
		if err != nil {
			return "", err
		}
		s.WriteString(strconv.Quote(p) + "\n")
	}
	s.WriteString(")\n")
	path := filepath.Join(b.root, "go.work")
	return path, os.WriteFile(path, []byte(s.String()), 0644)
}
func environment(values map[string]string) []string {
	result := os.Environ()
	for k, v := range values {
		prefix := k + "="
		filtered := result[:0]
		for _, s := range result {
			if !strings.HasPrefix(s, prefix) {
				filtered = append(filtered, s)
			}
		}
		result = append(filtered, prefix+v)
	}
	return result
}
func (b *builder) compiler() ([]string, error) {
	var v []string
	if json.Unmarshal(b.config.CC, &v) == nil && len(v) > 0 {
		return v, nil
	}
	var s string
	if json.Unmarshal(b.config.CC, &s) == nil && s != "" {
		return []string{s}, nil
	}
	return nil, fmt.Errorf("native stage requires cc")
}
func (b *builder) base() error {
	cc, err := b.compiler()
	if err != nil {
		return err
	}
	if len(cc) != 1 {
		return fmt.Errorf("base requires a compiler wrapper path")
	}
	source := filepath.Join(b.root, "base-main")
	if err = os.MkdirAll(source, 0755); err != nil {
		return err
	}
	if err = os.WriteFile(filepath.Join(source, "go.mod"), []byte("module example.com/goui-base\ngo 1.25.8\n"), 0644); err != nil {
		return err
	}
	if err = os.WriteFile(filepath.Join(source, "main.go"), []byte("package main\nimport _ \"github.com/golang-gui/goui/platform/mobile\"\nimport _ \"github.com/goexlib/cgo\"\nfunc main(){}\n"), 0644); err != nil {
		return err
	}
	work, err := b.workspace(source)
	if err != nil {
		return err
	}
	mode, minimum := "c-archive", "15.0"
	if b.config.Target == "android" {
		mode, minimum = "c-shared", "29"
	}
	env := environment(map[string]string{"GOWORK": work, "GOTOOLCHAIN": "local", "GOFLAGS": "", "GOOS": b.config.Target, "GOARCH": "arm64", "CGO_ENABLED": "1", "CC": cc[0], "CXX": cc[0]})
	return b.command("base-export", env, b.repo, b.config.GouiBase, "export", "-go", b.config.Go, "-dir", source, "-o", filepath.Join(b.root, "base"), "-buildmode", mode, "-variant", b.config.Variant, "-minimum", minimum, ".")
}
func (b *builder) native() error {
	cc, err := b.compiler()
	if err != nil {
		return err
	}
	work, err := b.workspace("")
	if err != nil {
		return err
	}
	generated := filepath.Join(b.root, "generated")
	env := environment(map[string]string{"GOUI_GENERATE_DIR": generated, "GOWORK": work, "GOTOOLCHAIN": "local", "GOFLAGS": ""})
	if err = b.command("generate", env, b.repo, b.config.Go, "generate", "./platform/mobile", "./platform/typography/androidtext", "./tests/window/platform/"+b.config.Fixture); err != nil {
		return err
	}
	extension := "android.c"
	gles := "0"
	if b.config.Painter == "opengl" {
		gles = "1"
	}
	flags := []string{"-O2", "-std=gnu11", "-DGOUI_GLES=" + gles}
	if b.config.Target == "ios" {
		extension = "ios.m"
		flags = append(flags, "-fobjc-arc")
	}
	input := filepath.Join(generated, "tests/window/platform", b.config.Fixture, "native", extension)
	command := append(append(append([]string{}, cc...), flags...), "-c", input, "-o", filepath.Join(b.root, "probe.o"))
	if err = b.command("native-probe", nil, b.repo, command...); err != nil {
		return err
	}
	if b.config.Target == "ios" {
		if b.config.Fixture == "mobile_start" {
			return nil
		}
		if err = b.command("generate-uikit", env, b.repo, b.config.Go, "generate", "./platform/darwin/frameworks/uikit"); err != nil {
			return err
		}
		metadata := append(append([]string{}, cc...), "-O2", "-fobjc-arc", "-c", filepath.Join(generated, "platform/darwin/frameworks/uikit/protocols.m"), "-o", filepath.Join(b.root, "uikit.o"))
		if err = b.command("native-uikit-metadata", nil, b.repo, metadata...); err != nil {
			return err
		}
		command = append(append([]string{}, cc...), "-O2", "-c", filepath.Join(generated, "platform/mobile/ios/main.c"), "-o", filepath.Join(b.root, "main.o"))
		return b.command("native-main", nil, b.repo, command...)
	}
	classes, dex := filepath.Join(b.root, "classes"), filepath.Join(b.root, "dex")
	if err = os.MkdirAll(classes, 0755); err != nil {
		return err
	}
	if err = os.MkdirAll(dex, 0755); err != nil {
		return err
	}
	java, err := filesWithSuffix(filepath.Join(generated, "platform/typography/androidtext/java"), ".java")
	if err != nil {
		return err
	}
	extra, err := filesWithSuffix(filepath.Dir(input), ".java")
	if err != nil {
		return err
	}
	java = append(java, extra...)
	if err = b.command("native-java", nil, b.repo, append([]string{b.config.Java, "-m", "jdk.compiler/com.sun.tools.javac.Main", "--release", "8", "-classpath", b.config.AndroidJar, "-d", classes}, java...)...); err != nil {
		return err
	}
	compiled, err := filesWithSuffix(classes, ".class")
	if err != nil {
		return err
	}
	if err = b.command("native-dex", nil, b.repo, append([]string{b.config.Java, "-cp", b.config.D8, "com.android.tools.r8.D8", "--lib", b.config.AndroidJar, "--min-api", "29", "--output", dex}, compiled...)...); err != nil {
		return err
	}
	if b.config.Fixture == "android_text" {
		return b.shapingReference()
	}
	return nil
}
func filesWithSuffix(root, suffix string) ([]string, error) {
	var files []string
	err := filepath.WalkDir(root, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if !d.IsDir() && strings.HasSuffix(path, suffix) {
			files = append(files, path)
		}
		return nil
	})
	sort.Strings(files)
	return files, err
}
func (b *builder) build() error {
	work, err := b.workspace("")
	if err != nil {
		return err
	}
	env := environment(map[string]string{"GOWORK": work, "GOTOOLCHAIN": "local", "GOFLAGS": "", "GOCACHE": filepath.Join(b.root, "consumer-cache"), "PATH": b.config.ConsumerPath})
	binary := "Probe"
	if b.config.Target == "android" {
		binary = "libgoui.so"
	}
	args := []string{b.config.GouiBase, "build", "-go", b.config.Go, "-driver", b.config.Driver, "-linker", b.config.Linker, "-base", b.config.Base, "-inputs", b.config.Inputs, "-tools", b.config.Tools, "-dir", b.fixture, "-o", filepath.Join(b.root, binary), "-object", filepath.Join(b.root, "probe.o"), "-entry", "-x"}
	if b.config.Target == "android" {
		args = append(args, "-library", "android")
	} else {
		if b.config.Fixture != "mobile_start" {
			args = append(args, "-object", filepath.Join(b.root, "main.o"), "-object", filepath.Join(b.root, "uikit.o"))
		}
		for _, name := range []string{"UIKit", "Foundation", "CoreGraphics", "QuartzCore", "OpenGLES"} {
			args = append(args, "-framework", name)
		}
		args = append(args, "-library", "objc")
	}
	if err = b.command("consume", env, b.repo, append(args, ".")...); err != nil {
		return err
	}
	data, err := os.ReadFile(filepath.Join(b.root, "consume.log"))
	if err != nil {
		return err
	}
	if bytes.Contains(data, []byte("/cgo -objdir")) || !bytes.Contains(data, []byte("# goui base: github.com/golang-gui/goui/platform/mobile/internal/entry")) {
		return fmt.Errorf("consumer did not reuse fixed entry archive")
	}
	if _, err = os.Stat(filepath.Join(b.fixture, "zz_goui_mobile_main.go")); !os.IsNotExist(err) {
		return fmt.Errorf("consumer modified application source")
	}
	return nil
}
func (b *builder) pack() error {
	if b.config.Target == "ios" {
		return b.packIOS()
	}
	return b.packAndroid()
}
func (b *builder) packAndroid() error {
	id := map[string]string{"mobile_paint": "mobilepaint", "mobile_surface": "mobilesurface", "mobile_start": "mobilestart", "android_gui": "androidgui", "android_text": "androidtext"}[b.config.Fixture]
	manifest := fmt.Sprintf(`<manifest xmlns:android="http://schemas.android.com/apk/res/android" package="org.golanggui.%s" android:versionCode="1" android:versionName="0.0.1">
<uses-sdk android:minSdkVersion="29" android:targetSdkVersion="35"/>
<application android:label="GOUI Mobile Probe" android:hasCode="true" android:debuggable="true" android:allowBackup="false" android:extractNativeLibs="true" android:theme="@android:style/Theme.Material.Light.NoActionBar">
<activity android:name="android.app.NativeActivity" android:exported="true"><meta-data android:name="android.app.lib_name" android:value="goui"/><meta-data android:name="android.app.func_name" android:value="probe_onCreate"/>
<intent-filter><action android:name="android.intent.action.MAIN"/><category android:name="android.intent.category.LAUNCHER"/></intent-filter></activity></application></manifest>`, id)
	path := filepath.Join(b.root, "AndroidManifest.xml")
	if err := os.WriteFile(path, []byte(manifest), 0644); err != nil {
		return err
	}
	unsigned := filepath.Join(b.root, "unsigned.apk")
	args := []string{b.config.Aapt2, "link", "-I", b.config.AndroidJar, "--manifest", path, "-o", unsigned}
	assets := b.config.Assets
	if assets == "" && b.config.Fixture == "android_text" {
		assets = filepath.Join(b.root, "assets")
	}
	if assets != "" {
		args = append(args, "-A", assets)
	}
	if err := b.command("resources", nil, b.repo, args...); err != nil {
		return err
	}
	if err := addZipFiles(unsigned, map[string]string{"lib/arm64-v8a/libgoui.so": filepath.Join(b.root, "libgoui.so"), "classes.dex": filepath.Join(b.root, "dex/classes.dex")}); err != nil {
		return err
	}
	apk := filepath.Join(b.root, "probe.apk")
	if err := b.command("sign", nil, b.repo, b.config.Signer, "-key", b.config.Key, "-cert", b.config.Cert, "-in", unsigned, "-out", apk); err != nil {
		return err
	}
	return b.command("verify-signature", nil, b.repo, b.config.Java, "-jar", b.config.Apksigner, "verify", apk)
}
func addZipFiles(path string, files map[string]string) error {
	reader, err := zip.OpenReader(path)
	if err != nil {
		return err
	}
	defer reader.Close()
	tmp, err := os.CreateTemp(filepath.Dir(path), "package-*.zip")
	if err != nil {
		return err
	}
	defer os.Remove(tmp.Name())
	writer := zip.NewWriter(tmp)
	success := false
	defer func() {
		if !success {
			writer.Close()
			tmp.Close()
		}
	}()
	for _, file := range reader.File {
		if _, replace := files[file.Name]; replace {
			return fmt.Errorf("duplicate ZIP entry %q", file.Name)
		}
		if err = writer.Copy(file); err != nil {
			return err
		}
	}
	names := make([]string, 0, len(files))
	for name := range files {
		names = append(names, name)
	}
	sort.Strings(names)
	for _, name := range names {
		input, err := os.Open(files[name])
		if err != nil {
			return err
		}
		header := &zip.FileHeader{Name: name, Method: zip.Deflate}
		out, err := writer.CreateHeader(header)
		if err == nil {
			_, err = io.Copy(out, input)
		}
		input.Close()
		if err != nil {
			return err
		}
	}
	if err = writer.Close(); err != nil {
		return err
	}
	if err = tmp.Close(); err != nil {
		return err
	}
	if err = reader.Close(); err != nil {
		return err
	}
	success = true
	return os.Rename(tmp.Name(), path)
}
func (b *builder) packIOS() error {
	app := filepath.Join(b.root, "Paint.app")
	if err := os.MkdirAll(app, 0755); err != nil {
		return err
	}
	data, err := os.ReadFile(filepath.Join(b.root, "Probe"))
	if err != nil {
		return err
	}
	if err = os.WriteFile(filepath.Join(app, "Probe"), data, 0755); err != nil {
		return err
	}
	id := strings.ReplaceAll(b.config.Fixture, "_", "")
	scene := "GoUIScene"
	if b.config.Fixture == "mobile_start" {
		scene = "ProbeScene"
	}
	platform := "iPhoneOS"
	if b.config.Variant == "simulator" {
		platform = "iPhoneSimulator"
	}
	// 固定测试模板；所有替换字段均受配置枚举限制，并进行 XML 转义。
	var escaped bytes.Buffer
	xml.EscapeText(&escaped, []byte("org.golanggui."+id))
	info := fmt.Sprintf(`<?xml version="1.0" encoding="UTF-8"?><!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd"><plist version="1.0"><dict>
<key>CFBundleIdentifier</key><string>%s</string><key>CFBundleExecutable</key><string>Probe</string><key>CFBundleName</key><string>GOUI Probe</string><key>CFBundlePackageType</key><string>APPL</string><key>CFBundleShortVersionString</key><string>0.0.1</string><key>CFBundleVersion</key><string>1</string><key>CFBundleSupportedPlatforms</key><array><string>%s</string></array><key>LSRequiresIPhoneOS</key><true/><key>MinimumOSVersion</key><string>15.0</string><key>UIDeviceFamily</key><array><integer>1</integer><integer>2</integer></array><key>UILaunchScreen</key><dict/><key>UISupportedInterfaceOrientations</key><array><string>UIInterfaceOrientationPortrait</string><string>UIInterfaceOrientationLandscapeLeft</string><string>UIInterfaceOrientationLandscapeRight</string></array>
<key>CFBundleURLTypes</key><array><dict><key>CFBundleURLSchemes</key><array><string>gouistart</string></array></dict></array>
<key>UIApplicationSceneManifest</key><dict><key>UIApplicationSupportsMultipleScenes</key><false/><key>UISceneConfigurations</key><dict><key>UIWindowSceneSessionRoleApplication</key><array><dict><key>UISceneConfigurationName</key><string>GoUI</string><key>UISceneDelegateClassName</key><string>%s</string></dict></array></dict></dict></dict></plist>`, escaped.String(), platform, scene)
	if err = os.WriteFile(filepath.Join(app, "Info.plist"), []byte(info), 0644); err != nil {
		return err
	}
	fmt.Println("Created unsigned Paint.app; sign on the target Mac before installation.")
	return nil
}
