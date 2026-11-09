//go:build ignore

// This maintainer command consumes verified local profiles. It never runs when
// an application is built. All embedded artifacts are emitted as Go source.
package main

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"flag"
	"fmt"
	"go/format"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"

	"github.com/golang-gui/goui/cmd/goui/internal/mobileassets"
)

type blob struct {
	Data, SHA256 string
	Files        map[string]string
}

func main() {
	dex := flag.String("android-dex", "", "replace only the embedded Android Dex using verified maintainer output")
	android := flag.String("android", os.Getenv("GOUI_ANDROID_COMPONENT"), "verified Android profile directory")
	ios := flag.String("ios", os.Getenv("GOUI_IOS_COMPONENT"), "verified iOS device profile directory")
	simulator := flag.String("simulator", os.Getenv("GOUI_IOS_SIMULATOR_COMPONENT"), "verified iOS simulator profile directory")
	list := flag.String("profiles", os.Getenv("GOUI_MOBILE_PROFILES"), "JSON array of verified profile directories across Go versions")
	flag.Parse()
	if *dex != "" {
		if *android != "" || *ios != "" || *simulator != "" || *list != "" {
			panic("-android-dex cannot be combined with profile options")
		}
		updateAndroidDex(*dex)
		return
	}
	roots := []string{*android, *ios, *simulator}
	if *list != "" {
		readJSON(*list, &roots)
	}
	if len(roots) == 0 {
		panic("verified component profiles are required")
	}
	shared := map[string]map[string][]byte{}
	objects := map[string]map[string][]byte{}
	recipes := map[string]map[string]map[string][]byte{}
	for _, root := range roots {
		if root == "" {
			panic("all verified profile directories are required")
		}
		var profile struct {
			Target, Variant string
			Files           map[string]string
		}
		readJSON(filepath.Join(root, "mobile.json"), &profile)
		key := profile.Target + "/" + profile.Variant
		if key != "android/arm64/device" && key != "ios/arm64/device" && key != "ios/arm64/simulator" {
			panic("unsupported profile target: " + key)
		}
		var manifest struct {
			Format    int
			GoVersion string `json:"go_version"`
		}
		readJSON(filepath.Join(root, "base/base.json"), &manifest)
		if manifest.Format != 2 {
			panic("embedded bases require object recipes (format 2)")
		}
		version := manifest.GoVersion
		if recipes[version] == nil {
			recipes[version] = map[string]map[string][]byte{}
		}
		if recipes[version][key] != nil {
			panic("duplicate version and target profile")
		}
		baseFiles, sharedFiles := map[string][]byte{}, map[string][]byte{}
		if objects[key] == nil {
			objects[key] = map[string][]byte{}
		}
		for name, want := range profile.Files {
			if !strings.HasPrefix(name, "base/") && !strings.HasPrefix(name, "inputs/") && !strings.HasPrefix(name, "platform/") {
				continue
			}
			if name == "platform/android.jar" {
				continue
			}
			if !filepath.IsLocal(name) || strings.Contains(name, "\\") {
				panic("invalid profile path")
			}
			b, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(name)))
			must(err)
			if digest(b) != want {
				panic("profile checksum mismatch: " + name)
			}
			switch {
			case strings.HasPrefix(name, "base/objects/"):
				if strings.TrimPrefix(name, "base/objects/") != want {
					panic("object path is not content addressed")
				}
				if old, ok := objects[key][name]; ok && !bytes.Equal(old, b) {
					panic("object content collision")
				}
				objects[key][name] = b
			case name == "base/base.json":
				baseFiles[name] = b
			case strings.HasPrefix(name, "base/"):
				panic("unexpected base artifact: " + name)
			default:
				sharedFiles[name] = b
			}
		}
		if baseFiles["base/base.json"] == nil {
			panic("undeclared base manifest")
		}
		// Preserve complete target-matching SDK symbols, independent of applications.
		var input map[string]json.RawMessage
		must(json.Unmarshal(sharedFiles["inputs/inputs.json"], &input))
		hashes := map[string]string{}
		for name, b := range sharedFiles {
			if strings.HasSuffix(name, ".tbd") {
				target := "arm64-ios"
				if strings.HasSuffix(key, "/simulator") {
					target += "-simulator"
				}
				b = projectTBD(b, target)
				sharedFiles[name] = b
			}
			if strings.HasPrefix(name, "inputs/") && name != "inputs/inputs.json" {
				hashes[strings.TrimPrefix(name, "inputs/")] = digest(b)
			}
		}
		input["files"], _ = json.Marshal(hashes)
		sharedFiles["inputs/inputs.json"], _ = json.MarshalIndent(input, "", "  ")
		if old := shared[key]; old != nil {
			if len(old) != len(sharedFiles) {
				panic("target inputs differ between Go profiles")
			}
			for name, b := range sharedFiles {
				if !bytes.Equal(b, old[name]) {
					panic("target inputs differ: " + name)
				}
			}
		} else {
			shared[key] = sharedFiles
		}
		recipes[version][key] = baseFiles
	}
	keys := []string{"android/arm64/device", "ios/arm64/device", "ios/arm64/simulator"}
	for version, targets := range recipes {
		var source strings.Builder
		source.WriteString("// Code generated by go generate; DO NOT EDIT.\npackage mobileassets\nfunc init() {\n")
		fmt.Fprintf(&source, "bases[%q] = map[string]Blob{}\n", version)
		for _, key := range keys {
			if targets[key] == nil {
				panic("missing target for " + version + ": " + key)
			}
			writeBlob(&source, fmt.Sprintf("bases[%q][%q]", version, key), pack(targets[key]))
		}
		source.WriteString("}\n")
		writeSource("base_"+strings.ReplaceAll(version, ".", "_")+".go", source.String())
	}
	for _, key := range keys {
		for _, group := range []struct {
			prefix, dest string
			files        map[string][]byte
		}{
			{"target_", "targets", shared[key]}, {"objects_", "objectPools", objects[key]},
		} {
			var source strings.Builder
			source.WriteString("// Code generated by go generate; DO NOT EDIT.\npackage mobileassets\nfunc init() {\n")
			writeBlob(&source, fmt.Sprintf("%s[%q]", group.dest, key), pack(group.files))
			source.WriteString("}\n")
			writeSource(group.prefix+strings.ReplaceAll(key, "/", "_")+".go", source.String())
		}
	}
}

func updateAndroidDex(path string) {
	current, ok := mobileassets.Target("android/arm64", "device")
	if !ok {
		panic("embedded Android target unavailable")
	}
	if digest([]byte(current.Data)) != current.SHA256 {
		panic("embedded target checksum mismatch")
	}
	reader, err := gzip.NewReader(strings.NewReader(current.Data))
	must(err)
	defer reader.Close()
	archive := tar.NewReader(reader)
	files := make(map[string][]byte)
	for {
		header, err := archive.Next()
		if err == io.EOF {
			break
		}
		must(err)
		if header.Typeflag != tar.TypeReg || !filepath.IsLocal(header.Name) {
			panic("invalid embedded target entry")
		}
		if _, exists := files[header.Name]; exists {
			panic("duplicate embedded target entry")
		}
		data, err := io.ReadAll(archive)
		must(err)
		if digest(data) != current.Files[header.Name] {
			panic("embedded target file checksum mismatch")
		}
		files[header.Name] = data
	}
	if len(files) != len(current.Files) {
		panic("incomplete embedded target")
	}
	data, err := os.ReadFile(path)
	must(err)
	if len(data) < 112 || !bytes.HasPrefix(data, []byte("dex\n")) {
		panic("invalid Dex input")
	}
	if _, exists := files["platform/classes.dex"]; !exists {
		panic("Android target has no Dex")
	}
	files["platform/classes.dex"] = data
	var source strings.Builder
	source.WriteString("// Code generated by go generate; DO NOT EDIT.\npackage mobileassets\nfunc init() {\n")
	writeBlob(&source, `targets["android/arm64/device"]`, pack(files))
	source.WriteString("}\n")
	writeSource("target_android_arm64_device.go", source.String())
}

func readJSON(path string, value any) {
	b, err := os.ReadFile(path)
	must(err)
	must(json.Unmarshal(b, value))
}
func digest(b []byte) string { sum := sha256.Sum256(b); return hex.EncodeToString(sum[:]) }
func must(err error) {
	if err != nil {
		panic(err)
	}
}

func pack(files map[string][]byte) blob {
	var out bytes.Buffer
	gz, err := gzip.NewWriterLevel(&out, 9)
	must(err)
	t := tar.NewWriter(gz)
	names := make([]string, 0, len(files))
	for name := range files {
		names = append(names, name)
	}
	sort.Strings(names)
	hashes := map[string]string{}
	for _, name := range names {
		b := files[name]
		hashes[name] = digest(b)
		must(t.WriteHeader(&tar.Header{Name: name, Mode: 0600, Size: int64(len(b)), Typeflag: tar.TypeReg, Format: tar.FormatUSTAR}))
		_, err := t.Write(b)
		must(err)
	}
	must(t.Close())
	must(gz.Close())
	return blob{out.String(), digest(out.Bytes()), hashes}
}

func writeBlob(out *strings.Builder, destination string, b blob) {
	fmt.Fprintf(out, "%s = Blob{SHA256:%q, Files:map[string]string{", destination, b.SHA256)
	names := make([]string, 0, len(b.Files))
	for name := range b.Files {
		names = append(names, name)
	}
	sort.Strings(names)
	for _, name := range names {
		fmt.Fprintf(out, "%q:%q,\n", name, b.Files[name])
	}
	out.WriteString("}, Data: \"\" +\n")
	for i := 0; i < len(b.Data); i += 120 {
		end := min(i+120, len(b.Data))
		fmt.Fprintf(out, "%s", strconv.Quote(b.Data[i:end]))
		if end != len(b.Data) {
			out.WriteString(" +")
		} else {
			out.WriteString(",")
		}
		out.WriteByte('\n')
	}
	out.WriteString("}\n")
}

func writeSource(name, source string) {
	b, err := format.Source([]byte(source))
	must(err)
	must(os.WriteFile(name, b, 0644))
}

// The linker's SDK subset uses flow lists. Keep target-matching documents and
// export groups, all symbols, dependency closures and library version metadata.
// Never reduce the symbol set based on one particular application.
func projectTBD(source []byte, target string) []byte {
	var out strings.Builder
	for _, raw := range strings.Split(string(source), "--- !tapi-tbd") {
		if strings.TrimSpace(raw) == "" {
			continue
		}
		lines := strings.Split(raw, "\n")
		var records []string
		for i := 0; i < len(lines); i++ {
			line := strings.TrimRight(lines[i], " \r")
			trim := strings.TrimSpace(line)
			if trim == "" || trim == "..." || strings.HasPrefix(trim, "#") {
				continue
			}
			if strings.Contains(line, "[") && !strings.Contains(line, "]") {
				parts := []string{line}
				for !strings.Contains(lines[i], "]") {
					i++
					if i == len(lines) {
						panic("unterminated SDK list")
					}
					parts = append(parts, strings.TrimSpace(lines[i]))
				}
				line = strings.Join(parts, " ")
			}
			records = append(records, line)
		}
		name, targets := "", ""
		for _, line := range records {
			if strings.HasPrefix(line, "install-name:") {
				name = strings.Trim(strings.TrimSpace(strings.SplitN(line, ":", 2)[1]), "'")
			}
			if strings.HasPrefix(line, "targets:") {
				targets = strings.SplitN(line, ":", 2)[1]
			}
		}
		selected := target
		if !hasTarget(targets, selected) {
			if target == "arm64-ios-simulator" && hasTarget(targets, "arm64-macos") && (name == "/usr/lib/system/libsystem_kernel.dylib" || name == "/usr/lib/system/libsystem_platform.dylib" || name == "/usr/lib/system/libsystem_pthread.dylib") {
				selected = "arm64-macos"
			} else {
				continue
			}
		}
		out.WriteString("--- !tapi-tbd\n")
		mode, keep := "", false
		for _, line := range records {
			trim := strings.TrimSpace(line)
			if len(line) == len(strings.TrimLeft(line, " ")) {
				key := strings.SplitN(trim, ":", 2)[0]
				mode = key
				switch key {
				case "targets":
					fmt.Fprintf(&out, "targets: [ %s ]\n", selected)
				case "tbd-version", "install-name", "current-version", "compatibility-version", "exports", "reexports", "reexported-libraries", "flags":
					out.WriteString(line + "\n")
				}
				keep = false
			} else if mode == "exports" || mode == "reexports" || mode == "reexported-libraries" {
				if strings.HasPrefix(trim, "- targets:") {
					keep = hasTarget(strings.TrimPrefix(trim, "- targets:"), selected)
					if keep {
						fmt.Fprintf(&out, "  - targets: [ %s ]\n", selected)
					}
				} else if keep {
					out.WriteString(line + "\n")
				}
			}
		}
	}
	if out.Len() == 0 {
		panic("SDK input has no matching target")
	}
	return []byte(out.String())
}

func hasTarget(value, target string) bool {
	for _, item := range strings.Split(strings.Trim(strings.TrimSpace(value), "[]"), ",") {
		if strings.Trim(strings.TrimSpace(item), "'") == target {
			return true
		}
	}
	return false
}
