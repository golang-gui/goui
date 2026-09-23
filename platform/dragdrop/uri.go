package dragdrop

import (
	"fmt"
	"net/url"
	"path/filepath"
	"runtime"
	"strings"
	"unicode/utf8"
)

// FileURLFromPath encodes one absolute local path without probing the file.
// Windows UNC paths retain their host; ordinary paths use an empty URI host.
func FileURLFromPath(path string) (string, error) {
	if err := validateFilePath(path); err != nil {
		return "", err
	}
	if runtime.GOOS == "windows" {
		if strings.HasPrefix(path, `\\`) {
			parts := strings.SplitN(strings.TrimPrefix(path, `\\`), `\`, 2)
			if len(parts) != 2 || parts[0] == "" || parts[1] == "" {
				return "", fmt.Errorf("dragdrop: invalid UNC file path %q", path)
			}
			return (&url.URL{Scheme: "file", Host: parts[0], Path: "/" + filepath.ToSlash(parts[1])}).String(), nil
		}
		return (&url.URL{Scheme: "file", Path: "/" + filepath.ToSlash(path)}).String(), nil
	}
	return (&url.URL{Scheme: "file", Path: path}).String(), nil
}

// FilePathFromURL accepts only a local file URI. A foreign file host on Unix
// is not silently interpreted as a local path.
func FilePathFromURL(raw string) (string, error) {
	u, err := url.Parse(raw)
	if err != nil || !strings.EqualFold(u.Scheme, "file") || u.Opaque != "" || u.User != nil || u.RawQuery != "" || u.Fragment != "" {
		return "", fmt.Errorf("dragdrop: invalid local file URI %q", raw)
	}
	if u.Path == "" || !strings.HasPrefix(u.Path, "/") {
		return "", fmt.Errorf("dragdrop: invalid local file URI %q", raw)
	}
	var path string
	if runtime.GOOS == "windows" {
		if u.Host != "" && !strings.EqualFold(u.Host, "localhost") {
			path = `\\` + u.Host + `\` + filepath.FromSlash(strings.TrimPrefix(u.Path, "/"))
		} else {
			path = filepath.FromSlash(strings.TrimPrefix(u.Path, "/"))
		}
	} else {
		if u.Host != "" && !strings.EqualFold(u.Host, "localhost") {
			return "", fmt.Errorf("dragdrop: non-local file URI host %q", u.Host)
		}
		path = u.Path
	}
	if err := validateFilePath(path); err != nil {
		return "", err
	}
	return path, nil
}

// EncodeURIList uses the text/uri-list wire representation (CRLF separated).
func EncodeURIList(urls []string) ([]byte, error) {
	if len(urls) == 0 || len(urls) > MaxItems {
		return nil, fmt.Errorf("dragdrop: invalid URI count %d", len(urls))
	}
	var b strings.Builder
	for _, raw := range urls {
		if err := validateURL(raw); err != nil {
			return nil, err
		}
		if b.Len()+len(raw)+2 > MaxDataBytes {
			return nil, fmt.Errorf("dragdrop: URI list exceeds %d bytes", MaxDataBytes)
		}
		b.WriteString(raw)
		b.WriteString("\r\n")
	}
	return []byte(b.String()), nil
}

// DecodeURIList accepts CRLF or LF, comments, and empty lines, preserving the
// order of actual URIs. It never opens or resolves a URL.
func DecodeURIList(data []byte) ([]string, error) {
	if len(data) > MaxDataBytes || !utf8.Valid(data) || strings.IndexByte(string(data), 0) >= 0 {
		return nil, fmt.Errorf("dragdrop: invalid URI list")
	}
	var urls []string
	for _, line := range strings.Split(string(data), "\n") {
		line = strings.TrimSuffix(line, "\r")
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		if err := validateURL(line); err != nil {
			return nil, err
		}
		urls = append(urls, line)
		if len(urls) > MaxItems {
			return nil, fmt.Errorf("dragdrop: URI list exceeds %d entries", MaxItems)
		}
	}
	return urls, nil
}

func validateFilePath(path string) error {
	if path == "" || !utf8.ValidString(path) || strings.ContainsRune(path, 0) || !filepath.IsAbs(path) {
		return fmt.Errorf("dragdrop: invalid absolute file path %q", path)
	}
	return nil
}

func validateURL(raw string) error {
	if raw == "" || !utf8.ValidString(raw) || strings.ContainsAny(raw, "\x00\r\n") {
		return fmt.Errorf("dragdrop: invalid absolute URI %q", raw)
	}
	u, err := url.Parse(raw)
	if err != nil || u.Scheme == "" {
		return fmt.Errorf("dragdrop: invalid absolute URI %q", raw)
	}
	if strings.EqualFold(u.Scheme, "file") {
		_, err = FilePathFromURL(raw)
		return err
	}
	return nil
}
