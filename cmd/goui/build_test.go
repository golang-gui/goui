package main

import (
	"bytes"
	"context"
	"debug/pe"
	"encoding/binary"
	"encoding/xml"
	"fmt"
	"image/png"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"unicode/utf16"
)

// These tests compile standard-library-only main packages in t.TempDir. They
// need a Go toolchain, not a desktop or foreign executable runner.
func TestBuildAndPackage(t *testing.T) {
	if testing.Short() {
		t.Skip("cross-build integration needs a Go toolchain; run without -short")
	}
	for _, name := range []string{"windows/amd64", "darwin/arm64", "linux/amd64", "linux/arm64"} {
		t.Run(name, func(t *testing.T) {
			c := testConfig(t)
			c.Metadata = "internal/appinfo"
			writeTestFile(t, filepath.Join(c.dir, "main.go"), []byte(`package main
import "example.com/packaging-test/internal/appinfo"
func main() { println(appinfo.ID, appinfo.Name, appinfo.Version, appinfo.Build) }
`))
			target, _ := parseTarget(name)
			var log bytes.Buffer
			binaryPath, err := build(context.Background(), c, target, "", "", "", &log, &log)
			if err != nil {
				t.Fatalf("build: %v\n%s", err, &log)
			}
			if binaryPath != c.binary(target) {
				t.Fatal("wrong build output", binaryPath)
			}
			original, err := os.ReadFile(binaryPath)
			if err != nil {
				t.Fatal(err)
			}
			if err := checkBinary(binaryPath, target); err != nil {
				t.Fatal(err)
			}
			other, _ := parseTarget("windows/amd64")
			if target.OS == "windows" {
				other, _ = parseTarget("linux/amd64")
				verifyWindowsResources(t, binaryPath, c, false)
			} else if target.String() == "linux/arm64" {
				other, _ = parseTarget("linux/amd64")
			}
			if err := checkBinary(binaryPath, other); err == nil {
				t.Fatal("accepted binary for wrong target")
			}
			if target.OS == "windows" {
				if _, err := packageApp(c, target, "exe", "", ""); err == nil {
					t.Fatal("meaningless EXE package accepted")
				}
				c.Windows.Console = true
				second, err := build(context.Background(), c, target, "", "", "console.exe", &log, &log)
				if err != nil {
					t.Fatalf("repeated build: %v\n%s", err, &log)
				}
				verifyWindowsResources(t, second, c, true)
				object := filepath.Join(c.dir, "goui_windows_amd64.syso")
				data, _ := os.ReadFile(object)
				writeTestFile(t, filepath.Join(c.dir, "user_windows_amd64.syso"), data)
				if _, err := generate(context.Background(), c, target, "", ""); err == nil || !strings.Contains(err.Error(), "resource syso") {
					t.Fatalf("did not reject competing resources: %v", err)
				}
			} else {
				// Empty PATH makes an accidental go build during package fail.
				t.Setenv("PATH", t.TempDir())
				if target.OS == "darwin" {
					if _, err := packageApp(c, target, "app", "", "NotAnApp"); err == nil {
						t.Fatal("accepted a macOS bundle without .app suffix")
					}
				}
				app, err := packageApp(c, target, "", "", "")
				if err != nil {
					t.Fatal(err)
				}
				executable := filepath.Join(app, "usr", "bin", c.Executable)
				if target.OS == "darwin" {
					executable = filepath.Join(app, "Contents", "MacOS", c.Executable)
					data, err := os.ReadFile(filepath.Join(app, "Contents", "Info.plist"))
					if err != nil {
						t.Fatal(err)
					}
					verifyPlist(t, data, c)
				} else {
					data, err := os.ReadFile(filepath.Join(app, "usr", "share", "applications", c.ID+".desktop"))
					if err != nil || !strings.Contains(string(data), "StartupWMClass="+c.ID) || !strings.Contains(string(data), "Exec=/usr/bin/"+c.Executable) {
						t.Fatalf("desktop entry: %s %v", data, err)
					}
					data, err = os.ReadFile(filepath.Join(app, "usr", "share", "icons", "hicolor", "256x256", "apps", c.ID+".png"))
					if err != nil {
						t.Fatal(err)
					}
					img, err := png.Decode(bytes.NewReader(data))
					if err != nil {
						t.Fatal(err)
					}
					assertIconPixels(t, img, 256)
				}
				data, _ := os.ReadFile(executable)
				if !bytes.Equal(data, original) {
					t.Fatal("packaged binary was changed")
				}
				info, err := os.Stat(executable)
				if err != nil || (runtime.GOOS != "windows" && info.Mode().Perm() != 0755) {
					t.Fatal("executable permissions lost", err)
				}
				if _, err := packageApp(c, target, "", "", ""); err == nil {
					t.Fatal("overwrote existing package")
				}
			}
			data, _ := os.ReadFile(binaryPath)
			if !bytes.Equal(data, original) {
				t.Fatal("input binary was changed")
			}
		})
	}
}

func TestBuildFailureKeepsBinary(t *testing.T) {
	c := testConfig(t)
	target, _ := parseTarget("windows/amd64")
	path, err := build(context.Background(), c, target, "", "", "", io.Discard, io.Discard)
	if err != nil {
		t.Fatal(err)
	}
	original, _ := os.ReadFile(path)
	writeTestFile(t, filepath.Join(c.dir, "main.go"), []byte("package main\nfunc main() { undefined() }\n"))
	if _, err := build(context.Background(), c, target, "", "", "", io.Discard, io.Discard); err == nil {
		t.Fatal("invalid code built successfully")
	}
	data, _ := os.ReadFile(path)
	if !bytes.Equal(data, original) {
		t.Fatal("failed build changed previous binary")
	}
	residue, err := filepath.Glob(filepath.Join(filepath.Dir(path), ".goui-build-*"))
	if err != nil || len(residue) != 0 {
		t.Fatal("build temporary files leaked", residue, err)
	}
}

func verifyPlist(t *testing.T, data []byte, c *config) {
	t.Helper()
	decoder := xml.NewDecoder(bytes.NewReader(data))
	values := map[string]string{}
	var key string
	for {
		token, err := decoder.Token()
		if err == io.EOF {
			break
		}
		if err != nil {
			t.Fatal("invalid plist XML", err)
		}
		if start, ok := token.(xml.StartElement); ok {
			switch start.Name.Local {
			case "key":
				if err := decoder.DecodeElement(&key, &start); err != nil {
					t.Fatal(err)
				}
			case "string":
				var value string
				if err := decoder.DecodeElement(&value, &start); err != nil {
					t.Fatal(err)
				}
				values[key] = value
			}
		}
	}
	for key, want := range map[string]string{
		"CFBundleIdentifier": c.ID, "CFBundleDisplayName": c.Name, "CFBundleExecutable": c.Executable,
		"CFBundlePackageType": "APPL", "CFBundleVersion": fmt.Sprint(c.Build),
		"CFBundleShortVersionString": c.Version, "CFBundleIconFile": "AppIcon.icns",
	} {
		if values[key] != want {
			t.Fatalf("plist %s: %q, want %q", key, values[key], want)
		}
	}
}

func verifyWindowsResources(t *testing.T, path string, c *config, console bool) {
	t.Helper()
	file, err := pe.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer file.Close()
	header := file.OptionalHeader.(*pe.OptionalHeader64)
	wantSubsystem := uint16(pe.IMAGE_SUBSYSTEM_WINDOWS_GUI)
	if console {
		wantSubsystem = pe.IMAGE_SUBSYSTEM_WINDOWS_CUI
	}
	if header.Subsystem != wantSubsystem {
		t.Fatalf("subsystem %d, want %d", header.Subsystem, wantSubsystem)
	}
	// Independently walk the linked PE resource directory. Do not merely check
	// generator output: Go must have linked RT_ICON/GROUP_ICON/VERSION/MANIFEST.
	rvaData := func(rva, length uint32) []byte {
		for _, section := range file.Sections {
			if rva < section.VirtualAddress {
				continue
			}
			data, err := section.Data()
			if err != nil {
				t.Fatal(err)
			}
			offset := uint64(rva - section.VirtualAddress)
			if offset+uint64(length) <= uint64(len(data)) {
				return data[offset : offset+uint64(length)]
			}
		}
		t.Fatal("invalid resource RVA", rva, length)
		return nil
	}
	directory := header.DataDirectory[2]
	data := rvaData(directory.VirtualAddress, directory.Size)
	resources := map[uint32][][]byte{}
	var walk func(uint32, uint32, int)
	walk = func(offset, resourceType uint32, depth int) {
		if depth > 3 || uint64(offset)+16 > uint64(len(data)) {
			t.Fatal("invalid resource directory")
		}
		count := int(binary.LittleEndian.Uint16(data[offset+12:])) + int(binary.LittleEndian.Uint16(data[offset+14:]))
		for i := 0; i < count; i++ {
			entry := uint64(offset) + 16 + uint64(i)*8
			if entry+8 > uint64(len(data)) {
				t.Fatal("truncated resource directory")
			}
			id := binary.LittleEndian.Uint32(data[entry:])
			child := binary.LittleEndian.Uint32(data[entry+4:])
			kind := resourceType
			if depth == 0 {
				kind = id
			}
			if child&0x80000000 != 0 {
				walk(child&0x7fffffff, kind, depth+1)
			} else {
				if uint64(child)+16 > uint64(len(data)) {
					t.Fatal("invalid resource leaf")
				}
				rva, size := binary.LittleEndian.Uint32(data[child:]), binary.LittleEndian.Uint32(data[child+4:])
				resources[kind] = append(resources[kind], rvaData(rva, size))
			}
		}
	}
	walk(0, 0, 0)
	for _, kind := range []uint32{3, 14, 16, 24} {
		if len(resources[kind]) == 0 {
			t.Fatal("missing linked resource type", kind)
		}
	}
	found256 := false
	for _, data := range resources[3] {
		img, err := png.Decode(bytes.NewReader(data))
		if err != nil {
			t.Fatal("linked icon is not PNG", err)
		}
		assertIconPixels(t, img, img.Bounds().Dx())
		found256 = found256 || img.Bounds().Dx() == 256
	}
	if !found256 {
		t.Fatal("linked icons lost 256px")
	}
	version := resources[16][0]
	var displayName []byte
	for _, unit := range utf16.Encode([]rune(c.Name)) {
		displayName = binary.LittleEndian.AppendUint16(displayName, unit)
	}
	if !bytes.Contains(version, displayName) || !bytes.Contains(version, []byte{0xbd, 0x04, 0xef, 0xfe}) {
		t.Fatal("linked version resource lost name or fixed version")
	}
	manifest := string(resources[24][0])
	if !strings.Contains(manifest, c.ID) || strings.Contains(manifest, "requestedExecutionLevel") || strings.Contains(manifest, "dpiAware") {
		t.Fatal("incorrect manifest", manifest)
	}
}
