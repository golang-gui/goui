package common

import "errors"

// ErrUnsupported is returned when the platform does not support a capability.
var ErrUnsupported = errors.New("unsupported platform")
