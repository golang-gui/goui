// Package desktopopen contains shared input rules for desktop open requests.
package desktopopen

import (
	"errors"
	"fmt"
	"net/url"
	"path/filepath"
	"strings"
)

// ValidateURL checks an absolute URI without normalizing or rewriting it.
// Parsing alone does not validate escapes in an opaque part or raw query.
func ValidateURL(rawURL string) error {
	u, err := url.Parse(rawURL)
	if err != nil {
		// url.Error includes the input, which may contain credentials. Keep only
		// the parser's reason in diagnostics.
		var parseErr *url.Error
		if errors.As(err, &parseErr) {
			err = parseErr.Err
		}
		return fmt.Errorf("open URL: %w", err)
	}
	if !u.IsAbs() {
		return errors.New("open URL: an absolute URL with a scheme is required")
	}
	if _, err := url.PathUnescape(rawURL); err != nil {
		return fmt.Errorf("open URL: %w", err)
	}
	return nil
}

// AbsolutePath preserves native filename characters and leaves existence and
// access checks to the operating system's open operation.
func AbsolutePath(path string) (string, error) {
	if path == "" || strings.ContainsRune(path, 0) {
		return "", errors.New("open path: an empty path or embedded NUL is not allowed")
	}
	abs, err := filepath.Abs(path)
	if err != nil {
		return "", fmt.Errorf("open path: %w", err)
	}
	return abs, nil
}
