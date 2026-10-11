package main

import (
	"bytes"
	"debug/pe"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/josephspurrier/goversioninfo"
)

func windowsManifest(c *config) string {
	// No requestedExecutionLevel or DPI declaration: elevation is out of scope,
	// and GOUI selects DPI awareness during its native startup.
	return fmt.Sprintf(`<?xml version="1.0" encoding="UTF-8" standalone="yes"?>
<assembly xmlns="urn:schemas-microsoft-com:asm.v1" manifestVersion="1.0">
  <assemblyIdentity type="win32" name="%s" version="%s.%d" processorArchitecture="amd64"/>
  <compatibility xmlns="urn:schemas-microsoft-com:compatibility.v1">
    <application><supportedOS Id="{8e0f7a12-bfb3-4fe8-b9a5-48fd50a15a9a}"/></application>
  </compatibility>
</assembly>
`, c.ID, c.Version, c.Build)
}

func generateWindows(c *config, pkg goPackage, assets map[string][]byte) (string, error) {
	const name = "goui_windows_amd64.syso"
	for _, file := range pkg.SysoFiles {
		if file == name {
			continue
		}
		object, err := pe.Open(filepath.Join(pkg.Dir, file))
		if err != nil {
			return "", fmt.Errorf("inspect syso %s: %w", file, err)
		}
		hasResources := false
		for _, section := range object.Sections {
			hasResources = hasResources || strings.HasPrefix(section.Name, ".rsrc")
		}
		object.Close()
		if hasResources {
			return "", fmt.Errorf("resource syso %s already exists; choose one resource generator before building", file)
		}
	}
	work, err := os.MkdirTemp("", "goui-resource-*")
	if err != nil {
		return "", err
	}
	defer os.RemoveAll(work) // Only this newly allocated scratch directory.
	for _, file := range []string{"AppIcon.ico", "app.manifest"} {
		if err := os.WriteFile(filepath.Join(work, file), assets[file], 0644); err != nil {
			return "", err
		}
	}
	var version goversioninfo.FileVersion
	parts := strings.Split(c.Version, ".")
	version.Major, _ = strconv.Atoi(parts[0])
	version.Minor, _ = strconv.Atoi(parts[1])
	version.Patch, _ = strconv.Atoi(parts[2])
	version.Build = c.Build
	vi := goversioninfo.VersionInfo{
		FixedFileInfo: goversioninfo.FixedFileInfo{
			FileVersion: version, ProductVersion: version, FileFlagsMask: "3f", FileOS: "040004", FileType: "01",
		},
		StringFileInfo: goversioninfo.StringFileInfo{
			FileDescription: c.Name, InternalName: c.Executable, OriginalFilename: c.Executable + ".exe", ProductName: c.Name,
		},
		VarFileInfo: goversioninfo.VarFileInfo{Translation: goversioninfo.Translation{LangID: 0x0409, CharsetID: goversioninfo.CsUnicode}},
		IconPath:    filepath.Join(work, "AppIcon.ico"), ManifestPath: filepath.Join(work, "app.manifest"),
	}
	vi.Build()
	vi.Walk()
	objectPath := filepath.Join(work, name)
	if err := vi.WriteSyso(objectPath, "amd64"); err != nil {
		return "", err
	}
	data, err := os.ReadFile(objectPath)
	if err != nil {
		return "", err
	}
	if object, err := pe.NewFile(bytes.NewReader(data)); err != nil {
		return "", fmt.Errorf("invalid generated COFF: %w", err)
	} else {
		object.Close()
	}
	path := filepath.Join(pkg.Dir, name)
	if err := writeGenerated(path, data); err != nil {
		return "", err
	}
	return path, nil
}
