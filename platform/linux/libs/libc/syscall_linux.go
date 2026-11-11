package libc

import (
	"syscall"

	"github.com/goexlib/cgo"
	"golang.org/x/sys/unix"
)

const (
	EINTR  = syscall.EINTR
	EAGAIN = syscall.EAGAIN
)

func Read(fd int32, buf []byte) (ret int, ec syscall.Errno) {
	r1, _, ec := syscall.RawSyscall(syscall.SYS_READ, uintptr(fd), uintptr(cgo.CSlice(buf)), uintptr(len(buf)))
	return int(r1), ec
}

func Write(fd int32, buf []byte) (ret int, ec syscall.Errno) {
	r1, _, ec := syscall.RawSyscall(syscall.SYS_WRITE, uintptr(fd), uintptr(cgo.CSlice(buf)), uintptr(len(buf)))
	return int(r1), ec
}

func Close(fd int32) {
	syscall.RawSyscall(syscall.SYS_CLOSE, uintptr(fd), 0, 0)
}

func Pipe(fds *[2]int32) error {
	var pair [2]int
	if err := unix.Pipe(pair[:]); err != nil {
		return err
	}
	fds[0], fds[1] = int32(pair[0]), int32(pair[1])
	return nil
}

const (
	F_GETFD = 1 /* get close_on_exec */
	F_SETFD = 2 /* set/clear close_on_exec */
	F_GETFL = 3 /* get file->f_flags */
	F_SETFL = 4 /* set file->f_flags */

	O_NONBLOCK = 00004000

	FD_CLOEXEC = 1
)

func Fcntl(fd int32, cmd, arg uintptr) int {
	ret, _, _ := syscall.RawSyscall(syscall.SYS_FCNTL, uintptr(fd), cmd, arg)
	return int(ret)
}

type PollFd = unix.PollFd

const POLLIN = unix.POLLIN

func Poll(fds []PollFd, timeout int) (ret int, eno syscall.Errno) {
	n, err := unix.Poll(fds, timeout)
	if err == nil {
		return n, 0
	}
	if errno, ok := err.(syscall.Errno); ok {
		return n, errno
	}
	return n, syscall.EINVAL
}
