package main

import (
	"fmt"
	"strings"
)

func desktopString(value string) string {
	return strings.NewReplacer("\\", "\\\\", "\n", "\\n", "\r", "\\r", "\t", "\\t").Replace(value)
}

func desktopEntry(c *config) string {
	categories := c.Linux.Categories
	if len(categories) == 0 {
		categories = []string{"Utility"}
	}
	// executable is a checked ASCII basename; it cannot contain Exec reserved
	// characters or field codes. Display names use Desktop Entry escaping.
	return fmt.Sprintf(`[Desktop Entry]
Type=Application
Name=%s
Exec=/usr/bin/%s
Icon=%s
StartupWMClass=%s
Terminal=false
Categories=%s;
`, desktopString(c.Name), c.Executable, c.ID, c.ID, strings.Join(categories, ";"))
}
