package main

import (
	"bytes"
	"encoding/xml"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
)

func plist(c *config) []byte {
	var b bytes.Buffer
	b.WriteString(`<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd">
<plist version="1.0"><dict>
`)
	for _, item := range [][2]string{
		{"CFBundleIdentifier", c.ID}, {"CFBundleName", c.Name}, {"CFBundleDisplayName", c.Name},
		{"CFBundleInfoDictionaryVersion", "6.0"},
		{"CFBundleExecutable", c.Executable}, {"CFBundlePackageType", "APPL"},
		{"CFBundleShortVersionString", c.Version}, {"CFBundleVersion", strconv.Itoa(c.Build)},
		{"CFBundleIconFile", "AppIcon.icns"},
	} {
		fmt.Fprintf(&b, "  <key>%s</key><string>", item[0])
		xml.EscapeText(&b, []byte(item[1]))
		b.WriteString("</string>\n")
	}
	b.WriteString("  <key>NSHighResolutionCapable</key><true/>\n</dict></plist>\n")
	return b.Bytes()
}

// Entitlements are separate from Info.plist and bundle assets. They become
// effective only when attached to the executable's signature by codesign.
func macOSEntitlements(c *config) []byte {
	keys := []string{"com.apple.security.app-sandbox"}
	if c.MacOS.UserFiles != "" {
		keys = append(keys, "com.apple.security.files.user-selected."+c.MacOS.UserFiles)
	}
	if c.MacOS.PersistAccess {
		keys = append(keys, "com.apple.security.files.bookmarks.app-scope")
	}
	var body strings.Builder
	body.WriteString("<?xml version=\"1.0\" encoding=\"UTF-8\"?>\n<!DOCTYPE plist PUBLIC \"-//Apple//DTD PLIST 1.0//EN\" \"http://www.apple.com/DTDs/PropertyList-1.0.dtd\">\n<plist version=\"1.0\"><dict>\n")
	for _, key := range keys {
		fmt.Fprintf(&body, "<key>%s</key><true/>\n", key)
	}
	body.WriteString("</dict></plist>\n")
	return []byte(body.String())
}

func signMacOS(c *config, bundle string) error {
	if runtime.GOOS != "darwin" {
		return fmt.Errorf("macOS signing requires packaging on macOS with codesign")
	}
	args := []string{"--force", "--sign", c.MacOS.SigningIdentity, "--timestamp=none"}
	if c.MacOS.Sandbox {
		work, err := os.MkdirTemp(filepath.Dir(bundle), ".goui-sign-*")
		if err != nil {
			return err
		}
		defer os.RemoveAll(work) // Only this call's temporary entitlement directory.
		path := filepath.Join(work, "Entitlements.plist")
		if err := os.WriteFile(path, macOSEntitlements(c), 0600); err != nil {
			return err
		}
		args = append(args, "--entitlements", path)
	}
	args = append(args, bundle)
	if output, err := exec.Command("/usr/bin/codesign", args...).CombinedOutput(); err != nil {
		return fmt.Errorf("codesign: %w: %s", err, strings.TrimSpace(string(output)))
	}
	if output, err := exec.Command("/usr/bin/codesign", "--verify", "--strict", bundle).CombinedOutput(); err != nil {
		return fmt.Errorf("verify signature: %w: %s", err, strings.TrimSpace(string(output)))
	}
	return nil
}
