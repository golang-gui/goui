package libc

import (
	"github.com/goexlib/cgo"
	"syscall"
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
	_, _, eno := syscall.RawSyscall(syscall.SYS_PIPE, uintptr(cgo.Pointer(fds)), 0, 0)
	if eno != 0 {
		return eno
	}
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

type PollFd struct {
	Fd      int32
	Events  int16
	REvents int16
}

const POLLIN = 0x0001

func Poll(fds []PollFd, timeout int) (ret int, eno syscall.Errno) {
	r1, _, eno := syscall.RawSyscall(syscall.SYS_POLL, uintptr(cgo.CSlice(fds)), uintptr(len(fds)), uintptr(timeout))
	return int(r1), eno
}
