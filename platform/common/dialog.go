package common

// FileFilter defines a file type filter for dialog boxes.
type FileFilter struct {
	Name     string   // Display name, e.g. "Images"
	Patterns []string // e.g. {"*.png", "*.jpg"}
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
	// OpenFile opens a native file dialog for selecting one or more files.
	// owner is the parent window; nil means desktop-centered.
	// cb is called exactly once on the platform thread with selected paths.
	// Cancel is reported as empty slice with nil error.
	OpenFile(owner Window, opts DialogOptions, cb func([]string, error))
	// OpenDirectory opens a native dialog for selecting a directory.
	// owner is the parent window; nil means desktop-centered.
	// cb is called exactly once on the platform thread with selected paths.
	// Cancel is reported as empty slice with nil error.
	OpenDirectory(owner Window, opts DialogOptions, cb func([]string, error))
	// SaveFile opens a native save-file dialog.
	// owner is the parent window; nil means desktop-centered.
	// cb is called exactly once on the platform thread with selected path.
	// Cancel is reported as empty slice with nil error.
	SaveFile(owner Window, opts DialogOptions, cb func([]string, error))
}
