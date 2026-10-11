package cocoa

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/golang-gui/goui/platform/common"
	. "github.com/golang-gui/goui/platform/darwin/frameworks/appkit"
	. "github.com/golang-gui/goui/platform/darwin/frameworks/core_foundation"
	. "github.com/golang-gui/goui/platform/darwin/frameworks/foundation"
	. "github.com/golang-gui/goui/platform/darwin/frameworks/security"
	"github.com/golang-gui/goui/platform/internal/fileaccess"
)

// The retained original URL carries picker/resolution authority. Rebuilding it
// from a path loses that authority. Only stop an access that this object started.
type fileAccess struct {
	url                      NSURL
	path, name               string
	sandbox, started, closed bool
	options                  NSURLBookmarkCreationOptions
}

func processEntitlement(name string) (bool, error) {
	task := SecTaskCreateFromSelf(0)
	if task == 0 {
		return false, fmt.Errorf("file access: SecTaskCreateFromSelf failed")
	}
	defer CFRelease(task)
	key := CFStringCreateWithString(name)
	defer CFRelease(key)
	var nativeErr CFErrorRef
	value := SecTaskCopyValueForEntitlement(task, key, &nativeErr)
	if value != 0 {
		defer CFRelease(value)
	}
	if nativeErr != 0 {
		defer CFRelease(nativeErr)
		return false, fmt.Errorf("file access: entitlement %s error %d", name, CFErrorGetCode(nativeErr))
	}
	return value == KCFBooleanTrue, nil
}

func (fd *fileDialog) newAccess(url NSURL, restored bool) (*fileAccess, error) {
	if url.ID == 0 {
		return nil, fmt.Errorf("file access: native result has no URL")
	}
	path := url.Path()
	if !filepath.IsAbs(path) {
		return nil, fmt.Errorf("file access: URL has no absolute local path")
	}
	url.Retain()
	a := &fileAccess{url: url, path: path, name: filepath.Base(path), sandbox: fd.sandbox, options: fd.bookmarkOptions}
	a.started = url.StartAccessingSecurityScopedResource()
	// Picker URLs may already have an implicit grant. Restored scoped URLs,
	// however, require a successful explicit start before returning access.
	if fd.sandbox && restored && !a.started {
		a.Close()
		return nil, fmt.Errorf("file access: start security scope: %w", os.ErrPermission)
	}
	return a, nil
}

func nativeFileError(operation string, err NSError) error {
	if err.ID == 0 {
		return fmt.Errorf("file access: %s failed without NSError", operation)
	}
	return fmt.Errorf("file access: %s: %s (%s %d)", operation, err.LocalizedDescription(), err.Domain(), err.Code())
}

func (a *fileAccess) Name() string { return a.name }
func (a *fileAccess) LocalPath() (string, error) {
	if a.closed {
		return "", os.ErrClosed
	}
	return a.path, nil
}

func (a *fileAccess) credentialKind() string {
	if a.sandbox {
		return "macos-security-scope"
	}
	return "macos-bookmark"
}

func (a *fileAccess) Credential() (result []byte, err error) {
	if a.closed {
		return nil, os.ErrClosed
	}
	AutoReleasePool(func() {
		var nativeErr NSError
		data := a.url.BookmarkData(a.options, &nativeErr)
		if data.ID == 0 {
			err = nativeFileError("create credential", nativeErr)
			return
		}
		result, err = fileaccess.Encode(a.credentialKind(), data.Bytes())
	})
	return
}

func (a *fileAccess) Close() error {
	if a.closed {
		return nil
	}
	a.closed = true
	AutoReleasePool(func() {
		if a.started {
			a.url.StopAccessingSecurityScopedResource()
		}
		a.url.Release()
		a.url.ID = 0
	})
	return nil
}

func (fd *fileDialog) RestoreAccess(credential []byte) (result common.FileAccess, err error) {
	kind := "macos-bookmark"
	if fd.sandbox {
		kind = "macos-security-scope"
	}
	data, err := fileaccess.Decode(credential, kind)
	if err != nil {
		return nil, err
	}
	AutoReleasePool(func() {
		options := NSURLBookmarkResolutionWithoutUI
		if fd.sandbox {
			options |= NSURLBookmarkResolutionWithSecurityScope
		}
		var stale bool
		var nativeErr NSError
		url := NSURLClassId.ResolveBookmarkData(NewNativeData(data), options, &stale, &nativeErr)
		if url.ID == 0 {
			err = nativeFileError("restore credential", nativeErr)
			return
		}
		var access *fileAccess
		access, err = fd.newAccess(url, true)
		if err != nil {
			return
		}
		if _, err = os.Stat(access.path); err == nil && stale {
			// Verify that a fresh credential can be produced. Credential always
			// serializes the current URL, never returns the stale input bytes.
			_, err = access.Credential()
		}
		if err != nil {
			access.Close()
			return
		}
		result = access
	})
	return
}

func (fd *fileDialog) selectedAccess(urls NSArray) (result []common.FileAccess, err error) {
	for i := uintptr(0); i < urls.Count(); i++ {
		access, createErr := fd.newAccess(Cast[NSURL](urls.ObjectAtIndex(i)), false)
		if createErr != nil {
			for _, previous := range result {
				previous.Close()
			}
			return nil, createErr
		}
		result = append(result, access)
	}
	if len(result) == 0 {
		return nil, fmt.Errorf("file access: successful picker returned no URLs")
	}
	return result, nil
}
