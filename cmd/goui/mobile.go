package main

import (
	"bytes"
	"context"
	"crypto/sha256"
	"debug/buildinfo"
	"debug/elf"
	"debug/macho"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"sort"
	"strconv"
	"strings"

	mobilebase "github.com/golang-gui/goui/platform/mobile/base"
)

// A component profile selects prepared inputs. Every path is relative to the
// profile directory and every consumed file must have a declared digest.
type mobileComponent struct {
	Format                int               `json:"format"`
	HostOS                string            `json:"host_os"`
	HostArch              string            `json:"host_arch"`
	Target                string            `json:"target"`
	Variant               string            `json:"variant"`
	Base                  string            `json:"base"`
	Inputs                string            `json:"inputs"`
	Tools                 string            `json:"tools"`
	Driver                string            `json:"driver"`
	Linker                string            `json:"linker"`
	Objects               []string          `json:"objects"`
	Libraries             []string          `json:"libraries"`
	Frameworks            []string          `json:"frameworks"`
	Aapt2                 string            `json:"aapt2,omitempty"`
	AndroidJar            string            `json:"android_jar,omitempty"`
	Dex                   string            `json:"dex,omitempty"`
	Signer                string            `json:"signer,omitempty"`
	Verifier              string            `json:"verifier,omitempty"`
	TargetSDK             int               `json:"target_sdk,omitempty"`
	GoLink                string            `json:"golink,omitempty"`
	Files                 map[string]string `json:"files"`
	root                  string
	digest                string
	driver, linker, tools string
}

type mobileOptions struct {
	Components, Variant, Package string
	CgoMode                      string
	Unsigned                     bool
}

type mobileReceipt struct {
	Format        int    `json:"format"`
	Target        string `json:"target"`
	Variant       string `json:"variant"`
	SHA256        string `json:"sha256"`
	ProfileSHA256 string `json:"profile_sha256"`
	GoVersion     string `json:"go_version"`
	CgoMode       string `json:"cgo_mode,omitempty"`
}

func (t target) mobile() bool { return t.OS == "android" || t.OS == "ios" }

func mobileFolder(t target, variant string) string {
	if variant == "simulator" {
		return t.folder() + "-simulator"
	}
	return t.folder()
}

func mobileReadJSON(path string, value any) error {
	f, err := os.Open(path)
	if err != nil {
		return err
	}
	defer f.Close()
	data, err := io.ReadAll(io.LimitReader(f, (4<<20)+1))
	if err != nil {
		return err
	}
	if len(data) > 4<<20 {
		return fmt.Errorf("mobile manifest exceeds 4 MiB: %s", path)
	}
	d := json.NewDecoder(bytes.NewReader(data))
	d.DisallowUnknownFields()
	if err = d.Decode(value); err != nil {
		return fmt.Errorf("%s: %w", path, err)
	}
	if err = d.Decode(new(any)); err != io.EOF {
		return fmt.Errorf("%s: trailing JSON data", path)
	}
	return nil
}

func mobileHash(path string) (string, error) {
	f, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer f.Close()
	h := sha256.New()
	if _, err = io.Copy(h, f); err != nil {
		return "", err
	}
	return hex.EncodeToString(h.Sum(nil)), nil
}

func (m *mobileComponent) file(name string) (string, error) {
	if name == "" || filepath.IsAbs(name) || strings.ContainsAny(name, "\\\x00") || filepath.ToSlash(filepath.Clean(name)) != name || name == ".." || strings.HasPrefix(name, "../") {
		return "", fmt.Errorf("invalid component path %q", name)
	}
	want, ok := m.Files[name]
	if !ok {
		return "", fmt.Errorf("undeclared component file %q", name)
	}
	if len(want) != 64 {
		return "", fmt.Errorf("invalid component digest for %q", name)
	}
	path := filepath.Join(m.root, filepath.FromSlash(name))
	// No file or parent may redirect outside the sealed profile tree.
	current := m.root
	for _, part := range strings.Split(name, "/") {
		current = filepath.Join(current, part)
		s, err := os.Lstat(current)
		if err != nil {
			return "", err
		}
		if s.Mode()&os.ModeSymlink != 0 {
			return "", fmt.Errorf("component symlink %q", name)
		}
	}
	info, err := os.Stat(path)
	if err != nil {
		return "", err
	}
	if !info.Mode().IsRegular() {
		return "", fmt.Errorf("component input is not a file: %s", name)
	}
	got, err := mobileHash(path)
	if err != nil {
		return "", err
	}
	if got != want {
		return "", fmt.Errorf("component SHA-256 mismatch: %s", name)
	}
	return path, nil
}

func openMobileComponent(directory string, t target, variant string) (*mobileComponent, error) {
	return openMobileProfile(directory, t, variant, true)
}

func openMobileProfile(directory string, t target, variant string, support bool) (*mobileComponent, error) {
	if directory == "" {
		return nil, fmt.Errorf("mobile build requires -components pointing to a prepared local component set")
	}
	root, err := filepath.Abs(filepath.Join(directory, mobileFolder(t, variant)))
	if err != nil {
		return nil, err
	}
	root, err = filepath.EvalSymlinks(root)
	if err != nil {
		return nil, err
	}
	path := filepath.Join(root, "mobile.json")
	var m mobileComponent
	if err = mobileReadJSON(path, &m); err != nil {
		return nil, err
	}
	if (m.Format != 1 && m.Format != 2) || m.Format == 1 && (m.HostOS != runtime.GOOS || m.HostArch != runtime.GOARCH) || m.Target != t.String() || m.Variant != variant {
		return nil, fmt.Errorf("mobile component host/target/variant mismatch")
	}
	m.root = root
	m.digest, err = mobileHash(path)
	if err != nil {
		return nil, err
	}
	if len(m.Files) == 0 {
		return nil, fmt.Errorf("empty mobile component")
	}
	keys := make([]string, 0, len(m.Files))
	for k := range m.Files {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	for _, k := range keys {
		if _, err = m.file(k); err != nil {
			return nil, err
		}
	}
	manifests := []string{m.Inputs}
	if support {
		manifests = append(manifests, m.Base)
	}
	if m.Format == 1 && support {
		manifests = append(manifests, m.Tools, m.Driver, m.Linker)
	}
	for _, name := range manifests {
		if _, err = m.file(name); err != nil {
			return nil, err
		}
	}
	inputsPath, _ := m.file(m.Inputs)
	inputs, err := mobilebase.OpenNativeInputs(inputsPath)
	if err != nil {
		return nil, err
	}
	if inputs.GOOS != t.OS || inputs.GOARCH != t.Arch || inputs.Variant != variant {
		return nil, fmt.Errorf("mobile profile/base/link inputs mismatch")
	}
	if support {
		basePath, _ := m.file(m.Base)
		base, err := mobilebase.BaseOpen(basePath)
		if err != nil {
			return nil, err
		}
		if base.GOOS != t.OS || base.GOARCH != t.Arch || base.Variant != variant || inputs.MinimumVersion != base.MinimumVersion {
			return nil, fmt.Errorf("mobile profile/base/link inputs mismatch")
		}
	}
	return &m, nil
}

func validateMobile(c *config, t target, o *mobileOptions) error {
	if o.CgoMode == "" {
		o.CgoMode = "prebuilt"
	}
	if o.CgoMode != "prebuilt" && o.CgoMode != "native" {
		return fmt.Errorf("unsupported mobile cgo strategy %q", o.CgoMode)
	}
	if o.Variant == "" {
		o.Variant = "device"
	}
	if o.Variant != "device" && o.Variant != "simulator" || t.OS == "android" && o.Variant != "device" {
		return fmt.Errorf("unsupported mobile variant %q", o.Variant)
	}
	if t.OS == "android" {
		if len(c.ID) > 127 {
			return fmt.Errorf("android application ID must fit 127 characters")
		}
		if c.Android.Signing.Profile != "" {
			return fmt.Errorf("android signing does not use provisioning profiles")
		}
		if c.Android.Signing.PasswordEnv != "" && c.Android.Signing.PKCS12 == "" {
			return fmt.Errorf("android signing password_env requires pkcs12")
		}
		for _, part := range strings.Split(c.ID, ".") {
			if strings.Contains(part, "-") {
				return fmt.Errorf("android application ID requires Java identifiers without hyphens")
			}
		}
		if c.Android.MinSDK == 0 {
			c.Android.MinSDK = 29
		}
		if c.Android.TargetSDK == 0 {
			c.Android.TargetSDK = 35
		}
		if c.Android.MinSDK < 29 || c.Android.TargetSDK < c.Android.MinSDK {
			return fmt.Errorf("android requires 29 <= min_sdk <= target_sdk")
		}
		if o.Unsigned {
			return fmt.Errorf("Android APK requires signing; omit -unsigned to use a local development key")
		}
	}
	if t.OS == "ios" && c.IOS.MinimumVersion == "" {
		c.IOS.MinimumVersion = "15.0"
	}
	if t.OS == "ios" {
		if _, err := mobileVersion(c.IOS.MinimumVersion); err != nil {
			return err
		}
	}
	return nil
}

func mobileVersion(value string) ([3]int, error) {
	var result [3]int
	parts := strings.Split(value, ".")
	if len(parts) < 2 || len(parts) > 3 {
		return result, fmt.Errorf("invalid iOS minimum_version %q", value)
	}
	for i, p := range parts {
		n, err := strconv.Atoi(p)
		if err != nil || n < 0 || n > 65535 || strconv.Itoa(n) != p {
			return result, fmt.Errorf("invalid iOS minimum_version %q", value)
		}
		result[i] = n
	}
	return result, nil
}

func (m *mobileComponent) options(c *config) (mobilebase.BuildOptions, error) {
	o := mobilebase.BuildOptions{GoCommand: "go", Directory: c.path(c.Main), Package: ".", Entry: true, Libraries: m.Libraries, Frameworks: m.Frameworks}
	paths := map[string]*string{m.Base: &o.Base, m.Inputs: &o.Inputs}
	if m.Format == 2 {
		o.Tools, o.GoDriver, o.Linker = m.tools, m.driver, m.linker
	} else {
		paths[m.Tools], paths[m.Driver], paths[m.Linker] = &o.Tools, &o.GoDriver, &o.Linker
	}
	for name, dest := range paths {
		value, err := m.file(name)
		if err != nil {
			return o, err
		}
		*dest = value
	}
	for _, name := range m.Objects {
		value, err := m.file(name)
		if err != nil {
			return o, err
		}
		o.Objects = append(o.Objects, value)
	}
	return o, nil
}

// Go linker flags use Go's quoted-list grammar, not shell syntax.
func mobileLDFlags(value string) ([]string, error) {
	var result []string
	for value = strings.TrimSpace(value); value != ""; value = strings.TrimSpace(value) {
		if value[0] == '\'' || value[0] == '"' {
			q := value[0]
			end := strings.IndexByte(value[1:], q)
			if end < 0 {
				return nil, fmt.Errorf("unterminated linker flag quote")
			}
			result = append(result, value[1:end+1])
			value = value[end+2:]
			if value != "" && value[0] != ' ' && value[0] != '\t' {
				return nil, fmt.Errorf("linker flag quote must delimit an argument")
			}
		} else {
			end := strings.IndexAny(value, " \t")
			if end < 0 {
				end = len(value)
			}
			result = append(result, value[:end])
			value = value[end:]
		}
	}
	return result, nil
}

func mobileEnvironment(t target, linker string) []string {
	var env []string
	values := map[string]string{"GOOS": t.OS, "GOARCH": t.Arch, "GOARM64": "v8.0", "GOEXPERIMENT": "", "CGO_ENABLED": "1", "GOFLAGS": "", "GOTOOLCHAIN": "local", "CC": strconv.Quote(linker), "CXX": strconv.Quote(linker), "GOUI_LINK_COMMAND": "1"}
	for _, item := range os.Environ() {
		k, _, _ := strings.Cut(item, "=")
		if _, ok := values[k]; !ok {
			env = append(env, item)
		}
	}
	for k, v := range values {
		env = append(env, k+"="+v)
	}
	return env
}

func checkMobileDependencies(ctx context.Context, c *config, t target, m *mobileComponent, tags string) error {
	basePath, err := m.file(m.Base)
	if err != nil {
		return err
	}
	base, err := mobilebase.BaseOpen(basePath)
	if err != nil {
		return err
	}
	linker := m.linker
	if m.Format != 2 {
		linker, err = m.file(m.Linker)
		if err != nil {
			return err
		}
	}
	version := exec.CommandContext(ctx, "go", "env", "GOVERSION")
	version.Dir = c.path(c.Main)
	version.Env = mobileEnvironment(t, linker)
	versionOutput, err := version.Output()
	if err != nil {
		return fmt.Errorf("inspect mobile Go version: %w", err)
	}
	if strings.TrimSpace(string(versionOutput)) != base.GoVersion {
		return fmt.Errorf("mobile component requires %s, found %s", base.GoVersion, strings.TrimSpace(string(versionOutput)))
	}
	args := []string{"list", "-deps", "-json"}
	if tags != "" {
		args = append(args, "-tags", tags)
	}
	args = append(args, ".")
	command := exec.CommandContext(ctx, "go", args...)
	command.Dir = c.path(c.Main)
	command.Env = mobileEnvironment(t, linker)
	var stdout, stderr bytes.Buffer
	command.Stdout = &stdout
	command.Stderr = &stderr
	if err = command.Run(); err != nil {
		return fmt.Errorf("inspect mobile dependencies: %w\n%s", err, &stderr)
	}
	d := json.NewDecoder(&stdout)
	for {
		var p struct {
			ImportPath, Name                  string
			CgoFiles, SwigFiles, SwigCXXFiles []string
		}
		if err = d.Decode(&p); err == io.EOF {
			break
		}
		if err != nil {
			return err
		}
		if p.ImportPath == "plugin" {
			return fmt.Errorf("plugin is unsupported for mobile applications")
		}
		if len(p.CgoFiles)+len(p.SwigFiles)+len(p.SwigCXXFiles) > 0 {
			if _, ok := base.Packages[p.ImportPath]; !ok {
				return fmt.Errorf("native support missing for %s; prepare a matching component or use -cgo=native", p.ImportPath)
			}
		}
	}
	return nil
}

func checkNativeMobileDependencies(ctx context.Context, c *config, t target, tags string) error {
	args := []string{"list", "-deps", "-json"}
	if tags != "" {
		args = append(args, "-tags="+tags)
	}
	command := exec.CommandContext(ctx, "go", append(args, ".")...)
	command.Dir = c.path(c.Main)
	command.Env = append(os.Environ(), "GOOS="+t.OS, "GOARCH="+t.Arch, "CGO_ENABLED=1", "GOUI_BASE=", "GOUI_LINK_COMMAND=")
	var stderr bytes.Buffer
	command.Stderr = &stderr
	if _, err := command.Output(); err != nil {
		return fmt.Errorf("inspect native mobile dependencies: %w\n%s", err, &stderr)
	}
	return nil
}

func mobileBinary(c *config, t target, variant string) string {
	name := c.Executable
	if t.OS == "android" {
		name = "libgoui.so"
	}
	return filepath.Join(c.dir, "dist", mobileFolder(t, variant), name)
}

func buildMobile(ctx context.Context, c *config, t target, m *mobileComponent, o mobileOptions, tags, ldflags, output string, out, errOut io.Writer) (string, error) {
	var options mobilebase.BuildOptions
	var err error
	if o.CgoMode == "native" {
		options = mobilebase.BuildOptions{CgoMode: "native", GoCommand: "go", Directory: c.path(c.Main), Package: ".", Entry: true, Libraries: m.Libraries, Frameworks: m.Frameworks}
		options.Inputs, err = m.file(m.Inputs)
		if err != nil {
			return "", err
		}
		for _, name := range m.Objects {
			var path string
			path, err = m.file(name)
			if err != nil {
				break
			}
			options.Objects = append(options.Objects, path)
		}
	} else {
		options, err = m.options(c)
	}
	if err != nil {
		return "", err
	}
	options.Tags = tags
	options.LDFlags, err = mobileLDFlags(ldflags)
	if err != nil {
		return "", err
	}
	if o.CgoMode == "native" {
		err = checkNativeMobileDependencies(ctx, c, t, tags)
	} else {
		err = checkMobileDependencies(ctx, c, t, m, tags)
	}
	if err != nil {
		return "", err
	}
	if output == "" {
		output = mobileBinary(c, t, o.Variant)
	} else {
		output = c.path(output)
	}
	options.Output = output
	if err = mobilebase.Build(ctx, options, out, errOut); err != nil {
		return "", err
	}
	info, err := buildinfo.ReadFile(output)
	if err != nil {
		return "", err
	}
	hash, err := mobileHash(output)
	if err != nil {
		return "", err
	}
	receipt := mobileReceipt{Format: 2, Target: t.String(), Variant: o.Variant, SHA256: hash, ProfileSHA256: mobileTargetDigest(m), GoVersion: info.GoVersion, CgoMode: o.CgoMode}
	data, err := json.MarshalIndent(receipt, "", "  ")
	if err != nil {
		return "", err
	}
	if err = mobileWriteFile(output+".goui.json", append(data, '\n')); err != nil {
		return "", err
	}
	return output, nil
}

func mobileWriteFile(path string, data []byte) error {
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return err
	}
	f, err := os.CreateTemp(filepath.Dir(path), ".goui-write-*")
	if err != nil {
		return err
	}
	defer os.Remove(f.Name())
	if _, err = f.Write(data); err != nil {
		f.Close()
		return err
	}
	if err = f.Close(); err != nil {
		return err
	}
	return os.Rename(f.Name(), path)
}

func checkMobileReceipt(binary string, t target, m *mobileComponent, o mobileOptions) error {
	var r mobileReceipt
	if err := mobileReadJSON(binary+".goui.json", &r); err != nil {
		return err
	}
	hash, err := mobileHash(binary)
	if err != nil {
		return err
	}
	profile := m.digest
	if r.Format == 2 {
		profile = mobileTargetDigest(m)
	}
	if (r.Format != 1 && r.Format != 2) || r.Target != t.String() || r.Variant != o.Variant || r.SHA256 != hash || r.ProfileSHA256 != profile || r.Format == 2 && r.CgoMode != "prebuilt" && r.CgoMode != "native" {
		return fmt.Errorf("mobile binary/build receipt/component mismatch")
	}
	inputs, err := m.file(m.Inputs)
	if err != nil {
		return err
	}
	selected, err := mobilebase.OpenNativeInputs(inputs)
	if err != nil {
		return err
	}
	if r.CgoMode == "native" {
		if err := checkMobileNativeLibraries(binary, t, m); err != nil {
			return err
		}
		return mobilebase.CheckNativeTarget(binary, selected)
	}
	return mobilebase.CheckNativeBinary(binary, selected)
}

// Packaging currently carries one application binary. Additional native shared
// libraries need an explicit packaging design rather than a silently broken app.
func checkMobileNativeLibraries(path string, t target, m *mobileComponent) error {
	var libraries []string
	if t.OS == "android" {
		file, err := elf.Open(path)
		if err != nil {
			return err
		}
		defer file.Close()
		libraries, err = file.ImportedLibraries()
		if err != nil {
			return err
		}
	} else {
		file, err := macho.Open(path)
		if err != nil {
			return err
		}
		defer file.Close()
		libraries, err = file.ImportedLibraries()
		if err != nil {
			return err
		}
	}
	for _, name := range libraries {
		allowed := strings.HasPrefix(name, "/usr/lib/") || strings.HasPrefix(name, "/System/Library/Frameworks/")
		if t.OS == "android" {
			allowed = false
			for path := range m.Files {
				if strings.HasPrefix(path, "inputs/") && strings.HasSuffix(path, "/"+name) && !strings.Contains(name, "/") {
					allowed = true
					break
				}
			}
		}
		if !allowed {
			return fmt.Errorf("native shared dependency %q is not included by mobile packaging; link it statically or supply matching target inputs", name)
		}
	}
	return nil
}

func runMobile(ctx context.Context, command string, c *config, t target, o mobileOptions, output, tags, ldflags, binary, format string, out, errOut io.Writer) error {
	if err := validateMobile(c, t, &o); err != nil {
		return err
	}
	if command == "package" || command == "build" && o.Package != "" {
		requested := format
		if command == "build" {
			requested = o.Package
		}
		if _, err := mobilePackageFormat(c, t, o, requested); err != nil {
			return err
		}
	}
	var m *mobileComponent
	var err error
	if command == "build" || command == "doctor" {
		if err = validateMobileCgoEnvironment(ctx, c, o.CgoMode, nil); err != nil {
			return err
		}
	}
	if o.CgoMode == "native" || command == "generate" || command == "package" {
		m, err = resolveMobileTarget(ctx, o.Components, t, o.Variant)
	} else {
		m, err = resolveMobileComponent(ctx, c, o.Components, t, o.Variant)
	}
	if err != nil {
		return err
	}
	if o.CgoMode == "prebuilt" && (command == "build" || command == "doctor") {
		path, err := m.file(m.Base)
		if err != nil {
			return err
		}
		base, err := mobilebase.BaseOpen(path)
		if err != nil {
			return err
		}
		flags := base.CgoFlags
		if base.Format == 1 {
			flags = map[string]string{"CGO_CFLAGS": "-O2 -g", "CGO_CPPFLAGS": "", "CGO_CXXFLAGS": "-O2 -g", "CGO_FFLAGS": "-O2 -g", "CGO_LDFLAGS": "-O2 -g"}
		}
		if err := validateMobileCgoEnvironment(ctx, c, o.CgoMode, flags); err != nil {
			return err
		}
	}
	inputPath, _ := m.file(m.Inputs)
	inputs, err := mobilebase.OpenNativeInputs(inputPath)
	if err != nil {
		return err
	}
	if t.OS == "android" {
		minimum, err := strconv.Atoi(inputs.MinimumVersion)
		if err != nil {
			return err
		}
		if c.Android.MinSDK < minimum {
			return fmt.Errorf("android min_sdk is below component minimum %d", minimum)
		}
	} else {
		minimum, err := mobileVersion(inputs.MinimumVersion)
		if err != nil {
			return err
		}
		requested, _ := mobileVersion(c.IOS.MinimumVersion)
		for i := range minimum {
			if requested[i] > minimum[i] {
				break
			}
			if requested[i] < minimum[i] {
				return fmt.Errorf("ios minimum_version is below component minimum %s", inputs.MinimumVersion)
			}
		}
	}
	if m.Format == 2 && o.CgoMode != "native" && (command == "build" || command == "doctor") {
		if err = prepareMobileTools(ctx, m, errOut); err != nil {
			return err
		}
	}
	if command == "doctor" {
		if _, err = loadIcon(c); err != nil {
			return err
		}
		if o.CgoMode == "native" {
			err = checkNativeMobileDependencies(ctx, c, t, tags)
		} else {
			err = checkMobileDependencies(ctx, c, t, m, tags)
		}
		if err != nil {
			return err
		}
		if t.OS == "android" && c.Android.Signing.PKCS12 != "" {
			if _, _, err = mobileIdentity(c, c.Android.Signing); err != nil {
				return err
			}
		}
		if t.OS == "ios" && (c.IOS.Signing.PKCS12 != "" || c.IOS.Signing.Profile != "") {
			if err = checkIOSSigning(c); err != nil {
				return err
			}
		}
		fmt.Fprintf(out, "OK: mobile configuration, components and package dependencies (%s, %s). Device execution not checked.\n", t, o.Variant)
		return nil
	}
	if command == "generate" {
		directory := output
		if directory == "" {
			directory = filepath.Join("dist", mobileFolder(t, o.Variant), "resources")
		}
		directory = c.path(directory)
		if _, err = os.Lstat(directory); err == nil {
			return fmt.Errorf("resource output already exists: %s", directory)
		} else if !os.IsNotExist(err) {
			return err
		}
		files, err := generateMobile(c, t, m, o, directory)
		if err != nil {
			return err
		}
		for _, file := range files {
			fmt.Fprintln(out, file)
		}
		return nil
	}
	if command == "build" && o.Package == "" {
		file, err := buildMobile(ctx, c, t, m, o, tags, ldflags, output, out, errOut)
		if err == nil {
			fmt.Fprintln(out, file)
		}
		return err
	}
	if command == "build" {
		tmp, err := os.MkdirTemp("", "goui-mobile-app-*")
		if err != nil {
			return err
		}
		defer os.RemoveAll(tmp)
		binary, err = buildMobile(ctx, c, t, m, o, tags, ldflags, filepath.Join(tmp, filepath.Base(mobileBinary(c, t, o.Variant))), out, errOut)
		if err != nil {
			return err
		}
		format = o.Package
	} else if binary == "" {
		binary = mobileBinary(c, t, o.Variant)
	} else {
		binary = c.path(binary)
	}
	if command == "package" && o.Components == "" {
		var receipt mobileReceipt
		if err = mobileReadJSON(binary+".goui.json", &receipt); err != nil {
			return err
		}
		if receipt.Format == 1 {
			m, err = embeddedMobileComponent(ctx, receipt.GoVersion, t, o.Variant)
			if err != nil {
				return fmt.Errorf("legacy build receipt requires matching -components or rebuilding the binary: %w", err)
			}
		}
	}
	if err = checkMobileReceipt(binary, t, m, o); err != nil {
		return err
	}
	file, err := packageMobile(ctx, c, t, m, o, binary, format, output, errOut)
	if err != nil {
		return err
	}
	fmt.Fprintln(out, file)
	return nil
}
