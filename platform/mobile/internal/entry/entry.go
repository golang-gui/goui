//go:build android || ios

// Package entry is the minimal precompiled C ABI support for mobile hosts.
package entry

import "C"

var start func() bool

// Register binds the pure Go startup implementation during package init. Native
// exports wait for package initialization through the stock Go cgo wrapper.
func Register(fn func() bool) { start = fn }

//export goui_run
func goui_run() C.int {
	if start == nil || !start() {
		return -1
	}
	return 0
}
