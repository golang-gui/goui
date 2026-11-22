package common

// FileAccess owns access to a selected file or directory, not an open file.
// Use it on the platform thread. The caller owns successful results and must
// Close them after all I/O finishes; ownership is independent of the dialog
// and its owner window. Do not rely on garbage collection to release access.
type FileAccess interface {
	// Name returns the display name, including after Close.
	Name() string
	// LocalPath returns an absolute local path. Backends without local paths
	// return ErrUnsupported. A path alone does not preserve sandbox access.
	LocalPath() (string, error)
	// Credential returns caller-owned opaque data for RestoreAccess. Persist
	// it in application storage, not as a password or permanent authorization.
	// For a new save destination, create the file before calling this method.
	// After restoration, persist the current data again to refresh stale references.
	Credential() ([]byte, error)
	// Close releases owned native access and references, not the file itself.
	// It is idempotent; LocalPath/Credential subsequently return os.ErrClosed.
	// It does not revoke access separately granted by the operating system.
	Close() error
}

// FileFilterFormat describes the native representation accepted by a file dialog.
// It does not define GUI filtering policy.
type FileFilterFormat uint8

const (
	// FileFilterUnknown means no supported filter representation is reported.
	FileFilterUnknown FileFilterFormat = iota
	// FileFilterGlob accepts case-sensitive shell globs, including bracket sets.
	FileFilterGlob
	// FileFilterWildcard accepts case-insensitive native wildcards.
	FileFilterWildcard
	// FileFilterExtension accepts bare extensions; an empty list allows all types.
	FileFilterExtension
)

// FileFilter contains values in the dialog's reported native FilterFormat.
// Native presentation of multiple named filters depends on the backend.
type FileFilter struct {
	Name   string
	Values []string
}

// DialogOptions configures a native file dialog.
type DialogOptions struct {
	Title         string       // Dialog title; empty uses platform default
	InitialDir    string       // Initial directory; empty uses platform default
	DefaultName   string       // Default filename (with extension) for save dialogs; ignored for open
	Filters       []FileFilter // Filter list; empty means no filter
	AllowMultiple bool         // Only effective for OpenFile; ignored for directory/save
}

// FileDialog is the native file dialog capability. It is thread-affine and
// must be used on the thread that owns the platform.
type FileDialog interface {
	// FilterFormat reports the native filter representation accepted by this
	// dialog, including its fallback paths. It does not start a picker.
	FilterFormat() FileFilterFormat
	// OpenFile opens a native file dialog for selecting one or more files.
	// owner requests native association; nil leaves the dialog unowned.
	// Positioning is controlled by the operating system.
	// cb is called exactly once on the platform thread with caller-owned access.
	// Cancel is reported as empty slice with nil error.
	OpenFile(owner Window, opts DialogOptions, cb func([]FileAccess, error))
	// OpenDirectory opens a native dialog for selecting a directory.
	// owner requests native association; nil leaves the dialog unowned.
	// Positioning is controlled by the operating system.
	// cb is called exactly once on the platform thread with caller-owned access.
	// Cancel is reported as empty slice with nil error.
	OpenDirectory(owner Window, opts DialogOptions, cb func([]FileAccess, error))
	// SaveFile opens a native save-file dialog.
	// owner requests native association; nil leaves the dialog unowned.
	// Positioning is controlled by the operating system.
	// cb is called exactly once on the platform thread with caller-owned access.
	// The dialog selects a destination; it does not create or write a file.
	// Cancel is reported as empty slice with nil error.
	SaveFile(owner Window, opts DialogOptions, cb func([]FileAccess, error))
	// RestoreAccess restores caller-owned access without displaying a picker.
	// Invalid data, unavailable resources and native failures are returned.
	// This never grants privileges beyond those permitted by the OS.
	RestoreAccess(credential []byte) (FileAccess, error)
}
