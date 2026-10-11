package main

import (
	"bytes"
	"encoding/xml"
	"fmt"
	"strconv"
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
