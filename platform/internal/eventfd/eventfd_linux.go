// Package eventfd provides a nonblocking wakeup for native event loops.
package eventfd

import (
	"encoding/binary"
	"io"
	"os"

	"golang.org/x/sys/unix"
)

// EventFD owns a nonblocking, close-on-exec eventfd and must not be copied.
// Signal and Drain may run
// concurrently. The owner must stop both operations before Close; a closed or
// zero EventFD is invalid. Native loops borrow the descriptor returned by FD.
type EventFD struct {
	fd   int
	open bool
}

func New() (*EventFD, error) {
	fd, err := unix.Eventfd(0, unix.EFD_NONBLOCK|unix.EFD_CLOEXEC)
	if err != nil {
		return nil, err
	}
	return &EventFD{fd: fd, open: true}, nil
}

func (e *EventFD) FD() int {
	if e == nil || !e.open {
		return -1
	}
	return e.fd
}

// Signal requests a wakeup. A saturated counter already holds a pending wakeup.
func (e *EventFD) Signal() error {
	fd := e.FD()
	if fd < 0 {
		return os.ErrClosed
	}
	var data [8]byte
	binary.NativeEndian.PutUint64(data[:], 1)
	for {
		n, err := unix.Write(fd, data[:])
		if err == unix.EINTR {
			continue
		}
		if err == unix.EAGAIN {
			return nil
		}
		if err != nil {
			return err
		}
		if n != len(data) {
			return io.ErrShortWrite
		}
		return nil
	}
}

// Drain consumes pending wakeups. An empty counter is already drained.
func (e *EventFD) Drain() error {
	fd := e.FD()
	if fd < 0 {
		return os.ErrClosed
	}
	var data [8]byte
	for {
		n, err := unix.Read(fd, data[:])
		if err == unix.EINTR {
			continue
		}
		if err == unix.EAGAIN {
			return nil
		}
		if err != nil {
			return err
		}
		if n != len(data) {
			return io.ErrUnexpectedEOF
		}
		return nil
	}
}

func (e *EventFD) Close() error {
	fd := e.FD()
	if fd < 0 {
		return os.ErrClosed
	}
	e.open = false
	e.fd = -1
	// Linux releases the descriptor even when close reports EINTR; do not retry.
	return unix.Close(fd)
}
