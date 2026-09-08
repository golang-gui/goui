package common

import "errors"

// ErrUnsupported is returned when the platform does not support a capability.
var ErrUnsupported = errors.New("unsupported platform")

// ErrUnavailable means a supported native observation or operation cannot be
// used currently, for example before WM management or after window destruction.
// A query returning this error has not observed false.
var ErrUnavailable = errors.New("platform information unavailable")
