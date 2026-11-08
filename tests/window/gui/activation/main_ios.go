package main

import (
	"os"
	"path/filepath"

	"github.com/goexlib/cgo"
	"github.com/golang-gui/goui/platform/darwin/libs/dispatch"
)

func threadID() uint64 {
	address, err := cgo.Dlsym(cgo.RTLD_DEFAULT, "pthread_self")
	must(err)
	return uint64(cgo.CallRet[uintptr](address))
}
func mainThread() bool { return dispatch.IsMainThread() }
func reportDirectory() string {
	home, err := os.UserHomeDir()
	must(err)
	return filepath.Join(home, "Documents", "MobileActivation")
}
