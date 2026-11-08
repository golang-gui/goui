package main

import (
	"os"
	"syscall"
)

func threadID() uint64        { return uint64(syscall.Gettid()) }
func mainThread() bool        { return syscall.Gettid() == syscall.Getpid() }
func reportDirectory() string { return os.TempDir() }
