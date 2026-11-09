package eventfd

import (
	"encoding/binary"
	"errors"
	"math"
	"os"
	"sync"

	"golang.org/x/sys/unix"
	"testing"
)

func TestEventFDWakeAndDrain(t *testing.T) {
	wakeup, err := New()
	if err != nil {
		t.Fatal(err)
	}
	defer wakeup.Close()
	fd := wakeup.FD()
	flags, errno := unix.FcntlInt(uintptr(fd), unix.F_GETFL, 0)
	if errno != nil || flags&unix.O_NONBLOCK == 0 {
		t.Fatal("唤醒描述符必须非阻塞", errno)
	}
	descriptorFlags, errno := unix.FcntlInt(uintptr(fd), unix.F_GETFD, 0)
	if errno != nil || descriptorFlags&unix.FD_CLOEXEC == 0 {
		t.Fatal("唤醒描述符不能泄漏给子进程", errno)
	}
	if err := wakeup.Drain(); err != nil {
		t.Fatal("空描述符不能阻塞或报错", err)
	}
	var wg sync.WaitGroup
	for range 8 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for range 64 {
				if err := wakeup.Signal(); err != nil {
					t.Error(err)
					return
				}
			}
		}()
	}
	wg.Wait()
	var value [8]byte
	if n, err := unix.Read(int(fd), value[:]); err != nil || n != 8 || binary.NativeEndian.Uint64(value[:]) != 512 {
		t.Fatalf("并发唤醒丢失：%v n=%d count=%d", err, n, binary.NativeEndian.Uint64(value[:]))
	}
	if err := wakeup.Signal(); err != nil {
		t.Fatal(err)
	}
	if err := wakeup.Drain(); err != nil {
		t.Fatal(err)
	}
	if _, err := unix.Read(int(fd), value[:]); !errors.Is(err, unix.EAGAIN) {
		t.Fatal("排空后仍可读取", err)
	}
	binary.NativeEndian.PutUint64(value[:], math.MaxUint64-1)
	if _, err := unix.Write(int(fd), value[:]); err != nil {
		t.Fatal(err)
	}
	if err := wakeup.Signal(); err != nil {
		t.Fatal("已有唤醒时不应因 EAGAIN 失败", err)
	}
	if err := wakeup.Drain(); err != nil {
		t.Fatal(err)
	}
}
func TestEventFDInvalidDescriptor(t *testing.T) {
	wakeup, err := New()
	if err != nil {
		t.Fatal(err)
	}
	// 外部关闭描述符时，保留原生错误；本例不再复用或关闭该失效号码。
	if err := unix.Close(wakeup.FD()); err != nil {
		t.Fatal(err)
	}
	if !errors.Is(wakeup.Signal(), unix.EBADF) || !errors.Is(wakeup.Drain(), unix.EBADF) {
		t.Fatal("原生描述符错误被隐藏")
	}
}
func TestEventFDClosed(t *testing.T) {
	var zero EventFD
	if zero.FD() != -1 || !errors.Is(zero.Signal(), os.ErrClosed) || !errors.Is(zero.Drain(), os.ErrClosed) || !errors.Is(zero.Close(), os.ErrClosed) {
		t.Fatal("零值不能操作标准输入或其他描述符")
	}
	wakeup, err := New()
	if err != nil {
		t.Fatal(err)
	}
	if err := wakeup.Close(); err != nil {
		t.Fatal(err)
	}
	if wakeup.FD() != -1 || !errors.Is(wakeup.Signal(), os.ErrClosed) || !errors.Is(wakeup.Drain(), os.ErrClosed) || !errors.Is(wakeup.Close(), os.ErrClosed) {
		t.Fatal("关闭后仍可使用唤醒对象")
	}
}
