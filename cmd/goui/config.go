package main

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"unicode"

	"github.com/pelletier/go-toml/v2"
)

type config struct {
	ID         string `toml:"id"`
	Name       string `toml:"name"`
	Executable string `toml:"executable"`
	Main       string `toml:"main"`
	Version    string `toml:"version"`
	Build      int    `toml:"build"`
	Icon       string `toml:"icon"`
	Metadata   string `toml:"metadata"`
	Windows    struct {
		Console bool   `toml:"console"`
		Icon    string `toml:"icon"`
	} `toml:"windows"`
	MacOS struct {
		Icon            string `toml:"icon"`
		Sandbox         bool   `toml:"sandbox"`
		UserFiles       string `toml:"user_files"`
		PersistAccess   bool   `toml:"persist_access"`
		SigningIdentity string `toml:"signing_identity"`
	} `toml:"macos"`
	Linux struct {
		Categories []string `toml:"categories"`
	} `toml:"linux"`

	dir string
}

type target struct{ OS, Arch string }

func (t target) String() string { return t.OS + "/" + t.Arch }
func (t target) folder() string { return t.OS + "-" + t.Arch }

func parseTarget(value string) (target, error) {
	osName, arch, _ := strings.Cut(value, "/")
	switch value {
	case "windows/amd64", "darwin/arm64", "linux/amd64", "linux/arm64":
		return target{osName, arch}, nil
	}
	return target{}, fmt.Errorf("unsupported target %q (windows/amd64, darwin/arm64, linux/amd64, linux/arm64)", value)
}

var (
	applicationID  = regexp.MustCompile(`^[A-Za-z][A-Za-z0-9-]*(\.[A-Za-z][A-Za-z0-9-]*)+$`)
	executableName = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9_-]*$`)
	versionNumber  = regexp.MustCompile(`^(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)$`)
	categoryName   = regexp.MustCompile(`^[A-Za-z][A-Za-z0-9-]*$`)
)

func loadConfig(name string) (*config, error) {
	name, err := filepath.Abs(name)
	if err != nil {
		return nil, err
	}
	f, err := os.Open(name)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	c := &config{dir: filepath.Dir(name), Main: ".", Build: 1}
	if err := toml.NewDecoder(f).DisallowUnknownFields().Decode(c); err != nil {
		return nil, fmt.Errorf("%s: %w", name, err)
	}
	if err := c.validate(); err != nil {
		return nil, fmt.Errorf("%s: %w", name, err)
	}
	return c, nil
}

func (c *config) validate() error {
	if len(c.ID) > 128 || !applicationID.MatchString(c.ID) {
		return fmt.Errorf("id must be a nonempty application ID, e.g. com.example.Notes")
	}
	if strings.TrimSpace(c.Name) == "" || strings.TrimSpace(c.Name) != c.Name ||
		strings.ContainsAny(c.Name, `/\`) || strings.IndexFunc(c.Name, unicode.IsControl) >= 0 ||
		c.Name == "." || c.Name == ".." || len(c.Name) > 200 {
		return fmt.Errorf("name must be a display name without path separators or control characters")
	}
	if len(c.Executable) > 100 || !executableName.MatchString(c.Executable) || reservedName(c.Executable) {
		return fmt.Errorf("executable must be a portable ASCII basename without extension")
	}
	if !versionNumber.MatchString(c.Version) {
		return fmt.Errorf("version must have three numeric components, e.g. 0.1.0")
	}
	for _, n := range strings.Split(c.Version, ".") {
		if _, err := strconv.ParseUint(n, 10, 16); err != nil {
			return fmt.Errorf("version components must be in 0..65535")
		}
	}
	if c.Build < 1 || c.Build > 9999 {
		return fmt.Errorf("build must be in 1..9999 (compatible with CFBundleVersion and Windows version resources)")
	}
	if c.Main == "" || filepath.IsAbs(c.Main) || strings.HasSuffix(c.Main, ".go") ||
		strings.Contains(c.Main, "...") || strings.IndexFunc(c.Main, unicode.IsControl) >= 0 {
		return fmt.Errorf("main must name a local directory containing one main package")
	}
	if c.Icon == "" || (strings.ToLower(filepath.Ext(c.Icon)) != ".svg" && strings.ToLower(filepath.Ext(c.Icon)) != ".png") {
		return fmt.Errorf("icon must name an SVG or PNG file")
	}
	if c.Windows.Icon != "" && strings.ToLower(filepath.Ext(c.Windows.Icon)) != ".ico" {
		return fmt.Errorf("windows.icon must be an ICO file")
	}
	if c.MacOS.Icon != "" && strings.ToLower(filepath.Ext(c.MacOS.Icon)) != ".icns" {
		return fmt.Errorf("macos.icon must be an ICNS file")
	}
	if c.MacOS.UserFiles != "" && c.MacOS.UserFiles != "read-only" && c.MacOS.UserFiles != "read-write" {
		return fmt.Errorf("macos.user_files must be read-only or read-write")
	}
	if !c.MacOS.Sandbox && (c.MacOS.UserFiles != "" || c.MacOS.PersistAccess) {
		return fmt.Errorf("macos.user_files and persist_access require sandbox = true")
	}
	if c.MacOS.PersistAccess && c.MacOS.UserFiles == "" {
		return fmt.Errorf("macos.persist_access requires user_files access")
	}
	identity := c.MacOS.SigningIdentity
	if strings.TrimSpace(identity) != identity || strings.IndexFunc(identity, unicode.IsControl) >= 0 {
		return fmt.Errorf("macos.signing_identity must not contain surrounding whitespace or control characters")
	}
	if c.MacOS.Sandbox && identity == "" {
		return fmt.Errorf("macos.sandbox requires an explicit signing_identity (use '-' only for development)")
	}
	if c.Metadata != "" {
		path := filepath.Clean(c.Metadata)
		if filepath.IsAbs(path) || path == "." || path == ".." || strings.HasPrefix(path, ".."+string(filepath.Separator)) {
			return fmt.Errorf("metadata must be a subdirectory of the configuration directory")
		}
	}
	for _, category := range c.Linux.Categories {
		if !categoryName.MatchString(category) {
			return fmt.Errorf("invalid linux category %q", category)
		}
	}
	return nil
}

func reservedName(name string) bool {
	name = strings.ToUpper(name)
	if name == "CON" || name == "PRN" || name == "AUX" || name == "NUL" {
		return true
	}
	return len(name) == 4 && (strings.HasPrefix(name, "COM") || strings.HasPrefix(name, "LPT")) && name[3] >= '1' && name[3] <= '9'
}

func (c *config) path(path string) string {
	if filepath.IsAbs(path) {
		return filepath.Clean(path)
	}
	return filepath.Join(c.dir, path)
}

func (c *config) binary(t target) string {
	name := c.Executable
	if t.OS == "windows" {
		name += ".exe"
	}
	return filepath.Join(c.dir, "dist", t.folder(), name)
}
